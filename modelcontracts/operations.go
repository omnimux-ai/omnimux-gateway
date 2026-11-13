package modelcontracts

import (
	"fmt"
	"slices"
	"strings"

	"github.com/QuantumNous/new-api/common"
)

// SchemaVersion is the published contract schema. It names the shape of the
// payload, not the governance spec: a spec without any v2 block is still a
// valid v1 authoring input and projects to the same v2 document.
const SchemaVersion = "2.0"

// Operation is one mode a model can actually execute through this gateway.
//
// Executable is the honesty bit: an operation listed from vendor documentation
// but with no verified gateway path is published with Executable=false and no
// invented inputs or parameters.
type Operation struct {
	ID         string          `json:"id"`
	Executable bool            `json:"executable"`
	Inputs     []Input         `json:"inputs"`
	Parameters []Parameter     `json:"parameters"`
	Output     OperationOutput `json:"output"`
}

// Input is one input slot of an operation. An absent constraint means "not
// established"; callers must not substitute a default of their own.
type Input struct {
	Slot           string   `json:"slot"`
	Type           string   `json:"type"`
	Role           string   `json:"role,omitempty"`
	Required       bool     `json:"required"`
	Min            *int     `json:"min,omitempty"`
	Max            *int     `json:"max,omitempty"`
	AllowedMimes   []string `json:"allowedMimes,omitempty"`
	MaxSizeMb      *float64 `json:"maxSizeMb,omitempty"`
	MinDurationSec *float64 `json:"minDurationSec,omitempty"`
	MaxDurationSec *float64 `json:"maxDurationSec,omitempty"`
}

// Parameter is one request parameter the operation accepts. Enum is []any
// because the governance block allows string, number and boolean enums.
type Parameter struct {
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Enum        []any        `json:"enum,omitempty"`
	Minimum     *float64     `json:"minimum,omitempty"`
	Maximum     *float64     `json:"maximum,omitempty"`
	Step        *float64     `json:"step,omitempty"`
	Default     any          `json:"default,omitempty"`
	Fixed       any          `json:"fixed,omitempty"`
	LimitSource *LimitSource `json:"limitSource,omitempty"`
}

// LimitSource records how a parameter bound was established. The published
// contract keeps the same kind vocabulary as the governance spec.
type LimitSource struct {
	Kind string `json:"kind"`
	Note string `json:"note,omitempty"`
}

// OperationOutput is the shape one operation produces.
type OperationOutput struct {
	Type    string   `json:"type"`
	Formats []string `json:"formats,omitempty"`
	Sizes   []string `json:"sizes,omitempty"`
}

// GatewaySpec is the optional governance block that marks a model as
// gateway-verified: the operations listed here are the ones this gateway
// actually accepts, which may be narrower than the vendor's own capability
// list.
type GatewaySpec struct {
	Profile     string   `json:"profile"`
	Operations  []string `json:"operations"`
	SourceClaim string   `json:"source_claim,omitempty"`
}

// ExecutionSpec is the optional governance block for a model that is not
// backed by a verified gateway operation set. `stub` means catalogued but not
// executable; `unavailable` means not offered on any gateway line.
type ExecutionSpec struct {
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// ParameterSpec mirrors one entry of the optional governance parameters block.
type ParameterSpec struct {
	Name        string       `json:"name"`
	Type        string       `json:"type"`
	Enum        []any        `json:"enum,omitempty"`
	Minimum     *float64     `json:"minimum,omitempty"`
	Maximum     *float64     `json:"maximum,omitempty"`
	Step        *float64     `json:"step,omitempty"`
	Default     any          `json:"default,omitempty"`
	Fixed       any          `json:"fixed,omitempty"`
	AppliesTo   []string     `json:"applies_to,omitempty"`
	LimitSource *LimitSource `json:"limit_source,omitempty"`
}

// UnmarshalJSON accepts both the v1 string list and the v2 object list, so a
// payload written before schema v2 (including one served through
// OMNIMUX_MODEL_SPECS_DIR) still parses instead of blanking the catalog.
func (o *Operation) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var id string
		if err := common.Unmarshal(data, &id); err != nil {
			return err
		}
		*o = Operation{ID: id, Inputs: []Input{}, Parameters: []Parameter{}}
		return nil
	}
	// A local alias stops the decoder from calling this method recursively.
	type plainOperation Operation
	var decoded plainOperation
	if err := common.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*o = Operation(decoded)
	return nil
}

