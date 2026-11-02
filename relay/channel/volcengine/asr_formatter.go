package volcengine

import (
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
)

// IASRFormatter ASR 响应格式转换接口
type IASRFormatter interface {
	Format(result *VolcengineASRResult, originModel string) (data []byte, contentType string, err error)
}

// JSONFormatter 标准 OpenAI json 格式：{"text": "..."}
type JSONFormatter struct{}

func (f *JSONFormatter) Format(result *VolcengineASRResult, originModel string) ([]byte, string, error) {
	text := ""
	if result != nil {
		text = result.Text
	}
	resp := dto.AudioResponse{
		Text: text,
	}
	data, err := common.Marshal(resp)
	if err != nil {
		return nil, "", fmt.Errorf("marshal json response failed: %w", err)
	}
	return data, "application/json", nil
}

// TextFormatter 纯文本格式：输出纯字符串
type TextFormatter struct{}

func (f *TextFormatter) Format(result *VolcengineASRResult, originModel string) ([]byte, string, error) {
	text := ""
	if result != nil {
		text = result.Text
	}
	return []byte(text), "text/plain; charset=utf-8", nil
}

// VerboseJSONFormatter OpenAI WhisperVerboseJSONResponse 详细 JSON 格式
type VerboseJSONFormatter struct{}

func (f *VerboseJSONFormatter) Format(result *VolcengineASRResult, originModel string) ([]byte, string, error) {
	if result == nil {
		result = &VolcengineASRResult{}
	}

	var duration float64
	segments := make([]dto.Segment, 0, len(result.Utterances))

	for i, u := range result.Utterances {
		speechRate := u.Additions.SpeechRate
		if speechRate == 0 && u.Additions.Speed != 0 {
			speechRate = u.Additions.Speed
		}
		startSec := float64(u.StartTime) / 1000.0
		endSec := float64(u.EndTime) / 1000.0

		seg := dto.Segment{
			Id:               i,
			Seek:             int(u.StartTime / 10),
			Start:            startSec,
			End:              endSec,
			Text:             u.Text,
			Tokens:           []int{},
			Temperature:      0.0,
			AvgLogprob:       0.0,
			CompressionRatio: 1.0,
			NoSpeechProb:     0.0,
			Speaker:          u.Additions.Speaker,
			Emotion:          u.Additions.Emotion,
			SpeechRate:       speechRate,
			Volume:           u.Additions.Volume,
		}
		segments = append(segments, seg)
	}

	if len(result.Utterances) > 0 {
		duration = float64(result.Utterances[len(result.Utterances)-1].EndTime) / 1000.0
	}

	verboseResp := dto.WhisperVerboseJSONResponse{
		Task:     "transcribe",
		Language: "chinese",
		Duration: duration,
		Text:     result.Text,
		Segments: segments,
	}

	data, err := common.Marshal(verboseResp)
	if err != nil {
		return nil, "", fmt.Errorf("marshal verbose_json response failed: %w", err)
	}
	return data, "application/json", nil
}

// SRTFormatter SubRip 字幕格式 (HH:MM:SS,mmm --> HH:MM:SS,mmm)
type SRTFormatter struct{}

func (f *SRTFormatter) Format(result *VolcengineASRResult, originModel string) ([]byte, string, error) {
	if result == nil || len(result.Utterances) == 0 {
		return []byte(""), "text/plain; charset=utf-8", nil
	}

	var sb strings.Builder
	for i, u := range result.Utterances {
		sb.WriteString(fmt.Sprintf("%d\n", i+1))
		sb.WriteString(fmt.Sprintf("%s --> %s\n", formatSRTTimestamp(u.StartTime), formatSRTTimestamp(u.EndTime)))
		sb.WriteString(fmt.Sprintf("%s\n\n", u.Text))
	}

	return []byte(sb.String()), "text/plain; charset=utf-8", nil
}

// VTTFormatter WebVTT 字幕格式 (首行 WEBVTT\n\n，时间轴 HH:MM:SS.mmm --> HH:MM:SS.mmm)
type VTTFormatter struct{}

func (f *VTTFormatter) Format(result *VolcengineASRResult, originModel string) ([]byte, string, error) {
	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")

	if result != nil {
		for _, u := range result.Utterances {
			sb.WriteString(fmt.Sprintf("%s --> %s\n", formatVTTTimestamp(u.StartTime), formatVTTTimestamp(u.EndTime)))
			sb.WriteString(fmt.Sprintf("%s\n\n", u.Text))
		}
	}

	return []byte(sb.String()), "text/vtt; charset=utf-8", nil
}

// DoubaoJSONFormatter 1:1 透传火山引擎原生出参格式
type DoubaoJSONFormatter struct{}

func (f *DoubaoJSONFormatter) Format(result *VolcengineASRResult, originModel string) ([]byte, string, error) {
	data, err := common.Marshal(result)
	if err != nil {
		return nil, "", fmt.Errorf("marshal doubao_json response failed: %w", err)
	}
	return data, "application/json", nil
}

// GetASRFormatter 工厂函数：依据 response_format 派发对应的 Formatter
func GetASRFormatter(format string) IASRFormatter {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "text":
		return &TextFormatter{}
	case "verbose_json":
		return &VerboseJSONFormatter{}
	case "srt":
		return &SRTFormatter{}
	case "vtt":
		return &VTTFormatter{}
	case "doubao_json":
		return &DoubaoJSONFormatter{}
	case "json":
		fallthrough
	default:
		return &JSONFormatter{}
	}
}

// formatSRTTimestamp 格式化毫秒为 SRT 时间轴格式 HH:MM:SS,mmm
func formatSRTTimestamp(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	hours := ms / 3600000
	rem := ms % 3600000
	minutes := rem / 60000
	rem = rem % 60000
	seconds := rem / 1000
	millis := rem % 1000
	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

// formatVTTTimestamp 格式化毫秒为 VTT 时间轴格式 HH:MM:SS.mmm
func formatVTTTimestamp(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	hours := ms / 3600000
	rem := ms % 3600000
	minutes := rem / 60000
	rem = rem % 60000
	seconds := rem / 1000
	millis := rem % 1000
	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
}
