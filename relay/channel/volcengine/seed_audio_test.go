package volcengine

import (
	"bytes"
	"encoding/base64"
	"io"
	"math"
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

func newTestRelayInfo(modelName string, apiKey string, baseUrl string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		RelayMode: constant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: modelName,
			ApiKey:            apiKey,
			ChannelBaseUrl:    baseUrl,
		},
	}
}

func TestExtractVolcengineApiKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "pure api key",
			input:    "volc_secret_token_12345",
			expected: "volc_secret_token_12345",
		},
		{
			name:     "appid and token with pipe",
			input:    "1234567890|real_volc_api_token",
			expected: "real_volc_api_token",
		},
		{
			name:     "appid and token with spaces",
			input:    "  appid123 | token456  ",
			expected: "token456",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "multiple pipes",
			input:    "app|token_part1|part2",
			expected: "token_part1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := extractVolcengineApiKey(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestIsSeedAudioModel(t *testing.T) {
	assert.True(t, isSeedAudioModel("seed-audio-1.0"))
	assert.True(t, isSeedAudioModel("seed-audio-2.0"))
	assert.True(t, isSeedAudioModel("doubao-seed-audio-1.0"))
	assert.True(t, isSeedAudioModel("SEED-AUDIO-1.0"))
	assert.True(t, isSeedAudioModel("  seed-audio-1.0  "))

	assert.False(t, isSeedAudioModel("Doubao-pro-128k"))
	assert.False(t, isSeedAudioModel("seedream-4-0-250828"))
	assert.False(t, isSeedAudioModel("seedance-1-0-pro-250528"))
	assert.False(t, isSeedAudioModel("openai/tts-1"))
}

func TestBuildSeedAudioV3Request(t *testing.T) {
	t.Run("basic mapping", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		info := newTestRelayInfo(ModelSeedAudio10, "", "")
		speed := 1.25
		req := dto.AudioRequest{
			Model:          ModelSeedAudio10,
			Input:          "你好，世界！",
			Voice:          "alloy",
			ResponseFormat: "mp3",
			Speed:          &speed,
		}

		v3Req, err := buildSeedAudioV3Request(c, info, req)
		require.NoError(t, err)
		assert.Equal(t, ModelSeedAudio10, v3Req.Model)
		assert.Equal(t, "你好，世界！", v3Req.TextPrompt)
		assert.Equal(t, "mp3", v3Req.AudioConfig.Format)
		assert.Equal(t, 24000, v3Req.AudioConfig.SampleRate)
		require.NotNil(t, v3Req.AudioConfig.SpeechRate)
		assert.Equal(t, 1.25, *v3Req.AudioConfig.SpeechRate)
		require.Len(t, v3Req.References, 1)
		assert.Equal(t, "zh_male_M392_conversation_wvae_bigtts", v3Req.References[0].Speaker)
	})

	t.Run("extended references and audio config", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		info := newTestRelayInfo(ModelSeedAudio10, "", "")

		refJSON := `[{"speaker":"custom_speaker_1","audio_url":"https://example.com/voice.mp3"}]`
		cfgJSON := `{"format":"wav","sample_rate":16000,"enable_subtitle":true}`
		wmJSON := `{"watermark_type":1}`

		req := dto.AudioRequest{
			Model:       ModelSeedAudio10,
			Input:       "声音复刻测试",
			References:  []byte(refJSON),
			AudioConfig: []byte(cfgJSON),
			Watermark:   []byte(wmJSON),
		}

		v3Req, err := buildSeedAudioV3Request(c, info, req)
		require.NoError(t, err)
		assert.Equal(t, "声音复刻测试", v3Req.TextPrompt)
		require.Len(t, v3Req.References, 1)
		assert.Equal(t, "custom_speaker_1", v3Req.References[0].Speaker)
		assert.Equal(t, "https://example.com/voice.mp3", v3Req.References[0].AudioURL)
		assert.Equal(t, "wav", v3Req.AudioConfig.Format)
		assert.Equal(t, 16000, v3Req.AudioConfig.SampleRate)
		assert.True(t, v3Req.AudioConfig.EnableSubtitle)
		require.NotNil(t, v3Req.Watermark)
		assert.Equal(t, 1, v3Req.Watermark.WatermarkType)
	})

	t.Run("metadata overrides", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		info := newTestRelayInfo(ModelSeedAudio10, "", "")

		metaJSON := `{"model":"seed-audio-custom","text_prompt":"覆盖Prompt","audio_config":{"format":"pcm"}}`
		req := dto.AudioRequest{
			Model:    ModelSeedAudio10,
			Input:    "原始Prompt",
			Metadata: []byte(metaJSON),
		}

		v3Req, err := buildSeedAudioV3Request(c, info, req)
		require.NoError(t, err)
		assert.Equal(t, "seed-audio-custom", v3Req.Model)
		assert.Equal(t, "覆盖Prompt", v3Req.TextPrompt)
		assert.Equal(t, "pcm", v3Req.AudioConfig.Format)
	})
}