// Operation IDs used by the gateway. Video ids come from the apimart and fal
// request validators; family ids mirror the operations each governance spec
// already names.
const (
	operationTextToVideo     = "text_to_video"
	operationFirstFrame      = "first_frame"
	operationEndFrame        = "end_frame"
	operationFirstLastFrame  = "first_last_frame"
	operationVideoMultiRef   = "video_multi_ref"
	operationVideoEdit       = "video_edit"
	operationVideoExtend     = "video_extend"
	operationDocumentToVideo = "document_to_video"
	operationWebpageToVideo  = "webpage_to_video"
	operationDigitalHuman    = "digital_human"
	operationLipSync         = "lip_sync"

	operationChat = "chat"

	operationTextToImage    = "text_to_image"
	operationImageToImage   = "image_to_image"
	operationMultiReference = "multi_reference"

	operationSpeechToText      = "speech_to_text"
	operationTextToSpeech      = "text_to_speech"
	operationVoiceClone        = "voice_clone"
	operationTextToMusic       = "text_to_music"
	operationSoundEffect       = "sound_effect_generation"
	operationEmotionControl    = "emotion_control"
	operationTranslation       = "translation"
	operationLanguageDetection = "language_identification"
)

const (
	profileAPIMartVideo  = "apimart_video"
	profileFalH3Video    = "fal_h3_video"
	profileFalH3LipSync  = "fal_h3_lip_sync"
	profileAutoDLH3Video = "autodl_h3_video"
)

// apimartVideoOperations mirrors operationAllowed in
// relay/channel/task/apimart/video_contract.go. It is deliberately duplicated:
// modelcontracts cannot import relay (the relay packages import the catalog),
// and this table is the bridge that keeps the published executable set honest.
// Adding a model here is a gateway-code change, not a spec-only change.
var apimartVideoOperations = map[string][]string{
	"seedance-2-0": {
		operationTextToVideo, operationFirstFrame, operationFirstLastFrame, operationVideoMultiRef,
	},
	"seedance-2-0-fast": {
		operationTextToVideo, operationFirstFrame, operationFirstLastFrame, operationVideoMultiRef,
	},
	"seedance-2-0-mini": {
		operationTextToVideo, operationFirstFrame, operationFirstLastFrame, operationVideoMultiRef,
	},
	"seedance-2-5": {
		operationTextToVideo, operationFirstFrame, operationFirstLastFrame, operationVideoMultiRef,
		operationVideoEdit, operationVideoExtend,
	},
	"wan-3.0": {
		operationTextToVideo, operationFirstFrame, operationFirstLastFrame, operationVideoMultiRef,
		operationDocumentToVideo, operationWebpageToVideo,
	},
	"minimax-h3": {
		operationTextToVideo, operationFirstFrame, operationEndFrame, operationFirstLastFrame, operationVideoMultiRef,
	},
	"grok-imagine-video-1-5": {
		operationTextToVideo, operationVideoMultiRef,
	},
}

// falH3VideoOperations mirrors the three branches of validateMediaOperation in
// relay/channel/task/fal/media_inputs.go.
var falH3VideoOperations = []string{
	operationTextToVideo, operationFirstFrame, operationVideoMultiRef,
}

// falH3LipSyncOperations mirrors the dedicated lip-sync branch in
// relay/channel/task/fal/payload.go: one portrait image plus one audio track.
var falH3LipSyncOperations = []string{operationLipSync}

// autodlH3VideoOperations mirrors the AutoDL task plugin's H3 workflows. The
// line has no reference-video entry and its lip-sync workflow is the
// digital_human shape.
var autodlH3VideoOperations = []string{
	operationTextToVideo, operationFirstLastFrame, operationVideoMultiRef, operationDigitalHuman,
}

// gatewayProfile is one registered projection template.
type gatewayProfile struct {
	allows func(modelID, operationID string) bool
	build  func(spec specFile, operationID string) (Operation, []string, bool)
}

var gatewayProfiles = map[string]gatewayProfile{
	profileAPIMartVideo: {
		allows: func(modelID, operationID string) bool {
			return slices.Contains(apimartVideoOperations[modelID], operationID)
		},
		build: buildVideoOperation,
	},
	profileFalH3Video: {
		allows: func(_, operationID string) bool {
			return slices.Contains(falH3VideoOperations, operationID)
		},
		build: buildVideoOperation,
	},
	profileFalH3LipSync: {
		allows: func(_, operationID string) bool {
			return slices.Contains(falH3LipSyncOperations, operationID)
		},
		build: buildFalH3LipSyncOperation,
	},
	profileAutoDLH3Video: {
		allows: func(_, operationID string) bool {
			return slices.Contains(autodlH3VideoOperations, operationID)
		},
		build: buildAutoDLH3VideoOperation,
	},
}

