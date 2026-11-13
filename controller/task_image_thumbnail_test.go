package controller

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func storedImageThumbnailTask(t *testing.T) (*model.Task, []byte) {
	t.Helper()
	task := setupGenericTaskTest(t)
	task.Action = "generate"
	task.Properties.OriginModelName = "gpt-image-2.5"
	task.PrivateData.ResultURL = "https://original.example/image.png"
	require.NoError(t, model.DB.Save(task).Error)
	thumbnail, err := base64.StdEncoding.DecodeString("UklGRmAAAABXRUJQVlA4WAoAAAAQAAAABwAACwAAQUxQSAkAAAABBxAREYiI/gcAVlA4IDAAAADQAQCdASoIAAwAAUAmJaACdLoB+AADsAD+8ut//NgVzXPv9//S4P0uD9Lg/9KQAAA=")
	require.NoError(t, err)
	_, err = service.PersistTaskArtifact(context.Background(), task, service.TaskImageThumbnailKey("image"), "image/webp", thumbnail)
	require.NoError(t, err)
	return task, thumbnail
}

func TestImageThumbnailHistoryKeepsOriginalAndAdvertisesDimensions(t *testing.T) {
	task, _ := storedImageThumbnailTask(t)
	views := tasksToDto([]*model.Task{task}, false, common.RoleCommonUser, true)
	require.Len(t, views, 1)
	require.Len(t, views[0].Artifacts, 1)
	artifact := views[0].Artifacts[0]
	assert.Contains(t, artifact.ContentURL, "/artifacts/image/content")
	assert.Contains(t, artifact.ThumbnailURL, "/artifacts/"+service.TaskImageThumbnailKey("image")+"/content")
	assert.Equal(t, 8, artifact.ThumbnailWidth)
	assert.Equal(t, 12, artifact.ThumbnailHeight)
	assert.Equal(t, artifact.ThumbnailURL, views[0].ThumbnailURL)
	assert.Equal(t, model.TaskStatus(model.TaskStatusSuccess), task.Status)
	assert.Equal(t, 0, task.Quota)
}

func TestImageThumbnailContentPreservesOwnershipAndBaseArtifact(t *testing.T) {
	task, thumbnail := storedImageThumbnailTask(t)
	for _, tc := range []struct {
		name, key string
		owner     int
		status    int
	}{
		{"owner", service.TaskImageThumbnailKey("image"), task.UserId, http.StatusOK},
		{"foreign owner", service.TaskImageThumbnailKey("image"), task.UserId + 1, http.StatusNotFound},
		{"unprojected source", service.TaskImageThumbnailKey("missing"), task.UserId, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Set("id", tc.owner)
			c.Set("role", common.RoleCommonUser)
			c.Params = gin.Params{{Key: "key", Value: task.TaskID}, {Key: "artifact_key", Value: tc.key}}
			c.Request = httptest.NewRequest(http.MethodGet, "/v1/tasks/"+task.TaskID+"/artifacts/"+tc.key+"/content", nil)
			TaskArtifactContent(c)
			assert.Equal(t, tc.status, recorder.Code)
			if tc.status == http.StatusOK {
				assert.Equal(t, thumbnail, recorder.Body.Bytes())
				assert.Equal(t, "image/webp", recorder.Header().Get("Content-Type"))
			}
		})
	}
	// A signed capability still requires the active owner check.
	require.NoError(t, model.DB.Model(&model.User{}).Where("id = ?", task.UserId).Update("status", common.UserStatusDisabled).Error)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Set(middleware.TaskArtifactAccessContextKey, true)
	c.Params = gin.Params{{Key: "key", Value: task.TaskID}, {Key: "artifact_key", Value: service.TaskImageThumbnailKey("image")}}
	c.Request = httptest.NewRequest(http.MethodGet, "/", nil)
	TaskArtifactContent(c)
	assert.Equal(t, http.StatusNotFound, recorder.Code)
}

