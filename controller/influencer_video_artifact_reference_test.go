package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupInfluencerArtifactReferenceTest(t *testing.T) *influencerPresetTestDriver {
	t.Helper()
	db := referenceInspectionTestDB(t)
	previousDriver := storage.GetStorageDriver()
	previousSecret, previousRedis := common.CryptoSecret, common.RedisEnabled
	previousAddress, previousPublic := system_setting.ServerAddress, system_setting.TaskPublicAddress
	previousTrusted := common.SessionCookieTrustedURLs
	previousFetch := *system_setting.GetFetchSetting()
	previousInspector := influencerVideoReferenceInspector
	t.Cleanup(func() {
		storage.SetGlobalDriver(previousDriver)
		common.CryptoSecret, common.RedisEnabled = previousSecret, previousRedis
		system_setting.ServerAddress, system_setting.TaskPublicAddress = previousAddress, previousPublic
		common.SessionCookieTrustedURLs = previousTrusted
		*system_setting.GetFetchSetting() = previousFetch
		influencerVideoReferenceInspector = previousInspector
	})
	common.CryptoSecret, common.RedisEnabled = "history-reference-capability-secret", false
	system_setting.ServerAddress, system_setting.TaskPublicAddress = "https://app.example/prefix", ""
	common.SessionCookieTrustedURLs = nil
	// No network is used: the fake inspector checks the resolved source, and
	// TaskArtifactContent reads the real in-memory persisted artifact below.
	system_setting.GetFetchSetting().ApplyIPFilterForDomain = false
	driver := newInfluencerPresetTestDriver()
	storage.SetGlobalDriver(driver)
	require.NoError(t, db.Create(&model.User{Id: 41, Username: "history-reference-owner", Status: common.UserStatusEnabled}).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID: "own-history-video", UserId: 41, Action: ActionInfluencerMotion, Status: model.TaskStatusSuccess,
		Properties:  model.Properties{Input: `{"video_url":"https://8.8.8.8/original.mp4"}`, OriginModelName: "seedance-2-5"},
		PrivateData: model.TaskPrivateData{ResultURL: "https://8.8.8.8/generated.mp4"},
	}).Error)
	return driver
}

func TestInspectInfluencerMotionVideoResolvesOwnedGeneratedArtifactCapability(t *testing.T) {
	for _, tc := range []struct {
		name, publicAddress string
		plugin              bool
	}{
		{"legacy-application-origin", "", false},
		{"legacy-isolated-artifact-origin", "https://artifacts.example/prefix", false},
		{"plugin-application-origin", "", true},
		{"plugin-isolated-artifact-origin", "https://artifacts.example/prefix", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			driver := setupInfluencerArtifactReferenceTest(t)
			system_setting.TaskPublicAddress = tc.publicAddress
			task, exists, err := model.GetByTaskId(41, "own-history-video")
			require.NoError(t, err)
			require.True(t, exists)
			if tc.plugin {
				task.Platform = constant.TaskPlatform("sora")
				task.PrivateData.Execution = &model.TaskExecutionSnapshot{TaskPlugin: &model.TaskPluginSnapshot{Key: "sora", Name: "Sora", Version: "1.0.0", APIVersion: 1}}
				task.SetData(map[string]any{"status": "completed", "video_url": "https://8.8.8.8/generated.mp4"})
				require.NoError(t, model.DB.Save(task).Error)
			}
			ref, err := service.PersistTaskArtifact(context.Background(), task, "video", "video/mp4", []byte("owned stored video bytes"))
			require.NoError(t, err)
			capability, err := service.BuildTaskArtifactContentURL(task.TaskID, "video")
			require.NoError(t, err)
			parsed, err := url.Parse(capability)
			require.NoError(t, err)
			require.NotEmpty(t, parsed.Query().Get(service.TaskArtifactAccessQueryParameter))

			engine := gin.New()
			engine.GET("/prefix/v1/tasks/:key/artifacts/:artifact_key/content", middleware.TokenOrTaskArtifactAccessAuth("key", "artifact_key"), TaskArtifactContent)
			engine.POST("/inspect", func(c *gin.Context) { c.Set("id", 41) }, InspectInfluencerMotionVideo)
			// The actual producer's capability must also resolve through the
			// native artifact authorization and storage boundary.
			content := httptest.NewRecorder()
			engine.ServeHTTP(content, httptest.NewRequest(http.MethodGet, capability, nil))
			require.Equal(t, http.StatusOK, content.Code, content.Body.String())
			assert.Equal(t, "owned stored video bytes", content.Body.String())

			calls := 0
			influencerVideoReferenceInspector = func(_ context.Context, source, mode string) (service.InfluencerVideoMetadata, error) {
				calls++
				assert.Equal(t, driver.GetPublicURL(ref.ObjectKey), source, "the capability resolves the exact owned stored artifact")
				assert.Equal(t, "motion", mode)
				reader, mime, _, err := driver.Get(context.Background(), ref.ObjectKey)
				require.NoError(t, err)
				defer reader.Close()
				bytes, err := io.ReadAll(reader)
				require.NoError(t, err)
				assert.Equal(t, "video/mp4", mime)
				assert.Equal(t, "owned stored video bytes", string(bytes))
				return service.InfluencerVideoMetadata{Duration: 5, Width: 800, Height: 640, FrameRate: 24, Container: "mp4", VideoCodec: "h264"}, nil
			}
			payload, err := common.Marshal(map[string]any{"video_url": capability, "mode": "motion"})
			require.NoError(t, err)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/inspect", strings.NewReader(string(payload)))
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(response, request)
			require.Equal(t, http.StatusOK, response.Code, response.Body.String())
			assert.Equal(t, 1, calls)
			var files int64
			require.NoError(t, model.DB.Model(&model.File{}).Count(&files).Error)
			assert.Zero(t, files, "task artifacts do not need an uploaded-file row")
			assert.Len(t, driver.objects, 1, "preflight does not create another stored object")
		})
	}
}