// operationIDs returns the v1 compatibility view of an operation list.
func operationIDs(operations []Operation) []string {
	if len(operations) == 0 {
		return nil
	}
	ids := make([]string, len(operations))
	for index, operation := range operations {
		ids[index] = operation.ID
	}
	return ids
}

// projectOperations expands one spec into its published operation objects and
// the operation-level unknown paths the expansion could not establish.
func projectOperations(spec specFile) ([]Operation, []string, error) {
	if spec.Gateway != nil {
		return projectGatewayOperations(spec)
	}
	switch spec.Family {
	case "image":
		operations, unknown := buildImageOperations(spec)
		return operations, unknown, nil
	case "llm":
		return []Operation{buildChatOperation(spec)}, nil, nil
	case "audio":
		operations, unknown := buildAudioOperations(spec)
		return operations, unknown, nil
	case "data":
		operations, unknown := buildDataAPIOperations(spec)
		return operations, unknown, nil
	case "video":
		return fallbackVideoOperations(spec)
	default:
		// A spec without a known family (for example a narrow unit-test
		// fixture) still gets its model-level projection; there is no family
		// vocabulary to invent an operation from.
		return nil, nil, nil
	}
}

// projectGatewayOperations turns an explicit gateway block into executable
// operations. Every failure names the operation so the SpecError wrapper can
// point at the spec file that needs fixing.
func projectGatewayOperations(spec specFile) ([]Operation, []string, error) {
	profile, ok := gatewayProfiles[spec.Gateway.Profile]
	if !ok {
		return nil, nil, fmt.Errorf("unknown gateway profile %q", spec.Gateway.Profile)
	}
	operationIDs := spec.Gateway.Operations
	if len(operationIDs) == 0 {
		return nil, nil, fmt.Errorf("gateway profile %q declares no operations", spec.Gateway.Profile)
	}
	seen := make(map[string]bool, len(operationIDs))
	operations := make([]Operation, 0, len(operationIDs))
	var unknown []string
	for _, operationID := range operationIDs {
		if strings.TrimSpace(operationID) == "" {
			return nil, nil, fmt.Errorf("gateway declares an empty operation id")
		}
		if seen[operationID] {
			return nil, nil, fmt.Errorf("duplicate gateway operation %q", operationID)
		}
		seen[operationID] = true
		operation, operationUnknown, known := profile.build(spec, operationID)
		if !known {
			return nil, nil, fmt.Errorf("unknown operation %q for profile %q", operationID, spec.Gateway.Profile)
		}
		if !profile.allows(spec.ModelID, operationID) {
			return nil, nil, fmt.Errorf("operation %q is not allowed for model %q by profile %q", operationID, spec.ModelID, spec.Gateway.Profile)
		}
		operations = append(operations, operation)
		for _, path := range operationUnknown {
			unknown = append(unknown, "operations."+operationID+"."+path)
		}
	}
	return operations, unknown, nil
}

// fallbackVideoOperations preserves the vendor/list-declared operation names
// for a video model whose gateway path has not been verified. They are marked
// non-executable and their missing input/parameter axes are recorded as
// unknown paths instead of being guessed.
//
// The vendor vocabulary has historical aliases (for example image_to_video and
// reference_to_video); the fallback normalizes them to the executable
// vocabulary so a client can match modes even while executable is false.
func fallbackVideoOperations(spec specFile) ([]Operation, []string, error) {
	operations := make([]Operation, 0, len(spec.Operations))
	var unknown []string
	seen := map[string]bool{}
	for _, declaredID := range spec.Operations {
		operationID := canonicalVideoOperationID(declaredID)
		if operationID == "" || seen[operationID] {
			continue
		}
		seen[operationID] = true
		operations = append(operations, Operation{
			ID:         operationID,
			Executable: false,
			Inputs:     []Input{},
			Parameters: []Parameter{},
			Output:     OperationOutput{Type: "video"},
		})
		unknown = append(unknown, "operations."+operationID+".inputs", "operations."+operationID+".parameters")
	}
	return operations, unknown, nil
}

// canonicalVideoOperationID normalizes the legacy vendor operation names that
// mean an executable mode. Unknown ids are kept verbatim so a new vendor term
// is visible instead of silently dropped.
func canonicalVideoOperationID(operationID string) string {
	switch strings.TrimSpace(operationID) {
	case "image_to_video":
		return operationFirstFrame
	case "reference_to_video":
		return operationVideoMultiRef
	case "first_last_frame_to_video":
		return operationFirstLastFrame
	default:
		return strings.TrimSpace(operationID)
	}
}

