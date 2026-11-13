package dto

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/relaykit/types"
)

type ChannelSettings struct {
	TaskPluginKey             string `json:"task_plugin_key,omitempty"`
	ForceFormat               bool   `json:"force_format,omitempty"`
	ThinkingToContent         bool   `json:"thinking_to_content,omitempty"`
	Proxy                     string `json:"proxy"`
	PassThroughBodyEnabled    bool   `json:"pass_through_body_enabled,omitempty"`
	ResponsesWebSocketEnabled bool   `json:"responses_websocket_enabled,omitempty"`
	SystemPrompt              string `json:"system_prompt,omitempty"`
	SystemPromptOverride      bool   `json:"system_prompt_override,omitempty"`
	// TaskExtendPluginKeys lists the task plugins a New API channel (type 60)
	// is extended with. The upstream gateway may host many plugins, so the
	// channel serves every listed plugin's models while the request still pins
	// the executing plugin. TaskPluginKey remains the single type-61 binding
	// and stays valid on a New API channel as well.
	TaskExtendPluginKeys []string `json:"task_extend_plugin_keys,omitempty"`
	// HTTPProtocol controls outbound HTTP version negotiation for this channel.
	// Accepted values: "", "auto" (default), "http1".
	HTTPProtocol string `json:"http_protocol,omitempty"`
	// HTTP2ConnectionShards spreads HTTP/2 traffic across N independent transports
	// (1-8). Zero/unset means 1. Ignored when HTTPProtocol is "http1".
	HTTP2ConnectionShards int `json:"http2_connection_shards,omitempty"`
	// TLSInsecureSkipVerify accepts upstream certificates that fail
	// verification, such as self-signed ones, for this channel only.
	TLSInsecureSkipVerify bool `json:"tls_insecure_skip_verify,omitempty"`
}

// BindsTaskPlugin reports whether the channel is bound to the task plugin,
// either through the single binding or the New API extension list.
func (s ChannelSettings) BindsTaskPlugin(key string) bool {
	if key == "" {
		return false
	}
	return s.TaskPluginKey == key || slices.Contains(s.TaskExtendPluginKeys, key)
}

// TaskPluginBindings returns the sorted, de-duplicated set of task plugins the
// channel binds through either field, so callers can compare bindings as a set.
func (s ChannelSettings) TaskPluginBindings() []string {
	bindings := make([]string, 0, len(s.TaskExtendPluginKeys)+1)
	if s.TaskPluginKey != "" {
		bindings = append(bindings, s.TaskPluginKey)
	}
	for _, key := range s.TaskExtendPluginKeys {
		if key != "" && !slices.Contains(bindings, key) {
			bindings = append(bindings, key)
		}
	}
	slices.Sort(bindings)
	return bindings
}

const (
	HTTPProtocolAuto         = "auto"
	HTTPProtocolHTTP1        = "http1"
	MaxHTTP2ConnectionShards = 8
)

// ValidateHTTPTransport validates save-time HTTP transport channel settings.
func (s *ChannelSettings) ValidateHTTPTransport() error {
	if s == nil {
		return nil
	}
	protocol := strings.ToLower(strings.TrimSpace(s.HTTPProtocol))
	switch protocol {
	case "", HTTPProtocolAuto, HTTPProtocolHTTP1:
	default:
		return fmt.Errorf("invalid http_protocol: %s", s.HTTPProtocol)
	}
	if s.HTTP2ConnectionShards < 0 || s.HTTP2ConnectionShards > MaxHTTP2ConnectionShards {
		return fmt.Errorf("invalid http2_connection_shards: %d", s.HTTP2ConnectionShards)
	}
	if protocol == HTTPProtocolHTTP1 && s.HTTP2ConnectionShards > 1 {
		return fmt.Errorf("http2_connection_shards must be 1 when http_protocol is http1")
	}
	return nil
}

type VertexKeyType string

const (
	VertexKeyTypeJSON   VertexKeyType = "json"
	VertexKeyTypeAPIKey VertexKeyType = "api_key"
)

type AwsKeyType string

const (
	AwsKeyTypeAKSK   AwsKeyType = "ak_sk" // 默认
	AwsKeyTypeApiKey AwsKeyType = "api_key"
)

