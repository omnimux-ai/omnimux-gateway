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

// sandAIIconPath is the canonical 24x24 monochrome SVG `<path d>` derived from
// the official Sand AI favicon mark (https://sand.ai/migrated/assets/svg/favicon.svg).
const sandAIIconPath = "M19.208 19.288C19.208 20.237 19.208 20.712 19.023 21.075C18.861 21.393 18.601 21.653 18.282 21.815C17.919 22 17.444 22 16.495 22H7.505C7.446 22 7.389 22 7.333 22C6.497 21.999 6.058 21.988 5.718 21.815C5.399 21.653 5.139 21.393 4.977 21.075C4.792 20.712 4.792 20.237 4.792 19.288V17.977C4.792 16.528 4.792 15.804 5.124 15.197C5.385 14.718 6.018 14.245 6.872 13.683C7.222 13.453 7.397 13.337 7.598 13.336C7.8 13.335 7.979 13.45 8.338 13.68L9.23 14.254C9.63 14.511 9.83 14.639 9.83 14.824C9.83 15.009 9.63 15.138 9.23 15.395L6.717 17.009C6.363 17.237 6.149 17.629 6.149 18.05C6.149 18.734 6.703 19.288 7.387 19.288H16.613C17.297 19.288 17.851 18.734 17.851 18.05C17.851 17.629 17.637 17.237 17.283 17.009L7.284 10.586C6.065 9.803 5.455 9.411 5.124 8.803C4.792 8.196 4.792 7.472 4.792 6.023V4.712C4.792 3.763 4.792 3.288 4.977 2.925C5.139 2.607 5.399 2.347 5.718 2.185C6.081 2 6.556 2 7.505 2L16.495 2C17.444 2 17.919 2 18.282 2.185C18.601 2.347 18.861 2.607 19.023 2.925C19.208 3.288 19.208 3.763 19.208 4.712V6.023C19.208 7.472 19.208 8.196 18.877 8.803C18.615 9.282 17.982 9.755 17.128 10.317C16.778 10.548 16.603 10.663 16.402 10.664C16.2 10.665 16.021 10.55 15.662 10.32L14.77 9.746C14.37 9.489 14.17 9.361 14.17 9.176C14.17 8.991 14.37 8.862 14.77 8.606L17.282 6.991C17.637 6.764 17.851 6.371 17.851 5.95C17.851 5.266 17.297 4.712 16.613 4.712L7.387 4.712C6.703 4.712 6.149 5.266 6.149 5.95C6.149 6.371 6.363 6.764 6.717 6.991L16.716 13.414C17.935 14.198 18.545 14.589 18.877 15.197C19.208 15.804 19.208 16.528 19.208 17.977V19.288Z"

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
	"Sand AI":            {ID: "sand-ai", Name: "Sand AI", IconPath: sandAIIconPath},
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

var canonicalBrandIconPaths = map[string]string{
	"sand-ai": sandAIIconPath,
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
		if iconPath == "" {
			iconPath = canonicalBrandIconPaths[explicitID]
			if err := ValidateBrandIconPath(iconPath); err != nil {
				return BrandMeta{}, err
			}
		}
		return BrandMeta{ID: explicitID, Name: name, IconPath: iconPath}, nil
	}

	mapped, ok := vendorBrandTable[vendor]
	if ok {
		name := mapped.Name
		if explicitName != "" {
			name = explicitName
		}
		if iconPath == "" {
			iconPath = mapped.IconPath
			if err := ValidateBrandIconPath(iconPath); err != nil {
				return BrandMeta{}, err
			}
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
