package fal

const (
	ModelMinimaxH3Max                  = "minimax/h3-max"
	ModelMinimaxH3MaxTextToVideo       = "minimax/h3-max/text-to-video"
	ModelMinimaxH3MaxImageToVideo      = "minimax/h3-max/image-to-video"
	ModelMinimaxH3MaxReferenceToVideo  = "minimax/h3-max/reference-to-video"
	ModelMinimaxH3MaxTurbo             = "minimax/h3-max-turbo"
	ModelMinimaxH3MaxTurboTextToVideo  = "minimax/h3-max-turbo/text-to-video"
	ModelMinimaxH3MaxTurboImageToVideo = "minimax/h3-max-turbo/image-to-video"
	ModelMinimaxH3MaxLipSyncImageToVideo = "minimax/h3-max/lip-sync/image-to-video"
)

var ModelList = []string{
	ModelMinimaxH3Max,
	ModelMinimaxH3MaxTextToVideo,
	ModelMinimaxH3MaxImageToVideo,
	ModelMinimaxH3MaxReferenceToVideo,
	ModelMinimaxH3MaxTurbo,
	ModelMinimaxH3MaxTurboTextToVideo,
	ModelMinimaxH3MaxTurboImageToVideo,
	ModelMinimaxH3MaxLipSyncImageToVideo,
}

const (
	ChannelName    = "fal.ai"
	DefaultBaseURL = "https://queue.fal.run"
)

// fal.ai Queue Statuses
const (
	StatusInQueue    = "IN_QUEUE"
	StatusInProgress = "IN_PROGRESS"
	StatusCompleted  = "COMPLETED"
	StatusFailed     = "FAILED"
)

// Aspect Ratios
const (
	AspectRatio16x9 = "16:9"
	AspectRatio9x16 = "9:16"
	AspectRatio1x1  = "1:1"
	AspectRatio4x3  = "4:3"
	AspectRatio21x9 = "21:9"
)

// Resolutions
const (
	Resolution1080P = "1080P"
	Resolution768P  = "768P"
	Resolution480P  = "480P"
	Resolution1080p = "1080P"
	Resolution720p  = "768P"
)

// Durations
const (
	Duration5  = "5"
	Duration10 = "10"
)
