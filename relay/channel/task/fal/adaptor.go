package fal

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

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	baseURL     string
	apiKey      string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	if info != nil && info.ChannelMeta != nil {
		a.ChannelType = info.ChannelType
		a.baseURL = strings.TrimRight(info.ChannelBaseUrl, "/")
		a.apiKey = info.ApiKey
	}
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *taskdto.TaskError {
	var raw map[string]any
	if err := common.UnmarshalBodyReusable(c, &raw); err != nil {
		return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
	}

	modelName := strings.TrimSpace(info.GetUpstreamModelName())
	if modelName == "" {
		modelName = strings.TrimSpace(info.OriginModelName)
	}
	if modelName == "" {
		if m, ok := raw["model"].(string); ok {
			modelName = strings.TrimSpace(m)
			info.OriginModelName = modelName
		}
	}
	if modelName == "" {
		return service.TaskErrorWrapperLocal(fmt.Errorf("model is required"), "invalid_request", http.StatusBadRequest)
	}

	if !IsSupportedModel(modelName) && !IsSupportedModel(info.OriginModelName) {
		return service.TaskErrorWrapperLocal(fmt.Errorf("unsupported model: %s", modelName), "unsupported_model", http.StatusBadRequest)
	}

	endpoint := ResolveFalEndpoint(modelName, raw)
	if info.ChannelMeta == nil {
		info.ChannelMeta = &relaycommon.ChannelMeta{}
	}
	info.UpstreamModelName = endpoint
	c.Set("fal_resolved_endpoint", endpoint)

	if info.TaskRelayInfo == nil {
		info.TaskRelayInfo = &relaycommon.TaskRelayInfo{}
	}
	info.Action = constant.TaskActionGenerate

	if endpoint == ModelMinimaxH3MaxLipSyncImageToVideo {
		if _, err := buildLipSyncPayload(raw); err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
	} else {
		if _, err := buildSubmitPayload(endpoint, raw); err != nil {
			return service.TaskErrorWrapperLocal(err, "invalid_request", http.StatusBadRequest)
		}
	}

	c.Set("task_request_raw", raw)
	return nil
}

func (a *TaskAdaptor) BuildRequestURL(info *relaycommon.RelayInfo) (string, error) {
	base := a.baseURL
	if base == "" {
		base = DefaultBaseURL
	}
	endpoint := ""
	if info != nil && info.GetUpstreamModelName() != "" {
		endpoint = GetFalEndpoint(info.GetUpstreamModelName())
	} else if info != nil && info.OriginModelName != "" {
		endpoint = GetFalEndpoint(info.OriginModelName)
	}
	return fmt.Sprintf("%s/%s", strings.TrimRight(base, "/"), endpoint), nil
}

func (a *TaskAdaptor) BuildRequestHeader(c *gin.Context, req *http.Request, info *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	key := a.apiKey
	if info != nil && info.ChannelMeta != nil && info.ApiKey != "" {
		key = info.ApiKey
	}
	req.Header.Set("Authorization", "Key "+cleanAPIKey(key))
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	raw, _ := c.Get("task_request_raw")
	body, ok := raw.(map[string]any)
	if !ok || body == nil {
		body = map[string]any{}
		_ = common.UnmarshalBodyReusable(c, &body)
	}

	endpoint := ""
	if ep, ok := c.Get("fal_resolved_endpoint"); ok {
		if s, ok := ep.(string); ok && s != "" {
			endpoint = s
		}
	}
	if endpoint == "" {
		if info != nil && info.GetUpstreamModelName() != "" {
			endpoint = GetFalEndpoint(info.GetUpstreamModelName())
		} else if info != nil && info.OriginModelName != "" {
			endpoint = GetFalEndpoint(info.OriginModelName)
		}
	}

	var data []byte
	if endpoint == ModelMinimaxH3MaxLipSyncImageToVideo {
		payload, err := buildLipSyncPayload(body)
		if err != nil {
			return nil, err
		}
		data, err = common.Marshal(payload)
		if err != nil {
			return nil, err
		}
	} else {
		payload, err := buildSubmitPayload(endpoint, body)
		if err != nil {
			return nil, err
		}
		data, err = common.Marshal(payload)
		if err != nil {
			return nil, err
		}
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
	_ = resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, service.TaskErrorWrapper(
			fmt.Errorf("upstream status %d: %s", resp.StatusCode, string(responseBody)),
			"upstream_error",
			resp.StatusCode,
		)
	}

	var submitResp FalSubmitResponse
	if err := common.Unmarshal(responseBody, &submitResp); err != nil {
		return nil, service.TaskErrorWrapper(errors.Wrap(err, string(responseBody)), "invalid_upstream_response", http.StatusBadGateway)
	}

	reqID := strings.TrimSpace(submitResp.RequestID)
	if reqID == "" {
		return nil, service.TaskErrorWrapper(fmt.Errorf("request_id missing in upstream response: %s", string(responseBody)), "invalid_upstream_response", http.StatusBadGateway)
	}

	endpoint := ""
	if ep, ok := c.Get("fal_resolved_endpoint"); ok {
		if s, ok := ep.(string); ok && s != "" {
			endpoint = s
		}
	}
	if endpoint == "" {
		if info != nil && info.GetUpstreamModelName() != "" {
			endpoint = GetFalEndpoint(info.GetUpstreamModelName())
		} else if info != nil && info.OriginModelName != "" {
			endpoint = GetFalEndpoint(info.OriginModelName)
		}
	}

	upstreamEndpoint := endpoint
	if endpoint == ModelMinimaxH3MaxLipSyncImageToVideo {
		upstreamEndpoint = ModelMinimaxH3Max
	}
	upstreamID := FormatUpstreamTaskID(upstreamEndpoint, reqID)
	return &channel.TaskSubmitResponse{
		UpstreamTaskID: upstreamID,
		TaskData:       responseBody,
	}, nil
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *taskdto.TaskError) {
	parsed, err := a.ParseResponse(c, resp, info)
	if err != nil {
		return "", nil, err
	}
	return parsed.UpstreamTaskID, parsed.TaskData, nil
}

func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	openAIVideo := originTask.ToOpenAIVideo()
	if originTask.Status == model.TaskStatusFailure {
		openAIVideo.Error = &dto.OpenAIVideoError{
			Message: originTask.FailReason,
			Code:    "task_failed",
		}
	}
	out := map[string]any{
		"id":           openAIVideo.ID,
		"task_id":      openAIVideo.ID,
		"object":       "video",
		"model":        openAIVideo.Model,
		"status":       openAIVideo.Status,
		"progress":     openAIVideo.Progress,
		"created_at":   openAIVideo.CreatedAt,
		"completed_at": openAIVideo.CompletedAt,
		"url":          originTask.GetResultURL(),
		"metadata":     openAIVideo.Metadata,
	}
	if openAIVideo.Error != nil {
		out["error"] = openAIVideo.Error
	}
	return common.Marshal(out)
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}
