package helper

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/samber/lo"

	"github.com/gin-gonic/gin"
)

func GetAndValidateRequest(c *gin.Context, format types.RelayFormat) (request dto.Request, err error) {
	relayMode := relayconstant.Path2RelayMode(c.Request.URL.Path)

	switch format {
	case types.RelayFormatOpenAI:
		request, err = GetAndValidateTextRequest(c, relayMode)
	case types.RelayFormatGemini:
		if strings.Contains(c.Request.URL.Path, ":embedContent") {
			request, err = GetAndValidateGeminiEmbeddingRequest(c)
		} else if strings.Contains(c.Request.URL.Path, ":batchEmbedContents") {
			request, err = GetAndValidateGeminiBatchEmbeddingRequest(c)
		} else {
			request, err = GetAndValidateGeminiRequest(c)
		}
	case types.RelayFormatClaude:
		request, err = GetAndValidateClaudeRequest(c)
	case types.RelayFormatOpenAIResponses:
		request, err = GetAndValidateResponsesRequest(c)
	case types.RelayFormatOpenAIResponsesCompaction:
		request, err = GetAndValidateResponsesCompactionRequest(c)
	case types.RelayFormatOpenAIAlphaSearch:
		request, err = GetAndValidateAlphaSearchRequest(c)

	case types.RelayFormatOpenAIImage:
		request, err = GetAndValidOpenAIImageRequest(c, relayMode)
	case types.RelayFormatEmbedding:
		request, err = GetAndValidateEmbeddingRequest(c, relayMode)
	case types.RelayFormatRerank:
		request, err = GetAndValidateRerankRequest(c)
	case types.RelayFormatReader:
		request, err = GetAndValidateReaderRequest(c)
	case types.RelayFormatSystemOne:
		request, err = GetAndValidateSystemOneRequest(c)
	case types.RelayFormatOpenAIAudio:
		request, err = GetAndValidAudioRequest(c, relayMode)
	case types.RelayFormatOpenAIRealtime:
		request = &dto.BaseRequest{}
	default:
		return nil, fmt.Errorf("unsupported relay format: %s", format)
	}
	return request, err
}

func GetAndValidAudioRequest(c *gin.Context, relayMode int) (*dto.AudioRequest, error) {
	audioRequest := &dto.AudioRequest{}
	err := common.UnmarshalBodyReusable(c, audioRequest)
	if err != nil {
		return nil, err
	}
	switch relayMode {
	case relayconstant.RelayModeAudioSpeech:
		if audioRequest.Model == "" {
			return nil, errors.New("model is required")
		}
		if dto.IsVolcengineTTS3Model(audioRequest.Model) {
			if err := validateVolcengineTTS3AudioRequest(audioRequest); err != nil {
				return nil, err
			}
		}
	default:
		if audioRequest.Model == "" {
			return nil, errors.New("model is required")
		}
		if audioRequest.ResponseFormat == "" {
			audioRequest.ResponseFormat = "json"
		}
	}
	return audioRequest, nil
}

