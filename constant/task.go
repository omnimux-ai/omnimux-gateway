package constant

type TaskPlatform string

const (
	TaskPlatformSuno       TaskPlatform = "suno"
	TaskPlatformMidjourney              = "mj"

	// TaskPlatformVoiceClone is the task platform for voice training. It is a
	// dedicated key rather than the channel type number: the VolcEngine channel
	// type already resolves to a factory plugin, and a voice-training task must
	// keep resolving to its own adaptor when it is polled later.
	TaskPlatformVoiceClone = "volcvoice"
)

const (
	TaskActionImageToVideo     = "image_to_video"
	TaskActionTextToVideo      = "text_to_video"
	TaskActionFirstTailToVideo = "first_tail_to_video"
	TaskActionReferenceToVideo = "reference_to_video"
	TaskActionRemix            = "remix"
	// TaskActionImageEdit marks an image request carrying reference material.
	// Adaptors that understand a reference share this value so the action string
	// stays comparable across channel types.
	TaskActionImageEdit = "imageEdit"

	// TaskActionVoiceClone marks a voice-training task. The task row is also the
	// ownership record for the resulting voice, so this action is what makes a
	// user's trained voices enumerable without a dedicated table.
	TaskActionVoiceClone = "voice_clone"

	// Legacy aliases
	TaskActionGenerate          = "generate"
	TaskActionTextGenerate      = "textGenerate"
	TaskActionFirstTailGenerate = "firstTailGenerate"
	TaskActionReferenceGenerate = "referenceGenerate"
)

var legacyTaskActionAliases = map[string]string{
	"generate":          TaskActionImageToVideo,
	"textGenerate":      TaskActionTextToVideo,
	"firstTailGenerate": TaskActionFirstTailToVideo,
	"referenceGenerate": TaskActionReferenceToVideo,
	"remixGenerate":     TaskActionRemix,
}

// TaskPluginEnabled is the master switch for the whole task-plugin system.
// When disabled, factory and override plugins both stop serving.
var TaskPluginEnabled = true

// NormalizeTaskAction maps persisted legacy action names to the canonical task
// action vocabulary. Unknown platform-specific actions pass through unchanged.
func NormalizeTaskAction(action string) string {
	if canonical, ok := legacyTaskActionAliases[action]; ok {
		return canonical
	}
	return action
}