// videoOperationInputs is the shared input-slot vocabulary of the video
// profiles. Whether an operation is allowed for a model is the profile's job;
// this table only says what an allowed operation consumes.
var videoOperationInputs = map[string]func(spec specFile) []Input{
	operationTextToVideo: func(spec specFile) []Input {
		return []Input{textPromptInput()}
	},
	operationFirstFrame: func(spec specFile) []Input {
		return []Input{
			textPromptInput(),
			frameInput("first_frame", "first_frame", spec.ReferenceImages),
		}
	},
	operationEndFrame: func(spec specFile) []Input {
		return []Input{
			textPromptInput(),
			frameInput("last_frame", "last_frame", spec.ReferenceImages),
		}
	},
	operationFirstLastFrame: func(spec specFile) []Input {
		return []Input{
			textPromptInput(),
			frameInput("first_frame", "first_frame", spec.ReferenceImages),
			frameInput("last_frame", "last_frame", spec.ReferenceImages),
		}
	},
	operationVideoMultiRef: func(spec specFile) []Input {
		return videoMultiRefInputs(spec)
	},
	operationVideoEdit: func(spec specFile) []Input {
		return []Input{textPromptInput(), sourceVideoInput(spec)}
	},
	operationVideoExtend: func(spec specFile) []Input {
		return []Input{textPromptInput(), sourceVideoInput(spec)}
	},
	operationDocumentToVideo: func(spec specFile) []Input {
		return []Input{
			textPromptInput(),
			{Slot: "file", Type: "file", Required: true},
		}
	},
	operationWebpageToVideo: func(spec specFile) []Input {
		return []Input{
			textPromptInput(),
			{Slot: "url", Type: "url", Required: true},
		}
	},
}

// buildVideoOperation expands one allowed apimart or fal video operation.
func buildVideoOperation(spec specFile, operationID string) (Operation, []string, bool) {
	inputBuilder, ok := videoOperationInputs[operationID]
	if !ok {
		return Operation{}, nil, false
	}
	inputs := inputBuilder(spec)
	parameters, unknown := videoParameters(spec, operationID)
	if operationID == operationVideoMultiRef && len(inputs) == 1 {
		unknown = append(unknown, "inputs.reference_media")
	}
	return Operation{
		ID:         operationID,
		Executable: true,
		Inputs:     inputs,
		Parameters: parameters,
		Output:     OperationOutput{Type: "video", Formats: canonical(spec.OutputFormats)},
	}, unknown, true
}

// buildFalH3LipSyncOperation expands the fal H3 lip-sync route. It consumes a
// portrait image and an audio track; the output length follows the audio, so no
// duration parameter is offered.
func buildFalH3LipSyncOperation(spec specFile, operationID string) (Operation, []string, bool) {
	if operationID != operationLipSync {
		return Operation{}, nil, false
	}
	inputs := []Input{
		fixedReferenceInput("reference_image", "image", spec.ReferenceImages),
		fixedReferenceInput("reference_audio", "audio", spec.ReferenceAudios),
	}
	parameters := []Parameter{}
	if len(spec.Resolutions) > 0 {
		parameters = append(parameters, enumParameter("resolution", stringsToAny(spec.Resolutions), nil))
	}
	return Operation{
		ID:         operationID,
		Executable: true,
		Inputs:     inputs,
		Parameters: parameters,
		Output:     OperationOutput{Type: "video"},
	}, nil, true
}

// buildAutoDLH3VideoOperation expands the AutoDL H3 workflow vocabulary. Its
// video_multi_ref has no reference-video slot, and digital_human is driven by
// one reference image plus one reference audio with no prompt.
func buildAutoDLH3VideoOperation(spec specFile, operationID string) (Operation, []string, bool) {
	var inputs []Input
	switch operationID {
	case operationTextToVideo:
		inputs = []Input{textPromptInput()}
	case operationFirstLastFrame:
		inputs = []Input{
			textPromptInput(),
			frameInput("first_frame", "first_frame", spec.ReferenceImages),
			frameInput("last_frame", "last_frame", spec.ReferenceImages),
		}
	case operationVideoMultiRef:
		inputs = []Input{textPromptInput()}
		if media := spec.ReferenceImages; media != nil && mediaAllowsAny(media.Max) {
			inputs = append(inputs, referenceInput("reference_images", "image", media))
		}
		if media := spec.ReferenceAudios; media != nil && mediaAllowsAny(media.Max) {
			inputs = append(inputs, referenceInput("reference_audios", "audio", media))
		}
	case operationDigitalHuman:
		inputs = []Input{
			fixedReferenceInput("reference_image", "image", spec.ReferenceImages),
			fixedReferenceInput("reference_audio", "audio", spec.ReferenceAudios),
		}
	default:
		return Operation{}, nil, false
	}
	parameters, unknown := videoParameters(spec, operationID)
	return Operation{
		ID:         operationID,
		Executable: true,
		Inputs:     inputs,
		Parameters: parameters,
		Output:     OperationOutput{Type: "video", Formats: canonical(spec.OutputFormats)},
	}, unknown, true
}

