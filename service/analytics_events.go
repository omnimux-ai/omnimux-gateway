package service

import (
	"context"
	"math"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/pkg/analytics"
	"github.com/QuantumNous/new-api/types"

	"github.com/go-redis/redis/v8"
	"gorm.io/gorm"
)

// Persisted actions narrow the read. The model or media still has to identify
// a video: generate was also persisted by legacy image routes.
var analyticsVideoActions = []string{
	constant.TaskActionImageToVideo,
	constant.TaskActionTextToVideo,
	constant.TaskActionFirstTailToVideo,
	constant.TaskActionReferenceToVideo,
	constant.TaskActionRemix,
	constant.TaskActionInfluencerMotion,
	constant.TaskActionGenerate,
	constant.TaskActionTextGenerate,
	constant.TaskActionFirstTailGenerate,
	constant.TaskActionReferenceGenerate,
	"remixGenerate",
}

// TrackFirstVideoStart schedules a milestone after the initial task write.
func TrackFirstVideoStart(task *model.Task, entry string) {
	trackVideoMilestone(task, analytics.EventFirstVideoStart, entry)
}

// TrackFirstVideoSuccess schedules a milestone after a committed success.
func TrackFirstVideoSuccess(task *model.Task) {
	if task != nil && task.Status == model.TaskStatusSuccess && task.ResultRetrievable() {
		trackVideoMilestone(task, analytics.EventFirstVideoSuccess, "")
	}
}

func trackVideoMilestone(task *model.Task, name, entry string) {
	if task == nil || task.ID <= 0 || task.UserId <= 0 || !slices.Contains(analyticsVideoActions, task.Action) {
		return
	}
	// Capture only committed identity. The worker rereads the row, including
	// any artifact metadata written since the original task snapshot.
	userID, taskID := task.UserId, task.ID
	occurredAt := time.Now().Unix()
	analytics.Enqueue(func(ctx context.Context) (analytics.Event, bool) {
		var redisClient *redis.Client
		if common.RedisEnabled {
			redisClient = common.RDB
		}
		event, failure := firstVideoMilestone(ctx, model.DB, redisClient, analytics.SiteID(), userID, taskID, name, entry)
		if failure != "" {
			analytics.WarnOnce(failure)
		}
		event.OccurredAt = occurredAt
		return event, event.Name != ""
	})
}

func firstVideoMilestone(ctx context.Context, db *gorm.DB, redisClient *redis.Client, siteID string, userID int, taskID int64, name, entry string) (analytics.Event, string) {
	if redisClient == nil {
		return analytics.Event{}, "video milestone Redis unavailable"
	}
	// The internal account key is scoped to the site and independent of the
	// transport HMAC, so secret rotation does not repeat a milestone.
	key := "analytics:e03:" + siteID + ":" + name + ":" + strconv.Itoa(userID)
	seen, err := redisClient.Exists(ctx, key).Result()
	if err != nil {
		return analytics.Event{}, "video milestone Redis unavailable"
	}
	if seen != 0 {
		return analytics.Event{}, ""
	}

	success := name == analytics.EventFirstVideoSuccess
	task, err := earliestVideoMilestoneTask(ctx, db, userID, success)
	if err != nil {
		return analytics.Event{}, "video milestone history unavailable"
	}
	if task == nil || task.ID != taskID || task.TaskID == "" {
		return analytics.Event{}, ""
	}

	claimed, err := redisClient.SetNX(ctx, key, "1", 0).Result()
	if err != nil {
		return analytics.Event{}, "video milestone Redis unavailable"
	}
	if !claimed {
		return analytics.Event{}, ""
	}
	data := map[string]any{
		"task_id":    task.TaskID,
		"model_name": analyticsVideoModel(task),
	}
	if success {
		data["is_first_success"] = true
		data["output_type"] = "video"
		metadata := observedAnalyticsVideoMetadata(task)
		if metadata.VideoDuration > 0 && metadata.VideoDuration < float64(math.MaxInt64)/1000 {
			data["duration_ms"] = int64(math.Round(metadata.VideoDuration * 1000))
		}
		if metadata.SizeBytes > 0 {
			data["file_size"] = metadata.SizeBytes
		}
	} else {
		data["entry"] = entry
		data["template_id"] = ""
	}
	return analytics.Event{Name: name, UserID: userID, Data: data}, ""
}

