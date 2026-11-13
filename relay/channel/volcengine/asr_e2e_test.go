package volcengine

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ==============================================================================
// 场景 1：OpenAI 标准入参到火山 Submit Body 的序列化与字段映射
// ==============================================================================
func TestE2E_ASR_ParameterMappingAndSerialization(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("default_parameters_mapping", func(t *testing.T) {
		var receivedReq VolcengineASRSubmitRequest

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, PathAUCBigModelSubmit, r.URL.Path)
			assert.Equal(t, "test-api-key", r.Header.Get("X-Api-Key"))
			assert.Equal(t, ResourceSeedASRAUC, r.Header.Get("X-Api-Resource-Id"))
			assert.Equal(t, "-1", r.Header.Get("X-Api-Sequence"))
			assert.NotEmpty(t, r.Header.Get("X-Api-Request-Id"))

			err := common.DecodeJson(r.Body, &receivedReq)
			require.NoError(t, err)

			w.Header().Set("X-Api-Status-Code", StatusSuccess)
			w.WriteHeader(http.StatusOK)
			resp := VolcengineASRSubmitResponse{
				TaskId:     "task_param_default",
				StatusCode: StatusSuccess,
			}
			data, _ := common.Marshal(resp)
			_, _ = w.Write(data)
		}))
		defer ts.Close()

		client := NewVolcengineASRClient(ts.URL, ts.Client())

		info := &relaycommon.RelayInfo{
			OriginModelName: ModelDoubaoASRBigModel,
			RelayMode:       constant.RelayModeAudioTranscription,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl: ts.URL,
				ApiKey:         "test-api-key",
			},
		}

		audioReq := dto.AudioRequest{
			Model: ModelDoubaoASRBigModel,
			URL:   "https://omnimux.ai/assets/sample.wav",
		}

		submitReq, err := buildASRSubmitRequest(nil, info, audioReq)
		require.NoError(t, err)

		// 验证构建出的默认参数
		assert.Equal(t, "bigmodel", submitReq.Request.ModelName)
		assert.True(t, submitReq.Request.EnableITN, "默认 enable_itn 应为 true")
		assert.True(t, submitReq.Request.EnablePunc, "默认 enable_punc 应为 true")
		assert.True(t, submitReq.Request.EnableDDC, "默认 enable_ddc 应为 true")
		assert.True(t, submitReq.Request.ShowUtterances, "默认 show_utterances 应为 true")
		assert.False(t, submitReq.Request.EnableSpeakerInfo)
		assert.False(t, submitReq.Request.EnableEmotionDetection)
		assert.Nil(t, submitReq.Request.Corpus)

		// 验证默认音频配置
		assert.Equal(t, "https://omnimux.ai/assets/sample.wav", submitReq.Audio.URL)
		assert.Equal(t, "wav", submitReq.Audio.Format)
		assert.Equal(t, 16000, submitReq.Audio.Rate)
		assert.Equal(t, 16, submitReq.Audio.Bits)
		assert.Equal(t, 1, submitReq.Audio.Channel)

		// 通过客户端实际提交，验证网络层序列化传递
		submitResp, err := client.SubmitTask(context.Background(), "test-api-key", ResourceSeedASRAUC, submitReq)
		require.NoError(t, err)
		assert.Equal(t, "task_param_default", submitResp.TaskId)

		// 验证远端反序列化后接收到的各个默认控制参数
		assert.Equal(t, "bigmodel", receivedReq.Request.ModelName)
		assert.True(t, receivedReq.Request.EnableITN)
		assert.True(t, receivedReq.Request.EnablePunc)
		assert.True(t, receivedReq.Request.EnableDDC)
		assert.True(t, receivedReq.Request.ShowUtterances)
		assert.Equal(t, "wav", receivedReq.Audio.Format)
	})

	t.Run("custom_parameters_override", func(t *testing.T) {
		f := false
		tr := true
		promptText := "TikTok爆款商品,带货口播"

		audioReq := dto.AudioRequest{
			Model:                  ModelBigASRAUC, // 1.0 模型
			URL:                    "https://cdn.example.com/audio/speech.m4a?version=2&auth=abc",
			EnableITN:              &f,
			EnablePunc:             &f,
			EnableDDC:              &f,
			EnableSpeakerInfo:      &tr,
			SSDVersion:             "200",
			EnableEmotionDetection: &tr,
			Prompt:                 promptText,
		}

		info := &relaycommon.RelayInfo{
			OriginModelName: ModelBigASRAUC,
			RelayMode:       constant.RelayModeAudioTranscription,
		}

		submitReq, err := buildASRSubmitRequest(nil, info, audioReq)
		require.NoError(t, err)

		// 字段覆盖断言
		assert.False(t, submitReq.Request.EnableITN, "显式传入 false 需正确覆盖")
		assert.False(t, submitReq.Request.EnablePunc, "显式传入 false 需正确覆盖")
		assert.False(t, submitReq.Request.EnableDDC, "显式传入 false 需正确覆盖")
		assert.True(t, submitReq.Request.EnableSpeakerInfo)
		assert.Equal(t, "200", submitReq.Request.SSDVersion)
		assert.True(t, submitReq.Request.EnableEmotionDetection)
		require.NotNil(t, submitReq.Request.Corpus)
		assert.Equal(t, promptText, submitReq.Request.Corpus.Context)

		// Resource ID 解析检查：bigasr-auc 应解析为 ResourceBigASRAUC (volc.bigasr.auc)
		assert.Equal(t, ResourceBigASRAUC, resolveASRResourceId(audioReq, info))
	})

	t.Run("speaker_info_default_ssd_version", func(t *testing.T) {
		tr := true
		audioReq := dto.AudioRequest{
			Model:             ModelDoubaoASRBigModel,
			URL:               "https://cdn.example.com/audio/speech.mp3",
			EnableSpeakerInfo: &tr,
			// 未指定 SSDVersion
		}

		submitReq, err := buildASRSubmitRequest(nil, nil, audioReq)
		require.NoError(t, err)
		assert.True(t, submitReq.Request.EnableSpeakerInfo)
		assert.Equal(t, "300", submitReq.Request.SSDVersion, "开启说话人分离时未提供版本应默认兜底 300")
	})

	t.Run("audio_format_inference_matrix", func(t *testing.T) {
		tests := []struct {
			url            string
			explicitFormat string
			expectedFormat string
		}{
			{"https://cdn.com/test.wav", "", "wav"},
			{"https://cdn.com/test.mp3", "", "mp3"},
			{"https://cdn.com/test.m4a", "", "m4a"},
			{"https://cdn.com/test.ogg", "", "ogg"},
			{"https://cdn.com/test.oga", "", "ogg"},
			{"https://cdn.com/test.aac", "", "aac"},
			{"https://cdn.com/test.flac", "", "flac"},
			{"https://cdn.com/test.mp4", "", "mp4"},
			{"https://cdn.com/test.pcm", "", "pcm"},
			{"https://cdn.com/test.unknown", "", "mp3"},                         // 默认兜底
			{"https://cdn.com/test.wav?sig=xxx&expires=123456", "", "wav"},       // Query 参数剥离
			{"https://cdn.com/test.wav", "flac", "flac"},                        // 显式 Format 覆盖 URL
		}

		for _, tc := range tests {
			req := dto.AudioRequest{
				Format: tc.explicitFormat,
			}
			assert.Equal(t, tc.expectedFormat, inferAudioFormat(req, tc.url), "URL: %s, Explicit: %s", tc.url, tc.explicitFormat)
		}
	})

	t.Run("url_resolution_precedence", func(t *testing.T) {
		// 1. request.URL 优先
		{
			req := dto.AudioRequest{URL: "https://url1.com/a.mp3", AudioURL: "https://url2.com/b.mp3"}
			submitReq, err := buildASRSubmitRequest(nil, nil, req)
			require.NoError(t, err)
			assert.Equal(t, "https://url1.com/a.mp3", submitReq.Audio.URL)
		}

		// 2. request.AudioURL 降级
		{
			req := dto.AudioRequest{AudioURL: "https://url2.com/b.mp3"}
			submitReq, err := buildASRSubmitRequest(nil, nil, req)
			require.NoError(t, err)
			assert.Equal(t, "https://url2.com/b.mp3", submitReq.Audio.URL)
		}

		// 3. 从 Gin Form / Query 读取
		{
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			formValues := url.Values{}
			formValues.Set("audio_url", "https://url3.com/c.mp3")
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", strings.NewReader(formValues.Encode()))
			c.Request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

			submitReq, err := buildASRSubmitRequest(c, nil, dto.AudioRequest{})
			require.NoError(t, err)
			assert.Equal(t, "https://url3.com/c.mp3", submitReq.Audio.URL)
		}
	})
}

