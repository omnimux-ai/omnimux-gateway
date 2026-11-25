package volcengine

import (
	"bytes"
	"encoding/base64"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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

func newTestTTS3RelayInfo(modelName, apiKey, baseURL string) *relaycommon.RelayInfo {
	return &relaycommon.RelayInfo{
		OriginModelName: modelName,
		RelayMode:       constant.RelayModeAudioSpeech,
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: modelName,
			ApiKey:            apiKey,
			ChannelBaseUrl:    baseURL,
		},
	}
}

func TestExtractVolcengineApiKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"pure api key", "volc_secret_token_12345", "volc_secret_token_12345"},
		{"appid and token with pipe", "1234567890|real_volc_api_token", "real_volc_api_token"},
		{"appid and token with spaces", "  appid123 | token456  ", "token456"},
		{"empty string", "", ""},
		{"multiple pipes", "app|token_part1|part2", "token_part1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, extractVolcengineApiKey(tt.input))
		})
	}
}

func TestTTS3BillableCharCounting(t *testing.T) {
	// Official Volcengine TTS 3.0 examples from seed-tts-3-0-intro.
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"official mixed zh and en", "你好，world!", 11},
		{"official jp kanji and katakana", "日本のアニメ", 8},
		{"ascii only", "Hello, TTS 3.0!", 15},
		{"pure chinese", "语音合成", 8},
		{"ssml tags count normally", "<speak>你好</speak>", 19},
		{"empty", "", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, dto.CountVolcengineTTS3BillableChars(tc.input))
		})
	}
}

func TestBuildTTS3Request(t *testing.T) {
	t.Run("basic OpenAI speech mapping", func(t *testing.T) {
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "", "")
		speed := 1.25
		req := dto.AudioRequest{
			Model:          ModelSeedTTS30,
			Input:          "你好，world!",
			Voice:          "alloy",
			Instructions:   "温柔自然",
			ResponseFormat: "mp3",
			Speed:          &speed,
		}

		ttsReq, voiceID, format, chars, err := buildTTS3Request(info, req)
		require.NoError(t, err)
		assert.Equal(t, "multi_female_vv_xi_v3_uranus_bigtts", voiceID)
		assert.Equal(t, "mp3", format)
		assert.Equal(t, 11, chars)
		assert.Equal(t, UpstreamModelDoubaoSeedTTS30, ttsReq.ModelID)
		assert.Equal(t, "你好，world!", ttsReq.Text)
		require.NotNil(t, ttsReq.OutputFormat)
		assert.Equal(t, "mp3", ttsReq.OutputFormat.Format)
		require.NotNil(t, ttsReq.OutputFormat.SampleRate)
		assert.Equal(t, 24000, *ttsReq.OutputFormat.SampleRate)
		require.NotNil(t, ttsReq.VoiceSettings)
		require.NotNil(t, ttsReq.VoiceSettings.Speed)
		assert.Equal(t, 1.25, *ttsReq.VoiceSettings.Speed)
		assert.Equal(t, []string{"温柔自然"}, ttsReq.VoiceSettings.Instructions)
	})

	t.Run("explicit preset voice and zero-value pointer preservation", func(t *testing.T) {
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "", "")
		normFalse := false
		subTrue := true
		silenceZero := 0
		req := dto.AudioRequest{
			Model:                   ModelSeedTTS30,
			Input:                   "hello",
			Voice:                   "multi_male_shenyeboke_v3_uranus_bigtts",
			LanguageCode:            "zh-CN-x-sichuan",
			ResponseFormat:          "opus",
			EnableSubtitle:          &subTrue,
			TextNormalization:       &normFalse,
			SilenceDuration:         &silenceZero,
			VoiceSettings:           []byte(`{"pitch":0,"similarity_boost":false,"dialect_boost":true}`),
			Watermark:               []byte(`{"aigc_watermark":true}`),
			PronunciationDictionary: []byte(`{"tone":["北京/(bei3)(jing1)"]}`),
		}

		ttsReq, voiceID, format, chars, err := buildTTS3Request(info, req)
		require.NoError(t, err)
		assert.Equal(t, "multi_male_shenyeboke_v3_uranus_bigtts", voiceID)
		assert.Equal(t, "ogg_opus", format)
		assert.Equal(t, 5, chars)
		assert.Equal(t, "zh-CN-x-sichuan", ttsReq.LanguageCode)
		require.NotNil(t, ttsReq.OutputFormat.SampleRate)
		assert.Equal(t, 48000, *ttsReq.OutputFormat.SampleRate)

		raw, marshalErr := common.Marshal(ttsReq)
		require.NoError(t, marshalErr)
		rawStr := string(raw)
		assert.Contains(t, rawStr, `"pitch":0`)
		assert.Contains(t, rawStr, `"similarity_boost":false`)
		assert.Contains(t, rawStr, `"text_normalization":false`)
		assert.Contains(t, rawStr, `"silence_duration":0`)
		assert.Contains(t, rawStr, `"enable_subtitle":true`)
		assert.Contains(t, rawStr, `"aigc_watermark":true`)
	})

	t.Run("pcm clears bit_rate per spec", func(t *testing.T) {
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "", "")
		req := dto.AudioRequest{
			Model:        ModelSeedTTS30,
			Input:        "pcm test",
			OutputFormat: []byte(`{"format":"pcm","sample_rate":16000,"bit_rate":64000}`),
		}
		ttsReq, _, format, _, err := buildTTS3Request(info, req)
		require.NoError(t, err)
		assert.Equal(t, "pcm", format)
		require.NotNil(t, ttsReq.OutputFormat.SampleRate)
		assert.Equal(t, 16000, *ttsReq.OutputFormat.SampleRate)
		assert.Nil(t, ttsReq.OutputFormat.BitRate)
	})

	t.Run("rejects mutually exclusive text and ssml", func(t *testing.T) {
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "", "")
		_, _, _, _, err := buildTTS3Request(info, dto.AudioRequest{
			Model: ModelSeedTTS30,
			Input: "hello",
			SSML:  "<speak>hello</speak>",
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "mutually exclusive")
	})

	t.Run("rejects text exceeding 10000 billable chars", func(t *testing.T) {
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "", "")
		// 5001 Han ideographs = 10002 billable chars > 10000
		_, _, _, _, err := buildTTS3Request(info, dto.AudioRequest{
			Model: ModelSeedTTS30,
			Input: strings.Repeat("汉", 5001),
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "exceeds maximum")
	})

	t.Run("rejects path traversal in voice id", func(t *testing.T) {
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "", "")
		for _, bad := range []string{"../other", "a/b", "voice?x=1", "voice#frag", "bad voice"} {
			_, _, _, _, err := buildTTS3Request(info, dto.AudioRequest{
				Model: ModelSeedTTS30,
				Input: "hi",
				Voice: bad,
			})
			require.Error(t, err, "expected invalid voice error for %q", bad)
		}
	})
}

