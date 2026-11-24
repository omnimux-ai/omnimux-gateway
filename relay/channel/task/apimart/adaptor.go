package apimart

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/modelcontracts"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	hosttypes "github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// TaskAdaptor implements APIMart async media:
//
//	POST /v1/videos/generations   (video: kling, seedance, …)
//	POST /v1/midjourney/generations (midjourney)
//	POST /v1/music/generations   (suno)
//	POST /v1/music/generations/sounds (suno sound effects)
//	GET  /v1/tasks/{task_id}     (unified poll)
//
// Docs: https://docs.apimart.ai/en

const (
	kindVideo       = "video"
	kindMidjourney  = "midjourney"
	kindMusic       = "music"
	kindSoundEffect = "sound"

	defaultBaseURL = "https://api.apimart.ai"
	// fork: upstream retired the v4 default (400: "Suno 旧版本 v4 已下线"), so the
	// injected default tracks the versions APIMart still serves.
	defaultSunoVersion    = "v6"
	contextKeyApimartKind = "apimart_task_kind"
)

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType  int
	baseURL      string
	apiKey       string
	managedVideo bool
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	if info != nil && info.TaskRelayInfo != nil {
		info.AutoDurationBilling = false
	}
	var raw map[string]any
	if err := common.UnmarshalBodyReusable(c, &raw); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	modelName := strings.TrimSpace(info.OriginModelName)
	if modelName == "" {
		if m, ok := raw["model"].(string); ok {
			modelName = strings.TrimSpace(m)
			info.OriginModelName = modelName
		}
	}
	if modelName == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model is required"), "invalid_request", http.StatusBadRequest)
	}

	kind := detectKind(modelName, requestPath(c))
	filter, managed := service.GetChannelConstraints(c).VideoFilter()
	a.managedVideo = managed && filter.VideoRequest != nil && filter.VideoRequest.Protocol == modelcontracts.VideoProtocolGenerations
	if a.managedVideo {
		kind = kindVideo
	}
	c.Set(contextKeyApimartKind, kind)
	info.Action = constant.TaskActionGenerate

	// Validate video contract, tier billing policy, and duration inputs using the
	// same estimator as EstimateBillingValidated so both stages agree fail-closed.
	if kind == kindVideo {
		if a.managedVideo {
			requestModel := modelcontracts.ActiveVideoCapabilities().NativeRequestModel(modelName)
			normalized, _, err := normalizeManagedVideoRequest(requestModel, raw)
			if err != nil {
				return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
			}
			raw = normalized
		} else if isPhaseOneVideoModel(modelName) {
			normalized, err := validatePhaseOneVideoRequest(modelName, raw)
			if err != nil {
				return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
			}
			raw = normalized
		}
		_, autoDuration, err := estimateVideoBillingForRequest(info, modelName, raw)
		if err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
		if info != nil && info.TaskRelayInfo != nil {
			info.AutoDurationBilling = autoDuration
		}
	}

	// Store raw for BuildRequestBody (preserves extra fields: duration, version, aspect_ratio, …)
	c.Set("task_request_raw", raw)
	return nil
}

func requestPath(c *gin.Context) string {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return ""
	}
	return c.Request.URL.Path
}

func estimateVideoBillingForRequest(info *relaycommon.RelayInfo, originModel string, body map[string]any) (map[string]float64, bool, error) {
	policy := ""
	if info != nil && info.ChannelMeta != nil {
		policy = info.ChannelOtherSettings.VideoTierBillingPolicy
	}
	origin := strings.TrimSpace(originModel)
	rateModel := origin
	if upstream := strings.TrimSpace(info.GetUpstreamModelName()); upstream != "" {
		rateModel = upstream
	}
	return estimateVideoBillingWithPolicy(policy, origin, rateModel, body)
}

func detectKind(modelName, path string) string {
	m := strings.ToLower(strings.TrimSpace(modelName))
	p := strings.ToLower(path)
	switch {
	// fork: sound effects are a sub-path of the Suno music family, so they must
	// be matched before the broader suno/music test below.
	case strings.Contains(m, "sounds") || strings.Contains(m, "sound-effect") || strings.Contains(m, "sfx") || strings.HasSuffix(p, "/music/generations/sounds"):
		return kindSoundEffect
	case strings.Contains(m, "suno") || strings.Contains(p, "/music") || strings.Contains(p, "/suno"):
		return kindMusic
	case strings.Contains(m, "midjourney") || strings.HasPrefix(m, "mj_") || strings.Contains(p, "/midjourney") || strings.Contains(p, "/mj"):
		return kindMidjourney
	default:
		return kindVideo
	}
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	ratios, err := a.EstimateBillingValidated(c, info)
	if err != nil {
		return nil
	}
	return ratios
}

