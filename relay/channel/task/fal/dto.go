package fal

// FalVideoSubmitPayload represents the upstream JSON payload for fal.ai Minimax H3 models.
type FalVideoSubmitPayload struct {
	Prompt             string   `json:"prompt"`
	ImageURL           string   `json:"image_url,omitempty"`
	ReferenceImageURLs []string `json:"reference_image_urls,omitempty"`
	ReferenceVideoURLs []string `json:"reference_video_urls,omitempty"`
	ReferenceAudioURLs []string `json:"reference_audio_urls,omitempty"`
	Duration           string   `json:"duration,omitempty"`         // "5" or "10"
	AspectRatio        string   `json:"aspect_ratio,omitempty"`     // "16:9", "9:16", "1:1", "4:3", "21:9"
	Resolution         string   `json:"resolution,omitempty"`       // "480P", "768P", or "1080P"
	PromptOptimizer    bool     `json:"prompt_optimizer,omitempty"` // default true
}

// FalLipSyncSubmitPayload represents the upstream JSON payload for fal.ai Minimax H3 Lip-Sync models.
type FalLipSyncSubmitPayload struct {
	ImageURL            string `json:"image_url"`
	AudioURL            string `json:"audio_url"`
	Resolution          string `json:"resolution,omitempty"` // "480P", "768P", "1080P", "2K"
	EnableTranscription bool   `json:"enable_transcription,omitempty"`
	Seed                *int64 `json:"seed,omitempty"`
	EnableSafetyChecker *bool  `json:"enable_safety_checker,omitempty"`
}

// FalSubmitResponse represents the response from POST https://queue.fal.run/{model_id}.
type FalSubmitResponse struct {
	RequestID   string          `json:"request_id"`
	Status      string          `json:"status"`
	ResponseURL string          `json:"response_url"`
	StatusURL   string          `json:"status_url"`
	CancelURL   string          `json:"cancel_url,omitempty"`
	Detail      any             `json:"detail,omitempty"`
	Error       *FalErrorDetail `json:"error,omitempty"`
}

// FalStatusResponse represents the polling response from GET .../requests/{request_id}/status.
type FalStatusResponse struct {
	Status        string          `json:"status"` // IN_QUEUE, IN_PROGRESS, COMPLETED, FAILED
	QueuePosition int             `json:"queue_position,omitempty"`
	ResponseURL   string          `json:"response_url,omitempty"`
	Logs          []any           `json:"logs,omitempty"`
	Error         *FalErrorDetail `json:"error,omitempty"`
	Detail        any             `json:"detail,omitempty"`
}

// FalResultResponse represents the response from GET .../requests/{request_id}.
type FalResultResponse struct {
	Video  *FalVideoFile   `json:"video,omitempty"`
	Usage  *FalUsage       `json:"usage,omitempty"`
	Detail any             `json:"detail,omitempty"`
	Error  *FalErrorDetail `json:"error,omitempty"`
}

// FalVideoFile represents the output video file metadata.
type FalVideoFile struct {
	URL         string `json:"url"`
	ContentType string `json:"content_type,omitempty"`
	FileName    string `json:"file_name,omitempty"`
	FileSize    int64  `json:"file_size,omitempty"`
}

// FalErrorDetail represents error information returned by fal.ai.
type FalErrorDetail struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// FalUsage represents execution metrics.
type FalUsage struct {
	ExecutionTime float64 `json:"execution_time,omitempty"`
}
