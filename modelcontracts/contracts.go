// Package modelcontracts publishes the per-model capability contract that
// docs/model-governance/specs already authors.
//
// The specs are written as documentation under docs/, which .dockerignore
// excludes from the build context, so a gateway image cannot read them where
// they are written and cannot embed them either. This package therefore carries
// a generated, committed projection of them (contracts.json) inside the binary,
// and its tests rebuild that projection from the specs so the shipped data and
// the authored source cannot drift apart unnoticed.
//
// The projection is descriptive only. Nothing here may participate in pricing,
// quota, precharge or settlement.
package modelcontracts

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
)

//go:embed contracts.json
var payload []byte

// DurationEnvelope is the output-length domain a model accepts.
type DurationEnvelope struct {
	Min     *float64  `json:"min,omitempty"`
	Max     *float64  `json:"max,omitempty"`
	Default *float64  `json:"default,omitempty"`
	Allowed []float64 `json:"allowed,omitempty"`
}

// MediaLimits is the ceiling and accepted formats for one reference family.
type MediaLimits struct {
	Min            *int     `json:"min,omitempty"`
	Max            *int     `json:"max,omitempty"`
	Formats        []string `json:"formats,omitempty"`
	MaxBytesEach   *int64   `json:"max_bytes_each,omitempty"`
	MaxSecondsEach *float64 `json:"max_seconds_each,omitempty"`
}

// Contract is one model's declared capability envelope.
//
// Every field is omitted when the spec does not declare it: an absent axis means
// "not established", which is information the caller must not replace with a
// default of its own.
type Contract struct {
	ModelID string `json:"model_id"`
	// SchemaVersion names the shape of this contract ("2.0"). It is repeated on
	// every contract so a caller holding one object can parse it without the
	// enclosing document.
	SchemaVersion string `json:"schemaVersion"`
	// ContractVersion is a deterministic 16-hex content hash of this contract,
	// computed with the field itself excluded. Any projected change moves it.
	ContractVersion string `json:"contractVersion,omitempty"`
	// ExecutionStatus states whether this model has a verified executable
	// gateway path. `live` requires at least one executable operation; `stub`
	// means the model is catalogued but has no verified execution profile yet;
	// `unavailable` means the model is not offered on any gateway line.
	ExecutionStatus string `json:"execution_status"`
	// ExecutionReason is required for any non-live status so a client can tell
	// "not implemented" apart from "not applicable".
	ExecutionReason string `json:"execution_reason,omitempty"`
	Family          string `json:"family,omitempty"`
	Status          string `json:"status,omitempty"`
	Basis           string `json:"basis,omitempty"`
	ServiceNotes    string `json:"service_notes,omitempty"`
	// OperationIDs is the v1 compatibility view: the same operation ids, as
	// strings, for clients that cannot read the object array below.
	OperationIDs    []string    `json:"operation_ids,omitempty"`
	Operations      []Operation `json:"operations,omitempty"`
	Unknowns        []string    `json:"unknowns,omitempty"`
	InputModalities []string    `json:"input_modalities,omitempty"`

	ToolCalling      *bool      `json:"tool_calling,omitempty"`
	Streaming        *bool      `json:"streaming,omitempty"`
	PromptCaching    *bool      `json:"prompt_caching,omitempty"`
	AudioKind        string     `json:"audio_kind,omitempty"`
	InputFormats     []string   `json:"input_formats,omitempty"`
	MaxInputBytes    *int64     `json:"max_input_bytes,omitempty"`
	MaxInputSeconds  *int       `json:"max_input_seconds,omitempty"`
	MaxOutputSeconds *int       `json:"max_output_seconds,omitempty"`
	Voices           *int       `json:"voices,omitempty"`
	Languages        []string   `json:"languages,omitempty"`
	Endpoints        []Endpoint `json:"endpoints,omitempty"`
	Auth             string     `json:"auth,omitempty"`
	RateLimit        any        `json:"rate_limit,omitempty"`
	Pagination       string     `json:"pagination,omitempty"`
	ResponseFields   []string   `json:"response_fields,omitempty"`

	DurationsSeconds    *DurationEnvelope `json:"durations_seconds,omitempty"`
	Resolutions         []string          `json:"resolutions,omitempty"`
	AspectRatios        []string          `json:"aspect_ratios,omitempty"`
	AudioSupported      *bool             `json:"audio_supported,omitempty"`
	OutputFormats       []string          `json:"output_formats,omitempty"`
	OutputSizes         []string          `json:"output_sizes,omitempty"`
	ReferenceImages     *MediaLimits      `json:"reference_images,omitempty"`
	ReferenceVideos     *MediaLimits      `json:"reference_videos,omitempty"`
	ReferenceAudios     *MediaLimits      `json:"reference_audios,omitempty"`
	PromptMaxChars      *int              `json:"prompt_max_chars,omitempty"`
	PromptMaxUTF8Bytes  *int              `json:"prompt_max_utf8_bytes,omitempty"`
	PromptMaxTokens     *int              `json:"prompt_max_tokens,omitempty"`
	ContextWindowTokens *int              `json:"context_window_tokens,omitempty"`
	MaxOutputTokens     *int              `json:"max_output_tokens,omitempty"`

	// ReasoningEfforts maps the user-facing reasoning level to the wire value a
	// client sends upstream. A level the spec does not establish stays absent,
	// and DefaultReasoningEffort is meaningful only when it names a key here.
	ReasoningEfforts       map[string]string `json:"reasoning_efforts,omitempty"`
	DefaultReasoningEffort string            `json:"default_reasoning_effort,omitempty"`

	// Billing declares how this model is charged, so a client can price a call
	// before submitting instead of inferring the unit from a bare number.
	//
	// fork: OmniMux — /api/pricing used to publish only a price and a two-state
	// quota_type, so a per-second video, a per-request video and a per-image
	// model were indistinguishable: all three shipped a single number. Callers
	// could not tell whether 0.0714 meant "per second" or "per call".
	//
	// Descriptive data only: it must never feed pricing, quota, precharge or
	// settlement. See FORK_CUSTOMIZATIONS.md.
	Billing *Billing `json:"billing,omitempty"`
}