// fork: validateVolcengineTTS3AudioRequest enforces Doubao TTS 3.0 request
// bounds before billing estimation: text/ssml mutual exclusion, non-empty
// payload, 10,000 billable-character ceiling, path-safe voice_id, and scalar
// parameter ranges.
func validateVolcengineTTS3AudioRequest(req *dto.AudioRequest) error {
	var meta struct {
		Text            string `json:"text"`
		SSML            string `json:"ssml"`
		SilenceDuration *int   `json:"silence_duration"`
		VoiceSettings   *struct {
			Speed  *float64 `json:"speed"`
			Volume *float64 `json:"volume"`
			Pitch  *int     `json:"pitch"`
		} `json:"voice_settings"`
	}
	if len(req.Metadata) > 0 && strings.TrimSpace(string(req.Metadata)) != "null" {
		if err := common.Unmarshal(req.Metadata, &meta); err != nil {
			return fmt.Errorf("invalid metadata: %w", err)
		}
	}

	if strings.TrimSpace(req.Input) == "" && strings.TrimSpace(meta.Text) != "" {
		req.Input = meta.Text
	}
	if strings.TrimSpace(req.SSML) == "" && strings.TrimSpace(meta.SSML) != "" {
		req.SSML = meta.SSML
	}

	hasText := strings.TrimSpace(req.Input) != ""
	hasSSML := strings.TrimSpace(req.SSML) != ""
	if hasText && hasSSML {
		return errors.New("input and ssml are mutually exclusive")
	}
	if !hasText && !hasSSML {
		return errors.New("input or ssml is required")
	}
	// Clear inactive whitespace-only fields so pre-consume token counting and
	// upstream serialization always observe the single active payload.
	if !hasText {
		req.Input = ""
	}
	if !hasSSML {
		req.SSML = ""
	}

	activeText := req.Input
	if hasSSML {
		activeText = req.SSML
	}
	if chars := dto.CountVolcengineTTS3BillableChars(activeText); chars > dto.VolcengineTTS3MaxBillableChars {
		return fmt.Errorf("billable character count %d exceeds maximum %d", chars, dto.VolcengineTTS3MaxBillableChars)
	}

	voice := strings.TrimSpace(req.Voice)
	if voice != "" {
		if voice == "." || strings.Contains(voice, "..") {
			return errors.New("invalid voice id")
		}
		for _, r := range voice {
			if r <= 0x20 || r == 0x7f || r == '/' || r == '\\' || r == '?' || r == '#' || r == '%' {
				return errors.New("invalid voice id")
			}
		}
	}

	// Validate top-level and metadata scalar bounds independently so neither can bypass validation.
	if req.Speed != nil && (math.IsNaN(*req.Speed) || math.IsInf(*req.Speed, 0) || *req.Speed < 0.5 || *req.Speed > 2.0) {
		return errors.New("speed must be between 0.5 and 2.0")
	}
	if req.Volume != nil && (math.IsNaN(*req.Volume) || math.IsInf(*req.Volume, 0) || *req.Volume < 0.5 || *req.Volume > 2.0) {
		return errors.New("volume must be between 0.5 and 2.0")
	}
	if req.Pitch != nil && (*req.Pitch < -12 || *req.Pitch > 12) {
		return errors.New("pitch must be between -12 and 12")
	}
	if req.SilenceDuration != nil && (*req.SilenceDuration < 0 || *req.SilenceDuration > 30000) {
		return errors.New("silence_duration must be between 0 and 30000")
	}
	if meta.SilenceDuration != nil && (*meta.SilenceDuration < 0 || *meta.SilenceDuration > 30000) {
		return errors.New("silence_duration must be between 0 and 30000")
	}
	if meta.VoiceSettings != nil {
		if meta.VoiceSettings.Speed != nil && (math.IsNaN(*meta.VoiceSettings.Speed) || math.IsInf(*meta.VoiceSettings.Speed, 0) || *meta.VoiceSettings.Speed < 0.5 || *meta.VoiceSettings.Speed > 2.0) {
			return errors.New("voice_settings.speed must be between 0.5 and 2.0")
		}
		if meta.VoiceSettings.Volume != nil && (math.IsNaN(*meta.VoiceSettings.Volume) || math.IsInf(*meta.VoiceSettings.Volume, 0) || *meta.VoiceSettings.Volume < 0.5 || *meta.VoiceSettings.Volume > 2.0) {
			return errors.New("voice_settings.volume must be between 0.5 and 2.0")
		}
		if meta.VoiceSettings.Pitch != nil && (*meta.VoiceSettings.Pitch < -12 || *meta.VoiceSettings.Pitch > 12) {
			return errors.New("voice_settings.pitch must be between -12 and 12")
		}
	}

	if trimmedVS := strings.TrimSpace(string(req.VoiceSettings)); trimmedVS != "" && trimmedVS != "null" {
		var vs struct {
			Speed  *float64 `json:"speed"`
			Volume *float64 `json:"volume"`
			Pitch  *int     `json:"pitch"`
		}
		if err := common.Unmarshal(req.VoiceSettings, &vs); err != nil {
			return fmt.Errorf("invalid voice_settings: %w", err)
		}
		if vs.Speed != nil && (math.IsNaN(*vs.Speed) || math.IsInf(*vs.Speed, 0) || *vs.Speed < 0.5 || *vs.Speed > 2.0) {
			return errors.New("voice_settings.speed must be between 0.5 and 2.0")
		}
		if vs.Volume != nil && (math.IsNaN(*vs.Volume) || math.IsInf(*vs.Volume, 0) || *vs.Volume < 0.5 || *vs.Volume > 2.0) {
			return errors.New("voice_settings.volume must be between 0.5 and 2.0")
		}
		if vs.Pitch != nil && (*vs.Pitch < -12 || *vs.Pitch > 12) {
			return errors.New("voice_settings.pitch must be between -12 and 12")
		}
	}

	return nil
}

