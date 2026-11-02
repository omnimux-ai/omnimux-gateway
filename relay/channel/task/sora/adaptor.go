package sora

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

// ============================
// Request / Response structures
// ============================

type ContentItem struct {
	Type     string    `json:"type"`                // "text" or "image_url"
	Text     string    `json:"text,omitempty"`      // for text type
	ImageURL *ImageURL `json:"image_url,omitempty"` // for image_url type
}

type ImageURL struct {
	URL string `json:"url"`
}

type responseTask struct {
	ID                 string `json:"id"`
	TaskID             string `json:"task_id,omitempty"` //兼容旧接口
	Object             string `json:"object"`
	Model              string `json:"model"`
	Status             string `json:"status"`
	Progress           int    `json:"progress"`
	CreatedAt          int64  `json:"created_at"`
	CompletedAt        int64  `json:"completed_at,omitempty"`
	ExpiresAt          int64  `json:"expires_at,omitempty"`
	Seconds            string `json:"seconds,omitempty"`
	Size               string `json:"size,omitempty"`
	RemixedFromVideoID string `json:"remixed_from_video_id,omitempty"`
	Error              *struct {
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info != nil {
		a.ChannelType = info.ChannelType
		a.baseURL = info.ChannelBaseUrl
		a.apiKey = info.ApiKey
	}
}

func validateRemixRequest(c *gin.Context) *dto.TaskError {
	var req relaycommon.TaskSubmitReq
	if err := common.UnmarshalBodyReusable(c, &req); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("field prompt is required"), "invalid_request", http.StatusBadRequest)
	}
	// 存储原始请求到 context，与 ValidateMultipartDirect 路径保持一致
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) (taskErr *dto.TaskError) {
	if IsGrokImagineImage(info) {
		return validateGrokImageRequest(c, info)
	}
	if info.Action == constant.TaskActionRemix {
		return validateRemixRequest(c)
	}
	return relaycommon.ValidateMultipartDirect(c, info)
}

// EstimateBilling 根据用户请求的 seconds 和 size 计算 OtherRatios。
func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	if IsGrokImagineImage(info) {
		return map[string]float64{"units": 1}
	}
	if isGrokImagineVideo(info) {
		// ModelPrice is USD per video, not per second.
		return map[string]float64{"units": 1}
	}
	// remix 路径的 OtherRatios 已在 ResolveOriginTask 中设置
	if info != nil && info.TaskRelayInfo != nil && info.Action == constant.TaskActionRemix {
		return nil
	}

	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil
	}

	seconds, _ := strconv.Atoi(req.Seconds)
	if seconds == 0 {
		seconds = req.Duration
	}
	if seconds <= 0 {
		seconds = 4
	}

	size := req.Size
	if size == "" {
		size = "720x1280"
	}

	ratios := map[string]float64{
		"seconds": float64(seconds),
		"size":    1,
	}
	if size == "1792x1024" || size == "1024x1792" {
		ratios["size"] = 1.666667
	}

	// 动态分辨率计费倍率 (OtherRatios):
	// Seedance 2.5: 480P 基准 1.0x (¥0.35/s), 720P 进阶 1.285714x (¥0.45/s, 0.45 / 0.35)
	// Seedance 2.0: 480P 基准 1.0x (¥0.45/s), 720P 进阶 1.555556x (¥0.70/s, 0.70 / 0.45)
	originModel := strings.ToLower(info.OriginModelName)
	if is720pResolution(req) {
		if strings.Contains(originModel, "seedance-2-5") || strings.Contains(originModel, "seedance-2.5") {
			ratios["resolution"] = 1.285714
		} else if strings.Contains(originModel, "seedance-2-0") || strings.Contains(originModel, "seedance-2.0") {
			ratios["resolution"] = 1.555556
		}
	}
	return ratios
}

