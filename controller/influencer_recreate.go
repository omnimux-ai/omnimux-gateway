package controller

import (
	"context"
	"errors"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// fork: influencer viral-recreation analysis. The workbench posts the reference
// clip plus the caller's own reference images; the server decomposes the clip and
// compiles one Seedance prompt. The response is the only place the clip's
// structure is ever exposed to the browser, and the later generation request
// carries images and the prompt only.

// influencerRecreateMaxIntentRunes bounds the caller's free-text intent. It is
// planner context, never a provider parameter.
const influencerRecreateMaxIntentRunes = 1200

// influencerCreditsPerUSD is the product-wide display rule (1 USD = 10 credits).
// The recreation mode shares the motion surface's generation line
// (influencerMotionModel + the resolution tier groups), so there is one public
// generation model id for all three modes.
const influencerCreditsPerUSD = 10

// influencerRecreateQualityChoices is the only user-facing choice in this mode:
// quality decides the price, so the workbench offers exactly these.
var influencerRecreateQualityChoices = []string{"480p", "720p"}

// ActionInfluencerRecreateAnalysis marks a decomposition job in the shared
// generation queue. It is not a provider submission: the worker plans only.
const ActionInfluencerRecreateAnalysis = "influencer_recreate_analysis"

// influencerRecreateAnalysisQueuePath is the durable queue marker for this job.
const influencerRecreateAnalysisQueuePath = "/pg/influencer/recreate/analyze"

// influencerRecreatePlanner runs the three planning turns; tests replace it so
// the handler's request validation stays provider-free.
var influencerRecreatePlanner = service.PlanInfluencerRecreatePrompt

type influencerRecreateAnalyzeRequest struct {
	VideoURL   string   `json:"video_url"`
	ImageURLs  []string `json:"image_urls"`
	ImageTags  []string `json:"image_tags"`
	Prompt     string   `json:"prompt"`
	Resolution string   `json:"resolution"`
}

// PostInfluencerRecreateAnalyze decomposes the reference clip and returns the
// compiled prompt together with the structured breakdown. It is synchronous on
// purpose for the first version: the workbench shows a bounded progress state and
// the planner is the same internal, unbilled call the motion planner already uses.
func PostInfluencerRecreateAnalyze(c *gin.Context) {
	// The token-scoped route is a creation write; it follows the same switch as
	// every other command-line creation. The session route has no such switch.
	if c.GetBool("influencer_creation_api") && !requireInfluencerCLIWriteAvailability(c) {
		return
	}
	var req influencerRecreateAnalyzeRequest
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		influencerError(c, http.StatusBadRequest, "invalid_request", "参考视频参数无法解析")
		return
	}
	video := strings.TrimSpace(req.VideoURL)
	if !isInfluencerMediaURL(video) {
		influencerError(c, http.StatusBadRequest, "invalid_request", "请先上传参考复刻视频")
		return
	}
	images := make([]string, 0, len(req.ImageURLs))
	for _, raw := range req.ImageURLs {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			continue
		}
		if !isInfluencerMediaURL(trimmed) {
			influencerError(c, http.StatusBadRequest, "invalid_request", "参考图必须是已上传的公网地址")
			return
		}
		images = append(images, trimmed)
	}
	if len(images) == 0 {
		influencerError(c, http.StatusBadRequest, "invalid_request", "至少需要一张参考图")
		return
	}
	if len(images) > service.InfluencerRecreateMaxImages {
		influencerError(c, http.StatusBadRequest, "invalid_request", fmt.Sprintf("参考图最多 %d 张", service.InfluencerRecreateMaxImages))
		return
	}
	if len(req.ImageTags) > len(images) {
		influencerError(c, http.StatusBadRequest, "invalid_request", "参考图标签数量超出参考图")
		return
	}
	tags := make([]string, 0, len(req.ImageTags))
	for _, tag := range req.ImageTags {
		tag = strings.TrimSpace(tag)
		if utf8.RuneCountInString(tag) > influencerMotionMaxTagLen {
			influencerError(c, http.StatusBadRequest, "invalid_request", "参考图标签过长")
			return
		}
		tags = append(tags, tag)
	}
	intent := strings.TrimSpace(req.Prompt)
	if utf8.RuneCountInString(intent) > influencerRecreateMaxIntentRunes {
		influencerError(c, http.StatusBadRequest, "invalid_request", "创作意图过长")
		return
	}
	resolution := strings.TrimSpace(req.Resolution)
	if resolution == "" {
		resolution = "480p"
	}
	if _, ok := influencerMotionGroups[resolution]; !ok {
		influencerError(c, http.StatusBadRequest, "invalid_request", "分辨率必须是 480p 或 720p")
		return
	}
	if !influencerRecreateDailyAllow(c) {
		c.Header("Retry-After", "3600")
		influencerError(c, http.StatusTooManyRequests, "recreate_analysis_limit", "今天的参考视频拆解次数已用完，请明天再试")
		return
	}
	// The analysis runs as a durable queue job: three model turns take longer than
	// a proxied request may live, so the browser receives a task id and polls.
	task, err := enqueueInfluencerRecreateAnalysis(c, influencerRecreateAnalysisSnapshot{
		VideoURL:   video,
		ImageURLs:  images,
		ImageTags:  tags,
		Prompt:     intent,
		Resolution: resolution,
	})
	if err != nil {
		influencerError(c, http.StatusInternalServerError, "recreation_analysis_unavailable", "拆解任务暂时无法创建，请稍后重试")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"success": true,
		"data": gin.H{
			"task_id":  task.TaskID,
			"status":   "waiting",
			"progress": task.Progress,
		},
	})
}