// Billing is the published billing declaration for one model. Unit is the
// settlement unit the caller is charged per; UnitPriceUSD is the price of one
// such unit in US dollars. Per-token models carry ModelRatio instead of a unit
// price, because their charge scales with reported usage.
type Billing struct {
	Unit         string            `json:"unit"`
	UnitLabel    map[string]string `json:"unit_label,omitempty"`
	UnitPriceUSD *float64          `json:"unit_price_usd,omitempty"`
	ModelRatio   *float64          `json:"model_ratio,omitempty"`
	PricingBasis string            `json:"pricing_basis,omitempty"`
	Notes        map[string]string `json:"notes,omitempty"`
}

// Endpoint is one published callable route of a data-family contract.
type Endpoint struct {
	Name   string `json:"name,omitempty"`
	Method string `json:"method,omitempty"`
	Path   string `json:"path,omitempty"`
}

// Document is the whole shipped payload.
type Document struct {
	SchemaVersion string              `json:"schemaVersion"`
	Version       string              `json:"version"`
	Defaults      map[string]string   `json:"defaults,omitempty"`
	Models        map[string]Contract `json:"models"`
}

// specFile mirrors the subset of the governance spec schema this package
// projects. Keys the spec carries for provenance (sources, notes, unknowns,
// routing) are deliberately not read: they are authoring material, not contract.
// Only explicitly authored service_notes are published as customer guidance.
type specFile struct {
	ModelID                string            `json:"model_id"`
	Family                 string            `json:"family"`
	Status                 string            `json:"status"`
	Basis                  string            `json:"basis,omitempty"`
	ServiceNotes           string            `json:"service_notes,omitempty"`
	Operations             []string          `json:"operations"`
	Gateway                *GatewaySpec      `json:"gateway"`
	Execution              *ExecutionSpec    `json:"execution"`
	Parameters             []ParameterSpec   `json:"parameters"`
	InputModalities        []string          `json:"input_modalities"`
	ToolCalling            *bool             `json:"tool_calling"`
	Streaming              *bool             `json:"streaming"`
	PromptCaching          *bool             `json:"prompt_caching"`
	AudioKind              string            `json:"audio_kind"`
	InputFormats           []string          `json:"input_formats"`
	MaxInputBytes          *int64            `json:"max_input_bytes"`
	MaxInputSeconds        *int              `json:"max_input_seconds"`
	MaxOutputSeconds       *int              `json:"max_output_seconds"`
	Voices                 any               `json:"voices"`
	Languages              any               `json:"languages"`
	Endpoints              []Endpoint        `json:"endpoints"`
	Auth                   string            `json:"auth"`
	RateLimit              any               `json:"rate_limit"`
	Pagination             string            `json:"pagination"`
	ResponseFields         []string          `json:"response_fields"`
	DurationsSeconds       *DurationEnvelope `json:"durations_seconds"`
	Resolutions            []string          `json:"resolutions"`
	AspectRatios           []string          `json:"aspect_ratios"`
	AudioSupported         *bool             `json:"audio_supported"`
	OutputFormats          []string          `json:"output_formats"`
	OutputSizes            []string          `json:"output_sizes"`
	ReferenceImages        *MediaLimits      `json:"reference_images"`
	ReferenceVideos        *MediaLimits      `json:"reference_videos"`
	ReferenceAudios        *MediaLimits      `json:"reference_audios"`
	PromptMaxChars         *int              `json:"prompt_max_chars"`
	PromptMaxUTF8Bytes     *int              `json:"prompt_max_utf8_bytes"`
	PromptMaxTokens        *int              `json:"prompt_max_tokens"`
	ContextWindowTokens    *int              `json:"context_window_tokens"`
	MaxOutputTokens        *int              `json:"max_output_tokens"`
	ReasoningEfforts       map[string]string `json:"reasoning_efforts"`
	DefaultReasoningEffort string            `json:"default_reasoning_effort"`
	DefaultFor             []string          `json:"default_for"`
	Billing                *Billing          `json:"billing"`
}