// videoParameters expands the model-level envelopes into operation parameters.
// Only axes the caller can actually supply are emitted; a wanted axis the spec
// does not establish is returned as an unknown path for this operation.
func videoParameters(spec specFile, operationID string) ([]Parameter, []string) {
	parameters := make([]Parameter, 0, 6)
	var unknown []string

	if duration, ok := durationParameter(spec, operationID); ok {
		parameters = append(parameters, duration)
	} else {
		unknown = append(unknown, "parameters.duration")
	}

	if len(spec.Resolutions) > 0 {
		parameters = append(parameters, enumParameter("resolution", stringsToAny(spec.Resolutions), nil))
	} else {
		unknown = append(unknown, "parameters.resolution")
	}

	if videoLocksAspectRatio(operationID) {
		parameters = append(parameters, Parameter{
			Name:    "aspect_ratio",
			Type:    "enum",
			Enum:    []any{"adaptive"},
			Default: "adaptive",
			Fixed:   "adaptive",
		})
	} else if len(spec.AspectRatios) > 0 {
		parameters = append(parameters, enumParameter("aspect_ratio", stringsToAny(spec.AspectRatios), nil))
	} else {
		unknown = append(unknown, "parameters.aspect_ratio")
	}

	if len(spec.OutputFormats) > 0 {
		parameters = append(parameters, enumParameter("output_format", stringsToAny(spec.OutputFormats), nil))
	}
	if spec.AudioSupported != nil {
		parameters = append(parameters, Parameter{
			Name:    "generate_audio",
			Type:    "boolean",
			Default: false,
		})
	}
	return mergeParameters(parameters, spec, operationID), unknown
}

// videoLocksAspectRatio lists the operations whose output ratio is fixed to
// adaptive by the channel contract: seedance 2.5 edit/extend and all
// first-frame variants. The spec cannot widen these.
func videoLocksAspectRatio(operationID string) bool {
	switch operationID {
	case operationFirstFrame, operationFirstLastFrame, operationVideoEdit, operationVideoExtend:
		return true
	default:
		return false
	}
}

func durationParameter(spec specFile, operationID string) (Parameter, bool) {
	if operationID == operationVideoEdit {
		// The channel validator rejects any duration other than -1 for video
		// editing; this is a gateway lock, not a vendor range.
		minusOne := -1.0
		return Parameter{
			Name:    "duration",
			Type:    "integer",
			Minimum: &minusOne,
			Maximum: &minusOne,
			Default: -1,
			Fixed:   -1,
		}, true
	}
	envelope := spec.DurationsSeconds
	if envelope == nil {
		return Parameter{}, false
	}
	if envelope.Min == nil && envelope.Max == nil && envelope.Default == nil && len(envelope.Allowed) == 0 {
		return Parameter{}, false
	}
	parameter := Parameter{
		Name:    "duration",
		Type:    "integer",
		Minimum: envelope.Min,
		Maximum: envelope.Max,
	}
	if envelope.Default != nil {
		parameter.Default = *envelope.Default
	}
	if len(envelope.Allowed) > 0 {
		parameter.Enum = float64sToAny(envelope.Allowed)
	} else {
		step := 1.0
		parameter.Step = &step
	}
	return parameter, true
}

func videoMultiRefInputs(spec specFile) []Input {
	inputs := []Input{textPromptInput()}
	if media := spec.ReferenceImages; media != nil && mediaAllowsAny(media.Max) {
		inputs = append(inputs, referenceInput("reference_images", "image", media))
	}
	if media := spec.ReferenceVideos; media != nil && mediaAllowsAny(media.Max) {
		inputs = append(inputs, referenceInput("reference_videos", "video", media))
	}
	if media := spec.ReferenceAudios; media != nil && mediaAllowsAny(media.Max) {
		inputs = append(inputs, referenceInput("reference_audios", "audio", media))
	}
	return inputs
}

