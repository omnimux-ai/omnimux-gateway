package goeasy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// TaskAdaptor implements GoEasyAPI async image generate/edit:
//   POST /api/image-tasks/generations
//   POST /api/image-tasks/edits
//   GET  /api/image-tasks?ids=
//
// Docs: https://wpsyjp1606.apifox.cn/
const taskActionEdit = "imageEdit"

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

	// Normalize image inputs for edits
	if len(req.Images) == 0 && strings.TrimSpace(req.Image) != "" {
		req.Images = []string{strings.TrimSpace(req.Image)}
	}
	if len(req.Images) == 0 && strings.TrimSpace(req.InputReference) != "" {
		req.Images = []string{strings.TrimSpace(req.InputReference)}
	}

	path := c.Request.URL.Path
	isEdit := strings.Contains(path, "/images/edits") || strings.HasSuffix(path, "/edits")
	if isEdit {
		if len(req.Images) == 0 {
			return service.TaskErrorWrapperLocal(fmt.Errorf("images is required for image edit"), "invalid_request", http.StatusBadRequest)
		}
		info.Action = taskActionEdit
	} else {
		info.Action = constant.TaskActionGenerate
	}

	// Capture optional top-level quality parameter
	var raw map[string]any
	if err := common.UnmarshalBodyReusable(c, &raw); err == nil {
		if q, ok := raw["quality"].(string); ok && strings.TrimSpace(q) != "" {
			c.Set("task_quality", strings.TrimSpace(q))
		}
	}

	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	if c == nil {
		return map[string]float64{"units": 1}
	}
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return map[string]float64{"units": 1}
	}
	size := strings.TrimSpace(req.Size)
	size = strings.ReplaceAll(size, "×", "x")
	size = strings.ToLower(size)
	quality := extractQuality(c, req)
	qLower := strings.ToLower(quality)

	isHD := false
	if strings.Contains(size, "2048") || strings.Contains(size, "4096") || strings.Contains(size, "2k") || strings.Contains(size, "4k") {
		isHD = true
	}
	if qLower == "hd" || qLower == "pro" || qLower == "high" {
		isHD = true
	}
	if isHD {
		return map[string]float64{"units": 3}
	}
	return map[string]float64{"units": 1}
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := a.baseURL
	if base == "" {
		base = "https://api.goeasyapi.xyz"
	}
	if info != nil && info.TaskRelayInfo != nil && info.Action == taskActionEdit {
		return base + "/api/image-tasks/edits", nil
	}
	return base + "/api/image-tasks/generations", nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+info.ApiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	modelName := ""
	if info != nil {
		if info.HasChannelMeta() && info.ChannelMeta.UpstreamModelName != "" {
			modelName = strings.TrimSpace(info.ChannelMeta.UpstreamModelName)
		} else {
			modelName = strings.TrimSpace(info.OriginModelName)
		}
	}
	modelName = normalizeUpstreamModel(modelName)

	size := strings.TrimSpace(req.Size)
	if size != "" {
		size = strings.ToLower(strings.ReplaceAll(size, "×", "x"))
		// fork: GoEasy accepts pixels rather than composition ratios. Use its
		// documented 1K sizes without mutating the request used for billing,
		// history or preparation by another provider on a retry.
		switch size {
		case "1:1":
			size = "1024x1024"
		case "2:3":
			size = "1024x1536"
		case "3:2":
			size = "1536x1024"
		case "3:4":
			size = "1024x1365"
		case "4:3":
			size = "1365x1024"
		case "9:16":
			size = "1088x1920"
		case "16:9":
			size = "1920x1088"
		}
	}
	quality := extractQuality(c, req)

	// Dynamic parameter dispatch (aligned with Minimax H3 pattern):
	// For unified gpt-image-2.5, route to high-res image2.5-pro-async if requested size >= 2k
	// or quality is hd/pro; otherwise default to standard gpt-image-2.5-async.
	if modelName == "gpt-image-2.5" || modelName == "gpt-image-2.5-async" {
		isHD := false
		if strings.Contains(size, "2048") || strings.Contains(size, "4096") || strings.Contains(size, "2k") || strings.Contains(size, "4k") {
			isHD = true
		}
		qLower := strings.ToLower(quality)
		if qLower == "hd" || qLower == "pro" || qLower == "high" {
			isHD = true
		}
		if isHD {
			modelName = "image2.5-pro-async"
		} else {
			modelName = "gpt-image-2.5-async"
		}
	}

	body := map[string]any{
		"model":  modelName,
		"prompt": strings.TrimSpace(req.Prompt),
	}
	if size != "" {
		body["size"] = size
	}
	if quality != "" {
		body["quality"] = quality
	}
	if info != nil && info.TaskRelayInfo != nil && info.Action == taskActionEdit {
		body["images"] = req.Images
	}

	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func extractQuality(c *gin.Context, req relaycommon.TaskSubmitReq) string {
	if req.Metadata != nil {
		if q, ok := req.Metadata["quality"].(string); ok && strings.TrimSpace(q) != "" {
			return strings.TrimSpace(q)
		}
	}
	if c != nil {
		if q, exists := c.Get("task_quality"); exists {
			if s, ok := q.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// normalizeUpstreamModel maps friendly / legacy names to goeasy async model ids.
func normalizeUpstreamModel(name string) string {
	n := strings.TrimSpace(name)
	switch n {
	case "gpt-image-2.5", "gpt-image-2.5-async", "gpt-image-2", "image2-1k", "image2-1k-async", "gpt-image-2-async":
		return "gpt-image-2.5-async"
	case "gpt-image-2.5-hd", "image2.5-pro", "image2.5-pro-async", "image2-pro", "image2-2k4k", "image2-2k4k-async", "image2-pro-async":
		return "image2.5-pro-async"
	default:
		// If already *-async or unknown, pass through
		if strings.HasSuffix(n, "-async") {
			return n
		}
		if strings.Contains(n, "pro") || strings.Contains(n, "hd") || strings.Contains(n, "2k") || strings.Contains(n, "4k") {
			return "image2.5-pro-async"
		}
		if strings.Contains(n, "2.5") || strings.Contains(n, "image") {
			return "gpt-image-2.5-async"
		}
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, service.TaskErrorWrapper(
			fmt.Errorf("upstream status %d: %s", resp.StatusCode, string(responseBody)),
			"upstream_error",
			resp.StatusCode,
		)
	}

	var parsed goeasySubmitResponse
	if err := common.Unmarshal(responseBody, &parsed); err != nil {
		return nil, service.TaskErrorWrapper(errors.Wrap(err, string(responseBody)), "unmarshal_response_failed", http.StatusBadGateway)
	}
	if strings.TrimSpace(parsed.ID) == "" {
		return nil, service.TaskErrorWrapperLocal(fmt.Errorf("upstream missing task id"), "upstream_task_id_missing", http.StatusBadGateway)
	}
	if errMsg := strings.TrimSpace(parsed.Error); errMsg != "" && !isNonTerminalStatus(parsed.Status) {
		return nil, service.TaskErrorWrapperLocal(fmt.Errorf("%s", errMsg), "upstream_task_submit_failed", http.StatusBadRequest)
	}

	return &channel.TaskSubmitResponse{
		UpstreamTaskID: parsed.ID,
		TaskData:       responseBody,
	}, nil
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
		baseUrl = "https://api.goeasyapi.xyz"
	}
	u, err := url.Parse(baseUrl + "/api/image-tasks")
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("ids", taskID)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
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
	return parseQueryToTaskInfo(respBody)
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{
		"gpt-image-2.5-async",
		"image2.5-pro-async",
	}
}

func (a *TaskAdaptor) GetChannelName() string {
	return "goeasyapi"
}

// ---------------------------------------------------------------------------

type goeasySubmitResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Mode      string `json:"mode"`
	Model     string `json:"model"`
	Size      string `json:"size"`
	Quality   string `json:"quality"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
	Error     string `json:"error"`
}

type goeasyQueryResponse struct {
	Items      []goeasyTaskItem `json:"items"`
	MissingIDs []string         `json:"missing_ids"`
}

type goeasyTaskItem struct {
	ID         string              `json:"id"`
	Status     string              `json:"status"`
	Mode       string              `json:"mode"`
	Model      string              `json:"model"`
	Size       string              `json:"size"`
	Quality    string              `json:"quality"`
	CreatedAt  string              `json:"created_at"`
	UpdatedAt  string              `json:"updated_at"`
	Data       []goeasyImageResult `json:"data"`
	Error      string              `json:"error"`
	Progress   string              `json:"progress"`
	DurationMs int                 `json:"duration_ms"`
}

type goeasyImageResult struct {
	URL            string `json:"url"`
	B64JSON        string `json:"b64_json"`
	RevisedPrompt  string `json:"revised_prompt"`
}

func parseQueryToTaskInfo(respBody []byte) (*relaycommon.TaskInfo, error) {
	var q goeasyQueryResponse
	if err := common.Unmarshal(respBody, &q); err != nil {
		return nil, err
	}
	if len(q.Items) == 0 {
		if len(q.MissingIDs) > 0 {
			return relaycommon.FailTaskInfo("task not found upstream"), nil
		}
		return nil, fmt.Errorf("empty task query items")
	}
	item := q.Items[0]
	info := &relaycommon.TaskInfo{
		TaskID: item.ID,
	}
	switch strings.ToLower(strings.TrimSpace(item.Status)) {
	case "success", "succeeded", "completed", "done":
		info.Status = string(model.TaskStatusSuccess)
		info.Progress = taskcommon.ProgressComplete
		if len(item.Data) > 0 {
			if u := strings.TrimSpace(item.Data[0].URL); u != "" {
				info.Url = u
				info.RemoteUrl = u
			}
		}
		if info.Url == "" && strings.TrimSpace(item.Error) != "" {
			info.Status = string(model.TaskStatusFailure)
			info.Reason = strings.TrimSpace(item.Error)
		}
	case "error", "failed", "failure", "cancelled", "canceled":
		info.Status = string(model.TaskStatusFailure)
		info.Reason = strings.TrimSpace(item.Error)
		if info.Reason == "" {
			info.Reason = item.Status
		}
	case "queued", "pending", "submitted":
		info.Status = string(model.TaskStatusQueued)
		info.Progress = taskcommon.ProgressQueued
	case "running", "processing", "in_progress":
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = taskcommon.ProgressInProgress
		if p := strings.TrimSpace(item.Progress); p != "" {
			info.Progress = p
		}
	default:
		info.Status = string(model.TaskStatusInProgress)
		info.Progress = taskcommon.ProgressInProgress
	}
	return info, nil
}

func isNonTerminalStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "queued", "pending", "processing", "running", "submitted", "":
		return true
	default:
		return false
	}
}

func mapUpstreamStatusToClient(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "succeeded", "completed", "done":
		return "completed"
	case "error", "failed", "failure":
		return "failed"
	case "processing", "running":
		return "processing"
	default:
		return "pending"
	}
}