// earliestVideoMilestoneTask uses the same historical rows as the live
// milestone and lets callers distinguish a missing observation from a read
// failure. The worker's deadline bounds the scan.
func earliestVideoMilestoneTask(ctx context.Context, db *gorm.DB, userID int, success bool) (*model.Task, error) {
	const pageSize = 32
	query := db.WithContext(ctx).Model(&model.Task{}).
		Select("id", "task_id", "action", "platform", "properties", "private_data", "status", "fail_reason", "finish_time", "output_video_metadata").
		Where("user_id = ? AND action IN ?", userID, analyticsVideoActions)
	if success {
		query = query.Where("status = ?", model.TaskStatusSuccess).Order("finish_time")
	}
	query = query.Order("id").Limit(pageSize).Session(&gorm.Session{})
	var lastID, lastFinish int64
	for {
		pageQuery := query
		if lastID != 0 {
			if success {
				pageQuery = pageQuery.Where("finish_time > ? OR (finish_time = ? AND id > ?)", lastFinish, lastFinish, lastID)
			} else {
				pageQuery = pageQuery.Where("id > ?", lastID)
			}
		}
		var tasks []model.Task
		if err := pageQuery.Find(&tasks).Error; err != nil {
			return nil, err
		}
		for i := range tasks {
			task := &tasks[i]
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			qualifies, err := analyticsVideoTaskQualifies(ctx, db, task, success)
			if err != nil {
				return nil, err
			}
			if qualifies {
				return task, nil
			}
		}
		if len(tasks) < pageSize {
			return nil, nil
		}
		last := tasks[len(tasks)-1]
		lastID, lastFinish = last.ID, last.FinishTime
	}
}

func analyticsVideoModel(task *model.Task) string {
	if name := strings.TrimSpace(task.Properties.OriginModelName); name != "" {
		return name
	}
	return strings.TrimSpace(task.Properties.UpstreamModelName)
}

func analyticsVideoTaskQualifies(ctx context.Context, db *gorm.DB, task *model.Task, success bool) (bool, error) {
	if !slices.Contains(analyticsVideoActions, task.Action) || task.Platform == constant.TaskPlatformMidjourney || task.Platform == constant.TaskPlatformSuno {
		return false, nil
	}
	contract, knownModel := modelcontracts.For(analyticsVideoModel(task))
	if !knownModel {
		contract, knownModel = modelcontracts.For(task.Properties.UpstreamModelName)
	}
	if knownModel && contract.Family != "video" {
		return false, nil
	}
	videoModel := knownModel && contract.Family == "video"
	execution := task.PrivateData.Execution
	if !success && (videoModel || task.Action == constant.TaskActionInfluencerMotion) {
		return true, nil
	}
	if !success && execution != nil {
		p := execution.RequestPath
		if p == "/v1/videos" || p == "/v1/video/generations" || p == "/pg/video/generations" ||
			(strings.HasPrefix(p, "/v1/videos/") && strings.HasSuffix(p, "/remix")) {
			return true, nil
		}
	}
	if task.Status != model.TaskStatusSuccess || !task.ResultRetrievable() {
		return false, nil
	}
	metadata := observedAnalyticsVideoMetadata(task)
	if metadata.Width > 0 && metadata.Height > 0 && metadata.VideoDuration > 0 {
		return true, nil
	}
	if analyticsVideoResultURL(task.GetRawUpstreamURL(), task.TaskID, videoModel) {
		return true, nil
	}
	if execution == nil || execution.TaskPlugin == nil || execution.TaskPlugin.Key == "" || GetTaskAdaptorFunc == nil {
		return false, nil
	}
	// Reuse the same credential-free artifact projection as the public result
	// route. It runs on a fresh adaptor, with no media fetch, and reads only
	// the snapshot needed for a task whose result isn't already identified.
	provider, ok := GetTaskAdaptorFunc(task.Platform).(interface {
		ListArtifactsWithContext(context.Context, *model.Task) ([]types.TaskArtifact, error)
	})
	if !ok {
		return false, nil
	}
	var snapshot model.Task
	if err := db.WithContext(ctx).Select("data").Where("id = ?", task.ID).Take(&snapshot).Error; err != nil {
		return false, err
	}
	task.Data = snapshot.Data
	artifacts, err := provider.ListArtifactsWithContext(ctx, task)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(artifacts, func(artifact types.TaskArtifact) bool {
		return artifact.Key != "" && artifact.Type == "video" &&
			(artifact.MimeType == "" || strings.HasPrefix(strings.ToLower(artifact.MimeType), "video/"))
	}), nil
}

func analyticsVideoResultURL(raw, taskID string, videoModel bool) bool {
	raw = strings.TrimSpace(raw)
	if header, data, ok := strings.Cut(raw, ","); ok && strings.HasPrefix(header, "data:video/") {
		return data != ""
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Hostname() == "" || parsed.User != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return false
	}
	if strings.HasSuffix(parsed.Path, "/content") &&
		(strings.HasSuffix(parsed.Path, "/v1/videos/"+taskID+"/content") ||
			strings.HasSuffix(parsed.Path, "/v1/images/"+taskID+"/content") ||
			strings.Contains(parsed.Path, "/v1/tasks/"+taskID+"/artifacts/")) {
		return false
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".mp4", ".webm", ".mov", ".m4v", ".mkv", ".avi":
		return true
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".avif", ".svg", ".mp3", ".wav", ".ogg", ".json", ".html":
		return false
	default:
		return videoModel
	}
}

func observedAnalyticsVideoMetadata(task *model.Task) dto.InfluencerVideoMetadata {
	var metadata dto.InfluencerVideoMetadata
	if len(task.OutputVideoMetadata) != 0 && common.Unmarshal(task.OutputVideoMetadata, &metadata) != nil {
		return dto.InfluencerVideoMetadata{}
	}
	return metadata
}
