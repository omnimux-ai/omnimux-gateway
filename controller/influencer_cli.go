package controller

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/service/influencer"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const influencerCLIWriteScope = "influencer_creation:write"

var influencerCLIRequestID = regexp.MustCompile(`^[A-Za-z0-9._:-]{8,128}$`)

// RegisterInfluencerCLIRoutes adds the owner-scoped product API to a UserAuth group.
// The existing playground routes remain session-only.
func RegisterInfluencerCLIRoutes(group *gin.RouterGroup) {
	read := middleware.RequireInfluencerCreation("read")
	write := middleware.RequireInfluencerCreation("write")
	group.Use(func(c *gin.Context) {
		c.Set("influencer_creation_api", true)
		c.Header("Cache-Control", "private, no-store")
	})
	group.GET("/options", read, GetInfluencerCLIOptions)
	group.POST("/characters", write, PostInfluencerCLICharacter)
	group.GET("/characters", read, GetInfluencerCLICharacters)
	group.GET("/characters/:task_id", read, GetInfluencerCLICharacter)
	group.POST("/characters/:task_id/multiview", write, PostInfluencerCLIMultiView)
	group.POST("/videos", write, PostInfluencerCLIVideo)
	// fork: viral recreation analysis — same planner as the workbench surface.
	group.POST("/recreations/analyze", write, PostInfluencerRecreateAnalyze)
	// The accepted job is polled by id; it performs no provider submission.
	group.GET("/recreations/:task_id", read, GetInfluencerRecreateAnalysis)
	group.GET("/tasks", read, GetInfluencerCLITasks)
	group.GET("/tasks/:task_id", read, GetInfluencerCLITask)
	group.GET("/tasks/:task_id/artifacts", read, GetInfluencerCLIArtifacts)
	group.GET("/requests/:request_id", read, GetInfluencerCLIRequest)
}

func influencerCLIWritesEnabled() bool {
	enabled, _ := strconv.ParseBool(os.Getenv("INFLUENCER_CLI_CREATION_ENABLED"))
	return enabled
}

func decodeInfluencerCLIRequest(c *gin.Context, request any, fields ...string) bool {
	var input map[string]any
	if err := common.UnmarshalBodyReusable(c, &input); err != nil || input == nil {
		influencerError(c, http.StatusBadRequest, "invalid_request", "a JSON object is required")
		return false
	}
	allowed := make(map[string]bool, len(fields))
	for _, field := range fields {
		allowed[field] = true
	}
	for field := range input {
		if !allowed[field] {
			influencerError(c, http.StatusBadRequest, "invalid_request", "unsupported product field: "+field)
			return false
		}
	}
	if err := common.UnmarshalBodyReusable(c, request); err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_request", "invalid product parameters")
		return false
	}
	return true
}

func influencerCLIHash(c *gin.Context, requestID string, intent any) (string, bool) {
	if !influencerCLIRequestID.MatchString(requestID) {
		influencerError(c, http.StatusBadRequest, "invalid_request", "request_id must contain 8 to 128 ASCII letters, digits, dots, underscores, colons or hyphens")
		return "", false
	}
	body, err := common.Marshal(intent)
	if err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_request", "invalid product parameters")
		return "", false
	}
	return fmt.Sprintf("%x", sha256.Sum256(body)), true
}

// Existing receipts are resolved before fetching materials or checking the write switch.
func replayInfluencerCLIRequest(c *gin.Context, requestID, payloadHash string) bool {
	receipt, err := model.GetInfluencerGenerationRequest(c.Request.Context(), c.GetInt("id"), requestID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false
	}
	if err != nil {
		if errors.Is(err, model.ErrInfluencerRequestExpired) && receipt != nil {
			c.JSON(http.StatusGone, gin.H{"success": false, "code": "creation_request_expired", "message": "the accepted request is outside its recovery period; this identifier cannot create another task", "operation_id": receipt.RequestID, "task_id": receipt.TaskID})
		} else {
			writeInfluencerCLIRequestError(c, err)
		}
		return true
	}
	if receipt.PayloadHash != payloadHash {
		writeInfluencerCLIRequestError(c, model.ErrInfluencerRequestConflict)
		return true
	}
	task, ok := ownedInfluencerCLITask(c, receipt.TaskID)
	if ok {
		writeInfluencerCLITask(c, task, receipt, true)
	}
	return true
}

