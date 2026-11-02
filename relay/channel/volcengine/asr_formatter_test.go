package volcengine

import (
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTimestampFormatting(t *testing.T) {
	// 3661123ms = 1h + 1m + 1s + 123ms -> 01:01:01,123 / 01:01:01.123
	ms := int64(3661123)

	srtTime := formatSRTTimestamp(ms)
	assert.Equal(t, "01:01:01,123", srtTime)

	vttTime := formatVTTTimestamp(ms)
	assert.Equal(t, "01:01:01.123", vttTime)

	// Zero milliseconds
	assert.Equal(t, "00:00:00,000", formatSRTTimestamp(0))
	assert.Equal(t, "00:00:00.000", formatVTTTimestamp(0))

	// Sub-second
	assert.Equal(t, "00:00:00,500", formatSRTTimestamp(500))
	assert.Equal(t, "00:00:00.500", formatVTTTimestamp(500))

	// Multi-hour: 25h 15m 30s 456ms = (25*3600 + 15*60 + 30)*1000 + 456 = 90930456
	assert.Equal(t, "25:15:30,456", formatSRTTimestamp(90930456))
	assert.Equal(t, "25:15:30.456", formatVTTTimestamp(90930456))
}

func sampleASRResult() *VolcengineASRResult {
	return &VolcengineASRResult{
		Text: "大家好，今天给大家推荐这款 TikTok 爆款脱毛仪，现在只要29.9元。",
		Utterances: []VolcengineASRUtterance{
			{
				Text:      "大家好，今天给大家推荐这款 TikTok 爆款脱毛仪，",
				StartTime: 120,
				EndTime:   2350,
				Additions: VolcengineASRAdditions{
					Speaker:    "speaker_0",
					Emotion:    "happy",
					SpeechRate: 4.2,
					Volume:     0.85,
				},
			},
			{
				Text:      "现在只要29.9元。",
				StartTime: 2350,
				EndTime:   5420,
				Additions: VolcengineASRAdditions{
					Speaker: "speaker_0",
					Emotion: "neutral",
					Speed:   3.8, // 测试 speed 兼容
					Volume:  0.80,
				},
			},
		},
	}
}

func TestJSONFormatter(t *testing.T) {
	formatter := &JSONFormatter{}
	result := sampleASRResult()

	data, ct, err := formatter.Format(result, ModelDoubaoASRBigModel)
	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)

	var resp dto.AudioResponse
	err = common.Unmarshal(data, &resp)
	require.NoError(t, err)
	assert.Equal(t, result.Text, resp.Text)
}

func TestTextFormatter(t *testing.T) {
	formatter := &TextFormatter{}
	result := sampleASRResult()

	data, ct, err := formatter.Format(result, ModelDoubaoASRBigModel)
	require.NoError(t, err)
	assert.Equal(t, "text/plain; charset=utf-8", ct)
	assert.Equal(t, result.Text, string(data))
}

func TestVerboseJSONFormatter(t *testing.T) {
	formatter := &VerboseJSONFormatter{}
	result := sampleASRResult()

	data, ct, err := formatter.Format(result, ModelDoubaoASRBigModel)
	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)

	var resp dto.WhisperVerboseJSONResponse
	err = common.Unmarshal(data, &resp)
	require.NoError(t, err)

	assert.Equal(t, "transcribe", resp.Task)
	assert.Equal(t, "chinese", resp.Language)
	assert.Equal(t, 5.42, resp.Duration)
	assert.Equal(t, result.Text, resp.Text)
	require.Len(t, resp.Segments, 2)

	seg0 := resp.Segments[0]
	assert.Equal(t, 0, seg0.Id)
	assert.InDelta(t, 0.12, seg0.Start, 0.001)
	assert.InDelta(t, 2.35, seg0.End, 0.001)
	assert.Equal(t, "大家好，今天给大家推荐这款 TikTok 爆款脱毛仪，", seg0.Text)
	assert.Equal(t, "speaker_0", seg0.Speaker)
	assert.Equal(t, "happy", seg0.Emotion)
	assert.Equal(t, 4.2, seg0.SpeechRate)
	assert.Equal(t, 0.85, seg0.Volume)

	seg1 := resp.Segments[1]
	assert.Equal(t, 1, seg1.Id)
	assert.InDelta(t, 2.35, seg1.Start, 0.001)
	assert.InDelta(t, 5.42, seg1.End, 0.001)
	assert.Equal(t, "现在只要29.9元。", seg1.Text)
	assert.Equal(t, 3.8, seg1.SpeechRate) // speed 属性映射为 speech_rate
}

