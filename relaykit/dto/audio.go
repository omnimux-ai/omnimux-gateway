package dto

import (
	"encoding/json"
	"net/http"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/relaykit/types"
)

const (
	// VolcengineTTS3MaxBillableChars is the official per-request billable
	// character limit for Doubao TTS 3.0 (Han characters count as 2, others 1).
	VolcengineTTS3MaxBillableChars = 10000
)

type AudioRequest struct {
	Model          string          `json:"model" form:"model"`
	Input          string          `json:"input" form:"input"`
	Voice          string          `json:"voice" form:"voice"`
	Instructions   string          `json:"instructions,omitempty" form:"instructions"`
	ResponseFormat string          `json:"response_format,omitempty" form:"response_format"`
	Speed          *float64        `json:"speed,omitempty" form:"speed"`
	StreamFormat   string          `json:"stream_format,omitempty" form:"stream_format"`
	Metadata       json.RawMessage `json:"metadata,omitempty"`

	// ASR / URL-first 扩展入参
	URL      string `json:"url,omitempty" form:"url"`
	AudioURL string `json:"audio_url,omitempty" form:"audio_url"`
	Prompt   string `json:"prompt,omitempty" form:"prompt"`
	Format   string `json:"format,omitempty" form:"format"`

	// ASR 豆包扩展控制
	ResourceId             string `json:"resource_id,omitempty" form:"resource_id"`
	EnableITN              *bool  `json:"enable_itn,omitempty" form:"enable_itn"`
	EnablePunc             *bool  `json:"enable_punc,omitempty" form:"enable_punc"`
	EnableDDC              *bool  `json:"enable_ddc,omitempty" form:"enable_ddc"`
	EnableSpeakerInfo      *bool  `json:"enable_speaker_info,omitempty" form:"enable_speaker_info"`
	SSDVersion             string `json:"ssd_version,omitempty" form:"ssd_version"`
	EnableEmotionDetection *bool  `json:"enable_emotion_detection,omitempty" form:"enable_emotion_detection"`

	// fork: 火山引擎豆包语音合成 3.0（seed-tts-3.0）扩展入参
	SSML                    string          `json:"ssml,omitempty" form:"ssml"`
	LanguageCode            string          `json:"language_code,omitempty" form:"language_code"`
	OutputFormat            json.RawMessage `json:"output_format,omitempty"`
	Volume                  *float64        `json:"volume,omitempty" form:"volume"`
	Pitch                   *int            `json:"pitch,omitempty" form:"pitch"`
	VoiceSettings           json.RawMessage `json:"voice_settings,omitempty"`
	EnableSubtitle          *bool           `json:"enable_subtitle,omitempty" form:"enable_subtitle"`
	PronunciationDictionary json.RawMessage `json:"pronunciation_dictionary,omitempty"`
	SessionID               string          `json:"session_id,omitempty" form:"session_id"`
	Watermark               json.RawMessage `json:"watermark,omitempty"`
	TextNormalization       *bool           `json:"text_normalization,omitempty" form:"text_normalization"`
	EnableMarkdownFilter    *bool           `json:"enable_markdown_filter,omitempty" form:"enable_markdown_filter"`
	SilenceDuration         *int            `json:"silence_duration,omitempty" form:"silence_duration"`
	// vllm-omini
	TaskType                json.RawMessage `json:"task_type,omitempty"`
	Language                json.RawMessage `json:"language,omitempty"`
	RefAudio                json.RawMessage `json:"ref_audio,omitempty"`
	RefText                 json.RawMessage `json:"ref_text,omitempty"`
	XVectorOnlyMode         json.RawMessage `json:"x_vector_only_mode,omitempty"`
	MaxNewTokens            json.RawMessage `json:"max_new_tokens,omitempty"`
	InitialCodecChunkFrames json.RawMessage `json:"initial_codec_chunk_frames,omitempty"`
	// TODO：ensure that the logic remains correct after the stream is started.
	//Stream                  json.RawMessage `json:"stream,omitempty"`
}

// IsVolcengineTTS3Model reports whether modelName identifies Volcengine Doubao TTS 3.0.
func IsVolcengineTTS3Model(modelName string) bool {
	lower := strings.ToLower(strings.TrimSpace(modelName))
	return lower == "seed-tts-3.0" ||
		lower == "doubao-seed-tts-3.0" ||
		strings.HasPrefix(lower, "seed-tts-3") ||
		strings.HasPrefix(lower, "doubao-seed-tts-3")
}

// CountVolcengineTTS3BillableChars counts billable characters according to
// Volcengine Doubao TTS 3.0 rules: Han ideographs (including Japanese kanji)
// count as 2 characters; all other runes count as 1.
func CountVolcengineTTS3BillableChars(text string) int {
	total := 0
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			total += 2
		} else {
			total++
		}
	}
	return total
}

func (r *AudioRequest) GetURL() string {
	if r.URL != "" {
		return r.URL
	}
	return r.AudioURL
}

func (r *AudioRequest) GetTokenCountMeta() *types.TokenCountMeta {
	combineText := r.Input
	if strings.TrimSpace(combineText) == "" && strings.TrimSpace(r.SSML) != "" {
		combineText = r.SSML
	}
	meta := &types.TokenCountMeta{
		CombineText: combineText,
		TokenType:   types.TokenTypeTextNumber,
	}
	if strings.Contains(r.Model, "gpt") {
		meta.TokenType = types.TokenTypeTokenizer
	}
	return meta
}

func (r *AudioRequest) IsStream(c *http.Request) bool {
	return r.StreamFormat == "sse"
}

func (r *AudioRequest) SetModelName(modelName string) {
	if modelName != "" {
		r.Model = modelName
	}
}

type AudioResponse struct {
	Text string `json:"text"`
}

type WhisperVerboseJSONResponse struct {
	Task     string    `json:"task,omitempty"`
	Language string    `json:"language,omitempty"`
	Duration float64   `json:"duration,omitempty"`
	Text     string    `json:"text,omitempty"`
	Segments []Segment `json:"segments,omitempty"`
}

type Segment struct {
	Id               int     `json:"id"`
	Seek             int     `json:"seek"`
	Start            float64 `json:"start"`
	End              float64 `json:"end"`
	Text             string  `json:"text"`
	Tokens           []int   `json:"tokens"`
	Temperature      float64 `json:"temperature"`
	AvgLogprob       float64 `json:"avg_logprob"`
	CompressionRatio float64 `json:"compression_ratio"`
	NoSpeechProb     float64 `json:"no_speech_prob"`
	Speaker          string  `json:"speaker,omitempty"`
	Emotion          string  `json:"emotion,omitempty"`
	SpeechRate       float64 `json:"speech_rate,omitempty"`
	Volume           float64 `json:"volume,omitempty"`
}