func (a *TaskAdaptor) EstimateBillingValidated(c *gin.Context, info *relaycommon.RelayInfo) (map[string]float64, error) {
	if info == nil {
		return nil, fmt.Errorf("relay info is required")
	}
	if info.TaskRelayInfo != nil {
		info.AutoDurationBilling = false
	}
	// MJ / Suno music / Suno sound effects: ModelPrice is USD per call.
	kind := detectKind(info.OriginModelName, requestPath(c))
	if !a.managedVideo && (kind == kindMusic || kind == kindSoundEffect || kind == kindMidjourney) {
		return map[string]float64{"units": 1}, nil
	}

	// Video: ModelPrice is USD per second for the model base tier.
	// OtherRatios: seconds × optional tier (controlled by channel video_tier_billing_policy).
	var body map[string]any
	if c != nil {
		if raw, exists := c.Get("task_request_raw"); exists {
			body, _ = raw.(map[string]any)
		}
		if body == nil && c.Request != nil && c.Request.Body != nil {
			body = map[string]any{}
			if err := common.UnmarshalBodyReusable(c, &body); err != nil {
				return nil, err
			}
		}
	}
	ratios, autoDuration, err := estimateVideoBillingForRequest(info, info.OriginModelName, body)
	if err != nil {
		return nil, err
	}
	if info.TaskRelayInfo != nil {
		info.AutoDurationBilling = autoDuration
	}
	return ratios, nil
}

func (a *TaskAdaptor) ExtractUsageFacts(c *gin.Context, info *relaycommon.RelayInfo) map[string]any {
	facts, err := a.ExtractUsageFactsValidated(c, info)
	if err != nil {
		return nil
	}
	return facts
}

