package volcvoice

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	taskcommon "github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// 官方命名规范
// ---------------------------------------------------------------------------

func TestValidateCustomSpeakerID(t *testing.T) {
	cases := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{"合法代号（平台命名空间内）", "omnimux_anchorzhfemale01", false},
		{"命名空间外被拒", "anchor_zh_female01", true},
		{"以中划线开头被拒", "-omnimux_anchorvoice", true},
		{"以数字开头被拒", "1omnimux_anchorvoice", true},
		{"以官方保留前缀开头被拒", "S_12345678", true},
		{"以 ICL 前缀开头被拒", "ICL_custom01", true},
		{"过短被拒", "abc", true},
		{"以中划线结尾被拒", "omnimux_anchorvoice-", true},
		{"含非法字符被拒", "omnimux_anchor voice", true},
		{"超长被拒", "omnimux_" + strings.Repeat("a", 257), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateCustomSpeakerID(tc.id)
			if tc.wantErr {
				assert.Error(t, err, "期望被拒绝: %s", tc.id)
			} else {
				assert.NoError(t, err, "期望通过: %s", tc.id)
			}
		})
	}
}

func TestResolveSpeakerIDGeneratesConformingID(t *testing.T) {
	generated := resolveSpeakerID("")
	require.NotEmpty(t, generated)
	// 生成值必须自洽地通过官方命名校验，否则平台自动命名的音色会被上游拒绝。
	assert.NoError(t, validateCustomSpeakerID(generated))

	other := resolveSpeakerID("")
	assert.NotEqual(t, generated, other, "两次生成不应相同")

	assert.Equal(t, "omnimux_keepme", resolveSpeakerID("omnimux_keepme"), "显式代号应原样保留")
}

// ---------------------------------------------------------------------------
// 输入约束
// ---------------------------------------------------------------------------

func TestValidateDemoTextBounds(t *testing.T) {
	assert.NoError(t, validateDemoText(""), "未提供试听文本时不做校验")
	assert.NoError(t, validateDemoText("夜色渐浓，城市的灯火次第亮起。"))
	assert.Error(t, validateDemoText("好"), "低于 4 字应被拒")
	assert.Error(t, validateDemoText(strings.Repeat("测", 301)), "超过 300 字应被拒")
	assert.NoError(t, validateDemoText(strings.Repeat("测", 300)), "边界 300 字应通过")
}

func TestValidateAudioPayload(t *testing.T) {
	t.Run("缺少音频来源被拒", func(t *testing.T) {
		err := validateAudioPayload(&voiceCloneRequest{})
		require.NotNil(t, err)
		assert.Equal(t, "audio_required", err.Code)
	})

	t.Run("仅提供 audio_url 通过", func(t *testing.T) {
		assert.Nil(t, validateAudioPayload(&voiceCloneRequest{AudioURL: "https://cdn.example.com/a.mp3"}))
	})

	t.Run("不支持格式被拒", func(t *testing.T) {
		err := validateAudioPayload(&voiceCloneRequest{AudioData: "AAAA", Format: "flac"})
		require.NotNil(t, err)
		assert.Equal(t, "unsupported_audio_format", err.Code)
	})

	t.Run("超过官方 10MB 上限被拒", func(t *testing.T) {
		// base64 每 4 字符代表 3 字节，构造刚好超过 10MB 的编码长度。
		oversized := strings.Repeat("A", (maxAudioBytes/3+2)*4)
		err := validateAudioPayload(&voiceCloneRequest{AudioData: oversized})
		require.NotNil(t, err)
		assert.Equal(t, "audio_too_large", err.Code)
	})

	t.Run("合法载荷通过", func(t *testing.T) {
		assert.Nil(t, validateAudioPayload(&voiceCloneRequest{AudioData: "QUJD", Format: "wav"}))
	})
}

// ---------------------------------------------------------------------------
// 请求构造
// ---------------------------------------------------------------------------