// ==============================================================================
// 场景 2：轮询器指数退避与多状态码流转（排队中 -> 处理中 -> 完成）
// ==============================================================================
func TestE2E_ASR_PollerExponentialBackoffAndStateTransitions(t *testing.T) {
	var queryCount int32
	var queryTimestamps []time.Time

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, PathAUCBigModelQuery, r.URL.Path)
		assert.Equal(t, "task_state_transition", r.Header.Get("X-Api-Request-Id"))

		count := atomic.AddInt32(&queryCount, 1)
		queryTimestamps = append(queryTimestamps, time.Now())
		w.WriteHeader(http.StatusOK)

		switch count {
		case 1:
			// 第一次轮询：排队中 StatusQueued (20000002)
			w.Header().Set("X-Api-Status-Code", StatusQueued)
			resp := VolcengineASRQueryResponse{
				StatusCode: StatusQueued,
				Message:    "Queued",
			}
			data, _ := common.Marshal(resp)
			_, _ = w.Write(data)

		case 2:
			// 第二次轮询：处理中 StatusProcessing (20000001)
			w.Header().Set("X-Api-Status-Code", StatusProcessing)
			resp := VolcengineASRQueryResponse{
				StatusCode: StatusProcessing,
				Message:    "Processing",
			}
			data, _ := common.Marshal(resp)
			_, _ = w.Write(data)

		case 3:
			// 第三次轮询：成功 StatusSuccess (20000000)
			w.Header().Set("X-Api-Status-Code", StatusSuccess)
			resp := VolcengineASRQueryResponse{
				StatusCode: StatusSuccess,
				Result: VolcengineASRResult{
					Text: "转写状态机流转成功",
					Utterances: []VolcengineASRUtterance{
						{
							Text:      "转写状态机流转成功",
							StartTime: 0,
							EndTime:   3000,
						},
					},
				},
			}
			data, _ := common.Marshal(resp)
			_, _ = w.Write(data)
		}
	}))
	defer ts.Close()

	client := NewVolcengineASRClient(ts.URL, ts.Client())

	// 配置快速指数退避测试：初始 20ms, 退避 2.0x, 上限 200ms
	poller := NewASRPoller(&ASRPollerConfig{
		InitialInterval:   20 * time.Millisecond,
		MaxInterval:       200 * time.Millisecond,
		BackoffFactor:     2.0,
		OverallTimeout:    2 * time.Second,
		MaxNetworkRetries: 3,
	})

	startTime := time.Now()
	result, err := poller.Poll(context.Background(), client, "test-api-key", ResourceSeedASRAUC, "task_state_transition")
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "转写状态机流转成功", result.Text)
	assert.Equal(t, int32(3), atomic.LoadInt32(&queryCount))

	// 验证退避时间间隔：第一次和第二次之间 >= 20ms，第二次和第三次之间 >= 40ms
	elapsed := time.Since(startTime)
	assert.GreaterOrEqual(t, elapsed, 60*time.Millisecond, "20ms + 40ms 累计至少 60ms")

	// 附加验证：偶发网络故障自愈重试
	t.Run("network_retry_healing", func(t *testing.T) {
		var retryCount int32
		retryTs := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			count := atomic.AddInt32(&retryCount, 1)
			if count <= 2 {
				// 前两次模拟上游连接断开 / 500
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte("bad gateway"))
				return
			}
			// 第三次恢复正常
			w.Header().Set("X-Api-Status-Code", StatusSuccess)
			resp := VolcengineASRQueryResponse{
				StatusCode: StatusSuccess,
				Result: VolcengineASRResult{
					Text: "网络自愈转写成功",
				},
			}
			data, _ := common.Marshal(resp)
			_, _ = w.Write(data)
		}))
		defer retryTs.Close()

		retryClient := NewVolcengineASRClient(retryTs.URL, retryTs.Client())
		p := NewASRPoller(&ASRPollerConfig{
			InitialInterval:   10 * time.Millisecond,
			OverallTimeout:    1 * time.Second,
			MaxNetworkRetries: 3,
		})

		res, err := p.Poll(context.Background(), retryClient, "key", ResourceSeedASRAUC, "task_retry")
		require.NoError(t, err)
		assert.Equal(t, "网络自愈转写成功", res.Text)
		assert.Equal(t, int32(3), atomic.LoadInt32(&retryCount))
	})
}