func requireInfluencerCLIWriteAvailability(c *gin.Context) bool {
	if !influencerCLIWritesEnabled() {
		influencerError(c, http.StatusServiceUnavailable, "influencer_creation_disabled", "new command-line creations are currently disabled; accepted requests remain recoverable")
		return false
	}
	return true
}

func writeInfluencerCLIRequestError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, model.ErrInfluencerRequestConflict):
		influencerError(c, http.StatusConflict, "creation_request_conflict", "request_id already belongs to different creation inputs")
	case errors.Is(err, model.ErrInfluencerRequestExpired):
		influencerError(c, http.StatusGone, "creation_request_expired", "the accepted request is outside its recovery period; this identifier cannot create another task")
	case errors.Is(err, model.ErrInfluencerRequestUnauthorized):
		influencerError(c, http.StatusForbidden, "influencer_creation_scope_denied", "creation permission is no longer valid")
	case errors.Is(err, model.ErrInfluencerRequestInvalid):
		influencerError(c, http.StatusBadRequest, "invalid_request", "invalid creation request")
	case errors.Is(err, gorm.ErrRecordNotFound):
		influencerError(c, http.StatusNotFound, "creation_request_not_found", "request has not been accepted")
	default:
		influencerError(c, http.StatusServiceUnavailable, "generation_queue_unavailable", "creation records are temporarily unavailable")
	}
}

// The result retains prepared media for a committed or uncertain acceptance.
func acceptInfluencerCLIRequest(c *gin.Context, requestID, payloadHash, group, path, action string, out, meta map[string]any) bool {
	task, err := buildInfluencerGenerationTask(c.GetInt("id"), group, path, action, out, meta, c.GetString("influencer_reference_preflight"))
	if err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "failed to prepare creation")
		return false
	}
	task.PrivateData.GenerationQueue.Routing = generationRequestRouting(c, group, path)
	attachInfluencerImageInputs(c, task)
	task, replay, err := model.CommitInfluencerGenerationRequest(c.Request.Context(), task, requestID, payloadHash, c.GetInt("access_token_id"), influencerCLIWriteScope)
	if err != nil {
		writeInfluencerCLIRequestError(c, err)
		// A transport/commit failure may hide a durable acceptance. Keep tracked
		// media until its normal expiry; definitive rejections may discard it.
		retain := !errors.Is(err, model.ErrInfluencerRequestConflict) && !errors.Is(err, model.ErrInfluencerRequestExpired) && !errors.Is(err, model.ErrInfluencerRequestUnauthorized) && !errors.Is(err, model.ErrInfluencerRequestInvalid)
		c.Set("influencer_image_inputs_retained", retain)
		return retain
	}
	if !replay {
		c.Set("influencer_image_inputs_retained", true)
		// A scheduled scanner also discovers this durable row after process failure.
		if _, _, err := service.EnqueueSystemTask(model.SystemTaskTypeGenerationQueue, nil); err != nil {
			common.SysError("generation scanner wakeup failed")
		}
	}
	receipt := &model.InfluencerGenerationRequest{UserID: task.UserId, RequestID: requestID, TaskID: task.TaskID}
	writeInfluencerCLITask(c, task, receipt, replay)
	return !replay
}