func TestHandleSeedAudioV3Response(t *testing.T) {
	dummyAudio := []byte("FAKE_MP3_BINARY_DATA_TEST")
	dummyBase64 := base64.StdEncoding.EncodeToString(dummyAudio)

	t.Run("default binary stream mode", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

		info := newTestRelayInfo(ModelSeedAudio10, "", "")

		v3RespObj := SeedAudioV3Response{
			Audio:            dummyBase64,
			Duration:         4.5,
			OriginalDuration: 5.2,
			URL:              "https://openspeech.bytedance.com/temp/preview.mp3",
			Code:             0,
		}
		respBytes, err := common.Marshal(v3RespObj)
		require.NoError(t, err)

		httpResp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(respBytes)),
		}

		req := dto.AudioRequest{
			Model:          ModelSeedAudio10,
			ResponseFormat: "mp3",
		}

		usageAny, apiErr := handleSeedAudioV3Response(c, httpResp, info, req)
		require.Nil(t, apiErr)
		require.NotNil(t, usageAny)

		usage, ok := usageAny.(*dto.Usage)
		require.True(t, ok)

		// 检查 Header 与数据
		assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
		assert.Equal(t, "4.500", w.Header().Get("X-Audio-Duration"))
		assert.Equal(t, "5.200", w.Header().Get("X-Audio-Original-Duration"))
		assert.Equal(t, "https://openspeech.bytedance.com/temp/preview.mp3", w.Header().Get("X-Audio-Url"))
		assert.Equal(t, dummyAudio, w.Body.Bytes())

		// 计费检查：original_duration = 5.2 -> math.Ceil(5.2) = 6.0
		// AudioTokens = common.QuotaRound(6.0 / 60.0 * 1000) = 100
		expectedTokens := common.QuotaRound(math.Ceil(5.2) / 60.0 * 1000)
		assert.Equal(t, expectedTokens, usage.CompletionTokens)
		assert.Equal(t, expectedTokens, usage.CompletionTokenDetails.AudioTokens)
	})

	t.Run("structured JSON mode via Accept header", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		c.Request.Header.Set("Accept", "application/json")

		info := newTestRelayInfo(ModelSeedAudio10, "", "")

		v3RespObj := SeedAudioV3Response{
			Audio:    dummyBase64,
			Duration: 3.0,
			URL:      "https://openspeech.bytedance.com/temp/json.mp3",
			Subtitle: &SeedAudioSubtitle{
				Sentences: []SeedAudioSentence{
					{
						Text:      "Hello",
						StartTime: 0.0,
						EndTime:   1.0,
					},
				},
			},
			Code: 0,
		}
		respBytes, err := common.Marshal(v3RespObj)
		require.NoError(t, err)

		httpResp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(respBytes)),
		}

		req := dto.AudioRequest{
			Model: ModelSeedAudio10,
		}

		usageAny, apiErr := handleSeedAudioV3Response(c, httpResp, info, req)
		require.Nil(t, apiErr)
		require.NotNil(t, usageAny)

		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
		assert.Equal(t, respBytes, w.Body.Bytes())
	})

	t.Run("structured JSON mode via response_format", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

		info := newTestRelayInfo(ModelSeedAudio10, "", "")

		v3RespObj := SeedAudioV3Response{
			Audio:    dummyBase64,
			Duration: 2.5,
			Code:     0,
		}
		respBytes, err := common.Marshal(v3RespObj)
		require.NoError(t, err)

		httpResp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(respBytes)),
		}

		req := dto.AudioRequest{
			Model:          ModelSeedAudio10,
			ResponseFormat: "verbose_json",
		}

		usageAny, apiErr := handleSeedAudioV3Response(c, httpResp, info, req)
		require.Nil(t, apiErr)
		require.NotNil(t, usageAny)

		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
	})

	t.Run("error response handling", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

		info := newTestRelayInfo(ModelSeedAudio10, "", "")

		v3RespObj := SeedAudioV3Response{
			Code:    40001,
			Message: "invalid speaker parameter",
		}
		respBytes, err := common.Marshal(v3RespObj)
		require.NoError(t, err)

		httpResp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(respBytes)),
		}

		req := dto.AudioRequest{
			Model: ModelSeedAudio10,
		}

		usageAny, apiErr := handleSeedAudioV3Response(c, httpResp, info, req)
		assert.Nil(t, usageAny)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
		assert.Contains(t, apiErr.Err.Error(), "invalid speaker parameter")
	})
}

