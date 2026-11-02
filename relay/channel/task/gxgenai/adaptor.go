package gxgenai

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
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// TaskAdaptor implements GxgenAI (upstream RunningHub) async submit/query.
//
// fork: type 61 is dual-track:
//   - numeric App ID (>=8 digits) → POST /openapi/v2/run/ai-app/{appId} (AI App / ComfyUI)
//   - endpoint slug containing '/' → POST /openapi/v2/{endpoint} (Standard Model API)
// Poll is always POST /openapi/v2/query. Mapping lives on the channel (model → appId or slug).
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
		info.OriginModelName = req.Model
	}
	if strings.TrimSpace(info.OriginModelName) == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model is required"), "invalid_request", http.StatusBadRequest)
	}

	// Strip client billing/cost controls before any further use.
	if req.Metadata != nil {
		delete(req.Metadata, "precharge_seconds")
		delete(req.Metadata, "instanceType")
		delete(req.Metadata, "instance_type")
		delete(req.Metadata, "usePersonalQueue")
		delete(req.Metadata, "use_personal_queue")
	}

	nodes, _ := extractNodeInfoList(req.Metadata)
	if len(nodes) == 0 && strings.TrimSpace(req.Prompt) == "" {
		return service.TaskErrorWrapperLocal(
			fmt.Errorf("prompt or metadata.nodeInfoList is required"),
			"invalid_request",
			http.StatusBadRequest,
		)
	}
	if strings.TrimSpace(req.Prompt) == "" {
		req.Prompt = "gxgenai-task"
	}

	info.Action = constant.TaskActionGenerate
	c.Set("task_request", req)
	return nil
}