func TestAdaptorTTS3RoutingAndLegacyRejection(t *testing.T) {
	t.Run("URL routing normalizes empty, ark, and legacy openspeech hosts", func(t *testing.T) {
		cases := []struct {
			name    string
			baseURL string
			voice   string
			wantURL string
		}{
			{"empty base", "", "multi_female_vv_xi_v3_uranus_bigtts", "https://voice.cn-beijing.volces.com/api/v3/text-to-speech/multi_female_vv_xi_v3_uranus_bigtts/streaming"},
			{"default ark base", "https://ark.cn-beijing.volces.com", "omnimux_voice01", "https://voice.cn-beijing.volces.com/api/v3/text-to-speech/omnimux_voice01/streaming"},
			{"legacy openspeech base", "https://openspeech.bytedance.com", "omnimux_voice01", "https://voice.cn-beijing.volces.com/api/v3/text-to-speech/omnimux_voice01/streaming"},
			{"custom proxy base", "https://proxy.internal.example/", "omnimux_voice01", "https://proxy.internal.example/api/v3/text-to-speech/omnimux_voice01/streaming"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				adaptor := &Adaptor{}
				c, _ := gin.CreateTestContext(httptest.NewRecorder())
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
				info := newTestTTS3RelayInfo(ModelSeedTTS30, "test-key", tc.baseURL)

				_, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{
					Model: ModelSeedTTS30,
					Input: "你好",
					Voice: tc.voice,
				})
				require.NoError(t, err)
				gotURL, err := adaptor.GetRequestURL(info)
				require.NoError(t, err)
				assert.Equal(t, tc.wantURL, gotURL)
			})
		}
	})

	t.Run("headers use X-Api-Key and X-Request-Id", func(t *testing.T) {
		adaptor := &Adaptor{}
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		header := http.Header{}
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "legacy_appid|real_tts3_key", "")

		require.NoError(t, adaptor.SetupRequestHeader(c, &header, info))
		assert.Equal(t, "real_tts3_key", header.Get("X-Api-Key"))
		assert.Equal(t, "application/json", header.Get("Content-Type"))
		assert.NotEmpty(t, header.Get("X-Request-Id"))
		assert.Empty(t, header.Get("Authorization"))
	})

	t.Run("retired legacy models are rejected", func(t *testing.T) {
		for _, retired := range []string{"seed-audio-1.0", "traditional-tts-model"} {
			adaptor := &Adaptor{}
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
			info := newTestTTS3RelayInfo(retired, "key", "")

			_, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{
				Model: retired,
				Input: "hello",
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "unsupported volcengine speech model")

			_, urlErr := adaptor.GetRequestURL(info)
			require.Error(t, urlErr)
		}
	})

	t.Run("GetModelList advertises seed-tts-3.0 and not seed-audio-1.0", func(t *testing.T) {
		adaptor := &Adaptor{}
		models := adaptor.GetModelList()
		assert.Contains(t, models, ModelSeedTTS30)
		assert.NotContains(t, models, "seed-audio-1.0")
	})
}

