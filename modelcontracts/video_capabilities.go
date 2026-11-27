package modelcontracts

import (
	"fmt"
	"io"
	"math"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"sync/atomic"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
)

const (
	VideoCapabilitiesFileEnv = "OMNIMUX_VIDEO_CAPABILITIES_FILE"
	VideoProtocolOpenAI      = "openai_video"
	VideoProtocolGenerations = "video_generations"
	VideoProtocolHeader      = "X-OmniMux-Video-Protocol"

	VideoProfileSeedance25OpenAI     = "seedance25_openai_video"
	VideoProfileSeedanceOpenAI       = "seedance_openai_video"
	seedance25VideoRequestModel      = "seedance-2-5"
	seedance25FlashVideoRequestModel = "seedance-2-5-flash"
)

func isSeedanceNativeRequestModel(modelID string) bool {
	return modelID == seedance25VideoRequestModel || modelID == seedance25FlashVideoRequestModel
}

// VideoRequest describes the operation and inputs already normalized by the
// gateway. Slots contain counts by contract slot. Parameters include explicit
// zero and false as well as defaults filled by the selected protocol.
type VideoRequest struct {
	Operation  string
	Protocol   string
	Slots      map[string]int
	Parameters map[string]any
}

// VideoChannelRule binds verified operation variants to a channel and its
// execution identity. Only the operations are projected into the catalog.
type VideoChannelRule struct {
	ChannelID     int         `json:"channel_id"`
	ChannelType   int         `json:"channel_type"`
	Profile       string      `json:"profile"`
	UpstreamModel string      `json:"upstream_model"`
	BaseURL       string      `json:"base_url"`
	Operations    []Operation `json:"operations"`
	Evidence      string      `json:"evidence"`
	RequestModel  string      `json:"request_model,omitempty"`
	Protocol      string      `json:"-"`

	fixedResolution string
}

type VideoChannelIdentity struct {
	ChannelType     int
	Profile         string
	UpstreamModel   string
	BaseURL         string
	RequestModel    string
	Protocol        string
	FixedResolution string
}

// VideoCapabilities is an immutable snapshot keyed by the public model and
// physical channel. An empty channel map enrolls a model with no verified routes.
type VideoCapabilities struct {
	models map[string]videoModelRules
}

type videoModelRules struct {
	channels                 map[int]VideoChannelRule
	protocols                map[string]struct{}
	nativeRequestModel       string
	seedance25OpenAI         bool
	seedanceMultimodalOpenAI bool
}

var activeVideoCapabilities atomic.Pointer[VideoCapabilities]

func ActiveVideoCapabilities() *VideoCapabilities {
	return activeVideoCapabilities.Load()
}

func SetVideoCapabilities(rules *VideoCapabilities) *VideoCapabilities {
	return activeVideoCapabilities.Swap(rules)
}

func (rules *VideoCapabilities) rulesForModel(modelID string) videoModelRules {
	if rules == nil {
		return videoModelRules{}
	}
	if channels, ok := rules.models[modelID]; ok {
		return channels
	}
	return rules.models[strings.ReplaceAll(strings.ToLower(modelID), ".", "-")]
}

func (rules *VideoCapabilities) Managed(modelID string) bool {
	return rules.rulesForModel(modelID).channels != nil
}

// NativeRequestModel is the shared native decoder for a model's verified channels.
func (rules *VideoCapabilities) NativeRequestModel(modelID string) string {
	return rules.rulesForModel(modelID).nativeRequestModel
}

// HasProtocol reports whether any compiled channel uses the public protocol.
func (rules *VideoCapabilities) HasProtocol(modelID, protocol string) bool {
	_, found := rules.rulesForModel(modelID).protocols[protocol]
	return found
}

// HasSeedance25OpenAI reports whether this public model has the single-image decoder.
func (rules *VideoCapabilities) HasSeedance25OpenAI(modelID string) bool {
	return rules.rulesForModel(modelID).seedance25OpenAI
}

// HasSeedanceMultimodalOpenAI reports whether this public model has the multimodal OpenAI decoder.
func (rules *VideoCapabilities) HasSeedanceMultimodalOpenAI(modelID string) bool {
	return rules.rulesForModel(modelID).seedanceMultimodalOpenAI
}

func (rules *VideoCapabilities) ChannelIDs() []int {
	if rules == nil {
		return nil
	}
	seen := make(map[int]struct{})
	var ids []int
	for _, model := range rules.models {
		for id := range model.channels {
			if _, ok := seen[id]; !ok {
				seen[id] = struct{}{}
				ids = append(ids, id)
			}
		}
	}
	slices.Sort(ids)
	return ids
}

func (rules *VideoCapabilities) UsesChannel(id int) bool {
	if rules != nil {
		for _, model := range rules.models {
			if _, found := model.channels[id]; found {
				return true
			}
		}
	}
	return false
}

func (rules *VideoCapabilities) ChannelIdentity(modelID string, id int) (VideoChannelIdentity, bool) {
	channel, found := rules.rulesForModel(modelID).channels[id]
	return VideoChannelIdentity{
		ChannelType: channel.ChannelType, Profile: channel.Profile, UpstreamModel: channel.UpstreamModel,
		BaseURL: channel.BaseURL, RequestModel: channel.RequestModel, Protocol: channel.Protocol,
		FixedResolution: channel.fixedResolution,
	}, found
}

func (rules *VideoCapabilities) Accepts(modelID string, id int, request VideoRequest) bool {
	channel, found := rules.rulesForModel(modelID).channels[id]
	if !found || request.Protocol != "" && channel.Protocol != request.Protocol {
		return false
	}
	return slices.ContainsFunc(channel.Operations, func(operation Operation) bool {
		return VideoOperationAccepts(operation, request)
	})
}

