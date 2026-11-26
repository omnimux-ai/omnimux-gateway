package modelcontracts

import (
	"fmt"
	"regexp"
	"strings"
)

// MaxBrandIconPathBytes bounds the optional fallback SVG path data (`<path d>`)
// carried by a brand entry so an oversized path can never bloat `/api/pricing`.
const MaxBrandIconPathBytes = 4096

var (
	brandIDPattern       = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)
	brandIconPathPattern = regexp.MustCompile(`^[MmLlHhVvCcSsQqTtAaZz0-9.,\s+-]+$`)
)

// BrandMeta describes a canonical user-facing model brand published alongside
// the catalog. It is purely presentational: it never feeds billing, quota,
// channel routing, or capability contract hashing.
type BrandMeta struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IconPath string `json:"icon_path,omitempty"`
}

// vendorBrandTable maps authored governance spec `vendor` values to canonical
// cross-client brand identities. Keeping one table in modelcontracts lets
// `make model-contract-check` reject spelling drift before publication.
var vendorBrandTable = map[string]BrandMeta{
	"Alibaba":            {ID: "alibaba", Name: "Alibaba"},
	"Anthropic":          {ID: "anthropic", Name: "Anthropic"},
	"Bilibili":           {ID: "bilibili", Name: "Bilibili"},
	"ByteDance":          {ID: "bytedance", Name: "ByteDance"},
	"DeepSeek":           {ID: "deepseek", Name: "DeepSeek"},
	"Google":             {ID: "google", Name: "Google"},
	"Instagram":          {ID: "instagram", Name: "Instagram"},
	"Kuaishou（可灵 Kling）": {ID: "kling", Name: "Kling"},
	"Midjourney":         {ID: "midjourney", Name: "Midjourney"},
	"MiniMax":            {ID: "minimax", Name: "MiniMax"},
	"Muse":               {ID: "muse", Name: "Muse"},
	"OpenAI":             {ID: "openai", Name: "OpenAI"},
	"Sand AI":            {ID: "sand-ai", Name: "Sand AI"},
	"Suno":               {ID: "suno", Name: "Suno"},
	"TikTok":             {ID: "tiktok", Name: "TikTok"},
	"TypeSafe AI":        {ID: "typesafe-ai", Name: "TypeSafe AI"},
	"Volcengine":         {ID: "bytedance", Name: "ByteDance"},
	"Volcengine（火山引擎 / 豆包 Seedance）": {ID: "bytedance", Name: "ByteDance"},
	"X":             {ID: "x", Name: "X"},
	"YouTube":       {ID: "youtube", Name: "YouTube"},
	"xAI":           {ID: "grok", Name: "xAI"},
	"智谱":            {ID: "zhipu", Name: "Zhipu"},
	"月之暗面 Moonshot": {ID: "kimi", Name: "Kimi"},
	"火山引擎":          {ID: "bytedance", Name: "ByteDance"},
}

var canonicalBrandNames = map[string]string{
	"alibaba":     "Alibaba",
	"anthropic":   "Anthropic",
	"bilibili":    "Bilibili",
	"bytedance":   "ByteDance",
	"deepseek":    "DeepSeek",
	"elevenlabs":  "ElevenLabs",
	"google":      "Google",
	"grok":        "xAI",
	"indextts":    "Index TTS2",
	"instagram":   "Instagram",
	"kimi":        "Kimi",
	"kling":       "Kling",
	"midjourney":  "Midjourney",
	"minimax":     "MiniMax",
	"muse":        "Muse",
	"openai":      "OpenAI",
	"runway":      "Runway",
	"sand-ai":     "Sand AI",
	"stability":   "Stability AI",
	"suno":        "Suno",
	"tiktok":      "TikTok",
	"typesafe-ai": "TypeSafe AI",
	"veo":         "Veo",
	"vidu":        "Vidu",
	"x":           "X",
	"youtube":     "YouTube",
	"zhipu":       "Zhipu",
}

// ValidateBrandID ensures a brand slug is lowercase kebab-case and bounded.
// Leading/trailing whitespace is rejected fail-closed.
func ValidateBrandID(id string) error {
	if id == "" {
		return fmt.Errorf("brand_id must not be empty")
	}
	if len(id) > 64 {
		return fmt.Errorf("brand_id %q exceeds 64 bytes", id)
	}
	if !brandIDPattern.MatchString(id) {
		return fmt.Errorf("brand_id %q must match %s", id, brandIDPattern.String())
	}
	return nil
}

