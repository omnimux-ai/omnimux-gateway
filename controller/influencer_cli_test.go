package controller

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/analytics"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// Public literal storage keeps ownership and frozen-byte validation real while
// every read remains inside the in-memory fixture.
type influencerCLIImageTestDriver struct {
	*influencerPresetTestDriver
	publicOrigin string
}

func (d *influencerCLIImageTestDriver) GetPublicURL(key string) string {
	origin := d.publicOrigin
	if origin == "" {
		origin = "https://8.8.8.8"
	}
	return origin + "/" + strings.TrimPrefix(key, "/")
}

func (d *influencerCLIImageTestDriver) Upload(ctx context.Context, key string, data []byte, contentType string) (string, error) {
	if _, err := d.influencerPresetTestDriver.Upload(ctx, key, data, contentType); err != nil {
		return "", err
	}
	return d.GetPublicURL(key), nil
}

func createInfluencerCLIImageUpload(t *testing.T, userID int, key string, data []byte) string {
	t.Helper()
	driver := storage.GetStorageDriver()
	imageURL, err := driver.Upload(context.Background(), key, data, "image/png")
	require.NoError(t, err)
	require.NoError(t, model.CreateFile(&model.File{FileId: "file_" + strings.ReplaceAll(key, "/", "_"), UserId: userID, FileName: filepath.Base(key), StorageKey: key, UploadPath: filepath.Dir(key), FileUrl: imageURL, MimeType: "image/png", FileSize: int64(len(data)), ExpiresAt: time.Now().Add(time.Hour), Status: model.FileStatusActive}))
	return imageURL
}

func setupInfluencerCLIAPI(t *testing.T) (*gin.Engine, *model.UserAccessToken) {
	t.Helper()
	user := model.User{Id: 41, Username: "cli-owner", AffCode: "cli-owner", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 100000}
	return setupInfluencerCLIAPIWithOwner(t, &user)
}

func setupInfluencerCLIAPIWithOwner(t *testing.T, user *model.User) (*gin.Engine, *model.UserAccessToken) {
	t.Helper()
	db, dialect := openInfluencerCLITestDatabase(t, &model.Task{}, &model.InfluencerGenerationRequest{}, &model.User{}, &model.UserAccessToken{}, &model.Ability{}, &model.Channel{}, &model.SystemTask{}, &model.SystemTaskLock{}, &model.File{})
	previousDB, previousMain, previousLog := model.DB, common.MainDatabaseType(), common.LogDatabaseType()
	previousStorage := storage.GetStorageDriver()
	previousGroups, previousRatios := setting.UserUsableGroups2JSONString(), ratio_setting.GroupRatio2JSONString()
	previousAuto := setting.AutoGroups2JsonString()
	previousAddress, previousPublic, previousSecret := system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret
	model.DB = db
	common.SetDatabaseTypes(dialect, dialect)
	storage.SetGlobalDriver(&influencerCLIImageTestDriver{influencerPresetTestDriver: newInfluencerPresetTestDriver()})
	system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret = "https://gateway.example", "", "cli-controller-test-only"
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"default","gpt-image-2.5-sunburst-image":"anchor","gpt-image-2.5-sunburst-test":"image"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-sunburst-image":1,"gpt-image-2.5-sunburst-test":1}`))
	require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-sunburst-test","gpt-image-2.5-sunburst-image"]`))
	t.Cleanup(func() {
		model.DB = previousDB
		common.SetDatabaseTypes(previousMain, previousLog)
		storage.SetGlobalDriver(previousStorage)
		require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(previousGroups))
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(previousRatios))
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(previousAuto))
		system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret = previousAddress, previousPublic, previousSecret
	})
	t.Setenv("INFLUENCER_CLI_CREATION_ENABLED", "true")
	require.NoError(t, db.Create(user).Error)
	token := &model.UserAccessToken{UserId: user.Id, TokenHash: "test-only-cli-token", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, token.SetScopes([]string{"influencer_creation:read", "influencer_creation:write"}))
	require.NoError(t, db.Create(token).Error)
	channel := model.Channel{Id: 1, Name: "test-only", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: influencerImageModel, Group: "gpt-image-2.5-sunburst-test", OtherSettings: `{"image_task_contract":{"protocol":"openai_multipart","modes":["text"],"sizes":["1024x576"]}}`}
	require.NoError(t, db.Create(&channel).Error)
	for _, binding := range []model.Ability{
		{Group: channel.Group, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true},
		{Group: "default", Model: influencerMotionModel, ChannelId: channel.Id, Enabled: true},
	} {
		require.NoError(t, db.Create(&binding).Error)
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/influencer/v1", func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Set("group", user.Group)
		user.ToBaseUser().WriteContext(c)
		c.Set("use_access_token", true)
		c.Set("access_token_id", token.Id)
		c.Set("access_token_scopes", []string{"influencer_creation:read", "influencer_creation:write"})
	})
	RegisterInfluencerCLIRoutes(group)
	return engine, token
}

func influencerCLIAPIRequest(t *testing.T, engine *gin.Engine, method, path string, input any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	return influencerCreationAPIRequest(t, engine, method, "/api/influencer/v1"+path, input)
}

func influencerCreationAPIRequest(t *testing.T, engine *gin.Engine, method, path string, input any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	body, err := common.Marshal(input)
	require.NoError(t, err)
	request := httptest.NewRequest(method, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	if strings.HasPrefix(path, "/api/influencer/v1/") {
		request.Header.Set("X-Omnimux-Group", "caller-cannot-choose")
	}
	recorder := httptest.NewRecorder()
	engine.ServeHTTP(recorder, request)
	var response map[string]any
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response), recorder.Body.String())
	return recorder, response
}

func TestInfluencerCLIAcceptanceReplaySurvivesSwitchAndExpiredMaterial(t *testing.T) {
	engine, _ := setupInfluencerCLIAPI(t)
	input := gin.H{"request_id": "character-replay-1", "brief": "person in a red shirt"}
	recorder, response := influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters", input)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	data := response["data"].(map[string]any)
	taskID := data["task_id"].(string)
	assert.Equal(t, "waiting", data["status"])
	assert.Equal(t, "pending", data["artifact_status"])
	assert.Equal(t, "character-replay-1", data["request_id"])
	assert.Empty(t, data["artifacts"])
	assert.NotContains(t, data, "credits")
	var accepted model.Task
	require.NoError(t, model.DB.Where("task_id = ?", taskID).First(&accepted).Error)
	assert.Equal(t, influencerImageGroup, accepted.Group)
	assert.Equal(t, influencerImageModel, accepted.Properties.OriginModelName)
	require.NotNil(t, accepted.PrivateData.GenerationQueue.ImageInputs)
	assert.Equal(t, "text", accepted.PrivateData.GenerationQueue.ImageInputs.Mode)
	assert.NotContains(t, string(accepted.PrivateData.GenerationQueue.Body), "caller-cannot-choose")
	assert.NotContains(t, string(accepted.PrivateData.GenerationQueue.Body), "Authorization")
	require.NotNil(t, accepted.PrivateData.GenerationQueue.Routing)
	// A pre-policy receipt must retain its original pinned route on replay,
	// rather than inheriting the newly prepared native policy.
	accepted.Group = "gpt-image-2.5-legacy"
	accepted.PrivateData.GenerationQueue.Routing = nil
	require.NoError(t, accepted.Update())

	t.Setenv("INFLUENCER_CLI_CREATION_ENABLED", "false")
	// Replay needs no current route, expired URLs or new scanner task.
	require.NoError(t, model.DB.Model(&model.Ability{}).Where("channel_id = ?", 1).Update("enabled", false).Error)
	recorder, response = influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters", input)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, taskID, response["data"].(map[string]any)["task_id"])
	assert.Equal(t, true, response["data"].(map[string]any)["replayed"])
	var legacy model.Task
	require.NoError(t, model.DB.Where("task_id = ?", taskID).First(&legacy).Error)
	assert.Equal(t, "gpt-image-2.5-legacy", legacy.Group)
	assert.Nil(t, legacy.PrivateData.GenerationQueue.Routing)
	input["brief"] = "different person"
	recorder, response = influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters", input)
	assert.Equal(t, http.StatusConflict, recorder.Code)
	assert.Equal(t, "creation_request_conflict", response["code"])
	input["request_id"] = "character-new-2"
	recorder, response = influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters", input)
	assert.Equal(t, http.StatusServiceUnavailable, recorder.Code)
	assert.Equal(t, "influencer_creation_disabled", response["code"])
	recorder, response = influencerCLIAPIRequest(t, engine, http.MethodGet, "/requests/character-replay-1", nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, taskID, response["data"].(map[string]any)["task_id"])
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestInfluencerCLIReceiptRecoveryAndAuthorizationBarrier(t *testing.T) {
	engine, token := setupInfluencerCLIAPI(t)
	// The guard sees an authenticated request, while the durable barrier sees a revoked token.
	require.NoError(t, model.DB.Delete(token).Error)
	recorder, response := influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters", gin.H{"request_id": "revoked-creation", "brief": "a person"})
	assert.Equal(t, http.StatusForbidden, recorder.Code, recorder.Body.String())
	assert.Equal(t, "influencer_creation_scope_denied", response["code"])
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&count).Error)
	assert.Zero(t, count)
	recorder, response = influencerCLIAPIRequest(t, engine, http.MethodGet, "/requests/never-accepted", nil)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
	assert.Equal(t, "creation_request_not_found", response["code"])
}