type ChannelOtherSettings struct {
	// ImageTaskContract opts a channel into a verified image-task wire contract.
	// It never grants a mode merely because an upstream model has the same name.
	ImageTaskContract                     *ImageTaskContract    `json:"image_task_contract,omitempty"`
	AzureResponsesVersion                 string                `json:"azure_responses_version,omitempty"`
	VertexKeyType                         VertexKeyType         `json:"vertex_key_type,omitempty"` // "json" or "api_key"
	OpenRouterEnterprise                  *bool                 `json:"openrouter_enterprise,omitempty"`
	ClaudeBetaQuery                       bool                  `json:"claude_beta_query,omitempty"`          // Claude 渠道是否强制追加 ?beta=true
	AllowServiceTier                      bool                  `json:"allow_service_tier,omitempty"`         // 是否允许 service_tier 透传（默认过滤以避免额外计费）
	AllowInferenceGeo                     bool                  `json:"allow_inference_geo,omitempty"`        // 是否允许 inference_geo 透传（仅 Claude，默认过滤以满足数据驻留合规
	AllowSpeed                            bool                  `json:"allow_speed,omitempty"`                // 是否允许 speed 透传（仅 Claude，默认过滤以避免意外切换推理速度模式）
	AllowSafetyIdentifier                 bool                  `json:"allow_safety_identifier,omitempty"`    // 是否允许 safety_identifier 透传（默认过滤以保护用户隐私）
	DisableStore                          bool                  `json:"disable_store,omitempty"`              // 是否禁用 store 透传（默认允许透传，禁用后可能导致 Codex 无法使用）
	AllowIncludeObfuscation               bool                  `json:"allow_include_obfuscation,omitempty"`  // 是否允许 stream_options.include_obfuscation 透传（默认过滤以避免关闭流混淆保护）
	DisableTaskPollingSleep               bool                  `json:"disable_task_polling_sleep,omitempty"` // 是否跳过异步任务轮询间隔
	AwsKeyType                            AwsKeyType            `json:"aws_key_type,omitempty"`
	UpstreamModelUpdateCheckEnabled       bool                  `json:"upstream_model_update_check_enabled,omitempty"`        // 是否检测上游模型更新
	UpstreamModelUpdateAutoSyncEnabled    bool                  `json:"upstream_model_update_auto_sync_enabled,omitempty"`    // 是否自动同步上游模型更新
	UpstreamModelUpdateLastCheckTime      int64                 `json:"upstream_model_update_last_check_time,omitempty"`      // 上次检测时间
	UpstreamModelUpdateLastDetectedModels []string              `json:"upstream_model_update_last_detected_models,omitempty"` // 上次检测到的可加入模型
	UpstreamModelUpdateLastRemovedModels  []string              `json:"upstream_model_update_last_removed_models,omitempty"`  // 上次检测到的可删除模型
	UpstreamModelUpdateIgnoredModels      []string              `json:"upstream_model_update_ignored_models,omitempty"`       // 手动忽略的模型
	AdvancedCustom                        *AdvancedCustomConfig `json:"advanced_custom,omitempty"`
	// OllamaOpenAIChat routes Ollama chat completions to the OpenAI-compatible
	// /v1/chat/completions endpoint. When unset, chat completions keep using
	// the native /api/chat protocol.
	OllamaOpenAIChat bool `json:"ollama_openai_chat,omitempty"`
	// ToolLossPolicy is a channel-level opt-in for request-phase conversion
	// rejection. Empty follows the default allow policy. Accepted values:
	// "", "allow", "safe", "strict".
	ToolLossPolicy string `json:"tool_loss_policy,omitempty"`
}

const (
	ImageTaskProtocolOpenAIMultipart = "openai_multipart"
	ImageTaskProtocolEvoLinkJSON     = "evolink_json"
	ImageTaskModeText                = "text"
	ImageTaskModeReference           = "reference"
	ImageTaskModeEdit                = "edit"
	ImageTaskModeMaskedEdit          = "masked_edit"
)

// ImageTaskContract describes capabilities evidenced for one channel. Transport
// paths and field names belong to the adaptor rather than mutable settings.
type ImageTaskContract struct {
	Protocol     string   `json:"protocol"`
	Modes        []string `json:"modes"`
	InputFormats []string `json:"input_formats,omitempty"`
	Sizes        []string `json:"sizes,omitempty"`
	Delivery     string   `json:"delivery,omitempty"`
}

const (
	ImageTaskSize1K         = "1024x576"
	ImageTaskSize2K         = "2048x1152"
	ImageTaskDeliveryURL    = "url"
	ImageTaskDeliveryBase64 = "b64_json"
)

// SupportsSize preserves the historical 2K-only contract when sizes is omitted.
// A new specification is eligible only through an explicit audited declaration.
func (c *ImageTaskContract) SupportsSize(size string) bool {
	if c == nil || (size != ImageTaskSize1K && size != ImageTaskSize2K) {
		return false
	}
	if len(c.Sizes) == 0 {
		return size == ImageTaskSize2K
	}
	return slices.Contains(c.Sizes, size)
}

// DeliveryFormat affects only the supplier wire request, never the public task.
func (c *ImageTaskContract) DeliveryFormat() string {
	if c == nil {
		return ""
	}
	if c.Delivery == "" {
		return ImageTaskDeliveryURL
	}
	return c.Delivery
}

func (c *ImageTaskContract) SupportsMode(mode string) bool {
	return c != nil && slices.Contains(c.Modes, mode)
}

// SupportsInputFormat never treats an omitted audit declaration as all formats.
// PNG is the narrow initial contract; wider input formats require explicit proof.
func (c *ImageTaskContract) SupportsInputFormat(format string) bool {
	if c == nil {
		return false
	}
	if len(c.InputFormats) == 0 {
		return format == "png"
	}
	return slices.Contains(c.InputFormats, format)
}

