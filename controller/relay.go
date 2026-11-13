package controller

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/QuantumNous/new-api/model"
	pluginruntime "github.com/QuantumNous/new-api/pkg/jsplugin"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay"
	"github.com/QuantumNous/new-api/relay/channel/task/sora"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relay/helper"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

func relayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	switch info.RelayMode {
	case relayconstant.RelayModeImagesGenerations, relayconstant.RelayModeImagesEdits:
		err = relay.ImageHelper(c, info)
	case relayconstant.RelayModeAudioSpeech:
		fallthrough
	case relayconstant.RelayModeAudioTranslation:
		fallthrough
	case relayconstant.RelayModeAudioTranscription:
		err = relay.AudioHelper(c, info)
	case relayconstant.RelayModeRerank:
		err = relay.RerankHelper(c, info)
	case relayconstant.RelayModeReader:
		err = relay.ReaderHelper(c, info)
	case relayconstant.RelayModeSystemOne:
		err = relay.SystemOneHelper(c, info)
	case relayconstant.RelayModeEmbeddings:
		err = relay.EmbeddingHelper(c, info)
	case relayconstant.RelayModeResponses, relayconstant.RelayModeResponsesCompact:
		err = relay.ResponsesHelper(c, info)
	case relayconstant.RelayModeAlphaSearch:
		err = relay.AlphaSearchHelper(c, info)
	default:
		err = relay.TextHelper(c, info)
	}
	return err
}

func geminiRelayHandler(c *gin.Context, info *relaycommon.RelayInfo) *types.NewAPIError {
	var err *types.NewAPIError
	if strings.Contains(c.Request.URL.Path, "embed") {
		err = relay.GeminiEmbeddingHandler(c, info)
	} else {
		err = relay.GeminiHelper(c, info)
	}
	return err
}

func Relay(c *gin.Context, relayFormat types.RelayFormat) {

	requestId := c.GetString(common.RequestIdKey)
	//group := common.GetContextKeyString(c, constant.ContextKeyUsingGroup)
	//originalModel := common.GetContextKeyString(c, constant.ContextKeyOriginalModel)

	var (
		newAPIError *types.NewAPIError
		ws          *websocket.Conn
	)

	if relayFormat == types.RelayFormatOpenAIRealtime {
		var err error
		ws, err = upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			helper.WssError(c, ws, types.NewError(err, types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry()).ToOpenAIError())
			return
		}
		defer ws.Close()
	}

	defer func() {
		if newAPIError != nil {
			service.RecordRequestPolicyTermination(c, newAPIError)
			logger.LogError(c, common.LogText("relay error: %s", common.LocalLogPreview(newAPIError.Error())))
			newAPIError.SetMessage(common.MessageWithRequestId(newAPIError.Error(), requestId))
			switch relayFormat {
			case types.RelayFormatOpenAIRealtime:
				// fork: final billing has finished and relay writers have joined.
				// Start the terminal-frame timeout at its write, not at relay return.
				_ = ws.SetWriteDeadline(time.Now().Add(time.Second))
				helper.WssError(c, ws, newAPIError.ToOpenAIError())
			case types.RelayFormatClaude:
				c.JSON(newAPIError.StatusCode, gin.H{
					"type":  "error",
					"error": newAPIError.ToClaudeError(),
				})
			default:
				writeOpenAIRelayError(c, relayFormat, newAPIError)
			}
		}
	}()

	request, err := helper.GetAndValidateRequest(c, relayFormat)
	if err != nil {
		// Map "request body too large" to 413 so clients can handle it correctly
		if common.IsRequestBodyTooLargeError(err) || errors.Is(err, common.ErrRequestBodyTooLarge) {
			newAPIError = types.NewErrorWithStatusCode(err, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
		} else {
			newAPIError = types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithStatusCode(http.StatusBadRequest), types.ErrOptionWithSkipRetry())
		}
		return
	}

	relayInfo, err := relaycommon.GenRelayInfo(c, relayFormat, request, ws)
	if err != nil {
		newAPIError = types.NewError(err, types.ErrorCodeGenRelayInfoFailed)
		return
	}

	defer func() {
		recovered := recover()
		resultErr := newAPIError
		if recovered != nil {
			resultErr = types.NewError(fmt.Errorf("relay panic: %v", recovered), types.ErrorCodeBadResponse)
		}
		if relayFormat != types.RelayFormatOpenAIRealtime {
			perfmetrics.RecordRelayResult(c.Request.Context(), relayInfo, resultErr)
		}
		if recovered != nil {
			panic(recovered)
		}
	}()

	if newAPIError = relay.PrepareRequestBilling(c, relayInfo); newAPIError != nil {
		return
	}
	defer func() {
		newAPIError = relay.RefundFailedRequestBilling(c, relayInfo, newAPIError)
	}()

	retryParam := &service.RetryParam{
		Ctx:         c,
		TokenGroup:  relayInfo.TokenGroup,
		ModelName:   relayInfo.OriginModelName,
		RequestPath: c.Request.URL.Path,
		Retry:       common.GetPointer(0),
	}
	relayInfo.RetryIndex = 0
	relayInfo.LastError = nil
	// fork: only this audited native model/group gets four distinct routes.
	// Lock auto-selected traffic to that group and retain the total budget while
	// excluding prior channels, so removing one priority cannot skip the next.
	retryLimit := common.RetryTimes
	argonNativeRetry := service.ConfigureArgonNativeRetry(c, relayInfo)
	if argonNativeRetry {
		retryLimit = 3
		retryParam.TokenGroup = relayInfo.UsingGroup
		retryParam.PreserveRetryBudget = true
	}

	for ; retryParam.GetRetry() <= retryLimit; retryParam.IncreaseRetry() {
		service.BeginArgonNativeAttempt(c)
		relayInfo.StreamStatus = nil
		relayInfo.PerformanceBusinessRejection = false
		relayInfo.PerformanceOutputTokens = 0
		relayInfo.RetryIndex = retryParam.GetRetry()
		channel, channelErr := getChannel(c, relayInfo, retryParam)
		if channelErr != nil {
			logger.LogError(c, channelErr.Error())
			newAPIError = channelErr
			// SetupContext selected this candidate but could not obtain a key;
			// no request could have reached its supplier.
			if argonNativeRetry && channelErr.GetErrorType() == types.ErrorTypeNewAPIError && channelErr.GetErrorCode() == types.ErrorCodeChannelNoAvailableKey {
				channelID := common.GetContextKeyInt(c, constant.ContextKeyChannelId)
				service.AppendUsedChannel(c, channelID)
				service.MarkArgonNativeNotSent(c)
				decision := service.DecideArgonNativeRetry(c, channelErr, retryLimit-retryParam.GetRetry())
				// No supplier failed and no health action ran at key selection.
				service.RequestPolicy(c).AddEvent(service.PolicyEvent{ChannelID: channelID, ErrorCode: string(channelErr.GetErrorCode()), ErrorSource: "channel", Decision: decision, Health: "unchanged"})
				if decision.Action == "retry" {
					service.ExcludeAttemptedChannel(c, channelID)
					continue
				}
			}
			break
		}
		service.AppendUsedChannel(c, channel.Id)
		if billingErr := service.PrepareTieredBillingForSelectedGroup(c, relayInfo); billingErr != nil {
			newAPIError = billingErr
			break
		}

		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			// Ensure consistent 413 for oversized bodies even when error occurs later (e.g., retry path)
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusRequestEntityTooLarge, types.ErrOptionWithSkipRetry())
			} else {
				newAPIError = types.NewErrorWithStatusCode(bodyErr, types.ErrorCodeReadRequestBodyFailed, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		switch relayFormat {
		case types.RelayFormatOpenAIRealtime:
			newAPIError = relay.WssHelper(c, relayInfo)
		case types.RelayFormatClaude:
			newAPIError = relay.ClaudeHelper(c, relayInfo)
		case types.RelayFormatGemini:
			newAPIError = geminiRelayHandler(c, relayInfo)
		default:
			newAPIError = relayHandler(c, relayInfo)
		}

		if newAPIError == nil {
			service.MarkRequestPolicySuccess(c, relayInfo.StreamStatus)
			relayInfo.LastError = nil
			return
		}

		newAPIError = service.NormalizeViolationFeeError(newAPIError)
		relayInfo.LastError = newAPIError

		decision := service.DecideArgonNativeRetry(c, newAPIError, retryLimit-retryParam.GetRetry())
		service.RecordPolicyFailure(c, channel.Id, newAPIError, decision)
		processChannelError(c, *types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey, common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()), newAPIError, relayInfo)

		if decision.Action != "retry" {
			break
		}
		if argonNativeRetry {
			service.ExcludeAttemptedChannel(c, channel.Id)
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := common.LogText("retry: %s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}
}

// fork: normalize only local account-quota failures at the OpenAI HTTP boundary.
// Internal billing errors and upstream errors retain their retry/channel semantics.
func writeOpenAIRelayError(c *gin.Context, format types.RelayFormat, apiErr *types.NewAPIError) {
	status := apiErr.StatusCode
	body := apiErr.ToOpenAIError()
	openAIText := format == types.RelayFormatOpenAI || format == types.RelayFormatOpenAIResponses || format == types.RelayFormatOpenAIResponsesCompaction
	if openAIText && !c.Writer.Written() && apiErr.GetErrorType() == types.ErrorTypeNewAPIError &&
		apiErr.GetErrorCode() == types.ErrorCodeInsufficientUserQuota && status == http.StatusForbidden {
		status = http.StatusPaymentRequired
		body.Type = "insufficient_quota"
		body.Code = "insufficient_quota"
		// DSH classifies flattened messages; amounts such as 403.00 can look
		// like an auth status. Keep numeric diagnostics in the server log.
		body.Message = common.MessageWithRequestId("insufficient_quota: Account balance or subscription quota is insufficient for this request. 用户余额或订阅额度不足，无法完成此次请求。", c.GetString(common.RequestIdKey))
	}
	c.JSON(status, gin.H{"error": body})
}

// CountClaudeTokens implements Anthropic's token-counting utility endpoint.
// It deliberately skips upstream generation and billing; callers use this
// endpoint to size prompts before creating a Message.
func CountClaudeTokens(c *gin.Context) {
	request, err := helper.GetAndValidateClaudeRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "invalid_request_error",
				"message": common.MessageWithRequestId(err.Error(), c.GetString(common.RequestIdKey)),
			},
		})
		return
	}

	info := relaycommon.GenRelayInfoClaude(c, request)
	inputTokens, err := service.CountRequestToken(c, request.GetTokenCountMeta(), info)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"type": "error",
			"error": gin.H{
				"type":    "api_error",
				"message": common.MessageWithRequestId(err.Error(), c.GetString(common.RequestIdKey)),
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"input_tokens": inputTokens})
}