func (rules *VideoCapabilities) ChannelRule(modelID string, id int) (VideoChannelRule, bool) {
	channel, found := rules.rulesForModel(modelID).channels[id]
	if found {
		channel.Operations = slices.Clone(channel.Operations)
		for index, operation := range channel.Operations {
			channel.Operations[index] = cloneVideoOperation(operation)
		}
	}
	return channel, found
}

func cloneVideoOperation(operation Operation) Operation {
	operation.Inputs = slices.Clone(operation.Inputs)
	for index := range operation.Inputs {
		input := &operation.Inputs[index]
		input.Min = cloneVideoPointer(input.Min)
		input.Max = cloneVideoPointer(input.Max)
		input.AllowedMimes = slices.Clone(input.AllowedMimes)
		input.MaxSizeMb = cloneVideoPointer(input.MaxSizeMb)
		input.MinDurationSec = cloneVideoPointer(input.MinDurationSec)
		input.MaxDurationSec = cloneVideoPointer(input.MaxDurationSec)
	}
	operation.Parameters = slices.Clone(operation.Parameters)
	for index := range operation.Parameters {
		parameter := &operation.Parameters[index]
		parameter.Enum = slices.Clone(parameter.Enum)
		parameter.Minimum = cloneVideoPointer(parameter.Minimum)
		parameter.Maximum = cloneVideoPointer(parameter.Maximum)
		parameter.Step = cloneVideoPointer(parameter.Step)
		parameter.LimitSource = cloneVideoPointer(parameter.LimitSource)
	}
	operation.Output.Formats = slices.Clone(operation.Output.Formats)
	operation.Output.Sizes = slices.Clone(operation.Output.Sizes)
	return operation
}

func cloneVideoPointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

// ReadVideoCapabilities compiles a complete snapshot before it can become
// active. The configured channels can only narrow executable video operations.
func ReadVideoCapabilities(reader io.Reader) (*VideoCapabilities, error) {
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	var document struct {
		Models map[string][]common.RawMessage `json:"models"`
	}
	if err := decodeVideoRule(data, &document, "models"); err != nil {
		return nil, fmt.Errorf("video capabilities: %w", err)
	}
	if document.Models == nil {
		return nil, fmt.Errorf("video capabilities: models object is required")
	}

	rules := &VideoCapabilities{models: make(map[string]videoModelRules, len(document.Models))}
	for name, rawChannels := range document.Models {
		contract, found := For(name)
		if !found || contract.Family != "video" || !contract.IsLiveExecutable() {
			return nil, fmt.Errorf("video capabilities: model %q has no live video contract", name)
		}
		if rawChannels == nil || rules.Managed(contract.ModelID) {
			return nil, fmt.Errorf("video capabilities: model %q needs one channel array", name)
		}
		legacyNative := isSeedanceNativeRequestModel(contract.ModelID)
		if contract.GatewayProtocol != VideoProtocolOpenAI && contract.GatewayProtocol != VideoProtocolGenerations && !legacyNative {
			return nil, fmt.Errorf("video capabilities: model %q has no verified video protocol", name)
		}

		model := videoModelRules{
			channels:  make(map[int]VideoChannelRule, len(rawChannels)),
			protocols: make(map[string]struct{}),
		}
		for index, raw := range rawChannels {
			channel, err := compileVideoChannelRule(raw, contract, legacyNative)
			if err != nil {
				return nil, fmt.Errorf("video capabilities: model %q channel %d: %w", contract.ModelID, index+1, err)
			}
			if _, found := model.channels[channel.ChannelID]; found {
				return nil, fmt.Errorf("video capabilities: model %q repeats channel %d", contract.ModelID, channel.ChannelID)
			}
			if channel.RequestModel != "" {
				if model.nativeRequestModel != "" && model.nativeRequestModel != channel.RequestModel {
					return nil, fmt.Errorf("video capabilities: model %q uses multiple native request dialects", contract.ModelID)
				}
				model.nativeRequestModel = channel.RequestModel
			}
			model.channels[channel.ChannelID] = channel
			model.protocols[channel.Protocol] = struct{}{}
			model.seedance25OpenAI = model.seedance25OpenAI || channel.Profile == VideoProfileSeedance25OpenAI
			model.seedanceMultimodalOpenAI = model.seedanceMultimodalOpenAI || channel.Profile == VideoProfileSeedanceOpenAI
		}
		for _, channel := range model.channels {
			if len(model.protocols) > 1 || channel.Protocol != contract.GatewayProtocol {
				for i := range channel.Operations {
					channel.Operations[i].GatewayProtocol = channel.Protocol
				}
			}
		}
		rules.models[contract.ModelID] = model
	}
	return rules, nil
}

