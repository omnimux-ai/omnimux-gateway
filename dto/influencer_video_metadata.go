package dto

// InfluencerVideoMetadata describes the actual reference media, not output
// settings. It is declared by the browser for influencer-reference uploads and
// persisted with the file record so later consumers can reuse the facts without
// reading the media again.
type InfluencerVideoMetadata struct {
	Duration      float64 `json:"duration"`
	VideoDuration float64 `json:"video_duration"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	FrameRate     float64 `json:"frame_rate"`
	VideoCodec    string  `json:"video_codec"`
	AudioCodec    string  `json:"audio_codec"`
	Container     string  `json:"container"`
	SizeBytes     int64   `json:"size_bytes"`
}
