package relay

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	taskvolcvoice "github.com/QuantumNous/new-api/relay/channel/task/volcvoice"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

func AudioHelper(c *gin.Context, info *relaycommon.RelayInfo) (newAPIError *types.NewAPIError) {
	info.InitChannelMeta(c)

	audioReq, ok := info.Request.(*dto.AudioRequest)
	if !ok {
		return types.NewError(errors.New("invalid request type"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	request, err := common.DeepCopy(audioReq)
	if err != nil {
		return types.NewError(fmt.Errorf("failed to copy request to AudioRequest: %w", err), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	err = helper.ModelMappedHelper(c, info, request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeChannelModelMappedError, types.ErrOptionWithSkipRetry())
	}

	// fork: 平台训练出的音色只允许其所有者使用（见 model.CheckVoiceCloneAccess）。
	// 授权判定属于处理器职责，放在这里而不是渠道适配器内，避免纯请求转换函数依赖数据库。
	//
	// 只对平台命名空间内的音色发起归属查询：官方预置音色（509 款）以及任何外部
	// 音色代号都不带该前缀，因此既有配音流量不增加任何数据库开销。平台音色代号
	// 由训练适配器强制落在该命名空间内，不会绕过本校验。
	if strings.HasPrefix(request.Voice, taskvolcvoice.VoiceNamespacePrefix) {
		allowed, ownErr := model.CheckVoiceCloneAccess(info.UserId, request.Voice)
		if ownErr != nil {
			return types.NewError(fmt.Errorf("voice ownership check failed: %w", ownErr), types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
		}
		if !allowed {
			return types.NewError(fmt.Errorf("voice %q belongs to another user", request.Voice), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
		}
	}

	adaptor := GetAdaptor(info.ApiType)
	if adaptor == nil {
		return types.NewError(fmt.Errorf("invalid api type: %d", info.ApiType), types.ErrorCodeInvalidApiType, types.ErrOptionWithSkipRetry())
	}
	adaptor.Init(info)

	ioReader, err := adaptor.ConvertAudioRequest(c, info, *request)
	if err != nil {
		return types.NewError(err, types.ErrorCodeConvertRequestFailed, types.ErrOptionWithSkipRetry())
	}

	resp, err := adaptor.DoRequest(c, info, ioReader)
	if err != nil {
		return types.NewOpenAIError(err, types.ErrorCodeDoRequestFailed, http.StatusInternalServerError)
	}
	statusCodeMappingStr := c.GetString("status_code_mapping")

	var httpResp *http.Response
	if resp != nil {
		httpResp = resp.(*http.Response)
		if httpResp.StatusCode != http.StatusOK {
			newAPIError = service.RelayErrorHandler(c.Request.Context(), httpResp, false)
			// reset status code 重置状态码
			service.ResetStatusCode(newAPIError, statusCodeMappingStr)
			return newAPIError
		}
	}

	usage, newAPIError := adaptor.DoResponse(c, httpResp, info)
	if newAPIError != nil {
		// reset status code 重置状态码
		service.ResetStatusCode(newAPIError, statusCodeMappingStr)
		return newAPIError
	}
	if usage != nil {
		if u, ok := usage.(*dto.Usage); ok && u != nil {
			if u.CompletionTokenDetails.AudioTokens > 0 || u.PromptTokensDetails.AudioTokens > 0 {
				service.PostAudioConsumeQuota(c, info, u, nil)
			} else {
				service.PostTextConsumeQuota(c, info, u, nil)
			}
		}
	}

	return nil
}
