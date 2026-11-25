package volcengine

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/QuantumNous/new-api/common"
	channelconstant "github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

const (
	defaultTTS3VoiceID       = "multi_female_vv_xi_v3_uranus_bigtts"
	tts3FinalReasonClosed    = "TTS_STREAM_FINAL_REASON_CLOSED"
	maxTTS3DecodedAudioBytes = 64 << 20 // 64 MiB cap for buffered modes
)

// openAIToTTS3VoiceMap maps standard OpenAI voice aliases to verified Doubao TTS 3.0 preset voices.
var openAIToTTS3VoiceMap = map[string]string{
	"alloy":   "multi_female_vv_xi_v3_uranus_bigtts",
	"echo":    "multi_male_taocheng_v3_uranus_bigtts",
	"fable":   "multi_female_wenrouxiaoya_v3_uranus_bigtts",
	"onyx":    "multi_male_m191_v3_uranus_bigtts",
	"nova":    "multi_female_zhixingnv_v3_uranus_bigtts",
	"shimmer": "multi_female_tuanzi_v3_uranus_bigtts",
}

func resolveTTS3VoiceID(rawVoice string) (string, error) {
	voice := strings.TrimSpace(rawVoice)
	if voice == "" {
		return defaultTTS3VoiceID, nil
	}
	if strings.Contains(voice, "..") {
		return "", errors.New("invalid voice id")
	}
	for _, r := range voice {
		if r <= 0x20 || r == 0x7f || r == '/' || r == '\\' || r == '?' || r == '#' {
			return "", errors.New("invalid voice id")
		}
	}
	if mapped, ok := openAIToTTS3VoiceMap[strings.ToLower(voice)]; ok {
		return mapped, nil
	}
	return voice, nil
}

func resolveTTS3BaseURL(rawBaseURL string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(rawBaseURL), "/")
	defaultArk := strings.TrimRight(channelconstant.GetChannelBaseURL(channelconstant.ChannelTypeVolcEngine), "/")
	if trimmed == "" || trimmed == defaultArk || trimmed == VolcengineLegacySpeechBaseURL {
		return VolcengineTTS3BaseURL
	}
	return trimmed
}

func resolveTTS3ModelID(info *relaycommon.RelayInfo) string {
	upstream := strings.TrimSpace(getUpstreamModelName(info))
	if upstream == "" || strings.EqualFold(upstream, ModelSeedTTS30) {
		return UpstreamModelDoubaoSeedTTS30
	}
	return upstream
}

func mapTTS3Format(responseFormat string) (string, int) {
	switch strings.ToLower(strings.TrimSpace(responseFormat)) {
	case "pcm":
		return "pcm", 24000
	case "opus", "ogg", "ogg_opus":
		return "ogg_opus", 48000
	default:
		return "mp3", 24000
	}
}

func tts3ContentType(format string) string {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "mp3":
		return "audio/mpeg"
	case "pcm":
		return "audio/pcm"
	case "ogg_opus":
		return "audio/ogg"
	default:
		return "application/octet-stream"
	}
}

func tts3ErrorHTTPStatus(errType string) int {
	switch strings.ToLower(strings.TrimSpace(errType)) {
	case "validation_error", "invalid_request":
		return http.StatusBadRequest
	case "authentication_error":
		return http.StatusUnauthorized
	case "payment_required":
		return http.StatusPaymentRequired
	case "authorization_error":
		return http.StatusForbidden
	case "not_found":
		return http.StatusNotFound
	case "rate_limit_error":
		return http.StatusTooManyRequests
	case "internal_error":
		return http.StatusInternalServerError
	default:
		return http.StatusBadGateway
	}
}

