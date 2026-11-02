package apimart

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
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
	ChannelType int
	baseURL     string
	apiKey      string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
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

	kind := detectKind(modelName, c.Request.URL.Path)
	c.Set(contextKeyApimartKind, kind)
	info.Action = constant.TaskActionGenerate

	// Validate video duration/resolution billing inputs early (EstimateBilling cannot return errors).
	if kind == kindVideo {
		if isPhaseOneVideoModel(modelName) {
			normalized, err := validatePhaseOneVideoRequest(modelName, raw)
			if err != nil {
				return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
			}
			raw = normalized
		}
		if _, err := estimateVideoBilling(modelName, raw); err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
	}

	// Store raw for BuildRequestBody (preserves extra fields: duration, version, aspect_ratio, …)
	c.Set("task_request_raw", raw)
	return nil
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
	// MJ / Suno music / Suno sound effects: ModelPrice is USD per call.
	kind := detectKind(info.OriginModelName, c.Request.URL.Path)
	if kind == kindMusic || kind == kindSoundEffect || kind == kindMidjourney {
		return map[string]float64{"units": 1}
	}

	// Video: ModelPrice is USD per second for the model base tier.
	// OtherRatios: seconds × tier (resolution/mode relative to base).
	raw, _ := c.Get("task_request_raw")
	body, _ := raw.(map[string]any)
	if body == nil {
		body = map[string]any{}
		_ = common.UnmarshalBodyReusable(c, &body)
	}
	ratios, err := estimateVideoBilling(info.OriginModelName, body)
	if err != nil {
		// Validated in ValidateRequestAndSetAction; fall back to default seconds only.
		return map[string]float64{"seconds": float64(defaultVideoSeconds)}
	}
	return ratios
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := a.baseURL
	if base == "" {
		base = defaultBaseURL
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
	if canonical, ok := canonicalPhaseOneVideoModel(info.OriginModelName); ok {
		// The documented phase-one map is authoritative even when an existing
		// channel row still carries an older model_mapping value.
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
	if kind == kindMusic {
		if _, ok := out["version"]; !ok {
			out["version"] = defaultSunoVersion
		}
		// APIMart inspiration mode uses custom=false when only prompt is set
		if _, ok := out["custom"]; !ok {
			out["custom"] = false
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
		// fork: Omni-Flash-Ext 的按次价只覆盖 ≤1080P 档，超档（4K / 超时长）在入口拒绝，避免上游计费高于售价。
		if isOmniFlashExtModel(upstream) || isOmniFlashExtModel(info.OriginModelName) {
			if err := validateOmniFlashExtCaps(out); err != nil {
				return nil, err
			}
		}
		if !isPhaseOneVideoModel(info.OriginModelName) {
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
			ID       string         `json:"id"`
			Status   string         `json:"status"`
			Progress int            `json:"progress"`
			Result   map[string]any `json:"result"`
			Error    *struct {
				Message string `json:"message"`
			} `json:"error"`
		} `json:"data"`
		// Flat fallbacks
		Status string `json:"status"`
		ID     string `json:"id"`
	}
	if err := common.Unmarshal(respBody, &envelope); err != nil {
		return nil, errors.Wrap(err, "unmarshal apimart task")
	}

	info := &relaycommon.TaskInfo{Code: 0}
	status := strings.ToLower(strings.TrimSpace(envelope.Status))
	if envelope.Data != nil {
		info.TaskID = strings.TrimSpace(envelope.Data.ID)
		status = strings.ToLower(strings.TrimSpace(envelope.Data.Status))
		if envelope.Data.Progress > 0 && envelope.Data.Progress < 100 {
			info.Progress = fmt.Sprintf("%d%%", envelope.Data.Progress)
		}
		if envelope.Data.Error != nil && strings.TrimSpace(envelope.Data.Error.Message) != "" {
			info.Reason = strings.TrimSpace(envelope.Data.Error.Message)
		}
		if u := extractMediaURL(envelope.Data.Result); u != "" {
			info.Url = u
			info.RemoteUrl = u
		}
	}
	if info.TaskID == "" {
		info.TaskID = strings.TrimSpace(envelope.ID)
	}

	switch status {
	case "completed", "success", "succeeded", "done":
		info.Status = string(model.TaskStatusSuccess)
		info.Progress = taskcommon.ProgressComplete
		if info.Url == "" && info.Reason == "" {
			// completed without URL still mark success; client may use stored raw data
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
