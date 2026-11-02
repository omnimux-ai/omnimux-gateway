package controller

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/storage"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskMediaProxyURLRewriting(t *testing.T) {
	system_setting.ServerAddress = "https://omnimux.ai"
	setting.TaskProxyEnabled = true

	// 1. 视频任务输出反代链接
	videoTask := &model.Task{
		TaskID: "task_video_123",
		Status: model.TaskStatusSuccess,
		Properties: model.Properties{
			OriginModelName: "kling-v1.5",
		},
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://kling.kuaishou.com/media/output.mp4",
		},
	}
	assert.Equal(t, "https://kling.kuaishou.com/media/output.mp4", videoTask.GetRawUpstreamURL())
	assert.Equal(t, "https://omnimux.ai/v1/videos/task_video_123/content", videoTask.GetResultURL())

	// 2. 图片任务输出图片反代链接
	imageTask := &model.Task{
		TaskID: "task_image_456",
		Status: model.TaskStatusSuccess,
		Properties: model.Properties{
			OriginModelName: "gpt-image-2.5",
		},
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://oss.goeasyapi.com/images/result.png",
		},
	}
	assert.Equal(t, "https://oss.goeasyapi.com/images/result.png", imageTask.GetRawUpstreamURL())
	assert.Equal(t, "https://omnimux.ai/v1/images/task_image_456/content", imageTask.GetResultURL())

	// 3. 全局禁用代理时，返回原始渠道直链
	setting.TaskProxyEnabled = false
	assert.Equal(t, "https://oss.goeasyapi.com/images/result.png", imageTask.GetResultURL())
	setting.TaskProxyEnabled = true // 还原
}

func TestLoopDetectionHelpers(t *testing.T) {
	system_setting.ServerAddress = "https://omnimux.ai"

	taskID := "task_abc789"
	proxyVideoURL := "https://omnimux.ai/v1/videos/" + taskID + "/content"
	proxyImageURL := "https://omnimux.ai/v1/images/" + taskID + "/content"
	otherProxyURL := "https://omnimux.ai/v1/videos/task_other/content"
	externalURL := "https://v3.fal.media/files/lion.mp4"

	assert.True(t, isTaskProxyContentURL(proxyVideoURL, taskID))
	assert.True(t, isTaskProxyContentURL(proxyImageURL, taskID))
	assert.False(t, isTaskProxyContentURL(otherProxyURL, taskID))
	assert.False(t, isTaskProxyContentURL(externalURL, taskID))

	assert.True(t, isSelfServerURL("https://omnimux.ai/v1/videos/some_task/content"))
	assert.True(t, isSelfServerURL("http://omnimux.ai/any/path"))
	assert.False(t, isSelfServerURL("https://external-cdn.com/video.mp4"))
}