func (a *TaskAdaptor) EstimateBilling(c *gin.Context, info *relaycommon.RelayInfo) map[string]float64 {
	// Precharge amount is configured per model via ModelRatio (ops: est RH coins → formula in skill).
	// Framework: precharge_raw ≈ ModelRatio/2 * quota_per_unit * product(OtherRatios).
	// Return units=1 so ModelRatio alone carries the precharge; never trust client metadata.
	return map[string]float64{"units": 1}
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if info == nil {
		return "", fmt.Errorf("relay info is nil")
	}
	var target string
	if info.ChannelMeta != nil {
		target = strings.TrimSpace(info.UpstreamModelName)
	}
	if target == "" {
		target = strings.TrimSpace(info.OriginModelName)
	}
	target = strings.TrimPrefix(target, "/")
	if target == "" {
		return "", fmt.Errorf("gxgenai target model or app id missing: set channel model_mapping to RunningHub app id or standard model endpoint")
	}

	// 1. AI App: numeric App ID -> /openapi/v2/run/ai-app/{appId}
	if isLikelyAppID(target) {
		return fmt.Sprintf("%s/openapi/v2/run/ai-app/%s", a.baseURL, target), nil
	}

	// 2. Standard Model: endpoint slug with '/' -> /openapi/v2/{endpoint}
	if strings.Contains(target, "/") {
		return fmt.Sprintf("%s/openapi/v2/%s", a.baseURL, target), nil
	}

	return "", fmt.Errorf("invalid gxgenai model mapping %q: must be numeric App ID or standard endpoint slug containing '/' (e.g. alibaba/wan-3.0/reference-to-video)", target)
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
	if info == nil {
		return nil, fmt.Errorf("relay info is nil")
	}
	var target string
	if info.ChannelMeta != nil {
		target = strings.TrimSpace(info.UpstreamModelName)
	}
	if target == "" {
		target = strings.TrimSpace(info.OriginModelName)
	}
	target = strings.TrimPrefix(target, "/")

	if isLikelyAppID(target) {
		body, err := buildRunBody(info.OriginModelName, &req)
		if err != nil {
			return nil, err
		}
		data, err := common.Marshal(body)
		if err != nil {
			return nil, err
		}
		return bytes.NewReader(data), nil
	}

	// fork: Standard Model API body (prompt/duration/resolution/images; billing keys stripped).
	bodyMap, err := buildStandardModelBody(info.OriginModelName, &req)
	if err != nil {
		return nil, err
	}
	data, err := common.Marshal(bodyMap)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) ParseResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (*channel.TaskSubmitResponse, *taskdto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
	}
	parsed, err := parseSubmitResponse(responseBody)
	if err != nil {
		return nil, service.TaskErrorWrapper(errors.Wrap(err, string(responseBody)), "unmarshal_response_failed", http.StatusBadGateway)
	}
	if errorCodeNonZero(parsed.ErrorCode) && parsed.TaskID == "" {
		msg := parsed.ErrorMessage
		if msg == "" {
			msg = fmt.Sprintf("upstream error code %v", parsed.ErrorCode)
		}
		return nil, service.TaskErrorWrapperLocal(fmt.Errorf("%s", msg), "upstream_task_submit_failed", http.StatusBadRequest)
	}
	if strings.TrimSpace(parsed.TaskID) == "" {
		return nil, service.TaskErrorWrapperLocal(fmt.Errorf("upstream missing taskId"), "upstream_task_id_missing", http.StatusBadGateway)
	}

	return &channel.TaskSubmitResponse{
		UpstreamTaskID: parsed.TaskID,
		TaskData:       redactQueryBodyForStorage(responseBody),
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
	payload, err := common.Marshal(map[string]string{"taskId": taskID})
	if err != nil {
		return nil, err
	}
	url := baseUrl + "/openapi/v2/query"
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
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

func (a *TaskAdaptor) AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int {
	if task == nil {
		return 0
	}
	bc := task.PrivateData.BillingContext
	if bc == nil {
		return 0
	}

	// Free group: GroupRatio 0 must stay free — never promote to 1.
	// Return 0; settleTaskBillingOnComplete skips token fallback when GroupRatio<=0.
	groupRatio := bc.GroupRatio
	if groupRatio < 0 {
		groupRatio = 0
	}
	if groupRatio == 0 {
		return 0
	}

	// Prefer real upstream consumption. ModelRatio encodes precharge only (est coins),
	// so do NOT treat ModelRatio as "USD per wall-clock second" on settle.
	snap := extractUsageFromTaskData(task.Data)

	// RH: consumeMoney and thirdPartyConsumeMoney are independent cost lines → sum.
	// Currency: runninghub.cn = CNY major units; runninghub.ai = USD major units.
	moneyMajor := 0.0
	if snap.HasMoney {
		moneyMajor = sumConsumeMoney(snap.ConsumeMoney, snap.ThirdPartyConsumeMoney)
	}
	moneyUSD := moneyToUSD(moneyMajor, channelBaseURLForTask(task.ChannelId))

	// Sell = cost / (1 - margin) with default margin 50% → ×2.
	const sellMarginFactor = 2.0

	if moneyUSD > 0 {
		q, clamp := common.QuotaFromFloatChecked(moneyUSD * sellMarginFactor * groupRatio * common.QuotaPerUnit)
		if clamp != nil {
			common.SysError(fmt.Sprintf("gxgenai settle money clamp task=%s: %v", task.TaskID, clamp))
		}
		return q
	}
	if snap.HasCoins && snap.ConsumeCoins > 0 {
		// Coins settled via 币多多 package USD anchor (same for cn/ai consumer keys).
		costUSD := snap.ConsumeCoins * usdPerRHCoin()
		q, clamp := common.QuotaFromFloatChecked(costUSD * sellMarginFactor * groupRatio * common.QuotaPerUnit)
		if clamp != nil {
			common.SysError(fmt.Sprintf("gxgenai settle coins clamp task=%s: %v", task.TaskID, clamp))
		}
		return q
	}

	// No upstream usage fields: keep precharge (return 0 = no delta).
	// Do not read taskResult.TotalTokens as currency micros (that dual-encoding is removed).
	return 0
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	openAIVideo := dto.NewOpenAIVideo()
	openAIVideo.ID = originTask.TaskID
	openAIVideo.Status = originTask.Status.ToVideoStatus()
	openAIVideo.SetProgressStr(originTask.Progress)
	openAIVideo.CreatedAt = originTask.CreatedAt
	openAIVideo.CompletedAt = originTask.UpdatedAt
	if originTask.Properties.OriginModelName != "" {
		openAIVideo.Model = originTask.Properties.OriginModelName
	}
	if url := originTask.GetResultURL(); url != "" {
		openAIVideo.SetMetadata("url", url)
	}
	if originTask.Status == model.TaskStatusFailure && originTask.FailReason != "" {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: originTask.FailReason,
			Code:    "task_failed",
		}
	}
	return common.Marshal(openAIVideo)
}

func (a *TaskAdaptor) GetModelList() []string {
	return []string{"zimage-makeup", "zimage-makeup-async", "ltx2.3-kj", "ltx2.3-kj-async"}
}

func (a *TaskAdaptor) GetChannelName() string {
	return "gxgenai"
}