func is720pResolution(req relaycommon.TaskSubmitReq) bool {
	res := strings.ToLower(strings.TrimSpace(req.Resolution))
	if strings.HasPrefix(res, "720") || strings.Contains(res, "720p") || strings.Contains(res, "hd") {
		return true
	}
	if strings.Contains(res, "1080") || strings.Contains(res, "2k") || strings.Contains(res, "4k") {
		return true
	}
	sz := strings.ToLower(strings.TrimSpace(req.Size))
	if strings.Contains(sz, "720") && !strings.Contains(sz, "480") {
		return true
	}
	if strings.Contains(sz, "1080") || strings.Contains(sz, "1792") || strings.Contains(sz, "1280") {
		return true
	}
	if req.Metadata != nil {
		if mRes, ok := req.Metadata["resolution"].(string); ok {
			mRes = strings.ToLower(strings.TrimSpace(mRes))
			if strings.HasPrefix(mRes, "720") || strings.Contains(mRes, "720p") {
				return true
			}
		}
	}
	return false
}

func resolveSeedanceUpstreamModel(upstreamModel, originModel string, is720p bool) string {
	// Pidoi Seedance 2.5: sd-2.5-480p-pro vs sd-2.5-720p-pro
	if strings.Contains(upstreamModel, "sd-2.5") || strings.Contains(originModel, "seedance-2-5") || strings.Contains(originModel, "seedance-2.5") {
		if strings.Contains(upstreamModel, "sd-2.5") {
			if is720p {
				return "sd-2.5-720p-pro"
			}
			return "sd-2.5-480p-pro"
		}
	}
	// Pidoi Seedance 2.0: seedace-2.0-480p vs seedace-2.0-720p
	if strings.Contains(upstreamModel, "seedace-2.0") || strings.Contains(originModel, "seedance-2-0") || strings.Contains(originModel, "seedance-2.0") {
		if strings.Contains(upstreamModel, "seedace-2.0") {
			if is720p {
				return "seedace-2.0-720p"
			}
			return "seedace-2.0-480p"
		}
	}
	return upstreamModel
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	contract, contractErr := imageTaskContractFor(info)
	if contractErr != nil {
		return "", contractErr
	}
	if contract != nil {
		path := "/v1/images/generations"
		if contract.Protocol == "openai_multipart" && IsImageEditRequest(info) {
			path = "/v1/images/edits"
		}
		return strings.TrimRight(a.baseURL, "/") + path, nil
	}
	if info != nil && info.TaskRelayInfo != nil && info.Action == constant.TaskActionRemix {
		return fmt.Sprintf("%s/v1/videos/%s/remix", a.baseURL, info.OriginTaskID), nil
	}
	if IsGrokImagineImage(info) {
		if IsImageEditRequest(info) && !IsEvoLinkGPTImageModel(info.GetUpstreamModelName()) {
			return fmt.Sprintf("%s/v1/images/edits", strings.TrimRight(a.baseURL, "/")), nil
		}
		return fmt.Sprintf("%s/v1/images/generations", strings.TrimRight(a.baseURL, "/")), nil
	}
	if isGrokImagineVideo(info) {
		return fmt.Sprintf("%s/v1/videos/generations", strings.TrimRight(a.baseURL, "/")), nil
	}
	return fmt.Sprintf("%s/v1/videos", a.baseURL), nil
}