func (s *ChannelOtherSettings) ValidateImageTaskContract() error {
	if s == nil || s.ImageTaskContract == nil {
		return nil
	}
	c := s.ImageTaskContract
	switch c.Protocol {
	case ImageTaskProtocolOpenAIMultipart, ImageTaskProtocolEvoLinkJSON:
	default:
		return fmt.Errorf("invalid image_task_contract protocol")
	}
	if len(c.Modes) == 0 {
		return fmt.Errorf("image_task_contract modes are required")
	}
	seen := make(map[string]bool, len(c.Modes))
	for _, mode := range c.Modes {
		switch mode {
		case ImageTaskModeText, ImageTaskModeReference, ImageTaskModeEdit, ImageTaskModeMaskedEdit:
		default:
			return fmt.Errorf("invalid image_task_contract mode")
		}
		if seen[mode] {
			return fmt.Errorf("duplicate image_task_contract mode")
		}
		seen[mode] = true
	}
	// Multipart masks have no implemented request contract; reject the whole
	// declaration before it can qualify a channel for native selection.
	if c.Protocol == ImageTaskProtocolOpenAIMultipart && seen[ImageTaskModeMaskedEdit] {
		return fmt.Errorf("invalid image_task_contract mode")
	}
	formats := make(map[string]bool, len(c.InputFormats))
	for _, format := range c.InputFormats {
		switch format {
		case "png", "jpeg", "webp":
		default:
			return fmt.Errorf("invalid image_task_contract input format")
		}
		if formats[format] {
			return fmt.Errorf("duplicate image_task_contract input format")
		}
		formats[format] = true
	}
	sizes := make(map[string]bool, len(c.Sizes))
	for _, size := range c.Sizes {
		if size != ImageTaskSize1K && size != ImageTaskSize2K {
			return fmt.Errorf("invalid image_task_contract size")
		}
		if sizes[size] {
			return fmt.Errorf("duplicate image_task_contract size")
		}
		sizes[size] = true
	}
	switch c.Delivery {
	case "", ImageTaskDeliveryURL, ImageTaskDeliveryBase64:
	default:
		return fmt.Errorf("invalid image_task_contract delivery")
	}
	return nil
}

func (s *ChannelOtherSettings) IsOpenRouterEnterprise() bool {
	if s == nil || s.OpenRouterEnterprise == nil {
		return false
	}
	return *s.OpenRouterEnterprise
}

// ValidateToolLossPolicy validates the channel-level request-phase tool-loss
// policy. Empty keeps the default allow policy.
func (s *ChannelOtherSettings) ValidateToolLossPolicy() error {
	if s == nil {
		return nil
	}
	switch strings.TrimSpace(s.ToolLossPolicy) {
	case "", string(types.ConversionLossPolicyAllow), string(types.ConversionLossPolicySafe), string(types.ConversionLossPolicyStrict):
		return nil
	default:
		return fmt.Errorf("invalid tool_loss_policy: %s", s.ToolLossPolicy)
	}
}

const (
	AdvancedCustomConverterSGLangRerank                = "jina_rerank_to_sglang"
	advancedCustomConverterNone                        = "none"
	advancedCustomConverterClaudeMessagesToOpenAIChat  = "anthropic_messages_to_openai_chat_completions"
	advancedCustomConverterOpenAIChatToClaudeMessages  = "openai_chat_completions_to_anthropic_messages"
	advancedCustomConverterOpenAIChatToOpenAIResponses = "openai_chat_completions_to_openai_responses"
	advancedCustomConverterOpenAIResponsesToOpenAIChat = "openai_responses_to_openai_chat_completions"
	advancedCustomConverterOpenAIResponsesToGemini     = "openai_responses_to_gemini_generate_content"
	advancedCustomConverterGeminiContentToOpenAIChat   = "gemini_generate_content_to_openai_chat_completions"
	advancedCustomConverterOpenAIChatToGeminiContent   = "openai_chat_completions_to_gemini_generate_content"
	// Same values as relayconvert.ConverterClaudeMessagesToOpenAIResponses and
	// relayconvert.ConverterGeminiContentToOpenAIResponses.
	advancedCustomConverterClaudeMessagesToOpenAIResponses = "claude_messages_to_openai_responses"
	advancedCustomConverterGeminiContentToOpenAIResponses  = "gemini_generate_content_to_openai_responses"
)

const (
	AdvancedCustomAuthTypeNone   = "none"
	AdvancedCustomAuthTypeHeader = "header"
	AdvancedCustomAuthTypeQuery  = "query"

	AdvancedCustomResponseValidationTikHubUser  = "tikhub-user"
	AdvancedCustomResponseValidationTikHubPosts = "tikhub-posts"
)

type AdvancedCustomConfig struct {
	Routes []AdvancedCustomRoute `json:"advanced_routes,omitempty"`
	// ModelByParam picks the upstream model from a request parameter, so one
	// public model ID can fan out to the SKU that matches the request (image
	// resolution in `size`, for example) without splitting the public catalog.
	// It only decides the upstream name: billing keeps using the origin model.
	ModelByParam map[string]ParamModelRouting `json:"model_by_param,omitempty"`
}

// ParamModelRouting describes the upstream-model rules for one public model ID.
type ParamModelRouting struct {
	// Param is the request field to read. Empty means "size".
	Param string           `json:"param,omitempty"`
	Rules []ParamModelRule `json:"rules,omitempty"`
}

// ParamModelRule maps a set of parameter values to one upstream model name.
type ParamModelRule struct {
	// AnyOf lists the accepted parameter values (compared case-insensitively,
	// surrounding whitespace ignored).
	AnyOf []string `json:"any_of,omitempty"`
	// Model is the upstream model name to use when a value matches.
	Model string `json:"model"`
}