func TestImageThumbnailQueueRejectsUnprojectedAndTerminalArtifacts(t *testing.T) {
	task, _ := storedImageThumbnailTask(t)
	queue := &taskImageThumbnailQueueState{}
	assert.False(t, queue.enqueue(task, "poster"), "video posters are not generated images")
	assert.False(t, queue.enqueue(task, "unprojected"))
	identity := fmt.Sprintf("%d/%s/%s", task.UserId, task.TaskID, "image")
	queue.failures.Store(identity, "source expired after three attempts")
	t.Cleanup(func() { queue.failures.Delete(identity) })
	assert.False(t, queue.enqueue(task, "image"), "list polling must not restart exhausted retries")
	assert.Equal(t, "source expired after three attempts", queue.failure(task, "image"))
}

func TestImageThumbnailProjectionKeepsEachOutputIndependent(t *testing.T) {
	task, thumbnail := storedImageThumbnailTask(t)
	_, err := service.PersistTaskArtifact(t.Context(), task, service.TaskImageThumbnailKey("image-2"), "image/webp", thumbnail)
	require.NoError(t, err)
	lists := [][]dto.TaskArtifactDto{{{Key: "image", Type: "image", ContentURL: "original-1"}, {Key: "image-2", Type: "image", ContentURL: "original-2"}}}
	populateTaskImageThumbnails([]*model.Task{task}, lists)
	assert.NotEmpty(t, lists[0][0].ThumbnailURL)
	assert.NotEmpty(t, lists[0][1].ThumbnailURL)
	assert.NotEqual(t, lists[0][0].ThumbnailURL, lists[0][1].ThumbnailURL)
	assert.Equal(t, "original-1", lists[0][0].ContentURL)
	assert.Equal(t, "original-2", lists[0][1].ContentURL)
}

func TestImageThumbnailRetriesAreBoundedAndReleaseDeduplication(t *testing.T) {
	task, _ := storedImageThumbnailTask(t)
	queue := &taskImageThumbnailQueueState{}
	calls := 0
	queue.prepare = func(ctx context.Context, _ *model.Task, _ string) error {
		calls++
		deadline, ok := ctx.Deadline()
		assert.True(t, ok)
		assert.LessOrEqual(t, time.Until(deadline), 30*time.Second)
		return errors.New("expired original")
	}
	identity := fmt.Sprintf("%d/%s/%s", task.UserId, task.TaskID, "image")
	queue.inflight.Store(identity, struct{}{})
	t.Cleanup(func() { queue.failures.Delete(identity); queue.inflight.Delete(identity) })
	queue.process(taskImageThumbnailJob{task: task, key: "image"})
	assert.Equal(t, 3, calls)
	assert.Equal(t, "expired original", queue.failure(task, "image"))
	_, inflight := queue.inflight.Load(identity)
	assert.False(t, inflight)
	assert.False(t, queue.enqueue(task, "image"))
	calls = 0
	task.TaskID = "retry-success"
	queue.prepare = func(context.Context, *model.Task, string) error {
		calls++
		if calls == 1 {
			return errors.New("temporary")
		}
		return nil
	}
	queue.process(taskImageThumbnailJob{task: task, key: "image"})
	assert.Equal(t, 2, calls)
	assert.Empty(t, queue.failure(task, "image"))
}

func TestImageThumbnailQueueDropsOverflowAndDeduplicates(t *testing.T) {
	task, _ := storedImageThumbnailTask(t)
	// No workers are attached to this isolated queue, so saturation is deterministic.
	queue := &taskImageThumbnailQueueState{}
	assert.True(t, queue.enqueue(task, "image"))
	assert.False(t, queue.enqueue(task, "image"))
	for i := 1; i < 32; i++ {
		other := *task
		other.TaskID = fmt.Sprintf("queue-%d", i)
		require.True(t, queue.enqueue(&other, "image"))
	}
	overflow := *task
	overflow.TaskID = "overflow"
	assert.False(t, queue.enqueue(&overflow, "image"))
	job := <-queue.jobs
	queue.inflight.Delete(fmt.Sprintf("%d/%s/%s", job.task.UserId, job.task.TaskID, job.key))
	assert.True(t, queue.enqueue(&overflow, "image"), "dropped work must remain eligible for scanner recovery")
}

