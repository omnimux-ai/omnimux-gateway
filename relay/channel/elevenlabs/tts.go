package elevenlabs

import (
	"fmt"
	"io"
	"net/http"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func handleTTSResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, textInput string) (usage any, err *types.NewAPIError) {
	defer service.CloseResponseBodyGracefully(resp)

	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("failed to read elevenlabs response body: %w", readErr),
			types.ErrorCodeReadResponseBodyFailed,
			http.StatusInternalServerError,
		)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var errResp ErrorResponse
		if unmarshalErr := common.Unmarshal(bodyBytes, &errResp); unmarshalErr == nil && errResp.Detail.Message != "" {
			return nil, types.NewErrorWithStatusCode(
				fmt.Errorf("elevenlabs tts error: %s", errResp.Detail.Message),
				types.ErrorCodeBadResponse,
				resp.StatusCode,
			)
		}
		return nil, types.NewErrorWithStatusCode(
			fmt.Errorf("elevenlabs tts returned status %d: %s", resp.StatusCode, string(bodyBytes)),
			types.ErrorCodeBadResponse,
			resp.StatusCode,
		)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "audio/mpeg"
	}

	for k, v := range resp.Header {
		if service.ShouldCopyUpstreamHeader(c, k, v) {
			c.Writer.Header().Set(k, v[0])
		}
	}

	c.Writer.Header().Set("Content-Type", contentType)
	c.Writer.WriteHeader(resp.StatusCode)
	_, _ = c.Writer.Write(bodyBytes)

	charCount := len([]rune(textInput))
	if charCount == 0 {
		charCount = info.GetEstimatePromptTokens()
	}

	usage = &dto.Usage{
		PromptTokens:     charCount,
		CompletionTokens: 0,
		TotalTokens:      charCount,
	}

	return usage, nil
}