func compileVideoChannelRule(raw common.RawMessage, contract Contract, legacyNative bool) (VideoChannelRule, error) {
	var spec struct {
		VideoChannelRule
		Operations []common.RawMessage `json:"operations"`
	}
	if err := decodeVideoRule(raw, &spec, "channel_id", "channel_type", "profile", "upstream_model", "base_url", "operations", "evidence", "request_model"); err != nil {
		return VideoChannelRule{}, err
	}
	channel := spec.VideoChannelRule
	var validType bool
	switch channel.Profile {
	case profileAPIMartVideo:
		channel.Protocol = VideoProtocolGenerations
		if channel.RequestModel == "" {
			if isSeedanceNativeRequestModel(contract.ModelID) {
				channel.RequestModel = contract.ModelID
			} else if contract.GatewayProtocol == VideoProtocolGenerations {
				channel.RequestModel = seedance25VideoRequestModel
			}
		}
		if !isSeedanceNativeRequestModel(channel.RequestModel) {
			return VideoChannelRule{}, fmt.Errorf("request_model must name a verified native request dialect")
		}
		validType = channel.ChannelType == constant.ChannelTypeApiMart
	case profileOpenAIVideo:
		channel.Protocol = VideoProtocolOpenAI
		switch channel.ChannelType {
		case constant.ChannelTypeOpenAI, constant.ChannelTypeSora, constant.ChannelTypeTaskPlugin, constant.ChannelTypeNewAPI:
			validType = contract.GatewayProtocol == VideoProtocolOpenAI && channel.RequestModel == ""
		}
	case VideoProfileSeedance25OpenAI:
		channel.Protocol = VideoProtocolOpenAI
		validType = contract.ModelID == seedance25VideoRequestModel && contract.GatewayProtocol != VideoProtocolOpenAI &&
			channel.ChannelType == constant.ChannelTypeOpenAI && channel.RequestModel == ""
	case VideoProfileSeedanceOpenAI:
		channel.Protocol = VideoProtocolOpenAI
		validType = legacyNative && contract.GatewayProtocol != VideoProtocolOpenAI &&
			channel.ChannelType == constant.ChannelTypeOpenAI && channel.RequestModel == ""
	}
	if channel.ChannelID < 1 || !validType || strings.TrimSpace(channel.UpstreamModel) != channel.UpstreamModel || channel.UpstreamModel == "" {
		return VideoChannelRule{}, fmt.Errorf("channel_id, channel_type, profile and upstream_model must identify a verified video route")
	}
	parsedURL, err := url.Parse(channel.BaseURL)
	if err != nil || (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Hostname() == "" ||
		parsedURL.User != nil || parsedURL.RawQuery != "" || parsedURL.ForceQuery || strings.Contains(channel.BaseURL, "#") ||
		strings.ContainsFunc(channel.BaseURL, unicode.IsSpace) {
		return VideoChannelRule{}, fmt.Errorf("base_url must be an HTTP(S) URL with no credentials, query or fragment")
	}
	channel.BaseURL = strings.TrimRight(channel.BaseURL, "/")
	if strings.TrimSpace(channel.Evidence) == "" || len(spec.Operations) == 0 {
		return VideoChannelRule{}, fmt.Errorf("evidence and at least one operation variant are required")
	}
	channel.Operations = make([]Operation, 0, len(spec.Operations))
	for _, rawOperation := range spec.Operations {
		operation, err := compileVideoOperationRule(rawOperation, contract, channel.Protocol, channel.Profile, channel.RequestModel, legacyNative)
		if err != nil {
			return VideoChannelRule{}, err
		}
		if channel.Profile == VideoProfileSeedance25OpenAI || channel.Profile == VideoProfileSeedanceOpenAI {
			for _, parameter := range operation.Parameters {
				if parameter.Name != "resolution" {
					continue
				}
				resolution, _ := parameter.Fixed.(string)
				if channel.fixedResolution != "" && channel.fixedResolution != resolution {
					return VideoChannelRule{}, fmt.Errorf("Seedance OpenAI variants on one channel must use the same fixed resolution")
				}
				channel.fixedResolution = resolution
			}
		}
		channel.Operations = append(channel.Operations, operation)
	}
	return channel, nil
}

func compileVideoOperationRule(raw common.RawMessage, contract Contract, protocol, profile, nativeRequestModel string, legacyNative bool) (Operation, error) {
	var spec struct {
		ID         string              `json:"id"`
		Executable *bool               `json:"executable"`
		ModeKind   *string             `json:"modeKind"`
		Inputs     []common.RawMessage `json:"inputs"`
		Parameters []common.RawMessage `json:"parameters"`
	}
	if err := decodeVideoRule(raw, &spec, "id", "executable", "modeKind", "inputs", "parameters"); err != nil {
		return Operation{}, err
	}
	index := slices.IndexFunc(contract.Operations, func(candidate Operation) bool {
		return candidate.ID == spec.ID && candidate.Executable && candidate.Output.Type == "video"
	})
	if index < 0 {
		return Operation{}, fmt.Errorf("operation %q is not executable", spec.ID)
	}
	operation := cloneVideoOperation(contract.Operations[index])
	if operation.ModeKind == "" {
		return Operation{}, fmt.Errorf("operation %q has no model contract modeKind", spec.ID)
	}
	if spec.Executable != nil && !*spec.Executable ||
		spec.ModeKind != nil && *spec.ModeKind != operation.ModeKind {
		return Operation{}, fmt.Errorf("operation %q conflicts with its model contract", spec.ID)
	}
	if spec.Inputs != nil {
		inputs, err := compileVideoInputRules(spec.Inputs, operation.Inputs)
		if err != nil {
			return Operation{}, fmt.Errorf("operation %q: %w", operation.ID, err)
		}
		operation.Inputs = inputs
	}
	for _, input := range operation.Inputs {
		if input.Type != "text" && (input.Min == nil || input.Max == nil) {
			return Operation{}, fmt.Errorf("operation %q requires count bounds for slot %q", operation.ID, input.Slot)
		}
	}
	if operation.ID == operationVideoMultiRef && !slices.ContainsFunc(operation.Inputs, func(input Input) bool {
		return input.Role == "reference" && input.Required
	}) {
		return Operation{}, fmt.Errorf("operation %q requires a reference slot in each variant", operation.ID)
	}
	// A static default can be a UI recommendation. Publish an effective
	// default only when the channel rule establishes it.
	for i := range operation.Parameters {
		operation.Parameters[i].Default = nil
	}
	if spec.Parameters != nil {
		parameters, err := compileVideoParameterRules(spec.Parameters, operation.Parameters, protocol == VideoProtocolGenerations && legacyNative)
		if err != nil {
			return Operation{}, fmt.Errorf("operation %q: %w", operation.ID, err)
		}
		operation.Parameters = parameters
	}
	switch profile {
	case VideoProfileSeedance25OpenAI:
		if err := validateSeedance25OpenAIVideoOperation(&operation); err != nil {
			return Operation{}, err
		}
		return operation, nil
	case VideoProfileSeedanceOpenAI:
		if err := validateSeedanceOpenAIVideoOperation(&operation, contract.ModelID); err != nil {
			return Operation{}, err
		}
		return operation, nil
	}
	if protocol == VideoProtocolOpenAI {
		// The Muse protocol decoder always supplies duration. Its default is
		// established by plugins/tasks/sora/plugin.js before a channel is chosen.
		const gatewayDuration = 5
		index := slices.IndexFunc(operation.Parameters, func(parameter Parameter) bool { return parameter.Name == "duration" })
		if index < 0 {
			return Operation{}, fmt.Errorf("operation %q requires a duration parameter for this video protocol", operation.ID)
		}
		duration := &operation.Parameters[index]
		if duration.Default != nil && !equalVideoValue(duration.Default, gatewayDuration) {
			return Operation{}, fmt.Errorf("operation %q duration default conflicts with the video protocol", operation.ID)
		}
		duration.Required = !videoParameterAccepts(*duration, gatewayDuration)
		if !duration.Required {
			duration.Default = gatewayDuration
		}
	}
	for _, parameter := range operation.Parameters {
		if parameter.Name != "output_format" {
			continue
		}
		formats := make([]string, 0, len(operation.Output.Formats))
		for _, format := range operation.Output.Formats {
			if videoParameterAccepts(parameter, format) {
				formats = append(formats, format)
			}
		}
		if len(formats) == 0 {
			return Operation{}, fmt.Errorf("operation %q has no compatible output format", operation.ID)
		}
		operation.Output.Formats = formats
		break
	}
	if err := validateManagedVideoOperation(&operation, protocol, nativeRequestModel, legacyNative); err != nil {
		return Operation{}, err
	}
	return operation, nil
}

// The economy decoder sends one unroled image and an explicit duration and
// ratio. The resolved channel supplies the resolution when callers omit it.
func validateSeedance25OpenAIVideoOperation(operation *Operation) error {
	if operation.ID != operationVideoMultiRef || len(operation.Inputs) != 2 || !slices.ContainsFunc(operation.Inputs, func(input Input) bool {
		return input.Slot == "prompt" && input.Required
	}) || !slices.ContainsFunc(operation.Inputs, func(input Input) bool {
		return input.Slot == "reference_images" && input.Required && input.FirstItemRole == "" &&
			input.Min != nil && *input.Min == 1 && input.Max != nil && *input.Max == 1
	}) {
		return fmt.Errorf("Seedance OpenAI requires a prompt and exactly one reference image")
	}
	if len(operation.Parameters) != 3 {
		return fmt.Errorf("Seedance OpenAI requires duration, aspect_ratio and resolution")
	}
	for i := range operation.Parameters {
		parameter := &operation.Parameters[i]
		switch parameter.Name {
		case "duration":
			seconds, ok := videoNumber(parameter.Fixed)
			if parameter.Type != "integer" || !ok || seconds < 4 || seconds > 30 || math.Trunc(seconds) != seconds || parameter.Default != nil {
				return fmt.Errorf("Seedance OpenAI requires an explicit fixed duration from 4 to 30 seconds")
			}
			parameter.Required = true
		case "aspect_ratio":
			if parameter.Type != "enum" || parameter.Fixed != "9:16" || parameter.Default != nil {
				return fmt.Errorf("Seedance OpenAI requires an explicit fixed 9:16 aspect ratio")
			}
			parameter.Required = true
		case "resolution":
			resolution, ok := parameter.Fixed.(string)
			if parameter.Type != "enum" || !ok || resolution != "480p" && resolution != "720p" {
				return fmt.Errorf("Seedance OpenAI requires one fixed 480p or 720p resolution")
			}
			// The group catalog may supply a default after checking all eligible channels.
			parameter.Default = nil
			parameter.Required = true
		default:
			return fmt.Errorf("parameter %q is unavailable for Seedance OpenAI", parameter.Name)
		}
	}
	// The model-wide media specifications do not establish the economy route's
	// input MIME/byte limits or its output encoding.
	for i := range operation.Inputs {
		operation.Inputs[i].AllowedMimes = nil
		operation.Inputs[i].MaxSizeMb = nil
	}
	operation.Output.Formats = nil
	operation.ModeKind = "image_reference"
	return nil
}

// validateSeedanceOpenAIVideoOperation validates the multimodal OpenAI-video
// route for Seedance 2.5 and Seedance 2.5 Flash fixed-resolution SKUs.
func validateSeedanceOpenAIVideoOperation(operation *Operation, modelID string) error {
	switch operation.ID {
	case operationTextToVideo, operationFirstFrame, operationFirstLastFrame, operationVideoMultiRef:
	default:
		return fmt.Errorf("operation %q is not supported by the verified %s decoder", operation.ID, VideoProfileSeedanceOpenAI)
	}

	inputLimits := map[string]int{
		"first_frame": 1, "last_frame": 1,
		"reference_images": 30, "reference_videos": 10, "reference_audios": 10,
	}
	allowedRes := []string{"480p", "720p", "1080p"}
	if modelID == seedance25FlashVideoRequestModel {
		inputLimits["reference_images"] = 10
		inputLimits["reference_videos"] = 5
		inputLimits["reference_audios"] = 5
		allowedRes = []string{"480p", "720p"}
	}

	hasPrompt, hasFirst, hasLast, refSlots := false, false, false, 0
	for _, input := range operation.Inputs {
		if input.Slot == "prompt" {
			if !input.Required {
				return fmt.Errorf("Seedance OpenAI requires a prompt input")
			}
			hasPrompt = true
			continue
		}
		limit, known := inputLimits[input.Slot]
		if !known || input.FirstItemRole != "" || input.Min == nil || input.Max == nil ||
			*input.Min < 0 || *input.Max < 1 || *input.Max > limit || *input.Min > *input.Max {
			return fmt.Errorf("operation %q slot %q has incompatible media semantics for %s", operation.ID, input.Slot, VideoProfileSeedanceOpenAI)
		}
		switch input.Slot {
		case "first_frame":
			hasFirst = true
		case "last_frame":
			hasLast = true
		default:
			refSlots++
		}
	}
	if !hasPrompt {
		return fmt.Errorf("Seedance OpenAI requires a prompt input")
	}
	switch operation.ID {
	case operationTextToVideo:
		if hasFirst || hasLast || refSlots > 0 {
			return fmt.Errorf("text_to_video accepts only prompt input")
		}
	case operationFirstFrame:
		if !hasFirst || hasLast || refSlots > 0 {
			return fmt.Errorf("first_frame requires prompt and first_frame only")
		}
	case operationFirstLastFrame:
		if !hasFirst || !hasLast || refSlots > 0 {
			return fmt.Errorf("first_last_frame requires prompt, first_frame and last_frame only")
		}
	case operationVideoMultiRef:
		if hasFirst || hasLast || refSlots == 0 {
			return fmt.Errorf("video_multi_ref requires at least one reference media slot")
		}
	}

	allowedRatios := []string{"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive"}
	hasDuration, hasResolution, hasRefMedia := false, false, false
	for _, input := range operation.Inputs {
		if input.Slot == "reference_videos" || input.Slot == "reference_audios" {
			hasRefMedia = true
			break
		}
	}
	for i := range operation.Parameters {
		parameter := &operation.Parameters[i]
		switch parameter.Name {
		case "duration":
			if parameter.Type != "integer" || parameter.Default != nil {
				return fmt.Errorf("Seedance OpenAI requires an explicit integer duration from 4 to 30 seconds")
			}
			valid := (len(parameter.Enum) > 0 || parameter.Minimum != nil && parameter.Maximum != nil) &&
				(parameter.Minimum == nil || *parameter.Minimum >= 4) &&
				(parameter.Maximum == nil || *parameter.Maximum <= 30) &&
				(parameter.Minimum == nil || parameter.Maximum == nil || *parameter.Minimum <= *parameter.Maximum)
			for _, choice := range parameter.Enum {
				value, numeric := videoNumber(choice)
				valid = valid && numeric && math.Trunc(value) == value && value >= 4 && value <= 30
			}
			if !valid || (parameter.Fixed != nil && !videoParameterAccepts(*parameter, parameter.Fixed)) {
				return fmt.Errorf("Seedance OpenAI duration must stay within 4 to 30 seconds")
			}
			parameter.Required = true
			hasDuration = true
		case "resolution":
			resolution, ok := parameter.Fixed.(string)
			if parameter.Type != "enum" || !ok || !slices.Contains(allowedRes, resolution) {
				return fmt.Errorf("Seedance OpenAI requires one fixed resolution in %v", allowedRes)
			}
			parameter.Default = nil
			parameter.Required = true
			hasResolution = true
		case "aspect_ratio":
			if parameter.Type != "enum" || len(parameter.Enum) == 0 {
				return fmt.Errorf("Seedance OpenAI aspect_ratio must be a non-empty enum")
			}
			for _, choice := range parameter.Enum {
				value, ok := choice.(string)
				if !ok || !slices.Contains(allowedRatios, value) {
					return fmt.Errorf("Seedance OpenAI aspect_ratio %v is unsupported", choice)
				}
			}
			if (parameter.Default != nil && !videoParameterAccepts(*parameter, parameter.Default)) ||
				(parameter.Fixed != nil && !videoParameterAccepts(*parameter, parameter.Fixed)) {
				return fmt.Errorf("Seedance OpenAI aspect_ratio default/fixed conflicts with enum")
			}
		default:
			return fmt.Errorf("parameter %q is unavailable for %s", parameter.Name, VideoProfileSeedanceOpenAI)
		}
	}
	if !hasDuration || !hasResolution {
		return fmt.Errorf("Seedance OpenAI requires duration and fixed resolution parameters")
	}
	for i := range operation.Inputs {
		operation.Inputs[i].AllowedMimes = nil
		operation.Inputs[i].MaxSizeMb = nil
		operation.Inputs[i].MinDurationSec = nil
		operation.Inputs[i].MaxDurationSec = nil
	}
	operation.Output.Formats = nil
	if operation.ID == operationVideoMultiRef {
		if hasRefMedia {
			operation.ModeKind = "multimodal_reference"
		} else {
			operation.ModeKind = "image_reference"
		}
	} else {
		operation.ModeKind = operation.ID
	}
	return nil
}

// The managed codecs retain different request fields and media semantics.
// Keep these boundaries in parity with decodeMuseVideo in the built-in
// plugin and normalizeManagedVideoRequest in the native video adapter.
func validateManagedVideoOperation(operation *Operation, protocol, nativeRequestModel string, legacyNative bool) error {
	native := protocol == VideoProtocolGenerations
	allowedOperations := openAIVideoOperations
	inputLimits := map[string]int{"first_frame": 1, "reference_images": 10}
	enums := map[string][]string{
		"aspect_ratio":  {"16:9", "9:16", "1:1", "adaptive"},
		"output_format": {"mp4"},
	}
	minDuration, maxDuration := 1.0, 60.0
	if native {
		if nativeRequestModel == "" {
			nativeRequestModel = seedance25VideoRequestModel
		}
		allowedOperations = apimartVideoOperations[nativeRequestModel]
		if nativeRequestModel == seedance25FlashVideoRequestModel {
			inputLimits = map[string]int{
				"first_frame": 1, "last_frame": 1, "reference_images": 10,
				"reference_videos": 5, "reference_audios": 5, "source_videos": 5,
			}
			enums = map[string][]string{
				"aspect_ratio":  {"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive"},
				"output_format": {"mp4", "mov"},
				"resolution":    {"480p", "720p"},
			}
		} else {
			inputLimits = map[string]int{
				"first_frame": 1, "last_frame": 1, "reference_images": 30,
				"reference_videos": 10, "reference_audios": 10, "source_videos": 10,
			}
			enums = map[string][]string{
				"aspect_ratio":  {"16:9", "9:16", "1:1", "4:3", "3:4", "21:9", "adaptive"},
				"output_format": {"mp4", "mov"},
				"resolution":    {"480p", "720p", "1080p"},
			}
		}
		if operation.ID == operationVideoEdit || operation.ID == operationVideoExtend {
			enums["aspect_ratio"] = []string{"adaptive"}
		}
		minDuration, maxDuration = 4, 30
	}
	if !slices.Contains(allowedOperations, operation.ID) || operation.ID == operationVideoEdit && !legacyNative {
		return fmt.Errorf("operation %q is not supported by the verified %s decoder", operation.ID, protocol)
	}
	for _, input := range operation.Inputs {
		if input.Slot == "prompt" {
			continue
		}
		limit, known := inputLimits[input.Slot]
		firstItemRole := ""
		minimumCount := 0
		if !native && input.Slot == "reference_images" {
			firstItemRole, minimumCount = "first_frame", 2
		}
		if !known || input.FirstItemRole != firstItemRole || input.Min == nil || *input.Min < minimumCount ||
			input.Max == nil || *input.Max < 1 || *input.Max > limit || *input.Min > *input.Max {
			return fmt.Errorf("operation %q slot %q has incompatible media semantics for the %s decoder", operation.ID, input.Slot, protocol)
		}
	}
	if native && !legacyNative {
		duration := slices.IndexFunc(operation.Parameters, func(parameter Parameter) bool {
			return parameter.Name == "duration"
		})
		if duration < 0 {
			return fmt.Errorf("operation %q requires a duration parameter for this video protocol", operation.ID)
		}
		if operation.Parameters[duration].Default != nil {
			return fmt.Errorf("operation %q duration requires an explicit value and cannot have a default", operation.ID)
		}
		operation.Parameters[duration].Required = true
	}
	for _, parameter := range operation.Parameters {
		valid := false
		switch parameter.Name {
		case "duration":
			if native && legacyNative && equalVideoValue(parameter.Fixed, -1) {
				valid = parameter.Type == "integer" &&
					parameter.Minimum != nil && *parameter.Minimum == -1 &&
					parameter.Maximum != nil && *parameter.Maximum == -1
				break
			}
			valid = parameter.Type == "integer" &&
				(len(parameter.Enum) > 0 || parameter.Minimum != nil && parameter.Maximum != nil) &&
				(parameter.Minimum == nil || *parameter.Minimum >= minDuration) &&
				(parameter.Maximum == nil || *parameter.Maximum <= maxDuration) &&
				(parameter.Minimum == nil || parameter.Maximum == nil || *parameter.Minimum <= *parameter.Maximum)
			for _, choice := range parameter.Enum {
				value, numeric := videoNumber(choice)
				valid = valid && numeric && math.Trunc(value) == value && value >= minDuration && value <= maxDuration
			}
		case "generate_audio":
			valid = native && parameter.Type == "boolean"
		default:
			choices, known := enums[parameter.Name]
			valid = known && parameter.Type == "enum" && len(parameter.Enum) > 0
			for _, choice := range parameter.Enum {
				value, isString := choice.(string)
				valid = valid && isString && slices.Contains(choices, value)
			}
		}
		valid = valid &&
			(parameter.Default == nil || videoParameterAccepts(parameter, parameter.Default)) &&
			(parameter.Fixed == nil || videoParameterAccepts(parameter, parameter.Fixed))
		if !valid {
			return fmt.Errorf("operation %q parameter %q exceeds the verified %s decoder contract", operation.ID, parameter.Name, protocol)
		}
	}
	for _, format := range operation.Output.Formats {
		if !slices.Contains(enums["output_format"], format) {
			return fmt.Errorf("operation %q output format %q is not supported by the verified %s decoder", operation.ID, format, protocol)
		}
	}
	return nil
}

func compileVideoInputRules(rules []common.RawMessage, base []Input) ([]Input, error) {
	selected := make(map[string]Input, len(rules))
	for _, raw := range rules {
		var spec struct {
			Input
			Required *bool `json:"required"`
		}
		if err := decodeVideoRule(raw, &spec, "slot", "type", "role", "firstItemRole", "required", "min", "max", "allowedMimes", "maxSizeMb", "minDurationSec", "maxDurationSec"); err != nil {
			return nil, err
		}
		index := slices.IndexFunc(base, func(input Input) bool { return input.Slot == spec.Slot })
		if _, duplicate := selected[spec.Slot]; duplicate || index < 0 {
			return nil, fmt.Errorf("unknown or repeated input slot %q", spec.Slot)
		}
		input := base[index]
		if spec.Type != "" && spec.Type != input.Type ||
			spec.Role != "" && spec.Role != input.Role ||
			spec.FirstItemRole != "" && spec.FirstItemRole != input.FirstItemRole ||
			spec.AllowedMimes != nil && !slices.Equal(spec.AllowedMimes, input.AllowedMimes) ||
			spec.MaxSizeMb != nil && !reflect.DeepEqual(spec.MaxSizeMb, input.MaxSizeMb) ||
			spec.MinDurationSec != nil && !reflect.DeepEqual(spec.MinDurationSec, input.MinDurationSec) ||
			spec.MaxDurationSec != nil && !reflect.DeepEqual(spec.MaxDurationSec, input.MaxDurationSec) {
			return nil, fmt.Errorf("input slot %q changes its established media semantics", spec.Slot)
		}
		if spec.Required != nil {
			if input.Required && !*spec.Required {
				return nil, fmt.Errorf("input slot %q cannot become optional", spec.Slot)
			}
			input.Required = *spec.Required
		}
		if spec.Min != nil {
			if *spec.Min < 0 || input.Min != nil && *spec.Min < *input.Min {
				return nil, fmt.Errorf("input slot %q widens the lower count limit", spec.Slot)
			}
			input.Min = spec.Min
		}
		if spec.Max != nil {
			if *spec.Max < 1 || input.Max != nil && *spec.Max > *input.Max {
				return nil, fmt.Errorf("input slot %q has an invalid upper count limit", spec.Slot)
			}
			input.Max = spec.Max
		}
		if input.Required && input.Type != "text" && input.Min != nil && *input.Min == 0 {
			input.Min = intPointer(1)
		}
		if input.Min != nil && input.Max != nil && *input.Min > *input.Max {
			return nil, fmt.Errorf("input slot %q has an empty count range", spec.Slot)
		}
		selected[spec.Slot] = input
	}

	inputs := make([]Input, 0, len(selected))
	for _, input := range base {
		if narrowed, ok := selected[input.Slot]; ok {
			inputs = append(inputs, narrowed)
		} else if input.Required {
			return nil, fmt.Errorf("required input slot %q is missing", input.Slot)
		}
	}
	return inputs, nil
}

func compileVideoParameterRules(rules []common.RawMessage, base []Parameter, seedanceAuto bool) ([]Parameter, error) {
	selected := make(map[string]Parameter, len(rules))
	for _, raw := range rules {
		var spec Parameter
		if err := decodeVideoRule(raw, &spec, "name", "type", "enum", "minimum", "maximum", "step", "default", "fixed"); err != nil {
			return nil, err
		}
		index := slices.IndexFunc(base, func(parameter Parameter) bool { return parameter.Name == spec.Name })
		if _, duplicate := selected[spec.Name]; duplicate || index < 0 {
			return nil, fmt.Errorf("unknown or repeated parameter %q", spec.Name)
		}
		parameter, err := narrowVideoParameter(base[index], spec, seedanceAuto)
		if err != nil {
			return nil, fmt.Errorf("parameter %q: %w", spec.Name, err)
		}
		selected[parameter.Name] = parameter
	}

	parameters := make([]Parameter, 0, len(selected))
	for _, parameter := range base {
		if narrowed, ok := selected[parameter.Name]; ok {
			parameters = append(parameters, narrowed)
		}
	}
	return parameters, nil
}

func narrowVideoParameter(base, spec Parameter, seedanceAuto bool) (Parameter, error) {
	if spec.Type != "" && spec.Type != base.Type || spec.Step != nil && !reflect.DeepEqual(spec.Step, base.Step) {
		return Parameter{}, fmt.Errorf("type and step must match the established parameter")
	}
	parameter := base
	if spec.Enum != nil {
		if base.Type != "enum" || len(spec.Enum) == 0 {
			return Parameter{}, fmt.Errorf("an enum restriction needs at least one allowed value")
		}
		parameter.Enum = nil
		for _, value := range base.Enum {
			if slices.ContainsFunc(spec.Enum, func(candidate any) bool { return equalVideoValue(candidate, value) }) {
				parameter.Enum = append(parameter.Enum, value)
			}
		}
		if len(parameter.Enum) != len(spec.Enum) {
			return Parameter{}, fmt.Errorf("enum restriction contains unsupported or repeated values")
		}
	}
	auto := seedanceAuto && base.Name == "duration" && equalVideoValue(spec.Fixed, -1)
	if auto && (spec.Minimum != nil && *spec.Minimum != -1 || spec.Maximum != nil && *spec.Maximum != -1) {
		return Parameter{}, fmt.Errorf("automatic duration is a separate fixed variant")
	}
	if spec.Minimum != nil || spec.Maximum != nil || auto {
		if base.Type != "integer" && base.Type != "number" {
			return Parameter{}, fmt.Errorf("only numeric parameters can have a range")
		}
		if spec.Minimum != nil {
			if (!auto || *spec.Minimum != -1) && !videoParameterAccepts(base, *spec.Minimum) {
				return Parameter{}, fmt.Errorf("minimum is outside the established parameter range")
			}
			parameter.Minimum = spec.Minimum
		}
		if spec.Maximum != nil {
			if (!auto || *spec.Maximum != -1) && !videoParameterAccepts(base, *spec.Maximum) {
				return Parameter{}, fmt.Errorf("maximum is outside the established parameter range")
			}
			parameter.Maximum = spec.Maximum
		}
		if auto {
			// Seedance's adapter admits -1 in addition to the published positive
			// range. A separate fixed variant never admits the gap between them.
			fixed := float64(-1)
			parameter.Minimum = &fixed
			parameter.Maximum = &fixed
		}
		if parameter.Minimum != nil && parameter.Maximum != nil && *parameter.Minimum > *parameter.Maximum {
			return Parameter{}, fmt.Errorf("the parameter range is empty")
		}
	}
	if spec.Fixed != nil {
		if !auto && !videoParameterAccepts(parameter, spec.Fixed) {
			return Parameter{}, fmt.Errorf("fixed value is outside the parameter's choices")
		}
		parameter.Fixed = spec.Fixed
	} else if parameter.Fixed != nil && !videoParameterAccepts(parameter, parameter.Fixed) {
		return Parameter{}, fmt.Errorf("restriction excludes the established fixed value")
	}
	narrowed := !slices.EqualFunc(parameter.Enum, base.Enum, equalVideoValue) ||
		!reflect.DeepEqual(parameter.Minimum, base.Minimum) || !reflect.DeepEqual(parameter.Maximum, base.Maximum) ||
		!equalVideoValue(parameter.Fixed, base.Fixed)
	if spec.Default != nil {
		if !videoParameterAccepts(parameter, spec.Default) {
			return Parameter{}, fmt.Errorf("default is outside the parameter's choices")
		}
		parameter.Default = spec.Default
	} else if narrowed {
		// A catalog default does not establish what this channel will do when
		// the caller omits a narrowed field.
		parameter.Default = nil
		parameter.Required = true
	}
	if !reflect.DeepEqual(parameter, base) {
		parameter.LimitSource = nil
	}
	return parameter, nil
}

func decodeVideoRule(raw common.RawMessage, target any, keys ...string) error {
	var fields map[string]common.RawMessage
	if err := common.Unmarshal(raw, &fields); err != nil {
		return err
	}
	if fields == nil {
		return fmt.Errorf("expected an object")
	}
	for key, value := range fields {
		if !slices.Contains(keys, key) {
			return fmt.Errorf("unrecognized field %q", key)
		}
		if string(value) == "null" {
			return fmt.Errorf("field %q cannot be null", key)
		}
	}
	return common.Unmarshal(raw, target)
}

// VideoOperationAccepts tests the whole intent against one variant. A caller
// checks each variant separately, so one channel cannot borrow another's axes.
func VideoOperationAccepts(operation Operation, request VideoRequest) bool {
	if !operation.Executable || operation.ID != request.Operation ||
		operation.GatewayProtocol != "" && request.Protocol != "" && operation.GatewayProtocol != request.Protocol {
		return false
	}
	for _, input := range operation.Inputs {
		count := request.Slots[input.Slot]
		if count == 0 {
			if input.Required {
				return false
			}
			continue
		}
		if count < 0 || input.Min != nil && count < *input.Min || input.Max != nil && count > *input.Max || input.Type == "text" && count != 1 {
			return false
		}
	}
	for name, count := range request.Slots {
		if count < 0 || count > 0 && !slices.ContainsFunc(operation.Inputs, func(input Input) bool { return input.Slot == name }) {
			return false
		}
	}
	for _, parameter := range operation.Parameters {
		if _, present := request.Parameters[parameter.Name]; parameter.Required && !present {
			return false
		}
	}
	for name, value := range request.Parameters {
		index := slices.IndexFunc(operation.Parameters, func(parameter Parameter) bool { return parameter.Name == name })
		if index >= 0 {
			if !videoParameterAccepts(operation.Parameters[index], value) {
				return false
			}
			continue
		}
		format, isString := value.(string)
		if name != "output_format" || !isString || !slices.Contains(operation.Output.Formats, format) {
			return false
		}
	}
	return true
}

func videoParameterAccepts(parameter Parameter, value any) bool {
	if parameter.Fixed != nil && !equalVideoValue(parameter.Fixed, value) {
		return false
	}
	switch parameter.Type {
	case "enum":
		return slices.ContainsFunc(parameter.Enum, func(candidate any) bool { return equalVideoValue(candidate, value) })
	case "integer", "number":
		number, ok := videoNumber(value)
		return ok && !math.IsNaN(number) && !math.IsInf(number, 0) &&
			(parameter.Type != "integer" || math.Trunc(number) == number) &&
			(parameter.Minimum == nil || number >= *parameter.Minimum) &&
			(parameter.Maximum == nil || number <= *parameter.Maximum) &&
			(len(parameter.Enum) == 0 || slices.ContainsFunc(parameter.Enum, func(candidate any) bool {
				return equalVideoValue(candidate, value)
			}))
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "string":
		_, ok := value.(string)
		return ok
	default:
		return false
	}
}

func videoNumber(value any) (float64, bool) {
	switch number := value.(type) {
	case int:
		return float64(number), true
	case int64:
		return float64(number), true
	case float64:
		return number, true
	default:
		return 0, false
	}
}

func equalVideoValue(first, second any) bool {
	if firstNumber, ok := videoNumber(first); ok {
		secondNumber, ok := videoNumber(second)
		return ok && firstNumber == secondNumber
	}
	return reflect.DeepEqual(first, second)
}