// influencerRecreateAnalysisSnapshot is the durable, validated job input. It
// carries no credential and no provider parameter.
type influencerRecreateAnalysisSnapshot struct {
	VideoURL   string   `json:"video_url"`
	ImageURLs  []string `json:"image_urls"`
	ImageTags  []string `json:"image_tags"`
	Prompt     string   `json:"prompt"`
	Resolution string   `json:"resolution"`
}

// enqueueInfluencerRecreateAnalysis persists the job with the generation queue so
// claim, lease, recovery and per-account concurrency stay shared.
func enqueueInfluencerRecreateAnalysis(c *gin.Context, snapshot influencerRecreateAnalysisSnapshot) (*model.Task, error) {
	body, err := common.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	task := &model.Task{
		TaskID:     model.GenerateTaskID(),
		UserId:     c.GetInt("id"),
		Group:      c.GetString("group"),
		Action:     ActionInfluencerRecreateAnalysis,
		Status:     model.TaskStatusWaiting,
		Progress:   "0%",
		SubmitTime: time.Now().Unix(),
		Properties: model.Properties{Input: string(body), OriginModelName: influencerMotionModel},
		PrivateData: model.TaskPrivateData{GenerationQueue: &model.GenerationQueueRequest{
			Path: influencerRecreateAnalysisQueuePath,
			Body: body,
		}},
	}
	if err := task.InsertWithContext(c.Request.Context()); err != nil {
		return nil, err
	}
	if _, _, err := service.EnqueueSystemTask(model.SystemTaskTypeGenerationQueue, nil); err != nil {
		common.SysError("generation scanner wakeup failed")
	}
	return task, nil
}

// GetInfluencerRecreateAnalysis is the poll surface for one accepted job. It only
// reads the owner's row and never starts or repeats model work.
func GetInfluencerRecreateAnalysis(c *gin.Context) {
	taskID := strings.TrimSpace(c.Param("task_id"))
	if taskID == "" || len(taskID) > 191 {
		influencerError(c, http.StatusBadRequest, "invalid_request", "缺少拆解任务号")
		return
	}
	task, exists, err := model.GetByTaskId(c.GetInt("id"), taskID)
	if err != nil {
		influencerError(c, http.StatusInternalServerError, "internal_error", "拆解任务暂时无法读取")
		return
	}
	if !exists || task.Action != ActionInfluencerRecreateAnalysis {
		influencerError(c, http.StatusNotFound, "task_not_found", "拆解任务不存在")
		return
	}
	out := gin.H{"task_id": task.TaskID, "progress": task.Progress}
	switch task.Status {
	case model.TaskStatusSuccess:
		out["status"] = "success"
		var data map[string]any
		if common.Unmarshal(task.Data, &data) == nil {
			for key, value := range data {
				out[key] = value
			}
		}
	case model.TaskStatusFailure:
		out["status"] = "failed"
		out["message"] = task.FailReason
	default:
		out["status"] = "running"
	}
	common.ApiSuccess(c, out)
}