func buildTTS3Request(info *relaycommon.RelayInfo, request dto.AudioRequest) (*TTS3Request, string, string, int, error) {
	voiceID, err := resolveTTS3VoiceID(request.Voice)
	if err != nil {
		return nil, "", "", 0, err
	}

	req := &TTS3Request{
		ModelID: resolveTTS3ModelID(info),
	}

	// 1. Unmarshal optional Metadata first; voice_id is path-only and cannot be overridden here.
	if len(request.Metadata) > 0 {
		if err := common.Unmarshal(request.Metadata, req); err != nil {
			return nil, "", "", 0, fmt.Errorf("error unmarshalling metadata to volcengine tts 3.0 request: %w", err)
		}
		if req.ModelID == "" || strings.EqualFold(req.ModelID, ModelSeedTTS30) {
			req.ModelID = resolveTTS3ModelID(info)
		}
	}

	// 2. Apply top-level Input / SSML (mutually exclusive).
	if request.Input != "" {
		req.Text = request.Input
	}
	if request.SSML != "" {
		req.SSML = request.SSML
	}
	hasText := strings.TrimSpace(req.Text) != ""
	hasSSML := strings.TrimSpace(req.SSML) != ""
	if hasText && hasSSML {
		return nil, "", "", 0, errors.New("input and ssml are mutually exclusive")
	}
	if !hasText && !hasSSML {
		return nil, "", "", 0, errors.New("input or ssml is required")
	}

	activeText := req.Text
	if hasSSML {
		activeText = req.SSML
	}
	billableChars := dto.CountVolcengineTTS3BillableChars(activeText)
	if billableChars > dto.VolcengineTTS3MaxBillableChars {
		return nil, "", "", 0, fmt.Errorf("billable character count %d exceeds maximum %d", billableChars, dto.VolcengineTTS3MaxBillableChars)
	}

	// 3. Apply explicit top-level extension fields over metadata defaults.
	if request.LanguageCode != "" {
		req.LanguageCode = strings.TrimSpace(request.LanguageCode)
	}
	if request.SessionID != "" {
		req.SessionID = strings.TrimSpace(request.SessionID)
	}
	if request.EnableSubtitle != nil {
		req.EnableSubtitle = request.EnableSubtitle
	}
	if request.TextNormalization != nil {
		req.TextNormalization = request.TextNormalization
	}
	if request.EnableMarkdownFilter != nil {
		req.EnableMarkdownFilter = request.EnableMarkdownFilter
	}
	if request.SilenceDuration != nil {
		req.SilenceDuration = request.SilenceDuration
	}

	if len(request.OutputFormat) > 0 {
		var outFmt TTS3OutputFormat
		if err := common.Unmarshal(request.OutputFormat, &outFmt); err != nil {
			return nil, "", "", 0, fmt.Errorf("invalid output_format: %w", err)
		}
		if req.OutputFormat == nil {
			req.OutputFormat = &TTS3OutputFormat{}
		}
		if outFmt.Format != "" {
			req.OutputFormat.Format = outFmt.Format
		}
		if outFmt.SampleRate != nil {
			req.OutputFormat.SampleRate = outFmt.SampleRate
		}
		if outFmt.BitRate != nil {
			req.OutputFormat.BitRate = outFmt.BitRate
		}
	}

	if len(request.VoiceSettings) > 0 {
		var vs TTS3VoiceSettings
		if err := common.Unmarshal(request.VoiceSettings, &vs); err != nil {
			return nil, "", "", 0, fmt.Errorf("invalid voice_settings: %w", err)
		}
		if req.VoiceSettings == nil {
			req.VoiceSettings = &TTS3VoiceSettings{}
		}
		if vs.Speed != nil {
			req.VoiceSettings.Speed = vs.Speed
		}
		if vs.Volume != nil {
			req.VoiceSettings.Volume = vs.Volume
		}
		if vs.Pitch != nil {
			req.VoiceSettings.Pitch = vs.Pitch
		}
		if len(vs.Instructions) > 0 {
			req.VoiceSettings.Instructions = vs.Instructions
		}
		if vs.SimilarityBoost != nil {
			req.VoiceSettings.SimilarityBoost = vs.SimilarityBoost
		}
		if vs.DialectBoost != nil {
			req.VoiceSettings.DialectBoost = vs.DialectBoost
		}
	}

	if len(request.PronunciationDictionary) > 0 {
		var dict TTS3PronunciationDictionary
		if err := common.Unmarshal(request.PronunciationDictionary, &dict); err != nil {
			return nil, "", "", 0, fmt.Errorf("invalid pronunciation_dictionary: %w", err)
		}
		req.PronunciationDictionary = &dict
	}

	if len(request.Watermark) > 0 {
		var wm TTS3Watermark
		if err := common.Unmarshal(request.Watermark, &wm); err != nil {
			return nil, "", "", 0, fmt.Errorf("invalid watermark: %w", err)
		}
		req.Watermark = &wm
	}

	// 4. Map OpenAI standard Speed & Instructions into voice_settings when not already set.
	if request.Speed != nil {
		if req.VoiceSettings == nil {
			req.VoiceSettings = &TTS3VoiceSettings{}
		}
		if req.VoiceSettings.Speed == nil {
			req.VoiceSettings.Speed = request.Speed
		}
	}
	if strings.TrimSpace(request.Instructions) != "" {
		if req.VoiceSettings == nil {
			req.VoiceSettings = &TTS3VoiceSettings{}
		}
		if len(req.VoiceSettings.Instructions) == 0 {
			req.VoiceSettings.Instructions = []string{strings.TrimSpace(request.Instructions)}
		}
	}

	// 5. Normalize output_format defaults per Doubao TTS 3.0 specification.
	if req.OutputFormat == nil {
		req.OutputFormat = &TTS3OutputFormat{}
	}
	mappedFormat, defaultRate := mapTTS3Format(request.ResponseFormat)
	if req.OutputFormat.Format == "" {
		req.OutputFormat.Format = mappedFormat
	} else {
		req.OutputFormat.Format, defaultRate = mapTTS3Format(req.OutputFormat.Format)
	}
	if req.OutputFormat.SampleRate == nil {
		rate := defaultRate
		req.OutputFormat.SampleRate = &rate
	}
	if req.OutputFormat.Format == "ogg_opus" {
		rate := 48000
		req.OutputFormat.SampleRate = &rate
	}
	if req.OutputFormat.Format == "pcm" {
		req.OutputFormat.BitRate = nil
	}

	return req, voiceID, req.OutputFormat.Format, billableChars, nil
}

