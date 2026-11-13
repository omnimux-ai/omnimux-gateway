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
)

// fork: generated videos are persisted on first use, but a user should not pay
// that first fetch at playback time. A bounded background queue warms the
// newest video artifacts of a task list — video first, then its cover — so the
// media element opens straight from our own object store.
const (
	// taskVideoArtifactKey is the artifact key every video plugin projects.
	taskVideoArtifactKey = "video"
	// maxTaskVideoPersistBytes bounds one buffered video. A larger stream keeps
	// the upstream proxy path instead of being held in memory.
	maxTaskVideoPersistBytes int64 = 32 << 20

	taskArtifactPreheatWorkers = 2
	taskArtifactPreheatQueue   = 32
	// taskVideoDerivedQueue stays tiny on purpose: each queued job pins one
	// buffered video until a worker takes it.
	taskVideoDerivedQueue        = 2
	taskArtifactPreheatListLimit = 6
	taskArtifactPreheatTimeout   = 3 * time.Minute
	taskVideoDerivedTimeout      = 30 * time.Second

	// Poster probes answer the list request: they run concurrently and give up
	// together, so a slow object store cannot stretch the response.
	taskArtifactPosterProbeConcurrency = 8
	taskArtifactPosterProbeBudget      = 800 * time.Millisecond
)

type taskArtifactPreheatJob struct {
	task *model.Task
}

var scheduleTaskArtifactPreheat = preheatTaskArtifacts

var (
	taskArtifactPreheatOnce sync.Once
	taskArtifactPreheatCh   chan taskArtifactPreheatJob
	// taskArtifactPreheatInflight keeps one job per task: list polling repeats
	// every couple of seconds and must not queue the same video repeatedly.
	taskArtifactPreheatInflight sync.Map
)

func startTaskArtifactPreheat() {
	taskArtifactPreheatCh = make(chan taskArtifactPreheatJob, taskArtifactPreheatQueue)
	for i := 0; i < taskArtifactPreheatWorkers; i++ {
		go taskArtifactPreheatWorker()
	}
}

func taskArtifactPreheatWorker() {
	for job := range taskArtifactPreheatCh {
		task := job.task
		ctx, cancel := context.WithTimeout(context.Background(), taskArtifactPreheatTimeout)
		if err := preheatTaskVideoArtifact(ctx, task); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("task %s artifact preheat failed: %v", task.TaskID, err))
		}
		cancel()
		taskArtifactPreheatInflight.Delete(task.TaskID)
	}
}

// preheatTaskArtifacts schedules warming for the newest video tasks of one list
// response. It never blocks the request and silently drops work when the queue
// is full: a cold artifact is still persisted on first playback.
func preheatTaskArtifacts(tasks []*model.Task) {
	if len(tasks) == 0 {
		return
	}
	taskArtifactPreheatOnce.Do(startTaskArtifactPreheat)
	scheduled := 0
	for _, task := range tasks {
		if scheduled >= taskArtifactPreheatListLimit {
			return
		}
		if !taskArtifactPreheatCandidate(task) {
			continue
		}
		if _, loaded := taskArtifactPreheatInflight.LoadOrStore(task.TaskID, struct{}{}); loaded {
			continue
		}
		select {
		case taskArtifactPreheatCh <- taskArtifactPreheatJob{task: task}:
			scheduled++
		default:
			taskArtifactPreheatInflight.Delete(task.TaskID)
			return
		}
	}
}

func taskArtifactPreheatCandidate(task *model.Task) bool {
	return task != nil &&
		task.Status == model.TaskStatusSuccess &&
		task.ResultRetrievable() &&
		taskHasPluginExecution(task)
}

// preheatTaskVideoArtifact keeps our own copy of a finished video and its
// cover. When the video is already stored, only a missing cover is rebuilt,
// and that rebuild reads our copy instead of the upstream provider.
func preheatTaskVideoArtifact(ctx context.Context, task *model.Task) error {
	if service.IsTaskArtifactStored(task, taskVideoArtifactKey) {
		return ensureTaskVideoPoster(ctx, task)
	}
	descriptor, err := taskVideoContentDescriptor(ctx, task)
	if err != nil {
		return err
	}
	buffered, err := fetchTaskMediaForPersistContext(ctx, nil, task, descriptor, maxTaskVideoPersistBytes, nil)
	if err != nil {
		return err
	}
	if len(buffered.data) == 0 {
		return errors.New("upstream returned an empty video")
	}
	if _, err := service.PersistTaskArtifact(ctx, task, taskVideoArtifactKey, buffered.mimeType, buffered.data); err != nil {
		return err
	}
	persistTaskOutputVideoMetadata(ctx, task, buffered.data)
	return service.PersistVideoArtifactPoster(ctx, task, buffered.data)
}

