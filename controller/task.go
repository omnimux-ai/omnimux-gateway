package controller

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay"
	relaychannel "github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/sora"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
)

type taskArtifactResponse = dto.TaskArtifactDto

var (
	taskArtifactKeyPattern           = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._~-]{0,127}$`)
	errTaskArtifactPluginUnavailable = errors.New("task artifact plugin unavailable")
	errTaskArtifactPlugin            = errors.New("task artifact plugin error")
)

func GetTask(c *gin.Context) {
	task, exists, err := model.GetByTaskId(c.GetInt("id"), c.Param("key"))
	if err != nil {
		videoProxyError(c, http.StatusInternalServerError, "server_error", "Failed to query task")
		return
	}
	if !exists {
		videoProxyError(c, http.StatusNotFound, "invalid_request_error", "Task not found")
		return
	}
	createdAt := task.CreatedAt
	if createdAt == 0 {
		createdAt = task.SubmitTime
	}
	failReason := task.FailReason
	if task.Status == model.TaskStatusSuccess && taskFailReasonIsLegacyResultURL(task.FailReason) {
		failReason = ""
	}
	c.JSON(http.StatusOK, gin.H{
		"task_id":     task.TaskID,
		"platform":    task.Platform,
		"status":      task.Status,
		"progress":    task.Progress,
		"fail_reason": failReason,
		"created_at":  createdAt,
		"finished_at": task.FinishTime,

		"result_unverified": task.PrivateData.ImageResultUnverified,
	})
}

func GetTaskArtifacts(c *gin.Context) {
	task, exists, err := model.GetByTaskId(c.GetInt("id"), c.Param("key"))
	if err != nil {
		writeTaskArtifactError(c, http.StatusInternalServerError, "artifact_internal_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Task or artifact not found")
		return
	}
	writeTaskArtifacts(c, task, false)
}

func GetDashboardTaskArtifacts(c *gin.Context) {
	task, exists, err := getTaskForArtifactRequest(c, c.Param("task_id"))
	if err != nil {
		writeTaskArtifactError(c, http.StatusInternalServerError, "artifact_internal_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Task or artifact not found")
		return
	}
	writeTaskArtifacts(c, task, true)
}

func resolveTaskArtifacts(task *model.Task) ([]dto.TaskArtifactDto, string, error) {
	artifacts, err := projectTaskArtifacts(task)
	if err != nil {
		return nil, "", err
	}
	outputMetadata := taskOutputVideoMetadata(task)
	items := make([]dto.TaskArtifactDto, 0, len(artifacts))
	for _, artifact := range artifacts {
		contentURL, buildErr := service.BuildTaskArtifactContentURL(task.TaskID, artifact.Key)
		if buildErr != nil {
			return nil, "", buildErr
		}
		item := dto.TaskArtifactDto{
			Key:        artifact.Key,
			Type:       artifact.Type,
			MimeType:   artifact.MimeType,
			ContentURL: contentURL,
		}
		if artifact.Type == "video" {
			item.VideoMetadata = outputMetadata
		}
		items = append(items, item)
	}
	var legacyContentURL string
	if legacyVideoAvailable(task) {
		legacyURL, buildErr := service.BuildTaskArtifactContentURL(task.TaskID, "video")
		if buildErr != nil {
			return nil, "", buildErr
		}
		legacyContentURL = legacyURL
	}
	if legacyImageAvailable(task) {
		imageContentURL, buildErr := service.BuildTaskArtifactContentURL(task.TaskID, "image")
		if buildErr != nil {
			return nil, "", buildErr
		}
		items = append(items, dto.TaskArtifactDto{
			Key:        "image",
			Type:       "image",
			MimeType:   "image/png",
			ContentURL: imageContentURL,
		})
	}
	return items, legacyContentURL, nil
}

// taskOutputVideoMetadata decodes the server-observed media facts of a
// generated output video. Historical rows and rows without a successful probe
// stay NULL, so their artifact responses are byte-for-byte unchanged.
func taskOutputVideoMetadata(task *model.Task) *dto.InfluencerVideoMetadata {
	if task == nil || len(task.OutputVideoMetadata) == 0 {
		return nil
	}
	var metadata dto.InfluencerVideoMetadata
	if err := common.Unmarshal(task.OutputVideoMetadata, &metadata); err != nil {
		return nil
	}
	if metadata.Duration <= 0 || metadata.Width <= 0 || metadata.Height <= 0 {
		return nil
	}
	return &metadata
}

func writeTaskArtifacts(c *gin.Context, task *model.Task, dashboard bool) {
	c.Header("Cache-Control", "private, no-store")
	items, legacyContentURL, err := resolveTaskArtifacts(task)
	if err != nil {
		writeTaskArtifactProjectionError(c, err)
		return
	}
	populateTaskImageThumbnails([]*model.Task{task}, [][]dto.TaskArtifactDto{items})
	response := gin.H{"task_id": task.TaskID, "artifacts": items}
	if legacyContentURL != "" {
		response["legacy_content_url"] = legacyContentURL
	}
	if posterURL := storedTaskPosterURL(task); posterURL != "" {
		response["poster_url"] = posterURL
	}
	if dashboard && task.Status == model.TaskStatusSuccess && task.Platform == constant.TaskPlatformSuno {
		response["legacy_audio_clips"] = legacySunoAudioClips(task.Data)
	}
	if dashboard {
		common.ApiSuccess(c, response)
		return
	}
	c.JSON(http.StatusOK, response)
}

// legacySunoAudioClips projects a pre-plugin Suno task's persisted snapshot
// into the clips the dashboard audio preview renders. Task lists no longer
// carry the snapshot, so the dashboard reads it here on demand. The snapshot
// is either a clip array or a JSON string holding one; only clips with an
// audio URL are kept and only preview fields are exposed.
func legacySunoAudioClips(data json.RawMessage) []map[string]any {
	clips := make([]map[string]any, 0)
	if len(data) == 0 {
		return clips
	}
	var items []map[string]any
	if err := common.Unmarshal(data, &items); err != nil {
		var encoded string
		if common.Unmarshal(data, &encoded) != nil || common.UnmarshalJsonStr(encoded, &items) != nil {
			return clips
		}
	}
	for _, item := range items {
		audioURL, _ := item["audio_url"].(string)
		if strings.TrimSpace(audioURL) == "" {
			continue
		}
		clip := map[string]any{"audio_url": audioURL}
		for _, key := range []string{"clip_id", "id", "title", "tags", "duration", "image_url", "image_large_url", "metadata"} {
			if value, ok := item[key]; ok {
				clip[key] = value
			}
		}
		clips = append(clips, clip)
	}
	return clips
}

func projectTaskArtifacts(task *model.Task) ([]relaychannel.TaskArtifact, error) {
	if task == nil || task.Status != model.TaskStatusSuccess || !taskHasPluginExecution(task) || !task.ResultRetrievable() {
		return []relaychannel.TaskArtifact{}, nil
	}
	adaptor := relay.GetTaskAdaptor(task.Platform)
	if adaptor == nil {
		return nil, errTaskArtifactPluginUnavailable
	}
	provider, ok := adaptor.(relaychannel.TaskArtifactProvider)
	if !ok {
		return []relaychannel.TaskArtifact{}, nil
	}
	artifacts, err := provider.ListArtifacts(task)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errTaskArtifactPlugin, err)
	}
	return validateProjectedTaskArtifacts(artifacts)
}

func validateProjectedTaskArtifacts(artifacts []relaychannel.TaskArtifact) ([]relaychannel.TaskArtifact, error) {
	if len(artifacts) > 64 {
		return nil, fmt.Errorf("%w: too many artifacts", errTaskArtifactPlugin)
	}
	seen := make(map[string]struct{}, len(artifacts))
	for i := range artifacts {
		if artifacts[i].Key != strings.TrimSpace(artifacts[i].Key) ||
			artifacts[i].Type != strings.TrimSpace(artifacts[i].Type) {
			return nil, fmt.Errorf("%w: invalid artifact identity", errTaskArtifactPlugin)
		}
		if !taskArtifactKeyPattern.MatchString(artifacts[i].Key) {
			return nil, fmt.Errorf("%w: invalid artifact key", errTaskArtifactPlugin)
		}
		if _, exists := seen[artifacts[i].Key]; exists {
			return nil, fmt.Errorf("%w: duplicate artifact key", errTaskArtifactPlugin)
		}
		seen[artifacts[i].Key] = struct{}{}
		switch artifacts[i].Type {
		case "video", "audio", "image", "file":
		default:
			return nil, fmt.Errorf("%w: invalid artifact type", errTaskArtifactPlugin)
		}
		if len(artifacts[i].MimeType) > 255 || strings.ContainsAny(artifacts[i].MimeType, "\r\n") {
			return nil, fmt.Errorf("%w: invalid artifact mime type", errTaskArtifactPlugin)
		}
	}
	return artifacts, nil
}

func initTaskArtifactAdaptor(task *model.Task) (relaychannel.TaskAdaptor, error) {
	if task == nil || !taskHasPluginExecution(task) {
		return nil, errTaskArtifactPluginUnavailable
	}
	channelModel, err := model.CacheGetChannel(task.ChannelId)
	if err != nil {
		return nil, fmt.Errorf("%w: channel unavailable", errTaskArtifactPluginUnavailable)
	}
	adaptor := relay.GetTaskAdaptor(task.Platform)
	if adaptor == nil {
		return nil, errTaskArtifactPluginUnavailable
	}
	pluginKey := task.PrivateData.Key
	if pluginKey == "" {
		pluginKey = channelModel.Key
	}
	baseURL := channelModel.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.GetChannelBaseURL(channelModel.Type)
	}
	adaptor.Init(&relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelType:    channelModel.Type,
			ChannelBaseUrl: baseURL,
			ApiKey:         pluginKey,
			ChannelSetting: channelModel.GetSetting(),
		},
	})
	return adaptor, nil
}

func taskHasPluginExecution(task *model.Task) bool {
	return task != nil &&
		task.PrivateData.Execution != nil &&
		task.PrivateData.Execution.TaskPlugin != nil &&
		strings.TrimSpace(task.PrivateData.Execution.TaskPlugin.Key) != ""
}

func legacyVideoAvailable(task *model.Task) bool {
	if task == nil || task.Status != model.TaskStatusSuccess ||
		taskHasPluginExecution(task) || task.Platform == constant.TaskPlatformSuno ||
		strings.TrimSpace(task.GetRawUpstreamURL()) == "" {
		return false
	}
	modelName := strings.ToLower(task.Properties.OriginModelName)
	if modelName == "" {
		modelName = strings.ToLower(task.Properties.UpstreamModelName)
	}
	if strings.Contains(modelName, "image") || sora.IsAsyncImageModel(modelName) || task.Platform == constant.TaskPlatformMidjourney {
		return false
	}
	// fork: influencer motion video — workbench video tasks keep their own
	// action so the gallery can list them, and serve like any video task.
	if task.Action == ActionInfluencerMotion {
		return true
	}
	switch constant.NormalizeTaskAction(task.Action) {
	case constant.TaskActionImageToVideo,
		constant.TaskActionTextToVideo,
		constant.TaskActionFirstTailToVideo,
		constant.TaskActionReferenceToVideo,
		constant.TaskActionRemix:
		return true
	default:
		return false
	}
}

func legacyImageAvailable(task *model.Task) bool {
	if task == nil || task.Status != model.TaskStatusSuccess ||
		taskHasPluginExecution(task) || task.Platform == constant.TaskPlatformSuno ||
		strings.TrimSpace(task.GetRawUpstreamURL()) == "" {
		return false
	}
	switch task.Action {
	case constant.TaskActionImageToVideo,
		constant.TaskActionTextToVideo,
		constant.TaskActionFirstTailToVideo,
		constant.TaskActionReferenceToVideo,
		constant.TaskActionRemix,
		constant.TaskActionTextGenerate,
		constant.TaskActionFirstTailGenerate,
		constant.TaskActionReferenceGenerate:
		return false
	}
	modelName := strings.ToLower(task.Properties.OriginModelName)
	if modelName == "" {
		modelName = strings.ToLower(task.Properties.UpstreamModelName)
	}
	if strings.Contains(modelName, "video") {
		return false
	}
	if strings.Contains(modelName, "image") || sora.IsAsyncImageModel(modelName) || task.Platform == constant.TaskPlatformMidjourney {
		return true
	}
	if task.Action == constant.TaskActionImageEdit || task.Action == "image" {
		return true
	}
	rawURL := strings.ToLower(task.GetRawUpstreamURL())
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".gif"} {
		if strings.Contains(rawURL, ext) {
			return true
		}
	}
	return false
}

func getTaskForArtifactRequest(c *gin.Context, taskID string) (*model.Task, bool, error) {
	if middleware.IsTaskArtifactAccess(c) {
		task, exists, err := model.GetUniqueByOnlyTaskId(taskID)
		if err != nil || !exists || task == nil {
			return task, exists, err
		}
		owner, err := model.GetUserCache(task.UserId)
		if err != nil || owner == nil || owner.Status != common.UserStatusEnabled {
			return nil, false, err
		}
		return task, true, nil
	}
	if c.GetInt("token_id") == 0 && c.GetInt("role") >= common.RoleAdminUser {
		return model.GetByOnlyTaskId(taskID)
	}
	return model.GetByTaskId(c.GetInt("id"), taskID)
}

func writeTaskArtifactProjectionError(c *gin.Context, err error) {
	if errors.Is(err, errTaskArtifactPluginUnavailable) {
		writeTaskArtifactError(c, http.StatusServiceUnavailable, "artifact_plugin_unavailable", "Artifact preview plugin is unavailable")
		return
	}
	writeTaskArtifactError(c, http.StatusInternalServerError, "artifact_plugin_error", "Artifact preview plugin failed")
}

func writeTaskArtifactError(c *gin.Context, status int, code, message string) {
	c.Header("Cache-Control", "private, no-store")
	if middleware.IsTaskArtifactAccess(c) {
		status = http.StatusNotFound
		code = "artifact_not_found"
		message = "Task or artifact not found"
	}
	if strings.HasPrefix(c.Request.URL.Path, "/api/") {
		c.JSON(status, gin.H{"success": false, "code": code, "message": message})
		return
	}
	c.JSON(status, gin.H{
		"error": gin.H{
			"message": message,
			"type":    code,
			"code":    code,
		},
	})
}

func TaskArtifactContent(c *gin.Context) {
	task, exists, err := getTaskForArtifactRequest(c, c.Param("key"))
	if err != nil {
		writeTaskArtifactError(c, http.StatusInternalServerError, "artifact_internal_error", "Failed to query task")
		return
	}
	if !exists || task == nil {
		writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Task or artifact not found")
		return
	}
	artifactKey := strings.TrimSpace(c.Param("artifact_key"))
	if !taskArtifactKeyPattern.MatchString(artifactKey) || !task.ResultRetrievable() {
		writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Task or artifact not found")
		return
	}
	if task.Status != model.TaskStatusSuccess {
		writeTaskArtifactError(c, http.StatusConflict, "artifact_not_ready", "Task artifacts are not ready")
		return
	}
	if serveTaskImageThumbnail(c, task, artifactKey) {
		return
	}
	artifactStore := service.GetTaskArtifactStore()
	if !taskHasPluginExecution(task) {
		isLegacyVideo := artifactKey == "video" && legacyVideoAvailable(task)
		isLegacyImage := artifactKey == "image" && legacyImageAvailable(task)
		if !isLegacyVideo && !isLegacyImage {
			writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Task or artifact not found")
			return
		}
		// fork: 已回存的对象直接由自家存储下发，不再依赖上游临时链接。
		if ref, resolveErr := artifactStore.Resolve(task, artifactKey); resolveErr == nil && ref != nil {
			if serveErr := artifactStore.Serve(c, task, ref); serveErr != nil && !c.Writer.Written() {
				writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Artifact storage is unavailable")
			}
			return
		}
		descriptor := &relaychannel.TaskContentRequest{
			URL:            task.GetRawUpstreamURL(),
			Method:         c.Request.Method,
			Credentialless: true,
		}
		artifactType := "image"
		if isLegacyVideo {
			artifactType = "video"
		}
		if persistAndServeTaskArtifact(c, task, descriptor, artifactKey, artifactType) {
			return
		}
		if err := proxyTaskMedia(c, task, descriptor); err != nil {
			writeTaskMediaProxyError(c, err)
		}
		return
	}
	artifacts, err := projectTaskArtifacts(task)
	if err != nil {
		writeTaskArtifactProjectionError(c, err)
		return
	}
	var matched *relaychannel.TaskArtifact
	for i := range artifacts {
		if artifacts[i].Key == artifactKey {
			matched = &artifacts[i]
			break
		}
	}
	if matched == nil {
		// fork: the stored cover belongs to us, not to the task plugin, so it is
		// served straight from object storage when it exists.
		if artifactKey != service.TaskArtifactPosterKey || !service.IsTaskPosterStored(task) {
			writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Task or artifact not found")
			return
		}
		if ref, resolveErr := artifactStore.Resolve(task, artifactKey); resolveErr == nil && ref != nil {
			if serveErr := artifactStore.Serve(c, task, ref); serveErr != nil && !c.Writer.Written() {
				writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Artifact storage is unavailable")
			}
			return
		}
		writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Task or artifact not found")
		return
	}
	if ref, resolveErr := artifactStore.Resolve(task, artifactKey); resolveErr == nil && ref != nil {
		if serveErr := artifactStore.Serve(c, task, ref); serveErr != nil && !c.Writer.Written() {
			writeTaskArtifactError(c, http.StatusNotFound, "artifact_not_found", "Artifact storage is unavailable")
		}
		return
	}

	adaptor, err := initTaskArtifactAdaptor(task)
	if err != nil {
		writeTaskArtifactProjectionError(c, err)
		return
	}
	provider, ok := adaptor.(relaychannel.TaskContentRequestProvider)
	if !ok {
		writeTaskArtifactError(c, http.StatusServiceUnavailable, "artifact_plugin_unavailable", "Artifact content plugin is unavailable")
		return
	}
	clientRequest := relaychannel.TaskArtifactClientRequest{
		Method:  c.Request.Method,
		Headers: taskArtifactClientHeaders(c.Request.Header),
	}
	descriptor, err := provider.BuildContentRequest(c.Request.Context(), task, artifactKey, clientRequest)
	if err != nil || descriptor == nil {
		if err != nil {
			logger.LogWarn(c.Request.Context(), fmt.Sprintf("Failed to resolve plugin %s content for task %s: %v", artifactKey, task.TaskID, err))
		}
		writeTaskArtifactError(c, http.StatusInternalServerError, "artifact_plugin_error", "Artifact content plugin failed")
		return
	}
	if persistAndServeTaskArtifact(c, task, descriptor, artifactKey, matched.Type) {
		return
	}
	if err := proxyTaskMedia(c, task, descriptor); err != nil {
		writeTaskMediaProxyError(c, err)
	}
}

// persistAndServeTaskArtifact buffers one artifact from the upstream
// descriptor, writes it into our own object storage, and answers the client
// from the buffered bytes. Returns true only when a full response was
// written; every failure path falls through to proxyTaskMedia unchanged.
// Images were the first case (small, and their upstream links expire); videos
// join them so a replay never waits on the provider again. A video also warms
// its first-frame cover, which is what the workbench grid paints before hover.
func persistAndServeTaskArtifact(c *gin.Context, task *model.Task, descriptor *relaychannel.TaskContentRequest, artifactKey, artifactType string) bool {
	if c.Request.Method != http.MethodGet {
		return false
	}
	// Range and conditional requests can return 206 or 304, which cannot be
	// persisted as a complete artifact. Proxy them once instead of fetching
	// them here and then repeating the same request on the fallback path.
	if len(taskArtifactClientHeaders(c.Request.Header)) != 0 {
		return false
	}
	var maxBytes int64
	switch artifactType {
	case "image":
		maxBytes = service.MaxTaskArtifactPersistBytes
	case "video":
		maxBytes = maxTaskVideoPersistBytes
	default:
		return false
	}
	buffered, err := fetchTaskMediaForPersist(c, task, descriptor, maxBytes)
	if err != nil || len(buffered.data) == 0 {
		return false
	}
	ref, persistErr := service.PersistTaskArtifact(c.Request.Context(), task, artifactKey, buffered.mimeType, buffered.data)
	if persistErr != nil {
		logger.LogWarn(c.Request.Context(), fmt.Sprintf("task %s artifact %s persist failed: %v", task.TaskID, artifactKey, persistErr))
		return false
	}
	if artifactType == "video" {
		// The cover and the output media facts both derive from the bytes
		// buffered here and must not delay the first frame the user asked for.
		enqueueTaskVideoDerivedWork(task, buffered.data)
	}
	if serveErr := service.GetTaskArtifactStore().Serve(c, task, ref); serveErr != nil {
		logger.LogError(c.Request.Context(), fmt.Sprintf("task %s artifact %s serve failed: %v", task.TaskID, artifactKey, serveErr))
		return false
	}
	return true
}

func taskArtifactClientHeaders(headers http.Header) map[string]string {
	result := make(map[string]string, 4)
	for _, name := range []string{"Range", "If-Range", "If-None-Match", "If-Modified-Since"} {
		if value := strings.TrimSpace(headers.Get(name)); value != "" {
			result[name] = value
		}
	}
	return result
}

/*
	The task list handlers below deliberately do not call projectTaskArtifacts.
	Artifact projection is confined to the explicit endpoints above.
*/

func GetAllTask(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	withArtifacts, _ := strconv.ParseBool(c.Query("with_artifacts"))
	queryParams := model.SyncTaskQueryParams{Platform: constant.TaskPlatform(c.Query("platform")), TaskID: c.Query("task_id"), Status: c.Query("status"), Action: c.Query("action"), StartTimestamp: startTimestamp, EndTimestamp: endTimestamp, ChannelID: c.Query("channel_id"), WithArtifacts: withArtifacts}
	items := model.TaskGetAllTasks(pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	pageInfo.SetTotal(int(model.TaskCountAllTasks(queryParams)))
	pageInfo.SetItems(tasksToDto(items, true, c.GetInt("role"), withArtifacts))
	common.ApiSuccess(c, pageInfo)
}

func GetUserTask(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	userID := c.GetInt("id")
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	withArtifacts, _ := strconv.ParseBool(c.Query("with_artifacts"))
	queryParams := model.SyncTaskQueryParams{Platform: constant.TaskPlatform(c.Query("platform")), TaskID: c.Query("task_id"), Status: c.Query("status"), Action: c.Query("action"), StartTimestamp: startTimestamp, EndTimestamp: endTimestamp, WithArtifacts: withArtifacts}
	items := model.TaskGetAllUserTask(userID, pageInfo.GetStartIdx(), pageInfo.GetPageSize(), queryParams)
	pageInfo.SetTotal(int(model.TaskCountAllUserTask(userID, queryParams)))
	pageInfo.SetItems(tasksToDto(items, false, common.RoleCommonUser, withArtifacts))
	common.ApiSuccess(c, pageInfo)
}

func tasksToDto(tasks []*model.Task, fillUser bool, viewerRole int, withArtifacts ...bool) []*dto.TaskDto {
	withArt := len(withArtifacts) > 0 && withArtifacts[0]
	var userIDMap map[int]*model.UserBase
	if fillUser {
		userIDMap = make(map[int]*model.UserBase)
		userIDs := types.NewSet[int]()
		for _, task := range tasks {
			userIDs.Add(task.UserId)
		}
		for _, userID := range userIDs.Items() {
			if cacheUser, err := model.GetUserCache(userID); err == nil {
				userIDMap[userID] = cacheUser
			}
		}
	}
	result := make([]*dto.TaskDto, len(tasks))
	// fork: video tasks of this page feed the artifact preheater, and a stored
	// cover is advertised so the grid can paint it before playback.
	var videoTasks []*model.Task
	for i, task := range tasks {
		if fillUser {
			if user, ok := userIDMap[task.UserId]; ok {
				task.Username = user.Username
			}
		}
		item := relay.TaskModel2Dto(task)
		// The snapshot is only an internal input to artifact projection.
		item.Data = nil
		if !fillUser {
			item.ChannelId = 0
		}
		item.LegacyVideoAvailable = legacyVideoAvailable(task)
		item.ResultDiscarded = task.PrivateData.ResultDiscarded
		if task.Status == model.TaskStatusSuccess {
			item.ResultURL = ""
			if taskFailReasonIsLegacyResultURL(task.FailReason) {
				item.FailReason = ""
			}
			if withArt {
				if artifacts, legacyURL, err := resolveTaskArtifacts(task); err == nil {
					item.Artifacts = artifacts
					item.LegacyContentURL = legacyURL
					if taskArtifactsContainVideo(artifacts) {
						videoTasks = append(videoTasks, task)
					}
				}
			}
		}
		if viewerRole >= common.RoleAdminUser {
			adminInfo := &dto.TaskAdminInfo{}
			if execution := task.PrivateData.Execution; execution != nil {
				adminInfo.RequestID = execution.RequestID
				adminInfo.RequestPath = execution.RequestPath
				if snapshot := execution.TaskPlugin; snapshot != nil {
					adminInfo.TaskPlugin = &dto.TaskPluginInfo{
						Key:     snapshot.Key,
						Name:    snapshot.Name,
						Version: snapshot.Version,
					}
					if snapshot.Author != nil {
						adminInfo.TaskPlugin.Author = &dto.TaskPluginAuthorInfo{
							Name: snapshot.Author.Name,
							URL:  snapshot.Author.URL,
						}
					}
				}
			}
			if adminInfo.RequestID != "" || adminInfo.RequestPath != "" || adminInfo.TaskPlugin != nil {
				item.AdminInfo = adminInfo
			}
		}
		if viewerRole >= common.RoleRootUser {
			rootInfo := &dto.TaskRootInfo{
				UpstreamTaskID: task.PrivateData.UpstreamTaskID,
				NodeName:       task.PrivateData.NodeName,
			}
			if execution := task.PrivateData.Execution; execution != nil {
				if snapshot := execution.TaskPlugin; snapshot != nil {
					rootInfo.TaskPlugin = &dto.TaskPluginRuntimeInfo{
						Key:        snapshot.Key,
						Version:    snapshot.Version,
						APIVersion: snapshot.APIVersion,
						Generation: snapshot.Generation,
					}
				}
			}
			if rootInfo.TaskPlugin != nil || rootInfo.UpstreamTaskID != "" || rootInfo.NodeName != "" {
				item.RootInfo = rootInfo
			}
		}
		result[i] = item
	}
	if withArt {
		artifactLists := make([][]dto.TaskArtifactDto, len(result))
		for i := range result {
			artifactLists[i] = result[i].Artifacts
		}
		populateTaskImageThumbnails(tasks, artifactLists)
		for i := range result {
			for _, artifact := range result[i].Artifacts {
				if artifact.Type == "image" {
					result[i].ThumbnailURL = artifact.ThumbnailURL
					result[i].ThumbnailWidth = artifact.ThumbnailWidth
					result[i].ThumbnailHeight = artifact.ThumbnailHeight
					break
				}
			}
		}
	}
	if len(videoTasks) > 0 {
		posterURLs := storedTaskPosterURLs(videoTasks)
		for i := range result {
			if url, ok := posterURLs[result[i].TaskID]; ok {
				result[i].PosterURL = url
			}
		}
		scheduleTaskArtifactPreheat(videoTasks)
	}
	return result
}

func taskFailReasonIsLegacyResultURL(value string) bool {
	value = strings.TrimSpace(value)
	return len(value) >= len("https://") && strings.EqualFold(value[:len("https://")], "https://") ||
		len(value) >= len("http://") && strings.EqualFold(value[:len("http://")], "http://") ||
		len(value) >= len("data:") && strings.EqualFold(value[:len("data:")], "data:")
}
