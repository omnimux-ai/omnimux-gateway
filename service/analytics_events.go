package service

import (
	"context"
	"math"
	"mime"
	"net/http"
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
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	"github.com/QuantumNous/new-api/types"

	"github.com/go-redis/redis/v8"
	"github.com/tidwall/gjson"
	"gorm.io/gorm"
)

// TrackFirstVideoStart schedules a milestone after the initial task write.
func TrackFirstVideoStart(task *model.Task, entry string) {
	trackVideoMilestone(task, analytics.EventFirstVideoStart, entry)
}

// TrackFirstVideoSuccess schedules a milestone after a committed success.
func TrackFirstVideoSuccess(task *model.Task) {
	trackVideoMilestone(task, analytics.EventFirstVideoSuccess, "")
}

func trackVideoMilestone(task *model.Task, name, entry string) {
	if task == nil || task.ID <= 0 || task.UserId <= 0 || task.TaskID == "" {
		return
	}
	video, known := analyticsVideoSubmission(task)
	if known && !video {
		return
	}
	if name == analytics.EventFirstVideoSuccess || !video {
		if task.Status != model.TaskStatusSuccess || !task.ResultRetrievable() {
			return
		}
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
		Where("user_id = ? AND task_id <> ''", userID)
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

// analyticsVideoSubmission classifies the committed request. Older tasks with
// unknown provenance can still qualify through independently typed output.
func analyticsVideoSubmission(task *model.Task) (video, known bool) {
	switch task.Platform {
	case constant.TaskPlatformMidjourney, constant.TaskPlatformSuno, constant.TaskPlatformVoiceClone:
		return false, true
	}
	switch task.Action {
	case "influencer_recreate_analysis", "influencer_sheet", constant.TaskActionImageEdit, constant.TaskActionVoiceClone, "MUSIC", "LYRICS":
		return false, true
	case constant.TaskActionInfluencerMotion:
		return true, true
	}

	requestPath := ""
	execution := task.PrivateData.Execution
	if execution != nil {
		requestPath = execution.RequestPath
	}
	if requestPath == "" && task.PrivateData.GenerationQueue != nil {
		requestPath = task.PrivateData.GenerationQueue.Path
	}
	protocol, _, _ := jsplugin.LookupHostProtocolOperation(http.MethodPost, requestPath)
	if protocol == jsplugin.ProtocolOpenAIImage || requestPath == "/pg/images/generations" || requestPath == "/pg/images/edits" {
		return false, true
	}

	contract, knownModel := modelcontracts.For(analyticsVideoModel(task))
	if !knownModel {
		contract, knownModel = modelcontracts.For(strings.TrimSpace(task.Properties.UpstreamModelName))
	}
	if knownModel {
		action := constant.NormalizeTaskAction(task.Action)
		for _, operation := range contract.Operations {
			if operation.ID == action && operation.Output.Type != "" {
				return operation.Output.Type == "video", true
			}
		}
		return contract.Family == "video", true
	}

	if protocol == "openai_video" {
		return true, true
	}
	if requestPath == "/v1/video/generations" || requestPath == "/pg/video/generations" {
		return true, true
	}
	if origin, ok := strings.CutPrefix(requestPath, "/v1/videos/"); ok {
		if id, remix := strings.CutSuffix(origin, "/remix"); remix && id != "" && !strings.Contains(id, "/") {
			return true, true
		}
	}
	if execution == nil || execution.TaskPlugin == nil || execution.TaskPlugin.Key == "" ||
		execution.TaskPlugin.APIVersion <= 0 || !strings.HasPrefix(execution.RequestPath, "/") {
		return false, false
	}
	// The host records the selected plugin and request path when it commits the
	// decoded task. They still identify the submission after a plugin is disabled.
	// Generic generate spans multiple media types, so keep it unclassified here.
	switch task.Action {
	case constant.TaskActionImageToVideo, constant.TaskActionTextToVideo, constant.TaskActionFirstTailToVideo,
		constant.TaskActionReferenceToVideo, constant.TaskActionRemix, "first_frame", "last_frame", "end_frame",
		"first_last_frame", "video_multi_ref", "video_edit", "video_extend", "video_extension",
		"document_to_video", "webpage_to_video":
		return true, true
	}
	return false, false
}

func analyticsVideoTaskQualifies(ctx context.Context, db *gorm.DB, task *model.Task, success bool) (bool, error) {
	video, known := analyticsVideoSubmission(task)
	if known && !video {
		return false, nil
	}
	if !success && video {
		return true, nil
	}
	if task.Status != model.TaskStatusSuccess || !task.ResultRetrievable() {
		return false, nil
	}
	metadata := observedAnalyticsVideoMetadata(task)
	if metadata.Width > 0 && metadata.Height > 0 && metadata.VideoDuration > 0 {
		return true, nil
	}
	raw := strings.TrimSpace(task.GetRawUpstreamURL())
	urlVideo, urlTyped := analyticsVideoResultURL(raw, task)
	if urlTyped && !urlVideo {
		return false, nil
	}
	channelType, _ := strconv.Atoi(string(task.Platform))
	directURL := analyticsDirectResultURL(raw, task) != nil
	nativeSnapshot := directURL && (channelType == constant.ChannelTypeFalAI || channelType == constant.ChannelTypeApiMart)
	pluginKey := ""
	if execution := task.PrivateData.Execution; execution != nil && execution.TaskPlugin != nil {
		pluginKey = execution.TaskPlugin.Key
	}
	if !nativeSnapshot && pluginKey == "" {
		return urlVideo, nil
	}
	var snapshot model.Task
	if err := db.WithContext(ctx).Select("data").Where("id = ?", task.ID).Take(&snapshot).Error; err != nil {
		return false, err
	}
	task.Data = snapshot.Data
	if nativeSnapshot {
		if videoResult, present := analyticsNativeVideoResult(channelType, task.Data, raw); present {
			return videoResult, nil
		}
	}
	// Explicit result fields take precedence over a video suffix. Some plugins
	// also persist a local proxy while their delivery URL stays in the snapshot.
	switch pluginKey {
	case "sora":
		return analyticsSoraVideoResult(task.Data, raw, urlVideo, video && directURL), nil
	case "xai":
		data := gjson.ParseBytes(task.Data)
		if status := data.Get("status").Str; status != "" {
			usable := status == "done" && data.Get("video.respect_moderation").Type != gjson.False
			return usable && analyticsVideoDeliveryLink(data.Get("video.url").Str, task), nil
		}
		return urlVideo, nil
	case "google":
		data := analyticsArtifactSnapshot(task.Data)
		if uri := data.Get("response.generateVideoResponse.generatedVideos.0.video.uri").Str; uri != "" {
			return analyticsVideoDeliveryLink(uri, task), nil
		}
		return urlVideo, nil
	case "doubao":
		data := analyticsArtifactSnapshot(task.Data)
		if data.Get("data").IsArray() {
			return false, nil
		}
		if uri := data.Get("content.video_url").Str; uri != "" {
			return video && analyticsVideoDeliveryLink(uri, task), nil
		}
		return urlVideo, nil
	}
	if pluginKey == "" || GetTaskAdaptorFunc == nil {
		return urlVideo, nil
	}
	// Other plugins may publish artifact types through the credential-free projection.
	provider, ok := GetTaskAdaptorFunc(task.Platform).(interface {
		ListArtifactsWithContext(context.Context, *model.Task) ([]types.TaskArtifact, error)
	})
	if !ok {
		return urlVideo, nil
	}
	artifacts, err := provider.ListArtifactsWithContext(ctx, task)
	if err != nil {
		return false, err
	}
	if len(artifacts) == 0 {
		return urlVideo, nil
	}
	return slices.ContainsFunc(artifacts, func(artifact types.TaskArtifact) bool {
		if artifact.Key == "" || artifact.Type != "video" {
			return false
		}
		if artifact.MimeType != "" {
			return analyticsVideoMIME(artifact.MimeType)
		}
		return urlVideo || (video && directURL)
	}), nil
}

// Some upstream gateways wrap the provider result in their task response.
func analyticsArtifactSnapshot(raw []byte) gjson.Result {
	data := gjson.ParseBytes(raw)
	if wrapper := data.Get("data"); wrapper.Get("task_id").Str != "" && wrapper.Get("data").Exists() {
		return wrapper.Get("data")
	}
	return data
}

func analyticsVideoMIME(value string) bool {
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.HasPrefix(mediaType, "video/") && len(mediaType) > len("video/")
}

// Native polling persists generic result URLs alongside the snapshot. A matched
// result gives the media type, including an explicit non-video MIME.
func analyticsNativeVideoResult(channelType int, data []byte, raw string) (video, present bool) {
	switch channelType {
	case constant.ChannelTypeFalAI:
		file := gjson.GetBytes(data, "video")
		if strings.TrimSpace(file.Get("url").Str) == raw {
			mediaType := strings.TrimSpace(file.Get("content_type").Str)
			return mediaType == "" || analyticsVideoMIME(mediaType), true
		}
	case constant.ChannelTypeApiMart:
		result := gjson.GetBytes(data, "data.result")
		if analyticsResultFieldMatches(result, raw, "videos.0", "videos.0.url", "videos.0.video_url") {
			return true, true
		}
		if analyticsResultFieldMatches(result, raw, "images.0", "images.0.url", "images.0.image_url",
			"music.0", "music.0.audio_url", "music.0.url", "music.0.video_url", "audio_url", "grid_image_url", "image_urls.0") {
			return false, true
		}
		if analyticsResultFieldMatches(result, raw, "video_url") {
			return true, true
		}
	}
	return false, false
}

func analyticsResultFieldMatches(result gjson.Result, raw string, fields ...string) bool {
	for _, field := range fields {
		value := result.Get(field)
		if value.IsArray() {
			value = value.Get("0")
		}
		if strings.TrimSpace(value.Str) == raw {
			return true
		}
	}
	return false
}

// Sora can project a video key from a status-only response. For an untyped
// link, require a recognized delivery field that matches the saved URL.
func analyticsSoraVideoResult(data []byte, raw string, urlVideo, opaqueVideo bool) bool {
	root := gjson.ParseBytes(data)
	payload := root.Get("data")
	if !payload.IsObject() {
		payload = root
	}
	if payload.Get("type").Str == "image" || payload.Get("object").Str == "image.generation.task" {
		return false
	}
	if firstAnalyticsArtifactURL(payload.Get("video_url")) == "" {
		for _, imageField := range []string{"results", "result_data", "result.images", "images", "data.images", "data"} {
			if firstAnalyticsArtifactURL(payload.Get(imageField)) != "" {
				return false
			}
		}
	}
	if urlVideo || !opaqueVideo {
		return urlVideo
	}
	for _, envelope := range []gjson.Result{payload, root} {
		for _, field := range []string{"video_url", "result_url", "url"} {
			if firstAnalyticsArtifactURL(envelope.Get(field)) == raw {
				return true
			}
		}
	}
	return false
}

func firstAnalyticsArtifactURL(value gjson.Result) string {
	if value.Type == gjson.String {
		return strings.TrimSpace(value.Str)
	}
	if value.IsArray() {
		var found string
		value.ForEach(func(_, item gjson.Result) bool {
			found = firstAnalyticsArtifactURL(item)
			return found == ""
		})
		return found
	}
	if value.IsObject() {
		for _, field := range []string{"url", "image_url", "video_url"} {
			if found := firstAnalyticsArtifactURL(value.Get(field)); found != "" {
				return found
			}
		}
	}
	return ""
}

// Some artifact projections identify video delivery URLs in the task snapshot
// while the host saves a proxy in ResultURL. Reject URLs typed as other media.
func analyticsVideoDeliveryLink(raw string, task *model.Task) bool {
	raw = strings.TrimSpace(raw)
	if analyticsDirectResultURL(raw, task) == nil {
		return false
	}
	video, typed := analyticsVideoResultURL(raw, task)
	return !typed || video
}

// A video family identifies the request. An opaque URL needs corroboration
// from observed media metadata or artifact projection.
func analyticsVideoResultURL(raw string, task *model.Task) (video, typed bool) {
	raw = strings.TrimSpace(raw)
	if dataURI, ok := strings.CutPrefix(raw, "data:"); ok {
		header, data, found := strings.Cut(dataURI, ",")
		mediaType, _, _ := strings.Cut(header, ";")
		return found && data != "" && analyticsVideoMIME(mediaType), true
	}
	parsed := analyticsDirectResultURL(raw, task)
	if parsed == nil {
		return false, false
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".mp4", ".webm", ".mov", ".m4v", ".mkv", ".avi":
		return true, true
	case ".aac", ".aif", ".aiff", ".flac", ".m4a", ".mp3", ".oga", ".opus", ".wav", ".weba",
		".avif", ".bmp", ".gif", ".heic", ".heif", ".ico", ".jpeg", ".jpg", ".png", ".svg", ".svgz", ".tif", ".tiff", ".webp",
		".csv", ".htm", ".html", ".json", ".pdf", ".txt", ".xml":
		return false, true
	default:
		return false, false
	}
}

func analyticsDirectResultURL(raw string, task *model.Task) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil || parsed == nil || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" ||
		(parsed.Scheme != "http" && parsed.Scheme != "https") {
		return nil
	}
	for _, id := range []string{task.TaskID, task.GetUpstreamTaskID()} {
		if strings.HasSuffix(parsed.Path, "/v1/videos/"+id+"/content") ||
			strings.HasSuffix(parsed.Path, "/v1/images/"+id+"/content") ||
			strings.Contains(parsed.Path, "/v1/tasks/"+id+"/artifacts/") {
			return nil
		}
	}
	return parsed
}

func observedAnalyticsVideoMetadata(task *model.Task) dto.InfluencerVideoMetadata {
	var metadata dto.InfluencerVideoMetadata
	if len(task.OutputVideoMetadata) != 0 && common.Unmarshal(task.OutputVideoMetadata, &metadata) != nil {
		return dto.InfluencerVideoMetadata{}
	}
	return metadata
}