// persistTaskOutputVideoMetadata records generated-output media facts for the
// workbench motion tasks only. The probe runs off the request path over bytes
// that are already in memory and is best-effort: a failure leaves the task row
// and its completion success untouched, and the client keeps its fallback read.
func persistTaskOutputVideoMetadata(ctx context.Context, task *model.Task, video []byte) {
	if task == nil || task.Action != ActionInfluencerMotion {
		return
	}
	if err := service.PersistTaskOutputVideoMetadata(ctx, task, video); err != nil {
		logger.LogWarn(ctx, fmt.Sprintf("task %s output video metadata skipped: %v", task.TaskID, err))
	}
}

func ensureTaskVideoPoster(ctx context.Context, task *model.Task) error {
	if service.IsTaskPosterStored(task) {
		return nil
	}
	video, _, err := service.ReadTaskArtifact(ctx, task, taskVideoArtifactKey, maxTaskVideoPersistBytes)
	if err != nil {
		return err
	}
	return service.PersistVideoArtifactPoster(ctx, task, video)
}

// taskVideoContentDescriptor asks the task plugin how to fetch the stored video
// from its provider, so the preheater uses the same route as a live playback.
func taskVideoContentDescriptor(ctx context.Context, task *model.Task) (*relaychannel.TaskContentRequest, error) {
	adaptor, err := initTaskArtifactAdaptor(task)
	if err != nil {
		return nil, err
	}
	provider, ok := adaptor.(relaychannel.TaskContentRequestProvider)
	if !ok {
		return nil, errors.New("artifact content plugin is unavailable")
	}
	descriptor, err := provider.BuildContentRequest(ctx, task, taskVideoArtifactKey, relaychannel.TaskArtifactClientRequest{
		Method: http.MethodGet,
	})
	if err != nil {
		return nil, err
	}
	if descriptor == nil {
		return nil, errors.New("artifact content plugin returned no request")
	}
	return descriptor, nil
}

// storedTaskPosterURLs reports the cover address of every task that already has
// a stored cover, probing concurrently and giving up on the shared budget.
func storedTaskPosterURLs(tasks []*model.Task) map[string]string {
	urls := make(map[string]string, len(tasks))
	if len(tasks) == 0 {
		return urls
	}
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, taskArtifactPosterProbeConcurrency)
	deadline := time.Now().Add(taskArtifactPosterProbeBudget)
	for _, task := range tasks {
		if time.Now().After(deadline) {
			break
		}
		slots <- struct{}{}
		wg.Add(1)
		go func(task *model.Task) {
			defer wg.Done()
			defer func() { <-slots }()
			if !service.IsTaskPosterStored(task) {
				return
			}
			url, err := service.BuildTaskArtifactContentURL(task.TaskID, service.TaskArtifactPosterKey)
			if err != nil {
				return
			}
			mu.Lock()
			urls[task.TaskID] = url
			mu.Unlock()
		}(task)
	}
	wg.Wait()
	return urls
}

type taskVideoDerivedJob struct {
	task  *model.Task
	video []byte
}

var (
	taskVideoDerivedOnce sync.Once
	taskVideoDerivedCh   chan taskVideoDerivedJob
	// enqueueTaskVideoDerivedWork is a variable so handler tests can observe the
	// first-play scheduling without waiting on the background worker.
	enqueueTaskVideoDerivedWork = enqueueTaskVideoDerived
)

// enqueueTaskVideoDerived stores a video's cover and its observed output media
// facts off the request path. A full queue drops the job on purpose: the
// preheater rebuilds a missing cover from our own stored copy, and metadata is
// best-effort with the client's existing fallback read.
func enqueueTaskVideoDerived(task *model.Task, video []byte) {
	taskVideoDerivedOnce.Do(startTaskVideoDerived)
	select {
	case taskVideoDerivedCh <- taskVideoDerivedJob{task: task, video: video}:
	default:
	}
}

func startTaskVideoDerived() {
	taskVideoDerivedCh = make(chan taskVideoDerivedJob, taskVideoDerivedQueue)
	for i := 0; i < taskArtifactPreheatWorkers; i++ {
		go taskVideoDerivedWorker()
	}
}

func taskVideoDerivedWorker() {
	for job := range taskVideoDerivedCh {
		ctx, cancel := context.WithTimeout(context.Background(), taskVideoDerivedTimeout)
		if err := service.PersistVideoArtifactPoster(ctx, job.task, job.video); err != nil {
			logger.LogWarn(ctx, fmt.Sprintf("task %s poster persist failed: %v", job.task.TaskID, err))
		}
		persistTaskOutputVideoMetadata(ctx, job.task, job.video)
		cancel()
	}
}

// taskArtifactsContainVideo reports whether a projected artifact list carries a
// video, which is what makes a task worth warming.
func taskArtifactsContainVideo(artifacts []dto.TaskArtifactDto) bool {
	for i := range artifacts {
		if artifacts[i].Type == "video" {
			return true
		}
	}
	return false
}

// storedTaskPosterURL reports the cover address when the task already has one.
func storedTaskPosterURL(task *model.Task) string {
	if task == nil || !service.IsTaskPosterStored(task) {
		return ""
	}
	url, err := service.BuildTaskArtifactContentURL(task.TaskID, service.TaskArtifactPosterKey)
	if err != nil {
		return ""
	}
	return url
}