func TestInfluencerCLIHistoryRemainsOwnedFilteredAndArtifactHonest(t *testing.T) {
	engine, _ := setupInfluencerCLIAPI(t)
	for _, task := range []*model.Task{
		{TaskID: "main-old", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"normal","brief":"owner"}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://upstream-secret.example/result.png"}},
		{TaskID: "child-new", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"kind":"multiview","parent_task_id":"main-old","tier":"normal","brief":"owner"}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://upstream-secret.example/child.png"}},
		{TaskID: "foreign-main", UserId: 99, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"cats","selection":{}}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://private-owner.example/cat.png"}},
		{TaskID: "other-business", UserId: 41, Action: "unrelated", Status: model.TaskStatusSuccess},
		{TaskID: "no-artifact", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"cats","selection":{}}`, OriginModelName: "gpt-image-2.5"}},
		{TaskID: "failed-unknown-refund", UserId: 41, Action: ActionInfluencerMotion, Status: model.TaskStatusFailure, Quota: 5000, FailReason: "private upstream https://secret.example/failed", Properties: model.Properties{Input: `{"kind":"motion","prepared_video_url":"https://secret.example/frozen","prepared_video_sha256":"private"}`}},
	} {
		require.NoError(t, model.DB.Create(task).Error)
	}
	recorder, response := influencerCLIAPIRequest(t, engine, http.MethodGet, "/characters?page=1&page_size=1", nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	data := response["data"].(map[string]any)
	assert.Equal(t, float64(1), data["total"])
	items := data["items"].([]any)
	require.Len(t, items, 1)
	assert.Equal(t, "main-old", items[0].(map[string]any)["task_id"])
	assert.NotContains(t, recorder.Body.String(), "upstream-secret")
	assert.NotContains(t, recorder.Body.String(), "foreign-main")
	recorder, response = influencerCLIAPIRequest(t, engine, http.MethodGet, "/characters/main-old", nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Len(t, response["data"].(map[string]any)["multiviews"], 1)
	for _, id := range []string{"foreign-main", "other-business"} {
		recorder, _ = influencerCLIAPIRequest(t, engine, http.MethodGet, "/tasks/"+id, nil)
		assert.Equal(t, http.StatusNotFound, recorder.Code)
	}
	recorder, response = influencerCLIAPIRequest(t, engine, http.MethodGet, "/tasks/no-artifact", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "succeeded", response["data"].(map[string]any)["status"])
	assert.Equal(t, "unavailable", response["data"].(map[string]any)["artifact_status"])
	recorder, response = influencerCLIAPIRequest(t, engine, http.MethodGet, "/tasks/failed-unknown-refund", nil)
	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "reconciling", response["data"].(map[string]any)["quota_status"])
	assert.NotContains(t, recorder.Body.String(), "secret.example")
	assert.NotContains(t, recorder.Body.String(), "prepared_video")
	assert.NotContains(t, recorder.Body.String(), "refunded")
}

func TestInfluencerCLIVideoUsesOneInspectionAndPreservesOrderedMaterials(t *testing.T) {
	for _, mode := range []string{"motion", "replace"} {
		t.Run(mode, func(t *testing.T) {
			aspectRatio := "9:16"
			if mode == "replace" {
				aspectRatio = "16:9"
			}
			engine, _ := setupInfluencerCLIAPI(t)
			previous := service.InfluencerVideoInspector
			defer func() { service.InfluencerVideoInspector = previous }()
			var inspected atomic.Int32
			service.InfluencerVideoInspector = func(_ context.Context, _ string, owner int, _ func(string) bool) (service.PreparedInfluencerVideo, error) {
				inspected.Add(1)
				assert.Equal(t, 41, owner)
				return service.PreparedInfluencerVideo{Metadata: service.InfluencerVideoMetadata{Duration: 5.2, Width: 1280, Height: 720, FrameRate: 30, VideoCodec: "h264", AudioCodec: "aac", Container: "mp4"}, URL: motionTestFrozenVideo, SHA256: "accepted-video-bytes"}, nil
			}
			character := model.Task{TaskID: "cat-main", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"cats","selection":{}}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://upstream.example/cat.png"}}
			require.NoError(t, model.DB.Create(&character).Error)
			input := gin.H{"mode": mode, "request_id": "video-ordered-1", "character_task_id": character.TaskID, "image_urls": []string{motionTestImage, "https://files.omnimux.ai/images/outfit.png"}, "video_url": motionTestVideo, "aspect_ratio": aspectRatio}
			recorder, response := influencerCLIAPIRequest(t, engine, http.MethodPost, "/videos", input)
			require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
			taskID := response["data"].(map[string]any)["task_id"].(string)
			var task model.Task
			require.NoError(t, model.DB.Where("task_id = ?", taskID).First(&task).Error)
			var out, meta map[string]any
			require.NoError(t, common.Unmarshal(task.PrivateData.GenerationQueue.Body, &out))
			require.NoError(t, common.UnmarshalJsonStr(task.Properties.Input, &meta))
			assert.Equal(t, int32(1), inspected.Load())
			assert.Equal(t, float64(5), out["duration"])
			assert.Equal(t, "1080p", out["resolution"])
			assert.Equal(t, aspectRatio, out["aspect_ratio"])
			assert.Equal(t, true, out["generate_audio"])
			if mode == "replace" {
				assert.Contains(t, out["prompt"], "自主确定合理的替换目标与范围")
				assert.Contains(t, out["prompt"], "完整实现替换，消除与参考目标冲突的原有特征")
				assert.Contains(t, out["prompt"], "只修改完成替换所必需的部分，其余画面与原音沿用原片")
				assert.Contains(t, out["prompt"], "允许自然融合所需的透视、遮挡、接触、光影和运动响应调整")
				assert.Contains(t, out["prompt"], "仅用于身份替换时采用其已核实身份：拟人猫身份")
				assert.Contains(t, out["prompt"], "局部引用仅作用对应部分。用户补充的明确意图优先。")
				for _, specific := range []string{"服饰", "衣物", "物品", "袖长", "类别", "同类"} {
					assert.NotContains(t, out["prompt"], specific)
				}
				assert.NotContains(t, out["prompt"], "用户补充：")
				assert.NotContains(t, out["prompt"], "PRIORITY:")
			} else {
				assert.Contains(t, out["prompt"], "anthropomorphic cat")
			}
			images := out["image_urls"].([]any)
			require.Len(t, images, 3)
			assert.Contains(t, images[0], "/v1/tasks/cat-main/artifacts/image/content?access=")
			assert.Equal(t, motionTestImage, images[1])
			assert.Equal(t, "https://files.omnimux.ai/images/outfit.png", images[2])
			assert.Equal(t, []any{motionTestFrozenVideo}, out["video_urls"])
			assert.NotContains(t, out, "seconds")
			assert.NotContains(t, out, "images")
			assert.NotContains(t, out, "videos")
			assert.NotContains(t, out, "audios", "no separate audio reference is submitted")
			assert.NotContains(t, meta, "audio_extracted")
			if mode == "replace" {
				assert.Contains(t, out["prompt"], "声音与参考视频1保持一致：保留视频1原音及其与画面的同步，不新增、不替换、不重写任何声音。")
			} else {
				assert.Contains(t, out["prompt"], influencerMotionAudioRule, "the prompt carries the audio-consistency instruction")
			}
			assert.Equal(t, "cat", meta["subject_character_type"])
			assert.Equal(t, "cat-main", meta["source_task_id"])
			// Accepted replay bypasses all material inspection and route work.
			recorder, _ = influencerCLIAPIRequest(t, engine, http.MethodPost, "/videos", input)
			assert.Equal(t, http.StatusOK, recorder.Code)
			assert.Equal(t, int32(1), inspected.Load())
		})
	}
}

func TestInfluencerCLIVideoFramingDefaultsAndRejections(t *testing.T) {
	for _, test := range []struct {
		name, mode, resolution, explicit, want string
		width, height, status                  int
	}{
		{"motion follows source", "motion", "1080p", "", "16:9", 1280, 720, 200},
		{"motion defaults portrait for arbitrary source", "motion", "1080p", "", "9:16", 1200, 960, 200},
		{"motion supports 1x1", "motion", "1080p", "", "1:1", 1000, 1000, 200},
		{"motion supports 4x3", "motion", "1080p", "4:3", "4:3", 1280, 960, 200},
		{"motion supports 21x9", "motion", "1080p", "21:9", "21:9", 2520, 1080, 200},
		{"replace follows source", "replace", "1080p", "", "16:9", 1280, 720, 200},
		{"replace rejects conflicting ratio", "replace", "1080p", "9:16", "", 1280, 720, 400},
		{"replace rejects unsupported source", "replace", "1080p", "", "", 1200, 960, 400},
		{"1080p accepts 3x4", "motion", "1080p", "3:4", "3:4", 960, 1280, 200},
		{"480p rejected", "motion", "480p", "", "", 1280, 720, 400},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, _ := setupInfluencerCLIAPI(t)
			previous := service.InfluencerVideoInspector
			defer func() { service.InfluencerVideoInspector = previous }()
			service.InfluencerVideoInspector = func(context.Context, string, int, func(string) bool) (service.PreparedInfluencerVideo, error) {
				return service.PreparedInfluencerVideo{Metadata: service.InfluencerVideoMetadata{Duration: 5, Width: test.width, Height: test.height, FrameRate: 24, VideoCodec: "h264", Container: "mp4"}, URL: motionTestFrozenVideo}, nil
			}
			input := gin.H{"request_id": "video-ratio-1", "mode": test.mode, "resolution": test.resolution, "image_urls": []string{motionTestImage}, "video_url": motionTestVideo}
			if test.explicit != "" {
				input["aspect_ratio"] = test.explicit
			}
			recorder, response := influencerCLIAPIRequest(t, engine, http.MethodPost, "/videos", input)
			require.Equal(t, test.status, recorder.Code, recorder.Body.String())
			var count int64
			require.NoError(t, model.DB.Model(&model.Task{}).Count(&count).Error)
			if test.status != 200 {
				assert.Zero(t, count, "invalid media never reaches durable generation or billing")
				return
			}
			assert.Equal(t, int64(1), count)
			assert.Equal(t, test.want, response["data"].(map[string]any)["input"].(map[string]any)["aspect_ratio"])
		})
	}
}

func TestInfluencerCLIVideoRejectsShortReferencesBeforeAcceptance(t *testing.T) {
	if service.GetHttpClient() == nil {
		service.InitHttpClient()
	}
	media := make(map[int][]byte)
	for _, seconds := range []int{2, 4} {
		path := filepath.Join(t.TempDir(), "reference.mp4")
		output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=720x1280:r=24", "-t", fmt.Sprint(seconds), "-an", "-c:v", "libx264", "-threads", "1", path).CombinedOutput()
		require.NoError(t, err, string(output))
		media[seconds], err = os.ReadFile(path)
		require.NoError(t, err)
	}
	for _, test := range []struct {
		name                string
		mediaSeconds        int
		probedDuration      float64
		probedVideoDuration float64
		status              int
	}{
		{"two seconds", 2, 0, 0, http.StatusBadRequest},
		{"fraction below four seconds", 4, 3.999, 3.999, http.StatusBadRequest},
		{"short visual track inside four seconds", 4, 0, 2, http.StatusBadRequest},
		{"four seconds", 4, 0, 0, http.StatusOK},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine, _ := setupInfluencerCLIAPI(t)
			require.NoError(t, model.DB.AutoMigrate(&model.File{}, &model.Log{}))
			t.Cleanup(func() { require.NoError(t, model.DB.Migrator().DropTable(&model.File{}, &model.Log{})) })
			previousLogDB := model.LOG_DB
			model.LOG_DB = model.DB
			t.Cleanup(func() { model.LOG_DB = previousLogDB })
			settings := system_setting.GetFetchSetting()
			previousSettings := *settings
			settings.EnableSSRFProtection = false
			t.Cleanup(func() { *settings = previousSettings })
			driver := storage.GetStorageDriver().(*influencerCLIImageTestDriver)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(media[test.mediaSeconds])
			}))
			t.Cleanup(server.Close)
			previousInspector := service.InfluencerVideoInspector
			t.Cleanup(func() { service.InfluencerVideoInspector = previousInspector })
			var inspections int
			service.InfluencerVideoInspector = func(ctx context.Context, _ string, owner int, valid func(string) bool) (service.PreparedInfluencerVideo, error) {
				inspections++
				prepared, err := previousInspector(ctx, server.URL, owner, valid)
				require.NoError(t, err)
				require.Len(t, driver.objects, 1, "execute actual freezing before checking the CLI-only duration policy")
				if test.probedDuration != 0 {
					// Exercise the exact fractional metadata boundary independently
					// of the generated fixture's frame-duration quantization.
					prepared.Metadata.Duration = test.probedDuration
				}
				if test.probedVideoDuration != 0 {
					prepared.Metadata.VideoDuration = test.probedVideoDuration
				}
				return prepared, nil
			}
			recorder, response := influencerCLIAPIRequest(t, engine, http.MethodPost, "/videos", gin.H{"request_id": "video-duration-guard-1", "image_urls": []string{motionTestImage}, "video_url": motionTestVideo})
			require.Equal(t, test.status, recorder.Code, recorder.Body.String())
			assert.Equal(t, 1, inspections, "the CLI must inspect each reference only once")
			var user model.User
			require.NoError(t, model.DB.First(&user, 41).Error)
			assert.Equal(t, 100000, user.Quota, "acceptance and rejection must not charge before queue execution")
			var logs int64
			require.NoError(t, model.LOG_DB.Model(&model.Log{}).Count(&logs).Error)
			assert.Zero(t, logs)
			if test.status == http.StatusBadRequest {
				assert.Equal(t, "reference_video_invalid", response["code"])
				for _, record := range []any{&model.Task{}, &model.InfluencerGenerationRequest{}, &model.File{}, &model.SystemTask{}} {
					var count int64
					require.NoError(t, model.DB.Model(record).Count(&count).Error)
					assert.Zero(t, count, "short references must leave no generation, receipt, frozen file or scanner task")
				}
				assert.Empty(t, driver.objects)
				assert.Len(t, driver.deleted, 1, "rejected frozen media must be discarded")
				return
			}
			var task model.Task
			require.NoError(t, model.DB.First(&task).Error)
			assert.Equal(t, model.TaskStatusWaiting, task.Status)
			assert.Zero(t, task.Quota)
			var out map[string]any
			require.NoError(t, common.Unmarshal(task.PrivateData.GenerationQueue.Body, &out))
			// A 4s source clip is rendered/billed at the 4s MAGI-2 output floor.
			assert.Equal(t, float64(4), out["duration"])
			assert.Len(t, driver.objects, 1, "accepted tasks retain their frozen reference")
			assert.Empty(t, driver.deleted)
		})
	}
}

func TestInfluencerCLIRejectsConsumerRoutingAndInvalidInput(t *testing.T) {
	engine, _ := setupInfluencerCLIAPI(t)
	for index, input := range []gin.H{
		{"request_id": "valid-id-123", "model": "caller-model", "brief": "person"},
		{"request_id": "valid-id-123", "group": "caller-group", "brief": "person"},
		{"request_id": "short", "brief": "person"},
		{"request_id": "valid-id-123", "tier": "unsupported", "brief": "person"},
		{"request_id": "valid-id-123", "selection": map[string][]string{"missing": {"missing"}}},
		{"request_id": "valid-id-123", "brief": strings.Repeat("a", 4001)},
	} {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			recorder, _ := influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters", input)
			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		})
	}
	recorder, _ := influencerCLIAPIRequest(t, engine, http.MethodPost, "/videos", gin.H{"request_id": "video-invalid-duration", "duration_sec": 5})
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestInfluencerCLIReferenceRejectsUnownedAndExpiredManagedMaterialBeforeAcceptance(t *testing.T) {
	engine, _ := setupInfluencerCLIAPI(t)
	data := influencerPresetTestPNG(t, 16, 9)
	foreign := createInfluencerCLIImageUpload(t, 99, "uploads/foreign.png", data)
	expired := createInfluencerCLIImageUpload(t, 41, "uploads/expired.png", data)
	require.NoError(t, model.DB.Model(&model.File{}).Where("file_url = ?", expired).Update("expires_at", time.Now().Add(-time.Minute)).Error)
	for index, imageURL := range []string{"https://8.8.8.8/unowned.png", foreign, expired} {
		recorder, response := influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters", gin.H{"request_id": fmt.Sprintf("invalid-reference-%d", index), "image_url": imageURL})
		assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		assert.Equal(t, "invalid_reference_image", response["code"])
	}
	var tasks, receipts, frozen int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&tasks).Error)
	require.NoError(t, model.DB.Model(&model.InfluencerGenerationRequest{}).Count(&receipts).Error)
	require.NoError(t, model.DB.Model(&model.File{}).Where("upload_path = ?", "influencer-image/frozen").Count(&frozen).Error)
	assert.Zero(t, tasks)
	assert.Zero(t, receipts)
	assert.Zero(t, frozen)
	var owner model.User
	require.NoError(t, model.DB.First(&owner, 41).Error)
	assert.Equal(t, 100000, owner.Quota)
}

func TestInfluencerCLIReferenceFreezesExternalPublicImageWithoutClaimingSourceOwnership(t *testing.T) {
	engine, _, _ := setupInfluencerCLINativeRoutes(t)
	const sourceURL = "https://8.8.4.4/external-reference.png"
	data := influencerPresetTestPNG(t, 16, 9)
	oldClient := influencerReferenceHTTPClient
	var reads int
	influencerReferenceHTTPClient = &http.Client{Transport: referenceControllerFetchFunc(func(request *http.Request) (*http.Response, error) {
		reads++
		assert.Equal(t, sourceURL, request.URL.String())
		assert.Empty(t, request.Header.Get("Authorization"))
		assert.Empty(t, request.Header.Get("Cookie"))
		return (influencerImageTransport{data}).RoundTrip(request)
	})}
	t.Cleanup(func() { influencerReferenceHTTPClient = oldClient })
	started := time.Now()
	recorder, response := influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters", gin.H{"request_id": "public-reference-character", "image_url": sourceURL})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, 1, reads, "the validated original bytes are frozen after one bounded fetch")
	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ?", response["data"].(map[string]any)["task_id"]).First(&task).Error)
	require.NotNil(t, task.PrivateData.GenerationQueue.ImageInputs)
	inputs := task.PrivateData.GenerationQueue.ImageInputs
	assert.Equal(t, "reference", inputs.Mode)
	require.NotNil(t, inputs.Source)
	assert.NotEqual(t, sourceURL, inputs.Source.URL)
	assert.Equal(t, 16, inputs.Source.Width)
	assert.Equal(t, 9, inputs.Source.Height)
	assert.Greater(t, inputs.Source.ExpiresAt, started.Unix())
	assert.LessOrEqual(t, inputs.Source.ExpiresAt, started.Add(model.DefaultFileExpireDuration+time.Minute).Unix())
	frozen, err := service.ReadInfluencerFrozenImage(context.Background(), 41, *inputs.Source)
	require.NoError(t, err)
	assert.Equal(t, data, frozen)
	var files []model.File
	require.NoError(t, model.DB.Find(&files).Error)
	require.Len(t, files, 1)
	assert.Equal(t, 41, files[0].UserId, "only the new frozen copy belongs to this caller")
	assert.Equal(t, inputs.Source.URL, files[0].FileUrl)
	assert.NotEqual(t, sourceURL, files[0].FileUrl, "freezing a public image does not create an ownership record for the external URL")
	assert.Zero(t, task.Quota)
	var owner model.User
	require.NoError(t, model.DB.First(&owner, 41).Error)
	assert.Equal(t, 100000, owner.Quota)
}

func TestInfluencerCLIEditedImagesDoNotBecomeOriginalCharacters(t *testing.T) {
	engine, _ := setupInfluencerCLIAPI(t)
	for _, mode := range []string{"edit", "masked_edit"} {
		metadata, err := common.Marshal(gin.H{"image_mode": mode, "tier": "normal", "edit_instruction": "change the background"})
		require.NoError(t, err)
		task := model.Task{TaskID: "edited-" + mode, UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: string(metadata), OriginModelName: influencerImageModel}, PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.8.8/edited.png"}}
		require.NoError(t, model.DB.Create(&task).Error)
		recorder, response := influencerCLIAPIRequest(t, engine, http.MethodGet, "/characters/"+task.TaskID, nil)
		assert.Equal(t, http.StatusNotFound, recorder.Code)
		assert.Equal(t, "character_not_found", response["code"])
		recorder, response = influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters/"+task.TaskID+"/multiview", gin.H{"request_id": "not-a-character-" + mode})
		assert.Equal(t, http.StatusBadRequest, recorder.Code)
		assert.Equal(t, "invalid_parent_character", response["code"])
	}
	recorder, response := influencerCLIAPIRequest(t, engine, http.MethodGet, "/characters", nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	assert.Equal(t, float64(0), response["data"].(map[string]any)["total"])
}

func TestInfluencerCLIMultiViewUsesParentArtifactAndPreservesRelation(t *testing.T) {
	engine, _ := setupInfluencerCLIAPI(t)
	// A public literal avoids DNS/network in the real reference-image validator.
	system_setting.ServerAddress = "https://8.8.8.8"
	previousClient := influencerReferenceHTTPClient
	influencerReferenceHTTPClient = &http.Client{Transport: influencerImageTransport{influencerPresetTestPNG(t, 16, 9)}}
	t.Cleanup(func() { influencerReferenceHTTPClient = previousClient })
	ratios := ratio_setting.GetGroupRatioCopy()
	ratios[influencerImageGroup] = 0.5
	encodedRatios, err := common.Marshal(ratios)
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(string(encodedRatios)))
	channel := model.Channel{Id: 2, Name: "reference-only", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: influencerImageModel, Group: influencerImageGroup, OtherSettings: `{"image_task_contract":{"protocol":"openai_multipart","modes":["reference"],"sizes":["1024x576"]}}`}
	require.NoError(t, model.DB.Create(&channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{Group: influencerImageGroup, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true}).Error)
	parent := model.Task{TaskID: "parent-original", UserId: 41, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{Input: `{"tier":"dogs","selection":{},"brief":"red coat"}`, OriginModelName: "gpt-image-2.5"}, PrivateData: model.TaskPrivateData{ResultURL: "https://upstream.example/dog.png"}}
	require.NoError(t, model.DB.Create(&parent).Error)
	recorder, response := influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters/parent-original/multiview", gin.H{"request_id": "multiview-once-1"})
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	data := response["data"].(map[string]any)
	assert.Equal(t, "multiview", data["kind"])
	assert.Equal(t, parent.TaskID, data["parent_task_id"])
	var child model.Task
	require.NoError(t, model.DB.Where("task_id = ?", data["task_id"]).First(&child).Error)
	assert.Equal(t, influencerImageGroup, child.Group)
	assert.Equal(t, "/pg/images/edits", child.PrivateData.GenerationQueue.Path)
	assert.Contains(t, child.PrivateData.GenerationQueue.ReferencePreflight, "/influencer-image/frozen/")
	require.NotNil(t, child.PrivateData.GenerationQueue.ImageInputs)
	require.NotNil(t, child.PrivateData.GenerationQueue.ImageInputs.Source)
	assert.Equal(t, "reference", child.PrivateData.GenerationQueue.ImageInputs.Mode)
	var out map[string]any
	require.NoError(t, common.Unmarshal(child.PrivateData.GenerationQueue.Body, &out))
	assert.Equal(t, influencerImagePixels, out["size"])
	assert.Equal(t, influencerImageModel, out["model"])
	assert.Equal(t, "reference", out["image_mode"])
	assert.Equal(t, child.PrivateData.GenerationQueue.ImageInputs.Source.URL, out["image"])
	assert.Contains(t, strings.ToLower(out["prompt"].(string)), "dog")
	// No child-of-child, even when the child has finished and has an image.
	child.Status = model.TaskStatusSuccess
	child.PrivateData.ResultURL = "https://upstream.example/child.png"
	require.NoError(t, child.Update())
	recorder, _ = influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters/"+child.TaskID+"/multiview", gin.H{"request_id": "multiview-grandchild"})
	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	// The main character detail includes the child request identity, not just its task ID.
	recorder, response = influencerCLIAPIRequest(t, engine, http.MethodGet, "/characters/parent-original", nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	children := response["data"].(map[string]any)["multiviews"].([]any)
	require.Len(t, children, 1)
	assert.Equal(t, "multiview-once-1", children[0].(map[string]any)["request_id"])
}

func TestInfluencerCLIExpiredRequestRetainsIdentityAndCannotRebill(t *testing.T) {
	engine, _ := setupInfluencerCLIAPI(t)
	input := gin.H{"request_id": "expired-character-1", "brief": "a person"}
	recorder, response := influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters", input)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	taskID := response["data"].(map[string]any)["task_id"].(string)
	require.NoError(t, model.DB.Model(&model.Task{}).Where("task_id = ?", taskID).Updates(map[string]any{"status": model.TaskStatusFailure, "finish_time": time.Now().Add(-8 * 24 * time.Hour).Unix()}).Error)
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		path := "/requests/expired-character-1"
		if method == http.MethodPost {
			path = "/characters"
		}
		recorder, response = influencerCLIAPIRequest(t, engine, method, path, input)
		require.Equal(t, http.StatusGone, recorder.Code, recorder.Body.String())
		assert.Equal(t, "creation_request_expired", response["code"])
		assert.Equal(t, "expired-character-1", response["operation_id"])
		assert.Equal(t, taskID, response["task_id"])
	}
	var count int64
	require.NoError(t, model.DB.Model(&model.Task{}).Count(&count).Error)
	assert.Equal(t, int64(1), count)
}

func TestInfluencerCLICharacterBriefCountsChineseCharacters(t *testing.T) {
	engine, _ := setupInfluencerCLIAPI(t)
	for _, count := range []int{4000, 4001} {
		recorder, _ := influencerCLIAPIRequest(t, engine, http.MethodPost, "/characters", gin.H{"request_id": fmt.Sprintf("chinese-brief-%d", count), "brief": strings.Repeat("字", count)})
		if count == 4000 {
			assert.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
		} else {
			assert.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
		}
	}
}

func TestInfluencerCLIOptionsReflectAccountRoutesWithoutSupplierControls(t *testing.T) {
	engine, _ := setupInfluencerCLIAPI(t)
	t.Setenv("INFLUENCER_CLI_CREATION_ENABLED", "false")
	recorder, response := influencerCLIAPIRequest(t, engine, http.MethodGet, "/options", nil)
	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	data := response["data"].(map[string]any)
	assert.Equal(t, false, data["creation_enabled"])
	assert.Equal(t, true, data["availability"].(map[string]any)["character"])
	assert.Equal(t, false, data["availability"].(map[string]any)["reference_image"])
	assert.NotContains(t, recorder.Body.String(), "gpt-image-2.5-sunburst-test")
	assert.NotContains(t, recorder.Body.String(), "seedance-2-5-480p")
	assert.NotContains(t, recorder.Body.String(), `"group"`)
	assert.Equal(t, map[string]any{
		"1080p": map[string]any{
			"aspect_ratios": []any{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9"},
			"available":     true,
		},
	}, data["video"].(map[string]any)["resolutions"])
	assert.Contains(t, data["taxonomy"], "categories")
}

const (
	influencerFundingParentID = "issue89-parent"
	influencerFundedBalance   = 100_000
)

type influencerFundingFixture struct {
	engine   *gin.Engine
	owner    *model.User
	storage  *influencerCLIImageTestDriver
	imageURL string
	videoURL string
	metadata service.InfluencerVideoMetadata
}

func setupInfluencerFundingAPI(t *testing.T, ownerID int, cache *redis.Client) influencerFundingFixture {
	t.Helper()
	alias := fmt.Sprintf("issue89-%d", ownerID)
	owner := &model.User{Id: ownerID, Username: alias, AffCode: alias, Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Setting: `{"billing_preference":"wallet_only"}`}
	engine, _ := setupInfluencerCLIAPIWithOwner(t, owner)
	db := model.DB
	require.NoError(t, db.AutoMigrate(&model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{}))
	oldLogDB, oldMaster := model.LOG_DB, common.IsMasterNode
	t.Setenv("LOG_SQL_DSN", "")
	common.IsMasterNode = false
	initErr := model.InitLogDB() // Initializes the dialect's reserved column names for distribution.
	common.IsMasterNode = oldMaster
	t.Cleanup(func() { model.LOG_DB = oldLogDB })
	require.NoError(t, initErr)
	oldCache, oldRedis, oldMemory := common.RDB, common.RedisEnabled, common.MemoryCacheEnabled
	common.RDB, common.RedisEnabled, common.MemoryCacheEnabled = cache, cache != nil, false
	oldPrices := ratio_setting.ModelPrice2JSONString()
	fetch := system_setting.GetFetchSetting()
	oldFetch := *fetch
	oldClient, oldBrowser, oldInspector := influencerReferenceHTTPClient, service.InfluencerBrowserVideoPreparer, service.InfluencerVideoInspector
	oldResolver := net.DefaultResolver
	// The only network peers are numeric loopback listeners. The public-looking
	// material URLs are handled by the synthetic transport or the storage driver.
	net.DefaultResolver = &net.Resolver{PreferGo: true, Dial: func(context.Context, string, string) (net.Conn, error) {
		return nil, fmt.Errorf("unexpected DNS in creation fixture")
	}}
	t.Cleanup(func() {
		requireBackgroundJobsDrained(t)
		common.RDB, common.RedisEnabled, common.MemoryCacheEnabled = oldCache, oldRedis, oldMemory
		*fetch = oldFetch
		net.DefaultResolver = oldResolver
		influencerReferenceHTTPClient, service.InfluencerBrowserVideoPreparer, service.InfluencerVideoInspector = oldClient, oldBrowser, oldInspector
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(oldPrices))
	})
	withTieredBillingConfig(t, map[string]string{influencerImageModel: "tiered_expr", influencerMotionModel: "ratio"}, map[string]string{influencerImageModel: `tier("fixture", fixed(0.01)) * image_count`})
	prices, err := common.Marshal(map[string]float64{influencerMotionModel: 0.01})
	require.NoError(t, err)
	require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(string(prices)))
	require.NoError(t, i18n.Init())

	// The browser already supplied the declaration. The local response supplies
	// bytes for native staging and Discard without requiring an ffmpeg install.
	const clip = "synthetic reference bytes"
	metadata := service.InfluencerVideoMetadata{Duration: 5, VideoDuration: 5, Width: 720, Height: 1280, FrameRate: 24, VideoCodec: "h264", Container: "mp4", SizeBytes: int64(len(clip))}
	local := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/reference" {
			_, _ = io.WriteString(w, clip)
			return
		}
		t.Errorf("generation acceptance contacted the provider: %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusBadGateway)
	}))
	t.Cleanup(local.Close)
	fetch.EnableSSRFProtection = false // This fixture downloads exclusively from its local server.
	if service.GetHttpClient() == nil {
		service.InitHttpClient()
	}
	service.InfluencerBrowserVideoPreparer = func(ctx context.Context, _ string, userID int, declaration service.InfluencerVideoMetadata, valid func(string) bool) (service.PreparedInfluencerVideo, error) {
		return oldBrowser(ctx, local.URL+"/reference", userID, declaration, valid)
	}
	service.InfluencerVideoInspector = func(ctx context.Context, _ string, userID int, valid func(string) bool) (service.PreparedInfluencerVideo, error) {
		return oldBrowser(ctx, local.URL+"/reference", userID, metadata, valid)
	}
	influencerReferenceHTTPClient = &http.Client{Transport: influencerImageTransport{influencerPresetTestPNG(t, 16, 9)}}

	// Reuse the existing native Sunburst route for text and reference. Video
	// needs the APIMart adaptor, which validates the MAGI-2 duration and resolution.
	require.NoError(t, db.Model(&model.Channel{}).Where("id = ?", 1).Updates(model.Channel{
		Key: "fixture-key", BaseURL: &local.URL,
		OtherSettings: `{"image_task_contract":{"protocol":"openai_multipart","modes":["text","reference"],"sizes":["1024x576"]}}`,
	}).Error)
	video := model.Channel{Type: constant.ChannelTypeApiMart, Name: "local video", Key: "fixture-key", BaseURL: &local.URL, Status: common.ChannelStatusEnabled, Models: influencerMotionModel, Group: influencerMotionGroups["1080p"]}
	require.NoError(t, db.Create(&video).Error)
	require.NoError(t, db.Model(&model.Ability{}).Where("model = ?", influencerMotionModel).UpdateColumn("channel_id", video.Id).Error)

	parent := model.Task{TaskID: influencerFundingParentID, UserId: ownerID, Action: ActionInfluencerSheet, Status: model.TaskStatusSuccess, Properties: model.Properties{OriginModelName: influencerImageModel, Input: `{"tier":"normal","selection":{},"brief":"a portrait"}`}, PrivateData: model.TaskPrivateData{ResultURL: "https://example.test/parent.png"}}
	require.NoError(t, db.Create(&parent).Error)
	imageURL, err := service.BuildTaskArtifactContentURL(parent.TaskID, "image")
	require.NoError(t, err)
	driver, ok := storage.GetStorageDriver().(*influencerCLIImageTestDriver)
	require.True(t, ok)
	driver.publicOrigin = "https://files.example.test"
	videoURL := driver.GetPublicURL("uploads/reference.mp4")
	require.NoError(t, model.CreateFile(&model.File{FileId: "issue89-reference", UserId: ownerID, FileName: "reference.mp4", StorageKey: "uploads/reference.mp4", UploadPath: "uploads", FileUrl: videoURL, MimeType: "video/mp4", FileSize: metadata.SizeBytes, Status: model.FileStatusActive, ExpiresAt: time.Now().Add(time.Hour)}))

	web := engine.Group("/pg/influencer", func(c *gin.Context) {
		c.Set("id", owner.Id)
		c.Set("group", owner.Group)
		owner.ToBaseUser().WriteContext(c)
	})
	web.POST("/sheet", PrepareInfluencerSheet, middleware.Distribute(), PostInfluencerSheet)
	web.POST("/multiview", PrepareInfluencerMultiView, middleware.Distribute(), PostInfluencerMultiView)
	web.POST("/motion", PrepareInfluencerMotion, middleware.Distribute(), PostInfluencerMotion)
	return influencerFundingFixture{engine: engine, owner: owner, storage: driver, imageURL: imageURL, videoURL: videoURL, metadata: metadata}
}

func addInfluencerFundingCandidate(t *testing.T, modelName, group, mappedModel string, priority int64) {
	t.Helper()
	previousRetries := common.RetryTimes
	common.RetryTimes = 1 // Keep the second candidate within the dispatch budget.
	t.Cleanup(func() { common.RetryTimes = previousRetries })
	db := model.DB
	var alternate model.Channel
	require.NoError(t, db.Where("models = ?", modelName).First(&alternate).Error)
	alternate.Id, alternate.Name, alternate.Group = 0, "alternate "+modelName, group
	// Reuse the fixture listener so either candidate fails the test if it sends anything.
	alternate.BaseURL = common.GetPointer(alternate.GetBaseURL() + "/alternate")
	alternate.Setting = common.GetPointer(`{"http_protocol":"http1"}`)
	alternate.Priority = common.GetPointer(priority)
	if mappedModel != "" {
		alternate.ModelMapping = common.GetPointer(fmt.Sprintf(`{%q:%q}`, modelName, mappedModel))
	}
	require.NoError(t, db.Create(&alternate).Error)
	binding := model.Ability{Group: group, Model: modelName, ChannelId: alternate.Id, Enabled: true, Priority: alternate.Priority}
	require.NoError(t, db.Create(&binding).Error)
	t.Cleanup(func() { require.NoError(t, db.Model(&binding).UpdateColumn("enabled", false).Error) })
}

func (f influencerFundingFixture) setBalance(t *testing.T, quota int) {
	t.Helper()
	require.NoError(t, model.DB.Model(f.owner).UpdateColumn("quota", quota).Error)
	f.owner.Quota = quota
}

func (f influencerFundingFixture) motionInput(mode string) gin.H {
	return gin.H{"mode": mode, "resolution": "1080p", "video_url": f.videoURL, "video_metadata": f.metadata, "subject_image_url": "https://example.test/subject.png"}
}

func (f influencerFundingFixture) cliVideoInput() gin.H {
	return gin.H{"video_url": f.videoURL, "image_urls": []string{"https://example.test/subject.png"}}
}

func (f influencerFundingFixture) submissionCounts(t *testing.T) (tasks, receipts int64) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.Task{}).Where("user_id = ?", f.owner.Id).Count(&tasks).Error)
	require.NoError(t, model.DB.Model(&model.InfluencerGenerationRequest{}).Where("user_id = ?", f.owner.Id).Count(&receipts).Error)
	return tasks, receipts
}

// Exercise the public envelopes and the durable acceptance boundaries together.
func (f influencerFundingFixture) submit(t *testing.T, path string, input gin.H, wantStatus int) string {
	t.Helper()
	tasks, receipts := f.submissionCounts(t)
	recorder, response := influencerCreationAPIRequest(t, f.engine, http.MethodPost, path, input)
	assert.Equal(t, wantStatus, recorder.Code, recorder.Body.String())
	cli := strings.HasPrefix(path, "/api/influencer/v1/")
	switch wantStatus {
	case http.StatusForbidden:
		if cli {
			assert.JSONEq(t, `{"success":false,"code":"insufficient_user_quota","message":"Insufficient user quota"}`, recorder.Body.String())
		} else {
			assert.JSONEq(t, `{"error":{"message":"Insufficient user quota","type":"invalid_request_error","param":null,"code":"insufficient_user_quota"}}`, recorder.Body.String())
		}
	case http.StatusOK:
		tasks++
		if cli {
			receipts++
		}
	}
	gotTasks, gotReceipts := f.submissionCounts(t)
	assert.Equal(t, tasks, gotTasks, "task rows")
	assert.Equal(t, receipts, gotReceipts, "CLI receipt rows")
	var account model.User
	require.NoError(t, model.DB.First(&account, f.owner.Id).Error)
	assert.Equal(t, f.owner.Quota, account.Quota)
	assert.Zero(t, account.UsedQuota)
	if wantStatus != http.StatusOK || recorder.Code != http.StatusOK {
		return ""
	}
	data := response
	if cli {
		assert.Equal(t, true, response["success"])
		assert.NotContains(t, response, "error")
		data, _ = response["data"].(map[string]any)
		require.NotNil(t, data)
		assert.Equal(t, input["request_id"], data["request_id"])
		assert.Equal(t, false, data["replayed"])
		assert.Equal(t, "waiting", data["status"])
	} else {
		assert.NotContains(t, response, "error")
		assert.Equal(t, "queued", data["status"])
		assert.Equal(t, data["task_id"], data["id"])
	}
	id, _ := data["task_id"].(string)
	require.NotEmpty(t, id)
	var task model.Task
	require.NoError(t, model.DB.Where("task_id = ? AND user_id = ?", id, f.owner.Id).Take(&task).Error)
	assert.Equal(t, model.TaskStatusWaiting, task.Status)
	assert.Zero(t, task.Quota)
	return id
}

func TestInfluencerCreationFunding(t *testing.T) {
	f := setupInfluencerFundingAPI(t, 41, nil)
	routes := []struct {
		name string
		path string
		body gin.H
	}{
		{"web_sheet", "/pg/influencer/sheet", gin.H{"model": influencerImageModel, "group": influencerImageGroup, "tier": "normal", "brief": "a portrait"}},
		{"web_multiview", "/pg/influencer/multiview", gin.H{"model": influencerImageModel, "group": influencerImageGroup, "parent_task_id": influencerFundingParentID, "image_url": f.imageURL}},
		{"web_motion", "/pg/influencer/motion", f.motionInput("motion")},
		{"cli_character", "/api/influencer/v1/characters", gin.H{"brief": "a portrait"}},
		{"cli_multiview", "/api/influencer/v1/characters/" + influencerFundingParentID + "/multiview", gin.H{}},
		{"cli_video", "/api/influencer/v1/videos", f.cliVideoInput()},
	}
	for _, route := range routes {
		t.Run(route.name, func(t *testing.T) {
			cli := strings.HasPrefix(route.path, "/api/influencer/v1/")
			for _, quota := range []int{0, 1} {
				t.Run(fmt.Sprintf("wallet_%d", quota), func(t *testing.T) {
					f.setBalance(t, quota)
					if cli {
						route.body["request_id"] = fmt.Sprintf("issue89-%s-%d", route.name, quota)
					}
					deleted, objects := len(f.storage.deleted), len(f.storage.objects)
					f.submit(t, route.path, route.body, http.StatusForbidden)
					if route.name == "web_motion" {
						require.Len(t, f.storage.deleted, deleted+1, "the staged reference must be discarded")
						assert.NotContains(t, f.storage.objects, f.storage.deleted[deleted])
						assert.Len(t, f.storage.objects, objects)
						var rows int64
						require.NoError(t, model.DB.Unscoped().Model(&model.File{}).Where("storage_key = ?", f.storage.deleted[deleted]).Count(&rows).Error)
						assert.Zero(t, rows, "the staged reference record must also be deleted")
					}
				})
			}
			t.Run("funded", func(t *testing.T) {
				f.setBalance(t, influencerFundedBalance)
				if cli {
					route.body["request_id"] = "issue89-accepted-" + route.name
				}
				id := f.submit(t, route.path, route.body, http.StatusOK)
				if !cli {
					return
				}
				require.NotEmpty(t, id)
				f.setBalance(t, 0)
				tasks, receipts := f.submissionCounts(t)
				objects, discarded := len(f.storage.objects), len(f.storage.deleted)
				recorder, response := influencerCreationAPIRequest(t, f.engine, http.MethodPost, route.path, route.body)
				require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
				assert.Equal(t, true, response["success"])
				data, ok := response["data"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, route.body["request_id"], data["request_id"])
				assert.Equal(t, id, data["task_id"])
				assert.Equal(t, true, data["replayed"])
				gotTasks, gotReceipts := f.submissionCounts(t)
				assert.Equal(t, tasks, gotTasks)
				assert.Equal(t, receipts, gotReceipts)
				assert.Len(t, f.storage.objects, objects)
				assert.Len(t, f.storage.deleted, discarded, "replay must not stage media again")
			})
		})
	}

	t.Run("same_price_candidates", func(t *testing.T) {
		addInfluencerFundingCandidate(t, influencerImageModel, "gpt-image-2.5-sunburst-test", "", 0)
		addInfluencerFundingCandidate(t, influencerMotionModel, influencerMotionGroups["1080p"], "", 0)
		f.setBalance(t, 0)
		for _, route := range routes {
			t.Run(route.name, func(t *testing.T) {
				if strings.HasPrefix(route.path, "/api/influencer/v1/") {
					route.body["request_id"] = "issue92-same-price-" + route.name
				}
				f.submit(t, route.path, route.body, http.StatusForbidden)
			})
		}
	})

	t.Run("subscription_with_empty_wallet", func(t *testing.T) {
		f.setBalance(t, 0)
		f.owner.Setting = `{"billing_preference":"subscription_only"}`
		require.NoError(t, model.DB.Model(f.owner).UpdateColumn("setting", f.owner.Setting).Error)
		plan := model.SubscriptionPlan{Title: "funding fixture", TotalAmount: influencerFundedBalance}
		require.NoError(t, model.DB.Create(&plan).Error)
		now := common.GetTimestamp()
		subscription := model.UserSubscription{UserId: f.owner.Id, PlanId: plan.Id, AmountTotal: plan.TotalAmount, Status: "active", StartTime: now - 60, EndTime: now + 3600}
		require.NoError(t, model.DB.Create(&subscription).Error)
		f.submit(t, "/pg/influencer/sheet", gin.H{"model": influencerImageModel, "group": influencerImageGroup, "tier": "normal", "brief": "a portrait"}, http.StatusOK)
		f.submit(t, "/api/influencer/v1/characters", gin.H{"request_id": "issue89-subscription", "brief": "a portrait"}, http.StatusOK)
		require.NoError(t, model.DB.First(&subscription, subscription.Id).Error)
		assert.Zero(t, subscription.AmountUsed)
		var reservations int64
		require.NoError(t, model.DB.Model(&model.SubscriptionPreConsumeRecord{}).Count(&reservations).Error)
		assert.Zero(t, reservations)
	})
}

func TestInfluencerCreationFundingQuoteBoundaries(t *testing.T) {
	f := setupInfluencerFundingAPI(t, 41, nil)
	const motion = "/pg/influencer/motion"
	t.Run("resolution_group_and_duration", func(t *testing.T) {
		input := f.motionInput("motion")
		input["resolution"] = "1080p"
		// The fixture price is 5000 raw quota per second, with MAGI-2 tier ratio 1, 5s, and default group ratio 1.
		f.setBalance(t, 24_999)
		f.submit(t, motion, input, http.StatusForbidden)
		f.setBalance(t, 25_000)
		f.submit(t, motion, input, http.StatusOK)

		ratios := ratio_setting.GroupRatio2JSONString()
		t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(ratios)) })
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":2,"gpt-image-2.5-sunburst-image":1,"gpt-image-2.5-sunburst-test":1}`))
		f.setBalance(t, 49_999)
		f.submit(t, motion, input, http.StatusForbidden)
		f.setBalance(t, 50_000)
		f.submit(t, motion, input, http.StatusOK)
	})
	t.Run("free_model_with_empty_wallet", func(t *testing.T) {
		prices := ratio_setting.ModelPrice2JSONString()
		quotaSettings := operation_setting.GetQuotaSetting()
		previous := quotaSettings.EnableFreeModelPreConsume
		quotaSettings.EnableFreeModelPreConsume = false
		t.Cleanup(func() {
			quotaSettings.EnableFreeModelPreConsume = previous
			require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(prices))
		})
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(fmt.Sprintf(`{%q:0}`, influencerMotionModel)))
		f.setBalance(t, 0)
		f.submit(t, motion, f.motionInput("motion"), http.StatusOK)
	})
	for _, tc := range []struct {
		name, condition string
		status          int
	}{
		{"declared_recreation_inputs_are_priced", `param("duration") == 5 && param("resolution") == "1080p" && param("aspect_ratio") == "9:16"`, http.StatusForbidden},
		{"planned_prompt_price_is_deferred", `param("prompt") != ""`, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			expression := tc.condition + ` ? tier("request", fixed(0.01)) : tier("other", fixed(0.02))`
			withTieredBillingConfig(t, map[string]string{influencerMotionModel: "tiered_expr"}, map[string]string{influencerMotionModel: expression})
			f.setBalance(t, 4999)
			f.submit(t, motion, f.motionInput("recreate"), tc.status)
		})
	}
	t.Run("channel_preparation_does_not_rotate_keys_or_repair_settings", func(t *testing.T) {
		var channel model.Channel
		require.NoError(t, model.DB.Where("type = ?", constant.ChannelTypeApiMart).First(&channel).Error)
		require.NoError(t, model.DB.Model(&channel).Updates(map[string]any{
			"key": "fixture-a\nfixture-b", "setting": "{", "settings": "{",
			"channel_info": model.ChannelInfo{IsMultiKey: true, MultiKeySize: 2, MultiKeyMode: constant.MultiKeyModePolling},
		}).Error)
		f.setBalance(t, 0)
		f.submit(t, motion, f.motionInput("motion"), http.StatusForbidden)
		var stored model.Channel
		require.NoError(t, model.DB.First(&stored, channel.Id).Error)
		assert.Equal(t, "fixture-a\nfixture-b", stored.Key)
		assert.Zero(t, stored.ChannelInfo.MultiKeyPollingIndex)
		require.NotNil(t, stored.Setting)
		assert.Equal(t, "{", *stored.Setting)
		assert.Equal(t, "{", stored.OtherSettings)
	})
	t.Run("alias_projection_does_not_repair_settings", func(t *testing.T) {
		previous := jsplugin.DefaultRegistry
		jsplugin.DefaultRegistry = jsplugin.NewRegistry()
		t.Cleanup(func() { jsplugin.DefaultRegistry = previous })
		const source = `
export const meta = {apiVersion:1,key:"quote-alias",name:"Alias fixture",version:"1.0.0",author:{name:"Test"},models:["alias-target"],fetchMode:"per_task"};
export function buildSubmitRequest() { return {}; }
export function parseSubmitResponse() { return {taskId:"fixture-id"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status:"SUCCESS"}; }
`
		// Prime the empty generation so registration forces a rebuild in the quote.
		model.InitChannelCache()
		_, err := jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
		require.NoError(t, err)
		alias := model.Channel{Status: common.ChannelStatusEnabled, Type: constant.ChannelTypeTaskPlugin,
			Name: "fixture alias", Key: "fixture-alias-key", Models: "alias-model",
			ModelMapping: common.GetPointer(`{"alias-model":"alias-target"}`), Setting: common.GetPointer("{")}
		require.NoError(t, model.DB.Create(&alias).Error)
		f.setBalance(t, 0)
		f.submit(t, motion, f.motionInput("motion"), http.StatusForbidden)
		var stored model.Channel
		require.NoError(t, model.DB.First(&stored, alias.Id).Error)
		assert.Equal(t, alias.Key, stored.Key)
		assert.Equal(t, alias.Setting, stored.Setting)
	})
	t.Run("different_channel_rates_use_cheapest_quote", func(t *testing.T) {
		const cheaper, expensive = "fixture-cheaper-video", "fixture-expensive-video"
		group := influencerMotionGroups["1080p"]
		withTieredBillingConfig(t, map[string]string{influencerMotionModel: "ratio", cheaper: "tiered_expr", expensive: "tiered_expr"},
			map[string]string{cheaper: `tier("fixture", fixed(0.01))`, expensive: `tier("fixture", fixed(0.02))`})
		var primary model.Channel
		require.NoError(t, model.DB.Where("name = ?", "local video").First(&primary).Error)
		channel := model.DB.Model(&model.Channel{}).Where("id = ?", primary.Id)
		require.NoError(t, channel.UpdateColumn("model_mapping", fmt.Sprintf(`{%q:%q}`, influencerMotionModel, cheaper)).Error)
		t.Cleanup(func() { require.NoError(t, channel.UpdateColumn("model_mapping", primary.ModelMapping).Error) })
		// The expensive candidate sorts first in both the channel and ability lists.
		addInfluencerFundingCandidate(t, influencerMotionModel, group, expensive, 10)
		t.Run("neither_candidate_funded", func(t *testing.T) {
			f.setBalance(t, 4_999)
			f.submit(t, motion, f.motionInput("motion"), http.StatusForbidden)
		})
		t.Run("only_cheaper_candidate_funded", func(t *testing.T) {
			f.setBalance(t, 5_000)
			f.submit(t, motion, f.motionInput("motion"), http.StatusOK)
		})
		t.Run("expensive_candidate_alone", func(t *testing.T) {
			binding := model.Ability{Group: group, Model: influencerMotionModel, ChannelId: primary.Id}
			require.NoError(t, model.DB.Model(&binding).UpdateColumn("enabled", false).Error)
			t.Cleanup(func() { require.NoError(t, model.DB.Model(&binding).UpdateColumn("enabled", true).Error) })
			f.setBalance(t, 5_000)
			f.submit(t, motion, f.motionInput("motion"), http.StatusForbidden)
			f.setBalance(t, 10_000)
			f.submit(t, motion, f.motionInput("motion"), http.StatusOK)
		})
	})
	t.Run("auto_quotes_stay_within_the_frozen_groups", func(t *testing.T) {
		const mapped = "gpt-image-2.5-sunburst-1k"
		withTieredBillingConfig(t, map[string]string{influencerImageModel: "ratio", mapped: "tiered_expr"},
			map[string]string{mapped: `tier("fixture", fixed(0.01))`})
		prices := ratio_setting.ModelPrice2JSONString()
		t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(prices)) })
		require.NoError(t, ratio_setting.UpdateModelPriceByJSONString(fmt.Sprintf(`{%q:0.02}`, influencerImageModel)))
		addInfluencerFundingCandidate(t, influencerImageModel, influencerImageGroup, mapped, 0)

		f.setBalance(t, 5_000)
		const path = "/api/influencer/v1/characters"
		f.submit(t, path, gin.H{"request_id": "issue92-auto-affordable", "brief": "a portrait"}, http.StatusOK)

		groups := setting.AutoGroups2JsonString()
		t.Cleanup(func() { require.NoError(t, setting.UpdateAutoGroupsByJsonString(groups)) })
		require.NoError(t, setting.UpdateAutoGroupsByJsonString(`["gpt-image-2.5-sunburst-test"]`))
		f.submit(t, path, gin.H{"request_id": "issue92-auto-restricted", "brief": "a portrait"}, http.StatusForbidden)
	})
	t.Run("plugin_quote_uses_mapped_usage_and_ignores_caught_fetch_fallback", func(t *testing.T) {
		previous := jsplugin.DefaultRegistry
		jsplugin.DefaultRegistry = jsplugin.NewRegistry()
		t.Cleanup(func() { jsplugin.DefaultRegistry = previous })
		source := fmt.Sprintf(`
export const meta = {apiVersion:1,key:"sora",name:"Quote fixture",version:"1.0.0",author:{name:"Test"},models:[%q],fetchMode:"per_task",usageSchema:{units:{type:"number",unit:"count"}}};
export function buildSubmitRequest(ctx) {
  if (ctx.requestBody.duration === 6 && !ctx.baseUrl.endsWith("/alternate")) {
    try { utils.fetch({url: ctx.baseUrl + "/price-input"}); } catch (e) {}
  }
  return {url:ctx.baseUrl+"/submit",rewriteModel:"fixture-mapped-video",body:ctx.requestBody};
}
export function extractUsage(ctx) { return {units:ctx.upstreamModel === "fixture-mapped-video" ? 2 : 1}; }
export function parseSubmitResponse() { return {taskId:"fixture-id"}; }
export function buildQueryRequest() { return {}; }
export function parseTaskResult() { return {status:"SUCCESS"}; }
`, influencerMotionModel)
		_, err := jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
		require.NoError(t, err)
		var channel model.Channel
		require.NoError(t, model.DB.Where("type = ? AND name = ?", constant.ChannelTypeApiMart, "local video").First(&channel).Error)
		require.NoError(t, model.DB.Model(&channel).Update("type", constant.ChannelTypeSora).Error)
		withTieredBillingConfig(t, map[string]string{influencerMotionModel: "tiered_expr"}, map[string]string{influencerMotionModel: `u("units") * 0.01`})
		const path = "/api/influencer/v1/videos"
		input := gin.H{"mode": "recreate", "duration_sec": 5, "prompt": validRecreatePrompt(), "image_urls": []string{"https://example.test/subject.png"}}
		f.setBalance(t, 9999)
		input["request_id"] = "issue89-mapped-short"
		f.submit(t, path, input, http.StatusForbidden)
		f.setBalance(t, 10_000)
		input["request_id"] = "issue89-mapped-funded"
		f.submit(t, path, input, http.StatusOK)
		f.setBalance(t, 0)
		input["request_id"], input["duration_sec"] = "issue89-deferred-fetch", 6
		f.submit(t, path, input, http.StatusOK)

		group := influencerMotionGroups["1080p"]
		addInfluencerFundingCandidate(t, influencerMotionModel, group, "", 10)
		t.Run("known_unaffordable_and_unknown", func(t *testing.T) {
			input["request_id"] = "issue92-known-and-unknown"
			f.submit(t, path, input, http.StatusOK)
		})
		t.Run("known_unaffordable_alone", func(t *testing.T) {
			binding := model.Ability{Group: group, Model: influencerMotionModel, ChannelId: channel.Id}
			require.NoError(t, model.DB.Model(&binding).UpdateColumn("enabled", false).Error)
			t.Cleanup(func() { require.NoError(t, model.DB.Model(&binding).UpdateColumn("enabled", true).Error) })
			input["request_id"] = "issue92-known-alone"
			f.submit(t, path, input, http.StatusForbidden)
		})
	})
}