// ResolveModelByParam returns the upstream model for publicModel given the
// request parameters. lookup reads one parameter by name and may return "".
//
// It reports false whenever the channel is not configured for this model, no
// rule matches, or the matched rule carries no upstream name — callers must
// then keep the model name they already resolved (channel model_mapping or the
// origin model), which keeps unconfigured channels byte-for-byte unchanged.
func (c *AdvancedCustomConfig) ResolveModelByParam(publicModel string, lookup func(param string) string) (string, bool) {
	if c == nil || len(c.ModelByParam) == 0 || lookup == nil {
		return "", false
	}
	routing, ok := c.ModelByParam[strings.TrimSpace(publicModel)]
	if !ok || len(routing.Rules) == 0 {
		return "", false
	}
	param := strings.ToLower(strings.TrimSpace(routing.Param))
	if param == "" {
		param = "size"
	}
	value := strings.ToLower(strings.TrimSpace(lookup(param)))
	if value == "" {
		return "", false
	}
	for _, rule := range routing.Rules {
		model := strings.TrimSpace(rule.Model)
		if model == "" {
			continue
		}
		for _, candidate := range rule.AnyOf {
			if strings.ToLower(strings.TrimSpace(candidate)) == value {
				return model, true
			}
		}
	}
	return "", false
}

type AdvancedCustomRoute struct {
	IncomingPath string                   `json:"incoming_path,omitempty"`
	UpstreamPath string                   `json:"upstream_path,omitempty"`
	Converter    string                   `json:"converter,omitempty"`
	Models       []string                 `json:"models,omitempty"`
	Auth         *AdvancedCustomRouteAuth `json:"auth,omitempty"`
	// PassThroughBodyEnabled forwards the client request body to the upstream
	// untouched (upstream semantics: request-body passthrough, requires a
	// body-preserving converter — see SupportsPassThroughBody). Not to be
	// confused with PassthroughResponse below, which governs the response.
	PassThroughBodyEnabled bool `json:"pass_through_body_enabled,omitempty"`
	// UpstreamMethod overrides the client HTTP method when calling the upstream.
	// Empty keeps the client method (backward compatible). Allowed: GET, POST, PUT, PATCH, DELETE, HEAD.
	UpstreamMethod string `json:"upstream_method,omitempty"`
	// QueryMapping maps upstream query parameter names to gjson paths in the
	// original client request JSON body (e.g. {"aweme_id":"aweme_id"}).
	QueryMapping map[string]string `json:"query_mapping,omitempty"`
	// PassthroughResponse, when true, returns the upstream body as-is and synthesizes
	// a minimal usage for ModelPrice billing. When nil, defaults to true for GET/HEAD
	// upstream methods and false otherwise.
	PassthroughResponse *bool `json:"passthrough_response,omitempty"`
	// ResponseValidation opts the four account-monitoring GET routes into
	// business-response validation before per-call billing. Empty preserves
	// existing passthrough behavior.
	ResponseValidation string `json:"response_validation,omitempty"`
}

// SupportsPassThroughBody reports whether the route converter leaves the request body untouched.
func (r AdvancedCustomRoute) SupportsPassThroughBody() bool {
	switch strings.TrimSpace(r.Converter) {
	case "", advancedCustomConverterNone, AdvancedCustomConverterSGLangRerank:
		return true
	default:
		return false
	}
}

type AdvancedCustomRouteAuth struct {
	Type  string `json:"type,omitempty"`
	Name  string `json:"name,omitempty"`
	Value string `json:"value,omitempty"`
}

const (
	advancedCustomModelPlaceholder = "{model}"
	advancedCustomModelRegexPrefix = "re:"
)

const (
	advancedCustomEndpointPathOpenAIChat             = "/v1/chat/completions"
	advancedCustomEndpointPathOpenAIResponses        = "/v1/responses"
	advancedCustomEndpointPathOpenAIResponsesCompact = "/v1/responses/compact"
	advancedCustomEndpointPathOpenAIAlphaSearch      = "/v1/alpha/search"
	advancedCustomEndpointPathClaudeMessages         = "/v1/messages"
	advancedCustomEndpointPathJinaRerank             = "/v1/rerank"
	advancedCustomEndpointPathImageGeneration        = "/v1/images/generations"
	advancedCustomEndpointPathEmbeddings             = "/v1/embeddings"
)

const (
	// AdvancedCustomModelListPath identifies the optional OpenAI Models discovery route.
	AdvancedCustomModelListPath = "/v1/models"
	// AdvancedCustomBalancePath identifies the optional balance lookup route used by channel management.
	AdvancedCustomBalancePath = "/v1/dashboard/billing/credit_grants"
)

// IsNative reports whether the route forwards requests without protocol
// conversion. Validate normalizes an empty converter to none, but callers may
// see configurations that were never saved.
func (r AdvancedCustomRoute) IsNative() bool {
	converter := strings.TrimSpace(r.Converter)
	return converter == "" || converter == advancedCustomConverterNone
}

// MatchPath returns the first route whose IncomingPath matches requestPath.
// Matching mirrors the relay adaptor: exact match, {model} placeholder, and
// :generateContent <-> :streamGenerateContent equivalence.
func (c *AdvancedCustomConfig) MatchPath(requestPath string) (AdvancedCustomRoute, bool) {
	if c == nil {
		return AdvancedCustomRoute{}, false
	}
	for _, route := range c.Routes {
		if matchAdvancedCustomIncomingPath(strings.TrimSpace(route.IncomingPath), requestPath) {
			return route, true
		}
	}
	return AdvancedCustomRoute{}, false
}