// ==============================================================================
// 场景 3：5 种格式输出的内容结构、Header Content-Type 和时间轴格式严格验证
// ==============================================================================
func TestE2E_ASR_FiveResponseFormatsAndTimeline(t *testing.T) {
	// 构造具有代表性的多句识别结果：
	// 包含跨秒、跨毫秒时间轴与说话人/情绪/语速/音量属性
	mockASRResult := VolcengineASRResult{
		Text: "欢迎来到直播间！今天为大家带来首发好物，点击下方小黄车下单享受立减。",
		Utterances: []VolcengineASRUtterance{
			{
				Text:      "欢迎来到直播间！",
				StartTime: 0,
				EndTime:   1250, // 00:00:00,000 -> 00:00:01,250
				Words: []VolcengineASRWord{
					{Text: "欢迎", StartTime: 0, EndTime: 400},
					{Text: "来到", StartTime: 400, EndTime: 800},
					{Text: "直播间", StartTime: 800, EndTime: 1250},
				},
				Additions: VolcengineASRAdditions{
					Speaker:    "speaker_host",
					Emotion:    "excited",
					SpeechRate: 4.8,
					Volume:     0.92,
				},
			},
			{
				Text:      "今天为大家带来首发好物，",
				StartTime: 1400,
				EndTime:   3800, // 00:00:01,400 -> 00:00:03,800
				Additions: VolcengineASRAdditions{
					Speaker:    "speaker_host",
					Emotion:    "neutral",
					SpeechRate: 4.2,
					Volume:     0.88,
				},
			},
			{
				Text:      "点击下方小黄车下单享受立减。",
				StartTime: 3950,
				EndTime:   65000, // 00:00:03,950 -> 00:01:05,000 (跨分钟 65秒)
				Additions: VolcengineASRAdditions{
					Speaker:    "speaker_assistant",
					Emotion:    "happy",
					SpeechRate: 5.0,
					Volume:     0.95,
				},
			},
		},
	}

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case PathAUCBigModelSubmit:
			w.Header().Set("X-Api-Status-Code", StatusSuccess)
			resp := VolcengineASRSubmitResponse{
				TaskId:     "task_format_matrix",
				StatusCode: StatusSuccess,
			}
			data, _ := common.Marshal(resp)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		case PathAUCBigModelQuery:
			w.Header().Set("X-Api-Status-Code", StatusSuccess)
			resp := VolcengineASRQueryResponse{
				StatusCode: StatusSuccess,
				Result:     mockASRResult,
			}
			data, _ := common.Marshal(resp)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(data)
		default:
			http.NotFound(w, r)
		}
	}))
	defer mockServer.Close()

	runFormatTest := func(format string) (int, string, string, *dto.Usage) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)

		info := &relaycommon.RelayInfo{
			OriginModelName: ModelDoubaoASRBigModel,
			RelayMode:       constant.RelayModeAudioTranscription,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl: mockServer.URL,
				ApiKey:         "test-key",
			},
		}

		req := dto.AudioRequest{
			Model:          ModelDoubaoASRBigModel,
			URL:            "https://cdn.omnimux.ai/test.mp3",
			ResponseFormat: format,
		}

		adaptor := &Adaptor{}
		_, err := adaptor.ConvertAudioRequest(c, info, req)
		require.NoError(t, err)

		usageObj, apiErr := adaptor.DoResponse(c, nil, info)
		require.Nil(t, apiErr)

		usage, _ := usageObj.(*dto.Usage)
		return w.Code, w.Header().Get("Content-Type"), w.Body.String(), usage
	}

	// 1. json 格式验证
	t.Run("format_json", func(t *testing.T) {
		code, ct, body, _ := runFormatTest("json")
		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, "application/json", ct)

		var resp dto.AudioResponse
		err := common.Unmarshal([]byte(body), &resp)
		require.NoError(t, err)
		assert.Equal(t, mockASRResult.Text, resp.Text)
	})

	// 2. text 纯文本格式验证
	t.Run("format_text", func(t *testing.T) {
		code, ct, body, _ := runFormatTest("text")
		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, "text/plain; charset=utf-8", ct)
		assert.Equal(t, mockASRResult.Text, body)
	})

	// 3. verbose_json 详细格式验证
	t.Run("format_verbose_json", func(t *testing.T) {
		code, ct, body, _ := runFormatTest("verbose_json")
		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, "application/json", ct)

		var resp dto.WhisperVerboseJSONResponse
		err := common.Unmarshal([]byte(body), &resp)
		require.NoError(t, err)

		assert.Equal(t, "transcribe", resp.Task)
		assert.Equal(t, "chinese", resp.Language)
		assert.Equal(t, 65.0, resp.Duration, "65000ms 对应 duration 65.0s")
		assert.Equal(t, mockASRResult.Text, resp.Text)
		require.Len(t, resp.Segments, 3)

		// 验证第 1 句段落属性
		seg0 := resp.Segments[0]
		assert.Equal(t, 0, seg0.Id)
		assert.Equal(t, 0.0, seg0.Start)
		assert.Equal(t, 1.25, seg0.End)
		assert.Equal(t, "欢迎来到直播间！", seg0.Text)
		assert.Equal(t, "speaker_host", seg0.Speaker)
		assert.Equal(t, "excited", seg0.Emotion)
		assert.Equal(t, 4.8, seg0.SpeechRate)
		assert.Equal(t, 0.92, seg0.Volume)

		// 验证第 3 句跨分钟段落属性
		seg2 := resp.Segments[2]
		assert.Equal(t, 2, seg2.Id)
		assert.Equal(t, 3.95, seg2.Start)
		assert.Equal(t, 65.0, seg2.End)
		assert.Equal(t, "点击下方小黄车下单享受立减。", seg2.Text)
		assert.Equal(t, "speaker_assistant", seg2.Speaker)
	})

	// 4. srt 字幕格式严格验证 (序号递增、--> 符号、逗号毫秒)
	t.Run("format_srt_timeline_strict", func(t *testing.T) {
		code, ct, body, _ := runFormatTest("srt")
		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, "text/plain; charset=utf-8", ct)

		// 必须包含递增序号
		assert.True(t, strings.HasPrefix(body, "1\n"), "SRT 第一行应以序号 1 开头")
		assert.Contains(t, body, "\n2\n")
		assert.Contains(t, body, "\n3\n")

		// 必须包含 --> 时间箭头
		assert.Contains(t, body, " --> ")

		// 必须是逗号 ',' 分隔毫秒，不可是点号 '.'
		assert.Contains(t, body, "00:00:00,000 --> 00:00:01,250")
		assert.Contains(t, body, "00:00:01,400 --> 00:00:03,800")
		assert.Contains(t, body, "00:00:03,950 --> 00:01:05,000") // 65秒 -> 01分05秒
		assert.False(t, strings.Contains(body, "00:00:00.000"), "SRT 严禁使用点号分隔毫秒")

		// 严格期望排版
		expectedSRT := "1\n00:00:00,000 --> 00:00:01,250\n欢迎来到直播间！\n\n" +
			"2\n00:00:01,400 --> 00:00:03,800\n今天为大家带来首发好物，\n\n" +
			"3\n00:00:03,950 --> 00:01:05,000\n点击下方小黄车下单享受立减。\n\n"
		assert.Equal(t, expectedSRT, body)
	})

	// 5. vtt 字幕格式严格验证 (WEBVTT 声明、点号毫秒)
	t.Run("format_vtt_timeline_strict", func(t *testing.T) {
		code, ct, body, _ := runFormatTest("vtt")
		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, "text/vtt; charset=utf-8", ct)

		// 头部声明校验
		assert.True(t, strings.HasPrefix(body, "WEBVTT\n\n"), "VTT 必须以 WEBVTT\\n\\n 声明起始")

		// 必须是点号 '.' 分隔毫秒，不可是逗号 ','
		assert.Contains(t, body, "00:00:00.000 --> 00:00:01.250")
		assert.Contains(t, body, "00:00:01.400 --> 00:00:03.800")
		assert.Contains(t, body, "00:00:03.950 --> 00:01:05.000")
		assert.False(t, strings.Contains(body, "00:00:00,000"), "VTT 严禁使用逗号分隔毫秒")

		// 严格期望排版
		expectedVTT := "WEBVTT\n\n" +
			"00:00:00.1250" // 确认实际格式
		_ = expectedVTT

		expectedVTTExact := "WEBVTT\n\n" +
			"00:00:00.000 --> 00:00:01.250\n欢迎来到直播间！\n\n" +
			"00:00:01.400 --> 00:00:03.800\n今天为大家带来首发好物，\n\n" +
			"00:00:03.950 --> 00:01:05.000\n点击下方小黄车下单享受立减。\n\n"
		assert.Equal(t, expectedVTTExact, body)
	})

	// 6. doubao_json 原生出参透传
	t.Run("format_doubao_json", func(t *testing.T) {
		code, ct, body, _ := runFormatTest("doubao_json")
		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, "application/json", ct)

		var rawResult VolcengineASRResult
		err := common.Unmarshal([]byte(body), &rawResult)
		require.NoError(t, err)
		assert.Equal(t, mockASRResult.Text, rawResult.Text)
		assert.Len(t, rawResult.Utterances, 3)
	})
}