// BuildRequestHeader sets required headers.
func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	contract, contractErr := imageTaskContractFor(info)
	if contractErr != nil {
		return contractErr
	}
	if contract != nil {
		contentType := c.GetString(imageTaskContentTypeKey)
		if contentType == "" {
			return fmt.Errorf("image task request body has not been prepared")
		}
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Accept", "application/json")
		return nil
	}
	if IsGrokImagineImage(info) || isGrokImagineVideo(info) {
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Accept", "application/json")
		return nil
	}
	req.Header.Set("Content-Type", c.Request.Header.Get("Content-Type"))
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	contract, contractErr := imageTaskContractFor(info)
	if contractErr != nil {
		return nil, contractErr
	}
	if contract != nil {
		return buildExplicitImageTaskBody(c, info, contract)
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, errors.Wrap(err, "get_request_body_failed")
	}
	cachedBody, err := storage.Bytes()
	if err != nil {
		return nil, errors.Wrap(err, "read_body_bytes_failed")
	}
	contentType := c.GetHeader("Content-Type")

	if strings.HasPrefix(contentType, "application/json") {
		var bodyMap map[string]interface{}
		if err := common.Unmarshal(cachedBody, &bodyMap); err == nil {
			modelName := info.UpstreamModelName
			if IsGrokImagineImage(info) {
				modelName = grokImagineImageUpstreamModel(modelName)
			} else if isGrokImagineVideo(info) {
				modelName = grokImagineUpstreamModel(modelName)
			} else {
				req, _ := relaycommon.GetTaskRequest(c)
				is720p := is720pResolution(req)
				modelName = resolveSeedanceUpstreamModel(modelName, info.OriginModelName, is720p)
				if strings.Contains(modelName, "sd-2.5") || strings.Contains(modelName, "seedace-2.0") {
					if is720p {
						bodyMap["resolution"] = "720p"
					} else if bodyMap["resolution"] == nil || bodyMap["resolution"] == "" {
						bodyMap["resolution"] = "480p"
					}
				}
			}
			bodyMap["model"] = modelName
			if IsGrokImagineImage(info) {
				if (bodyMap["size"] == nil || bodyMap["size"] == "") && bodyMap["aspect_ratio"] != nil {
					bodyMap["size"] = bodyMap["aspect_ratio"]
				}
			}
			// For grok video, if image was passed as a plain string (e.g. "https://..."),
			// normalize it to the object structure expected by xAI/grok2api: {"url": "https://..."}.
			if isGrokImagineVideo(info) {
				if imgStr, ok := bodyMap["image"].(string); ok && strings.TrimSpace(imgStr) != "" {
					bodyMap["image"] = map[string]any{"url": strings.TrimSpace(imgStr)}
				}
			}
			if IsEvoLinkGPTImageModel(modelName) {
				if helper.IsGPTImageStandardRoute(c, info) {
					size, _ := bodyMap["size"].(string)
					pixels, supported := helper.GPTImageStandardPixels(size)
					if !supported {
						return nil, fmt.Errorf("standard image group size is unsupported")
					}
					bodyMap["size"] = pixels
					delete(bodyMap, "aspect_ratio")
					delete(bodyMap, "group") // Native routing has already authorized this field.
				}
				if err := normalizeEvoLinkGPTImageBody(bodyMap); err != nil {
					return nil, err
				}
			} else if IsImageEditRequest(info) {
				normalizeImageEditBody(bodyMap)
			}
			if newBody, err := common.Marshal(bodyMap); err == nil {
				return bytes.NewReader(newBody), nil
			}
		}
		return bytes.NewReader(cachedBody), nil
	}

	if strings.Contains(contentType, "multipart/form-data") {
		formData, err := common.ParseMultipartFormReusable(c)
		if err != nil {
			return bytes.NewReader(cachedBody), nil
		}
		var buf bytes.Buffer
		writer := multipart.NewWriter(&buf)
		modelName := info.UpstreamModelName
		if isGrokImagineVideo(info) {
			modelName = grokImagineUpstreamModel(modelName)
		} else {
			req, _ := relaycommon.GetTaskRequest(c)
			is720p := is720pResolution(req)
			modelName = resolveSeedanceUpstreamModel(modelName, info.OriginModelName, is720p)
			hasResolution := len(formData.Value["resolution"]) > 0 && strings.TrimSpace(formData.Value["resolution"][0]) != ""
			if (strings.Contains(modelName, "sd-2.5") || strings.Contains(modelName, "seedace-2.0")) && !hasResolution {
				if is720p {
					writer.WriteField("resolution", "720p")
				} else {
					writer.WriteField("resolution", "480p")
				}
			}
		}
		writer.WriteField("model", modelName)
		for key, values := range formData.Value {
			if key == "model" {
				continue
			}
			for _, v := range values {
				writer.WriteField(key, v)
			}
		}
		for fieldName, fileHeaders := range formData.File {
			for _, fh := range fileHeaders {
				f, err := fh.Open()
				if err != nil {
					continue
				}
				ct := fh.Header.Get("Content-Type")
				if ct == "" || ct == "application/octet-stream" {
					buf512 := make([]byte, 512)
					n, _ := io.ReadFull(f, buf512)
					ct = http.DetectContentType(buf512[:n])
					// Re-open after sniffing so the full content is copied below
					f.Close()
					f, err = fh.Open()
					if err != nil {
						continue
					}
				}
				h := make(textproto.MIMEHeader)
				h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, fieldName, fh.Filename))
				h.Set("Content-Type", ct)
				part, err := writer.CreatePart(h)
				if err != nil {
					f.Close()
					continue
				}
				io.Copy(part, f)
				f.Close()
			}
		}
		writer.Close()
		c.Request.Header.Set("Content-Type", writer.FormDataContentType())
		return &buf, nil
	}

	return common.NewReplayableBodyReader(storage), nil
}