// MatchPathForModel returns the first route whose IncomingPath and Models match.
// An empty Models list is a catch-all fallback for that incoming path.
func (c *AdvancedCustomConfig) MatchPathForModel(requestPath string, model string) (AdvancedCustomRoute, bool) {
	if c == nil {
		return AdvancedCustomRoute{}, false
	}
	model = strings.TrimSpace(model)
	for _, route := range c.Routes {
		if matchAdvancedCustomIncomingPath(strings.TrimSpace(route.IncomingPath), requestPath) &&
			matchAdvancedCustomRouteModel(route.Models, model) {
			return route, true
		}
	}
	return AdvancedCustomRoute{}, false
}

// ModelListRoute returns the explicitly configured OpenAI Models discovery route.
// Template routes that merely happen to match /v1/models are not discovery routes.
func (c *AdvancedCustomConfig) ModelListRoute() (AdvancedCustomRoute, bool) {
	if c == nil {
		return AdvancedCustomRoute{}, false
	}
	for _, route := range c.Routes {
		if strings.TrimSpace(route.IncomingPath) == AdvancedCustomModelListPath {
			return route, true
		}
	}
	return AdvancedCustomRoute{}, false
}

// BalanceRoute returns the explicitly configured channel-management balance route.
func (c *AdvancedCustomConfig) BalanceRoute() (AdvancedCustomRoute, bool) {
	if c == nil {
		return AdvancedCustomRoute{}, false
	}
	for _, route := range c.Routes {
		if strings.TrimSpace(route.IncomingPath) == AdvancedCustomBalancePath {
			return route, true
		}
	}
	return AdvancedCustomRoute{}, false
}

// SupportsPath reports whether any route matches requestPath.
func (c *AdvancedCustomConfig) SupportsPath(requestPath string) bool {
	_, ok := c.MatchPath(requestPath)
	return ok
}

// SupportsPathForModel reports whether any route matches requestPath and model.
func (c *AdvancedCustomConfig) SupportsPathForModel(requestPath string, model string) bool {
	_, ok := c.MatchPathForModel(requestPath, model)
	return ok
}

func (c *AdvancedCustomConfig) SupportedEndpointTypesForModel(model string) []types.EndpointType {
	if c == nil {
		return nil
	}
	model = strings.TrimSpace(model)
	endpoints := make([]types.EndpointType, 0, len(c.Routes))
	seen := make(map[types.EndpointType]struct{}, len(c.Routes))
	for _, route := range c.Routes {
		if !matchAdvancedCustomRouteModel(route.Models, model) {
			continue
		}
		endpointType, ok := advancedCustomEndpointTypeFromIncomingPath(strings.TrimSpace(route.IncomingPath))
		if !ok {
			continue
		}
		if _, exists := seen[endpointType]; exists {
			continue
		}
		seen[endpointType] = struct{}{}
		endpoints = append(endpoints, endpointType)
	}
	return endpoints
}

func advancedCustomEndpointTypeFromIncomingPath(incomingPath string) (types.EndpointType, bool) {
	switch incomingPath {
	case advancedCustomEndpointPathOpenAIChat:
		return types.EndpointTypeOpenAI, true
	case advancedCustomEndpointPathOpenAIResponses:
		return types.EndpointTypeOpenAIResponse, true
	case advancedCustomEndpointPathOpenAIResponsesCompact:
		return types.EndpointTypeOpenAIResponseCompact, true
	case advancedCustomEndpointPathOpenAIAlphaSearch:
		return types.EndpointTypeOpenAIAlphaSearch, true
	case advancedCustomEndpointPathClaudeMessages:
		return types.EndpointTypeAnthropic, true
	case advancedCustomEndpointPathJinaRerank:
		return types.EndpointTypeJinaRerank, true
	case advancedCustomEndpointPathImageGeneration:
		return types.EndpointTypeImageGeneration, true
	case advancedCustomEndpointPathEmbeddings:
		return types.EndpointTypeEmbeddings, true
	default:
		if isAdvancedCustomGeminiIncomingPath(incomingPath) {
			return types.EndpointTypeGemini, true
		}
		return "", false
	}
}

func isAdvancedCustomGeminiIncomingPath(incomingPath string) bool {
	if !strings.HasPrefix(incomingPath, "/v1beta/models/") {
		return false
	}
	return strings.Contains(incomingPath, ":generateContent") || strings.Contains(incomingPath, ":streamGenerateContent")
}

func matchAdvancedCustomRouteModel(models []string, model string) bool {
	normalizedModels := normalizeAdvancedCustomRouteModels(models)
	if len(normalizedModels) == 0 {
		return true
	}
	for _, allowedModel := range normalizedModels {
		if matchAdvancedCustomRouteModelRule(allowedModel, model) {
			return true
		}
	}
	return false
}

// advancedCustomModelRegexCache caches compiled route model patterns. Route model
// matching runs on the request hot path (distributor affinity, ability filtering,
// channel cache filtering, adaptor resolve), so patterns must not be recompiled per
// request. Invalid patterns are cached as nil to avoid recompiling them as well.
var advancedCustomModelRegexCache sync.Map // pattern string -> *regexp.Regexp (nil when invalid)