func ownedInfluencerCLITask(c *gin.Context, taskID string) (*model.Task, bool) {
	task, exists, err := model.GetByTaskId(c.GetInt("id"), taskID)
	if err != nil {
		influencerError(c, http.StatusServiceUnavailable, "task_read_unavailable", "creation records are temporarily unavailable")
		return nil, false
	}
	if !exists || task == nil || !task.ResultRetrievable() || (task.Action != ActionInfluencerSheet && task.Action != ActionInfluencerMotion) {
		influencerError(c, http.StatusNotFound, "task_not_found", "creation not found")
		return nil, false
	}
	return task, true
}

func influencerCLITaskKind(task *model.Task) string {
	if task.Action == ActionInfluencerMotion {
		return "video"
	}
	meta, _ := parseInfluencerTaskMeta(task.Properties.Input)
	if meta.Kind == influencerTaskKindMultiView {
		return "multiview"
	}
	return "character"
}

func influencerCLITaskStatus(task *model.Task) string {
	if task.PrivateData.ImageResultUnverified && task.Status != model.TaskStatusSuccess && task.Status != model.TaskStatusFailure {
		return "reconciling"
	}
	switch task.Status {
	case model.TaskStatusWaiting:
		return "waiting"
	case model.TaskStatusClaimed:
		return "submitting"
	case model.TaskStatusDispatching:
		if task.FailReason != "" {
			return "reconciling"
		}
		return "submitting"
	case model.TaskStatusNotStart, model.TaskStatusSubmitted, model.TaskStatusQueued:
		return "queued"
	case model.TaskStatusInProgress:
		return "running"
	case model.TaskStatusSuccess:
		return "succeeded"
	case model.TaskStatusFailure:
		return "failed"
	default:
		return "unknown"
	}
}

// Only product inputs are projected. Frozen media, provider data and routing stay private.
func influencerCLIProductInput(task *model.Task) map[string]any {
	var meta map[string]any
	_ = common.UnmarshalJsonStr(task.Properties.Input, &meta)
	result := map[string]any{}
	fields := []string{"tier", "selection", "brief", "parent_task_id"}
	if task.Action == ActionInfluencerMotion {
		fields = []string{"mode", "prompt", "resolution", "duration_sec"}
		if value, ok := meta["size"]; ok {
			result["aspect_ratio"] = value
		}
		if sourceID, ok := meta["source_task_id"].(string); ok && sourceID != "" {
			result["character_task_id"] = sourceID
		}
	} else if value, ok := meta["image_url"]; ok && influencerCLITaskKind(task) != "multiview" {
		result["image_url"] = value
	}
	for _, field := range fields {
		if value, ok := meta[field]; ok {
			result[field] = value
		}
	}
	return result
}

func influencerCLIArtifacts(task *model.Task) ([]dto.TaskArtifactDto, error) {
	if task.Status != model.TaskStatusSuccess {
		return []dto.TaskArtifactDto{}, nil
	}
	items, legacyVideoURL, err := resolveTaskArtifacts(task)
	if err != nil {
		return nil, err
	}
	if legacyVideoURL != "" {
		items = append(items, dto.TaskArtifactDto{Key: "video", Type: "video", MimeType: "video/mp4", ContentURL: legacyVideoURL})
	}
	if items == nil {
		items = []dto.TaskArtifactDto{}
	}
	return items, nil
}