var upgrader = websocket.Upgrader{
	Subprotocols: []string{"realtime", "responses"}, // WS 握手支持的协议，如果有使用 Sec-WebSocket-Protocol，则必须在此声明对应的 Protocol
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许跨域
	},
}

// argonInitialSetupComplete reads the selected context without advancing the
// channel's shared key index. Membership checks reject stale partial context.
func argonInitialSetupComplete(c *gin.Context, channel *model.Channel) bool {
	lock := model.GetChannelPollingLock(channel.Id)
	lock.Lock()
	defer lock.Unlock()
	key, hasKey := common.GetContextKeyType[string](c, constant.ContextKeyChannelKey)
	baseURL, hasBaseURL := common.GetContextKeyType[string](c, constant.ContextKeyChannelBaseUrl)
	if !hasKey || !hasBaseURL || baseURL != channel.GetBaseURL() {
		return false
	}
	if !channel.ChannelInfo.IsMultiKey {
		return key == channel.Key && !common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey)
	}
	index, hasIndex := common.GetContextKeyType[int](c, constant.ContextKeyChannelMultiKeyIndex)
	keys := channel.GetKeys()
	if !hasIndex || !common.GetContextKeyBool(c, constant.ContextKeyChannelIsMultiKey) || index < 0 || index >= len(keys) || key != keys[index] {
		return false
	}
	status, found := channel.ChannelInfo.MultiKeyStatusList[index]
	return !found || status == common.ChannelStatusEnabled
}

