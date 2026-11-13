package advancedcustom

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/claude"
	"github.com/QuantumNous/new-api/relay/channel/gemini"
	"github.com/QuantumNous/new-api/relay/channel/openai"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/relayconvert"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/samber/lo"
	"github.com/tidwall/gjson"
)

const ChannelName = "advanced_custom"

const advancedCustomModelPlaceholder = "{model}"

type Adaptor struct {
	openaiAdaptor openai.Adaptor
	claudeAdaptor claude.Adaptor
	geminiAdaptor gemini.Adaptor

	resolved   bool
	converted  bool
	route      dto.AdvancedCustomRoute
	converter  string
	extraQuery url.Values
}

func (a *Adaptor) Init(info *relaycommon.RelayInfo) {
	a.openaiAdaptor.Init(info)
	a.claudeAdaptor.Init(info)
	a.geminiAdaptor.Init(info)
}

func (a *Adaptor) ConvertOpenAIRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter == relayconvert.ConverterNone {
		return a.convertOpenAICompatibleRequest(c, info, request)
	}

	switch converter {
	case relayconvert.ConverterOpenAIChatToClaudeMessages,
		relayconvert.ConverterOpenAIChatToOpenAIResponses,
		relayconvert.ConverterOpenAIChatToGeminiContent:
		result, err := service.ConvertRequestByID(c, info, converter, request)
		if err != nil {
			return nil, err
		}
		return result.Value, nil
	default:
		return nil, fmt.Errorf("converter %q does not support OpenAI chat completions requests", converter)
	}
}

func (a *Adaptor) ConvertClaudeRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.ClaudeRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}

	switch converter {
	case relayconvert.ConverterNone:
		return a.claudeAdaptor.ConvertClaudeRequest(c, info, request)
	case relayconvert.ConverterClaudeMessagesToOpenAIChat:
		return a.convertCrossProtocolChatRequest(c, info, converter, request)
	case relayconvert.ConverterClaudeMessagesToOpenAIResponses:
		return convertCrossProtocolResponsesRequest(c, info, converter, request)
	default:
		return nil, fmt.Errorf("converter %q does not support Anthropic Messages requests", converter)
	}
}

func (a *Adaptor) ConvertGeminiRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeminiChatRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}

	switch converter {
	case relayconvert.ConverterNone:
		return a.geminiAdaptor.ConvertGeminiRequest(c, info, request)
	case relayconvert.ConverterGeminiContentToOpenAIChat:
		return a.convertCrossProtocolChatRequest(c, info, converter, request)
	case relayconvert.ConverterGeminiContentToOpenAIResponses:
		return convertCrossProtocolResponsesRequest(c, info, converter, request)
	default:
		return nil, fmt.Errorf("converter %q does not support Gemini generateContent requests", converter)
	}
}

func (a *Adaptor) ConvertOpenAIResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	switch converter {
	case relayconvert.ConverterNone:
		return a.convertOpenAICompatibleResponsesRequest(c, info, request)
	case relayconvert.ConverterOpenAIResponsesToOpenAIChat:
		return a.convertCrossProtocolChatRequest(c, info, converter, request)
	case relayconvert.ConverterOpenAIResponsesToGemini:
		result, err := service.ConvertRequestByID(c, info, converter, request)
		if err != nil {
			return nil, err
		}
		geminiRequest, ok := result.Value.(*dto.GeminiChatRequest)
		if !ok {
			return nil, fmt.Errorf("expected Gemini generateContent request, got %T", result.Value)
		}
		return geminiRequest, nil
	default:
		return nil, fmt.Errorf("converter %q does not support OpenAI Responses requests", converter)
	}
}

func (a *Adaptor) ConvertEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter != relayconvert.ConverterNone {
		return nil, fmt.Errorf("converter %q does not support embedding requests", converter)
	}
	return a.convertOpenAICompatibleEmbeddingRequest(c, info, request)
}

func (a *Adaptor) ConvertAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter != relayconvert.ConverterNone {
		return nil, fmt.Errorf("converter %q does not support audio requests", converter)
	}
	return a.convertOpenAICompatibleAudioRequest(c, info, request)
}

func (a *Adaptor) ConvertImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	converter, err := a.resolveForConversion(c, info)
	if err != nil {
		return nil, err
	}
	if converter != relayconvert.ConverterNone {
		return nil, fmt.Errorf("converter %q does not support image requests", converter)
	}
	return a.convertOpenAICompatibleImageRequest(c, info, request)
}

func (a *Adaptor) ConvertRerankRequest(c *gin.Context, relayMode int, request dto.RerankRequest) (any, error) {
	a.converted = true
	return a.openaiAdaptor.ConvertRerankRequest(c, relayMode, request)
}

func (a *Adaptor) GetRequestURL(info *relaycommon.RelayInfo) (string, error) {
	if err := a.resolve(nil, info); err != nil {
		return "", err
	}
	return a.routeURL(info)
}