func TestInspectInfluencerMotionVideoKeepsUnpersistedHistoryOnNativeArtifactBoundary(t *testing.T) {
	setupInfluencerArtifactReferenceTest(t)
	capability, err := service.BuildTaskArtifactContentURL("own-history-video", "video")
	require.NoError(t, err)
	calls := 0
	influencerVideoReferenceInspector = func(_ context.Context, source, _ string) (service.InfluencerVideoMetadata, error) {
		calls++
		assert.Equal(t, capability, source, "unpersisted output must use the validated native artifact route, not a caller URL")
		return service.InfluencerVideoMetadata{Duration: 5, Width: 800, Height: 640}, nil
	}
	engine := gin.New()
	engine.POST("/inspect", func(c *gin.Context) { c.Set("id", 41) }, InspectInfluencerMotionVideo)
	payload, err := common.Marshal(map[string]any{"video_url": capability, "mode": "motion"})
	require.NoError(t, err)
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/inspect", strings.NewReader(string(payload)))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	assert.Equal(t, 1, calls)
}

func TestInspectInfluencerMotionVideoRejectsUnauthorizedOrUnavailableHistoryArtifacts(t *testing.T) {
	for _, name := range []string{
		"foreign-owner", "discarded-result", "not-completed", "wrong-artifact-key", "missing-task", "tampered-signature", "retired-signature",
		"foreign-origin", "extra-query", "same-origin-non-artifact", "disabled-owner", "ambiguous-task-id",
	} {
		t.Run(name, func(t *testing.T) {
			setupInfluencerArtifactReferenceTest(t)
			capability, err := service.BuildTaskArtifactContentURL("own-history-video", "video")
			require.NoError(t, err)
			switch name {
			case "foreign-owner":
				require.NoError(t, model.DB.Model(&model.Task{}).Where("task_id = ?", "own-history-video").Update("user_id", 42).Error)
			case "discarded-result":
				task, exists, err := model.GetByTaskId(41, "own-history-video")
				require.NoError(t, err)
				require.True(t, exists)
				task.PrivateData.ResultDiscarded = true
				require.NoError(t, model.DB.Save(task).Error)
			case "not-completed":
				require.NoError(t, model.DB.Model(&model.Task{}).Where("task_id = ?", "own-history-video").Update("status", model.TaskStatusInProgress).Error)
			case "wrong-artifact-key":
				capability, err = service.BuildTaskArtifactContentURL("own-history-video", "image")
				require.NoError(t, err)
			case "missing-task":
				capability, err = service.BuildTaskArtifactContentURL("missing-history-video", "video")
				require.NoError(t, err)
			case "tampered-signature":
				capability += "x"
			case "retired-signature":
				common.CryptoSecret = "new-history-reference-capability-secret"
			case "foreign-origin":
				capability = strings.Replace(capability, "https://app.example", "https://evil.example", 1)
			case "extra-query":
				capability += "&next=https://evil.example/video.mp4"
			case "same-origin-non-artifact":
				capability = "https://app.example/prefix/api/user/self"
			case "disabled-owner":
				require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", 41).Update("status", common.UserStatusDisabled).Error)
			case "ambiguous-task-id":
				require.NoError(t, model.DB.Create(&model.Task{TaskID: "own-history-video", UserId: 42, Action: ActionInfluencerMotion, Status: model.TaskStatusSuccess}).Error)
			}
			influencerVideoReferenceInspector = func(context.Context, string, string) (service.InfluencerVideoMetadata, error) {
				t.Fatal("rejected history references must not start a media download")
				return service.InfluencerVideoMetadata{}, nil
			}
			engine := gin.New()
			engine.POST("/inspect", func(c *gin.Context) { c.Set("id", 41) }, InspectInfluencerMotionVideo)
			payload, err := common.Marshal(map[string]any{"video_url": capability, "mode": "motion"})
			require.NoError(t, err)
			response := httptest.NewRecorder()
			request := httptest.NewRequest(http.MethodPost, "/inspect", strings.NewReader(string(payload)))
			request.Header.Set("Content-Type", "application/json")
			engine.ServeHTTP(response, request)
			assert.Contains(t, []int{http.StatusBadRequest, http.StatusForbidden}, response.Code, response.Body.String())
		})
	}
}