func getChannel(c *gin.Context, info *relaycommon.RelayInfo, retryParam *service.RetryParam) (*model.Channel, *types.NewAPIError) {
	argonNativeRetry := service.ArgonNativeRetryConfigured(c)
	if info.ChannelMeta == nil && (!argonNativeRetry || retryParam.GetRetry() == 0) {
		if argonNativeRetry {
			// fork: Distribute does not propagate its setup error. Complete a
			// missing setup, but retain a healthy key already selected there.
			channel, err := model.CacheGetChannel(c.GetInt("channel_id"))
			if err != nil || channel == nil || channel.Status != common.ChannelStatusEnabled {
				return nil, types.NewError(errors.New(i18n.T(c, i18n.MsgRelayRetryNoAvailableChannel, map[string]any{"Group": info.UsingGroup, "Model": info.OriginModelName})), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
			}
			service.RequestPolicy(c).BeginAttempt(channel, info.UsingGroup)
			if !argonInitialSetupComplete(c, channel) {
				if apiErr := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName); apiErr != nil {
					return nil, apiErr
				}
			}
			return channel, nil
		}
		autoBan := c.GetBool("auto_ban")
		autoBanInt := 1
		if !autoBan {
			autoBanInt = 0
		}
		channel := &model.Channel{
			Id:      c.GetInt("channel_id"),
			Type:    c.GetInt("channel_type"),
			Name:    c.GetString("channel_name"),
			AutoBan: &autoBanInt,
		}
		service.RequestPolicy(c).BeginAttempt(channel, info.UsingGroup)
		return channel, nil
	}
	channel, selectGroup, err := service.CacheGetRandomSatisfiedChannel(retryParam)
	if err != nil {
		return nil, types.NewError(errors.New(i18n.T(c, i18n.MsgRelayRetryGetChannelFailed, map[string]any{"Group": selectGroup, "Model": info.OriginModelName, "Error": err.Error()})), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}
	if channel == nil {
		return nil, types.NewError(errors.New(i18n.T(c, i18n.MsgRelayRetryNoAvailableChannel, map[string]any{"Group": selectGroup, "Model": info.OriginModelName})), types.ErrorCodeGetChannelFailed, types.ErrOptionWithSkipRetry())
	}

	info.PriceData.GroupRatioInfo = helper.HandleGroupRatio(c, info)

	service.RequestPolicy(c).BeginAttempt(channel, selectGroup)
	newAPIError := middleware.SetupContextForSelectedChannel(c, channel, info.OriginModelName)
	if newAPIError != nil {
		return nil, newAPIError
	}
	return channel, nil
}

func processChannelError(c *gin.Context, channelError types.ChannelError, err *types.NewAPIError, relayInfo *relaycommon.RelayInfo) {
	service.ProcessChannelError(c, channelError, err, relayInfo)
}

func RelayMidjourney(c *gin.Context) {
	policy := service.RequestPolicy(c)
	defer func() {
		if policy.Attempts > 0 && !policy.Successful {
			service.RecordRequestPolicyTermination(c, types.NewErrorWithStatusCode(errors.New("Midjourney submission failed"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway, types.ErrOptionWithSkipRetry()))
		}
	}()
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatMjProxy, nil, nil)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"description": fmt.Sprintf("failed to generate relay info: %s", err.Error()),
			"type":        "upstream_error",
			"code":        4,
		})
		return
	}

	var mjErr *taskdto.MidjourneyResponse
	switch relayInfo.RelayMode {
	case relayconstant.RelayModeMidjourneyNotify:
		mjErr = relay.RelayMidjourneyNotify(c)
	case relayconstant.RelayModeMidjourneyTaskFetch, relayconstant.RelayModeMidjourneyTaskFetchByCondition:
		mjErr = relay.RelayMidjourneyTask(c, relayInfo.RelayMode)
	case relayconstant.RelayModeMidjourneyTaskImageSeed:
		mjErr = relay.RelayMidjourneyTaskImageSeed(c)
	case relayconstant.RelayModeSwapFace:
		mjErr = relay.RelaySwapFace(c, relayInfo)
	default:
		mjErr = relay.RelayMidjourneySubmit(c, relayInfo)
	}
	//err = relayMidjourneySubmit(c, relayMode)
	log.Println(mjErr)
	if mjErr != nil {
		policy.Successful = false
		statusCode := http.StatusBadRequest
		if mjErr.Code == 30 {
			mjErr.Result = i18n.T(c, i18n.MsgRelayGroupSaturated)
			statusCode = http.StatusTooManyRequests
		}
		c.JSON(statusCode, gin.H{
			"description": fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result),
			"type":        "upstream_error",
			"code":        mjErr.Code,
		})
		channelId := c.GetInt("channel_id")
		logger.LogError(c, common.LogText("relay error (channel #%d, status code %d): %s", channelId, statusCode, fmt.Sprintf("%s %s", mjErr.Description, mjErr.Result)))
	}
}

func RelayNotImplemented(c *gin.Context) {
	err := types.OpenAIError{
		Message: "API not implemented",
		Type:    "new_api_error",
		Param:   "",
		Code:    "api_not_implemented",
	}
	c.JSON(http.StatusNotImplemented, gin.H{
		"error": err,
	})
}

func RelayNotFound(c *gin.Context) {
	// The web fallback may already have applied static-asset cache headers.
	// A missing API or asset can appear after an upgrade; never cache its 404.
	c.Header("Cache-Control", "no-store, no-cache, must-revalidate, private, max-age=0")
	c.Header("Pragma", "no-cache")
	c.Header("Expires", "0")
	err := types.OpenAIError{
		Message: fmt.Sprintf("Invalid URL (%s %s)", c.Request.Method, c.Request.URL.Path),
		Type:    "invalid_request_error",
		Param:   "",
		Code:    "",
	}
	c.JSON(http.StatusNotFound, gin.H{
		"error": err,
	})
}

// RelayTaskPluginEndpoint keeps unclaimed shared-endpoint traffic on its
// existing handler while claimed requests enter the generation-pinned
// host-owned protocol bridge.
func RelayTaskPluginEndpoint(c *gin.Context, fallback gin.HandlerFunc) {
	pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedEndpoint)
	if !exists {
		fallback(c)
		return
	}
	pinned, ok := pinnedValue.(pluginruntime.PinnedEndpoint)
	if !ok || pinned.Plugin == nil || pinned.Generation == nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{
				"message": "Task protocol request failed",
				"type":    "new_api_error",
				"code":    "task_protocol_error",
			},
		})
		return
	}
	switch pinned.Protocol {
	case "openai_responses":
		serveTaskPluginProtocol(c, pinned, defaultPluginProtocolBridgeDeps())
	case pluginruntime.ProtocolOpenAIImage:
		serveTaskPluginImageProtocol(c, pinned, defaultPluginProtocolBridgeDeps())
	default:
		fallback(c)
	}
}

func RelayTaskFetch(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, &taskdto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if taskErr := relay.RelayTaskFetch(c, relayInfo.RelayMode); taskErr != nil {
		respondTaskError(c, taskErr)
	}
}

// RelayImage routes OpenAI-compatible image endpoints.
// GoEasyAPI / ApiMart async image (e.g. Midjourney) and async image models (e.g. grok-imagine-image on type 1/Sora)
// use the task submit path; other channels/models keep sync ImageHelper.
func RelayImage(c *gin.Context) {
	channelType := common.GetContextKeyInt(c, constant.ContextKeyChannelType)
	originalModel := c.GetString("original_model")
	if channelType == constant.ChannelTypeGoEasyAPI || channelType == constant.ChannelTypeApiMart ||
		((channelType == constant.ChannelTypeOpenAI || channelType == constant.ChannelTypeSora) && sora.IsAsyncImageModel(originalModel)) {
		// Reuse video submit mode so task fetch builders and distributor paths stay consistent.
		if _, ok := c.Get("relay_mode"); !ok {
			c.Set("relay_mode", relayconstant.RelayModeVideoSubmit)
		}
		RelayTask(c)
		return
	}
	Relay(c, types.RelayFormatOpenAIImage)
}

