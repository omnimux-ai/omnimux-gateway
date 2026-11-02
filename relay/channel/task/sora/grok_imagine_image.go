package sora

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// IsGrokImagineImage checks whether the request / model is for grok-imagine-image-* (type-1 async image).
func IsGrokImagineImage(info *relaycommon.RelayInfo) bool {
	if info == nil {
		return false
	}
	for _, name := range []string{info.OriginModelName, info.GetUpstreamModelName()} {
		if isGrokImagineImageName(name) {
			return true
		}
	}
	return false
}

func isGrokImagineImageName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	if strings.Contains(n, "grok-imagine-image") {
		return true
	}
	if IsEvoLinkGPTImageModel(n) {
		return true
	}
	switch n {
	case "nano-banana-pro", "nano-banana-2", "nano-banana-2-pro", "seedream-5-0-pro", "qwen-image-3-0", "gpt-image-2.5", "gemini-3.1-flash-image", "gemini-3.1-flash-lite-image":
		return true
	}
	return false
}

// IsAsyncImageModel reports whether modelName should be routed through async task submit.
func IsAsyncImageModel(modelName string) bool {
	return isGrokImagineImageName(modelName)
}

// IsEvoLinkGPTImageModel identifies the exact upstream models whose reference
// requests use the image_urls generation contract, including on an edit action.
// Callers selecting a channel must check its type and mapped upstream model.
func IsEvoLinkGPTImageModel(modelName string) bool {
	switch strings.ToLower(strings.TrimSpace(modelName)) {
	case "gpt-image-2.5-flare", "gpt-image-2.5-sunburst":
		return true
	default:
		return false
	}
}

// IsImageEditRequest reports whether this task was submitted as an image edit
// rather than a plain text-to-image generation. Action is promoted through the
// embedded TaskRelayInfo pointer, so the nil check has to come first.
func IsImageEditRequest(info *relaycommon.RelayInfo) bool {
	return info != nil && info.TaskRelayInfo != nil && info.Action == constant.TaskActionImageEdit
}

// isImageEditRequestPath reports whether the caller posted to an image-edit
// surface. Both the token-authenticated route (/v1/images/edits) and the
// session-authenticated playground route (/pg/images/edits) end in this suffix.
func isImageEditRequestPath(c *gin.Context) bool {
	if c == nil || c.Request == nil || c.Request.URL == nil {
		return false
	}
	path := c.Request.URL.Path
	return strings.Contains(path, "/images/edits") || strings.HasSuffix(path, "/edits")
}

// referenceImageURLs returns the reference material a request carries, whichever
// accepted spelling the caller used.
func referenceImageURLs(req relaycommon.TaskSubmitReq) []string {
	refs := make([]string, 0, len(req.Images)+2)
	for _, raw := range req.Images {
		if trimmed := strings.TrimSpace(raw); trimmed != "" {
			refs = append(refs, trimmed)
		}
	}
	for _, single := range []string{req.Image, req.InputReference} {
		if trimmed := strings.TrimSpace(single); trimmed != "" {
			refs = append(refs, trimmed)
		}
	}
	return refs
}

// referenceURLsFromBody reads references straight off the decoded body, so a
// spelling the typed struct does not model still reaches the upstream under the
// one canonical field.
func referenceURLsFromBody(body map[string]any) []string {
	refs := []string{}
	var appendValue func(value any)
	appendValue = func(value any) {
		switch typed := value.(type) {
		case string:
			if trimmed := strings.TrimSpace(typed); trimmed != "" {
				refs = append(refs, trimmed)
			}
		case []any:
			for _, item := range typed {
				appendValue(item)
			}
		case []string:
			for _, item := range typed {
				appendValue(item)
			}
		}
	}
	for _, key := range []string{"images", "image_urls", "image", "input_reference"} {
		if value, ok := body[key]; ok {
			appendValue(value)
		}
	}
	return refs
}

// normalizeImageEditBody rewrites every accepted reference spelling onto the one
// field the OpenAI images edit surface defines, and drops the legacy keys so an
// upstream never has to reconcile two answers to the same question.
func normalizeImageEditBody(body map[string]any) {
	if refs := referenceURLsFromBody(body); len(refs) > 0 {
		body["images"] = refs
	}
	for _, legacy := range []string{"image_urls", "input_reference", "image"} {
		delete(body, legacy)
	}
}

func normalizeEvoLinkGPTImageBody(body map[string]any) error {
	if quality, exists := body["quality"]; exists {
		switch quality {
		case "low", "medium", "high", "xhigh", "max":
		default:
			return fmt.Errorf("quality must be one of low, medium, high, xhigh, max")
		}
	}
	// Preserve the documented medium default when quality was omitted. In
	// particular, a provider-derived auto from another attempt is not low.
	referenceBody := make(map[string]any, 4)
	for _, key := range []string{"images", "image_urls", "image", "input_reference"} {
		value, err := evoLinkImageReferenceValue(body[key])
		if err != nil {
			return err
		}
		referenceBody[key] = value
	}
	if refs := referenceURLsFromBody(referenceBody); len(refs) > 0 {
		body["image_urls"] = refs
	}
	for _, alias := range []string{"images", "image", "input_reference"} {
		delete(body, alias)
	}
	return nil
}

