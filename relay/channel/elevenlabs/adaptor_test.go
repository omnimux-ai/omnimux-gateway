package elevenlabs

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestGetRequestURL(t *testing.T) {
	t.Run("default base url and voice", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			RelayMode: constant.RelayModeAudioSpeech,
		}
		u, err := GetRequestURL(info, "", "")
		require.NoError(t, err)
		assert.Equal(t, "https://api.elevenlabs.io/v1/text-to-speech/"+DefaultVoice, u)
	})

	t.Run("custom base url and voice with format", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl: "https://custom.elevenlabs.proxy",
			},
			RelayMode: constant.RelayModeAudioSpeech,
		}
		u, err := GetRequestURL(info, "custom_voice_123", "mp3_44100_128")
		require.NoError(t, err)
		assert.Equal(t, "https://custom.elevenlabs.proxy/v1/text-to-speech/custom_voice_123?output_format=mp3_44100_128", u)
	})

	t.Run("unsupported relay mode", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			RelayMode: constant.RelayModeChatCompletions,
		}
		_, err := GetRequestURL(info, "", "")
		require.Error(t, err)
	})
}

func TestConvertAudioRequest(t *testing.T) {
	adaptor := &Adaptor{}
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	t.Run("convert valid audio request with defaults", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			RelayMode:        constant.RelayModeAudioSpeech,
			OriginModelName: "eleven_multilingual_v2",
		}
		req := dto.AudioRequest{
			Model: "eleven_multilingual_v2",
			Input: "Hello from ElevenLabs!",
		}

		reader, err := adaptor.ConvertAudioRequest(c, info, req)
		require.NoError(t, err)

		body, err := io.ReadAll(reader)
		require.NoError(t, err)

		var ttsReq TTSRequest
		err = common.Unmarshal(body, &ttsReq)
		require.NoError(t, err)

		assert.Equal(t, "Hello from ElevenLabs!", ttsReq.Text)
		assert.Equal(t, "eleven_multilingual_v2", ttsReq.ModelID)
		assert.Equal(t, DefaultVoice, adaptor.voiceID)
		assert.NotNil(t, ttsReq.VoiceSettings)
		assert.Equal(t, 0.5, ttsReq.VoiceSettings.Stability)
	})

	t.Run("convert audio request with custom voice and format", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			RelayMode:        constant.RelayModeAudioSpeech,
			OriginModelName: "eleven_v3",
		}
		req := dto.AudioRequest{
			Model:          "eleven_v3",
			Input:          "Testing voice custom",
			Voice:          "JBFqnCBsd6RMkjVDRZzb",
			ResponseFormat: "mp3_44100_128",
		}

		reader, err := adaptor.ConvertAudioRequest(c, info, req)
		require.NoError(t, err)
		assert.Equal(t, "JBFqnCBsd6RMkjVDRZzb", adaptor.voiceID)
		assert.Equal(t, "mp3_44100_128", adaptor.outputFormat)

		url, err := adaptor.GetRequestURL(info)
		require.NoError(t, err)
		assert.Contains(t, url, "JBFqnCBsd6RMkjVDRZzb")
		assert.Contains(t, url, "output_format=mp3_44100_128")

		body, _ := io.ReadAll(reader)
		var ttsReq TTSRequest
		_ = common.Unmarshal(body, &ttsReq)
		assert.Equal(t, "eleven_v3", ttsReq.ModelID)
	})

	t.Run("reject non-speech relay mode", func(t *testing.T) {
		info := &relaycommon.RelayInfo{
			RelayMode: constant.RelayModeChatCompletions,
		}
		req := dto.AudioRequest{
			Input: "Should fail",
		}
		_, err := adaptor.ConvertAudioRequest(c, info, req)
		require.Error(t, err)
	})
}

func TestSetupRequestHeader(t *testing.T) {
	adaptor := &Adaptor{}
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
	c.Request.Header.Set("Content-Type", "application/json")
	c.Request.Header.Set("Accept", "audio/mpeg")

	info := &relaycommon.RelayInfo{
		RelayMode: constant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{
			ApiKey: "test_xi_api_key_12345",
		},
	}
	header := make(http.Header)

	err := adaptor.SetupRequestHeader(c, &header, info)
	require.NoError(t, err)
	assert.Equal(t, "test_xi_api_key_12345", header.Get("xi-api-key"))
	assert.Equal(t, "application/json", header.Get("Content-Type"))
	assert.Equal(t, "audio/mpeg", header.Get("Accept"))
}

func TestHandleTTSResponse(t *testing.T) {
	t.Run("success 200 response with audio stream", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		fakeAudioData := []byte("ID3FAKEAUDIOSTREAMDATA")
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Header: http.Header{
				"Content-Type": []string{"audio/mpeg"},
			},
			Body: io.NopCloser(bytes.NewReader(fakeAudioData)),
		}

		info := &relaycommon.RelayInfo{
			RelayMode: constant.RelayModeAudioSpeech,
		}
		inputText := "你好，世界！"

		usageAny, apiErr := handleTTSResponse(c, resp, info, inputText)
		require.Nil(t, apiErr)

		usage, ok := usageAny.(*dto.Usage)
		require.True(t, ok)
		assert.Equal(t, 6, usage.TotalTokens) // 6 runes
		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
		assert.Equal(t, fakeAudioData, w.Body.Bytes())
	})

	t.Run("error 400 response with json details", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		errJSON := `{"detail": {"status": "invalid_voice", "message": "Voice not found"}}`
		resp := &http.Response{
			StatusCode: http.StatusBadRequest,
			Header: http.Header{
				"Content-Type": []string{"application/json"},
			},
			Body: io.NopCloser(bytes.NewBufferString(errJSON)),
		}

		info := &relaycommon.RelayInfo{
			RelayMode: constant.RelayModeAudioSpeech,
		}

		usageAny, apiErr := handleTTSResponse(c, resp, info, "test")
		assert.Nil(t, usageAny)
		require.NotNil(t, apiErr)
		assert.Contains(t, apiErr.Error(), "Voice not found")
	})
}