// buildImageOperations expands the image operations the spec actually names.
// text_to_image consumes only a prompt; image_to_image adds one reference-image
// slot whose limits come from the spec. No generic image_generation operation
// is invented.
func buildImageOperations(spec specFile) ([]Operation, []string) {
	operations := make([]Operation, 0, len(spec.Operations))
	var unknown []string
	seen := map[string]bool{}
	for _, operationID := range spec.Operations {
		if operationID == "" || seen[operationID] {
			continue
		}
		seen[operationID] = true
		inputs := []Input{textPromptInput()}
		switch operationID {
		case operationTextToImage:
		case operationImageToImage, operationMultiReference:
			if spec.ReferenceImages == nil {
				unknown = append(unknown, "operations."+operationID+".inputs.reference_image")
			} else if mediaAllowsAny(spec.ReferenceImages.Max) {
				inputs = append(inputs, referenceInput("reference_image", "image", spec.ReferenceImages))
			}
		default:
			operations = append(operations, Operation{
				ID:         operationID,
				Executable: false,
				Inputs:     []Input{},
				Parameters: []Parameter{},
				Output:     OperationOutput{Type: "image"},
			})
			unknown = append(unknown, "operations."+operationID+".inputs", "operations."+operationID+".parameters")
			continue
		}
		operations = append(operations, Operation{
			ID:         operationID,
			Executable: true,
			Inputs:     inputs,
			Parameters: mergeParameters(imagePlatformParameters(), spec, operationID),
			Output: OperationOutput{
				Type:    "image",
				Formats: canonical(spec.OutputFormats),
				Sizes:   canonical(spec.OutputSizes),
			},
		})
	}
	return operations, unknown
}

// imagePlatformParameters are the platform's conservative image controls. They
// exist so a spec that never published an image parameter still has an honest
// baseline; a spec's own parameters block overrides them by name.
func imagePlatformParameters() []Parameter {
	return []Parameter{
		{
			Name:        "seed",
			Type:        "integer",
			Minimum:     float64Pointer(0),
			Maximum:     float64Pointer(2147483647),
			Step:        float64Pointer(1),
			Default:     0,
			LimitSource: policySource("平台既有参数表；上游未公布取值范围"),
		},
		{
			Name:        "n",
			Type:        "integer",
			Minimum:     float64Pointer(1),
			Maximum:     float64Pointer(4),
			Step:        float64Pointer(1),
			Default:     1,
			LimitSource: policySource("平台既有参数表；上游未公布数量上限"),
		},
		{
			Name:        "quality",
			Type:        "enum",
			Enum:        []any{"low", "medium", "high"},
			Default:     "medium",
			LimitSource: policySource("前端既有参数表；上游未公布枚举"),
		},
		{
			Name:        "background",
			Type:        "enum",
			Enum:        []any{"opaque", "transparent"},
			Default:     "opaque",
			LimitSource: policySource("前端既有参数表；上游未公布枚举"),
		},
	}
}

func buildChatOperation(spec specFile) Operation {
	inputs := []Input{textPromptInput()}
	seen := map[string]bool{"text": true}
	addSlot := func(slot, slotType string, media *MediaLimits) {
		if seen[slotType] {
			return
		}
		seen[slotType] = true
		input := Input{Slot: slot, Type: slotType}
		if media != nil {
			input.AllowedMimes = canonical(media.Formats)
			input.MaxSizeMb = megabytes(media.MaxBytesEach)
			input.MaxDurationSec = media.MaxSecondsEach
		}
		inputs = append(inputs, input)
	}
	for _, modality := range spec.InputModalities {
		switch modality {
		case "image":
			addSlot("image", "image", spec.ReferenceImages)
		case "video":
			addSlot("video", "video", spec.ReferenceVideos)
		case "audio":
			addSlot("audio", "audio", spec.ReferenceAudios)
		case "pdf":
			addSlot("pdf", "pdf", nil)
		}
	}
	if !seen["image"] && spec.ReferenceImages != nil {
		addSlot("image", "image", spec.ReferenceImages)
	}
	if !seen["video"] && spec.ReferenceVideos != nil {
		addSlot("video", "video", spec.ReferenceVideos)
	}
	if !seen["audio"] && spec.ReferenceAudios != nil {
		addSlot("audio", "audio", spec.ReferenceAudios)
	}
	return Operation{
		ID:         operationChat,
		Executable: true,
		Inputs:     inputs,
		Parameters: mergeParameters(nil, spec, operationChat),
		Output:     OperationOutput{Type: "text"},
	}
}

