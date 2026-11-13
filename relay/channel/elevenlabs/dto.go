package elevenlabs

type VoiceSettings struct {
	Stability       float64 `json:"stability,omitempty"`
	SimilarityBoost float64 `json:"similarity_boost,omitempty"`
	Style           float64 `json:"style,omitempty"`
	UseSpeakerBoost bool    `json:"use_speaker_boost,omitempty"`
}

type TTSRequest struct {
	Text          string         `json:"text"`
	ModelID       string         `json:"model_id,omitempty"`
	VoiceSettings *VoiceSettings `json:"voice_settings,omitempty"`
}

type ErrorDetail struct {
	Status  any    `json:"status,omitempty"`
	Message string `json:"message,omitempty"`
}

type ErrorResponse struct {
	Detail ErrorDetail `json:"detail"`
}
