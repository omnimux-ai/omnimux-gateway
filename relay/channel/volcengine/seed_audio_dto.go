package volcengine

// SeedAudioV3Request 火山引擎 seed-audio-1.0 官方请求体
type SeedAudioV3Request struct {
	Model       string               `json:"model"`
	TextPrompt  string               `json:"text_prompt"`
	References  []SeedAudioReference `json:"references,omitempty"`
	AudioConfig SeedAudioConfig      `json:"audio_config"`
	Watermark   *SeedAudioWatermark  `json:"watermark,omitempty"`
}

// SeedAudioReference 音色与参考音频配置
type SeedAudioReference struct {
	Speaker   string `json:"speaker,omitempty"`
	AudioURL  string `json:"audio_url,omitempty"`
	AudioData string `json:"audio_data,omitempty"` // Base64 编码音频
	ImageURL  string `json:"image_url,omitempty"`  // 可选形象参考
	ImageData string `json:"image_data,omitempty"` // 可选 Base64 形象
}

// SeedAudioConfig 目标音频生成参数
type SeedAudioConfig struct {
	Format         string   `json:"format,omitempty"`          // mp3, wav, pcm, ogg_opus
	SampleRate     int      `json:"sample_rate,omitempty"`     // 24000, 16000
	SpeechRate     *float64 `json:"speech_rate,omitempty"`     // 语速倍率 [0.5, 2.0]
	LoudnessRate   *float64 `json:"loudness_rate,omitempty"`   // 音量倍率 [0.5, 2.0]
	PitchRate      *float64 `json:"pitch_rate,omitempty"`      // 音高倍率 [0.5, 2.0]
	EnableSubtitle bool     `json:"enable_subtitle,omitempty"` // 是否返回字级时间戳字幕
}

// SeedAudioWatermark 水印参数
type SeedAudioWatermark struct {
	WatermarkType int `json:"watermark_type,omitempty"`
}

// SeedAudioV3Response 火山引擎 seed-audio-1.0 官方响应体
type SeedAudioV3Response struct {
	Audio            string             `json:"audio"`                       // Base64 编码音频
	Duration         float64            `json:"duration"`                    // 音频时长（秒）
	OriginalDuration float64            `json:"original_duration,omitempty"` // 计费原始时长（秒）
	URL              string             `json:"url,omitempty"`               // 2小时有效试听/下载直链
	Subtitle         *SeedAudioSubtitle `json:"subtitle,omitempty"`          // 字级时间戳
	Code             int                `json:"code,omitempty"`              // 状态码
	Message          string             `json:"message,omitempty"`           // 错误信息
}

// SeedAudioSubtitle 字幕结构
type SeedAudioSubtitle struct {
	Sentences []SeedAudioSentence `json:"sentences,omitempty"`
}

// SeedAudioSentence 句子级与字词时间戳
type SeedAudioSentence struct {
	Text      string          `json:"text"`
	StartTime float64         `json:"start_time"`
	EndTime   float64         `json:"end_time"`
	Words     []SeedAudioWord `json:"words,omitempty"`
}

// SeedAudioWord 单字/词级时间戳
type SeedAudioWord struct {
	Word      string  `json:"word"`
	StartTime float64 `json:"start_time"`
	EndTime   float64 `json:"end_time"`
}