// buildAudioOperations expands every operation the spec names, mapping the
// vendor/plugin spelling to the gateway's operation vocabulary.
func buildAudioOperations(spec specFile) ([]Operation, []string) {
	operations := make([]Operation, 0, len(spec.Operations))
	var unknown []string
	seen := map[string]bool{}
	for _, declaredID := range spec.Operations {
		operationID := canonicalAudioOperationID(declaredID)
		if operationID == "" || seen[operationID] {
			continue
		}
		seen[operationID] = true
		outputType := "audio"
		if spec.AudioKind == "asr" {
			outputType = "text"
		}
		operations = append(operations, Operation{
			ID:         operationID,
			Executable: true,
			Inputs:     audioInputs(spec, operationID),
			Parameters: mergeParameters(nil, spec, operationID),
			Output: OperationOutput{
				Type:    outputType,
				Formats: canonical(spec.OutputFormats),
			},
		})
	}
	return operations, unknown
}

func canonicalAudioOperationID(operationID string) string {
	switch strings.TrimSpace(operationID) {
	case "transcription":
		return operationSpeechToText
	case "text_to_speech":
		return operationTextToSpeech
	case "voice_clone":
		return operationVoiceClone
	case "music_generation":
		return operationTextToMusic
	case "sound_effect_generation":
		return operationSoundEffect
	case "emotion_control":
		return operationEmotionControl
	case "translation":
		return operationTranslation
	case "language_identification":
		return operationLanguageDetection
	case "reference_audio_generation", "reference_image_generation":
		return strings.TrimSpace(operationID)
	default:
		return strings.TrimSpace(operationID)
	}
}

func audioInputs(spec specFile, operationID string) []Input {
	switch operationID {
	case operationSpeechToText, operationTranslation, operationLanguageDetection:
		input := Input{Slot: "audio", Type: "audio", Required: true}
		input.AllowedMimes = canonical(spec.InputFormats)
		input.MaxSizeMb = megabytes(spec.MaxInputBytes)
		if spec.MaxInputSeconds != nil {
			seconds := float64(*spec.MaxInputSeconds)
			input.MaxDurationSec = &seconds
		}
		return []Input{input}
	case operationTextToSpeech, operationVoiceClone, operationTextToMusic, operationEmotionControl:
		inputs := []Input{textPromptInput()}
		if media := spec.ReferenceAudios; media != nil && mediaAllowsAny(media.Max) {
			inputs = append(inputs, referenceInput("reference_audio", "audio", media))
		}
		return inputs
	case "reference_audio_generation":
		inputs := []Input{textPromptInput()}
		if media := spec.ReferenceAudios; media != nil && mediaAllowsAny(media.Max) {
			inputs = append(inputs, referenceInput("reference_audio", "audio", media))
		}
		return inputs
	case "reference_image_generation":
		inputs := []Input{textPromptInput()}
		if media := spec.ReferenceImages; media != nil && mediaAllowsAny(media.Max) {
			inputs = append(inputs, referenceInput("reference_image", "image", media))
		}
		return inputs
	default:
		return []Input{textPromptInput()}
	}
}

// buildDataAPIOperations expands each endpoint operation the spec names. Data
// APIs have no declared parameter domain yet, so that gap is recorded per
// operation instead of inventing a parameter list.
func buildDataAPIOperations(spec specFile) ([]Operation, []string) {
	operations := make([]Operation, 0, len(spec.Operations))
	var unknown []string
	seen := map[string]bool{}
	for _, operationID := range spec.Operations {
		if operationID == "" || seen[operationID] {
			continue
		}
		seen[operationID] = true
		operations = append(operations, Operation{
			ID:         operationID,
			Executable: true,
			Inputs:     []Input{textPromptInput()},
			Parameters: mergeParameters(nil, spec, operationID),
			Output:     OperationOutput{Type: "text"},
		})
	}
	return operations, unknown
}

// mergeParameters applies the spec's parameters block over a template's own
// parameters. A same-named spec entry replaces the template entry in place so
// ordering stays deterministic and the more specific declaration wins.
func mergeParameters(base []Parameter, spec specFile, operationID string) []Parameter {
	merged := base
	if merged == nil {
		merged = []Parameter{}
	}
	for _, declared := range spec.Parameters {
		if declared.Name == "" || !parameterAppliesTo(declared, operationID) {
			continue
		}
		published := publishedParameter(declared)
		replaced := false
		for index := range merged {
			if merged[index].Name == published.Name {
				merged[index] = published
				replaced = true
				break
			}
		}
		if !replaced {
			merged = append(merged, published)
		}
	}
	return merged
}

