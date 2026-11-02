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
	ModelID             string            `json:"model_id"`
	Family              string            `json:"family,omitempty"`
	Status              string            `json:"status,omitempty"`
	Basis               string            `json:"basis,omitempty"`
	ServiceNotes        string            `json:"service_notes,omitempty"`
	Operations          []string          `json:"operations,omitempty"`
	InputModalities     []string          `json:"input_modalities,omitempty"`
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

// Document is the whole shipped payload.
type Document struct {
	Version string              `json:"version"`
	Models  map[string]Contract `json:"models"`
}

// specFile mirrors the subset of the governance spec schema this package
// projects. Keys the spec carries for provenance (sources, notes, unknowns,
// routing) are deliberately not read: they are authoring material, not contract.
// Only explicitly authored service_notes are published as customer guidance.
type specFile struct {
	ModelID             string            `json:"model_id"`
	Family              string            `json:"family"`
	Status              string            `json:"status"`
	Basis               string            `json:"basis,omitempty"`
	ServiceNotes        string            `json:"service_notes,omitempty"`
	Operations          []string          `json:"operations"`
	InputModalities     []string          `json:"input_modalities"`
	DurationsSeconds    *DurationEnvelope `json:"durations_seconds"`
	Resolutions         []string          `json:"resolutions"`
	AspectRatios        []string          `json:"aspect_ratios"`
	AudioSupported      *bool             `json:"audio_supported"`
	OutputFormats       []string          `json:"output_formats"`
	OutputSizes         []string          `json:"output_sizes"`
	ReferenceImages     *MediaLimits      `json:"reference_images"`
	ReferenceVideos     *MediaLimits      `json:"reference_videos"`
	ReferenceAudios     *MediaLimits      `json:"reference_audios"`
	PromptMaxChars      *int              `json:"prompt_max_chars"`
	PromptMaxUTF8Bytes  *int              `json:"prompt_max_utf8_bytes"`
	PromptMaxTokens     *int              `json:"prompt_max_tokens"`
	ContextWindowTokens *int              `json:"context_window_tokens"`
	MaxOutputTokens     *int              `json:"max_output_tokens"`
	Billing             *Billing          `json:"billing"`
}

// lineRestrictedBasis marks a spec that describes our own line's restrictions
// rather than the vendor model's published capabilities.
const lineRestrictedBasis = "line_restricted"

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

	doc := &Document{Models: make(map[string]Contract)}
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
		doc.Models[spec.ModelID] = Contract{
			ModelID:             spec.ModelID,
			Family:              spec.Family,
			Status:              spec.Status,
			Basis:               spec.Basis,
			ServiceNotes:        spec.ServiceNotes,
			Operations:          canonical(spec.Operations),
			InputModalities:     canonical(spec.InputModalities),
			DurationsSeconds:    spec.DurationsSeconds,
			Resolutions:         canonical(spec.Resolutions),
			AspectRatios:        canonical(spec.AspectRatios),
			AudioSupported:      spec.AudioSupported,
			OutputFormats:       canonical(spec.OutputFormats),
			OutputSizes:         canonical(spec.OutputSizes),
			ReferenceImages:     normalizeMedia(spec.ReferenceImages),
			ReferenceVideos:     normalizeMedia(spec.ReferenceVideos),
			ReferenceAudios:     normalizeMedia(spec.ReferenceAudios),
			PromptMaxChars:      spec.PromptMaxChars,
			PromptMaxUTF8Bytes:  spec.PromptMaxUTF8Bytes,
			PromptMaxTokens:     spec.PromptMaxTokens,
			ContextWindowTokens: spec.ContextWindowTokens,
			MaxOutputTokens:     spec.MaxOutputTokens,
			Billing:             spec.Billing,
		}
	}
	doc.Version = Version(doc)
	return doc, nil
}

// Version derives the payload version from its own contents, so any change to a
// projected contract changes the version and a reviewer can tell at a glance
// whether the committed payload moved.
func Version(doc *Document) string {
	ids := make([]string, 0, len(doc.Models))
	for id := range doc.Models {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	digest := sha256.New()
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
