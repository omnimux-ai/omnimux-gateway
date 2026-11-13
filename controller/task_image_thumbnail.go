package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type taskImageThumbnailJob struct {
	task *model.Task
	key  string
}

type taskImageThumbnailQueueState struct {
	once     sync.Once
	jobs     chan taskImageThumbnailJob
	inflight sync.Map
	failures sync.Map
	workers  int
	prepare  func(context.Context, *model.Task, string) error
}

var taskImageThumbnails = &taskImageThumbnailQueueState{workers: 2, prepare: prepareQueuedTaskImageThumbnail}

// Tests can suppress asynchronous storage work while swapping process-wide drivers.
var taskImageThumbnailSchedule = queueTaskImageThumbnail

func enqueueTaskImageThumbnail(task *model.Task, key string) bool {
	return taskImageThumbnailSchedule(task, key)
}

func taskImageThumbnailFailure(task *model.Task, key string) string {
	return taskImageThumbnails.failure(task, key)
}

func (queue *taskImageThumbnailQueueState) failure(task *model.Task, key string) string {
	if task == nil {
		return ""
	}
	value, ok := queue.failures.Load(fmt.Sprintf("%d/%s/%s", task.UserId, task.TaskID, key))
	if !ok {
		return ""
	}
	return value.(string)
}

func init() {
	service.SetTaskImageThumbnailScheduler(func(task *model.Task, key string) { enqueueTaskImageThumbnail(task, key) })
}

func queueTaskImageThumbnail(task *model.Task, key string) bool {
	return taskImageThumbnails.enqueue(task, key)
}

// Jobs retain only task metadata and an artifact key, never image buffers.
func (queue *taskImageThumbnailQueueState) enqueue(task *model.Task, key string) bool {
	if task == nil || task.Status != model.TaskStatusSuccess || !task.ResultRetrievable() || key == "" {
		return false
	}
	keys, err := taskImageArtifactKeys(task)
	if err != nil {
		return false
	}
	found := false
	for _, imageKey := range keys {
		if imageKey == key {
			found = true
			break
		}
	}
	if !found || queue.failure(task, key) != "" {
		return false
	}
	queue.once.Do(func() {
		queue.jobs = make(chan taskImageThumbnailJob, 32)
		for i := 0; i < queue.workers; i++ {
			go func() {
				for job := range queue.jobs {
					queue.process(job)
				}
			}()
		}
	})
	identity := fmt.Sprintf("%d/%s/%s", task.UserId, task.TaskID, key)
	if _, loaded := queue.inflight.LoadOrStore(identity, struct{}{}); loaded {
		return false
	}
	select {
	case queue.jobs <- taskImageThumbnailJob{task: &model.Task{ID: task.ID, UserId: task.UserId, TaskID: task.TaskID}, key: key}:
		return true
	default:
		queue.inflight.Delete(identity)
		return false
	}
}

func (queue *taskImageThumbnailQueueState) process(job taskImageThumbnailJob) {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		err = queue.prepare(ctx, job.task, job.key)
		cancel()
		if err == nil {
			break
		}
	}
	identity := fmt.Sprintf("%d/%s/%s", job.task.UserId, job.task.TaskID, job.key)
	if err != nil {
		queue.failures.Store(identity, err.Error())
		logger.LogWarn(context.Background(), fmt.Sprintf("task %s image thumbnail failed: %v", job.task.TaskID, err))
	}
	queue.inflight.Delete(identity)
}

func taskImageArtifactKeys(task *model.Task) ([]string, error) {
	artifacts, err := projectTaskArtifacts(task)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		if artifact.Type == "image" {
			keys = append(keys, artifact.Key)
		}
	}
	if legacyImageAvailable(task) {
		keys = append(keys, "image")
	}
	return keys, nil
}

// Resolve the original only while a worker is active. Queued snapshots carry
// identifiers alone, so inline image data cannot accumulate in the queue.
func prepareQueuedTaskImageThumbnail(ctx context.Context, identity *model.Task, key string) error {
	var task model.Task
	err := model.DB.WithContext(ctx).Where("id = ? AND user_id = ? AND task_id = ?", identity.ID, identity.UserId, identity.TaskID).First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if task.Status != model.TaskStatusSuccess || !task.ResultRetrievable() {
		return nil
	}
	keys, err := taskImageArtifactKeys(&task)
	if err != nil {
		return err
	}
	for _, imageKey := range keys {
		if imageKey == key {
			return prepareTaskImageThumbnail(ctx, &task, key)
		}
	}
	return nil
}