// evoLinkImageReferenceValue preserves reference-object spellings accepted by
// TaskSubmitReq before the shared string-list collector canonicalizes them.
func evoLinkImageReferenceValue(value any) (any, error) {
	switch ref := value.(type) {
	case nil, string:
		return value, nil
	case []any:
		refs := make([]any, len(ref))
		for i, item := range ref {
			converted, err := evoLinkImageReferenceValue(item)
			if err != nil {
				return nil, err
			}
			refs[i] = converted
		}
		return refs, nil
	case map[string]any:
		for _, key := range []string{"url", "image_url"} {
			if url, ok := ref[key].(string); ok && strings.TrimSpace(url) != "" {
				return strings.TrimSpace(url), nil
			}
		}
	}
	return nil, fmt.Errorf("image references must contain URL strings")
}

func grokImagineImageUpstreamModel(name string) string {
	n := strings.TrimSpace(name)
	switch strings.ToLower(n) {
	case "grok-imagine-image-2-0":
		return "grok-imagine-image-2.0"
	default:
		return n
	}
}

// validateGrokImageRequest validates and parses OpenAI-compatible /v1/images/generations request into task context.
func validateGrokImageRequest(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	contract, contractErr := imageTaskContractFor(info)
	if contractErr != nil {
		return service.TaskErrorWrapperLocal(contractErr, "invalid_image_task_contract", http.StatusBadRequest)
	}
	if contract != nil {
		return validateExplicitImageTask(c, info, contract)
	}
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(info.OriginModelName) == "" && strings.TrimSpace(req.Model) != "" {
		info.OriginModelName = strings.TrimSpace(req.Model)
	}
	if strings.TrimSpace(info.OriginModelName) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model is required"), "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("prompt is required"), "invalid_request", http.StatusBadRequest)
	}
	refs := referenceImageURLs(req)
	if IsEvoLinkGPTImageModel(info.GetUpstreamModelName()) {
		var body map[string]any
		if err := common.UnmarshalBodyReusable(c, &body); err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
		if err := normalizeEvoLinkGPTImageBody(body); err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
		refs = referenceURLsFromBody(body)
	}

	if len(refs) == 0 {
		// A caller may use a spelling the typed struct does not model, so the
		// edit gate reads the decoded body before rejecting the request.
		var raw map[string]any
		if err := common.UnmarshalBodyReusable(c, &raw); err == nil {
			refs = referenceURLsFromBody(raw)
		}
	}
	if isImageEditRequestPath(c) {
		if len(refs) == 0 {
			return service.TaskErrorWrapperLocal(fmt.Errorf("images is required for image edit"), "invalid_request", http.StatusBadRequest)
		}
		info.Action = constant.TaskActionImageEdit
	} else {
		info.Action = constant.TaskActionGenerate
	}
	c.Set("task_request", req)
	return nil
}

// LooksLikeSyncImageResponse checks if the upstream body is a synchronous OpenAI image response like
// {"created":..., "data":[{"url":"http://..."}]} or {"data":[{"b64_json":"..."}]}.
func LooksLikeSyncImageResponse(body []byte) (firstURL string, isSync bool) {
	return looksLikeSyncImageResponse(body)
}

