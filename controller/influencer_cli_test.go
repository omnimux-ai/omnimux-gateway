package controller

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Public literal storage keeps ownership and frozen-byte validation real while
// every read remains inside the in-memory fixture.
type influencerCLIImageTestDriver struct {
	*influencerPresetTestDriver
}

func (d *influencerCLIImageTestDriver) GetPublicURL(key string) string {
	return "https://8.8.8.8/" + strings.TrimPrefix(key, "/")
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
	db, dialect := openInfluencerCLITestDatabase(t, &model.Task{}, &model.InfluencerGenerationRequest{}, &model.User{}, &model.UserAccessToken{}, &model.Ability{}, &model.Channel{}, &model.SystemTask{}, &model.SystemTaskLock{}, &model.File{})
	previousDB, previousMain, previousLog := model.DB, common.MainDatabaseType(), common.LogDatabaseType()
	previousStorage := storage.GetStorageDriver()
	previousGroups, previousRatios := setting.UserUsableGroups2JSONString(), ratio_setting.GroupRatio2JSONString()
	previousAuto := setting.AutoGroups2JsonString()
	previousAddress, previousPublic, previousSecret := system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret
	model.DB = db
	common.SetDatabaseTypes(dialect, dialect)
	storage.SetGlobalDriver(&influencerCLIImageTestDriver{newInfluencerPresetTestDriver()})
	system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret = "https://gateway.example", "", "cli-controller-test-only"
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"default","gpt-image-2.5-sunburst-image":"anchor","gpt-image-2.5-sunburst-test":"image","seedance-2-5-480p":"video","seedance-2-5-720p":"video"}`))
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"gpt-image-2.5-sunburst-image":1,"gpt-image-2.5-sunburst-test":1,"seedance-2-5-480p":1,"seedance-2-5-720p":2}`))
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
	user := model.User{Id: 41, Username: "cli-owner", AffCode: "cli-owner", Status: common.UserStatusEnabled, Role: common.RoleCommonUser, Group: "default", Quota: 100000}
	require.NoError(t, db.Create(&user).Error)
	token := &model.UserAccessToken{UserId: user.Id, TokenHash: "test-only-cli-token", ExpiresAt: time.Now().Add(time.Hour).Unix()}
	require.NoError(t, token.SetScopes([]string{"influencer_creation:read", "influencer_creation:write"}))
	require.NoError(t, db.Create(token).Error)
	channel := model.Channel{Id: 1, Name: "test-only", Type: constant.ChannelTypeOpenAI, Status: common.ChannelStatusEnabled, Models: influencerImageModel, Group: "gpt-image-2.5-sunburst-test", OtherSettings: `{"image_task_contract":{"protocol":"openai_multipart","modes":["text"],"sizes":["1024x576"]}}`}
	require.NoError(t, db.Create(&channel).Error)
	for _, binding := range []model.Ability{
		{Group: channel.Group, Model: influencerImageModel, ChannelId: channel.Id, Enabled: true},
		{Group: "seedance-2-5-480p", Model: influencerMotionModel, ChannelId: channel.Id, Enabled: true},
		{Group: "seedance-2-5-720p", Model: influencerMotionModel, ChannelId: channel.Id, Enabled: true},
	} {
		require.NoError(t, db.Create(&binding).Error)
	}
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	group := engine.Group("/api/influencer/v1", func(c *gin.Context) {
		c.Set("id", user.Id)
		c.Set("group", user.Group)
		common.SetContextKey(c, constant.ContextKeyUserGroup, user.Group)
		c.Set("use_access_token", true)
		c.Set("access_token_id", token.Id)
		c.Set("access_token_scopes", []string{"influencer_creation:read", "influencer_creation:write"})
	})
	RegisterInfluencerCLIRoutes(group)
	return engine, token
}

func influencerCLIAPIRequest(t *testing.T, engine *gin.Engine, method, path string, input any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	body, err := common.Marshal(input)
	require.NoError(t, err)
	request := httptest.NewRequest(method, "/api/influencer/v1"+path, bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-Omnimux-Group", "caller-cannot-choose")
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
			assert.Equal(t, "5", out["seconds"])
			assert.Equal(t, "480p", out["resolution"])
			assert.Equal(t, aspectRatio, out["aspect_ratio"])
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
			images := out["images"].([]any)
			require.Len(t, images, 3)
			assert.Contains(t, images[0], "/v1/tasks/cat-main/artifacts/image/content?access=")
			assert.Equal(t, motionTestImage, images[1])
			assert.Equal(t, "https://files.omnimux.ai/images/outfit.png", images[2])
			assert.Equal(t, []any{motionTestFrozenVideo}, out["videos"])
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
		{"motion follows source", "motion", "480p", "", "16:9", 1280, 720, 200},
		{"motion defaults portrait for arbitrary source", "motion", "480p", "", "9:16", 1000, 1000, 200},
		{"replace follows source", "replace", "720p", "", "16:9", 1280, 720, 200},
		{"replace rejects conflicting ratio", "replace", "480p", "9:16", "", 1280, 720, 400},
		{"replace rejects unsupported source", "replace", "480p", "", "", 1000, 1000, 400},
		{"720 rejects 3x4", "motion", "720p", "3:4", "", 960, 1280, 400},
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
			assert.Equal(t, "4", out["seconds"])
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
	assert.Contains(t, data["taxonomy"], "categories")
}