func projectInfluencerCLITask(task *model.Task, receipt *model.InfluencerGenerationRequest) (gin.H, error) {
	artifacts, err := influencerCLIArtifacts(task)
	if err != nil {
		return nil, err
	}
	artifactStatus := "pending"
	if task.Status == model.TaskStatusFailure || task.Status == model.TaskStatusSuccess {
		artifactStatus = "unavailable"
	}
	if task.Status == model.TaskStatusSuccess && len(artifacts) > 0 {
		artifactStatus = "ready"
	}
	expiresAt := int64(0)
	requestID := ""
	if receipt != nil {
		requestID = receipt.RequestID
		expiresAt = receipt.RecoveryExpiresAt
	}
	if (task.Status == model.TaskStatusFailure || task.Status == model.TaskStatusSuccess) && expiresAt == 0 && task.FinishTime > 0 {
		expiresAt = task.FinishTime + int64(7*24*time.Hour/time.Second)
	}
	failReason := task.FailReason
	if task.Status == model.TaskStatusSuccess && taskFailReasonIsLegacyResultURL(failReason) {
		failReason = ""
	}
	result := gin.H{"request_id": requestID, "task_id": task.TaskID, "kind": influencerCLITaskKind(task), "status": influencerCLITaskStatus(task), "raw_status": task.Status, "fail_reason": common.MaskSensitiveInfo(failReason), "artifact_status": artifactStatus, "input": influencerCLIProductInput(task), "recovery_expires_at": expiresAt, "created_at": task.SubmitTime, "finished_at": task.FinishTime, "artifacts": artifacts}
	result["result_unverified"] = task.PrivateData.ImageResultUnverified
	if meta, ok := parseInfluencerTaskMeta(task.Properties.Input); ok && meta.Parent != "" {
		result["parent_task_id"] = meta.Parent
	}
	if task.Quota > 0 {
		result["raw_quota"] = task.Quota
		result["credits"] = float64(task.Quota) / 50000
		result["quota_status"] = "recorded"
		if task.Status == model.TaskStatusFailure || task.PrivateData.ImageResultUnverified {
			result["quota_status"] = "reconciling"
		}
	}
	return result, nil
}

func influencerCLITaskReceipt(c *gin.Context, task *model.Task) (*model.InfluencerGenerationRequest, bool) {
	var receipt model.InfluencerGenerationRequest
	err := model.DB.WithContext(c.Request.Context()).Where("user_id = ? AND task_id = ?", task.UserId, task.TaskID).First(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, true
	}
	if err != nil {
		influencerError(c, http.StatusServiceUnavailable, "task_read_unavailable", "creation records are temporarily unavailable")
		return nil, false
	}
	return &receipt, true
}