func (a *TaskAdaptor) ExtractUsageFactsValidated(c *gin.Context, info *relaycommon.RelayInfo) (map[string]any, error) {
	ratios, err := a.EstimateBillingValidated(c, info)
	if err != nil {
		return nil, err
	}
	if len(ratios) == 0 {
		return nil, nil
	}
	facts := make(map[string]any, len(ratios))
	for k, v := range ratios {
		facts[k] = v
	}
	return facts, nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := a.baseURL
	if base == "" {
		base = defaultBaseURL
	}
	if a.managedVideo {
		return base + "/v1/videos/generations", nil
	}
	// Kind is also recoverable from model name if context is unavailable.
	kind := detectKind(info.OriginModelName, "")
	switch kind {
	case kindSoundEffect:
		return base + "/v1/music/generations/sounds", nil
	case kindMusic:
		return base + "/v1/music/generations", nil
	case kindMidjourney:
		return base + "/v1/midjourney/generations", nil
	default:
		return base + "/v1/videos/generations", nil
	}
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	raw, _ := c.Get("task_request_raw")
	body, ok := raw.(map[string]any)
	if !ok || body == nil {
		body = map[string]any{}
		_ = common.UnmarshalBodyReusable(c, &body)
	}
	// Shallow copy so we do not mutate the cached map oddly
	out := make(map[string]any, len(body)+4)
	for k, v := range body {
		out[k] = v
	}

	upstream := ""
	if canonical, ok := canonicalPhaseOneVideoModel(info.OriginModelName); ok && !info.IsModelMapped {
		// The documented phase-one map supplies the default APIMart wire name.
		// A channel that sets an explicit model_mapping (IsModelMapped) overrides
		// this fallback so a deliberate SKU — e.g. seedance-2.5-text-to-video —
		// reaches upstream instead of the generic phase-one alias.
		upstream = phaseOneWireModels[canonical]
	} else {
		upstream = strings.TrimSpace(info.UpstreamModelName)
		if upstream == "" {
			upstream = strings.TrimSpace(info.OriginModelName)
		}
		upstream = mapUpstreamModel(upstream)
	}
	out["model"] = upstream

	kind := detectKind(info.OriginModelName, c.Request.URL.Path)
	if a.managedVideo {
		kind = kindVideo
	}
	if kind == kindMusic {
		if _, ok := out["version"]; !ok {
			out["version"] = defaultSunoVersion
		}
		// APIMart inspiration mode uses custom=false when only prompt is set
		if _, ok := out["custom"]; !ok {
			out["custom"] = false
		}
		// fork: ignore legacy duration in inspiration mode, where the upstream rejects it.
		if custom, ok := out["custom"].(bool); ok && !custom {
			delete(out, "duration")
		}
	}
	if kind == kindMidjourney {
		// Unified MJ route injects model server-side; still send model for billing clarity.
		if prompt, _ := out["prompt"].(string); strings.TrimSpace(prompt) == "" {
			return nil, fmt.Errorf("prompt is required for midjourney")
		}
	}
	if kind == kindVideo {
		delete(out, "operation")
		if isEvoLinkWanUpstream(upstream) {
			normalizeEvoLinkWanRequestBody(out)
		}
		// fork: Omni-Flash-Ext 的按次价只覆盖 ≤1080P 档，超档（4K / 超时长）在入口拒绝，避免上游计费高于售价。
		if isOmniFlashExtModel(upstream) || isOmniFlashExtModel(info.OriginModelName) {
			if err := validateOmniFlashExtCaps(out); err != nil {
				return nil, err
			}
		}
		if !a.managedVideo && !isPhaseOneVideoModel(info.OriginModelName) {
			if prompt, _ := out["prompt"].(string); strings.TrimSpace(prompt) == "" {
				return nil, fmt.Errorf("prompt is required for video generation")
			}
		}
	}

	data, err := common.Marshal(out)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

// mapUpstreamModel maps public OmniMux ids to APIMart model ids.
func mapUpstreamModel(name string) string {
	n := strings.TrimSpace(name)
	if canonical, ok := canonicalPhaseOneVideoModel(n); ok {
		return phaseOneWireModels[canonical]
	}
	switch strings.ToLower(n) {
	case "mj_imagine", "mj-imagine":
		return "midjourney"
	// fork: the public sound-effect id rides Suno's music upstream model.
	case "suno-sounds", "suno-sound-effect":
		return "suno"
	default:
		return n
	}
}

// isEvoLinkWanUpstream identifies the EvoLink Wan 3.0 wire SKUs. EvoLink's
// video API names its quality and duration fields `quality` and `duration`,
// while the gateway's task request surface uses `resolution` and `seconds`.
// Without this translation EvoLink silently falls back to its 720p/5s defaults,
// so the customer-facing resolution tier and the supplier cost diverge.
func isEvoLinkWanUpstream(name string) bool {
	return strings.Contains(strings.ToLower(strings.TrimSpace(name)), "wan3.0")
}

func normalizeEvoLinkWanRequestBody(body map[string]any) {
	if value, ok := body["resolution"]; ok {
		if value != nil {
			if _, exists := body["quality"]; !exists {
				body["quality"] = strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
			}
		}
		delete(body, "resolution")
	} else if value, ok := body["quality"]; ok && value != nil {
		body["quality"] = strings.ToLower(strings.TrimSpace(fmt.Sprint(value)))
	}
	if _, ok := body["duration"]; !ok {
		if value, ok := body["seconds"]; ok {
			body["duration"] = value
		}
	}
	delete(body, "seconds")
	if value, ok := body["size"]; ok {
		if _, exists := body["aspect_ratio"]; !exists && value != nil {
			if size := strings.TrimSpace(fmt.Sprint(value)); size != "" && size != "auto" {
				body["aspect_ratio"] = size
			}
		}
		delete(body, "size")
	}
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) ParseResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*channel.TaskSubmitResponse, *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	_ = resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, service.TaskErrorWrapper(
			fmt.Errorf("upstream status %d: %s", resp.StatusCode, string(responseBody)),
			"upstream_error",
			resp.StatusCode,
		)
	}

	upstreamID, err := parseSubmitTaskID(responseBody)
	if err != nil {
		return nil, service.TaskErrorWrapper(errors.Wrap(err, string(responseBody)), "invalid_upstream_response", http.StatusBadGateway)
	}

	return &channel.TaskSubmitResponse{
		UpstreamTaskID: upstreamID,
		TaskData:       responseBody,
	}, nil
}

func clientObject(kind string) string {
	switch kind {
	case kindMidjourney:
		return "image.generation.task"
	case kindMusic, kindSoundEffect:
		return "audio.generation.task"
	default:
		return "video"
	}
}