func looksLikeSyncImageResponse(body []byte) (firstURL string, isSync bool) {
	var envelope struct {
		Created int64 `json:"created"`
		Data    []struct {
			URL           string `json:"url"`
			B64JSON       string `json:"b64_json"`
			RevisedPrompt string `json:"revised_prompt"`
		} `json:"data"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil {
		return "", false
	}
	if len(envelope.Data) > 0 {
		for _, item := range envelope.Data {
			if strings.TrimSpace(item.URL) != "" {
				return strings.TrimSpace(item.URL), true
			}
			if strings.TrimSpace(item.B64JSON) != "" {
				return "", true
			}
		}
	}
	return "", false
}

// rewriteSyncImageResponse replaces upstream internal URLs with public image proxy URLs.
func rewriteSyncImageResponse(body []byte, publicTaskID string) ([]byte, string, error) {
	var envelope struct {
		ID      string           `json:"id,omitempty"`
		TaskID  string           `json:"task_id,omitempty"`
		Created int64            `json:"created"`
		Data    []map[string]any `json:"data"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil {
		return body, "", err
	}
	if len(envelope.Data) == 0 {
		return body, "", nil
	}

	originalURL := ""
	proxyURL := taskcommon.BuildImageProxyURL(publicTaskID)

	for i, item := range envelope.Data {
		if u, ok := item["url"].(string); ok && strings.TrimSpace(u) != "" {
			if originalURL == "" {
				originalURL = strings.TrimSpace(u)
			}
			envelope.Data[i]["url"] = proxyURL
		}
	}
	if strings.TrimSpace(publicTaskID) != "" {
		envelope.ID = publicTaskID
		envelope.TaskID = publicTaskID
	}

	rewritten, err := common.Marshal(envelope)
	if err != nil {
		return body, originalURL, err
	}
	return rewritten, originalURL, nil
}

// looksLikeAsyncImageSubmit checks if the upstream response matches {code:200, data:[{status:"submitted", task_id:...}]}
// or {task_id:...} / {id:...} with status queued|submitted|pending|processing.
func looksLikeAsyncImageSubmit(body []byte) (taskID string, ok bool) {
	var envelope struct {
		Code int `json:"code"`
		Data []struct {
			ID      string `json:"id"`
			TaskID  string `json:"task_id"`
			Status  string `json:"status"`
			Message string `json:"message"`
		} `json:"data"`
		ID        string `json:"id"`
		TaskID    string `json:"task_id"`
		RequestID string `json:"request_id"`
		Status    string `json:"status"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil {
		return "", false
	}
	if len(envelope.Data) > 0 {
		first := envelope.Data[0]
		st := strings.ToLower(strings.TrimSpace(first.Status))
		id := strings.TrimSpace(first.TaskID)
		if id == "" {
			id = strings.TrimSpace(first.ID)
		}
		if id != "" && (st == "" || isAsyncStatus(st) || envelope.Code == 200) {
			return id, true
		}
	}
	st := strings.ToLower(strings.TrimSpace(envelope.Status))
	id := strings.TrimSpace(envelope.TaskID)
	if id == "" {
		id = strings.TrimSpace(envelope.ID)
	}
	if id == "" {
		id = strings.TrimSpace(envelope.RequestID)
	}
	if id != "" && isAsyncStatus(st) {
		return id, true
	}
	return "", false
}

func isAsyncStatus(st string) bool {
	switch st {
	case "submitted", "queued", "pending", "processing", "in_progress", "running", "done", "completed", "success":
		return true
	default:
		return false
	}
}

// looksLikeImageTaskPoll checks whether the polling response matches Apimart / Evolink / generic image task poll shape.
func looksLikeImageTaskPoll(body []byte) bool {
	var envelope struct {
		Code int `json:"code"`
		Data *struct {
			ID     string         `json:"id"`
			TaskID string         `json:"task_id"`
			Status string         `json:"status"`
			Result map[string]any `json:"result"`
			Images []any          `json:"images"`
		} `json:"data"`
		Status  string `json:"status"`
		ID      string `json:"id"`
		TaskID  string `json:"task_id"`
		Images  []any  `json:"images"`
		Results any    `json:"results"`
	}
	if err := common.Unmarshal(body, &envelope); err != nil {
		return false
	}
	if envelope.Data != nil {
		st := strings.ToLower(strings.TrimSpace(envelope.Data.Status))
		if isAsyncStatus(st) || envelope.Data.Result != nil || len(envelope.Data.Images) > 0 {
			return true
		}
	}
	st := strings.ToLower(strings.TrimSpace(envelope.Status))
	if isAsyncStatus(st) && (envelope.ID != "" || envelope.TaskID != "" || len(envelope.Images) > 0 || envelope.Results != nil) {
		return true
	}
	return false
}

// parseImageTaskPoll parses Apimart, Evolink, and generic image task polling response bodies into TaskInfo.
func parseImageTaskPoll(respBody []byte) (*relaycommon.TaskInfo, error) {
	var envelope struct {
		Code int `json:"code"`
		Data *struct {
			ID       string         `json:"id"`
			TaskID   string         `json:"task_id"`
			Status   string         `json:"status"`
			Progress int            `json:"progress"`
			Result   map[string]any `json:"result"`
			Images   []any          `json:"images"`
			Error    *struct {
				Message string `json:"message"`
				Code    string `json:"code"`
			} `json:"error"`
		} `json:"data"`
		// Flat format support (Evolink / other upstream variants)
		ID       string `json:"id"`
		TaskID   string `json:"task_id"`
		Status   string `json:"status"`
		Progress any    `json:"progress"`
		Error    *struct {
			Message string `json:"message"`
			Code    string `json:"code"`
		} `json:"error"`
		Result     map[string]any `json:"result"`
		Images     []any          `json:"images"`
		Results    []any          `json:"results"`
		ResultData []any          `json:"result_data"`
	}
	if err := common.Unmarshal(respBody, &envelope); err != nil {
		return nil, errors.Wrap(err, "unmarshal image task poll response")
	}

	info := &relaycommon.TaskInfo{Code: 0}
	var rawStatus string
	var rawID string
	var rawProgress string
	var rawErr string
	var mediaResult map[string]any
	var imageList []any

	if envelope.Data != nil {
		rawID = strings.TrimSpace(envelope.Data.TaskID)
		if rawID == "" {
			rawID = strings.TrimSpace(envelope.Data.ID)
		}
		rawStatus = envelope.Data.Status
		if envelope.Data.Progress > 0 && envelope.Data.Progress < 100 {
			rawProgress = fmt.Sprintf("%d%%", envelope.Data.Progress)
		}
		if envelope.Data.Error != nil && strings.TrimSpace(envelope.Data.Error.Message) != "" {
			rawErr = strings.TrimSpace(envelope.Data.Error.Message)
		}
		mediaResult = envelope.Data.Result
		imageList = envelope.Data.Images
	} else {
		rawID = strings.TrimSpace(envelope.TaskID)
		if rawID == "" {
			rawID = strings.TrimSpace(envelope.ID)
		}
		rawStatus = envelope.Status
		if p, ok := envelope.Progress.(float64); ok && p > 0 && p < 100 {
			rawProgress = fmt.Sprintf("%d%%", int(p))
		} else if ps, ok := envelope.Progress.(string); ok && ps != "" {
			rawProgress = ps
		}
		if envelope.Error != nil && strings.TrimSpace(envelope.Error.Message) != "" {
			rawErr = strings.TrimSpace(envelope.Error.Message)
		}
		mediaResult = envelope.Result
		imageList = envelope.Images
		// Evolink flat shape: results: ["https://..."], result_data: [{"url": "..."}]
		if len(envelope.Results) > 0 {
			if u := firstURLFromAnyList(envelope.Results); u != "" {
				info.Url = u
				info.RemoteUrl = u
			}
		}
		if info.Url == "" && len(envelope.ResultData) > 0 {
			if u := firstURLFromAnyList(envelope.ResultData); u != "" {
				info.Url = u
				info.RemoteUrl = u
			}
		}
		if info.Url == "" && len(envelope.Images) > 0 {
			imageList = envelope.Images
		}
	}

	info.TaskID = rawID
	info.Reason = rawErr
	info.Progress = rawProgress

	// Extract URL — only fill if not already set by flat results/result_data above.
	url := extractImageMediaURL(mediaResult, imageList)
	if info.Url == "" && url != "" {
		info.Url = url
		info.RemoteUrl = url
	}

	switch strings.ToLower(strings.TrimSpace(rawStatus)) {
	case "completed", "success", "succeeded", "done":
		info.Status = string(model.TaskStatusSuccess)
		info.Progress = taskcommon.ProgressComplete
	case "failed", "error", "failure", "cancelled", "canceled", "expired":
		info.Status = string(model.TaskStatusFailure)
		if info.Reason == "" {
			info.Reason = rawStatus
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
		if url != "" {
			info.Status = string(model.TaskStatusSuccess)
			info.Progress = taskcommon.ProgressComplete
		} else {
			info.Status = string(model.TaskStatusInProgress)
			if info.Progress == "" {
				info.Progress = taskcommon.ProgressInProgress
			}
		}
	}

	return info, nil
}

func extractImageMediaURL(result map[string]any, images []any) string {
	if len(images) > 0 {
		if u := firstURLFromAnyList(images); u != "" {
			return u
		}
	}
	if result != nil {
		if u := firstURLFromAnyList(result["images"]); u != "" {
			return u
		}
		for _, k := range []string{"image_url", "url", "image", "b64_json"} {
			if s, ok := result[k].(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
		if arr, ok := result["image_urls"].([]any); ok && len(arr) > 0 {
			if u := firstURLFromAnyList(arr); u != "" {
				return u
			}
		}
	}
	return ""
}

func firstURLFromAnyList(val any) string {
	if val == nil {
		return ""
	}
	switch list := val.(type) {
	case []any:
		for _, item := range list {
			switch v := item.(type) {
			case string:
				if strings.TrimSpace(v) != "" {
					return strings.TrimSpace(v)
				}
			case map[string]any:
				for _, k := range []string{"url", "image_url", "image"} {
					if s, ok := v[k].(string); ok && strings.TrimSpace(s) != "" {
						return strings.TrimSpace(s)
					}
					// Some upstreams (e.g. APIMart) return url as an array: {"url": ["https://..."]}
					if arr, ok := v[k].([]any); ok {
						if u := firstURLFromAnyList(arr); u != "" {
							return u
						}
					}
					if arr, ok := v[k].([]string); ok {
						if u := firstURLFromAnyList(arr); u != "" {
							return u
						}
					}
				}
			}
		}
	case []string:
		for _, s := range list {
			if strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}