// lineRestrictedBasis marks a spec that describes our own line's restrictions
// rather than the vendor model's published capabilities.
const lineRestrictedBasis = "line_restricted"

// defaultOperationFamily is the closed vocabulary of gateway-recommended
// defaults. A default recommendation is product policy, not a vendor capability
// claim, so it lives in the governance spec as `default_for` and is aggregated
// into the published document without a per-model source claim.
var defaultOperationFamily = map[string]string{
	"chat":           "llm",
	"vision_chat":    "llm",
	"text_to_image":  "image",
	"text_to_video":  "video",
	"text_to_music":  "audio",
	"speech_to_text": "audio",
}

// canonical collapses a declared-but-empty list to absent.
//
// A spec writes `"aspect_ratios": []` to mean "this axis has no vocabulary" and
// omits the key to mean "not established". Both reach every consumer as "there
// is nothing to offer here", and a JSON round-trip cannot tell them apart
// anyway (`omitempty` drops the empty list, parsing restores it as absent).
// Collapsing them in one place keeps the shipped payload stable and the drift
// gate honest about what it compares.
func canonical[T any](values []T) []T {
	if len(values) == 0 {
		return nil
	}
	return values
}

// normalizeMedia applies the same rule to a media family's format list.
func normalizeMedia(limits *MediaLimits) *MediaLimits {
	if limits == nil {
		return nil
	}
	normalized := *limits
	normalized.Formats = canonical(limits.Formats)
	return &normalized
}