func GetAndValidateRerankRequest(c *gin.Context) (*dto.RerankRequest, error) {
	var rerankRequest *dto.RerankRequest
	err := common.UnmarshalBodyReusable(c, &rerankRequest)
	if err != nil {
		logger.LogError(c, common.LogText("getAndValidateTextRequest failed: %s", err.Error()))
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	if rerankRequest.Query == "" {
		return nil, types.NewError(fmt.Errorf("query is empty"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if len(rerankRequest.Documents) == 0 {
		return nil, types.NewError(fmt.Errorf("documents is empty"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	return rerankRequest, nil
}

func GetAndValidateReaderRequest(c *gin.Context) (*dto.ReaderRequest, error) {
	var readerRequest *dto.ReaderRequest
	err := common.UnmarshalBodyReusable(c, &readerRequest)
	if err != nil {
		logger.LogError(c, fmt.Sprintf("getAndValidateReaderRequest failed: %s", err.Error()))
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	if readerRequest.Model == "" {
		return nil, types.NewError(fmt.Errorf("model is required"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if readerRequest.Url == "" {
		return nil, types.NewError(fmt.Errorf("url is required"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if !strings.HasPrefix(readerRequest.Url, "http://") && !strings.HasPrefix(readerRequest.Url, "https://") {
		return nil, types.NewError(fmt.Errorf("url must start with http:// or https://"), types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	return readerRequest, nil
}

func GetAndValidateSystemOneRequest(c *gin.Context) (*dto.SystemOneRequest, error) {
	var request *dto.SystemOneRequest
	if err := common.UnmarshalBodyReusable(c, &request); err != nil {
		logger.LogError(c, fmt.Sprintf("getAndValidateSystemOneRequest failed: %s", err.Error()))
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}
	if request == nil {
		request = &dto.SystemOneRequest{}
	}
	if strings.TrimSpace(request.Model) == "" {
		request.Model = "jev"
	}
	return request, nil
}

func GetAndValidateEmbeddingRequest(c *gin.Context, relayMode int) (*dto.EmbeddingRequest, error) {
	var embeddingRequest *dto.EmbeddingRequest
	err := common.UnmarshalBodyReusable(c, &embeddingRequest)
	if err != nil {
		logger.LogError(c, common.LogText("getAndValidateTextRequest failed: %s", err.Error()))
		return nil, types.NewError(err, types.ErrorCodeInvalidRequest, types.ErrOptionWithSkipRetry())
	}

	if embeddingRequest.Input == nil {
		return nil, fmt.Errorf("input is empty")
	}
	if relayMode == relayconstant.RelayModeModerations && embeddingRequest.Model == "" {
		embeddingRequest.Model = "omni-moderation-latest"
	}
	if relayMode == relayconstant.RelayModeEmbeddings && embeddingRequest.Model == "" {
		embeddingRequest.Model = c.Param("model")
	}
	return embeddingRequest, nil
}

// maxTokensLimit bounds user-supplied max token fields. These values feed
// pre-consume quota math (preConsumedTokens * ratio); an unbounded value can
// overflow the conversion and corrupt billing.
const maxTokensLimit = math.MaxInt32 / 2

// ExceedsMaxTokensLimit checks token limits before they reach billing arithmetic.
func ExceedsMaxTokensLimit(values ...*uint) bool {
	for _, v := range values {
		if lo.FromPtrOr(v, uint(0)) > maxTokensLimit {
			return true
		}
	}
	return false
}

func GetAndValidateResponsesRequest(c *gin.Context) (*dto.OpenAIResponsesRequest, error) {
	request := &dto.OpenAIResponsesRequest{}
	err := common.UnmarshalBodyReusable(c, request)
	if err != nil {
		return nil, err
	}
	if request.Model == "" {
		return nil, errors.New("model is required")
	}
	if request.Input == nil {
		return nil, errors.New("input is required")
	}
	if ExceedsMaxTokensLimit(request.MaxOutputTokens) {
		return nil, errors.New("max_output_tokens is invalid")
	}
	return request, nil
}

func GetAndValidateAlphaSearchRequest(c *gin.Context) (*dto.AlphaSearchRequest, error) {
	request := &dto.AlphaSearchRequest{}
	if err := common.UnmarshalBodyReusable(c, request); err != nil {
		return nil, err
	}
	if request.Model == "" {
		return nil, errors.New("model is required")
	}
	storage, err := common.GetBodyStorage(c)
	if err != nil {
		return nil, err
	}
	rawBody, err := storage.Bytes()
	if err != nil {
		return nil, err
	}
	request.RawBody = rawBody
	return request, nil
}

func GetAndValidateResponsesCompactionRequest(c *gin.Context) (*dto.OpenAIResponsesCompactionRequest, error) {
	request := &dto.OpenAIResponsesCompactionRequest{}
	if err := common.UnmarshalBodyReusable(c, request); err != nil {
		return nil, err
	}
	if request.Model == "" {
		return nil, errors.New("model is required")
	}
	return request, nil
}

func GetAndValidOpenAIImageRequest(c *gin.Context, relayMode int) (*dto.ImageRequest, error) {
	imageRequest := &dto.ImageRequest{}

	switch relayMode {
	case relayconstant.RelayModeImagesEdits:
		if strings.Contains(c.Request.Header.Get("Content-Type"), "multipart/form-data") {
			form, err := common.ParseMultipartFormReusable(c)
			if err != nil {
				return nil, fmt.Errorf("failed to parse image edit form request: %w", err)
			}
			formData := url.Values(form.Value)
			c.Request.MultipartForm = form
			c.Request.PostForm = formData
			imageRequest.Prompt = formData.Get("prompt")
			imageRequest.Model = formData.Get("model")
			if imageRequest.Model == service.ModelVolcImageUpscale {
				return nil, errors.New("multipart/form-data is not supported for volc-image-upscale")
			}
			if nValue := strings.TrimSpace(formData.Get("n")); nValue != "" {
				n, err := strconv.Atoi(nValue)
				if err != nil || n < 0 || n > dto.MaxImageN {
					return nil, fmt.Errorf("n must be an integer between 1 and %d", dto.MaxImageN)
				}
				imageRequest.N = common.GetPointer(uint(n))
			}
			imageRequest.Quality = formData.Get("quality")
			imageRequest.Size = formData.Get("size")
			if parameters := formData.Get("parameters"); parameters != "" {
				imageRequest.Extra = map[string]json.RawMessage{"parameters": json.RawMessage(parameters)}
			}
			if streamValue := strings.TrimSpace(formData.Get("stream")); streamValue != "" {
				stream, err := strconv.ParseBool(streamValue)
				if err != nil {
					return nil, fmt.Errorf("invalid stream value: %w", err)
				}
				imageRequest.Stream = common.GetPointer(stream)
			}
			if imageValue := formData.Get("image"); imageValue != "" {
				imageRequest.Image, _ = common.Marshal(imageValue)
			}

			if imageRequest.Model == "gpt-image-1" {
				if imageRequest.Quality == "" {
					imageRequest.Quality = "standard"
				}
			}
			if imageRequest.N == nil || *imageRequest.N == 0 {
				imageRequest.N = common.GetPointer(uint(1))
			}

			hasWatermark := formData.Has("watermark")
			if hasWatermark {
				watermark := formData.Get("watermark") == "true"
				imageRequest.Watermark = &watermark
			}
			break
		}
		fallthrough
	default:
		err := common.UnmarshalBodyReusable(c, imageRequest)
		if err != nil {
			return nil, err
		}

		if imageRequest.Model == "" {
			return nil, errors.New("model is required")
		}

		if imageRequest.Model == service.ModelVolcImageUpscale {
			if relayMode != relayconstant.RelayModeImagesEdits {
				return nil, errors.New("volc-image-upscale is only supported on /v1/images/edits")
			}
			if imageRequest.N != nil && *imageRequest.N != 1 {
				return nil, errors.New("n must be 1 for volc-image-upscale")
			}
			imageRequest.N = common.GetPointer(uint(1))

			if len(imageRequest.Image) == 0 {
				return nil, errors.New("image field is required for volc-image-upscale")
			}
			var imageURL string
			if err := json.Unmarshal(imageRequest.Image, &imageURL); err != nil {
				return nil, errors.New("image field must be a single string URL for volc-image-upscale")
			}
			imageURL = strings.TrimSpace(imageURL)
			if imageURL == "" {
				return nil, errors.New("image field is required and cannot be empty for volc-image-upscale")
			}

			// Validate internal parameters cannot be overridden
			if len(imageRequest.ExtraFields) > 0 {
				var extraMap map[string]any
				if err := json.Unmarshal(imageRequest.ExtraFields, &extraMap); err == nil {
					if _, exists := extraMap["req_key"]; exists {
						return nil, errors.New("req_key override is not allowed")
					}
				}
			}
			if rawParams, exists := imageRequest.Extra["parameters"]; exists {
				var paramMap map[string]any
				if err := json.Unmarshal(rawParams, &paramMap); err == nil {
					if _, exists := paramMap["req_key"]; exists {
						return nil, errors.New("req_key override is not allowed")
					}
				}
			}

			var testClient *http.Client
			if val, ok := c.Get("test_http_client"); ok {
				testClient, _ = val.(*http.Client)
			}
			if _, err := service.ValidateVolcImageUpscaleURL(c.Request.Context(), imageURL, testClient); err != nil {
				return nil, fmt.Errorf("invalid image URL or image specification: %w", err)
			}
		}

		if strings.Contains(imageRequest.Size, "×") {
			return nil, errors.New("size an unexpected error occurred in the parameter, please use 'x' instead of the multiplication sign '×'")
		}

		if imageRequest.N != nil && *imageRequest.N > dto.MaxImageN {
			return nil, fmt.Errorf("n must be an integer between 1 and %d", dto.MaxImageN)
		}

		if err := imageRequest.NormalizeLegacyDalleImageRequest(); err != nil {
			return nil, err
		}
		if imageRequest.Model == "gpt-image-1" {
			if imageRequest.Quality == "" {
				imageRequest.Quality = "auto"
			}
		}

		//if imageRequest.Prompt == "" {
		//	return nil, errors.New("prompt is required")
		//}

		if imageRequest.N == nil || *imageRequest.N == 0 {
			imageRequest.N = common.GetPointer(uint(1))
		}
	}

	// Provider parameters can override the top-level count. Validate before
	// pricing so malformed multipliers return a client error, not a pricing
	// failure after reservation has started.
	if raw, exists := imageRequest.Extra["parameters"]; exists {
		parameters := &dto.ImageBillingParameters{}
		if err := common.Unmarshal(raw, parameters); err != nil {
			return nil, fmt.Errorf("invalid image parameters: %w", err)
		}
		imageRequest.BillingParameters = parameters
	}
	if _, err := imageRequest.ImageCount(false); err != nil {
		return nil, err
	}
	return imageRequest, nil
}

func GetAndValidateClaudeRequest(c *gin.Context) (textRequest *dto.ClaudeRequest, err error) {
	textRequest = &dto.ClaudeRequest{}
	err = common.UnmarshalBodyReusable(c, textRequest)
	if err != nil {
		return nil, err
	}
	if textRequest.Messages == nil || len(textRequest.Messages) == 0 {
		return nil, errors.New("field messages is required")
	}
	if textRequest.Model == "" {
		return nil, errors.New("field model is required")
	}
	if ExceedsMaxTokensLimit(textRequest.MaxTokens, textRequest.MaxTokensToSample) {
		return nil, errors.New("max_tokens is invalid")
	}

	//if textRequest.Stream {
	//	relayInfo.IsStream = true
	//}

	return textRequest, nil
}

func GetAndValidateTextRequest(c *gin.Context, relayMode int) (*dto.GeneralOpenAIRequest, error) {
	textRequest := &dto.GeneralOpenAIRequest{}
	err := common.UnmarshalBodyReusable(c, textRequest)
	if err != nil {
		return nil, err
	}

	if relayMode == relayconstant.RelayModeModerations && textRequest.Model == "" {
		textRequest.Model = "text-moderation-latest"
	}
	if relayMode == relayconstant.RelayModeEmbeddings && textRequest.Model == "" {
		textRequest.Model = c.Param("model")
	}

	if ExceedsMaxTokensLimit(textRequest.MaxTokens, textRequest.MaxCompletionTokens) {
		return nil, errors.New("max_tokens is invalid")
	}
	if ExceedsMaxTokensLimit(textRequest.MinTokens) {
		return nil, errors.New("min_tokens is invalid")
	}
	if textRequest.Model == "" {
		return nil, errors.New("model is required")
	}
	if textRequest.WebSearchOptions != nil {
		if textRequest.WebSearchOptions.SearchContextSize != "" {
			validSizes := map[string]bool{
				"high":   true,
				"medium": true,
				"low":    true,
			}
			if !validSizes[textRequest.WebSearchOptions.SearchContextSize] {
				return nil, errors.New("invalid search_context_size, must be one of: high, medium, low")
			}
		} else {
			textRequest.WebSearchOptions.SearchContextSize = "medium"
		}
	}
	switch relayMode {
	case relayconstant.RelayModeCompletions:
		if textRequest.Prompt == "" {
			return nil, errors.New("field prompt is required")
		}
	case relayconstant.RelayModeChatCompletions:
		// For FIM (Fill-in-the-middle) requests with prefix/suffix, messages is optional
		// It will be filled by provider-specific adaptors if needed (e.g., SiliconFlow)。Or it is allowed by model vendor(s) (e.g., DeepSeek)
		if len(textRequest.Messages) == 0 && textRequest.Prefix == nil && textRequest.Suffix == nil {
			return nil, errors.New("field messages is required")
		}
	case relayconstant.RelayModeEmbeddings:
	case relayconstant.RelayModeModerations:
		if textRequest.Input == nil || textRequest.Input == "" {
			return nil, errors.New("field input is required")
		}
	case relayconstant.RelayModeEdits:
		if textRequest.Instruction == "" {
			return nil, errors.New("field instruction is required")
		}
	}
	return textRequest, nil
}

func GetAndValidateGeminiRequest(c *gin.Context) (*dto.GeminiChatRequest, error) {
	request := &dto.GeminiChatRequest{}
	err := common.UnmarshalBodyReusable(c, request)
	if err != nil {
		return nil, err
	}
	if len(request.Contents) == 0 && len(request.Requests) == 0 {
		return nil, errors.New("contents is required")
	}
	if ExceedsMaxTokensLimit(request.GenerationConfig.MaxOutputTokens) {
		return nil, errors.New("maxOutputTokens is invalid")
	}

	//if c.Query("alt") == "sse" {
	//	relayInfo.IsStream = true
	//}

	return request, nil
}

func GetAndValidateGeminiEmbeddingRequest(c *gin.Context) (*dto.GeminiEmbeddingRequest, error) {
	request := &dto.GeminiEmbeddingRequest{}
	err := common.UnmarshalBodyReusable(c, request)
	if err != nil {
		return nil, err
	}
	return request, nil
}

func GetAndValidateGeminiBatchEmbeddingRequest(c *gin.Context) (*dto.GeminiBatchEmbeddingRequest, error) {
	request := &dto.GeminiBatchEmbeddingRequest{}
	err := common.UnmarshalBodyReusable(c, request)
	if err != nil {
		return nil, err
	}
	return request, nil
}
