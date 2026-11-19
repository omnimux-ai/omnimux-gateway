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

const VideoCapabilitiesFileEnv = "OMNIMUX_VIDEO_CAPABILITIES_FILE"

// VideoRequest describes the operation and inputs already normalized by the
// gateway. Slots contain counts by contract slot. Parameters include explicit
// zero and false as well as defaults filled by the selected protocol.
type VideoRequest struct {
	Operation  string
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
}

type VideoChannelIdentity struct {
	ChannelType   int
	Profile       string
	UpstreamModel string
	BaseURL       string
}

// VideoCapabilities is an immutable snapshot keyed by the public model and
// physical channel. A model with an empty rule set is enrolled in the pilot.
type VideoCapabilities struct {
	models map[string]map[int]VideoChannelRule
}

var activeVideoCapabilities atomic.Pointer[VideoCapabilities]

func ActiveVideoCapabilities() *VideoCapabilities {
	return activeVideoCapabilities.Load()
}

func SetVideoCapabilities(rules *VideoCapabilities) *VideoCapabilities {
	return activeVideoCapabilities.Swap(rules)
}

func (rules *VideoCapabilities) rulesForModel(modelID string) map[int]VideoChannelRule {
	if rules == nil {
		return nil
	}
	if channels, ok := rules.models[modelID]; ok {
		return channels
	}
	return rules.models[strings.ReplaceAll(strings.ToLower(modelID), ".", "-")]
}

func (rules *VideoCapabilities) Managed(modelID string) bool {
	return rules.rulesForModel(modelID) != nil
}

func (rules *VideoCapabilities) ChannelIDs() []int {
	if rules == nil {
		return nil
	}
	seen := make(map[int]struct{})
	var ids []int
	for _, channels := range rules.models {
		for id := range channels {
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
		for _, channels := range rules.models {
			if _, found := channels[id]; found {
				return true
			}
		}
	}
	return false
}

func (rules *VideoCapabilities) ChannelIdentity(modelID string, id int) (VideoChannelIdentity, bool) {
	channel, found := rules.rulesForModel(modelID)[id]
	return VideoChannelIdentity{ChannelType: channel.ChannelType, Profile: channel.Profile, UpstreamModel: channel.UpstreamModel, BaseURL: channel.BaseURL}, found
}

func (rules *VideoCapabilities) Accepts(modelID string, id int, request VideoRequest) bool {
	channel := rules.rulesForModel(modelID)[id]
	return slices.ContainsFunc(channel.Operations, func(operation Operation) bool {
		return VideoOperationAccepts(operation, request)
	})
}

func (rules *VideoCapabilities) ChannelRule(modelID string, id int) (VideoChannelRule, bool) {
	channel, found := rules.rulesForModel(modelID)[id]
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
// active. The configured channels can only narrow executable pilot operations.
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

	rules := &VideoCapabilities{models: make(map[string]map[int]VideoChannelRule, len(document.Models))}
	for model, rawChannels := range document.Models {
		contract, found := For(model)
		if !found || !contract.IsLiveExecutable() {
			return nil, fmt.Errorf("video capabilities: model %q has no live contract", model)
		}
		modelID := contract.ModelID
		var profile string
		switch modelID {
		case "muse-video":
			profile = profileOpenAIVideo
		case "seedance-2-5":
			profile = profileAPIMartVideo
		default:
			return nil, fmt.Errorf("video capabilities: model %q is outside the pilot", model)
		}
		if rawChannels == nil || rules.Managed(modelID) {
			return nil, fmt.Errorf("video capabilities: model %q needs one channel array", model)
		}

		channels := make(map[int]VideoChannelRule, len(rawChannels))
		for index, raw := range rawChannels {
			channel, err := compileVideoChannelRule(raw, contract, profile)
			if err != nil {
				return nil, fmt.Errorf("video capabilities: model %q channel %d: %w", modelID, index+1, err)
			}
			if _, found := channels[channel.ChannelID]; found {
				return nil, fmt.Errorf("video capabilities: model %q repeats channel %d", modelID, channel.ChannelID)
			}
			channels[channel.ChannelID] = channel
		}
		rules.models[modelID] = channels
	}
	return rules, nil
}

func compileVideoChannelRule(raw common.RawMessage, contract Contract, profile string) (VideoChannelRule, error) {
	var spec struct {
		VideoChannelRule
		Operations []common.RawMessage `json:"operations"`
	}
	if err := decodeVideoRule(raw, &spec, "channel_id", "channel_type", "profile", "upstream_model", "base_url", "operations", "evidence"); err != nil {
		return VideoChannelRule{}, err
	}
	channel := spec.VideoChannelRule
	validType := channel.ChannelType == constant.ChannelTypeApiMart
	if profile == profileOpenAIVideo {
		switch channel.ChannelType {
		case constant.ChannelTypeOpenAI, constant.ChannelTypeSora, constant.ChannelTypeTaskPlugin, constant.ChannelTypeNewAPI:
			validType = true
		default:
			validType = false
		}
	}
	if channel.ChannelID < 1 || !validType || channel.Profile != profile || strings.TrimSpace(channel.UpstreamModel) != channel.UpstreamModel || channel.UpstreamModel == "" {
		return VideoChannelRule{}, fmt.Errorf("channel_id, channel_type, profile and upstream_model must identify the pilot route")
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
		operation, err := compileVideoOperationRule(rawOperation, contract, profile)
		if err != nil {
			return VideoChannelRule{}, err
		}
		channel.Operations = append(channel.Operations, operation)
	}
	return channel, nil
}

func compileVideoOperationRule(raw common.RawMessage, contract Contract, profile string) (Operation, error) {
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
		parameters, err := compileVideoParameterRules(spec.Parameters, operation.Parameters, profile == profileAPIMartVideo)
		if err != nil {
			return Operation{}, fmt.Errorf("operation %q: %w", operation.ID, err)
		}
		operation.Parameters = parameters
	}
	if profile == profileOpenAIVideo {
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
	return operation, nil
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
	if !operation.Executable || operation.ID != request.Operation {
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
			(parameter.Maximum == nil || number <= *parameter.Maximum)
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
