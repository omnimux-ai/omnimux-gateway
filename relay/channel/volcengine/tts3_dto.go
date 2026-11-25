package volcengine

// TTS3Request is the upstream request body for Volcengine Doubao TTS 3.0
// HTTP unidirectional streaming synthesis:
// POST https://voice.cn-beijing.volces.com/api/v3/text-to-speech/{voice_id}/streaming
type TTS3Request struct {
	Text                    string                       `json:"text,omitempty"`
	SSML                    string                       `json:"ssml,omitempty"`
	ModelID                 string                       `json:"model_id,omitempty"`
	LanguageCode            string                       `json:"language_code,omitempty"`
	OutputFormat            *TTS3OutputFormat            `json:"output_format,omitempty"`
	VoiceSettings           *TTS3VoiceSettings           `json:"voice_settings,omitempty"`
	EnableSubtitle          *bool                        `json:"enable_subtitle,omitempty"`
	PronunciationDictionary *TTS3PronunciationDictionary `json:"pronunciation_dictionary,omitempty"`
	SessionID               string                       `json:"session_id,omitempty"`
	Watermark               *TTS3Watermark               `json:"watermark,omitempty"`
	TextNormalization       *bool                        `json:"text_normalization,omitempty"`
	EnableMarkdownFilter    *bool                        `json:"enable_markdown_filter,omitempty"`
	SilenceDuration         *int                         `json:"silence_duration,omitempty"`
}

// TTS3OutputFormat configures output encoding, sample rate, and bit rate.
type TTS3OutputFormat struct {
	Format     string `json:"format,omitempty"`
	SampleRate *int   `json:"sample_rate,omitempty"`
	BitRate    *int   `json:"bit_rate,omitempty"`
}

// TTS3VoiceSettings configures voice delivery and expressive instructions.
type TTS3VoiceSettings struct {
	Speed           *float64 `json:"speed,omitempty"`
	Volume          *float64 `json:"volume,omitempty"`
	Pitch           *int     `json:"pitch,omitempty"`
	Instructions    []string `json:"instructions,omitempty"`
	SimilarityBoost *bool    `json:"similarity_boost,omitempty"`
	DialectBoost    *bool    `json:"dialect_boost,omitempty"`
}

// TTS3PronunciationDictionary configures pronunciation overrides and text rewrites.
type TTS3PronunciationDictionary struct {
	Tone []string `json:"tone,omitempty"`
}

// TTS3Watermark configures explicit and implicit AIGC watermarks.
type TTS3Watermark struct {
	AigcWatermark *bool             `json:"aigc_watermark,omitempty"`
	AigcMetadata  *TTS3AIGCMetadata `json:"aigc_metadata,omitempty"`
}

// TTS3AIGCMetadata carries implicit AIGC watermark metadata.
type TTS3AIGCMetadata struct {
	Enable            *bool  `json:"enable,omitempty"`
	ContentProducer   string `json:"content_producer,omitempty"`
	ProduceID         string `json:"produce_id,omitempty"`
	ContentPropagator string `json:"content_propagator,omitempty"`
	PropagateID       string `json:"propagate_id,omitempty"`
}

// TTS3StreamEvent represents one JSONL event emitted by Doubao TTS 3.0 HTTP streaming.
type TTS3StreamEvent struct {
	Event     string              `json:"event"`
	SessionID string              `json:"session_id,omitempty"`
	Audio     string              `json:"audio,omitempty"`
	Text      string              `json:"text,omitempty"`
	Words     []TTS3WordAlignment `json:"words,omitempty"`
	IsFinal   bool                `json:"is_final,omitempty"`
	Reason    string              `json:"reason,omitempty"`
	Error     *TTS3ErrorPayload   `json:"error,omitempty"`
}

// TTS3WordAlignment represents word-level timing returned by alignment events.
type TTS3WordAlignment struct {
	Word      string  `json:"word"`
	StartTime float64 `json:"startTime"`
	EndTime   float64 `json:"endTime"`
	Phonemes  []any   `json:"phonemes,omitempty"`
}

// TTS3ErrorPayload is the structured upstream error object inside an error event.
type TTS3ErrorPayload struct {
	Type      string  `json:"type"`
	Code      string  `json:"code"`
	Message   string  `json:"message"`
	Param     *string `json:"param,omitempty"`
	RequestID string  `json:"request_id,omitempty"`
}

// TTS3AlignmentSegment aggregates one alignment event for structured JSON responses.
type TTS3AlignmentSegment struct {
	Text  string              `json:"text"`
	Words []TTS3WordAlignment `json:"words,omitempty"`
}

// TTS3AggregatedResponse is returned when the caller requests json/verbose_json
// or Accept: application/json so both base64 audio and alignments are available.
type TTS3AggregatedResponse struct {
	SessionID  string                 `json:"session_id,omitempty"`
	Format     string                 `json:"format"`
	Audio      string                 `json:"audio"`
	Alignments []TTS3AlignmentSegment `json:"alignments,omitempty"`
}