func publishedParameter(declared ParameterSpec) Parameter {
	return Parameter{
		Name:        declared.Name,
		Type:        declared.Type,
		Enum:        declared.Enum,
		Minimum:     declared.Minimum,
		Maximum:     declared.Maximum,
		Step:        declared.Step,
		Default:     declared.Default,
		Fixed:       declared.Fixed,
		LimitSource: declared.LimitSource,
	}
}

// operationAliases lets a spec author the vendor-facing operation names while
// the projection uses its canonical operation id (for example the image
// parameters block that applies to text_to_image / image_to_image still applies
// to both projected operations).
var operationAliases = map[string][]string{
	operationImageToImage:   {"image_to_image", "multi_reference", "image_generation"},
	operationMultiReference: {"multi_reference", "image_to_image"},
	operationChat:           {"chat", "chat_completion", "llm_chat"},
	operationTextToSpeech:   {"text_to_speech", "tts"},
	operationVoiceClone:     {"voice_clone", "voice-clone"},
	operationSpeechToText:   {"speech_to_text", "transcription"},
	operationTextToMusic:    {"text_to_music", "music_generation"},
}

func parameterAppliesTo(declared ParameterSpec, operationID string) bool {
	if len(declared.AppliesTo) == 0 {
		return true
	}
	if slices.Contains(declared.AppliesTo, operationID) {
		return true
	}
	for _, alias := range operationAliases[operationID] {
		if slices.Contains(declared.AppliesTo, alias) {
			return true
		}
	}
	return false
}

func textPromptInput() Input {
	return Input{Slot: "prompt", Type: "text", Required: true}
}

func frameInput(slot, role string, limits *MediaLimits) Input {
	input := Input{Slot: slot, Type: "image", Role: role, Required: true, Min: intPointer(1), Max: intPointer(1)}
	if limits != nil {
		input.AllowedMimes = canonical(limits.Formats)
		input.MaxSizeMb = megabytes(limits.MaxBytesEach)
	}
	return input
}

// fixedReferenceInput is a single required media slot whose limits come from
// the matching reference family when the spec establishes them.
func fixedReferenceInput(slot, slotType string, limits *MediaLimits) Input {
	input := Input{Slot: slot, Type: slotType, Required: true, Min: intPointer(1), Max: intPointer(1)}
	if limits != nil {
		input.AllowedMimes = canonical(limits.Formats)
		input.MaxSizeMb = megabytes(limits.MaxBytesEach)
		input.MaxDurationSec = limits.MaxSecondsEach
	}
	return input
}

func referenceInput(slot, slotType string, limits *MediaLimits) Input {
	return Input{
		Slot:           slot,
		Type:           slotType,
		Role:           "reference",
		Min:            limits.Min,
		Max:            limits.Max,
		AllowedMimes:   canonical(limits.Formats),
		MaxSizeMb:      megabytes(limits.MaxBytesEach),
		MaxDurationSec: limits.MaxSecondsEach,
	}
}

func sourceVideoInput(spec specFile) Input {
	input := Input{Slot: "source_videos", Type: "video", Role: "source", Required: true, Min: intPointer(1), Max: intPointer(1)}
	if limits := spec.ReferenceVideos; limits != nil {
		input.AllowedMimes = canonical(limits.Formats)
		input.MaxSizeMb = megabytes(limits.MaxBytesEach)
		input.MaxDurationSec = limits.MaxSecondsEach
	}
	return input
}

// mediaAllowsAny distinguishes "no ceiling established" (nil, keep the slot)
// from an explicit zero ceiling (the family is not accepted at all).
func mediaAllowsAny(maximum *int) bool {
	return maximum == nil || *maximum > 0
}

func megabytes(bytes *int64) *float64 {
	if bytes == nil {
		return nil
	}
	value := float64(*bytes) / (1024 * 1024)
	return &value
}

func enumParameter(name string, values []any, defaultValue any) Parameter {
	parameter := Parameter{Name: name, Type: "enum", Enum: values}
	if defaultValue != nil {
		parameter.Default = defaultValue
	}
	return parameter
}

func policySource(note string) *LimitSource {
	return &LimitSource{Kind: "policy_conservative", Note: note}
}

func stringsToAny(values []string) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func float64sToAny(values []float64) []any {
	result := make([]any, len(values))
	for index, value := range values {
		result[index] = value
	}
	return result
}

func intPointer(value int) *int             { return &value }
func float64Pointer(value float64) *float64 { return &value }