func TestHandleTTS3Response(t *testing.T) {
	chunk1 := []byte("MP3_PART_ONE_")
	chunk2 := bytes.Repeat([]byte("A"), 70*1024) // >64KB verifies bufio.Reader handles large JSONL lines
	b64_1 := base64.StdEncoding.EncodeToString(chunk1)
	b64_2 := base64.StdEncoding.EncodeToString(chunk2)

	jsonlStream := strings.Join([]string{
		`{"event":"ready","session_id":"sess-001"}`,
		`{"event":"audio","audio":"` + b64_1 + `"}`,
		`{"event":"alignment","text":"你好","words":[{"word":"你","startTime":0.0,"endTime":0.2},{"word":"好","startTime":0.2,"endTime":0.4}]}`,
		`{"event":"audio","audio":"` + b64_2 + `"}`,
		`{"event":"final","is_final":true,"reason":"TTS_STREAM_FINAL_REASON_CLOSED"}`,
	}, "\n") + "\n"

	t.Run("default binary audio mode aggregates chunks and bills chars", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "", "")

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(jsonlStream)),
		}
		usageAny, apiErr := handleTTS3Response(c, resp, info, dto.AudioRequest{Model: ModelSeedTTS30}, "mp3", 11)
		require.Nil(t, apiErr)

		usage, ok := usageAny.(*dto.Usage)
		require.True(t, ok)
		assert.Equal(t, 11, usage.PromptTokens)
		assert.Equal(t, 0, usage.CompletionTokens)
		assert.Equal(t, 11, usage.TotalTokens)
		assert.Equal(t, 0, usage.CompletionTokenDetails.AudioTokens)
		assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
		assert.Equal(t, "sess-001", w.Header().Get("X-Audio-Session-Id"))
		assert.Equal(t, append(append([]byte{}, chunk1...), chunk2...), w.Body.Bytes())
	})

	t.Run("structured json mode includes base64 audio and alignments", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "", "")

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(jsonlStream)),
		}
		usageAny, apiErr := handleTTS3Response(c, resp, info, dto.AudioRequest{
			Model:          ModelSeedTTS30,
			ResponseFormat: "verbose_json",
		}, "mp3", 4)
		require.Nil(t, apiErr)
		require.NotNil(t, usageAny)
		assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

		var out TTS3AggregatedResponse
		require.NoError(t, common.Unmarshal(w.Body.Bytes(), &out))
		assert.Equal(t, "sess-001", out.SessionID)
		assert.Equal(t, "mp3", out.Format)
		decoded, err := base64.StdEncoding.DecodeString(out.Audio)
		require.NoError(t, err)
		assert.Equal(t, append(append([]byte{}, chunk1...), chunk2...), decoded)
		require.Len(t, out.Alignments, 1)
		assert.Equal(t, "你好", out.Alignments[0].Text)
		require.Len(t, out.Alignments[0].Words, 2)
	})

	t.Run("ndjson accept header streams raw JSONL events", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		c.Request.Header.Set("Accept", "application/x-ndjson")
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "", "")

		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(jsonlStream)),
		}
		usageAny, apiErr := handleTTS3Response(c, resp, info, dto.AudioRequest{Model: ModelSeedTTS30}, "mp3", 4)
		require.Nil(t, apiErr)
		require.NotNil(t, usageAny)
		assert.Contains(t, w.Header().Get("Content-Type"), "application/x-ndjson")
		assert.Contains(t, w.Body.String(), `"event":"ready"`)
		assert.Contains(t, w.Body.String(), `"event":"final"`)
	})

	t.Run("mid-stream error event fails cleanly with mapped status and nil usage", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "", "")

		errStream := `{"event":"ready","session_id":"s2"}` + "\n" +
			`{"event":"error","error":{"type":"authorization_error","code":"voice_access_denied","message":"no permission"}}` + "\n"
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(errStream)),
		}
		usageAny, apiErr := handleTTS3Response(c, resp, info, dto.AudioRequest{Model: ModelSeedTTS30}, "mp3", 4)
		assert.Nil(t, usageAny)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusForbidden, apiErr.StatusCode)
		assert.Contains(t, apiErr.Err.Error(), "voice_access_denied")
		assert.Empty(t, w.Body.Bytes())
	})

	t.Run("truncated stream without final event fails cleanly", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
		info := newTestTTS3RelayInfo(ModelSeedTTS30, "", "")

		truncStream := `{"event":"ready","session_id":"s3"}` + "\n" +
			`{"event":"audio","audio":"` + b64_1 + `"}` + "\n"
		resp := &http.Response{
			StatusCode: http.StatusOK,
			Body:       io.NopCloser(strings.NewReader(truncStream)),
		}
		usageAny, apiErr := handleTTS3Response(c, resp, info, dto.AudioRequest{Model: ModelSeedTTS30}, "mp3", 4)
		assert.Nil(t, usageAny)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	})
}