func TestImageThumbnailSyntheticPluginExposesAllProjectedImages(t *testing.T) {
	task, _ := storedImageThumbnailTask(t)
	key := "thumbnail-multiple-test"
	source := taskPluginControllerTestSource(key, "1.0.0") + `\nexport function listArtifacts() { return [{key:"first",type:"image",mimeType:"image/png"},{key:"second",type:"image",mimeType:"image/png"}]; }
export function buildContentRequest(ctx) { return {url:"https://example.com/"+ctx.artifactKey,method:"GET",credentialless:true}; }`
	source = strings.ReplaceAll(source, `\nexport`, "\nexport")
	_, err := jsplugin.DefaultRegistry.Register(source, jsplugin.Options{})
	require.NoError(t, err)
	t.Cleanup(func() { jsplugin.DefaultRegistry.Unregister(key) })
	task.Platform = constant.TaskPlatform(key)
	task.PrivateData.Execution = &model.TaskExecutionSnapshot{TaskPlugin: &model.TaskPluginSnapshot{Key: key, Name: "Test", Version: "1.0.0", APIVersion: 1}}
	keys, err := taskImageArtifactKeys(task)
	require.NoError(t, err)
	assert.Equal(t, []string{"first", "second"}, keys)
}

func TestImageThumbnailPageProbesShareDeadlineAndCancelOutstandingReads(t *testing.T) {
	task, _ := storedImageThumbnailTask(t)
	deadlines := make(chan time.Time, 64)
	finished := make(chan struct{}, 64)
	resolve := func(ctx context.Context, _ *model.Task, _ string) (*service.ImageThumbnailMetadata, error) {
		deadline, _ := ctx.Deadline()
		deadlines <- deadline
		<-ctx.Done()
		finished <- struct{}{}
		return nil, ctx.Err()
	}
	lists := [][]dto.TaskArtifactDto{make([]dto.TaskArtifactDto, 32)}
	for i := range lists[0] {
		lists[0][i] = dto.TaskArtifactDto{Key: fmt.Sprintf("image-%d", i), Type: "image", ContentURL: "original"}
	}
	populateTaskImageThumbnailsWithResolver([]*model.Task{task}, lists, resolve)
	count := len(deadlines)
	assert.Positive(t, count)
	assert.LessOrEqual(t, count, taskArtifactPosterProbeConcurrency)
	var deadline time.Time
	for i := 0; i < count; i++ {
		value := <-deadlines
		if i == 0 {
			deadline = value
		} else {
			assert.Equal(t, deadline, value)
		}
		<-finished
	}
	for _, artifact := range lists[0] {
		assert.Empty(t, artifact.ThumbnailURL)
		assert.Equal(t, "original", artifact.ContentURL)
	}
}

func TestImageThumbnailQueuedJobsRetainOnlyImmutableIdentifiers(t *testing.T) {
	task, _ := storedImageThumbnailTask(t)
	queue := &taskImageThumbnailQueueState{prepare: prepareQueuedTaskImageThumbnail}
	task.Data = []byte(`{"images":[{"b64_json":"large-inline-payload"}]}`)
	task.Properties.Input = "large-input-payload"
	capturedID, capturedUser, capturedTaskID := task.ID, task.UserId, task.TaskID
	require.True(t, queue.enqueue(task, "image"))
	task.ID++
	task.UserId++
	task.TaskID = "caller-mutated"
	task.Data[0] = 'x'
	job := <-queue.jobs
	assert.NotSame(t, task, job.task)
	assert.Equal(t, capturedID, job.task.ID)
	assert.Equal(t, capturedUser, job.task.UserId)
	assert.Equal(t, capturedTaskID, job.task.TaskID)
	assert.Nil(t, job.task.Data)
	assert.Empty(t, job.task.Properties.Input)
	assert.Empty(t, job.task.PrivateData.ResultURL)
	// The thin snapshot has no provider metadata, but real preparation reloads
	// the matching successful database row and finds its ready derivative.
	require.NoError(t, prepareQueuedTaskImageThumbnail(t.Context(), job.task, job.key))
	// Current database eligibility wins over the original successful snapshot.
	require.NoError(t, model.DB.Model(&model.Task{}).Where("id = ?", capturedID).Update("status", model.TaskStatusFailure).Error)
	require.NoError(t, prepareQueuedTaskImageThumbnail(t.Context(), job.task, job.key))
	// Owner/row identity must match, so an identifier cannot resolve another row.
	job.task.UserId++
	require.NoError(t, prepareQueuedTaskImageThumbnail(t.Context(), job.task, job.key))
}