func TestAdaptorAudioRouting(t *testing.T) {
	adaptor := &Adaptor{}

	t.Run("GetRequestURL routing", func(t *testing.T) {
		// seed-audio model
		infoSeed := newTestRelayInfo(ModelSeedAudio10, "", "")
		url, err := adaptor.GetRequestURL(infoSeed)
		require.NoError(t, err)
		assert.Equal(t, "https://openspeech.bytedance.com/api/v3/tts/create", url)

		// custom proxy BaseURL
		infoProxy := newTestRelayInfo(ModelSeedAudio10, "", "https://custom-proxy.internal.net")
		urlProxy, err := adaptor.GetRequestURL(infoProxy)
		require.NoError(t, err)
		assert.Equal(t, "https://custom-proxy.internal.net/api/v3/tts/create", urlProxy)

		// traditional model
		infoTrad := newTestRelayInfo("traditional-tts-model", "", "")
		urlTrad, err := adaptor.GetRequestURL(infoTrad)
		require.NoError(t, err)
		assert.Equal(t, "wss://openspeech.bytedance.com/api/v1/tts/ws_binary", urlTrad)
	})

	t.Run("SetupRequestHeader authentication", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

		// Seed-audio with appid|token
		headerSeed := http.Header{}
		infoSeed := newTestRelayInfo(ModelSeedAudio10, "dummy_appid|real_api_key_token", "")
		err := adaptor.SetupRequestHeader(c, &headerSeed, infoSeed)
		require.NoError(t, err)
		assert.Equal(t, "real_api_key_token", headerSeed.Get("X-Api-Key"))
		assert.Equal(t, "application/json", headerSeed.Get("Content-Type"))
		assert.Empty(t, headerSeed.Get("Authorization"))

		// Traditional TTS with appid|token
		headerTrad := http.Header{}
		infoTrad := newTestRelayInfo("traditional-tts-model", "dummy_appid|real_token", "")
		err = adaptor.SetupRequestHeader(c, &headerTrad, infoTrad)
		require.NoError(t, err)
		assert.Equal(t, "Bearer;real_token", headerTrad.Get("Authorization"))
		assert.Empty(t, headerTrad.Get("X-Api-Key"))
	})

	t.Run("ConvertAudioRequest protocol branch", func(t *testing.T) {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

		infoSeed := newTestRelayInfo(ModelSeedAudio10, "test_app|test_key", "")

		req := dto.AudioRequest{
			Model: ModelSeedAudio10,
			Input: "测试分流逻辑",
		}

		reader, err := adaptor.ConvertAudioRequest(c, infoSeed, req)
		require.NoError(t, err)
		assert.False(t, infoSeed.IsStream, "HTTP v3 should not set IsStream to true")

		bodyBytes, err := io.ReadAll(reader)
		require.NoError(t, err)

		var parsed SeedAudioV3Request
		err = common.Unmarshal(bodyBytes, &parsed)
		require.NoError(t, err)
		assert.Equal(t, ModelSeedAudio10, parsed.Model)
		assert.Equal(t, "测试分流逻辑", parsed.TextPrompt)
	})
}