func compileAdvancedCustomModelRegex(pattern string) *regexp.Regexp {
	if cached, ok := advancedCustomModelRegexCache.Load(pattern); ok {
		re, _ := cached.(*regexp.Regexp)
		return re
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		re = nil
	}
	advancedCustomModelRegexCache.Store(pattern, re)
	return re
}

func matchAdvancedCustomRouteModelRule(rule string, model string) bool {
	if !strings.HasPrefix(rule, advancedCustomModelRegexPrefix) {
		return rule == model
	}
	pattern := strings.TrimPrefix(rule, advancedCustomModelRegexPrefix)
	if pattern == "" {
		return false
	}
	re := compileAdvancedCustomModelRegex(pattern)
	return re != nil && re.MatchString(model)
}

func matchAdvancedCustomIncomingPath(configuredPath string, requestPath string) bool {
	if matchAdvancedCustomIncomingPathTemplate(configuredPath, requestPath) {
		return true
	}
	if strings.Contains(configuredPath, ":generateContent") {
		streamPath := strings.Replace(configuredPath, ":generateContent", ":streamGenerateContent", 1)
		return matchAdvancedCustomIncomingPathTemplate(streamPath, requestPath)
	}
	return false
}

func matchAdvancedCustomIncomingPathTemplate(configuredPath string, requestPath string) bool {
	if !strings.Contains(configuredPath, advancedCustomModelPlaceholder) {
		return configuredPath == requestPath
	}

	parts := strings.Split(configuredPath, advancedCustomModelPlaceholder)
	if len(parts) != 2 {
		return false
	}
	if !strings.HasPrefix(requestPath, parts[0]) || !strings.HasSuffix(requestPath, parts[1]) {
		return false
	}

	model := strings.TrimSuffix(strings.TrimPrefix(requestPath, parts[0]), parts[1])
	return model != "" && !strings.Contains(model, "/")
}

func IsAdvancedCustomConverterAllowed(converter string) bool {
	switch converter {
	case advancedCustomConverterNone,
		AdvancedCustomConverterSGLangRerank,
		advancedCustomConverterClaudeMessagesToOpenAIChat,
		advancedCustomConverterOpenAIChatToClaudeMessages,
		advancedCustomConverterOpenAIChatToOpenAIResponses,
		advancedCustomConverterOpenAIResponsesToOpenAIChat,
		advancedCustomConverterOpenAIResponsesToGemini,
		advancedCustomConverterGeminiContentToOpenAIChat,
		advancedCustomConverterOpenAIChatToGeminiContent,
		advancedCustomConverterClaudeMessagesToOpenAIResponses,
		advancedCustomConverterGeminiContentToOpenAIResponses:
		return true
	default:
		return false
	}
}

func (c *AdvancedCustomConfig) Validate() error {
	if c == nil {
		return fmt.Errorf("advanced_custom is required")
	}
	if len(c.Routes) == 0 {
		return fmt.Errorf("advanced_custom requires at least one route")
	}

	paths := make(map[string]*advancedCustomPathModelState, len(c.Routes))
	modelListRouteIndex := -1
	balanceRouteIndex := -1
	for i := range c.Routes {
		route := c.Routes[i]
		route.IncomingPath = strings.TrimSpace(route.IncomingPath)
		upstreamPath := strings.TrimSpace(route.UpstreamPath)
		route.Converter = strings.TrimSpace(route.Converter)
		if route.Converter == "" {
			route.Converter = advancedCustomConverterNone
		}

		if route.IncomingPath == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].incoming_path is required", i)
		}
		if !strings.HasPrefix(route.IncomingPath, "/") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].incoming_path must start with /", i)
		}
		if strings.Contains(route.IncomingPath, "?") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].incoming_path must not include query", i)
		}
		if route.IncomingPath == AdvancedCustomModelListPath || route.IncomingPath == AdvancedCustomBalancePath {
			managementRouteName := route.IncomingPath
			previousIndex := modelListRouteIndex
			if route.IncomingPath == AdvancedCustomBalancePath {
				previousIndex = balanceRouteIndex
			}
			if previousIndex >= 0 {
				return fmt.Errorf("advanced_custom.advanced_routes[%d] duplicates the %s route at advanced_routes[%d]", i, managementRouteName, previousIndex)
			}
			if route.IncomingPath == AdvancedCustomModelListPath {
				modelListRouteIndex = i
			} else {
				balanceRouteIndex = i
			}
			if len(normalizeAdvancedCustomRouteModels(route.Models)) > 0 {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].models must be empty for %s", i, managementRouteName)
			}
			if route.Converter != advancedCustomConverterNone {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].converter must be none for %s", i, managementRouteName)
			}
			if strings.Contains(upstreamPath, advancedCustomModelPlaceholder) {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path must not contain %s for %s", i, advancedCustomModelPlaceholder, managementRouteName)
			}
			if route.PassThroughBodyEnabled {
				return fmt.Errorf("advanced_custom.advanced_routes[%d].pass_through_body_enabled must be false for %s", i, managementRouteName)
			}
		}
		if err := validateAdvancedCustomRouteModels(i, route.IncomingPath, route.Models, paths); err != nil {
			return err
		}

		if upstreamPath == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path is required", i)
		}
		if err := validateAdvancedCustomUpstreamTarget(i, upstreamPath); err != nil {
			return err
		}

		if !IsAdvancedCustomConverterAllowed(route.Converter) {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].converter is not registered: %s", i, route.Converter)
		}
		if err := validateAdvancedCustomConverterPath(i, route.IncomingPath, route.Converter); err != nil {
			return err
		}
		if route.PassThroughBodyEnabled && !route.SupportsPassThroughBody() {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].pass_through_body_enabled requires converter none: %s", i, route.Converter)
		}
		if err := validateAdvancedCustomRouteAuth(i, route.Auth); err != nil {
			return err
		}
		if err := validateAdvancedCustomUpstreamMethod(i, route.UpstreamMethod, route.Converter); err != nil {
			return err
		}
		if err := validateAdvancedCustomQueryMapping(i, route.QueryMapping); err != nil {
			return err
		}
		if err := route.ValidateResponseValidation(); err != nil {
			return fmt.Errorf("advanced_custom.advanced_routes[%d]: %w", i, err)
		}
	}

	return nil
}