func TestBuildRequestBodyMatchesOfficialContract(t *testing.T) {
	language := 0
	denoise := true
	adaptor := &TaskAdaptor{parsed: &voiceCloneRequest{
		AudioData: "QUJDRA==",
		Format:    "WAV",
		Text:      "欢迎收看今天的节目",
		Language:  &language,
		SpeakerID: "anchor_zh_female01",
		DemoText:  "这是一段试听文本。",
		Denoise:   &denoise,
	}}

	reader, err := adaptor.BuildRequestBody(nil, &relaycommon.RelayInfo{})
	require.NoError(t, err)

	raw, err := io.ReadAll(reader)
	require.NoError(t, err)

	var body upstreamVoiceCloneBody
	require.NoError(t, common.Unmarshal(raw, &body))

	// 官方规定：使用自定义音色时 speaker_id 必须传固定值。
	assert.Equal(t, "custom_speaker_id", body.SpeakerID)
	assert.Equal(t, "anchor_zh_female01", body.CustomSpeakerID)
	assert.Equal(t, "QUJDRA==", body.Audio.Data)
	assert.Equal(t, "wav", body.Audio.Format, "格式应归一化为小写")
	assert.Equal(t, "欢迎收看今天的节目", body.Text)
	require.NotNil(t, body.Language)
	assert.Equal(t, 0, *body.Language)
	require.NotNil(t, body.ExtraParams)
	assert.Equal(t, "这是一段试听文本。", body.ExtraParams.DemoText)
	require.NotNil(t, body.ExtraParams.EnableAudioDenoise)
	assert.True(t, *body.ExtraParams.EnableAudioDenoise)
}

func TestBuildRequestURLCarriesOfficialPath(t *testing.T) {
	adaptor := &TaskAdaptor{baseURL: "https://openspeech.bytedance.com"}
	url, err := adaptor.BuildRequestURL(&relaycommon.RelayInfo{})
	require.NoError(t, err)
	assert.Equal(t, "https://openspeech.bytedance.com/api/v3/tts/voice_clone", url)
}

func TestBuildRequestHeaderUsesSingleKeyAuth(t *testing.T) {
	adaptor := &TaskAdaptor{apiKey: "resolved-key"}
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	require.NoError(t, adaptor.BuildRequestHeader(nil, req, &relaycommon.RelayInfo{}))

	assert.Equal(t, "resolved-key", req.Header.Get("X-Api-Key"))
	assert.NotEmpty(t, req.Header.Get("X-Api-Request-Id"), "官方要求必传请求 ID")
	assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
}

func TestExtractVolcengineApiKeyHandlesLegacyPair(t *testing.T) {
	assert.Equal(t, "token-part", extractVolcengineApiKey("appid|token-part"))
	assert.Equal(t, "plain-key", extractVolcengineApiKey("plain-key"))
	assert.Equal(t, "plain-key", extractVolcengineApiKey("  plain-key  "))
}

// ---------------------------------------------------------------------------
// 状态映射（官方状态机 0/1/2/3/4）
// ---------------------------------------------------------------------------

func newVoiceCloneResponse(t *testing.T, payload string) *http.Response {
	t.Helper()
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewBufferString(payload)),
	}
}

func TestParseResponseMapsTerminalStatusImmediately(t *testing.T) {
	adaptor := &TaskAdaptor{parsed: &voiceCloneRequest{SpeakerID: "anchor_zh_female01"}}
	resp := newVoiceCloneResponse(t, `{
		"code": 0,
		"message": "success",
		"speaker_id": "anchor_zh_female01",
		"status": 2,
		"demo_audio": "https://example.com/demo.mp3",
		"available_training_times": 3,
		"create_time": 1700000000000,
		"speaker_status": [{"model_type": 5}]
	}`)

	submit, taskErr := adaptor.ParseResponse(nil, resp, &relaycommon.RelayInfo{})
	require.Nil(t, taskErr)
	require.NotNil(t, submit)
	assert.Equal(t, "anchor_zh_female01", submit.UpstreamTaskID)

	// 上游在提交阶段即返回终态时应直接给出结果，避免无意义轮询。
	require.NotNil(t, submit.Immediate)
	assert.Equal(t, string(model.TaskStatusSuccess), submit.Immediate.Status)
	assert.Equal(t, taskcommon.ProgressComplete, submit.Immediate.Progress)
	assert.Equal(t, "https://example.com/demo.mp3", submit.Immediate.Url)

	var data voiceCloneTaskData
	require.NoError(t, common.Unmarshal(submit.TaskData, &data))
	require.NotNil(t, data.ModelType)
	assert.Equal(t, 5, *data.ModelType, "复刻 2.0 应记录 model_type=5")
	require.NotNil(t, data.AvailableTrainingTimes)
	assert.Equal(t, 3, *data.AvailableTrainingTimes)
	require.NotNil(t, data.CreateTime)
	assert.Equal(t, int64(1700000000000), *data.CreateTime)
}