func (a *Adaptor) BuildModelListRequest(info *relaycommon.RelayInfo) (string, http.Header, error) {
	return a.buildManagementRequest(info, dto.AdvancedCustomModelListPath)
}

func (a *Adaptor) BuildBalanceRequest(info *relaycommon.RelayInfo) (string, http.Header, error) {
	return a.buildManagementRequest(info, dto.AdvancedCustomBalancePath)
}

func (a *Adaptor) buildManagementRequest(info *relaycommon.RelayInfo, managementPath string) (string, http.Header, error) {
	if info == nil {
		return "", nil, errors.New("missing relay info")
	}
	config := info.ChannelOtherSettings.AdvancedCustom
	if config == nil {
		return "", nil, errors.New("advanced_custom is required")
	}
	if err := config.Validate(); err != nil {
		return "", nil, err
	}
	var route dto.AdvancedCustomRoute
	var ok bool
	switch managementPath {
	case dto.AdvancedCustomModelListPath:
		route, ok = config.ModelListRoute()
	case dto.AdvancedCustomBalancePath:
		route, ok = config.BalanceRoute()
	default:
		return "", nil, fmt.Errorf("unsupported advanced custom management path: %s", managementPath)
	}
	if !ok {
		return "", nil, fmt.Errorf("advanced custom channel does not configure a %s route", managementPath)
	}
	converter := strings.TrimSpace(route.Converter)
	if converter == "" {
		converter = relayconvert.ConverterNone
	}
	if converter != relayconvert.ConverterNone {
		return "", nil, fmt.Errorf("converter %q does not support %s requests", converter, managementPath)
	}

	requestURL, err := buildRouteURL(route, converter, info)
	if err != nil {
		return "", nil, err
	}

	header := http.Header{}
	auth := route.Auth
	if auth == nil {
		header.Set("Authorization", "Bearer "+info.ApiKey)
		return requestURL, header, nil
	}

	switch strings.TrimSpace(auth.Type) {
	case dto.AdvancedCustomAuthTypeNone, dto.AdvancedCustomAuthTypeQuery:
	case dto.AdvancedCustomAuthTypeHeader:
		header.Set(strings.TrimSpace(auth.Name), applyAuthTemplate(auth.Value, info.ApiKey))
	default:
		return "", nil, fmt.Errorf("invalid advanced custom auth type: %s", auth.Type)
	}
	return requestURL, header, nil
}

func (a *Adaptor) SetupRequestHeader(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) error {
	if err := a.resolve(c, info); err != nil {
		return err
	}

	channel.SetupApiRequestHeader(info, c, header)
	auth := a.route.Auth
	if auth == nil {
		header.Set("Authorization", "Bearer "+info.ApiKey)
	} else {
		switch strings.TrimSpace(auth.Type) {
		case dto.AdvancedCustomAuthTypeNone:
		case dto.AdvancedCustomAuthTypeHeader:
			header.Set(strings.TrimSpace(auth.Name), applyAuthTemplate(auth.Value, info.ApiKey))
		case dto.AdvancedCustomAuthTypeQuery:
		default:
			return fmt.Errorf("invalid advanced custom auth type: %s", auth.Type)
		}
	}

	if shouldApplyClaudeHeaders(a.converter, info) {
		applyClaudeHeaders(c, header, info)
	}

	return nil
}