// ==============================================================================
// 场景 4：音频时长与 AudioTokens 计费精确度
// ==============================================================================
func TestE2E_ASR_AudioTokensBillingPrecision(t *testing.T) {
	testCases := []struct {
		name           string
		durationMs     int64
		hasText        bool
		expectedTokens int
	}{
		{
			name:           "65s_audio_1084_tokens",
			durationMs:     65000, // 65秒: ceil(65 / 60 * 1000) = ceil(1083.333) = 1084
			hasText:        true,
			expectedTokens: 1084,
		},
		{
			name:           "30s_audio_500_tokens",
			durationMs:     30000, // 30秒: ceil(30 / 60 * 1000) = 500
			hasText:        true,
			expectedTokens: 500,
		},
		{
			name:           "60s_audio_exact_1_minute_1000_tokens",
			durationMs:     60000, // 60秒: ceil(60 / 60 * 1000) = 1000
			hasText:        true,
			expectedTokens: 1000,
		},
		{
			name:           "61s_audio_exceeds_1_minute_1017_tokens",
			durationMs:     61000, // 61秒: ceil(61 / 60 * 1000) = ceil(1016.666) = 1017
			hasText:        true,
			expectedTokens: 1017,
		},
		{
			name:           "0.5s_audio_sub_second_9_tokens",
			durationMs:     500, // 0.5秒: ceil(0.5 / 60 * 1000) = ceil(8.333) = 9
			hasText:        true,
			expectedTokens: 9,
		},
		{
			name:           "zero_duration_with_text_fallback_1000_tokens",
			durationMs:     0, // 0时长但有文本: 保底 1000 tokens (1分钟)
			hasText:        true,
			expectedTokens: 1000,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case PathAUCBigModelSubmit:
					w.Header().Set("X-Api-Status-Code", StatusSuccess)
					resp := VolcengineASRSubmitResponse{TaskId: "task_billing", StatusCode: StatusSuccess}
					data, _ := common.Marshal(resp)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
				case PathAUCBigModelQuery:
					w.Header().Set("X-Api-Status-Code", StatusSuccess)
					var utterances []VolcengineASRUtterance
					if tc.durationMs > 0 {
						utterances = []VolcengineASRUtterance{
							{
								Text:      "计费时长测试",
								StartTime: 0,
								EndTime:   tc.durationMs,
							},
						}
					}
					resp := VolcengineASRQueryResponse{
						StatusCode: StatusSuccess,
						Result: VolcengineASRResult{
							Text:       "计费时长测试",
							Utterances: utterances,
						},
					}
					data, _ := common.Marshal(resp)
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(data)
				}
			}))
			defer ts.Close()

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)

			info := &relaycommon.RelayInfo{
				OriginModelName: ModelDoubaoASRBigModel,
				RelayMode:       constant.RelayModeAudioTranscription,
				ChannelMeta: &relaycommon.ChannelMeta{
					ChannelBaseUrl: ts.URL,
					ApiKey:         "test-key",
				},
			}

			adaptor := &Adaptor{}
			_, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{
				Model: ModelDoubaoASRBigModel,
				URL:   "https://omnimux.ai/billing.mp3",
			})
			require.NoError(t, err)

			usageObj, apiErr := adaptor.DoResponse(c, nil, info)
			require.Nil(t, apiErr)
			require.NotNil(t, usageObj)

			usage, ok := usageObj.(*dto.Usage)
			require.True(t, ok)

			// 严密验证三种 token 计费字段的一致性与精确性
			assert.Equal(t, tc.expectedTokens, usage.PromptTokens, "PromptTokens 计量不符")
			assert.Equal(t, tc.expectedTokens, usage.TotalTokens, "TotalTokens 计量不符")
			assert.Equal(t, tc.expectedTokens, usage.PromptTokensDetails.AudioTokens, "AudioTokens 细分计量不符")

			// 验证 math.Ceil 理论计算公式等价性
			if tc.durationMs > 0 {
				expectedMath := int(math.Ceil(float64(tc.durationMs) / 1000.0 / 60.0 * 1000.0))
				assert.Equal(t, expectedMath, usage.PromptTokensDetails.AudioTokens)
			}
		})
	}
}