type taskSubmissionOutcome struct {
	Result    *relay.TaskSubmitResult
	Task      *model.Task
	RelayInfo *relaycommon.RelayInfo
}

func RelayTask(c *gin.Context) {
	relayInfo, err := relaycommon.GenRelayInfo(c, types.RelayFormatTask, nil, nil)
	if err != nil {
		respondTaskSubmissionError(c, &taskdto.TaskError{
			Code:       "gen_relay_info_failed",
			Message:    err.Error(),
			StatusCode: http.StatusInternalServerError,
		})
		return
	}
	if action := c.GetString("task_action"); action != "" {
		relayInfo.Action = action
	}

	if taskErr := relay.ResolveOriginTask(c, relayInfo); taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}
	if taskErr := relay.ApplyOriginTaskAffinity(c, relayInfo); taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}

	outcome, taskErr := executeTaskSubmission(c, relayInfo)
	if taskErr != nil {
		respondTaskSubmissionError(c, taskErr)
		return
	}
	presentTaskSubmission(c, outcome)
}

// executeTaskSubmission owns the retry, billing, and persistence lifecycle.
// It deliberately performs no client response writes so JSON and protocol
// presenters share the same durable task barrier. Its cancellation semantics
// come from c.Request.Context: native task endpoints use the client context,
// while the Responses bridge supplies an independently bounded context.
func executeTaskSubmission(c *gin.Context, relayInfo *relaycommon.RelayInfo) (*taskSubmissionOutcome, *taskdto.TaskError) {
	return executeTaskSubmissionWith(c, relayInfo, relay.RelayTaskSubmit)
}

type taskSubmitAttempt func(*gin.Context, *relaycommon.RelayInfo) (*relay.TaskSubmitResult, *taskdto.TaskError)

