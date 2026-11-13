package volcengine

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
)

// handleSeedAudioV3Response 解析火山引擎 seed-audio-1.0 响应并处理双模输出与计费
func handleSeedAudioV3Response(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, request dto.AudioRequest) (any, *types.NewAPIError) {
	if resp == nil || resp.Body == nil {
		return nil, types.NewErrorWithStatusCode(
			errors.New("volcengine seed-audio empty response"),
			types.ErrorCodeBadResponse,
			http.StatusBadGateway,
		)
	}

	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to read volcengine seed-audio response body: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}

	var v3Resp SeedAudioV3Response
	if unmarshalErr := common.Unmarshal(body, &v3Resp); unmarshalErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to parse volcengine seed-audio response: %w", unmarshalErr),
			types.ErrorCodeBadResponseBody,
			http.StatusInternalServerError,
		)
	}

	// 错误处理：检查 Code
	if v3Resp.Code != 0 {
		errMsg := v3Resp.Message
		if errMsg == "" {
			errMsg = fmt.Sprintf("upstream error with code %d", v3Resp.Code)
		}
		return nil, types.NewErrorWithStatusCode(
			errors.New(errMsg),
			types.ErrorCodeBadResponse,
			http.StatusBadRequest,
		)
	}

	// 计费时长计算：提取 official original_duration，若 <= 0 则取 duration
	duration := v3Resp.OriginalDuration
	if duration <= 0 {
		duration = v3Resp.Duration
	}

	// 遵循 OpenAI TTS 换算公式计算：AudioTokens = common.QuotaRound(math.Ceil(duration) / 60.0 * 1000)
	var audioTokens int
	if duration > 0 {
		audioTokens = common.QuotaRound(math.Ceil(duration) / 60.0 * 1000)
	}

	// 双模输出分流：
	// 当 Accept: application/json 或 response_format 为 json/verbose_json 时，采用结构化 JSON 模式透传
	isJSONMode := (c != nil && c.GetHeader("Accept") == "application/json") ||
		request.ResponseFormat == "json" ||
		request.ResponseFormat == "verbose_json"

	if isJSONMode {
		if c != nil {
			c.Header("Content-Type", "application/json")
			c.Data(http.StatusOK, "application/json", body)
		}
	} else {
		// 默认模式：解码 base64 音频为二进制流写入客户端
		audioBytes, decodeErr := base64.StdEncoding.DecodeString(v3Resp.Audio)
		if decodeErr != nil {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("failed to decode seed-audio base64 data: %w", decodeErr),
				types.ErrorCodeBadResponseBody,
				http.StatusInternalServerError,
			)
		}

		encoding := mapEncoding(request.ResponseFormat)
		contentType := getContentTypeByEncoding(encoding)

		if c != nil {
			c.Header("Content-Type", contentType)
			if v3Resp.Duration > 0 {
				c.Header("X-Audio-Duration", fmt.Sprintf("%.3f", v3Resp.Duration))
			}
			if v3Resp.OriginalDuration > 0 {
				c.Header("X-Audio-Original-Duration", fmt.Sprintf("%.3f", v3Resp.OriginalDuration))
			}
			if v3Resp.URL != "" {
				c.Header("X-Audio-Url", v3Resp.URL)
			}
			c.Data(http.StatusOK, contentType, audioBytes)
		}
	}

	promptTokens := 0
	if info != nil {
		promptTokens = info.GetEstimatePromptTokens()
	}

	usage := &dto.Usage{
		PromptTokens:     promptTokens,
		CompletionTokens: audioTokens,
		TotalTokens:      promptTokens + audioTokens,
		CompletionTokenDetails: dto.OutputTokenDetails{
			AudioTokens: audioTokens,
		},
	}

	return usage, nil
}