func TestParseResponseLeavesTrainingInProgressUnpolled(t *testing.T) {
	adaptor := &TaskAdaptor{parsed: &voiceCloneRequest{SpeakerID: "anchor_zh_female01"}}
	resp := newVoiceCloneResponse(t, `{"code":0,"speaker_id":"anchor_zh_female01","status":1}`)

	submit, taskErr := adaptor.ParseResponse(nil, resp, &relaycommon.RelayInfo{})
	require.Nil(t, taskErr)
	require.NotNil(t, submit)
	assert.Nil(t, submit.Immediate, "训练中不应产生终态")
}

func TestParseResponseSurfacesReadableUpstreamError(t *testing.T) {
	adaptor := &TaskAdaptor{parsed: &voiceCloneRequest{SpeakerID: "anchor_zh_female01"}}
	resp := newVoiceCloneResponse(t, `{"code":45001109,"message":"WERError"}`)

	_, taskErr := adaptor.ParseResponse(nil, resp, &relaycommon.RelayInfo{})
	require.NotNil(t, taskErr)
	assert.Contains(t, taskErr.Message, "45001109")
	assert.Contains(t, taskErr.Message, "差异过大", "错误码应被翻译为可读处置建议")
}

func TestParseTaskResultStatusMapping(t *testing.T) {
	cases := []struct {
		name         string
		payload      string
		wantStatus   string
		wantProgress string
		wantURL      string
	}{
		{"Success 可合成", `{"code":0,"status":2,"demo_audio":"https://example.com/d.mp3"}`, string(model.TaskStatusSuccess), taskcommon.ProgressComplete, "https://example.com/d.mp3"},
		{"Active 可合成", `{"code":0,"status":4,"demo_audio":"https://example.com/d.mp3"}`, string(model.TaskStatusSuccess), taskcommon.ProgressComplete, "https://example.com/d.mp3"},
		{"Training 进行中", `{"code":0,"status":1}`, string(model.TaskStatusInProgress), taskcommon.ProgressInProgress, ""},
		{"NotFound 未开始", `{"code":0,"status":0}`, string(model.TaskStatusNotStart), taskcommon.ProgressSubmitted, ""},
		{"Failed 失败", `{"code":0,"status":3}`, string(model.TaskStatusFailure), "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			adaptor := &TaskAdaptor{}
			task := &model.Task{TaskID: "anchor_zh_female01"}
			info, err := adaptor.ParseTaskResult(task, nil, []byte(tc.payload))
			require.NoError(t, err)
			require.NotNil(t, info)
			assert.Equal(t, tc.wantStatus, info.Status)
			if tc.wantProgress != "" {
				assert.Equal(t, tc.wantProgress, info.Progress)
			}
			assert.Equal(t, tc.wantURL, info.Url)
		})
	}
}

func TestParseTaskResultTreatsUpstreamErrorAsFailure(t *testing.T) {
	adaptor := &TaskAdaptor{}
	info, err := adaptor.ParseTaskResult(&model.Task{TaskID: "x"}, nil, []byte(`{"code":45001112,"message":"no quota"}`))
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, string(model.TaskStatusFailure), info.Status)
	assert.Contains(t, info.Reason, "音色槽位", "失败原因应可读")
}

// ---------------------------------------------------------------------------
// 适配器契约
// ---------------------------------------------------------------------------

func TestAdaptorDeclaresPublicModelAndChannelName(t *testing.T) {
	adaptor := &TaskAdaptor{}
	assert.Equal(t, []string{PublicModelVoiceClone}, adaptor.GetModelList())
	assert.NotEmpty(t, adaptor.GetChannelName())
}

func TestMapUpstreamStatusIgnoresNonTerminal(t *testing.T) {
	training := upstreamStatusTraining
	assert.Nil(t, mapUpstreamStatus(&upstreamVoiceCloneResponse{Status: &training}))
	assert.Nil(t, mapUpstreamStatus(&upstreamVoiceCloneResponse{}))

	active := upstreamStatusActive
	info := mapUpstreamStatus(&upstreamVoiceCloneResponse{Status: &active, DemoAudio: "https://example.com/d.mp3"})
	require.NotNil(t, info)
	assert.Equal(t, string(model.TaskStatusSuccess), info.Status)
}