// Build projects every governance spec in specDir into a payload document.
//
// It is the single implementation of the projection: the generator writes what
// it returns and the drift test compares against it, so the two cannot disagree
// about what a contract is.
func Build(specDir string) (*Document, error) {
	entries, err := os.ReadDir(specDir)
	if err != nil {
		return nil, err
	}

	doc := &Document{SchemaVersion: SchemaVersion, Models: make(map[string]Contract)}
	specs := make([]specFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(specDir, entry.Name()))
		if err != nil {
			return nil, err
		}
		var spec specFile
		if err := common.Unmarshal(raw, &spec); err != nil {
			return nil, &SpecError{File: entry.Name(), Err: err}
		}
		if spec.ModelID == "" {
			return nil, &SpecError{File: entry.Name(), Err: errNoModelID}
		}
		contract, err := projectContract(spec)
		if err != nil {
			return nil, &SpecError{File: entry.Name(), Err: err}
		}
		specs = append(specs, spec)
		doc.Models[spec.ModelID] = contract
	}
	defaults, err := projectDefaults(specs)
	if err != nil {
		return nil, err
	}
	doc.Defaults = defaults
	doc.Version = Version(doc)
	return doc, nil
}

// projectDefaults aggregates the optional `default_for` recommendation keys
// declared by the governance specs. Duplicate or mis-family recommendations are
// authoring errors: a silent overwrite would make the published recommendation
// depend on file iteration order.
func projectDefaults(specs []specFile) (map[string]string, error) {
	defaults := make(map[string]string)
	for _, spec := range specs {
		seen := make(map[string]struct{}, len(spec.DefaultFor))
		for _, operation := range spec.DefaultFor {
			family, known := defaultOperationFamily[operation]
			if !known {
				return nil, fmt.Errorf("model %s: unknown default_for %q", spec.ModelID, operation)
			}
			if spec.Family != family {
				return nil, fmt.Errorf("model %s: default_for %q requires family %q, got %q", spec.ModelID, operation, family, spec.Family)
			}
			if _, duplicate := seen[operation]; duplicate {
				return nil, fmt.Errorf("model %s: repeated default_for %q", spec.ModelID, operation)
			}
			seen[operation] = struct{}{}
			if previous, claimed := defaults[operation]; claimed {
				return nil, fmt.Errorf("default_for %q claimed by both %s and %s", operation, previous, spec.ModelID)
			}
			defaults[operation] = spec.ModelID
		}
	}
	if len(defaults) == 0 {
		return nil, nil
	}
	return defaults, nil
}

// projectContract assembles one contract, including its operation subtree and
// per-model content hash. It does not know which file it came from; Build adds
// that context to any error.
func projectContract(spec specFile) (Contract, error) {
	if spec.DefaultReasoningEffort != "" {
		if _, ok := spec.ReasoningEfforts[spec.DefaultReasoningEffort]; !ok {
			return Contract{}, fmt.Errorf("default_reasoning_effort %q is not a key in reasoning_efforts", spec.DefaultReasoningEffort)
		}
	}
	for level, wire := range spec.ReasoningEfforts {
		if strings.TrimSpace(level) == "" || strings.TrimSpace(wire) == "" {
			return Contract{}, fmt.Errorf("reasoning_efforts contains an empty level or wire value")
		}
	}
	operations, operationUnknowns, err := projectOperations(spec)
	if err != nil {
		return Contract{}, err
	}
	executionStatus, executionReason, err := executionStateFor(spec, operations)
	if err != nil {
		return Contract{}, err
	}
	contract := Contract{
		ModelID:                spec.ModelID,
		SchemaVersion:          SchemaVersion,
		ExecutionStatus:        executionStatus,
		ExecutionReason:        executionReason,
		Family:                 spec.Family,
		Status:                 spec.Status,
		Basis:                  spec.Basis,
		ServiceNotes:           spec.ServiceNotes,
		OperationIDs:           operationIDs(operations),
		Operations:             operations,
		Unknowns:               mergeUnknowns(modelUnknowns(spec), operationUnknowns),
		InputModalities:        canonical(spec.InputModalities),
		ToolCalling:            spec.ToolCalling,
		Streaming:              spec.Streaming,
		PromptCaching:          spec.PromptCaching,
		AudioKind:              spec.AudioKind,
		InputFormats:           canonical(spec.InputFormats),
		MaxInputBytes:          spec.MaxInputBytes,
		MaxInputSeconds:        spec.MaxInputSeconds,
		MaxOutputSeconds:       spec.MaxOutputSeconds,
		Voices:                 numericVoices(spec.Voices),
		Languages:              stringLanguages(spec.Languages),
		Endpoints:              canonical(spec.Endpoints),
		Auth:                   spec.Auth,
		RateLimit:              spec.RateLimit,
		Pagination:             spec.Pagination,
		ResponseFields:         canonical(spec.ResponseFields),
		DurationsSeconds:       spec.DurationsSeconds,
		Resolutions:            canonical(spec.Resolutions),
		AspectRatios:           canonical(spec.AspectRatios),
		AudioSupported:         spec.AudioSupported,
		OutputFormats:          canonical(spec.OutputFormats),
		OutputSizes:            canonical(spec.OutputSizes),
		ReferenceImages:        normalizeMedia(spec.ReferenceImages),
		ReferenceVideos:        normalizeMedia(spec.ReferenceVideos),
		ReferenceAudios:        normalizeMedia(spec.ReferenceAudios),
		PromptMaxChars:         spec.PromptMaxChars,
		PromptMaxUTF8Bytes:     spec.PromptMaxUTF8Bytes,
		PromptMaxTokens:        spec.PromptMaxTokens,
		ContextWindowTokens:    spec.ContextWindowTokens,
		MaxOutputTokens:        spec.MaxOutputTokens,
		ReasoningEfforts:       spec.ReasoningEfforts,
		DefaultReasoningEffort: spec.DefaultReasoningEffort,
		Billing:                spec.Billing,
	}
	version, err := contractVersion(contract)
	if err != nil {
		return Contract{}, err
	}
	contract.ContractVersion = version
	return contract, nil
}