func writeInfluencerCLITask(c *gin.Context, task *model.Task, receipt *model.InfluencerGenerationRequest, replay bool) {
	if receipt == nil {
		var ok bool
		receipt, ok = influencerCLITaskReceipt(c, task)
		if !ok {
			return
		}
	}
	data, err := projectInfluencerCLITask(task, receipt)
	if err != nil {
		operationID := ""
		if receipt != nil {
			operationID = receipt.RequestID
		}
		c.JSON(http.StatusServiceUnavailable, gin.H{"success": false, "code": "artifact_projection_unavailable", "message": "creation is recorded but its artifacts are temporarily unavailable", "operation_id": operationID, "task_id": task.TaskID})
		return
	}
	data["replayed"] = replay
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func GetInfluencerCLIRequest(c *gin.Context) {
	receipt, err := model.GetInfluencerGenerationRequest(c.Request.Context(), c.GetInt("id"), c.Param("request_id"))
	if err != nil {
		if errors.Is(err, model.ErrInfluencerRequestExpired) && receipt != nil {
			c.JSON(http.StatusGone, gin.H{"success": false, "code": "creation_request_expired", "message": "the accepted request is outside its recovery period; this identifier cannot create another task", "operation_id": receipt.RequestID, "task_id": receipt.TaskID})
		} else {
			writeInfluencerCLIRequestError(c, err)
		}
		return
	}
	if task, ok := ownedInfluencerCLITask(c, receipt.TaskID); ok {
		writeInfluencerCLITask(c, task, receipt, true)
	}
}

func GetInfluencerCLITask(c *gin.Context) {
	if task, ok := ownedInfluencerCLITask(c, c.Param("task_id")); ok {
		writeInfluencerCLITask(c, task, nil, false)
	}
}

func GetInfluencerCLIArtifacts(c *gin.Context) {
	GetInfluencerCLITask(c)
}

func influencerCLIOriginalCharacter(task *model.Task) bool {
	meta, valid := parseInfluencerTaskMeta(task.Properties.Input)
	return task.Action == ActionInfluencerSheet && task.Status == model.TaskStatusSuccess && valid && meta.Kind != influencerTaskKindMultiView && (meta.ImageMode == "" || meta.ImageMode == "text" || meta.ImageMode == "reference") && (meta.Tier == "" || influencer.ValidTier(meta.Tier)) && task.ResultRetrievable()
}

func GetInfluencerCLICharacter(c *gin.Context) {
	task, ok := ownedInfluencerCLITask(c, c.Param("task_id"))
	if !ok {
		return
	}
	if !influencerCLIOriginalCharacter(task) {
		influencerError(c, http.StatusNotFound, "character_not_found", "a completed original character was not found")
		return
	}
	artifacts, err := influencerCLIArtifacts(task)
	if err != nil {
		influencerError(c, http.StatusServiceUnavailable, "artifact_projection_unavailable", "character artifacts are temporarily unavailable")
		return
	}
	if !influencerCLIHasArtifact(artifacts, "image") {
		influencerError(c, http.StatusNotFound, "character_not_found", "character has no reusable image")
		return
	}
	receipt, ok := influencerCLITaskReceipt(c, task)
	if !ok {
		return
	}
	data, err := projectInfluencerCLITask(task, receipt)
	if err != nil {
		influencerError(c, http.StatusServiceUnavailable, "artifact_projection_unavailable", "character artifacts are temporarily unavailable")
		return
	}
	// Filtering persisted metadata in Go keeps legacy rows and all supported DBs compatible.
	var candidates []*model.Task
	if err := model.DB.WithContext(c.Request.Context()).Where("user_id = ? AND action = ?", c.GetInt("id"), ActionInfluencerSheet).Order("id DESC").Find(&candidates).Error; err != nil {
		influencerError(c, http.StatusServiceUnavailable, "task_read_unavailable", "character relations are temporarily unavailable")
		return
	}
	children := []gin.H{}
	for _, candidate := range candidates {
		meta, valid := parseInfluencerTaskMeta(candidate.Properties.Input)
		if !valid || meta.Kind != influencerTaskKindMultiView || meta.Parent != task.TaskID || !candidate.ResultRetrievable() {
			continue
		}
		receipt, ok := influencerCLITaskReceipt(c, candidate)
		if !ok {
			return
		}
		child, err := projectInfluencerCLITask(candidate, receipt)
		if err != nil {
			influencerError(c, http.StatusServiceUnavailable, "artifact_projection_unavailable", "character relations are temporarily unavailable")
			return
		}
		children = append(children, child)
	}
	data["multiviews"] = children
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func influencerCLIHasArtifact(artifacts []dto.TaskArtifactDto, kind string) bool {
	for _, artifact := range artifacts {
		if artifact.Type == kind && artifact.ContentURL != "" {
			return true
		}
	}
	return false
}

func GetInfluencerCLICharacters(c *gin.Context) { listInfluencerCLITasks(c, true) }
func GetInfluencerCLITasks(c *gin.Context)      { listInfluencerCLITasks(c, false) }

func listInfluencerCLITasks(c *gin.Context, charactersOnly bool) {
	page, pageSize := 1, 20
	var err error
	if c.Query("page") != "" {
		page, err = strconv.Atoi(c.Query("page"))
	}
	if err != nil || page < 1 || page > 1000000 {
		influencerError(c, http.StatusBadRequest, "invalid_request", "page must be a positive integer")
		return
	}
	if c.Query("page_size") != "" {
		pageSize, err = strconv.Atoi(c.Query("page_size"))
	}
	if err != nil || pageSize < 1 || pageSize > 100 {
		influencerError(c, http.StatusBadRequest, "invalid_request", "page_size must be between 1 and 100")
		return
	}
	kind, status := c.Query("kind"), c.Query("status")
	if kind != "" && kind != "character" && kind != "multiview" && kind != "video" {
		influencerError(c, http.StatusBadRequest, "invalid_request", "invalid creation kind")
		return
	}
	if status != "" && !slices.Contains([]string{"waiting", "submitting", "reconciling", "queued", "running", "succeeded", "failed", "unknown"}, status) {
		influencerError(c, http.StatusBadRequest, "invalid_request", "invalid creation status")
		return
	}
	query := model.DB.WithContext(c.Request.Context()).Where("user_id = ? AND action IN ?", c.GetInt("id"), []string{ActionInfluencerSheet, ActionInfluencerMotion})
	if charactersOnly {
		query = query.Where("action = ? AND status = ?", ActionInfluencerSheet, model.TaskStatusSuccess)
	}
	var candidates []*model.Task
	if err := query.Order("id DESC").Find(&candidates).Error; err != nil {
		influencerError(c, http.StatusServiceUnavailable, "task_read_unavailable", "creation records are temporarily unavailable")
		return
	}
	items, total := []gin.H{}, 0
	for _, task := range candidates {
		if !task.ResultRetrievable() || (charactersOnly && !influencerCLIOriginalCharacter(task)) || (kind != "" && influencerCLITaskKind(task) != kind) || (status != "" && influencerCLITaskStatus(task) != status) {
			continue
		}
		if charactersOnly {
			artifacts, err := influencerCLIArtifacts(task)
			if err != nil {
				influencerError(c, http.StatusServiceUnavailable, "artifact_projection_unavailable", "character artifacts are temporarily unavailable")
				return
			}
			if !influencerCLIHasArtifact(artifacts, "image") {
				continue
			}
		}
		total++
		if total <= (page-1)*pageSize || len(items) >= pageSize {
			continue
		}
		request, ok := influencerCLITaskReceipt(c, task)
		if !ok {
			return
		}
		item, err := projectInfluencerCLITask(task, request)
		if err != nil {
			influencerError(c, http.StatusServiceUnavailable, "artifact_projection_unavailable", "creation artifacts are temporarily unavailable")
			return
		}
		items = append(items, item)
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"items": items, "total": total, "page": page, "page_size": pageSize}})
}