func TestSRTFormatter(t *testing.T) {
	formatter := &SRTFormatter{}
	result := sampleASRResult()

	data, ct, err := formatter.Format(result, ModelDoubaoASRBigModel)
	require.NoError(t, err)
	assert.Equal(t, "text/plain; charset=utf-8", ct)

	expected := "1\n00:00:00,120 --> 00:00:02,350\n大家好，今天给大家推荐这款 TikTok 爆款脱毛仪，\n\n" +
		"2\n00:00:02,350 --> 00:00:05,420\n现在只要29.9元。\n\n"
	assert.Equal(t, expected, string(data))
}

func TestVTTFormatter(t *testing.T) {
	formatter := &VTTFormatter{}
	result := sampleASRResult()

	data, ct, err := formatter.Format(result, ModelDoubaoASRBigModel)
	require.NoError(t, err)
	assert.Equal(t, "text/vtt; charset=utf-8", ct)

	expected := "WEBVTT\n\n" +
		"00:00:00.120 --> 00:00:02.350\n大家好，今天给大家推荐这款 TikTok 爆款脱毛仪，\n\n" +
		"00:00:02.350 --> 00:00:05.420\n现在只要29.9元。\n\n"
	assert.Equal(t, expected, string(data))
}

func TestDoubaoJSONFormatter(t *testing.T) {
	formatter := &DoubaoJSONFormatter{}
	result := sampleASRResult()

	data, ct, err := formatter.Format(result, ModelDoubaoASRBigModel)
	require.NoError(t, err)
	assert.Equal(t, "application/json", ct)

	var res VolcengineASRResult
	err = common.Unmarshal(data, &res)
	require.NoError(t, err)
	assert.Equal(t, result.Text, res.Text)
	assert.Len(t, res.Utterances, 2)
}

func TestGetASRFormatter(t *testing.T) {
	assert.IsType(t, &JSONFormatter{}, GetASRFormatter("json"))
	assert.IsType(t, &JSONFormatter{}, GetASRFormatter(""))
	assert.IsType(t, &TextFormatter{}, GetASRFormatter("text"))
	assert.IsType(t, &VerboseJSONFormatter{}, GetASRFormatter("verbose_json"))
	assert.IsType(t, &SRTFormatter{}, GetASRFormatter("srt"))
	assert.IsType(t, &VTTFormatter{}, GetASRFormatter("vtt"))
	assert.IsType(t, &DoubaoJSONFormatter{}, GetASRFormatter("doubao_json"))
	assert.IsType(t, &JSONFormatter{}, GetASRFormatter("unknown_format"))
}

func TestFormatterEmptyResult(t *testing.T) {
	empty := &VolcengineASRResult{}

	// JSON
	data, _, err := (&JSONFormatter{}).Format(empty, "")
	require.NoError(t, err)
	assert.True(t, strings.Contains(string(data), `"text":""`))

	// Text
	data, _, err = (&TextFormatter{}).Format(empty, "")
	require.NoError(t, err)
	assert.Equal(t, "", string(data))

	// SRT
	data, _, err = (&SRTFormatter{}).Format(empty, "")
	require.NoError(t, err)
	assert.Equal(t, "", string(data))

	// VTT
	data, _, err = (&VTTFormatter{}).Format(empty, "")
	require.NoError(t, err)
	assert.Equal(t, "WEBVTT\n\n", string(data))
}