// runInfluencerRecreateAnalysis is the queue worker branch for one job. A failed
// plan is a user-visible failure, never a silent fallback: a wrong prompt still
// bills a video. Recovery resets CLAIMED rows, so an interrupted worker simply
// re-runs the absorbed planning turns.
func runInfluencerRecreateAnalysis(ctx context.Context, task *model.Task) {
	current, exists, err := model.GetByTaskId(task.UserId, task.TaskID)
	if err != nil || !exists || current.Status != model.TaskStatusClaimed || current.PrivateData.GenerationQueue == nil || task.PrivateData.GenerationQueue == nil || current.PrivateData.GenerationQueue.ClaimOwner != task.PrivateData.GenerationQueue.ClaimOwner {
		return
	}
	var snapshot influencerRecreateAnalysisSnapshot
	if err := common.UnmarshalJsonStr(string(current.PrivateData.GenerationQueue.Body), &snapshot); err != nil {
		_ = model.RejectGenerationDispatch(ctx, task, "拆解任务参数不可读取")
		return
	}
	plan, err := influencerRecreatePlanner(ctx, service.InfluencerRecreatePlanInput{
		VideoURL:   snapshot.VideoURL,
		Resolution: snapshot.Resolution,
		ImageURLs:  snapshot.ImageURLs,
		ImageTags:  snapshot.ImageTags,
		UserPrompt: snapshot.Prompt,
	})
	if err != nil {
		reason := err.Error()
		if errors.Is(err, service.ErrInfluencerRecreateTimeout) || errors.Is(err, context.DeadlineExceeded) {
			reason = "参考视频拆解超时，请稍后重试"
		}
		_ = model.RejectGenerationDispatch(ctx, task, reason)
		return
	}
	userGroup := "default"
	if user, userErr := model.GetUserById(task.UserId, false); userErr == nil && user.Group != "" {
		userGroup = user.Group
	}
	task.Status = model.TaskStatusSuccess
	task.Progress = "100%"
	task.FinishTime = time.Now().Unix()
	task.SetData(influencerRecreateAnalysisResult(plan, snapshot.Resolution, len(snapshot.ImageURLs), userGroup))
	if won, updateErr := task.UpdateWithStatus(model.TaskStatusClaimed); updateErr != nil || !won {
		common.SysError("influencer recreation analysis result could not be saved")
	}
}

// influencerRecreateAnalysisResult projects a plan into the poll payload.
func influencerRecreateAnalysisResult(plan service.InfluencerRecreatePlan, resolution string, imageCount int, userGroup string) map[string]any {
	analysis := any(nil)
	if strings.TrimSpace(plan.AnalysisJSON) != "" {
		var parsed map[string]any
		if common.UnmarshalJsonStr(plan.AnalysisJSON, &parsed) == nil {
			analysis = parsed
		}
	}
	imageAnalysis := any(nil)
	if strings.TrimSpace(plan.ImageJSON) != "" {
		var parsed []any
		if common.UnmarshalJsonStr(plan.ImageJSON, &parsed) == nil {
			imageAnalysis = parsed
		}
	}
	return map[string]any{
		"prompt":         plan.Prompt,
		"analysis":       analysis,
		"image_analysis": imageAnalysis,
		"meta": gin.H{
			"shot_count":             plan.ShotCount,
			"hook":                   plan.Hook,
			"risk_flags":             plan.RiskFlags,
			"duration_sec":           plan.DurationSec,
			"reference_duration_sec": plan.ReferenceSeconds,
			"duration_source":        "reference_video",
			"resolution":             resolution,
			"aspect_ratio":           influencerMotionAspectRatio,
			"image_count":            imageCount,
			"pricing":                influencerRecreatePricingFor(plan.DurationSec, userGroup),
		},
	}
}