// ValidateResponseValidation restricts account-monitoring validation to the
// four known raw GET contracts. It never changes unconfigured routes.
func (r AdvancedCustomRoute) ValidateResponseValidation() error {
	policy := strings.TrimSpace(r.ResponseValidation)
	if policy == "" {
		return nil
	}
	if policy != AdvancedCustomResponseValidationTikHubUser && policy != AdvancedCustomResponseValidationTikHubPosts {
		return fmt.Errorf("unsupported response_validation")
	}
	if strings.TrimSpace(r.IncomingPath) != "/v1/chat/completions" ||
		r.EffectiveUpstreamMethod() != http.MethodGet ||
		!r.ShouldPassthroughResponse() ||
		(strings.TrimSpace(r.Converter) != "" && strings.TrimSpace(r.Converter) != advancedCustomConverterNone) || len(r.Models) != 1 {
		return fmt.Errorf("response_validation requires a fixed account-monitoring GET passthrough route")
	}
	var expectedPath, expectedPolicy string
	switch strings.TrimSpace(r.Models[0]) {
	case "x-user":
		expectedPath, expectedPolicy = "/api/v1/twitter/web/fetch_user_profile", AdvancedCustomResponseValidationTikHubUser
	case "x-posts":
		expectedPath, expectedPolicy = "/api/v1/twitter/web/fetch_user_post_tweet", AdvancedCustomResponseValidationTikHubPosts
	case "tiktok-user":
		expectedPath, expectedPolicy = "/api/v1/tiktok/web/fetch_user_profile", AdvancedCustomResponseValidationTikHubUser
	case "tiktok-posts":
		expectedPath, expectedPolicy = "/api/v1/tiktok/app/v3/fetch_user_post_videos", AdvancedCustomResponseValidationTikHubPosts
	default:
		return fmt.Errorf("response_validation requires an account-monitoring capability")
	}
	if policy != expectedPolicy || strings.TrimSpace(r.UpstreamPath) != expectedPath {
		return fmt.Errorf("response_validation does not match the account-monitoring route")
	}
	return nil
}

// EffectiveUpstreamMethod returns the configured upstream method, uppercased, or empty.
func (r AdvancedCustomRoute) EffectiveUpstreamMethod() string {
	return strings.ToUpper(strings.TrimSpace(r.UpstreamMethod))
}

// ShouldPassthroughResponse reports whether the route should stream the upstream
// body back without OpenAI response conversion.
func (r AdvancedCustomRoute) ShouldPassthroughResponse() bool {
	if r.PassthroughResponse != nil {
		return *r.PassthroughResponse
	}
	switch r.EffectiveUpstreamMethod() {
	case http.MethodGet, http.MethodHead:
		return true
	default:
		return false
	}
}

func validateAdvancedCustomUpstreamMethod(index int, method string, converter string) error {
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return nil
	}
	switch method {
	case http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodHead:
	default:
		return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_method is invalid: %s", index, method)
	}
	converter = strings.TrimSpace(converter)
	if converter == "" {
		converter = advancedCustomConverterNone
	}
	if (method == http.MethodGet || method == http.MethodHead) && converter != advancedCustomConverterNone {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].converter must be none when upstream_method is %s", index, method)
	}
	return nil
}

func validateAdvancedCustomQueryMapping(index int, mapping map[string]string) error {
	if len(mapping) == 0 {
		return nil
	}
	for key, path := range mapping {
		key = strings.TrimSpace(key)
		path = strings.TrimSpace(path)
		if key == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].query_mapping has empty query key", index)
		}
		if path == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].query_mapping[%s] path is empty", index, key)
		}
		if strings.Contains(path, "..") || strings.HasPrefix(path, "$") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].query_mapping[%s] path is not allowed", index, key)
		}
	}
	return nil
}

type advancedCustomPathModelState struct {
	catchAllIndex int
	modelIndexes  map[string]int
}