func TestAdaptorTTS3LoopbackRoundTrip(t *testing.T) {
	audioBytes := []byte("FAKE_TTS3_MP3_BYTES")
	audioB64 := base64.StdEncoding.EncodeToString(audioBytes)

	var gotPath, gotKey, gotReqID string
	var gotPayload TTS3Request
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("X-Api-Key")
		gotReqID = r.Header.Get("X-Request-Id")
		raw, _ := io.ReadAll(r.Body)
		_ = common.Unmarshal(raw, &gotPayload)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(
			`{"event":"ready","session_id":"sess-e2e"}` + "\n" +
				`{"event":"audio","audio":"` + audioB64 + `"}` + "\n" +
				`{"event":"final","is_final":true,"reason":"TTS_STREAM_FINAL_REASON_CLOSED"}` + "\n",
		))
	}))
	defer srv.Close()

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/speech", nil)
	info := newTestTTS3RelayInfo(ModelSeedTTS30, "test-volc-key", srv.URL)
	info.ChannelType = 45

	adaptor := &Adaptor{}
	bodyReader, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{
		Model:        ModelSeedTTS30,
		Input:        "你好，world!",
		Voice:        "multi_male_shenyeboke_v3_uranus_bigtts",
		Instructions: "温柔播客风格",
	})
	require.NoError(t, err)

	respAny, err := adaptor.DoRequest(c, info, bodyReader)
	require.NoError(t, err)
	httpResp, ok := respAny.(*http.Response)
	require.True(t, ok)

	usageAny, apiErr := adaptor.DoResponse(c, httpResp, info)
	require.Nil(t, apiErr)
	usage, ok := usageAny.(*dto.Usage)
	require.True(t, ok)

	assert.Equal(t, "/api/v3/text-to-speech/multi_male_shenyeboke_v3_uranus_bigtts/streaming", gotPath)
	assert.Equal(t, "test-volc-key", gotKey)
	assert.NotEmpty(t, gotReqID)
	assert.Equal(t, UpstreamModelDoubaoSeedTTS30, gotPayload.ModelID)
	assert.Equal(t, "你好，world!", gotPayload.Text)
	require.NotNil(t, gotPayload.VoiceSettings)
	assert.Equal(t, []string{"温柔播客风格"}, gotPayload.VoiceSettings.Instructions)
	assert.Equal(t, 11, usage.PromptTokens)
	assert.Equal(t, 0, usage.CompletionTokens)
	assert.Equal(t, audioBytes, w.Body.Bytes())
	assert.Equal(t, "audio/mpeg", w.Header().Get("Content-Type"))
	assert.Equal(t, "sess-e2e", w.Header().Get("X-Audio-Session-Id"))
}