func TestInfluencerCreationFundingRechecksConcurrentAcceptance(t *testing.T) {
	f := setupInfluencerFundingAPI(t, 41, nil)
	f.setBalance(t, influencerFundedBalance)
	const path = "/api/influencer/v1/characters"
	input := gin.H{"request_id": "issue89-concurrent-replay", "brief": "a portrait"}
	body, err := common.Marshal(input)
	require.NoError(t, err)

	// Pause the first request on its channel read after its initial receipt miss.
	type pausedQuote struct{}
	arrived, proceed := make(chan struct{}), make(chan struct{})
	var paused atomic.Bool
	queries := model.DB.Callback().Query()
	const callback = "test:pause_influencer_quote"
	require.NoError(t, queries.After("gorm:query").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Table == "channels" && tx.Statement.Context.Value(pausedQuote{}) != nil && paused.CompareAndSwap(false, true) {
			close(arrived)
			select {
			case <-proceed:
			case <-tx.Statement.Context.Done():
			}
		}
	}))
	t.Cleanup(func() { require.NoError(t, queries.Remove(callback)) })
	ctx, cancel := context.WithTimeout(context.WithValue(t.Context(), pausedQuote{}, true), 10*time.Second)
	request := httptest.NewRequestWithContext(ctx, http.MethodPost, path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	first := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		defer close(done)
		f.engine.ServeHTTP(first, request)
	}()
	defer func() { cancel(); <-done }()
	select {
	case <-arrived:
	case <-ctx.Done():
		require.FailNow(t, "first request did not reach the quote")
	}
	id := f.submit(t, path, input, http.StatusOK)
	f.setBalance(t, 0)
	close(proceed)
	<-done
	require.Equal(t, http.StatusOK, first.Code, first.Body.String())
	var response struct {
		Data struct {
			Replayed bool   `json:"replayed"`
			TaskID   string `json:"task_id"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(first.Body.Bytes(), &response))
	assert.True(t, response.Data.Replayed)
	assert.Equal(t, id, response.Data.TaskID)
	tasks, receipts := f.submissionCounts(t)
	assert.Equal(t, int64(2), tasks, "one new task plus the fixture parent")
	assert.Equal(t, int64(1), receipts)
}

func TestInfluencerCreationFundingRedis(t *testing.T) {
	const redisEnv = "OMNIMUX_TEST_INFLUENCER_REDIS_ADDR"
	const childEnv = "OMNIMUX_TEST_INFLUENCER_REDIS_CHILD"
	addr := os.Getenv(redisEnv)
	if addr == "" {
		t.Skip("set " + redisEnv + " to a dedicated loopback Redis fixture")
	}
	// The analytics collector is initialized once per process. This test runs
	// in a fresh process so it can install its own local collector and site ID.
	if os.Getenv(childEnv) != "1" {
		binary, err := os.Executable()
		require.NoError(t, err)
		child := exec.CommandContext(t.Context(), binary, "-test.run=^TestInfluencerCreationFundingRedis$", "-test.v")
		child.Env = append(os.Environ(), childEnv+"=1")
		out, err := child.CombinedOutput()
		t.Log(string(out))
		require.NoError(t, err)
		return
	}
	host, port, err := net.SplitHostPort(addr)
	require.True(t, err == nil && net.ParseIP(host).IsLoopback(), "Redis fixture address must be a numeric loopback address")
	portNumber, err := strconv.Atoi(port)
	require.True(t, err == nil && portNumber >= 49152 && portNumber <= 65535, "Redis fixture must use a dedicated high port")
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { assert.NoError(t, client.Close()) })
	require.NoError(t, client.Ping(t.Context()).Err())

	type videoStart struct {
		Website string `json:"website"`
		Name    string `json:"name"`
		Data    struct {
			Entry  string `json:"entry"`
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	events := make(chan videoStart, 8)
	collector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var event struct {
			Payload videoStart `json:"payload"`
		}
		if assert.NoError(t, common.DecodeJson(r.Body, &event)) {
			select {
			case events <- event.Payload:
			default:
				t.Error("unexpected analytics events")
			}
		}
		_, _ = io.WriteString(w, `{"cache":"local-fixture"}`)
	}))
	t.Cleanup(collector.Close)
	site := uuid.NewString()
	t.Setenv("ANALYTICS_ENDPOINT", collector.URL)
	t.Setenv("UMAMI_WEBSITE_ID", site)
	t.Setenv("ANALYTICS_SITE_URL", "")
	t.Setenv("ANALYTICS_USER_KEY_SECRET", "issue89-local-analytics-fixture")
	t.Setenv("INFLUENCER_RECREATE_DAILY_ANALYSES", "1")

	for i, tc := range []struct {
		name, entry, mode string
		quota, code       int
		samePrice         bool
	}{
		{"accepted_web", "workbench", "motion", influencerFundedBalance, http.StatusOK, false},
		{"accepted_cli", "cli", "motion", influencerFundedBalance, http.StatusOK, false},
		{"rejected_web", "workbench", "motion", 0, http.StatusForbidden, false},
		{"rejected_cli", "cli", "motion", 0, http.StatusForbidden, false},
		{"rejected_cached_wallet", "cli", "motion", influencerFundedBalance, http.StatusForbidden, false},
		{"accepted_recreation", "workbench", "recreate", influencerFundedBalance, http.StatusOK, false},
		{"rejected_recreation", "workbench", "recreate", 0, http.StatusForbidden, false},
		{"same_price_accepted_recreation", "workbench", "recreate", influencerFundedBalance, http.StatusOK, true},
		{"same_price_rejected_recreation", "workbench", "recreate", 0, http.StatusForbidden, true},
		{"same_price_rejected_cli", "cli", "motion", 0, http.StatusForbidden, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// Isolate both the site-scoped milestone and Redis keys keyed only
			// by account (the daily budget and user cache).
			userID := 1_000_000 + 100*os.Getpid() + i
			key := fmt.Sprintf("analytics:e03:%s:%s:%d", site, analytics.EventFirstVideoStart, userID)
			day := time.Now()
			budgetKeys := []string{
				fmt.Sprintf("influencer:recreate:analysis:%d:%s", userID, day.Format("20060102")),
				fmt.Sprintf("influencer:recreate:analysis:%d:%s", userID, day.AddDate(0, 0, 1).Format("20060102")),
			}
			keys := append([]string{
				key,
				fmt.Sprintf("user:%d", userID),
				fmt.Sprintf("auth:user:version:%d", userID),
				fmt.Sprintf("auth:user:fence:%d", userID),
			}, budgetKeys...)
			count, err := client.Exists(t.Context(), keys...).Result()
			require.NoError(t, err)
			require.Zero(t, count, "Redis account must be isolated")
			t.Cleanup(func() {
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
				defer cancel()
				assert.NoError(t, client.Del(ctx, keys...).Err())
			})
			f := setupInfluencerFundingAPI(t, userID, client)
			if tc.samePrice {
				addInfluencerFundingCandidate(t, influencerMotionModel, influencerMotionGroups["1080p"], "", 0)
			}
			f.setBalance(t, tc.quota)
			cacheKey := fmt.Sprintf("user:%d", userID)
			if tc.name == "rejected_cached_wallet" {
				_, err := model.GetUserCache(userID)
				require.NoError(t, err)
				require.NoError(t, client.HSet(t.Context(), cacheKey, "Quota", 0).Err())
			}
			path, input := "/pg/influencer/motion", f.motionInput(tc.mode)
			if tc.entry == "cli" {
				path, input = "/api/influencer/v1/videos", f.cliVideoInput()
				input["request_id"] = "issue89-redis-" + tc.name
			}
			id := f.submit(t, path, input, tc.code)
			if tc.name == "rejected_cached_wallet" {
				quota, err := client.HGet(t.Context(), cacheKey, "Quota").Result()
				require.NoError(t, err)
				assert.Equal(t, "0", quota)
			} else {
				count, err := client.Exists(t.Context(), cacheKey).Result()
				require.NoError(t, err)
				assert.Zero(t, count, "a cache miss must stay read-only")
			}
			drainInfluencerFundingAnalytics(t)
			received := make([]videoStart, 0, len(events))
			for range len(events) {
				received = append(received, <-events)
			}
			claim, err := client.Get(t.Context(), key).Result()
			if tc.mode == "recreate" {
				balances, err := client.MGet(t.Context(), budgetKeys...).Result()
				require.NoError(t, err)
				if tc.code == http.StatusOK {
					assert.ElementsMatch(t, []any{"1", nil}, balances, "accepted recreation consumes one unit of analysis budget")
				} else {
					assert.Equal(t, []any{nil, nil}, balances, "a funding rejection must not consume the recreation analysis budget")
				}
			}
			if tc.code == http.StatusOK {
				require.NoError(t, err)
				assert.Equal(t, "1", claim)
				require.Len(t, received, 1)
				assert.Equal(t, site, received[0].Website)
				assert.Equal(t, analytics.EventFirstVideoStart, received[0].Name)
				assert.Equal(t, tc.entry, received[0].Data.Entry)
				assert.Equal(t, id, received[0].Data.TaskID)
			} else {
				assert.ErrorIs(t, err, redis.Nil, "a 403 must not leave a video milestone claim")
				assert.Empty(t, received, "a 403 must not send an analytics event")
				if tc.samePrice {
					remaining, err := client.Exists(t.Context(), keys...).Result()
					require.NoError(t, err)
					assert.Zero(t, remaining, "a funding rejection must leave the account and milestone keys absent")
				}
			}
		})
	}
}

// Place a FIFO barrier on both analytics workers after the handler returns.
// Once both arrive, all earlier milestone attempts and sends have finished.
func drainInfluencerFundingAnalytics(t *testing.T) {
	t.Helper()
	arrived := make(chan struct{}, 2)
	release := make(chan struct{})
	defer close(release)
	for range cap(arrived) {
		analytics.Enqueue(func(context.Context) (analytics.Event, bool) {
			arrived <- struct{}{}
			<-release
			return analytics.Event{}, false
		})
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	for range cap(arrived) {
		select {
		case <-arrived:
		case <-ctx.Done():
			require.FailNow(t, "analytics workers did not drain before the Redis check")
		}
	}
}