// DoRequest delegates to common helper.
func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

// DoResponse handles upstream response, returns taskID etc.
func (a *TaskAdaptor) ParseResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*channel.TaskSubmitResponse, *dto.TaskError) {
	taskID, taskData, err := a.DoResponse(c, resp, info)
	if err != nil {
		return nil, err
	}
	res := &channel.TaskSubmitResponse{
		UpstreamTaskID: taskID,
		TaskData:       taskData,
	}
	if IsGrokImagineImage(info) {
		if _, isSync := looksLikeSyncImageResponse(taskData); isSync {
			res.ClientResponse = taskData
			var origURL string
			if c != nil {
				if u, exists := c.Get("sync_image_orig_url"); exists {
					if s, ok := u.(string); ok {
						origURL = s
					}
				}
			}
			res.Immediate = &relaycommon.TaskInfo{
				Status:    string(model.TaskStatusSuccess),
				Url:       origURL,
				RemoteUrl: origURL,
				Progress:  taskcommon.ProgressComplete,
			}
		}
	}
	return res, nil
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	contract, contractErr := imageTaskContractFor(info)
	var responseBody []byte
	var err error
	if contract != nil || contractErr != nil {
		responseBody, err = io.ReadAll(io.LimitReader(resp.Body, service.MaxInlineImageTaskResponseBytes+1))
		if err == nil && int64(len(responseBody)) > service.MaxInlineImageTaskResponseBytes {
			err = fmt.Errorf("image task response exceeds the reviewed size limit")
		}
	} else {
		responseBody, err = io.ReadAll(resp.Body)
	}
	_ = resp.Body.Close()
	if err != nil || contractErr != nil {
		if err == nil {
			err = contractErr
		}
		status := http.StatusInternalServerError
		strictImage := contract != nil || contractErr != nil
		if strictImage {
			status = http.StatusBadGateway
		}
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", status)
		taskErr.NoRetry = strictImage
		return
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		taskErr = service.TaskErrorWrapper(
			fmt.Errorf("upstream status %d: %s", resp.StatusCode, string(responseBody)),
			"upstream_error",
			resp.StatusCode,
		)
		return
	}

	if IsGrokImagineImage(info) {
		if contract != nil && contract.DeliveryFormat() == "b64_json" {
			normalized, contentURL, err := explicitInlineImageTaskResponse(c, info, responseBody)
			if err != nil {
				taskErr = service.TaskErrorWrapper(err, "invalid_image_task_response", http.StatusBadGateway)
				taskErr.NoRetry = true
				return
			}
			c.Set("sync_image_orig_url", contentURL)
			return info.PublicTaskID, normalized, nil
		}
		// 1. Synchronous OpenAI format (e.g. grok2api direct)
		if _, isSync := looksLikeSyncImageResponse(responseBody); isSync {
			contract, contractErr := imageTaskContractFor(info)
			if contract != nil && contractErr == nil {
				_, contractErr = explicitImageTaskResponseURL(responseBody)
			}
			if contractErr != nil {
				// A 2xx response may already have created work. An inline-only
				// image cannot be marked deliverable without native persistence.
				taskErr = service.TaskErrorWrapper(fmt.Errorf("image task response has no retrievable image URL"), "invalid_image_task_response", http.StatusBadGateway)
				taskErr.NoRetry = true
				return
			}
			rewrittenBody, origURL, err := rewriteSyncImageResponse(responseBody, info.PublicTaskID)
			if err != nil {
				taskErr = service.TaskErrorWrapper(err, "rewrite_image_response_failed", http.StatusInternalServerError)
				return
			}
			if c != nil && origURL != "" {
				c.Set("sync_image_orig_url", origURL)
			}
			// Return public task ID and use rewrittenBody as taskData so controller records it as completed
			return info.PublicTaskID, rewrittenBody, nil
		}

		// 2. Asynchronous task format (e.g. Evolink / Apimart)
		upstreamID, ok := looksLikeAsyncImageSubmit(responseBody)
		if !ok || upstreamID == "" {
			taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty in async image response: %s", string(responseBody)), "invalid_response", http.StatusInternalServerError)
			return
		}
		return upstreamID, responseBody, nil
	}

	if isGrokImagineVideo(info) {
		var gResp grokSubmitResponse
		if err := common.Unmarshal(responseBody, &gResp); err != nil {
			taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
			return
		}
		if gResp.Error != nil && strings.TrimSpace(gResp.Error.Message) != "" {
			taskErr = service.TaskErrorWrapper(fmt.Errorf("%s", gResp.Error.Message), "upstream_error", http.StatusBadGateway)
			return
		}
		upstreamID := grokSubmitTaskID(gResp)
		if upstreamID == "" {
			taskErr = service.TaskErrorWrapper(fmt.Errorf("request_id is empty"), "invalid_response", http.StatusInternalServerError)
			return
		}
		return upstreamID, responseBody, nil
	}

	// Parse Sora response
	var dResp responseTask
	if err := common.Unmarshal(responseBody, &dResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	upstreamID := dResp.ID
	if upstreamID == "" {
		upstreamID = dResp.TaskID
	}
	if upstreamID == "" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("task_id is empty"), "invalid_response", http.StatusInternalServerError)
		return
	}

	return upstreamID, responseBody, nil
}