// ValidateBrandIconPath validates an optional 24x24 monochrome SVG `<path d>`
// fallback string. Raw `<svg>` markup, attributes, URLs, scripts, oversize
// payloads, and leading/trailing whitespace are rejected fail-closed.
func ValidateBrandIconPath(path string) error {
	if path == "" {
		return nil
	}
	if len(path) > MaxBrandIconPathBytes {
		return fmt.Errorf("brand_icon_path exceeds %d bytes (got %d)", MaxBrandIconPathBytes, len(path))
	}
	if strings.TrimSpace(path) != path {
		return fmt.Errorf("brand_icon_path must not have leading or trailing whitespace")
	}
	if path[0] != 'M' && path[0] != 'm' {
		return fmt.Errorf("brand_icon_path must begin with an SVG moveto command (M/m)")
	}
	if !brandIconPathPattern.MatchString(path) {
		return fmt.Errorf("brand_icon_path contains disallowed characters (must match %s)", brandIconPathPattern.String())
	}
	return nil
}

func resolveSpecBrand(spec specFile) (BrandMeta, error) {
	explicitID := strings.TrimSpace(spec.BrandID)
	explicitName := strings.TrimSpace(spec.BrandName)
	iconPath := strings.TrimSpace(spec.BrandIconPath)
	vendor := strings.TrimSpace(spec.Vendor)

	if err := ValidateBrandIconPath(iconPath); err != nil {
		return BrandMeta{}, err
	}

	if explicitID == "" && vendor == "" {
		if explicitName != "" || iconPath != "" {
			return BrandMeta{}, fmt.Errorf("brand_name or brand_icon_path requires vendor or brand_id")
		}
		return BrandMeta{}, nil
	}

	if explicitID != "" {
		if err := ValidateBrandID(explicitID); err != nil {
			return BrandMeta{}, err
		}
		name := explicitName
		if name == "" {
			if known, ok := canonicalBrandNames[explicitID]; ok {
				name = known
			} else if vendor != "" {
				name = vendor
			}
		}
		if name == "" {
			return BrandMeta{}, fmt.Errorf("brand_id %q requires a non-empty brand_name", explicitID)
		}
		return BrandMeta{ID: explicitID, Name: name, IconPath: iconPath}, nil
	}

	mapped, ok := vendorBrandTable[vendor]
	if ok {
		name := mapped.Name
		if explicitName != "" {
			name = explicitName
		}
		return BrandMeta{ID: mapped.ID, Name: name, IconPath: iconPath}, nil
	}

	// For runtime-mounted specs (`OMNIMUX_MODEL_SPECS_DIR`) or newly onboarded
	// ASCII vendors that omit explicit `brand_id`, derive a deterministic
	// lowercase kebab-case slug from `vendor` when it contains only ASCII
	// letters, digits, spaces, or hyphens. Non-ASCII vendors require explicit
	// `brand_id` or a table entry so slugs never silently drop text.
	slug := slugifyASCIIVendor(vendor)
	if slug == "" {
		return BrandMeta{}, fmt.Errorf("unmapped vendor %q: declare explicit brand_id/brand_name or register in modelcontracts/brands.go", vendor)
	}
	if err := ValidateBrandID(slug); err != nil {
		return BrandMeta{}, fmt.Errorf("vendor %q cannot derive valid brand_id: %w", vendor, err)
	}
	name := vendor
	if explicitName != "" {
		name = explicitName
	}
	return BrandMeta{ID: slug, Name: name, IconPath: iconPath}, nil
}

func slugifyASCIIVendor(vendor string) string {
	var b strings.Builder
	prevDash := false
	for _, r := range vendor {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r + ('a' - 'A'))
			prevDash = false
		case r == ' ' || r == '-' || r == '_' || r == '/' || r == '.':
			if b.Len() > 0 && !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		default:
			return ""
		}
	}
	return strings.TrimRight(b.String(), "-")
}

// IsMappedVendor reports whether vendor has an explicit canonical mapping in
// vendorBrandTable. Shipped specs use this check in CI to prevent spelling drift.
func IsMappedVendor(vendor string) bool {
	_, ok := vendorBrandTable[strings.TrimSpace(vendor)]
	return ok
}

// projectBrands aggregates canonical brand metadata across all projected specs
// and rejects conflicting display names or icon paths for the same brand_id.
func projectBrands(specs []specFile) (map[string]BrandMeta, error) {
	brands := make(map[string]BrandMeta)
	for _, spec := range specs {
		meta, err := resolveSpecBrand(spec)
		if err != nil {
			return nil, fmt.Errorf("model %s: %w", spec.ModelID, err)
		}
		if meta.ID == "" {
			continue
		}
		if existing, ok := brands[meta.ID]; ok {
			if existing.Name != meta.Name {
				return nil, fmt.Errorf("brand_id %q has conflicting brand_name values: %q vs %q (model %s)", meta.ID, existing.Name, meta.Name, spec.ModelID)
			}
			if existing.IconPath != "" && meta.IconPath != "" && existing.IconPath != meta.IconPath {
				return nil, fmt.Errorf("brand_id %q has conflicting brand_icon_path values (model %s)", meta.ID, spec.ModelID)
			}
			if existing.IconPath == "" && meta.IconPath != "" {
				existing.IconPath = meta.IconPath
				brands[meta.ID] = existing
			}
			continue
		}
		brands[meta.ID] = meta
	}
	if len(brands) == 0 {
		return nil, nil
	}
	return brands, nil
}