func TestAudioTokensCalculationAccuracy(t *testing.T) {
	testCases := []struct {
		name             string
		duration         float64
		originalDuration float64
		expectedTokens   int
	}{
		{
			name:             "zero duration",
			duration:         0,
			originalDuration: 0,
			expectedTokens:   0,
		},
		{
			name:             "sub-second 0.1s duration",
			duration:         0.1,
			originalDuration: 0,
			// ceil(0.1)=1 -> 1/60*1000 = 16.666... -> 17
			expectedTokens: common.QuotaRound(math.Ceil(0.1) / 60.0 * 1000),
		},
		{
			name:             "5.0s duration",
			duration:         5.0,
			originalDuration: 0,
			// ceil(5)=5 -> 5/60*1000 = 83.333... -> 83
			expectedTokens: common.QuotaRound(math.Ceil(5.0) / 60.0 * 1000),
		},
		{
			name:             "5.2s original duration takes precedence over 3.0s duration",
			duration:         3.0,
			originalDuration: 5.2,
			// ceil(5.2)=6 -> 6/60*1000 = 100
			expectedTokens: common.QuotaRound(math.Ceil(5.2) / 60.0 * 1000),
		},
		{
			name:             "59.1s duration",
			duration:         59.1,
			originalDuration: 0,
			// ceil(59.1)=60 -> 60/60*1000 = 1000
			expectedTokens: common.QuotaRound(math.Ceil(59.1) / 60.0 * 1000),
		},
		{
			name:             "exact 60.0s duration",
			duration:         60.0,
			originalDuration: 0,
			// ceil(60)=60 -> 60/60*1000 = 1000
			expectedTokens: common.QuotaRound(math.Ceil(60.0) / 60.0 * 1000),
		},
		{
			name:             "60.1s duration exceeds one minute boundary",
			duration:         60.1,
			originalDuration: 0,
			// ceil(60.1)=61 -> 61/60*1000 = 1016.666... -> 1017
			expectedTokens: common.QuotaRound(math.Ceil(60.1) / 60.0 * 1000),
		},
		{
			name:             "120.0s duration exact two minutes",
			duration:         120.0,
			originalDuration: 0,
			// ceil(120)=120 -> 120/60*1000 = 2000
			expectedTokens: 2000,
		},
	}

	dummyAudio := base64.StdEncoding.EncodeToString([]byte("PCM_BYTES"))

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
			info := newTestRelayInfo(ModelSeedAudio10, "", "")

			v3RespObj := SeedAudioV3Response{
				Audio:            dummyAudio,
				Duration:         tc.duration,
				OriginalDuration: tc.originalDuration,
				Code:             0,
			}
			respBytes, err := common.Marshal(v3RespObj)
			require.NoError(t, err)

			httpResp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(respBytes)),
			}

			req := dto.AudioRequest{Model: ModelSeedAudio10}
			usageAny, apiErr := handleSeedAudioV3Response(c, httpResp, info, req)
			require.Nil(t, apiErr)
			require.NotNil(t, usageAny)

			usage := usageAny.(*dto.Usage)
			assert.Equal(t, tc.expectedTokens, usage.CompletionTokens)
			assert.Equal(t, tc.expectedTokens, usage.CompletionTokenDetails.AudioTokens)
			assert.Equal(t, tc.expectedTokens, usage.TotalTokens)
		})
	}
}

func TestHandleSeedAudioV3ResponseCornerCases(t *testing.T) {
	t.Run("nil response body", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		info := newTestRelayInfo(ModelSeedAudio10, "", "")

		usage, apiErr := handleSeedAudioV3Response(c, nil, info, dto.AudioRequest{})
		assert.Nil(t, usage)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	})

	t.Run("invalid json response body", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		info := newTestRelayInfo(ModelSeedAudio10, "", "")

		httpResp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader([]byte("not-valid-json{{{{"))),
		}

		usage, apiErr := handleSeedAudioV3Response(c, httpResp, info, dto.AudioRequest{})
		assert.Nil(t, usage)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	})

	t.Run("invalid base64 in binary mode", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		info := newTestRelayInfo(ModelSeedAudio10, "", "")

		v3RespObj := SeedAudioV3Response{
			Audio:    "!!!NOT_VALID_BASE64@@@",
			Duration: 5.0,
			Code:     0,
		}
		respBytes, _ := common.Marshal(v3RespObj)

		httpResp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(bytes.NewReader(respBytes)),
		}

		usage, apiErr := handleSeedAudioV3Response(c, httpResp, info, dto.AudioRequest{})
		assert.Nil(t, usage)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusInternalServerError, apiErr.StatusCode)
	})
}