// executionStateFor derives the published execution status. A model with a
// verified gateway path is live; a model whose spec explicitly declares a
// non-live state must say why; a video model without a gateway block stays a
// stub instead of silently passing as executable.
func executionStateFor(spec specFile, operations []Operation) (string, string, error) {
	hasExecutable := false
	for _, operation := range operations {
		if operation.Executable {
			hasExecutable = true
			break
		}
	}

	if spec.Execution != nil {
		status := strings.TrimSpace(spec.Execution.Status)
		reason := strings.TrimSpace(spec.Execution.Reason)
		switch status {
		case "live":
			if !hasExecutable {
				return "", "", fmt.Errorf("execution.status=live requires at least one executable operation")
			}
			return "live", reason, nil
		case "stub", "unavailable":
			if hasExecutable {
				return "", "", fmt.Errorf("execution.status=%s contradicts executable operations", status)
			}
			if reason == "" {
				return "", "", fmt.Errorf("execution.status=%s requires execution.reason", status)
			}
			return status, reason, nil
		default:
			return "", "", fmt.Errorf("unknown execution.status %q", status)
		}
	}

	if hasExecutable {
		return "live", "", nil
	}
	return "stub", "no verified gateway operation profile is published for this model", nil
}

// unknownList keeps generated unknown paths ordered and duplicate-free.
type unknownList struct {
	paths []string
	seen  map[string]bool
}

func newUnknownList() *unknownList {
	return &unknownList{seen: map[string]bool{}}
}

func (list *unknownList) add(path string) {
	if path == "" || list.seen[path] {
		return
	}
	list.seen[path] = true
	list.paths = append(list.paths, path)
}

// mergeUnknowns concatenates generated path groups in a stable order.
func mergeUnknowns(groups ...[]string) []string {
	list := newUnknownList()
	for _, group := range groups {
		for _, path := range group {
			list.add(path)
		}
	}
	return list.paths
}