func parseSubmitTaskID(body []byte) (string, error) {
	var envelope struct {
		Code int `json:"code"`
		Data any `json:"data"`
		// Some gateways may return flat id
		TaskID string `json:"task_id"`
		ID     string `json:"id"`
		Error  *struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil {
		return "", err
	}
	if envelope.Error != nil && strings.TrimSpace(envelope.Error.Message) != "" {
		return "", fmt.Errorf("%s", envelope.Error.Message)
	}
	if id := strings.TrimSpace(envelope.TaskID); id != "" {
		return id, nil
	}
	if id := strings.TrimSpace(envelope.ID); id != "" {
		return id, nil
	}
	switch data := envelope.Data.(type) {
	case []any:
		if len(data) == 0 {
			return "", fmt.Errorf("empty data array")
		}
		if m, ok := data[0].(map[string]any); ok {
			if id, _ := m["task_id"].(string); strings.TrimSpace(id) != "" {
				return strings.TrimSpace(id), nil
			}
			if id, _ := m["id"].(string); strings.TrimSpace(id) != "" {
				return strings.TrimSpace(id), nil
			}
		}
	case map[string]any:
		if id, _ := data["task_id"].(string); strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id), nil
		}
		if id, _ := data["id"].(string); strings.TrimSpace(id) != "" {
			return strings.TrimSpace(id), nil
		}
	}
	return "", fmt.Errorf("task_id missing in upstream response")
}