func GetInfluencerCLIOptions(c *gin.Context) {
	imageAvailable := influencerCLIImageRouteAvailable(c, false)
	referenceAvailable := influencerCLIImageRouteAvailable(c, true)
	video := map[string]any{}
	for resolution, group := range influencerMotionGroups {
		video[resolution] = gin.H{"aspect_ratios": influencerMotionOutputRatios[resolution], "available": influencerCLIRouteAvailable(c, influencerMotionModel, group)}
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{
		"version": 1, "taxonomy": influencer.Taxonomy(), "taxonomy_etag": influencer.ETag(),
		"defaults":         gin.H{"tier": "normal", "mode": "motion", "resolution": "480p", "aspect_ratio": "9:16", "wait_timeout_seconds": 600},
		"creation_enabled": influencerCLIWritesEnabled(),
		"availability":     gin.H{"character": imageAvailable, "reference_image": referenceAvailable, "multiview": referenceAvailable},
		"character":        gin.H{"types": []string{"normal", "freak", "total", "cats", "dogs"}, "aspect_ratio": "16:9", "max_brief_characters": 4000},
		"video":            gin.H{"modes": []string{"motion", "replace"}, "resolutions": video, "min_seconds": 4, "max_seconds": 30, "min_images": 1, "max_images": 30, "max_prompt_characters": 4000, "preserves_reference_audio": true, "server_inspects_media": true},
		"media":            gin.H{"reference_image_max_bytes": 5 * 1024 * 1024, "reference_image_mime_types": []string{"image/jpeg", "image/png", "image/webp"}, "video_containers": []string{"mp4", "mov"}, "video_codecs": []string{"h264", "hevc"}, "min_dimension": 300, "max_dimension": 6000, "min_pixels": 407696, "max_pixels": 8295044, "min_fps": 24, "max_fps": 60, "min_aspect_ratio": 0.4, "max_aspect_ratio": 2.5},
	}})
}