func executeTaskSubmissionWith(
	c *gin.Context,
	relayInfo *relaycommon.RelayInfo,
	submit taskSubmitAttempt,
) (*taskSubmissionOutcome, *taskdto.TaskError) {
	reserved, _ := c.Get("generation_queue_reserved_task")
	queuedTask, _ := reserved.(*model.Task)
	strictRawImage := queuedTask == nil && relayInfo.OriginModelName == influencerImageModel
	var routing *model.GenerationRoutingPolicy
	if queuedTask != nil && queuedTask.PrivateData.GenerationQueue != nil {
		routing = queuedTask.PrivateData.GenerationQueue.Routing
	}
	if routing != nil {
		c.Set("generation_queue_native_routing", true)
		// Middleware may have advanced its speculative group cursor. The
		// durable submission loop owns the one global attempt budget.
		delete(c.Keys, string(constant.ContextKeyAutoGroupIndex))
		delete(c.Keys, string(constant.ContextKeyAutoGroupRetryIndex))
	}
	if queuedTask != nil {
		relayInfo.PublicTaskID = queuedTask.TaskID
		relayInfo.IsPlayground = true
	}
	policy := service.RequestPolicy(c)
	diagnostics := newTaskPluginSubmitDiagnostics(c)
	diagnostics.start(relayInfo)
	var result *relay.TaskSubmitResult
	var taskErr *taskdto.TaskError
	durable := false
	mayHaveSent := false
	attemptSent := false
	definitelyRejected := false
	var firstQuote *generationSubmissionQuote
	if routing != nil {
		c.Set("generation_queue_quote_gate", func() error {
			snapshot, err := taskFundingSnapshot(c, relayInfo, "")
			if err != nil {
				return err
			}
			if firstQuote == nil {
				firstQuote = &generationSubmissionQuote{Quota: relayInfo.PriceData.Quota, Billing: snapshot.PrivateData.BillingContext}
				return nil
			}
			if !firstQuote.Matches(relayInfo.PriceData.Quota, snapshot.PrivateData.BillingContext) {
				return errors.New("generation backup price differs from the original quote")
			}
			return nil
		})
		defer delete(c.Keys, "generation_queue_quote_gate")
	}
	// queueReleaseClaimed records that this worker persisted the release marker
	// before releasing its reservation, so cleanup does not claim it twice.
	queueReleaseClaimed := false
	stage := "start"
	if queuedTask != nil {
		relayInfo.BeforeTaskSend = func(platform constant.TaskPlatform) error {
			if err := c.Request.Context().Err(); err != nil {
				return err
			}
			snapshot, err := taskFundingSnapshot(c, relayInfo, platform)
			if err != nil {
				return err
			}
			if routing != nil {
				if firstQuote == nil {
					firstQuote = &generationSubmissionQuote{Quota: relayInfo.PriceData.Quota, Billing: snapshot.PrivateData.BillingContext}
				} else if !firstQuote.Matches(relayInfo.PriceData.Quota, snapshot.PrivateData.BillingContext) {
					return errors.New("generation backup price differs from the original quote")
				}
			}
			scanner, _ := c.Get("generation_queue_scanner")
			lease, _ := scanner.(*model.SystemTask)
			if err := model.PersistGenerationSendSnapshot(c.Request.Context(), queuedTask, snapshot, lease, c.GetString("generation_queue_runner"), true); err != nil {
				return err
			}
			mayHaveSent = true
			attemptSent = true
			definitelyRejected = false
			c.Set("generation_queue_attempt_started", true)
			return nil
		}
		defer func() { relayInfo.BeforeTaskSend = nil }()
	}
	defer func() {
		if durable {
			return
		}
		if queuedTask == nil {
			if relayInfo.Billing != nil {
				diagnostics.refund(stage)
				relayInfo.Billing.Refund(c)
			}
			return
		}
		// Queue cleanup must survive a cancelled HTTP context, but remains bounded.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), 5*time.Second)
		defer cancel()
		reason := "提交结果待核对，不会自动重发"
		if !mayHaveSent || definitelyRejected || routing == nil && relayInfo.TaskSubmitHTTPStatus == http.StatusBadRequest {
			// fork: a sent row may already have been expired and refunded by
			// a newer scanner; the persisted marker decides who releases.
			releaseAllowed := true
			if mayHaveSent && !queueReleaseClaimed {
				if err := model.MarkGenerationReleaseStarted(ctx, queuedTask); err != nil {
					common.SysError("generation release was not claimed: " + err.Error())
					releaseAllowed = false
				} else {
					queueReleaseClaimed = true
				}
			}
			if releaseAllowed {
				if err := service.SettleBilling(c, relayInfo, 0); err == nil {
					snapshot, snapshotErr := taskFundingSnapshot(c, relayInfo, "")
					if snapshotErr == nil {
						if err := model.RejectGenerationDispatch(ctx, queuedTask, "生成未被接受，预留已释放", snapshot); err == nil {
							c.Set("generation_queue_safe_failure", true)
							return
						}
					}
				} else {
					common.SysError("generation funding release failed: " + err.Error())
				}
			}
			reason = "提交或资金释放待核对，不会自动重发"
			// Prevent the worker tail from declaring a refund/persistence failure safe.
			c.Set("generation_queue_attempt_started", true)
		}
		code := "unclassified"
		if taskErr != nil {
			switch taskErr.Code {
			case "generation_snapshot_failed", "task_snapshot_failed", "task_insert_failed", "task_billing_settlement_failed", "request_cancelled", "plugin_submit_response_invalid", "plugin_submit_response_failed", "task_submit_failed", "fail_to_fetch_task", "read_response_body_failed":
				code = taskErr.Code
			default:
				code = "submission_failed"
			}
		}
		reason += fmt.Sprintf("（阶段：%s；代码：%s）", stage, code)
		logger.LogError(c, fmt.Sprintf("generation submission requires reconciliation task=%s stage=%s code=%s", queuedTask.TaskID, stage, code))
		if err := model.MarkGenerationSubmissionUnknown(ctx, queuedTask, reason); err != nil {
			common.SysError("generation reconciliation state could not be saved")
		}
	}()
	stage = "before_attempt"
	if requestErr := c.Request.Context().Err(); requestErr != nil {
		diagnostics.cancelled("before_attempt", 0)
		taskErr = service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
		return nil, taskErr
	}

	retryParam := &service.RetryParam{
		Ctx:                 c,
		TokenGroup:          relayInfo.TokenGroup,
		ModelName:           relayInfo.OriginModelName,
		RequestPath:         c.Request.URL.Path,
		Retry:               common.GetPointer(0),
		PreserveRetryBudget: routing != nil,
	}
	rawImageAttempts := 0

	for ; retryParam.GetRetry() <= common.RetryTimes; retryParam.IncreaseRetry() {
		if strictRawImage && rawImageAttempts >= 2 {
			break
		}
		if routing != nil && (retryParam.GetRetry() >= routing.MaxAttempts || retryParam.GetRetry() >= 2) {
			break
		}
		attemptSent = false
		stage = "select_channel"
		if requestErr := c.Request.Context().Err(); requestErr != nil {
			diagnostics.cancelled("before_attempt", retryParam.GetRetry()+1)
			taskErr = service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
			break
		}
		var channel *model.Channel

		if lockedCh, ok := relayInfo.LockedChannel.(*model.Channel); ok && lockedCh != nil {
			channel = lockedCh
			policy.BeginAttempt(channel, relayInfo.UsingGroup)
			if retryParam.GetRetry() > 0 {
				if setupErr := middleware.SetupContextForSelectedChannel(c, channel, relayInfo.OriginModelName); setupErr != nil {
					taskErr = service.TaskErrorWrapperLocal(setupErr.Err, "setup_locked_channel_failed", http.StatusInternalServerError)
					break
				}
			}
		} else {
			var channelErr *types.NewAPIError
			channel, channelErr = getChannel(c, relayInfo, retryParam)
			if channelErr != nil {
				logger.LogError(c, channelErr.Error())
				taskErr = service.TaskErrorWrapperLocal(channelErr.Err, "get_channel_failed", channelErr.StatusCode)
				break
			}
		}
		diagnostics.attempt(retryParam.GetRetry()+1, channel, relayInfo.LockedChannel != nil)

		service.AppendUsedChannel(c, channel.Id)
		bodyStorage, bodyErr := common.GetBodyStorage(c)
		if bodyErr != nil {
			stage = "read_body"
			if common.IsRequestBodyTooLargeError(bodyErr) || errors.Is(bodyErr, common.ErrRequestBodyTooLarge) {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusRequestEntityTooLarge)
			} else {
				taskErr = service.TaskErrorWrapperLocal(bodyErr, "read_request_body_failed", http.StatusBadRequest)
			}
			break
		}
		c.Request.Body = io.NopCloser(bodyStorage)

		stage = "submit"
		if queuedTask != nil {
			if queuedTask.Status == model.TaskStatusClaimed {
				scanner, _ := c.Get("generation_queue_scanner")
				begun, err := model.BeginGenerationDispatch(c.Request.Context(), queuedTask, scanner.(*model.SystemTask), c.GetString("generation_queue_runner"))
				if err != nil || !begun {
					return nil, service.TaskErrorWrapperLocal(errors.New("generation dispatch ownership lost"), "generation_claim_lost", http.StatusConflict)
				}
				queuedTask.Status = model.TaskStatusDispatching
			}
		}
		relayInfo.TaskSubmitHTTPStatus = 0
		if strictRawImage {
			rawImageAttempts++
		}
		result, taskErr = submit(c, relayInfo)
		if routing != nil && attemptSent && taskErr == nil && result != nil {
			if err := persistGenerationAcceptedReceipt(c, queuedTask, relayInfo, result, firstQuote); err != nil {
				taskErr = service.TaskErrorWrapperLocal(err, "generation_snapshot_failed", http.StatusInternalServerError)
				break
			}
		}
		if requestErr := c.Request.Context().Err(); requestErr != nil {
			diagnostics.cancelled("after_submit", retryParam.GetRetry()+1)
			taskErr = service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
			break
		}
		if taskErr == nil {
			diagnostics.attemptSucceeded(retryParam.GetRetry()+1, result)
			break
		}

		taskAPIError := taskSubmissionAPIError(taskErr)
		relayInfo.LastError = taskAPIError
		decision := decideTaskRetry(c, taskErr, common.RetryTimes-retryParam.GetRetry())
		// The raw entry remains closed until durable funding reconciliation is
		// implemented. Its submit primitive must still never repeat an unknown
		// result if an internal caller invokes it directly.
		if strictRawImage && (!taskErr.NoTaskCreated || taskErr.NoRetry || rawImageAttempts >= 2) {
			decision = service.PolicyDecision{Action: "stop", Reason: "submission_not_proven_unaccepted", Source: "native_generation_safety"}
		}
		service.RecordPolicyFailure(c, channel.Id, taskAPIError, decision)
		if !taskErr.LocalError {
			processChannelError(c,
				*types.NewChannelError(channel.Id, channel.Type, channel.Name, channel.ChannelInfo.IsMultiKey,
					common.GetContextKeyString(c, constant.ContextKeyChannelKey), channel.GetAutoBan()),
				taskAPIError,
				relayInfo)
		}

		willRetry := decision.Action == "retry"
		if queuedTask != nil {
			// fork: only a host-proven rejection may reuse the same reservation.
			// An accepted, timed-out or unclassified POST retains the send fence.
			definitelyRejected = routing != nil && (!attemptSent || taskErr.NoTaskCreated) && !taskErr.NoRetry
			willRetry = willRetry && routing != nil && definitelyRejected && retryParam.GetRetry()+1 < routing.MaxAttempts && retryParam.GetRetry()+1 < 2
			if routing != nil && attemptSent {
				outcome := model.GenerationAttemptUnknown
				if definitelyRejected {
					outcome = model.GenerationAttemptRejected
				}
				if err := persistGenerationAttemptOutcome(c, queuedTask, channel.Id, outcome); err != nil {
					taskErr = service.TaskErrorWrapperLocal(err, "generation_snapshot_failed", http.StatusInternalServerError)
					willRetry = false
				}
			}
			if willRetry {
				service.ExcludeAttemptedChannel(c, channel.Id)
			}
		} else if strictRawImage && willRetry {
			service.ExcludeAttemptedChannel(c, channel.Id)
		}
		diagnostics.attemptFailed(retryParam.GetRetry()+1, channel, taskErr, willRetry)
		if !willRetry {
			break
		}
	}

	useChannel := c.GetStringSlice("use_channel")
	if len(useChannel) > 1 {
		retryLogStr := common.LogText("retry: %s", strings.Trim(strings.Join(strings.Fields(fmt.Sprint(useChannel)), "->"), "[]"))
		logger.LogInfo(c, retryLogStr)
	}

	if taskErr != nil {
		diagnostics.failed(stage, "task_error", taskErr, false)
		return nil, taskErr
	}
	if result == nil {
		taskErr = service.TaskErrorWrapperLocal(errors.New("task submission returned no result"), "task_submit_failed", http.StatusInternalServerError)
		diagnostics.failed("submit", "missing_result", taskErr, false)
		return nil, taskErr
	}
	if routing != nil && firstQuote != nil && !(result.Immediate != nil && result.Immediate.Status == model.TaskStatusFailure) {
		// Fixed/request pricing has no measured-usage reason to change its total
		// after acceptance. Keep the accepted task and funding for reconciliation
		// rather than reserving an unexplained provider adjustment.
		if firstQuote.Billing.TieredSnapshot == nil || !firstQuote.Billing.TieredSnapshot.TaskUsageBilling {
			snapshot, err := taskFundingSnapshot(c, relayInfo, result.Platform)
			if err != nil || !firstQuote.Matches(result.Quota, snapshot.PrivateData.BillingContext) {
				taskErr = service.TaskErrorWrapperLocal(errors.New("accepted generation pricing requires reconciliation"), "generation_quote_changed", http.StatusConflict)
				return nil, taskErr
			}
		}
	}
	if requestErr := c.Request.Context().Err(); requestErr != nil {
		diagnostics.cancelled("before_reserve", retryParam.GetRetry()+1)
		taskErr = service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
		return nil, taskErr
	}

	// Reserve any submit-time upward billing adjustment before persistence.
	// This keeps insertion failures fully refundable while ensuring settlement
	// after the barrier normally has a zero positive delta.
	if relayInfo.Billing != nil {
		stage = "reserve"
		diagnostics.reserve("reserve_start", result.Quota)
		if reserveErr := relayInfo.Billing.Reserve(result.Quota); reserveErr != nil {
			common.SysError(common.LogText("reserve adjusted task billing error: %s", reserveErr.Error()))
			taskErr = service.TaskErrorWrapperLocal(errors.New("insufficient quota for adjusted task cost"), string(types.ErrorCodeInsufficientUserQuota), http.StatusForbidden)
			diagnostics.failed("reserve", "insufficient_quota", taskErr, false)
			return nil, taskErr
		}
		diagnostics.reserve("reserve_complete", result.Quota)
	}
	if queuedTask != nil && mayHaveSent {
		stage = "snapshot"
		snapshot, err := taskFundingSnapshot(c, relayInfo, result.Platform)
		if err != nil {
			taskErr = service.TaskErrorWrapperLocal(err, "generation_snapshot_failed", http.StatusInternalServerError)
			return nil, taskErr
		}
		scanner, _ := c.Get("generation_queue_scanner")
		lease, _ := scanner.(*model.SystemTask)
		if err := model.PersistGenerationSendSnapshot(c.Request.Context(), queuedTask, snapshot, lease, c.GetString("generation_queue_runner")); err != nil {
			taskErr = service.TaskErrorWrapperLocal(err, "generation_snapshot_failed", http.StatusInternalServerError)
			return nil, taskErr
		}
	}
	if requestErr := c.Request.Context().Err(); requestErr != nil {
		diagnostics.cancelled("before_insert", retryParam.GetRetry()+1)
		taskErr = service.TaskErrorWrapperLocal(requestErr, "request_cancelled", http.StatusRequestTimeout)
		return nil, taskErr
	}

	// fork: a definite immediate queue failure must release native funding before
	// publishing a zero-cost terminal row or freeing its concurrency slot.
	queueFailureSettled := queuedTask != nil && result.Immediate != nil && result.Immediate.Status == model.TaskStatusFailure
	if queueFailureSettled {
		stage = "settle"
		if err := model.MarkGenerationReleaseStarted(c.Request.Context(), queuedTask); err != nil {
			taskErr = service.TaskErrorWrapperLocal(err, "generation_snapshot_failed", http.StatusInternalServerError)
			return nil, taskErr
		}
		queueReleaseClaimed = true
		if err := service.SettleBilling(c, relayInfo, 0); err != nil {
			common.SysError("generation immediate failure funding release failed: " + err.Error())
			taskErr = service.TaskErrorWrapperLocal(errors.New("failed to release generation funding"), "task_billing_settlement_failed", http.StatusInternalServerError)
			return nil, taskErr
		}
	}
	stage = "insert"
	task, snapshotErr := taskFundingSnapshot(c, relayInfo, result.Platform)
	if snapshotErr != nil {
		taskErr = service.TaskErrorWrapperLocal(snapshotErr, "task_snapshot_failed", http.StatusInternalServerError)
		return nil, taskErr
	}
	task.PrivateData.UpstreamTaskID = result.UpstreamTaskID
	attachInfluencerImageInputs(c, task)
	if queuedTask == nil {
		task.PrivateData.TokenId = relayInfo.TokenId
	}
	task.Quota = result.Quota
	task.Data = result.TaskData
	if len(result.PluginState) > 0 {
		task.PrivateData.PluginState = result.PluginState
	}
	task.Action = relayInfo.Action
	if sora.IsAsyncImageModel(relayInfo.OriginModelName) {
		if origURL, isSync := sora.LooksLikeSyncImageResponse(result.TaskData); isSync {
			task.Status = model.TaskStatusSuccess
			task.Progress = "100%"
			task.FinishTime = time.Now().Unix()
			if origURL != "" {
				task.PrivateData.ResultURL = origURL
			}
		}
	}
	if immediate := result.Immediate; immediate != nil {
		task.Status = model.TaskStatus(immediate.Status)
		task.Progress = immediate.Progress
		if immediate.Status == model.TaskStatusSuccess || immediate.Status == model.TaskStatusFailure {
			task.FinishTime = time.Now().Unix()
		}
		if immediate.Status == model.TaskStatusFailure {
			task.FailReason = immediate.Reason
		}
		if immediate.Url != "" {
			task.PrivateData.ResultURL = immediate.Url
		} else if immediate.Status == model.TaskStatusSuccess {
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		}
	}
	// A native submit route may declare retainResult: false. It applies only
	// to immediate terminal results: the client receives the complete response
	// once, the upstream snapshot is never persisted, and the task is not
	// retrievable afterwards. An asynchronous result on such a route keeps its
	// snapshot because polling and retrieval need it. The OpenAI Images
	// protocol delivers its images inline in the same HTTP response, so its
	// immediate results follow the same rule; an asynchronous image task is
	// expected there and is polled inside the request.
	var insertOmits []string
	immediateTerminal := result.Immediate != nil && (result.Immediate.Status == model.TaskStatusSuccess || result.Immediate.Status == model.TaskStatusFailure)
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedRoute); exists {
		pinned, ok := pinnedValue.(pluginruntime.PinnedRoute)
		if ok && pinned.Route.RetainResult != nil && !*pinned.Route.RetainResult {
			if immediateTerminal {
				task.PrivateData.ResultDiscarded = true
				insertOmits = append(insertOmits, "data")
			} else {
				logger.LogWarn(c, common.LogText("task plugin route %s %s declares retainResult: false but returned an asynchronous result; retaining task %s", pinned.Route.Method, pinned.Route.Path, task.TaskID))
			}
		}
	}
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedEndpoint); exists && immediateTerminal && queuedTask == nil {
		if pinned, ok := pinnedValue.(pluginruntime.PinnedEndpoint); ok && pinned.Protocol == pluginruntime.ProtocolOpenAIImage {
			task.PrivateData.ResultDiscarded = true
			insertOmits = append(insertOmits, "data")
		}
	}
	diagnostics.insertStart(task)
	var insertErr error
	if queuedTask != nil {
		insertErr = model.PersistGenerationSubmission(c.Request.Context(), task, queuedTask)
	} else {
		insertErr = task.InsertWithContext(c.Request.Context(), insertOmits...)
	}
	if insertErr != nil {
		common.SysError(common.LogText("insert task error: %s", insertErr.Error()))
		taskErr = service.TaskErrorWrapperLocal(errors.New("failed to persist task"), "task_insert_failed", http.StatusInternalServerError)
		diagnostics.failed("insert", "database_error", taskErr, false)
		return nil, taskErr
	}
	durable = true
	stage = "settle"
	diagnostics.durable(task)
	diagnostics.settleStart(task, result.Quota)

	if !queueFailureSettled {
		if settleErr := service.SettleBilling(c, relayInfo, result.Quota); settleErr != nil {
			common.SysError(common.LogText("settle task billing error: %s", settleErr.Error()))
			taskErr = service.TaskErrorWrapperLocal(errors.New("failed to settle task billing"), "task_billing_settlement_failed", http.StatusInternalServerError)
			diagnostics.failed("settle", "billing_error", taskErr, true)
			return nil, taskErr
		}
	}
	if task.Status != model.TaskStatusFailure {
		service.MarkRequestPolicySuccess(c, nil)
	} else {
		policy.AddEvent(service.PolicyEvent{Decision: service.PolicyDecision{Action: "stop", Reason: "task_failed", Source: "upstream"}})
	}
	service.LogTaskConsumption(c, relayInfo, task)
	diagnostics.complete(task, result.Quota)

	return &taskSubmissionOutcome{Result: result, Task: task, RelayInfo: relayInfo}, nil
}