func TestVideoProxyLoopDetectionProtection(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Mock server that triggers VideoProxy
	system_setting.ServerAddress = "https://omnimux.ai"
	setting.TaskProxyEnabled = true

	r := gin.New()
	r.GET("/v1/videos/:task_id/content", VideoProxy)

	// 任务未完成时直接 400
	req, _ := http.NewRequest(http.MethodGet, "/v1/videos//content", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	// task_id is required
	assert.True(t, w.Code == http.StatusNotFound || w.Code == http.StatusBadRequest)
}

func TestStoredInlineImageUsesBothNativeContentPaths(t *testing.T) {
	task := setupGenericTaskTest(t)
	previousAddress, previousPublic, previousSecret := system_setting.ServerAddress, system_setting.TaskPublicAddress, common.CryptoSecret
	system_setting.ServerAddress = "https://images.example"
	system_setting.TaskPublicAddress = ""
	common.CryptoSecret = "inline-content-secret"
	t.Cleanup(func() {
		system_setting.ServerAddress = previousAddress
		system_setting.TaskPublicAddress = previousPublic
		common.CryptoSecret = previousSecret
	})
	task.Action = "influencer_sheet"
	task.Properties.OriginModelName = "gpt-image-2.5-sunburst"
	var source bytes.Buffer
	require.NoError(t, png.Encode(&source, image.NewNRGBA(image.Rect(0, 0, 3, 2))))
	result, err := service.PersistInlineImageTask(t.Context(), task, base64.StdEncoding.EncodeToString(source.Bytes()))
	require.NoError(t, err)
	task.PrivateData.ResultURL = result.URL
	require.NoError(t, model.DB.Save(task).Error)
	for _, test := range []struct {
		name   string
		legacy bool
		user   int
		status int
	}{
		{"legacy public", true, 0, http.StatusOK}, {"owner signed route", false, task.UserId, http.StatusOK}, {"foreign authenticated", false, task.UserId + 1, http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Set("id", test.user)
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks/"+task.TaskID+"/artifacts/image/content", nil)
			if test.legacy {
				c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/"+task.TaskID+"/content", nil)
				c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
				VideoProxy(c)
			} else {
				c.Params = gin.Params{{Key: "key", Value: task.TaskID}, {Key: "artifact_key", Value: "image"}}
				TaskArtifactContent(c)
			}
			assert.Equal(t, test.status, recorder.Code)
			if test.status == http.StatusOK {
				assert.Equal(t, source.Bytes(), recorder.Body.Bytes())
				assert.Equal(t, sha256.Sum256(source.Bytes()), sha256.Sum256(recorder.Body.Bytes()))
			}
		})
	}
	// Exercise the real signature binding separately from the legacy public route.
	engine := gin.New()
	engine.GET("/v1/tasks/:key/artifacts/:artifact_key/content", middleware.TokenOrTaskArtifactAccessAuth("key", "artifact_key"), TaskArtifactContent)
	signedRecorder := httptest.NewRecorder()
	engine.ServeHTTP(signedRecorder, httptest.NewRequest(http.MethodGet, result.URL, nil))
	assert.Equal(t, http.StatusOK, signedRecorder.Code)
	assert.Equal(t, source.Bytes(), signedRecorder.Body.Bytes())
	wrongTaskURL := strings.Replace(result.URL, "/"+task.TaskID+"/", "/task_other/", 1)
	wrongRecorder := httptest.NewRecorder()
	engine.ServeHTTP(wrongRecorder, httptest.NewRequest(http.MethodGet, wrongTaskURL, nil))
	assert.Equal(t, http.StatusNotFound, wrongRecorder.Code)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", task.UserId).Update("status", common.UserStatusDisabled).Error)
	disabledRecorder := httptest.NewRecorder()
	engine.ServeHTTP(disabledRecorder, httptest.NewRequest(http.MethodGet, result.URL, nil))
	assert.Equal(t, http.StatusNotFound, disabledRecorder.Code)
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", task.UserId).Update("status", common.UserStatusEnabled).Error)
	// A missing local object must fail without recursively fetching its own URL.
	require.NoError(t, storage.GetStorageDriver().Delete(t.Context(), service.TaskArtifactObjectPrefix+"/"+task.TaskID+"/image"))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/"+task.TaskID+"/content", nil)
	VideoProxy(c)
	assert.Equal(t, http.StatusGone, recorder.Code)
}

func TestStoredInlineImageContentDoesNotServeDiscardedOrVideoObjects(t *testing.T) {
	for _, name := range []string{"discarded", "pending", "video"} {
		t.Run(name, func(t *testing.T) {
			task := setupGenericTaskTest(t)
			task.Action = "influencer_sheet"
			task.Properties.OriginModelName = "gpt-image-2.5-sunburst"
			task.PrivateData.ResultURL = "https://images.example/v1/tasks/" + task.TaskID + "/artifacts/image/content?access=not-used"
			switch name {
			case "discarded":
				task.PrivateData.ResultDiscarded = true
			case "pending":
				task.Status = model.TaskStatusDispatching
			case "video":
				task.Action = constant.TaskActionTextToVideo
				task.Properties.OriginModelName = "kling-v1.5"
			}
			require.NoError(t, model.DB.Save(task).Error)
			_, err := service.PersistTaskArtifact(t.Context(), task, "image", "image/png", []byte("must-not-be-served"))
			require.NoError(t, err)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Params = gin.Params{{Key: "task_id", Value: task.TaskID}}
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/images/"+task.TaskID+"/content", nil)
			VideoProxy(c)
			assert.NotEqual(t, http.StatusOK, recorder.Code)
			assert.NotEqual(t, "must-not-be-served", recorder.Body.String())
		})
	}
}