func TestImageThumbnailQueuedPreparationUsesFreshDatabaseSourceAndStatus(t *testing.T) {
	task := setupGenericTaskTest(t)
	allowPrivateTaskMediaTest(t)
	task.Action = "generate"
	task.Properties.OriginModelName = "gpt-image-2.5"
	task.PrivateData.ResultURL = "https://stale-provider.invalid/image.png"
	require.NoError(t, model.DB.Save(task).Error)
	queue := &taskImageThumbnailQueueState{prepare: prepareQueuedTaskImageThumbnail}
	require.True(t, queue.enqueue(task, "image"))
	job := <-queue.jobs
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("invalid-image-header"))
	}))
	defer upstream.Close()
	current := *task
	current.PrivateData.ResultURL = upstream.URL
	require.NoError(t, model.DB.Save(&current).Error)
	task.PrivateData.ResultURL = "https://mutated-caller.invalid/image.png"
	require.ErrorContains(t, prepareQueuedTaskImageThumbnail(t.Context(), job.task, job.key), "image thumbnail header")
	assert.Equal(t, int32(1), calls.Load(), "processing must fetch the current database source, not the queued caller snapshot")
	require.NoError(t, model.DB.Model(&model.Task{}).Where("id = ?", current.ID).Update("status", model.TaskStatusFailure).Error)
	require.NoError(t, prepareQueuedTaskImageThumbnail(t.Context(), job.task, job.key))
	assert.Equal(t, int32(1), calls.Load(), "a task no longer successful must not be fetched again")
}

func TestImageThumbnailEagerFetchDoesNotPersistErrorBodyAndRecovers(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("image codec unavailable")
	}
	task := setupGenericTaskTest(t)
	allowPrivateTaskMediaTest(t)
	original := influencerPresetTestPNG(t, 16, 24)
	var calls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		if calls.Add(1) == 1 {
			_, _ = w.Write([]byte("provider temporarily unavailable"))
			return
		}
		_, _ = w.Write(original)
	}))
	defer upstream.Close()
	task.Action = "generate"
	task.Properties.OriginModelName = "gpt-image-2.5"
	task.PrivateData.ResultURL = upstream.URL
	require.NoError(t, model.DB.Save(task).Error)
	require.ErrorContains(t, prepareTaskImageThumbnail(t.Context(), task, "image"), "image thumbnail header")
	assert.False(t, service.IsTaskArtifactStored(task, "image"), "a successful HTTP status and image MIME must not cache an error body")
	require.NoError(t, prepareTaskImageThumbnail(t.Context(), task, "image"))
	assert.Equal(t, int32(2), calls.Load(), "an invalid first body must not prevent a fresh provider fetch")
	stored, _, err := service.ReadTaskArtifact(t.Context(), task, "image", service.MaxTaskArtifactPersistBytes)
	require.NoError(t, err)
	assert.Equal(t, original, stored, "valid original bytes remain unchanged")
	thumbnail, err := service.ResolveTaskImageThumbnail(t.Context(), task, "image")
	require.NoError(t, err)
	require.NotNil(t, thumbnail)
	assert.Equal(t, 16, thumbnail.Width)
	assert.Equal(t, 24, thumbnail.Height)
}

func TestImageThumbnailEagerFetchRejectsOversizedHeaderBeforePersistence(t *testing.T) {
	task := setupGenericTaskTest(t)
	allowPrivateTaskMediaTest(t)
	oversized := append([]byte("GIF89a"), []byte{255, 255, 255, 255, 0, 0, 0}...)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "image/gif")
		_, _ = w.Write(oversized)
	}))
	defer upstream.Close()
	task.Action = "generate"
	task.Properties.OriginModelName = "gpt-image-2.5"
	task.PrivateData.ResultURL = upstream.URL
	require.NoError(t, model.DB.Save(task).Error)
	require.ErrorContains(t, prepareTaskImageThumbnail(t.Context(), task, "image"), "image thumbnail header")
	assert.False(t, service.IsTaskArtifactStored(task, "image"))
}