// taskFundingSnapshot captures native actual reservation and immutable pricing, not consume statistics.
func taskFundingSnapshot(c *gin.Context, info *relaycommon.RelayInfo, platform constant.TaskPlatform) (*model.Task, error) {
	task := model.InitTask(platform, info)
	task.PrivateData.Execution = service.TaskExecutionSnapshotFromContext(c)
	if task.PrivateData.Execution != nil && task.PrivateData.Execution.RequestID == "" {
		task.PrivateData.Execution.RequestID = info.RequestId
	}
	task.PrivateData.BillingSource = info.BillingSource
	task.PrivateData.SubscriptionId = info.SubscriptionId
	task.PrivateData.SubscriptionWalletConsumed = info.SubscriptionWalletConsumed
	task.PrivateData.SubscriptionAllowanceEpoch = info.SubscriptionAllowanceEpoch
	task.PrivateData.SubscriptionAllowanceAllocated = info.SubscriptionAllowanceAllocated
	if !info.IsPlayground {
		task.PrivateData.TokenId = info.TokenId
	}
	task.PrivateData.NodeName = common.NodeName
	billing := &model.TaskBillingContext{
		ModelPrice: info.PriceData.ModelPrice, GroupRatio: info.PriceData.GroupRatioInfo.GroupRatio,
		ModelRatio: info.PriceData.ModelRatio, OtherRatios: info.PriceData.OtherRatios(),
		OriginModelName: info.OriginModelName,
		PerCallBilling:  common.StringsContains(constant.TaskPricePatches, info.OriginModelName) || info.PriceData.UsePrice,
		TieredSnapshot:  info.TieredBillingSnapshot,
	}
	encoded, err := common.Marshal(billing)
	if err != nil {
		return nil, err
	}
	task.PrivateData.BillingContext = &model.TaskBillingContext{}
	if err := common.Unmarshal(encoded, task.PrivateData.BillingContext); err != nil {
		return nil, err
	}
	if info.Billing != nil {
		task.Quota = info.Billing.GetPreConsumedQuota()
	}
	if task.Quota < 0 || task.Quota > common.MaxQuota {
		return nil, errors.New("invalid native reservation")
	}
	return task, nil
}