// ==============================================================================
// 场景 5：异常处理与优雅降级（45000010 鉴权错误、55000000 内部错误、客户端断连、缺失 URL）
// ==============================================================================
func TestE2E_ASR_ErrorHandlingAndGracefulDegradation(t *testing.T) {
	// 1. Submit 阶段返回 45000010 鉴权错误
	t.Run("submit_auth_error_45000010", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Api-Status-Code", "45000010")
			w.Header().Set("X-Api-Message", "Authentication failed: invalid token or secret key")
			w.WriteHeader(http.StatusOK)
			resp := VolcengineASRSubmitResponse{
				StatusCode: "45000010",
				Message:    "Authentication failed: invalid token or secret key",
			}
			data, _ := common.Marshal(resp)
			_, _ = w.Write(data)
		}))
		defer ts.Close()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)

		info := &relaycommon.RelayInfo{
			OriginModelName: ModelDoubaoASRBigModel,
			RelayMode:       constant.RelayModeAudioTranscription,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl: ts.URL,
				ApiKey:         "invalid-key",
			},
		}

		adaptor := &Adaptor{}
		_, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{
			Model: ModelDoubaoASRBigModel,
			URL:   "https://omnimux.ai/test.mp3",
		})
		require.NoError(t, err)

		_, apiErr := adaptor.DoResponse(c, nil, info)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
		assert.Contains(t, apiErr.Error(), "45000010")
		assert.Contains(t, apiErr.Error(), "Authentication failed")
	})

	// 2. Query 轮询阶段返回 55000000 内部服务异常
	t.Run("query_internal_error_55000000", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case PathAUCBigModelSubmit:
				w.Header().Set("X-Api-Status-Code", StatusSuccess)
				resp := VolcengineASRSubmitResponse{TaskId: "task_internal_err", StatusCode: StatusSuccess}
				data, _ := common.Marshal(resp)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)
			case PathAUCBigModelQuery:
				w.Header().Set("X-Api-Status-Code", "55000000")
				resp := VolcengineASRQueryResponse{
					StatusCode: "55000000",
					Message:    "Internal engine failure during speech decode",
				}
				data, _ := common.Marshal(resp)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)
			}
		}))
		defer ts.Close()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)

		info := &relaycommon.RelayInfo{
			OriginModelName: ModelDoubaoASRBigModel,
			RelayMode:       constant.RelayModeAudioTranscription,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl: ts.URL,
				ApiKey:         "valid-key",
			},
		}

		adaptor := &Adaptor{}
		_, _ = adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{
			Model: ModelDoubaoASRBigModel,
			URL:   "https://omnimux.ai/test.mp3",
		})

		_, apiErr := adaptor.DoResponse(c, nil, info)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
		assert.Contains(t, apiErr.Error(), "55000000")
		assert.Contains(t, apiErr.Error(), "Internal engine failure")
	})

	// 3. 客户端在轮询中主动断开 / 超时 (Context Canceled -> 499)
	t.Run("client_context_cancel_499_graceful_exit", func(t *testing.T) {
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case PathAUCBigModelSubmit:
				resp := VolcengineASRSubmitResponse{TaskId: "task_cancel_499", StatusCode: StatusSuccess}
				data, _ := common.Marshal(resp)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)
			case PathAUCBigModelQuery:
				// 一直返回 processing
				w.Header().Set("X-Api-Status-Code", StatusProcessing)
				resp := VolcengineASRQueryResponse{StatusCode: StatusProcessing}
				data, _ := common.Marshal(resp)
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(data)
			}
		}))
		defer ts.Close()

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		ctx, cancel := context.WithCancel(context.Background())
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil).WithContext(ctx)

		info := &relaycommon.RelayInfo{
			OriginModelName: ModelDoubaoASRBigModel,
			RelayMode:       constant.RelayModeAudioTranscription,
			ChannelMeta: &relaycommon.ChannelMeta{
				ChannelBaseUrl: ts.URL,
				ApiKey:         "valid-key",
			},
		}

		// 异步 40ms 后客户端断开
		go func() {
			time.Sleep(40 * time.Millisecond)
			cancel()
		}()

		adaptor := &Adaptor{}
		_, _ = adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{
			Model: ModelDoubaoASRBigModel,
			URL:   "https://omnimux.ai/test.mp3",
		})

		_, apiErr := adaptor.DoResponse(c, nil, info)
		require.NotNil(t, apiErr)
		assert.Equal(t, 499, apiErr.StatusCode, "客户端主动断连应优雅映射为 HTTP 499")
	})

	// 4. 缺少音频 URL 时的参数校验防御（URL-first 模式）
	t.Run("missing_audio_url_returns_400", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodPost, "/v1/audio/transcriptions", nil)

		info := &relaycommon.RelayInfo{
			OriginModelName: ModelDoubaoASRBigModel,
			RelayMode:       constant.RelayModeAudioTranscription,
		}

		adaptor := &Adaptor{}
		// 传完全空的 AudioRequest
		_, err := adaptor.ConvertAudioRequest(c, info, dto.AudioRequest{
			Model: ModelDoubaoASRBigModel,
		})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "audio url is required")

		// 通过 DoResponse 调用也会在 buildASRSubmitRequest 拦截并返回 400
		_, apiErr := adaptor.DoResponse(c, nil, info)
		require.NotNil(t, apiErr)
		assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
		assert.Equal(t, types.ErrorCodeInvalidRequest, apiErr.GetErrorCode())
		assert.Contains(t, apiErr.Error(), "audio url is required")
	})
}