func (a *TaskAdaptor) FetchTask(baseUrl, key string, task *model.Task, proxy string) (*http.Response, error) {
	if task == nil {
		return nil, fmt.Errorf("task is required")
	}
	taskID := task.GetUpstreamTaskID()
	if strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	baseUrl = strings.TrimRight(baseUrl, "/")
	if baseUrl == "" {
		baseUrl = defaultBaseURL
	}
	uri := fmt.Sprintf("%s/v1/tasks/%s", baseUrl, taskID)
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(task *model.Task, resp *http.Response, respBody []byte) (*relaycommon.TaskInfo, error) {
	var envelope struct {
		Code int `json:"code"`
		Data *struct {
			ID             string         `json:"id"`
			Status         string         `json:"status"`
			Progress       int            `json:"progress"`
			Result         map[string]any `json:"result"`
			Usage          map[string]any `json:"usage"`
			ActualDuration any            `json:"actual_duration"`
			VideoDuration  any            `json:"video_duration"`
			Error          *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"data"`
		// Flat fallbacks
		Status         string         `json:"status"`
		ID             string         `json:"id"`
		Result         map[string]any `json:"result"`
		Usage          map[string]any `json:"usage"`
		ActualDuration any            `json:"actual_duration"`
		VideoDuration  any            `json:"video_duration"`
	}
	if err := common.Unmarshal(respBody, &envelope); err != nil {
		return nil, errors.Wrap(err, "unmarshal apimart task")
	}

	info := &relaycommon.TaskInfo{Code: 0}
	status := strings.ToLower(strings.TrimSpace(envelope.Status))
	resultMap := envelope.Result
	var dataUsage map[string]any
	var dataDirect map[string]any
	if envelope.Data != nil {
		info.TaskID = strings.TrimSpace(envelope.Data.ID)
		status = strings.ToLower(strings.TrimSpace(envelope.Data.Status))
		if envelope.Data.Progress > 0 && envelope.Data.Progress < 100 {
			info.Progress = fmt.Sprintf("%d%%", envelope.Data.Progress)
		}
		if envelope.Data.Error != nil && strings.TrimSpace(envelope.Data.Error.Message) != "" {
			info.Reason = strings.TrimSpace(envelope.Data.Error.Message)
		}
		if envelope.Data.Result != nil {
			resultMap = envelope.Data.Result
		}
		dataUsage = envelope.Data.Usage
		dataDirect = map[string]any{
			"actual_duration": envelope.Data.ActualDuration,
			"video_duration":  envelope.Data.VideoDuration,
		}
	}
	if u := extractMediaURL(resultMap); u != "" {
		info.Url = u
		info.RemoteUrl = u
	}
	if info.TaskID == "" {
		info.TaskID = strings.TrimSpace(envelope.ID)
	}

	switch status {
	case "completed", "success", "succeeded", "done":
		info.Status = string(model.TaskStatusSuccess)
		info.Progress = taskcommon.ProgressComplete
		if !isNonVideoResult(task, resultMap) {
			flatDirect := map[string]any{
				"actual_duration": envelope.ActualDuration,
				"video_duration":  envelope.VideoDuration,
			}
			if sec, ok := extractActualVideoDuration(resultMap, dataUsage, dataDirect, envelope.Usage, flatDirect); ok {
				info.UsageFacts = map[string]any{"seconds": sec}
			}
		}
	case "failed", "error", "failure", "cancelled", "canceled":
		info.Status = string(model.TaskStatusFailure)
		if info.Reason == "" {
			info.Reason = status
		}
	case "pending", "submitted", "queued":
		info.Status = string(model.TaskStatusQueued)
		if info.Progress == "" {
			info.Progress = taskcommon.ProgressQueued
		}
	case "processing", "in_progress", "running":
		info.Status = string(model.TaskStatusInProgress)
		if info.Progress == "" {
			info.Progress = taskcommon.ProgressInProgress
		}
	default:
		info.Status = string(model.TaskStatusInProgress)
		if info.Progress == "" {
			info.Progress = taskcommon.ProgressInProgress
		}
	}
	return info, nil
}

// AdjustBillingOnComplete reconciles auto-duration (duration=-1) video tasks
// against the actual output duration reported by upstream. It never exceeds the
// pre-authorized task quota and returns 0 on failure, free groups, or missing/
// invalid/oversized duration so pre-charge or full refund handling applies.
func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if task == nil || taskResult == nil {
		return 0
	}
	if task.Status != "" && task.Status != model.TaskStatusSuccess {
		return 0
	}
	if taskResult.Status != "" && taskResult.Status != string(model.TaskStatusSuccess) {
		return 0
	}
	if task.Status != model.TaskStatusSuccess && taskResult.Status != string(model.TaskStatusSuccess) {
		return 0
	}
	bc := task.PrivateData.BillingContext
	if bc == nil || !bc.AutoDurationBilling {
		return 0
	}
	if task.Quota <= 0 || bc.GroupRatio <= 0 || math.IsNaN(bc.GroupRatio) || math.IsInf(bc.GroupRatio, 0) {
		return 0
	}
	if math.IsNaN(bc.ModelPrice) || math.IsInf(bc.ModelPrice, 0) || math.IsNaN(bc.ModelRatio) || math.IsInf(bc.ModelRatio, 0) {
		return 0
	}
	actualSeconds, ok := parseBoundedPositiveSeconds(taskResult.UsageFacts["seconds"])
	if !ok {
		return 0
	}
	reservedSeconds := bc.OtherRatios["seconds"]
	if math.IsNaN(reservedSeconds) || math.IsInf(reservedSeconds, 0) || reservedSeconds <= 0 || actualSeconds >= reservedSeconds {
		return 0
	}

	var baseQuota int
	var clamp *common.QuotaClamp
	if bc.ModelPrice > 0 {
		baseQuota, clamp = common.QuotaFromFloatChecked(bc.ModelPrice * common.QuotaPerUnit * bc.GroupRatio)
	} else if bc.ModelRatio > 0 {
		baseQuota, clamp = common.QuotaFromFloatChecked(bc.ModelRatio / 2 * common.QuotaPerUnit * bc.GroupRatio)
	} else {
		return 0
	}
	if clamp != nil {
		common.SysError(fmt.Sprintf("apimart settle base clamp task=%s: %v", task.TaskID, clamp))
		return 0
	}
	if baseQuota <= 0 {
		return 0
	}

	priceData := &hosttypes.PriceData{}
	for k, v := range bc.OtherRatios {
		if k == "seconds" {
			continue
		}
		priceData.AddOtherRatio(k, v)
	}
	priceData.AddOtherRatio("seconds", actualSeconds)

	actualQuota, clamp := common.QuotaFromFloatChecked(priceData.ApplyOtherRatiosToFloat(float64(baseQuota)))
	if clamp != nil {
		common.SysError(fmt.Sprintf("apimart settle quota clamp task=%s: %v", task.TaskID, clamp))
		return 0
	}
	if actualQuota <= 0 || actualQuota >= task.Quota {
		return 0
	}
	return actualQuota
}

// mediaDurationKeys applies to media artifacts and billing usage sub-objects
// where bare duration/seconds represent output media length.
var mediaDurationKeys = []string{"actual_duration", "video_duration", "duration", "seconds"}

// envelopeDurationKeys applies to outer task envelopes (data.* and flat root),
// where bare duration/seconds frequently carry wall-clock task execution time
// rather than output video length. Only unambiguous media duration keys are
// accepted on outer envelopes; unrecognized layouts fail closed to reserved quota.
var envelopeDurationKeys = []string{"actual_duration", "video_duration"}

func isNonVideoResult(task *model.Task, result map[string]any) bool {
	if task != nil && task.IsImageTask() {
		return true
	}
	if result == nil {
		return false
	}
	if _, hasVideos := result["videos"]; hasVideos {
		return false
	}
	if _, hasVideo := result["video"]; hasVideo {
		return false
	}
	if _, hasVideoURL := result["video_url"]; hasVideoURL {
		return false
	}
	if _, hasMusic := result["music"]; hasMusic {
		return true
	}
	if _, hasImages := result["images"]; hasImages {
		return true
	}
	if _, hasImageURLs := result["image_urls"]; hasImageURLs {
		return true
	}
	return false
}

func extractDurationFromMap(m map[string]any, keys []string) (seconds float64, found bool, valid bool) {
	if m == nil {
		return 0, false, false
	}
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		if s, isStr := v.(string); isStr && strings.TrimSpace(s) == "" {
			continue
		}
		sec, ok := parseBoundedPositiveSeconds(v)
		return sec, true, ok
	}
	return 0, false, false
}

func extractActualVideoDuration(result, dataUsage, dataDirect, flatUsage, flatDirect map[string]any) (float64, bool) {
	if result != nil {
		if arr, ok := result["videos"].([]any); ok && len(arr) > 0 {
			if item, ok := arr[0].(map[string]any); ok {
				if sec, found, valid := extractDurationFromMap(item, mediaDurationKeys); found {
					return sec, valid
				}
			}
		}
		if videoObj, ok := result["video"].(map[string]any); ok {
			if sec, found, valid := extractDurationFromMap(videoObj, mediaDurationKeys); found {
				return sec, valid
			}
		}
		if sec, found, valid := extractDurationFromMap(result, mediaDurationKeys); found {
			return sec, valid
		}
		if usageObj, ok := result["usage"].(map[string]any); ok {
			if sec, found, valid := extractDurationFromMap(usageObj, mediaDurationKeys); found {
				return sec, valid
			}
		}
	}
	for _, candidate := range []struct {
		fields map[string]any
		keys   []string
	}{
		{dataUsage, mediaDurationKeys},
		{dataDirect, envelopeDurationKeys},
		{flatUsage, mediaDurationKeys},
		{flatDirect, envelopeDurationKeys},
	} {
		if sec, found, valid := extractDurationFromMap(candidate.fields, candidate.keys); found {
			return sec, valid
		}
	}
	return 0, false
}

func extractMediaURL(result map[string]any) string {
	if result == nil {
		return ""
	}
	// videos
	if u := firstURLFromList(result["videos"], []string{"url", "video_url"}); u != "" {
		return u
	}
	// images (MJ / image gen) — url may be string or []string
	if u := firstURLFromList(result["images"], []string{"url", "image_url"}); u != "" {
		return u
	}
	// music
	if u := firstURLFromList(result["music"], []string{"audio_url", "url", "video_url"}); u != "" {
		return u
	}
	// flat
	for _, k := range []string{"video_url", "audio_url", "url", "grid_image_url"} {
		if u := asURLString(result[k]); u != "" {
			return u
		}
	}
	if arr, ok := result["image_urls"].([]any); ok && len(arr) > 0 {
		if u := asURLString(arr[0]); u != "" {
			return u
		}
	}
	return ""
}

func firstURLFromList(v any, keys []string) string {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return ""
	}
	item, ok := arr[0].(map[string]any)
	if !ok {
		return asURLString(arr[0])
	}
	for _, k := range keys {
		if u := asURLString(item[k]); u != "" {
			return u
		}
	}
	return ""
}

func asURLString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []any:
		if len(t) > 0 {
			if s, ok := t[0].(string); ok {
				return strings.TrimSpace(s)
			}
		}
	case []string:
		if len(t) > 0 {
			return strings.TrimSpace(t[0])
		}
	}
	return ""
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{
		"kling-v2-6",
		"kling-v3",
		"seedance-2-0",
		"seedance-2-0-fast",
		"seedance-2-0-mini",
		"seedance-2-5",
		"wan-3.0",
		"minimax-h3",
		"grok-imagine-video-1-5",
		"midjourney",
		"suno",
		"suno-sounds",
	}
}

func (a *TaskAdaptor) GetChannelName() string {
	return "apimart"
}

var (
	_ channel.TaskAdaptor                     = (*TaskAdaptor)(nil)
	_ channel.TaskValidatedBillingProvider    = (*TaskAdaptor)(nil)
	_ channel.TaskUsageFactsProvider          = (*TaskAdaptor)(nil)
	_ channel.TaskValidatedUsageFactsProvider = (*TaskAdaptor)(nil)
)