func presentTaskSubmission(c *gin.Context, outcome *taskSubmissionOutcome) {
	if c.Writer.Written() {
		return
	}
	diagnostics := newTaskPluginSubmitDiagnostics(c)
	otherRatios := outcome.RelayInfo.PriceData.OtherRatios()
	if otherRatios == nil {
		otherRatios = map[string]float64{}
	}
	if ratiosJSON, err := common.Marshal(otherRatios); err == nil {
		c.Header("X-New-Api-Other-Ratios", string(ratiosJSON))
	}
	if outcome.Result != nil && outcome.Result.ClientResponse != nil {
		diagnostics.present(outcome.Task, "client_response")
		clientResp := outcome.Result.ClientResponse
		if outcome.Task != nil && outcome.Task.TaskID != "" && c.Request != nil && c.Request.URL != nil && strings.HasPrefix(c.Request.URL.Path, "/pg/") {
			clientResp = ensureTaskIDInClientResponse(clientResp, outcome.Task.TaskID)
		}
		switch resp := clientResp.(type) {
		case []byte:
			c.Data(http.StatusOK, "application/json; charset=utf-8", resp)
		case string:
			c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(resp))
		default:
			c.JSON(http.StatusOK, resp)
		}
		return
	}
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedRoute); exists {
		if pinned, ok := pinnedValue.(pluginruntime.PinnedRoute); ok && pinned.Plugin != nil && pinned.Route.Render != "" {
			view, err := service.BuildTaskPluginView(outcome.Task)
			requestValue, _ := c.Get(pluginruntime.ContextKeyRouteRequest)
			requestContext, _ := requestValue.(pluginruntime.RouteRequestContext)
			if err == nil {
				viewValue, valueErr := taskPluginProtocolJSONValue(view)
				if valueErr == nil {
					if body, callErr := pinned.Plugin.Engine.CallPath(c.Request.Context(), "native", []string{pinned.Route.Render}, requestContext.JSValue(), viewValue); callErr == nil {
						diagnostics.present(outcome.Task, "native_presenter")
						c.JSON(http.StatusOK, body)
						return
					} else {
						logger.LogError(c, common.LogText("task plugin native submit presenter failed: %s", callErr.Error()))
					}
				} else {
					logger.LogError(c, common.LogText("encode task plugin native submit view failed: %s", valueErr.Error()))
				}
			} else {
				logger.LogError(c, common.LogText("build task plugin native submit view failed: %s", err.Error()))
			}
		}
	}
	if pinnedValue, exists := c.Get(pluginruntime.ContextKeyPinnedEndpoint); exists {
		if pinned, ok := pinnedValue.(pluginruntime.PinnedEndpoint); ok && pinned.Protocol == "openai_video" && pinned.Operation.Name == "create" {
			diagnostics.present(outcome.Task, "openai_video_create")
			c.JSON(http.StatusOK, outcome.Task.ToOpenAIVideo())
			return
		}
	}
	createdAt := outcome.Task.CreatedAt
	if createdAt == 0 {
		createdAt = outcome.Task.SubmitTime
	}
	diagnostics.present(outcome.Task, "host_fallback")
	c.JSON(http.StatusOK, map[string]any{
		"id":         outcome.Task.TaskID,
		"task_id":    outcome.Task.TaskID,
		"status":     "queued",
		"model":      outcome.RelayInfo.OriginModelName,
		"created_at": createdAt,
	})
}