func prepareTaskImageThumbnail(ctx context.Context, task *model.Task, key string) error {
	if metadata, _ := service.ResolveTaskImageThumbnail(ctx, task, key); metadata != nil {
		return nil
	}
	data, _, err := service.ReadTaskArtifact(ctx, task, key, service.MaxTaskArtifactPersistBytes)
	if err != nil {
		var descriptor *relaychannel.TaskContentRequest
		if !taskHasPluginExecution(task) {
			if key != "image" || !legacyImageAvailable(task) {
				return errors.New("image artifact unavailable")
			}
			descriptor = &relaychannel.TaskContentRequest{URL: task.GetRawUpstreamURL(), Method: http.MethodGet, Credentialless: true}
		} else {
			adaptor, initErr := initTaskArtifactAdaptor(task)
			if initErr != nil {
				return initErr
			}
			provider, ok := adaptor.(relaychannel.TaskContentRequestProvider)
			if !ok {
				return errors.New("image content provider unavailable")
			}
			descriptor, err = provider.BuildContentRequest(ctx, task, key, relaychannel.TaskArtifactClientRequest{Method: http.MethodGet})
			if err != nil {
				return err
			}
		}
		if descriptor == nil {
			return errors.New("image content descriptor unavailable")
		}
		buffered, fetchErr := fetchTaskMediaForPersistContext(ctx, nil, task, descriptor, service.MaxTaskArtifactPersistBytes, nil)
		if fetchErr != nil {
			return fetchErr
		}
		data = buffered.data
		if len(data) == 0 {
			return errors.New("empty image artifact")
		}
		config, _, decodeErr := service.DecodeImageConfig(data)
		if decodeErr != nil {
			return fmt.Errorf("image thumbnail header: %w", decodeErr)
		}
		if config.Width <= 0 || config.Height <= 0 || int64(config.Width) > service.ImageThumbnailMaxPixels/int64(config.Height) {
			return errors.New("image thumbnail header: invalid dimensions or pixel limit exceeded")
		}
		if _, err = service.PersistTaskArtifact(ctx, task, key, buffered.mimeType, data); err != nil {
			return err
		}
	}
	return service.PersistTaskImageThumbnail(ctx, task, key, data)
}

// A single request budget covers every image in the page, including multi-output tasks.
func populateTaskImageThumbnails(tasks []*model.Task, lists [][]dto.TaskArtifactDto) {
	populateTaskImageThumbnailsWithResolver(tasks, lists, service.ResolveTaskImageThumbnail)
}

func populateTaskImageThumbnailsWithResolver(tasks []*model.Task, lists [][]dto.TaskArtifactDto, resolve func(context.Context, *model.Task, string) (*service.ImageThumbnailMetadata, error)) {
	type probe struct {
		row, index int
		metadata   *service.ImageThumbnailMetadata
	}
	ctx, cancel := context.WithTimeout(context.Background(), taskArtifactPosterProbeBudget)
	defer cancel()
	jobs := make(chan [2]int)
	results := make(chan probe)
	count := 0
	for i := range lists {
		for j := range lists[i] {
			if lists[i][j].Type == "image" {
				count++
			}
		}
	}
	if count == 0 {
		return
	}
	for n := 0; n < taskArtifactPosterProbeConcurrency; n++ {
		go func() {
			for job := range jobs {
				if ctx.Err() != nil {
					return
				}
				task := tasks[job[0]]
				artifact := lists[job[0]][job[1]]
				metadata, _ := resolve(ctx, task, artifact.Key)
				if metadata == nil && ctx.Err() == nil {
					enqueueTaskImageThumbnail(task, artifact.Key)
				}
				select {
				case results <- probe{job[0], job[1], metadata}:
				case <-ctx.Done():
					return
				}
			}
		}()
	}
	go func() {
		defer close(jobs)
		for i := range lists {
			for j := range lists[i] {
				if lists[i][j].Type == "image" {
					select {
					case jobs <- [2]int{i, j}:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	for n := 0; n < count; n++ {
		select {
		case result := <-results:
			if result.metadata == nil {
				continue
			}
			artifact := &lists[result.row][result.index]
			url, err := service.BuildTaskArtifactContentURL(tasks[result.row].TaskID, service.TaskImageThumbnailKey(artifact.Key))
			if err != nil {
				continue
			}
			artifact.ThumbnailURL = url
			artifact.ThumbnailWidth = result.metadata.Width
			artifact.ThumbnailHeight = result.metadata.Height
		case <-ctx.Done():
			return
		}
	}
}

func serveTaskImageThumbnail(c *gin.Context, task *model.Task, key string) bool {
	if !service.IsTaskImageThumbnailKey(key) {
		return false
	}
	keys, err := taskImageArtifactKeys(task)
	if err != nil {
		writeTaskArtifactProjectionError(c, err)
		return true
	}
	for _, base := range keys {
		if service.TaskImageThumbnailKey(base) == key {
			ref, resolveErr := service.GetTaskArtifactStore().Resolve(task, key)
			if resolveErr == nil && ref != nil {
				if serveErr := service.GetTaskArtifactStore().Serve(c, task, ref); serveErr != nil && !c.Writer.Written() {
					writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Task or artifact not found")
				}
				return true
			}
			enqueueTaskImageThumbnail(task, base)
			break
		}
	}
	writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Task or artifact not found")
	return true
}