func buildTTS3UpstreamURL(baseURL string, voiceID string) string {
	return fmt.Sprintf("%s"+VolcengineTTS3PathTemplate, resolveTTS3BaseURL(baseURL), url.PathEscape(voiceID))
}

func formatTTS3Error(ev *TTS3ErrorPayload) *types.NewAPIError {
	if ev == nil {
		return types.NewErrorWithStatusCode(
			errors.New("volcengine tts 3.0 upstream error"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}
	msg := strings.TrimSpace(ev.Message)
	if ev.Code != "" {
		if msg != "" {
			msg = fmt.Sprintf("%s: %s", ev.Code, msg)
		} else {
			msg = ev.Code
		}
	}
	if msg == "" {
		msg = "volcengine tts 3.0 upstream error"
	}
	return types.NewErrorWithStatusCode(
		errors.New(msg),
		types.ErrorCodeBadResponse,
		tts3ErrorHTTPStatus(ev.Type),
	)
}

func handleTTS3Response(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, request dto.AudioRequest, outputFormat string, billableChars int) (any, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewErrorWithStatusCode(
			errors.New("volcengine tts 3.0 empty response"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}
	defer func() { _ = resp.Body.Close() }()

	if billableChars <= 0 && info != nil {
		billableChars = info.GetEstimatePromptTokens()
	}

	accept := ""
	if c != nil {
		accept = strings.ToLower(c.GetHeader("Accept"))
	}
	isNDJSONMode := strings.Contains(accept, "application/x-ndjson")
	isJSONMode := !isNDJSONMode && (strings.Contains(accept, "application/json") ||
		strings.EqualFold(request.ResponseFormat, "json") ||
		strings.EqualFold(request.ResponseFormat, "verbose_json"))

	if isNDJSONMode {
		return handleTTS3NDJSONStream(c, resp.Body, billableChars)
	}

	reader := bufio.NewReaderSize(resp.Body, 64*1024)
	var audioBuf bytes.Buffer
	var alignments []TTS3AlignmentSegment
	var sessionID string
	closedCleanly := false

	for {
		line, readErr := reader.ReadBytes('\n')
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			var ev TTS3StreamEvent
			if err := common.Unmarshal(trimmed, &ev); err != nil {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("failed to parse volcengine tts 3.0 stream event: %w", err),
					types.ErrorCodeBadResponseBody,
					http.StatusBadGateway,
				)
			}

			switch ev.Event {
			case "ready":
				if ev.SessionID != "" {
					sessionID = ev.SessionID
				}
			case "audio":
				if ev.Audio != "" {
					decoded, decErr := base64.StdEncoding.DecodeString(ev.Audio)
					if decErr != nil {
						return nil, types.NewErrorWithStatusCode(
							fmt.Errorf("failed to decode volcengine tts 3.0 audio chunk: %w", decErr),
							types.ErrorCodeBadResponseBody,
							http.StatusBadGateway,
						)
					}
					if audioBuf.Len()+len(decoded) > maxTTS3DecodedAudioBytes {
						return nil, types.NewErrorWithStatusCode(
							errors.New("volcengine tts 3.0 response exceeded maximum buffered audio size"),
							types.ErrorCodeBadResponseBody,
							http.StatusBadGateway,
						)
					}
					audioBuf.Write(decoded)
				}
			case "alignment":
				alignments = append(alignments, TTS3AlignmentSegment{
					Text:  ev.Text,
					Words: ev.Words,
				})
			case "final":
				if ev.Reason == tts3FinalReasonClosed || ev.IsFinal {
					closedCleanly = true
				}
			case "error":
				return nil, formatTTS3Error(ev.Error)
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("failed to read volcengine tts 3.0 stream: %w", readErr),
				types.ErrorCodeReadResponseBodyFailed,
				http.StatusBadGateway,
			)
		}
	}

	if !closedCleanly {
		return nil, types.NewErrorWithStatusCode(
			errors.New("volcengine tts 3.0 stream terminated before final event"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}

	if c != nil {
		if sessionID != "" {
			c.Header("X-Audio-Session-Id", sessionID)
		}
		if isJSONMode {
			payload := TTS3AggregatedResponse{
				SessionID:  sessionID,
				Format:     outputFormat,
				Audio:      base64.StdEncoding.EncodeToString(audioBuf.Bytes()),
				Alignments: alignments,
			}
			encoded, err := common.Marshal(payload)
			if err != nil {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("failed to marshal volcengine tts 3.0 json response: %w", err),
					types.ErrorCodeBadResponseBody,
					http.StatusInternalServerError,
				)
			}
			c.Data(http.StatusOK, "application/json", encoded)
		} else {
			c.Data(http.StatusOK, tts3ContentType(outputFormat), audioBuf.Bytes())
		}
	}

	usage := &dto.Usage{
		PromptTokens:     billableChars,
		CompletionTokens: 0,
		TotalTokens:      billableChars,
	}
	return usage, nil
}

func handleTTS3NDJSONStream(c *gin.Context, body io.Reader, billableChars int) (any, *types.NewAPIError) {
	reader := bufio.NewReaderSize(body, 64*1024)
	headersWritten := false
	closedCleanly := false
	wroteAudio := false

	for {
		line, readErr := reader.ReadBytes('\n')
		trimmed := bytes.TrimSpace(line)
		if len(trimmed) > 0 {
			var ev TTS3StreamEvent
			if err := common.Unmarshal(trimmed, &ev); err != nil {
				if !headersWritten {
					return nil, types.NewErrorWithStatusCode(
						fmt.Errorf("failed to parse volcengine tts 3.0 stream event: %w", err),
						types.ErrorCodeBadResponseBody,
						http.StatusBadGateway,
					)
				}
				break
			}

			if ev.Event == "error" && !headersWritten {
				return nil, formatTTS3Error(ev.Error)
			}
			if ev.Event == "audio" && ev.Audio != "" {
				wroteAudio = true
			}
			if ev.Event == "final" && (ev.Reason == tts3FinalReasonClosed || ev.IsFinal) {
				closedCleanly = true
			}

			if c != nil {
				if !headersWritten {
					c.Header("Content-Type", "application/x-ndjson; charset=utf-8")
					c.Header("Transfer-Encoding", "chunked")
					c.Status(http.StatusOK)
					headersWritten = true
				}
				if _, writeErr := c.Writer.Write(append(trimmed, '\n')); writeErr != nil {
					if wroteAudio {
						return &dto.Usage{
							PromptTokens:     billableChars,
							CompletionTokens: 0,
							TotalTokens:      billableChars,
						}, nil
					}
					return nil, types.NewErrorWithStatusCode(
						fmt.Errorf("failed to stream volcengine tts 3.0 event: %w", writeErr),
						types.ErrorCodeBadResponse,
						http.StatusInternalServerError,
					)
				}
				c.Writer.Flush()
			}
		}

		if readErr != nil {
			if errors.Is(readErr, io.EOF) {
				break
			}
			if !headersWritten {
				return nil, types.NewErrorWithStatusCode(
					fmt.Errorf("failed to read volcengine tts 3.0 stream: %w", readErr),
					types.ErrorCodeReadResponseBodyFailed,
					http.StatusBadGateway,
				)
			}
			break
		}
	}

	if !closedCleanly && !wroteAudio {
		return nil, types.NewErrorWithStatusCode(
			errors.New("volcengine tts 3.0 stream terminated before final event"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}

	return &dto.Usage{
		PromptTokens:     billableChars,
		CompletionTokens: 0,
		TotalTokens:      billableChars,
	}, nil
}