func ensureTaskIDInClientResponse(clientResp any, taskID string) any {
	if strings.TrimSpace(taskID) == "" {
		return clientResp
	}
	switch v := clientResp.(type) {
	case []byte:
		var m map[string]any
		if err := common.Unmarshal(v, &m); err == nil && m != nil {
			changed := false
			if _, ok := m["task_id"]; !ok {
				m["task_id"] = taskID
				changed = true
			}
			if _, ok := m["id"]; !ok {
				m["id"] = taskID
				changed = true
			}
			if changed {
				if b, err := common.Marshal(m); err == nil {
					return b
				}
			}
		}
	case string:
		var m map[string]any
		if err := common.Unmarshal([]byte(v), &m); err == nil && m != nil {
			changed := false
			if _, ok := m["task_id"]; !ok {
				m["task_id"] = taskID
				changed = true
			}
			if _, ok := m["id"]; !ok {
				m["id"] = taskID
				changed = true
			}
			if changed {
				if b, err := common.Marshal(m); err == nil {
					return string(b)
				}
			}
		}
	case map[string]any:
		if _, ok := v["task_id"]; !ok {
			v["task_id"] = taskID
		}
		if _, ok := v["id"]; !ok {
			v["id"] = taskID
		}
		return v
	}
	return clientResp
}

func respondTaskSubmissionError(c *gin.Context, taskErr *taskdto.TaskError) {
	service.RecordRequestPolicyTermination(c, taskSubmissionAPIError(taskErr))
	newTaskPluginSubmitDiagnostics(c).presentError(taskErr)
	if middleware.RespondTaskPluginError(c, taskErr) {
		return
	}
	respondTaskError(c, taskErr)
}

// respondTaskError 统一输出 Task 错误响应（含 429 限流提示改写）
func respondTaskError(c *gin.Context, taskErr *taskdto.TaskError) {
	if taskErr.StatusCode == http.StatusTooManyRequests {
		taskErr.Message = i18n.T(c, i18n.MsgRelayGroupUpstreamSaturated)
	}
	c.JSON(taskErr.StatusCode, taskErr)
}

// taskSubmissionAPIError adapts a task error for the shared relay error paths.
// TaskError.Error is nil for many local rejections, so fall back to the message.
func taskSubmissionAPIError(taskErr *taskdto.TaskError) *types.NewAPIError {
	err := taskErr.Error
	if err == nil {
		err = errors.New(taskErr.Message)
	}
	return types.NewOpenAIError(err, types.ErrorCodeBadResponseStatusCode, taskErr.StatusCode)
}

// decideTaskRetry is the single retry decision for task submissions. The
// reason is recorded in the request policy decision events of the log details.
func decideTaskRetry(c *gin.Context, taskErr *taskdto.TaskError, retryTimes int) service.PolicyDecision {
	stop := service.PolicyDecision{Action: "stop", Source: "system"}
	retry := service.PolicyDecision{Action: "retry", Reason: "retry_status_matched", Source: "system"}
	switch {
	case taskErr == nil:
		stop.Reason = "request_completed"
	case taskErr.NoRetry:
		stop.Reason = "task_accepted"
	case service.ShouldSkipRetryAfterChannelAffinityFailure(c):
		stop.Reason, stop.Source = "strict_session", "session_rule"
		if source := service.RequestPolicy(c).SessionModeSource; source != "" {
			stop.Source = source
		}
	case retryTimes <= 0:
		stop.Reason, stop.Source = "attempt_budget_exhausted", "global"
	case service.GetChannelConstraints(c).SuppressesRetry():
		stop.Reason, stop.Source = "pinned_channel", "channel_constraint"
	case taskErr.StatusCode == http.StatusTooManyRequests, taskErr.StatusCode == 307:
		return retry
	case taskErr.StatusCode/100 == 5:
		// 超时不重试
		if operation_setting.IsAlwaysSkipRetryStatusCode(taskErr.StatusCode) {
			stop.Reason = "system_retry_exclusion"
			break
		}
		return retry
	case taskErr.StatusCode == http.StatusBadRequest, taskErr.StatusCode == 408:
		// azure处理超时不重试
		stop.Reason = "status_not_retryable"
	case taskErr.LocalError:
		stop.Reason = "local_rejection"
	case taskErr.StatusCode/100 == 2:
		stop.Reason = "system_retry_exclusion"
	default:
		return retry
	}
	return stop
}
