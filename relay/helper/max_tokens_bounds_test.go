package helper

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMaxTokensBounds guards the billing invariant that user-supplied max
// token fields are bounded on every relay format. These values feed
// pre-consume quota math (preConsumedTokens * ratio); a huge or
// wrapped-negative value (e.g. 18446744073686646784 parsed into *uint) must
// be rejected at validation instead of corrupting the pre-charge.
func TestMaxTokensBounds(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newJSONContext := func(t *testing.T, body string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/relay", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		return c
	}

	const hugeN = "18446744073686646784"

	t.Run("openai max_tokens overflow rejected", func(t *testing.T) {
		c := newJSONContext(t, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"max_tokens":`+hugeN+`}`)
		_, err := GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_tokens is invalid")
	})

	t.Run("openai max_completion_tokens overflow rejected", func(t *testing.T) {
		c := newJSONContext(t, `{"model":"gpt-4o","messages":[{"role":"user","content":"hi"}],"max_completion_tokens":`+hugeN+`}`)
		_, err := GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_tokens is invalid")
	})

	t.Run("claude max_tokens overflow rejected", func(t *testing.T) {
		c := newJSONContext(t, `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hi"}],"max_tokens":`+hugeN+`}`)
		_, err := GetAndValidateClaudeRequest(c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_tokens is invalid")
	})

	t.Run("claude normal max_tokens accepted", func(t *testing.T) {
		c := newJSONContext(t, `{"model":"claude-sonnet-4","messages":[{"role":"user","content":"hi"}],"max_tokens":8192}`)
		req, err := GetAndValidateClaudeRequest(c)
		require.NoError(t, err)
		require.EqualValues(t, 8192, *req.MaxTokens)
	})

	t.Run("gemini maxOutputTokens overflow rejected", func(t *testing.T) {
		c := newJSONContext(t, `{"contents":[{"parts":[{"text":"hi"}]}],"generationConfig":{"maxOutputTokens":`+hugeN+`}}`)
		_, err := GetAndValidateGeminiRequest(c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "maxOutputTokens is invalid")
	})

	t.Run("responses max_output_tokens overflow rejected", func(t *testing.T) {
		c := newJSONContext(t, `{"model":"gpt-4o","input":"hi","max_output_tokens":`+hugeN+`}`)
		_, err := GetAndValidateResponsesRequest(c)
		require.Error(t, err)
		require.Contains(t, err.Error(), "max_output_tokens is invalid")
	})
}

func TestSGLangMinTokensBounds(t *testing.T) {
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{"0", true}, {"8192", true}, {"-1", false}, {"18446744073686646784", false},
	} {
		t.Run(tc.value, func(t *testing.T) {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewBufferString(`{"model":"served-model","messages":[{"role":"user","content":"hi"}],"min_tokens":`+tc.value+`}`))
			c.Request.Header.Set("Content-Type", "application/json")
			request, err := GetAndValidateTextRequest(c, relayconstant.RelayModeChatCompletions)
			if !tc.valid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, request.MinTokens)
		})
	}
}

func TestVolcengineTTS3AudioValidationAndTokenCount(t *testing.T) {
	gin.SetMode(gin.TestMode)

	newCtx := func(body string) *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", bytes.NewBufferString(body))
		c.Request.Header.Set("Content-Type", "application/json")
		return c
	}

	t.Run("rejects >10000 billable chars (Han counts as 2)", func(t *testing.T) {
		body := `{"model":"seed-tts-3.0","input":"` + strings.Repeat("汉", 5001) + `"}`
		_, err := GetAndValidAudioRequest(newCtx(body), relayconstant.RelayModeAudioSpeech)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("rejects mutually exclusive input and ssml", func(t *testing.T) {
		body := `{"model":"seed-tts-3.0","input":"hi","metadata":{"ssml":"<speak>hi</speak>"}}`
		_, err := GetAndValidAudioRequest(newCtx(body), relayconstant.RelayModeAudioSpeech)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("rejects invalid voice path and out-of-range scalar parameters", func(t *testing.T) {
		for _, body := range []string{
			`{"model":"seed-tts-3.0","input":"hi","voice":"."}`,
			`{"model":"seed-tts-3.0","input":"hi","voice":"../etc"}`,
			`{"model":"seed-tts-3.0","input":"hi","voice":"a%2Fb"}`,
			`{"model":"seed-tts-3.0","input":"hi","speed":2.5}`,
			`{"model":"seed-tts-3.0","input":"hi","volume":2.5}`,
			`{"model":"seed-tts-3.0","input":"hi","pitch":13}`,
			`{"model":"seed-tts-3.0","input":"hi","silence_duration":30001}`,
			`{"model":"seed-tts-3.0","input":"hi","speed":1.0,"metadata":{"voice_settings":{"speed":99}}}`,
			`{"model":"seed-tts-3.0","input":"hi","silence_duration":0,"metadata":{"silence_duration":99999}}`,
		} {
			_, err := GetAndValidAudioRequest(newCtx(body), relayconstant.RelayModeAudioSpeech)
			require.Error(t, err, body)
		}
	})

	t.Run("normalizes metadata ssml and counts Han=2 other=1 in CountRequestToken", func(t *testing.T) {
		c := newCtx(`{"model":"seed-tts-3.0","metadata":{"ssml":"<speak>你好</speak>"}}`)
		req, err := GetAndValidAudioRequest(c, relayconstant.RelayModeAudioSpeech)
		require.NoError(t, err)
		assert.Equal(t, "<speak>你好</speak>", req.SSML)

		common.SetContextKey(c, constant.ContextKeyOriginalModel, "seed-tts-3.0")
		info := &relaycommon.RelayInfo{
			OriginModelName: "seed-tts-3.0",
			RelayMode:       relayconstant.RelayModeAudioSpeech,
			Request:         req,
		}
		tokens, countErr := service.CountRequestToken(c, req.GetTokenCountMeta(), info)
		require.NoError(t, countErr)
		// "<speak>" (7) + "你好" (4) + "</speak>" (8) = 19
		assert.Equal(t, 19, tokens)
	})

	t.Run("whitespace input with ssml clears input and counts ssml billable chars", func(t *testing.T) {
		c := newCtx(`{"model":"seed-tts-3.0","input":"   ","ssml":"<speak>你好</speak>","metadata":null,"voice_settings":null}`)
		req, err := GetAndValidAudioRequest(c, relayconstant.RelayModeAudioSpeech)
		require.NoError(t, err)
		assert.Empty(t, req.Input)
		assert.Equal(t, "<speak>你好</speak>", req.SSML)

		common.SetContextKey(c, constant.ContextKeyOriginalModel, "seed-tts-3.0")
		info := &relaycommon.RelayInfo{
			OriginModelName: "seed-tts-3.0",
			RelayMode:       relayconstant.RelayModeAudioSpeech,
			Request:         req,
		}
		tokens, countErr := service.CountRequestToken(c, req.GetTokenCountMeta(), info)
		require.NoError(t, countErr)
		assert.Equal(t, 19, tokens)
	})
}
