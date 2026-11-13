package controller

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fork: generated motion videos record their own media facts once, from the
// bytes the artifact path already buffers, so the workbench never has to
// re-read the remote video before reusing it as a reference.

func outputMetadataTestVideo(t *testing.T) []byte {
	t.Helper()
	_, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("ffmpeg required for real media fixture")
	}
	path := filepath.Join(t.TempDir(), "generated-output.mp4")
	output, err := exec.Command("ffmpeg", "-nostdin", "-v", "error", "-f", "lavfi", "-i", "color=c=black:s=720x1280:r=24", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100", "-t", "4", "-c:v", "libx264", "-threads", "1", "-c:a", "aac", path).CombinedOutput()
	require.NoError(t, err, string(output))
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func outputMetadataTestTask(t *testing.T, action string) *model.Task {
	t.Helper()
	task := setupGenericTaskTest(t)
	task.Action = action
	require.NoError(t, model.DB.Save(task).Error)
	return task
}

func reloadTask(t *testing.T, task *model.Task) *model.Task {
	t.Helper()
	var stored model.Task
	require.NoError(t, model.DB.First(&stored, task.ID).Error)
	return &stored
}

func TestInfluencerMotionOutputMetadataPersistsFromBufferedBytes(t *testing.T) {
	video := outputMetadataTestVideo(t)
	task := outputMetadataTestTask(t, ActionInfluencerMotion)

	persistTaskOutputVideoMetadata(context.Background(), task, video)

	stored := reloadTask(t, task)
	require.NotEmpty(t, stored.OutputVideoMetadata)
	var metadata dto.InfluencerVideoMetadata
	require.NoError(t, common.Unmarshal(stored.OutputVideoMetadata, &metadata))
	assert.Equal(t, 720, metadata.Width)
	assert.Equal(t, 1280, metadata.Height)
	assert.Equal(t, 24.0, metadata.FrameRate)
	assert.Equal(t, "h264", metadata.VideoCodec)
	assert.Equal(t, "aac", metadata.AudioCodec)
	assert.Equal(t, "mp4", metadata.Container)
	assert.Equal(t, int64(len(video)), metadata.SizeBytes)
	assert.Greater(t, metadata.Duration, 0.0)
}

func TestOutputMetadataProbeFailureLeavesTaskSuccessUnchanged(t *testing.T) {
	task := outputMetadataTestTask(t, ActionInfluencerMotion)

	persistTaskOutputVideoMetadata(context.Background(), task, []byte("not-a-video"))

	stored := reloadTask(t, task)
	assert.Equal(t, string(model.TaskStatusSuccess), string(stored.Status))
	assert.Equal(t, "100%", stored.Progress)
	assert.Equal(t, int64(20), stored.FinishTime)
	assert.Empty(t, stored.OutputVideoMetadata, "a failed probe must leave the column NULL")
}

func TestOutputMetadataIsNotProbedForOtherActions(t *testing.T) {
	task := outputMetadataTestTask(t, ActionInfluencerSheet)

	// A workbench image task shares the video preheat path; it must not spend a
	// probe on non-motion tasks even when the bytes are unprobeable.
	persistTaskOutputVideoMetadata(context.Background(), task, []byte("not-a-video"))

	assert.Empty(t, reloadTask(t, task).OutputVideoMetadata)
}

func outputMetadataProjectionTask(t *testing.T, action string, data map[string]any) *model.Task {
	t.Helper()
	task := setupGenericTaskTest(t)
	task.Action = action
	task.Platform = constant.TaskPlatform("sora")
	task.PrivateData.Execution = &model.TaskExecutionSnapshot{TaskPlugin: &model.TaskPluginSnapshot{Key: "sora", Version: "1.1.0", APIVersion: 1}}
	task.SetData(data)
	require.NoError(t, model.DB.Save(task).Error)
	previousSecret, previousAddress := common.CryptoSecret, system_setting.TaskPublicAddress
	common.CryptoSecret = "output-metadata-test-secret"
	system_setting.TaskPublicAddress = "https://gateway.example"
	t.Cleanup(func() { common.CryptoSecret, system_setting.TaskPublicAddress = previousSecret, previousAddress })
	return task
}

func storeOutputMetadata(t *testing.T, task *model.Task) {
	t.Helper()
	raw, err := common.Marshal(dto.InfluencerVideoMetadata{
		Duration: 4, VideoDuration: 4, Width: 720, Height: 1280, FrameRate: 24,
		VideoCodec: "h264", AudioCodec: "aac", Container: "mp4", SizeBytes: 4096,
	})
	require.NoError(t, err)
	require.NoError(t, model.UpdateTaskOutputVideoMetadata(task.ID, raw))
}

func getUserTaskArtifacts(t *testing.T, task *model.Task) (*httptest.ResponseRecorder, []*dto.TaskDto) {
	t.Helper()
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set("id", task.UserId)
	c.Set("role", common.RoleCommonUser)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/task/self?p=1&page_size=10&with_artifacts=true", nil)
	GetUserTask(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool `json:"success"`
		Data    struct {
			Items []*dto.TaskDto `json:"items"`
		}
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Len(t, response.Data.Items, 1)
	return recorder, response.Data.Items
}

func TestTaskArtifactProjectsStoredOutputVideoMetadata(t *testing.T) {
	task := outputMetadataProjectionTask(t, ActionInfluencerMotion, map[string]any{
		"object": "video", "status": "completed", "url": "https://private.example/output.mp4?signature=private",
	})
	storeOutputMetadata(t, task)

	recorder, items := getUserTaskArtifacts(t, task)

	item := items[0]
	require.Len(t, item.Artifacts, 1)
	artifact := item.Artifacts[0]
	require.Equal(t, "video", artifact.Type)
	require.NotNil(t, artifact.VideoMetadata)
	assert.Equal(t, 720, artifact.VideoMetadata.Width)
	assert.Equal(t, 1280, artifact.VideoMetadata.Height)
	assert.Equal(t, 24.0, artifact.VideoMetadata.FrameRate)
	assert.Contains(t, recorder.Body.String(), `"video_metadata"`)
	assert.NotContains(t, recorder.Body.String(), "private.example", "the persisted snapshot stays private")
}

func TestTaskArtifactOmitsMissingOutputVideoMetadata(t *testing.T) {
	task := outputMetadataProjectionTask(t, ActionInfluencerMotion, map[string]any{
		"object": "video", "status": "completed", "url": "https://private.example/output.mp4",
	})

	recorder, items := getUserTaskArtifacts(t, task)

	require.Len(t, items[0].Artifacts, 1)
	assert.Nil(t, items[0].Artifacts[0].VideoMetadata)
	assert.NotContains(t, recorder.Body.String(), `"video_metadata"`, "historical rows must keep their exact response shape")
}

func TestTaskArtifactDoesNotAttachOutputVideoMetadataToImages(t *testing.T) {
	task := outputMetadataProjectionTask(t, ActionInfluencerMotion, map[string]any{
		"status": "completed", "type": "image", "results": []string{"https://private.example/image.png"},
	})
	storeOutputMetadata(t, task)

	recorder, items := getUserTaskArtifacts(t, task)

	require.Len(t, items[0].Artifacts, 1)
	assert.Equal(t, "image", items[0].Artifacts[0].Type)
	assert.Nil(t, items[0].Artifacts[0].VideoMetadata)
	assert.NotContains(t, recorder.Body.String(), `"video_metadata"`)
}

func TestFirstPlaySchedulesOutputMetadataFromBufferedBytes(t *testing.T) {
	videoBytes := []byte("buffered-output-video")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write(videoBytes)
	}))
	defer upstream.Close()

	task := setupGenericTaskTest(t)
	task.Action = ActionInfluencerMotion
	task.Platform = constant.TaskPlatform("sora")
	task.PrivateData.ResultURL = upstream.URL + "/output.mp4"
	require.NoError(t, model.DB.Save(task).Error)
	allowPrivateTaskMediaTest(t)
	previousMemoryCache := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = false
	t.Cleanup(func() { common.MemoryCacheEnabled = previousMemoryCache })

	type derivedWork struct {
		task  *model.Task
		video []byte
	}
	var captured *derivedWork
	previousEnqueue := enqueueTaskVideoDerivedWork
	enqueueTaskVideoDerivedWork = func(task *model.Task, video []byte) {
		captured = &derivedWork{task: task, video: append([]byte(nil), video...)}
	}
	t.Cleanup(func() { enqueueTaskVideoDerivedWork = previousEnqueue })

	c, recorder := videoArtifactTestRequest(task, nil)
	TaskArtifactContent(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.NotNil(t, captured, "first play must hand the buffered bytes to derived work")
	assert.Equal(t, task.TaskID, captured.task.TaskID)
	assert.Equal(t, videoBytes, captured.video)
}
