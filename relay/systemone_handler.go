package relay

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// SystemOneHelper 处理 /v1/systemone 专有协议请求：
// 转发至 TypeSafe AI 官方端点，并按输入 Token 计量扣除 Quota（输出免费）。
func SystemOneHelper(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	info.InitChannelMeta(c)

	sysOneReq, ok := info.Request.(*dto.SystemOneRequest)
	if !ok {
		return types.NewErrorWithStatusCode(fmt.Errorf("invalid request type, expected *dto.SystemOneRequest, got %T", info.Request), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(sysOneReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy SystemOneRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	if err := helper.ModelMappedHelper(c, info, request); err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	// 读取原始请求体，并将 model 替换为映射后的上游模型名称（如 jev -> jev-latest）
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
	rawBytes, err := storage.Bytes()
	if err != nil {
		return types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	modelName := request.Model
	if info.UpstreamModelName != "" {
		modelName = info.UpstreamModelName
	}
	var jsonMap map[string]json.RawMessage
	if err := common.Unmarshal(rawBytes, &jsonMap); err != nil || jsonMap == nil {
		// JSON null has no object fields to patch. Forward the validated DTO
		// instead so the default/mapped model is not silently lost.
		request.SetModelName(modelName)
		rawBytes, err = common.Marshal(request)
		if err != nil {
			return types.NewErrorWithStatusCode(fmt.Errorf("failed to encode System One request: %w", err), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
	} else {
		if modelName != "" {
			modelBytes, err := common.Marshal(modelName)
			if err != nil {
				return types.NewErrorWithStatusCode(fmt.Errorf("failed to encode System One model: %w", err), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			jsonMap["model"] = modelBytes
		}
		modifiedBytes, err := common.Marshal(jsonMap)
		if err != nil {
			return types.NewErrorWithStatusCode(fmt.Errorf("failed to encode System One request: %w", err), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
		}
		rawBytes = modifiedBytes
	}

	requestBody := bytes.NewReader(rawBytes)
	resp, err := adaptor.DoRequest(c, info, requestBody)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}

	statusCodeMappingStr := c.GetString("status_code_mapping")
	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			newAPIError := service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}
	if usage != nil {
		if u, ok := usage.(*dto.Usage); ok {
			service.PostTextConsumeQuota(c, info, u, nil)
		}
	}
	return nil
}