// modelUnknowns builds the dot-separated field paths for family-required axes
// the spec does not establish. Governance prose in spec.unknowns is authoring
// material and is deliberately never copied here.
func modelUnknowns(spec specFile) []string {
	list := newUnknownList()
	if spec.PromptMaxChars == nil {
		switch spec.Family {
		case "image", "video", "llm":
			list.add("prompt_max_chars")
		case "audio":
			// Speech-to-text consumes media, not a prompt.
			if spec.AudioKind != "asr" {
				list.add("prompt_max_chars")
			}
		}
	}
	switch spec.Family {
	case "video":
		if spec.DurationsSeconds == nil {
			list.add("durations_seconds")
			list.add("durations_seconds.min")
			list.add("durations_seconds.max")
		} else {
			if spec.DurationsSeconds.Min == nil {
				list.add("durations_seconds.min")
			}
			if spec.DurationsSeconds.Max == nil {
				list.add("durations_seconds.max")
			}
		}
		if len(spec.Resolutions) == 0 {
			list.add("resolutions")
		}
		if len(spec.AspectRatios) == 0 {
			list.add("aspect_ratios")
		}
		addMediaUnknowns(list, spec.ReferenceImages, "reference_images")
		addMediaUnknowns(list, spec.ReferenceVideos, "reference_videos")
		addMediaUnknowns(list, spec.ReferenceAudios, "reference_audios")
	case "image":
		if spec.ReferenceImages == nil {
			list.add("reference_images")
			list.add("reference_images.max")
			list.add("reference_images.max_bytes_each")
		} else {
			if spec.ReferenceImages.Max == nil {
				list.add("reference_images.max")
			}
			if spec.ReferenceImages.MaxBytesEach == nil {
				list.add("reference_images.max_bytes_each")
			}
		}
		if len(spec.OutputSizes) == 0 {
			list.add("output_sizes")
		}
		if len(spec.OutputFormats) == 0 {
			list.add("output_formats")
		}
	case "audio":
		if spec.AudioKind == "" {
			list.add("audio_kind")
		}
		if len(spec.OutputFormats) == 0 {
			list.add("output_formats")
		}
		if spec.MaxInputBytes == nil {
			list.add("max_input_bytes")
		}
		if spec.MaxInputSeconds == nil {
			list.add("max_input_seconds")
		}
	case "llm":
		if spec.ContextWindowTokens == nil {
			list.add("context_window_tokens")
		}
		if len(spec.InputModalities) == 0 {
			list.add("input_modalities")
		}
		if spec.MaxOutputTokens == nil {
			list.add("max_output_tokens")
		}
	case "data":
		if len(spec.Endpoints) == 0 {
			list.add("endpoints")
		}
		if spec.Auth == "" {
			list.add("auth")
		}
	}
	return list.paths
}

// addMediaUnknowns records missing per-family limits. A family that explicitly
// caps at zero is not a gap: the family is intentionally not accepted.
func addMediaUnknowns(list *unknownList, limits *MediaLimits, path string) {
	if limits == nil || (limits.Max != nil && *limits.Max == 0) {
		return
	}
	if limits.Max == nil {
		list.add(path + ".max")
	}
	if limits.MaxBytesEach == nil {
		list.add(path + ".max_bytes_each")
	}
}

// contractVersion hashes the contract with its own version field excluded, so
// the hash is stable and cannot depend on itself.
func contractVersion(contract Contract) (string, error) {
	contract.ContractVersion = ""
	body, err := common.Marshal(contract)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])[:16], nil
}