func (a *Adaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (any, error) {
	if err := a.resolve(c, info); err != nil {
		return nil, err
	}
	if !a.converted && !a.route.SupportsPassThroughBody() {
		return nil, errors.New("advanced custom converter routes cannot be used with pass-through request body")
	}

	if err := a.prepareUpstreamQuery(c); err != nil {
		return nil, err
	}
	if method := a.route.EffectiveUpstreamMethod(); method != "" && info != nil {
		info.UpstreamRequestMethod = method
	}
	method := ""
	if info != nil {
		method = strings.ToUpper(strings.TrimSpace(info.UpstreamRequestMethod))
	}
	if method == http.MethodGet || method == http.MethodHead {
		// GET/HEAD: empty body; ContentLength/GetBody come from ReplayableBody when present.
		requestBody = http.NoBody
	}

	if info.RelayMode == relayconstant.RelayModeAudioTranscription ||
		info.RelayMode == relayconstant.RelayModeAudioTranslation ||
		(info.RelayMode == relayconstant.RelayModeImagesEdits && !isJSONRequest(c)) {
		return channel.DoFormRequest(a, c, info, requestBody)
	}
	if info.RelayMode == relayconstant.RelayModeRealtime {
		return channel.DoWssRequest(a, c, info, requestBody)
	}
	if accountMonitorProbeEnabled(c, info.OriginModelName, a.route.ResponseValidation) {
		return channel.DoApiRequestWithContext(a, c, info, requestBody)
	}
	return channel.DoApiRequest(a, c, info, requestBody)
}

func (a *Adaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (usage any, err *types.NewAPIError) {
	if err := a.resolve(c, info); err != nil {
		return nil, types.NewOpenAIError(err, types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}

	switch a.converter {
	case dto.AdvancedCustomConverterSGLangRerank:
		return a.doSGLangRerankResponse(c, resp, info)
	case relayconvert.ConverterNone:
		return a.doNativeResponse(c, resp, info)
	case relayconvert.ConverterClaudeMessagesToOpenAIChat,
		relayconvert.ConverterGeminiContentToOpenAIChat:
		return a.openaiAdaptor.DoResponse(c, resp, info)
	case relayconvert.ConverterOpenAIChatToClaudeMessages:
		return a.claudeAdaptor.DoResponse(c, resp, info)
	case relayconvert.ConverterOpenAIChatToGeminiContent:
		return a.geminiAdaptor.DoResponse(c, resp, info)
	case relayconvert.ConverterOpenAIResponsesToGemini:
		return a.geminiAdaptor.DoResponse(c, resp, info)
	case relayconvert.ConverterOpenAIChatToOpenAIResponses,
		relayconvert.ConverterClaudeMessagesToOpenAIResponses,
		relayconvert.ConverterGeminiContentToOpenAIResponses:
		// These handlers convert the Responses upstream into info.RelayFormat.
		if info.IsStream {
			return openai.OaiResponsesToChatStreamHandler(c, info, resp)
		}
		return openai.OaiResponsesToChatHandler(c, info, resp)
	case relayconvert.ConverterOpenAIResponsesToOpenAIChat:
		if info.IsStream {
			return openai.OaiChatToResponsesStreamHandler(c, info, resp)
		}
		return openai.OaiChatToResponsesHandler(c, info, resp)
	default:
		return nil, types.NewOpenAIError(fmt.Errorf("unsupported advanced custom converter: %s", a.converter), types.ErrorCodeInvalidRequest, http.StatusBadRequest, types.ErrOptionWithSkipRetry())
	}
}

func (a *Adaptor) GetModelList() []string {
	models := make([]string, 0, len(openai.ModelList)+len(claude.ModelList)+len(gemini.ModelList))
	models = append(models, openai.ModelList...)
	models = append(models, claude.ModelList...)
	models = append(models, gemini.ModelList...)
	return lo.Uniq(models)
}

func (a *Adaptor) GetChannelName() string {
	return ChannelName
}

func (a *Adaptor) doNativeResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (any, *types.NewAPIError) {
	if a.route.ShouldPassthroughResponse() {
		return a.doPassthroughResponse(c, resp, info.OriginModelName)
	}
	switch info.RelayFormat {
	case types.RelayFormatClaude:
		return a.claudeAdaptor.DoResponse(c, resp, info)
	case types.RelayFormatGemini:
		return a.geminiAdaptor.DoResponse(c, resp, info)
	default:
		return a.openaiAdaptor.DoResponse(c, resp, info)
	}
}

// doPassthroughResponse writes the upstream body as-is and returns a synthetic
// usage so ModelPrice (UsePrice) settle still has billable usage.
func (a *Adaptor) doPassthroughResponse(c *gin.Context, resp *http.Response, model string) (any, *types.NewAPIError) {
	if resp == nil {
		return nil, types.NewOpenAIError(errors.New("empty upstream response"), types.ErrorCodeBadResponseBody, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
	}
	diagnostic := accountMonitorShapeCollectorFor(c, model, a.route.ResponseValidation)
	boundedProbe := accountMonitorProbeEnabled(c, model, a.route.ResponseValidation)
	if diagnostic != nil {
		diagnostic.evidence = nil
	}
	if boundedProbe {
		defer func() {
			if err := resp.Body.Close(); err != nil {
				common.SysError(common.LogText("failed to close response body: %s", "account monitoring diagnostic response close failed"))
			}
		}()
	} else {
		defer service.CloseResponseBodyGracefully(resp)
	}
	reader := io.Reader(resp.Body)
	if boundedProbe {
		// Only a validated management probe enables these limits. Shape sampling
		// remains separate; ordinary relay retains its existing behavior.
		if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
			return nil, types.NewOpenAIError(errors.New("account monitoring diagnostic upstream HTTP failure"), types.ErrorCodeBadResponseBody, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
		}
		if c.Request != nil && c.Request.Context().Err() != nil {
			return nil, types.NewOpenAIError(errors.New("account monitoring diagnostic response read failed"), types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
		}
		reader = io.LimitReader(reader, accountMonitorShapeMaxBody+1)
	}
	body, err := io.ReadAll(reader)
	if err != nil {
		if boundedProbe {
			// Reader errors may contain provider text; keep the diagnostic path local.
			err = errors.New("account monitoring diagnostic response read failed")
		}
		return nil, types.NewOpenAIError(err, types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
	}
	if boundedProbe {
		if c.Request != nil && c.Request.Context().Err() != nil {
			return nil, types.NewOpenAIError(errors.New("account monitoring diagnostic response read failed"), types.ErrorCodeReadResponseBodyFailed, http.StatusInternalServerError, types.ErrOptionWithSkipRetry())
		}
		if len(body) > accountMonitorShapeMaxBody {
			if diagnostic != nil {
				diagnostic.setStatus(accountMonitorShapeTooLarge)
			}
			return nil, types.NewOpenAIError(errors.New("account monitoring diagnostic response exceeded limit"), types.ErrorCodeBadResponseBody, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
		}
	}
	if strings.TrimSpace(a.route.ResponseValidation) != "" {
		if _, err := ValidateAccountMonitorResponseForModel(model, a.route.ResponseValidation, body); err != nil {
			if diagnostic != nil {
				diagnostic.collect(body, err)
			}
			return nil, types.NewOpenAIError(err, types.ErrorCodeBadResponseBody, http.StatusBadGateway, types.ErrOptionWithSkipRetry())
		}
	}
	service.IOCopyBytesGracefully(c, resp, body)

	// Synthetic one-unit usage: fixed ModelPrice path charges per call, not tokens.
	usage := &dto.Usage{
		PromptTokens:     1,
		CompletionTokens: 0,
		TotalTokens:      1,
	}
	return usage, nil
}

// AccountMonitorResponseSummary contains only recognition facts, never upstream
// text, identifiers, credentials or media URLs.
type AccountMonitorResponseSummary struct {
	ProfileRecognized bool `json:"profile_recognized"`
	PostCount         int  `json:"post_count"`
}

// ValidateAccountMonitorResponse verifies the opt-in data contract before the
// caller writes a response or settles a per-call charge.
func ValidateAccountMonitorResponse(policy string, body []byte) (AccountMonitorResponseSummary, error) {
	return ValidateAccountMonitorResponseForModel("", policy, body)
}

// ValidateAccountMonitorResponseForModel adds only payload contracts proven for
// the trusted routed model. The model-less entry point retains its old scope.
func ValidateAccountMonitorResponseForModel(model, policy string, body []byte) (AccountMonitorResponseSummary, error) {
	summary := AccountMonitorResponseSummary{}
	policy = strings.TrimSpace(policy)
	if policy != dto.AdvancedCustomResponseValidationTikHubUser && policy != dto.AdvancedCustomResponseValidationTikHubPosts {
		return summary, errors.New("unsupported account monitoring response validation")
	}
	var payload any
	if err := common.Unmarshal(body, &payload); err != nil {
		return summary, errors.New("account monitoring upstream response is invalid")
	}
	root := gjson.ParseBytes(body)
	if !root.IsObject() && !root.IsArray() {
		return summary, errors.New("account monitoring upstream response is invalid")
	}
	// Only bounded provider envelopes are traversed. Post bodies and arbitrary
	// nested objects must not become response-level business statuses.
	layers := []gjson.Result{root}
	for depth := 0; depth < 4; depth++ {
		next := make([]gjson.Result, 0, len(layers)*2)
		for _, layer := range layers {
			if accountMonitorBusinessFailure(layer) {
				return summary, errors.New("account monitoring upstream reported a failure")
			}
			for _, key := range []string{"data", "result"} {
				if nested := layer.Get(key); nested.IsObject() {
					next = append(next, nested)
				}
			}
		}
		layers = next
	}
	payloadRoot := root
	if data := root.Get("data"); data.IsObject() || data.IsArray() {
		payloadRoot = data
	}
	if policy == dto.AdvancedCustomResponseValidationTikHubUser {
		tikTokRecognized := false
		if model == "tiktok-user" {
			var failed bool
			tikTokRecognized, failed = accountMonitorTikTokProfileRecognized(payloadRoot)
			if failed {
				return summary, errors.New("account monitoring upstream reported a failure")
			}
		}
		summary.ProfileRecognized = accountMonitorProfileRecognized(payloadRoot)
		if !summary.ProfileRecognized && model == "tiktok-user" && !accountMonitorBusinessFailure(accountMonitorProfileLayer(payloadRoot)) {
			summary.ProfileRecognized = tikTokRecognized
		}
		if !summary.ProfileRecognized {
			return AccountMonitorResponseSummary{}, errors.New("account monitoring upstream profile is unrecognized")
		}
		return summary, nil
	}
	valid, total, recognized := accountMonitorPostRows(payloadRoot)
	// The observed X container is a fallback only; keep all existing list and
	// row selection rules, including recognized empty or invalid old lists.
	if !recognized && model == "x-posts" && payloadRoot.IsObject() {
		if timeline := payloadRoot.Get("timeline"); timeline.IsArray() {
			if accountMonitorXTimelineEnvelopeFailure(payloadRoot) {
				return summary, errors.New("account monitoring upstream reported a failure")
			}
			valid, total, recognized = accountMonitorRows(timeline)
		}
	}
	if !recognized || (total > 0 && valid == 0) {
		return summary, errors.New("account monitoring upstream posts are unrecognized")
	}
	summary.PostCount = valid
	return summary, nil
}

// The consumer receives a hub wrapper at depth 0 and this payload at depth 1.
// Match its fixed envelope keys through depth 3: this payload and two child
// layers. Never inspect timeline rows or extend old recognized-list behavior.
func accountMonitorXTimelineEnvelopeFailure(payload gjson.Result) bool {
	layers := []gjson.Result{payload}
	for depth := 0; depth < 3 && len(layers) > 0; depth++ {
		next := make([]gjson.Result, 0, len(layers)*5)
		for _, layer := range layers {
			if accountMonitorBusinessFailure(layer) {
				return true
			}
			if depth == 2 {
				continue
			}
			for _, key := range [...]string{"data", "result", "user", "channel", "author"} {
				if nested := layer.Get(key); nested.IsObject() {
					next = append(next, nested)
				}
			}
		}
		layers = next
	}
	return false
}

// accountMonitorTikTokProfileRecognized consumes only the observed userInfo
// wrapper and five leaves. Business failures preempt even an old valid shape.
func accountMonitorTikTokProfileRecognized(root gjson.Result) (recognized, failed bool) {
	wrapper := root.Get("userInfo")
	user := wrapper.Get("user")
	stats := wrapper.Get("stats")
	for _, layer := range [...]gjson.Result{wrapper, user, stats} {
		if accountMonitorBusinessFailure(layer) {
			return false, true
		}
	}
	if !wrapper.IsObject() || !user.IsObject() || !stats.IsObject() {
		return false, false
	}
	for _, field := range [...]string{"nickname", "signature"} {
		value := user.Get(field)
		if value.Type == gjson.String && strings.TrimSpace(value.String()) != "" {
			return true, false
		}
	}
	if avatar := user.Get("avatarLarger"); avatar.Type == gjson.String {
		parsed, err := url.Parse(strings.TrimSpace(avatar.String()))
		if err == nil && parsed.Hostname() != "" && (strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")) {
			return true, false
		}
	}
	for _, field := range [...]string{"followerCount", "videoCount"} {
		value := stats.Get(field)
		if value.Type != gjson.Number {
			continue
		}
		count := value.Float()
		if !math.IsNaN(count) && !math.IsInf(count, 0) && count >= 0 {
			return true, false
		}
	}
	return false, false
}

func accountMonitorBusinessFailure(layer gjson.Result) bool {
	if !layer.IsObject() {
		return false
	}
	for _, key := range []string{"success", "ok"} {
		if layer.Get(key).Type == gjson.False {
			return true
		}
	}
	failure := layer.Get("error")
	if (failure.IsObject() && len(failure.Map()) > 0) || (failure.IsArray() && len(failure.Array()) > 0) ||
		(!failure.IsObject() && !failure.IsArray() && failure.Type != gjson.Null && failure.Type != gjson.False && !(failure.Type == gjson.Number && failure.Float() == 0) && strings.TrimSpace(failure.String()) != "") {
		return true
	}
	for _, key := range []string{"code", "status", "status_code"} {
		value := layer.Get(key)
		if key == "status" && value.Type == gjson.False {
			return true
		}
		if value.Type != gjson.Number && value.Type != gjson.String {
			continue
		}
		text := strings.TrimSpace(value.String())
		if code, err := strconv.ParseFloat(text, 64); err == nil {
			if code != 0 && (code < 200 || code >= 300) {
				return true
			}
			continue
		}
		switch strings.ToLower(text) {
		case "error", "failed", "failure", "denied", "forbidden", "unauthorized", "needs-omnimux", "quota-exceeded", "omnimux-unconfigured":
			return true
		}
	}
	return false
}

func accountMonitorProfileRecognized(root gjson.Result) bool {
	layer := accountMonitorProfileLayer(root)
	if !layer.IsObject() || accountMonitorBusinessFailure(layer) {
		return false
	}
	for _, key := range []string{"nickname", "nick_name", "name", "full_name", "display_name", "title", "signature", "bio", "description", "desc", "biography", "channel_description"} {
		value := layer.Get(key)
		if (value.Type == gjson.String || value.Type == gjson.Number) && strings.TrimSpace(value.String()) != "" {
			return true
		}
	}
	for _, key := range []string{"avatar_url", "avatar", "avatar_larger", "avatar_thumb", "profile_pic_url", "profile_picture", "thumbnail"} {
		value := strings.TrimSpace(layer.Get(key).String())
		if strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
			return true
		}
	}
	for _, key := range []string{"follower_count", "followers", "followerCount", "subscriber_count", "subscriberCount", "fans", "fans_count", "edge_followed_by", "video_count", "aweme_count", "post_count", "media_count", "videosCount", "posts_count"} {
		value := layer.Get(key)
		if value.Type != gjson.Number && value.Type != gjson.String {
			continue
		}
		number, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(value.String()), ",", ""), 64)
		if err == nil && !math.IsNaN(number) && !math.IsInf(number, 0) && number >= 0 {
			return true
		}
	}
	return false
}

func accountMonitorProfileLayer(root gjson.Result) gjson.Result {
	layer := root
	for _, key := range []string{"user", "channel", "author", "result", "data", "items"} {
		nested := root.Get(key)
		if nested.IsArray() {
			for _, row := range nested.Array() {
				if row.IsObject() {
					layer = row
					break
				}
			}
			if layer.Raw != root.Raw {
				break
			}
			continue
		}
		if !nested.IsObject() || len(nested.Map()) == 0 {
			continue
		}
		layer = nested
		for _, inner := range []string{"user", "channel", "author", "result"} {
			candidate := nested.Get(inner)
			if candidate.IsObject() && len(candidate.Map()) > 0 {
				layer = candidate
				break
			}
		}
		break
	}
	return layer
}

var accountMonitorListContainers = []string{"items", "aweme_list", "item_list", "contents", "posts", "videos", "video", "data", "result", "list", "edges", "media"}

func accountMonitorPostRows(root gjson.Result) (valid, total int, recognized bool) {
	if !root.IsObject() {
		return 0, 0, false
	}
	empty := false
	for _, key := range accountMonitorListContainers {
		count, sourceCount, found := accountMonitorRows(root.Get(key))
		if found && sourceCount > 0 {
			return count, sourceCount, true
		}
		empty = empty || found
	}
	for _, key := range accountMonitorListContainers {
		nested := root.Get(key)
		if !nested.IsObject() {
			continue
		}
		for _, inner := range accountMonitorListContainers {
			count, sourceCount, found := accountMonitorRows(nested.Get(inner))
			if found && sourceCount > 0 {
				return count, sourceCount, true
			}
			empty = empty || found
		}
	}
	return 0, 0, empty
}

func accountMonitorRows(value gjson.Result) (valid, total int, recognized bool) {
	if value.IsArray() {
		rows := value.Array()
		for _, row := range rows {
			if node := row.Get("node"); !accountMonitorPostIdentity(row) && node.IsObject() {
				row = node
			}
			if accountMonitorPostIdentity(row) {
				valid++
			}
		}
		return valid, len(rows), true
	}
	if !value.IsObject() || len(value.Map()) == 0 {
		return 0, 0, false
	}
	if edges := value.Get("edges"); edges.IsArray() {
		return accountMonitorRows(edges)
	}
	if accountMonitorPostIdentity(value) {
		return 1, 1, true
	}
	for _, key := range accountMonitorListContainers {
		if value.Get(key).Exists() {
			return 0, 0, false
		}
	}
	return 0, 1, true
}

func accountMonitorPostIdentity(row gjson.Result) bool {
	if !row.IsObject() {
		return false
	}
	for _, key := range []string{"id", "post_id", "aweme_id", "shortCode", "shortcode", "pk", "cid", "tweet_id", "rest_id", "videoId"} {
		value := row.Get(key)
		if (value.Type == gjson.String || value.Type == gjson.Number) && strings.TrimSpace(value.String()) != "" {
			return true
		}
	}
	return false
}

func (a *Adaptor) prepareUpstreamQuery(c *gin.Context) error {
	a.extraQuery = nil
	if len(a.route.QueryMapping) == 0 {
		return nil
	}
	if c == nil {
		return errors.New("request context is required for advanced custom query_mapping")
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return fmt.Errorf("read request body for query_mapping: %w", err)
	}
	raw, err := storage.Bytes()
	if err != nil {
		return fmt.Errorf("read request body for query_mapping: %w", err)
	}
	values := url.Values{}
	for queryKey, path := range a.route.QueryMapping {
		queryKey = strings.TrimSpace(queryKey)
		path = strings.TrimSpace(path)
		if queryKey == "" || path == "" {
			continue
		}
		result := gjson.GetBytes(raw, path)
		if !result.Exists() {
			continue
		}
		// Prefer non-empty scalar values; skip null/empty string.
		if result.Type == gjson.Null {
			continue
		}
		value := strings.TrimSpace(result.String())
		if value == "" {
			continue
		}
		values.Set(queryKey, value)
	}
	if len(values) > 0 {
		a.extraQuery = values
	}
	return nil
}

func (a *Adaptor) resolveForConversion(c *gin.Context, info *relaycommon.RelayInfo) (string, error) {
	if err := a.resolve(c, info); err != nil {
		return "", err
	}
	a.converted = true
	return a.converter, nil
}

func (a *Adaptor) resolve(c *gin.Context, info *relaycommon.RelayInfo) error {
	if a.resolved {
		return nil
	}
	if info == nil {
		return errors.New("missing relay info")
	}
	config := info.ChannelOtherSettings.AdvancedCustom
	if config == nil {
		return errors.New("advanced_custom is required")
	}
	if err := config.Validate(); err != nil {
		return err
	}

	incomingPath := incomingRequestPath(c, info)
	route, ok := config.MatchPathForModel(incomingPath, info.OriginModelName)
	if ok {
		route.Converter = strings.TrimSpace(route.Converter)
		if route.Converter == "" {
			route.Converter = relayconvert.ConverterNone
		}
		a.route = route
		a.converter = route.Converter
		a.resolved = true
		return nil
	}
	return fmt.Errorf("advanced custom channel does not support request path %s for model %s", incomingPath, info.OriginModelName)
}

func incomingRequestPath(c *gin.Context, info *relaycommon.RelayInfo) string {
	if c != nil && c.Request != nil && c.Request.URL != nil {
		return c.Request.URL.Path
	}
	if info == nil {
		return ""
	}
	return strings.Split(info.RequestURLPath, "?")[0]
}

func (a *Adaptor) routeURL(info *relaycommon.RelayInfo) (string, error) {
	return buildRouteURLWithExtraQuery(a.route, a.converter, info, a.extraQuery)
}

func buildRouteURL(route dto.AdvancedCustomRoute, converter string, info *relaycommon.RelayInfo) (string, error) {
	return buildRouteURLWithExtraQuery(route, converter, info, nil)
}

func buildRouteURLWithExtraQuery(route dto.AdvancedCustomRoute, converter string, info *relaycommon.RelayInfo, extraQuery url.Values) (string, error) {
	parsedURL, err := resolveUpstreamTargetURL(applyUpstreamPathTemplate(strings.TrimSpace(route.UpstreamPath), info), info)
	if err != nil {
		return "", err
	}
	if shouldUseGeminiStreamURL(converter, info) {
		useGeminiStreamGenerateContentURL(parsedURL)
	}
	if info != nil && info.RelayMode == relayconstant.RelayModeRealtime {
		switch parsedURL.Scheme {
		case "https":
			parsedURL.Scheme = "wss"
		case "http":
			parsedURL.Scheme = "ws"
		}
	}
	query := parsedURL.Query()
	for key, values := range extraQuery {
		for _, value := range values {
			if strings.TrimSpace(value) == "" {
				continue
			}
			query.Set(key, value)
		}
	}
	if route.Auth != nil && strings.TrimSpace(route.Auth.Type) == dto.AdvancedCustomAuthTypeQuery {
		query.Set(strings.TrimSpace(route.Auth.Name), applyAuthTemplate(route.Auth.Value, info.ApiKey))
	}
	parsedURL.RawQuery = query.Encode()
	return parsedURL.String(), nil
}

func resolveUpstreamTargetURL(upstreamPath string, info *relaycommon.RelayInfo) (*url.URL, error) {
	if strings.HasPrefix(upstreamPath, "/") {
		if strings.HasPrefix(upstreamPath, "//") {
			return nil, errors.New("advanced custom upstream path must be a full URL or a path starting with /")
		}
		if info == nil || strings.TrimSpace(info.ChannelBaseUrl) == "" {
			return nil, errors.New("channel base URL is required when advanced custom upstream path is relative")
		}
		return joinBaseURLAndUpstreamPath(info.ChannelBaseUrl, upstreamPath)
	}

	parsedURL, err := url.Parse(upstreamPath)
	if err != nil {
		return nil, err
	}
	if parsedURL.Scheme == "" || parsedURL.Host == "" {
		return nil, errors.New("advanced custom upstream path must be a full URL or a path starting with /")
	}
	if !strings.EqualFold(parsedURL.Scheme, "http") && !strings.EqualFold(parsedURL.Scheme, "https") {
		return nil, errors.New("advanced custom upstream path must use http or https")
	}
	return parsedURL, nil
}

func joinBaseURLAndUpstreamPath(baseURL string, upstreamPath string) (*url.URL, error) {
	parsedBaseURL, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, err
	}
	if parsedBaseURL.Scheme == "" || parsedBaseURL.Host == "" {
		return nil, errors.New("channel base URL must be a full URL when advanced custom upstream path is relative")
	}
	if !strings.EqualFold(parsedBaseURL.Scheme, "http") && !strings.EqualFold(parsedBaseURL.Scheme, "https") {
		return nil, errors.New("channel base URL must use http or https when advanced custom upstream path is relative")
	}

	parsedPath, err := url.Parse(upstreamPath)
	if err != nil {
		return nil, err
	}
	parsedBaseURL.Path = strings.TrimRight(parsedBaseURL.Path, "/") + "/" + strings.TrimLeft(parsedPath.Path, "/")
	parsedBaseURL.RawPath = ""
	parsedBaseURL.RawQuery = parsedPath.RawQuery
	parsedBaseURL.Fragment = parsedPath.Fragment
	return parsedBaseURL, nil
}

func applyUpstreamPathTemplate(upstreamPath string, info *relaycommon.RelayInfo) string {
	if info == nil {
		return upstreamPath
	}
	return strings.ReplaceAll(upstreamPath, advancedCustomModelPlaceholder, info.UpstreamModelName)
}

func shouldUseGeminiStreamURL(converter string, info *relaycommon.RelayInfo) bool {
	return info != nil &&
		info.IsStream &&
		(converter == relayconvert.ConverterOpenAIChatToGeminiContent ||
			converter == relayconvert.ConverterOpenAIResponsesToGemini)
}

func useGeminiStreamGenerateContentURL(parsedURL *url.URL) {
	if strings.Contains(parsedURL.Path, ":generateContent") {
		parsedURL.Path = strings.Replace(parsedURL.Path, ":generateContent", ":streamGenerateContent", 1)
	}
	if strings.Contains(parsedURL.Path, ":streamGenerateContent") {
		query := parsedURL.Query()
		query.Set("alt", "sse")
		parsedURL.RawQuery = query.Encode()
	}
}

func shouldApplyClaudeHeaders(converter string, info *relaycommon.RelayInfo) bool {
	return converter == relayconvert.ConverterOpenAIChatToClaudeMessages ||
		(converter == relayconvert.ConverterNone && info != nil && info.RelayFormat == types.RelayFormatClaude)
}

func applyClaudeHeaders(c *gin.Context, header *http.Header, info *relaycommon.RelayInfo) {
	anthropicVersion := ""
	if c != nil && c.Request != nil {
		anthropicVersion = c.Request.Header.Get("anthropic-version")
	}
	if anthropicVersion == "" {
		anthropicVersion = "2023-06-01"
	}
	header.Set("anthropic-version", anthropicVersion)
	if c != nil {
		claude.CommonClaudeHeadersOperation(c, header, info)
	}
}

func applyAuthTemplate(template string, apiKey string) string {
	return strings.ReplaceAll(template, "{api_key}", apiKey)
}

func isJSONRequest(c *gin.Context) bool {
	if c == nil || c.Request == nil {
		return false
	}
	return strings.Contains(strings.ToLower(c.Request.Header.Get("Content-Type")), "application/json")
}

// convertCrossProtocolChatRequest converts a Claude, Gemini, or Responses request into an
// OpenAI chat completions request for an OpenAI-compatible upstream. Streaming requests
// always ask the upstream for usage, because the downstream protocol reports usage in its
// own format and never carries stream_options itself.
func (a *Adaptor) convertCrossProtocolChatRequest(c *gin.Context, info *relaycommon.RelayInfo, converter string, request any) (any, error) {
	result, err := service.ConvertRequestByID(c, info, converter, request)
	if err != nil {
		return nil, err
	}
	chatRequest, ok := result.Value.(*dto.GeneralOpenAIRequest)
	if !ok {
		return nil, fmt.Errorf("expected OpenAI chat completions request, got %T", result.Value)
	}
	if info.SupportStreamOptions && info.IsStream {
		chatRequest.StreamOptions = &dto.StreamOptions{IncludeUsage: true}
	}
	return a.convertOpenAICompatibleRequest(c, info, chatRequest)
}

// convertCrossProtocolResponsesRequest converts a Claude or Gemini client
// request for an OpenAI Responses upstream.
func convertCrossProtocolResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, converter string, request any) (any, error) {
	result, err := service.ConvertRequestByID(c, info, converter, request)
	if err != nil {
		return nil, err
	}
	responsesRequest, ok := result.Value.(*dto.OpenAIResponsesRequest)
	if !ok {
		return nil, fmt.Errorf("expected OpenAI Responses request, got %T", result.Value)
	}
	return responsesRequest, nil
}

func (a *Adaptor) convertOpenAICompatibleRequest(c *gin.Context, info *relaycommon.RelayInfo, request *dto.GeneralOpenAIRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertOpenAIRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertOpenAICompatibleResponsesRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.OpenAIResponsesRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertOpenAIResponsesRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertOpenAICompatibleEmbeddingRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.EmbeddingRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertEmbeddingRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertOpenAICompatibleAudioRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.AudioRequest) (io.Reader, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertAudioRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}

func (a *Adaptor) convertOpenAICompatibleImageRequest(c *gin.Context, info *relaycommon.RelayInfo, request dto.ImageRequest) (any, error) {
	old := info.ChannelType
	info.ChannelType = constant.ChannelTypeOpenAI
	converted, err := a.openaiAdaptor.ConvertImageRequest(c, info, request)
	info.ChannelType = old
	return converted, err
}