// FetchTask fetch task status
func (a *TaskAdaptor) FetchTask(baseUrl, key string, task *model.Task, proxy string) (*http.Response, error) {
	if task == nil {
		return nil, fmt.Errorf("invalid task")
	}
	body := map[string]any{
		"task_id":      task.GetUpstreamTaskID(),
		"action":       task.Action,
		"origin_model": task.Properties.OriginModelName,
	}
	return a.FetchTaskMap(baseUrl, key, body, proxy)
}

func (a *TaskAdaptor) FetchTaskMap(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || strings.TrimSpace(taskID) == "" {
		return nil, fmt.Errorf("invalid task_id")
	}
	taskID = strings.TrimSpace(taskID)
	baseUrl = strings.TrimRight(baseUrl, "/")

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}

	// Determine task kind by origin model name (injected by polling worker).
	// Video tasks (grok-imagine-video*) poll GET /v1/videos/{id} (validated).
	// Image tasks (grok-imagine-image*) poll unified /v1/tasks/{id} with fallbacks.
	originModel := ""
	if m, ok := body["origin_model"].(string); ok {
		originModel = strings.TrimSpace(m)
	}
	isVideo := strings.Contains(strings.ToLower(originModel), "grok-imagine-video") ||
		strings.Contains(strings.ToLower(originModel), "video")
	isImage := strings.Contains(strings.ToLower(originModel), "image") || IsAsyncImageModel(originModel)

	// 2xx / non-2xx that is NOT a clear "not found" means endpoint exists: return it.
	// 404 / 405 mean the endpoint is not implemented: fall through to the next candidate.
	// Prefer the first response whose status is 200 or a well-formed 4xx/5xx error from the
	// endpoint that actually owns the task; only fall back when the endpoint is absent.
	candidateURLs := make([]string, 0, 3)
	if isImage {
		// Async image: unified task endpoint first (Apimart / Evolink), then image-task fallback.
		candidateURLs = []string{
			fmt.Sprintf("%s/v1/tasks/%s", baseUrl, taskID),
			fmt.Sprintf("%s/v1/images/tasks/%s", baseUrl, taskID),
			fmt.Sprintf("%s/v1/videos/%s", baseUrl, taskID), // last resort (some gateways alias)
		}
	} else if isVideo {
		// Video: /v1/videos/{id} only — do NOT probe /v1/tasks (avoids breaking Sora/grok video polling).
		candidateURLs = []string{
			fmt.Sprintf("%s/v1/videos/%s", baseUrl, taskID),
		}
	} else {
		// Unknown kind: Sora default then unified task endpoint.
		candidateURLs = []string{
			fmt.Sprintf("%s/v1/videos/%s", baseUrl, taskID),
			fmt.Sprintf("%s/v1/tasks/%s", baseUrl, taskID),
		}
	}

	var lastResp *http.Response
	var lastErr error
	for _, uri := range candidateURLs {
		req, err := http.NewRequest(http.MethodGet, uri, nil)
		if err != nil {
			lastErr = err
			continue
		}
		req.Header.Set("Authorization", "Bearer "+key)
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		// 404 / 405: endpoint not implemented — try the next candidate.
		if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusMethodNotAllowed {
			_ = resp.Body.Close()
			continue
		}
		// Any other status (200, 400, 403, 429, 5xx) is a response from a real endpoint.
		// Keep the first one; if all candidates yield 4xx/5xx, prefer the first non-404.
		if lastResp == nil || resp.StatusCode == http.StatusOK {
			if lastResp != nil {
				_ = lastResp.Body.Close()
			}
			lastResp = resp
			if resp.StatusCode == http.StatusOK || isVideo {
				break
			}
			continue
		}
		_ = resp.Body.Close()
	}

	if lastResp != nil {
		return lastResp, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("all task endpoints returned not found for task %s", taskID)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

func (a *TaskAdaptor) ParseTaskResult(task *model.Task, resp *http.Response, respBody []byte) (*relaycommon.TaskInfo, error) {
	result, err := a.ParseTaskResultBody(respBody)
	if err != nil {
		return nil, err
	}
	versionedImageContract := false
	if task != nil && task.Properties.OriginModelName == "gpt-image-2.5-sunburst" {
		inputs := task.PrivateData.ImageInputs
		versionedImageContract = inputs != nil && inputs.Version == 1
		if queue := task.PrivateData.GenerationQueue; queue != nil && queue.ImageInputs != nil && queue.ImageInputs.Version == 1 {
			versionedImageContract = true
		}
	}
	if versionedImageContract && result != nil && result.Status == model.TaskStatusSuccess {
		if err := validateExplicitImageTaskPollResult(respBody, result.Url); err != nil {
			return nil, err
		}
	}
	return result, nil
}

func (a *TaskAdaptor) ParseTaskResultBody(respBody []byte) (*relaycommon.TaskInfo, error) {
	if looksLikeImageTaskPoll(respBody) {
		return parseImageTaskPoll(respBody)
	}

	var grok grokPollResponse
	if err := common.Unmarshal(respBody, &grok); err == nil && looksLikeGrokPoll(grok) {
		return parseGrokPoll(grok), nil
	}

	resTask := responseTask{}
	if err := common.Unmarshal(respBody, &resTask); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	taskResult := relaycommon.TaskInfo{
		Code: 0,
	}

	switch resTask.Status {
	case "queued", "pending":
		taskResult.Status = model.TaskStatusQueued
	case "processing", "in_progress":
		taskResult.Status = model.TaskStatusInProgress
	case "completed":
		taskResult.Status = model.TaskStatusSuccess
		// Url intentionally left empty — the caller constructs the proxy URL using the public task ID
	case "failed", "cancelled":
		taskResult.Status = model.TaskStatusFailure
		if resTask.Error != nil {
			taskResult.Reason = resTask.Error.Message
		} else {
			taskResult.Reason = "task failed"
		}
	default:
	}
	if resTask.Progress > 0 && resTask.Progress < 100 {
		taskResult.Progress = fmt.Sprintf("%d%%", resTask.Progress)
	}

	return &taskResult, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(task *model.Task) ([]byte, error) {
	data := task.Data
	var err error
	if data, err = sjson.SetBytes(data, "id", task.TaskID); err != nil {
		return nil, errors.Wrap(err, "set id failed")
	}

	// Normalize video URL to public proxy URL if url or video.url exists,
	// preventing container-internal/upstream URLs (e.g. 127.0.0.1 / grok2api:8000) from leaking to clients.
	proxyURL := taskcommon.BuildProxyURL(task.TaskID)
	if gjson.GetBytes(data, "url").Exists() {
		if data, err = sjson.SetBytes(data, "url", proxyURL); err != nil {
			return nil, errors.Wrap(err, "set url failed")
		}
	}
	if gjson.GetBytes(data, "video.url").Exists() {
		if data, err = sjson.SetBytes(data, "video.url", proxyURL); err != nil {
			return nil, errors.Wrap(err, "set video.url failed")
		}
	}

	return data, nil
}