func validateAdvancedCustomRouteModels(index int, incomingPath string, models []string, paths map[string]*advancedCustomPathModelState) error {
	state := paths[incomingPath]
	if state == nil {
		state = &advancedCustomPathModelState{
			catchAllIndex: -1,
			modelIndexes:  make(map[string]int),
		}
		paths[incomingPath] = state
	}

	normalizedModels := normalizeAdvancedCustomRouteModels(models)
	if len(normalizedModels) == 0 {
		if state.catchAllIndex >= 0 {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].models catch-all already exists for incoming_path: %s", index, incomingPath)
		}
		state.catchAllIndex = index
		return nil
	}

	if state.catchAllIndex >= 0 {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].models catch-all route must be last for incoming_path: %s", index, incomingPath)
	}

	seenInRoute := make(map[string]struct{}, len(normalizedModels))
	for _, model := range normalizedModels {
		if err := validateAdvancedCustomRouteModelRule(index, incomingPath, model); err != nil {
			return err
		}
		if _, exists := seenInRoute[model]; exists {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].models contains duplicate model for incoming_path %s: %s", index, incomingPath, model)
		}
		seenInRoute[model] = struct{}{}
		if existingIndex, exists := state.modelIndexes[model]; exists {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].models overlaps with advanced_routes[%d] for incoming_path %s: %s", index, existingIndex, incomingPath, model)
		}
		state.modelIndexes[model] = index
	}
	return nil
}

func validateAdvancedCustomRouteModelRule(index int, incomingPath string, model string) error {
	if !strings.HasPrefix(model, advancedCustomModelRegexPrefix) {
		return nil
	}
	pattern := strings.TrimPrefix(model, advancedCustomModelRegexPrefix)
	if pattern == "" {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].models regex is empty for incoming_path %s: %s", index, incomingPath, model)
	}
	if _, err := regexp.Compile(pattern); err != nil {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].models regex is invalid for incoming_path %s: %s", index, incomingPath, model)
	}
	return nil
}

func normalizeAdvancedCustomRouteModels(models []string) []string {
	if len(models) == 0 {
		return nil
	}
	normalized := make([]string, 0, len(models))
	for _, model := range models {
		model = strings.TrimSpace(model)
		if model != "" {
			normalized = append(normalized, model)
		}
	}
	return normalized
}

func validateAdvancedCustomUpstreamTarget(index int, upstreamPath string) error {
	if strings.HasPrefix(upstreamPath, "/") {
		if strings.HasPrefix(upstreamPath, "//") {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path must be a full URL or a path starting with /", index)
		}
		return nil
	}

	parsedURL, err := url.Parse(upstreamPath)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path must be a full URL or a path starting with /", index)
	}
	if !strings.EqualFold(parsedURL.Scheme, "http") && !strings.EqualFold(parsedURL.Scheme, "https") {
		return fmt.Errorf("advanced_custom.advanced_routes[%d].upstream_path must use http or https", index)
	}
	return nil
}

func validateAdvancedCustomConverterPath(index int, incomingPath string, converter string) error {
	if converter == AdvancedCustomConverterSGLangRerank && (incomingPath == "/v1/rerank" || incomingPath == "/rerank") {
		return nil
	}
	if incomingPath == advancedCustomEndpointPathOpenAIAlphaSearch {
		if converter == advancedCustomConverterNone {
			return nil
		}
		return fmt.Errorf("advanced_custom.advanced_routes[%d].converter does not match incoming_path: %s", index, converter)
	}
	switch converter {
	case advancedCustomConverterNone:
		return nil
	case advancedCustomConverterClaudeMessagesToOpenAIChat,
		advancedCustomConverterClaudeMessagesToOpenAIResponses:
		if incomingPath == "/v1/messages" {
			return nil
		}
	case advancedCustomConverterOpenAIChatToClaudeMessages,
		advancedCustomConverterOpenAIChatToOpenAIResponses,
		advancedCustomConverterOpenAIChatToGeminiContent:
		if incomingPath == "/v1/chat/completions" {
			return nil
		}
	case advancedCustomConverterOpenAIResponsesToOpenAIChat:
		if incomingPath == "/v1/responses" {
			return nil
		}
	case advancedCustomConverterOpenAIResponsesToGemini:
		if incomingPath == "/v1/responses" {
			return nil
		}
	case advancedCustomConverterGeminiContentToOpenAIChat,
		advancedCustomConverterGeminiContentToOpenAIResponses:
		if strings.Contains(incomingPath, ":generateContent") || strings.Contains(incomingPath, ":streamGenerateContent") {
			return nil
		}
	}
	return fmt.Errorf("advanced_custom.advanced_routes[%d].converter does not match incoming_path: %s", index, converter)
}

func validateAdvancedCustomRouteAuth(index int, auth *AdvancedCustomRouteAuth) error {
	if auth == nil {
		return nil
	}
	authType := strings.TrimSpace(auth.Type)
	switch authType {
	case AdvancedCustomAuthTypeNone:
		return nil
	case AdvancedCustomAuthTypeHeader, AdvancedCustomAuthTypeQuery:
		if strings.TrimSpace(auth.Name) == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].auth.name is required", index)
		}
		if strings.TrimSpace(auth.Value) == "" {
			return fmt.Errorf("advanced_custom.advanced_routes[%d].auth.value is required", index)
		}
		return nil
	default:
		return fmt.Errorf("advanced_custom.advanced_routes[%d].auth.type is invalid: %s", index, auth.Type)
	}
}
