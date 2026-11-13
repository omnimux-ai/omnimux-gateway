// Package influencer hosts the AI Influencer workbench taxonomy: the
// structured "character builder" dataset (18 human categories with 170 options
// across 3 exaggeration tiers, plus cat/dog traits) and prompt assembly shared by the
// session endpoint and, for preview, the web client.
//
// The dataset is embedded at build time (Config-as-Code): option changes ship
// with the binary instead of drifting in the database.
package influencer

import (
	"crypto/sha256"
	_ "embed"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

//go:embed data/taxonomy.json
var taxonomyBytes []byte

type Option struct {
	ID        string   `json:"id"`
	LabelEN   string   `json:"label_en"`
	VisibleIn []string `json:"visibleIn"`
	Slot      *string  `json:"slot"`
	Swatch    string   `json:"swatch"`
	ImageURL  string   `json:"imageUrl"`
}

type Category struct {
	ID      string   `json:"id"`
	LabelEN string   `json:"label_en"`
	Kind    string   `json:"kind"` // media | color | text
	Max     int      `json:"max"`
	Options []Option `json:"options"`
}

type TierGroup struct {
	Options             []Option `json:"options"`
	ComingSoonImageURLs []string `json:"comingSoonImageUrls"`
}

// PromptMap maps category id -> option id -> prompt fragment.
type PromptMap struct {
	Tiers            map[string]string              `json:"tiers"`
	Categories       map[string][]PromptFragmentOpt `json:"categories"`
	AnimalCategories map[string][]PromptFragmentOpt `json:"animal_categories,omitempty"`
}

type PromptFragmentOpt struct {
	ID       string `json:"id"`
	Fragment string `json:"fragment"`
}

type TaxonomyData struct {
	Version          int        `json:"version"`
	Source           string     `json:"source"`
	TierGroup        TierGroup  `json:"tier_group"`
	CategoryPriority []string   `json:"category_priority"`
	Categories       []Category `json:"categories"`
	PromptMap        PromptMap  `json:"prompt_map"`
}

var (
	taxonomy     TaxonomyData
	taxonomyETag string
)

func init() {
	if err := common.Unmarshal(taxonomyBytes, &taxonomy); err != nil {
		panic(fmt.Sprintf("influencer taxonomy embedded data is invalid: %v", err))
	}
	taxonomyETag = fmt.Sprintf("%x", sha256.Sum256(taxonomyBytes))[:16]
}

// Taxonomy returns the embedded dataset for read-only use.
func Taxonomy() *TaxonomyData {
	return &taxonomy
}

// ETag is a short content hash clients can use for cache busting.
func ETag() string {
	return taxonomyETag
}

// CategoryByID finds a category by id.
func CategoryByID(id string) *Category {
	for i := range taxonomy.Categories {
		if taxonomy.Categories[i].ID == id {
			return &taxonomy.Categories[i]
		}
	}
	return nil
}

// OptionByID finds an option inside a category.
func OptionByID(catID, optID string) *Option {
	c := CategoryByID(catID)
	if c == nil {
		return nil
	}
	for i := range c.Options {
		if c.Options[i].ID == optID {
			return &c.Options[i]
		}
	}
	return nil
}

// Fragment resolves the prompt fragment for an option; falls back to the raw id.
func Fragment(catID, optID string) string {
	if arr, ok := taxonomy.PromptMap.Categories[catID]; ok {
		for _, o := range arr {
			if o.ID == optID {
				return o.Fragment
			}
		}
	}
	return optID
}

// IsAnimalTier reports whether the tier fixes an anthropomorphic animal species.
func IsAnimalTier(tier string) bool {
	return tier == "cats" || tier == "dogs"
}

// ValidTier reports whether the dataset supports the tier.
func ValidTier(tier string) bool {
	for _, o := range taxonomy.TierGroup.Options {
		if o.ID == tier {
			return true
		}
	}
	return false
}