func TestAdaptorDoResponseDispatching(t *testing.T) {
	adaptor := &Adaptor{}
	dummyAudio := []byte("ADAPTOR_TEST_AUDIO")
	dummyBase64 := base64.StdEncoding.EncodeToString(dummyAudio)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)

	req := dto.AudioRequest{
		Model:          ModelSeedAudio10,
		ResponseFormat: "mp3",
	}
	c.Set(contextKeySeedAudioRequest, req)

	info := newTestRelayInfo(ModelSeedAudio10, "", "")
	info.OriginModelName = ModelSeedAudio10

	v3RespObj := SeedAudioV3Response{
		Audio:            dummyBase64,
		Duration:         3.0,
		OriginalDuration: 3.5,
		URL:              "https://openspeech.bytedance.com/audio.mp3",
		Code:             0,
	}
	respBytes, _ := common.Marshal(v3RespObj)

	httpResp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(respBytes)),
	}

	usageAny, apiErr := adaptor.DoResponse(c, httpResp, info)
	require.Nil(t, apiErr)
	require.NotNil(t, usageAny)

	usage := usageAny.(*dto.Usage)
	expectedTokens := common.QuotaRound(math.Ceil(3.5) / 60.0 * 1000)
	assert.Equal(t, expectedTokens, usage.CompletionTokenDetails.AudioTokens)
	assert.Equal(t, "3.000", w.Header().Get("X-Audio-Duration"))
	assert.Equal(t, "3.500", w.Header().Get("X-Audio-Original-Duration"))
	assert.Equal(t, "https://openspeech.bytedance.com/audio.mp3", w.Header().Get("X-Audio-Url"))
	assert.Equal(t, dummyAudio, w.Body.Bytes())
}

func TestTraditionalTTSRegression(t *testing.T) {
	adaptor := &Adaptor{}

	t.Run("Traditional TTS authentication and URL preservation", func(t *testing.T) {
		info := newTestRelayInfo("traditional-voice-model", "appid_legacy|token_legacy", "")
		info.OriginModelName = "traditional-voice-model"

		// 1. URL must remain WebSocket
		url, err := adaptor.GetRequestURL(info)
		require.NoError(t, err)
		assert.Equal(t, "wss://openspeech.bytedance.com/api/v1/tts/ws_binary", url)

		// 2. Headers must remain Bearer token in Authorization
		header := http.Header{}
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		err = adaptor.SetupRequestHeader(c, &header, info)
		require.NoError(t, err)
		assert.Equal(t, "Bearer;token_legacy", header.Get("Authorization"))
		assert.Empty(t, header.Get("X-Api-Key"))

		// 3. ConvertAudioRequest builds traditional VolcengineTTSRequest
		req := dto.AudioRequest{
			Model: "traditional-voice-model",
			Input: "传统TTS测试文本",
			Voice: "alloy",
		}
		reader, err := adaptor.ConvertAudioRequest(c, info, req)
		require.NoError(t, err)
		assert.NotNil(t, reader)

		bodyBytes, err := io.ReadAll(reader)
		require.NoError(t, err)

		var volcReq VolcengineTTSRequest
		err = common.Unmarshal(bodyBytes, &volcReq)
		require.NoError(t, err)
		assert.Equal(t, "appid_legacy", volcReq.App.AppID)
		assert.Equal(t, "token_legacy", volcReq.App.Token)
		assert.Equal(t, "传统TTS测试文本", volcReq.Request.Text)
		assert.Equal(t, "zh_male_M392_conversation_wvae_bigtts", volcReq.Audio.VoiceType)
	})
}

func TestGetModelListContainsSeedAudio(t *testing.T) {
	adaptor := &Adaptor{}
	models := adaptor.GetModelList()
	assert.Contains(t, models, ModelSeedAudio10)
}