// influencerRecreateGenerationOut builds the canonical submit body for a
// recreation generation. The request has no video and no audio by construction:
// the clip was decomposition material only and the prompt already carries its
// structure. The returned reason is empty when the request is usable.
func influencerRecreateGenerationOut(req influencerMotionRequest, resolution, prompt string, subjectURLs, imageTags []string) (map[string]any, map[string]any, *service.PreparedInfluencerVideo, string) {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		return nil, nil, nil, "爆款复刻需要先拆解参考视频并生成提示词"
	}
	if reason := service.InfluencerRecreatePromptGate(trimmed, len(subjectURLs)); reason != "" {
		return nil, nil, nil, "提示词不可用：" + reason
	}
	// The submit body repeats the analysis-derived length; the mode has no user
	// duration choice. The shared motion line accepts 4-30 whole seconds.
	seconds := req.DurationSec
	if seconds < service.InfluencerRecreateMinSeconds || seconds > service.InfluencerRecreateMaxSeconds {
		return nil, nil, nil, "爆款复刻时长跟随参考视频（4 到 30 秒）"
	}
	if req.AspectRatio != nil {
		if ratio := strings.TrimSpace(*req.AspectRatio); ratio != "" && ratio != influencerMotionAspectRatio {
			return nil, nil, nil, "爆款复刻只支持 9:16 画幅"
		}
	}
	// fork: the recreation now submits on the same audited line as motion/replace
	// (seedance-2-5, resolution tier group), so all three modes share one public
	// generation model id. The clip still never travels with this request; the
	// body is the 2.5 contract (string seconds, `images`) without `videos`.
	group := influencerMotionGroups[resolution]
	out := map[string]any{
		"model":        influencerMotionModel,
		"group":        group,
		"prompt":       trimmed,
		"seconds":      strconv.Itoa(seconds),
		"aspect_ratio": influencerMotionAspectRatio,
		"resolution":   resolution,
		"images":       subjectURLs,
	}
	meta := map[string]any{
		"kind":               influencerMotionKind,
		"mode":               influencerMotionModeRecreate,
		"prompt":             trimmed,
		"subject_image_url":  subjectURLs[0],
		"subject_image_urls": subjectURLs,
		"duration_sec":       seconds,
		"resolution":         resolution,
		"size":               influencerMotionAspectRatio,
	}
	if len(imageTags) > 0 {
		meta["subject_image_tags"] = imageTags
	}
	return out, meta, &service.PreparedInfluencerVideo{}, ""
}

// influencerRecreatePricing returns the credit amount per selectable quality for
// the derived duration. It reuses the APIMart adapter's own tier factors and the
// live model price, so the amount shown on the generate button is the amount the
// pre-charge will take instead of a parallel estimate.
func influencerRecreatePricing(c *gin.Context, seconds int) map[string]float64 {
	if seconds <= 0 {
		return nil
	}
	return influencerRecreatePricingFor(seconds, c.GetString("group"))
}

// influencerRecreatePricingFor follows the motion surface's per-second quote:
// the shared model price × the resolution tier group's ratio × seconds. The
// worker has no request context, so it passes the account's group.
func influencerRecreatePricingFor(seconds int, userGroup string) map[string]float64 {
	if seconds <= 0 {
		return nil
	}
	modelPrice, ok := ratio_setting.GetModelPrice(influencerMotionModel, false)
	if !ok || modelPrice <= 0 {
		return nil
	}
	pricing := make(map[string]float64, len(influencerRecreateQualityChoices))
	for _, resolution := range influencerRecreateQualityChoices {
		group := influencerMotionGroups[resolution]
		groupRatio := service.GetUserGroupRatio(userGroup, group)
		// Product rule: 1 USD = 10 credits.
		usd := modelPrice * float64(seconds) * groupRatio
		credits := usd * influencerCreditsPerUSD
		pricing[resolution] = math.Round(credits*100) / 100
	}
	if len(pricing) == 0 {
		return nil
	}
	return pricing
}

// influencerRecreateDailyLimit is the per-account analysis budget. The planner
// calls models that are absorbed into the video price, so a cap keeps one account
// from repeatedly spending on decomposition.
func influencerRecreateDailyLimit() int {
	raw := strings.TrimSpace(os.Getenv("INFLUENCER_RECREATE_DAILY_ANALYSES"))
	if raw == "" {
		return 20
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 20
	}
	return value
}

// influencerRecreateDailyAllow counts successful analysis attempts in shared
// Redis. It fails open on a Redis problem: a cache outage must not block a
// product surface, while the request-level limiter still applies.
func influencerRecreateDailyAllow(c *gin.Context) bool {
	limit := influencerRecreateDailyLimit()
	if limit <= 0 || !common.RedisEnabled || common.RDB == nil {
		return true
	}
	userID := c.GetInt("id")
	if userID <= 0 {
		return false
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()
	key := fmt.Sprintf("influencer:recreate:analysis:%d:%s", userID, time.Now().Format("20060102"))
	count, err := common.RDB.Incr(ctx, key).Result()
	if err != nil {
		return true
	}
	if count == 1 {
		common.RDB.Expire(ctx, key, 48*time.Hour)
	}
	return count <= int64(limit)
}