// Version derives the payload version from its own contents, so any change to a
// projected contract changes the version and a reviewer can tell at a glance
// whether the committed payload moved.
//
// A document that already carries a version returns it unchanged: Build and
// All have already computed it, and callers asking for the version of a loaded
// document should not pay to re-hash every contract.
func Version(doc *Document) string {
	if doc.Version != "" {
		return doc.Version
	}
	ids := make([]string, 0, len(doc.Models))
	for id := range doc.Models {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	digest := sha256.New()
	digest.Write([]byte(doc.SchemaVersion))
	digest.Write([]byte{0})
	defaultKeys := make([]string, 0, len(doc.Defaults))
	for operation := range doc.Defaults {
		defaultKeys = append(defaultKeys, operation)
	}
	sort.Strings(defaultKeys)
	for _, operation := range defaultKeys {
		digest.Write([]byte(operation))
		digest.Write([]byte{0})
		digest.Write([]byte(doc.Defaults[operation]))
		digest.Write([]byte{0})
	}
	for _, id := range ids {
		body, err := common.Marshal(doc.Models[id])
		if err != nil {
			continue
		}
		digest.Write([]byte(id))
		digest.Write([]byte{0})
		digest.Write(body)
		digest.Write([]byte{0})
	}
	return hex.EncodeToString(digest.Sum(nil))[:16]
}

// numericVoices publishes a voice count only when the spec states a number.
// Some specs describe voices in prose; that description is not a count and is
// deliberately not published as one.
func numericVoices(value any) *int {
	switch typed := value.(type) {
	case float64:
		if typed != float64(int64(typed)) {
			return nil
		}
		count := int(typed)
		return &count
	case int:
		count := typed
		return &count
	default:
		return nil
	}
}

// stringLanguages publishes a language list only when every entry is a string.
// A prose description or a numeric code is not a language list.
func stringLanguages(value any) []string {
	switch typed := value.(type) {
	case []string:
		return canonical(typed)
	case []any:
		languages := make([]string, 0, len(typed))
		for _, entry := range typed {
			language, ok := entry.(string)
			if !ok {
				return nil
			}
			languages = append(languages, language)
		}
		return canonical(languages)
	default:
		return nil
	}
}

var (
	loadOnce sync.Once
	loaded   Document
	loadErr  error
)

// All returns the shipped payload, parsed once on first use.
//
// A payload that cannot be parsed yields an empty document, never a panic: the
// catalog must keep serving even when this optional data is unreadable.
func All() Document {
	loadOnce.Do(func() {
		if err := common.Unmarshal(payload, &loaded); err != nil {
			loadErr = err
			common.SysError("modelcontracts: shipped payload is unreadable: " + err.Error())
			loaded = Document{Models: map[string]Contract{}}
		}
		if loaded.Models == nil {
			loaded.Models = map[string]Contract{}
		}
	})
	return loaded
}

// LoadError reports why All() fell back to an empty document, if it did.
func LoadError() error { return loadErr }

// SpecDirEnv names a directory of governance specs the gateway may serve instead
// of the projection embedded at build time.
//
// It exists so a contract correction is a configuration change: an operator
// edits the specs and restarts, without a code change or a rebuild. Leave it
// unset and the embedded payload is used, so the shipped behaviour does not
// depend on anything being mounted.
const SpecDirEnv = "OMNIMUX_MODEL_SPECS_DIR"

// ApplySpecDir serves the contracts projected from dir instead of the embedded
// payload.
//
// Call it once during startup, before the server accepts traffic: it swaps the
// document without locking, which is only safe while nothing reads it. A dir
// that cannot be read, or that holds no specs, is refused as an error and
// leaves the embedded payload in place — a bad mount must not blank the
// catalog's contracts, and must not stop the gateway.
func ApplySpecDir(dir string) error {
	built, err := Build(dir)
	if err != nil {
		return err
	}
	if len(built.Models) == 0 {
		return fmt.Errorf("modelcontracts: %s holds no specs", dir)
	}
	loaded = *built
	loadErr = nil
	// Mark the lazy load satisfied so All() serves what was just built rather
	// than re-parsing the embedded payload over it.
	loadOnce.Do(func() {})
	return nil
}

// For returns the contract for a model id.
//
// The catalog and the specs are 1:1, so an exact match is the normal path. The
// hyphenated retry covers the two spellings the catalog is documented to use
// (`seedance-2.5` and `seedance-2-5` are both in the wild).
func For(modelID string) (Contract, bool) {
	models := All().Models
	if contract, ok := models[modelID]; ok {
		return contract, true
	}
	hyphenated := strings.ReplaceAll(strings.ToLower(modelID), ".", "-")
	if contract, ok := models[hyphenated]; ok {
		return contract, true
	}
	return Contract{}, false
}

// Count reports how many models the shipped payload carries.
func Count() int { return len(All().Models) }
