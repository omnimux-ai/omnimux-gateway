package influencer

import (
	"fmt"
	"sort"
	"strings"
)

// Selection is the client-submitted per-category pick map.
type Selection map[string][]string

// ValidationError is returned for rule violations; the HTTP layer renders it
// as a 400 with a human-readable English message (same copy family as the
// upstream reference product).
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// promptOrder is the fixed category order used for prompt assembly.
var promptOrder = []string{
	"gender", "ethnicity_origin_base", "age", "skin_tone", "height",
	"body_type", "proportions", "freak_head", "freak_neck", "eye_shape",
	"eye_color", "freak_face", "facial_hair", "hair", "hair_colour",
	"distinctive", "aesthetic", "accessory",
}

func has(sel Selection, cat, opt string) bool {
	for _, v := range sel[cat] {
		if v == opt {
			return true
		}
	}
	return false
}

func hasAny(sel Selection, cat string, opts ...string) bool {
	for _, v := range sel[cat] {
		for _, o := range opts {
			if v == o {
				return true
			}
		}
	}
	return false
}

func contains(arr []string, v string) bool {
	for _, a := range arr {
		if a == v {
			return true
		}
	}
	return false
}

// conflictMessage implements the 4 hard constraints: 2 from the upstream
// conflict table plus 2 neck/teeth exclusions from the same rules dataset.
func conflictMessage(tier string, sel Selection) string {
	switch {
	case has(sel, "body_type", "body_heavy") && hasAny(sel, "height", "h_tall", "h_very_tall"):
		return "Heavy build cannot be combined with Tall or Very tall."
	case tier != "normal" &&
		hasAny(sel, "body_type", "body_heavy", "body_muscular", "body_ultra") &&
		hasAny(sel, "proportions", "pr_shortlegs", "pr_longlimbs"):
		return "This build cannot be combined with Short legs or Long limbs in this mode."
	case has(sel, "body_type", "body_ultra") && has(sel, "freak_neck", "neck_long"):
		return "Extreme muscular build cannot be combined with Long neck."
	case has(sel, "freak_neck", "neck_long") && has(sel, "freak_face", "ff_teeth_11"):
		return "Long neck cannot be combined with Buck teeth."
	}
	return ""
}

// NormalizeSelection applies the same normalization the UI enforces:
// - unknown category / option is rejected
// - options not visible in the tier are dropped
// - acc_none is exclusive and clears other accessory picks
// - same-slot options in a multi-select category replace each other (last wins)
// - category max keeps the LAST picked options (matching UI append semantics)
func NormalizeSelection(tier string, sel Selection) (Selection, *ValidationError) {
	if !ValidTier(tier) {
		return nil, &ValidationError{Field: "tier", Message: "invalid tier"}
	}
	out := Selection{}
	for catID, picks := range sel {
		c := CategoryByID(catID)
		if c == nil {
			return nil, &ValidationError{Field: catID, Message: fmt.Sprintf("unknown category %q", catID)}
		}
		maxN := c.Max
		if maxN <= 0 {
			maxN = 1
		}
		kept := make([]string, 0, len(picks))
		for _, id := range picks {
			o := OptionByID(catID, id)
			if o == nil {
				return nil, &ValidationError{Field: catID, Message: fmt.Sprintf("unknown option %q", id)}
			}
			if IsAnimalTier(tier) && catID == "animal_breed" && !contains(o.VisibleIn, tier) {
				return nil, &ValidationError{Field: catID, Message: fmt.Sprintf("breed %q does not match selected species", id)}
			}
			if !contains(o.VisibleIn, tier) {
				continue
			}
			if catID == "accessory" && id == "acc_none" {
				kept = []string{id}
				continue
			}
			if catID == "accessory" && contains(kept, "acc_none") {
				kept = kept[:0]
			}
			if o.Slot != nil && *o.Slot != "" {
				filtered := kept[:0]
				for _, prev := range kept {
					po := OptionByID(catID, prev)
					if po != nil && po.Slot != nil && *po.Slot == *o.Slot {
						continue
					}
					filtered = append(filtered, prev)
				}
				kept = filtered
			}
			if len(kept) >= maxN {
				continue
			}
			if !contains(kept, id) {
				kept = append(kept, id)
			}
		}
		if len(kept) > 0 {
			out[catID] = kept
		}
	}
	return out, nil
}

// ValidateSelection normalizes then enforces the hard conflicts.
func ValidateSelection(tier string, sel Selection) (Selection, error) {
	norm, verr := NormalizeSelection(tier, sel)
	if verr != nil {
		return nil, verr
	}
	if IsAnimalTier(tier) {
		return norm, nil
	}
	if msg := conflictMessage(tier, norm); msg != "" {
		return nil, &ValidationError{Field: "selection", Message: msg}
	}
	return norm, nil
}

// tierLead returns the prompt lead for the exaggeration tier.
func tierLead(tier string) string {
	switch tier {
	case "total":
		return "Extreme caricature, exaggerated proportions, character reference portrait of a"
	case "freak":
		return "Stylized, slightly exaggerated, character reference portrait of a"
	default:
		return "Natural, realistic, character reference portrait of a"
	}
}

// promptTail is the fixed closing block every character sheet shares, mirrored
// by PROMPT_TAIL in the workbench preview. The background clause is
// load-bearing: "studio background" on its own renders a grey seamless
// backdrop, which reads as a different product beside the white-backdrop
// reference library. The constraint is phrased positively because the image
// endpoints this prompt reaches ignore negative prompts.
const promptTail = `. Create ONE character reference image on a 16:9 landscape canvas with exactly two side-by-side views in a single row: LEFT HALF and RIGHT HALF, approximately equal width, separated only by a narrow white gap; no borders, labels, text, inset panels or additional views.
LEFT HALF: a large front-facing chest-up portrait, looking straight into the camera, with the face as the visual focus. Include the face, ears and recognisable hairstyle; keep both eyes and chin fully visible. Crop naturally through the upper chest at the bottom edge; shoulders may extend beyond this half. Keep the hairstyle inside the panel by adjusting the camera distance rather than distorting its shape.
RIGHT HALF: a front-facing full-body standing view of the same single character, looking straight into the camera, centred within this half. Show the entire silhouette from the highest point of the hairstyle to the soles of both shoes, including both hands. Fit the whole character within approximately 90-94% of the panel height, with clear white margins above the hair, below the shoes and on both sides. For unusually wide bodies or tall/wide hairstyles, scale the entire figure down enough to fit; never crop extremities, stretch the body or normalise the selected proportions.
CONSISTENCY: both views depict the same single character with identical face, age, skin tone, expression, hairstyle, selected proportions, clothing and accessories. Preserve every specified distinctive or exaggerated feature. Only the camera framing changes; do not create two different people.
SURFACE AND LIGHT: use the appearance of an unretouched camera photograph, preserving skin micro-texture and subtle local colour variation appropriate to the character's age and each view's distance. Show delicate pores on the nose and cheeks in the close-up without enlarging them across the full-body view. Keep skin reflections restrained, natural hair clumps with a few flyaways, and fabric drape and folds consistent with its material and pose. Use one broad soft directional light with gentle natural shading on the face and body, consistent in both views. Keep colours natural and details clearly readable without beauty-filter smoothing, waxy or plastic surfaces, glowing skin, excessive sharpening, artificial blur or heavy grain. Photographic material does not override the chosen character proportions or styling.
BACKGROUND: both views are isolated on a seamless pure white background (#FFFFFF), flat, even and shadow-free, with no gradient, no vignette, no coloured cast, no props and no environment behind the subject; only a subtle soft contact shadow directly beneath the feet in the right view. The shadow-free requirement applies to the backdrop, not to the character's own natural shading.
LAYOUT PRIORITY: these fixed canvas, two-view framing and background requirements take precedence over conflicting composition instructions in the character description. Interpret the character description as appearance and wardrobe guidance, not as permission to add panels or change the layout.`

// BuildPrompt assembles the generation prompt.
func BuildPrompt(tier string, sel Selection, brief string) string {
	if IsAnimalTier(tier) {
		return buildAnimalPrompt(tier, sel, brief)
	}
	parts := make([]string, 0, 8)
	for _, catID := range promptOrder {
		ids := sel[catID]
		for _, id := range ids {
			parts = append(parts, Fragment(catID, id))
		}
	}
	var b strings.Builder
	b.WriteString(tierLead(tier))
	if len(parts) > 0 {
		b.WriteString(" ")
		b.WriteString(strings.Join(parts, ", "))
	}
	brief = strings.TrimSpace(brief)
	if brief != "" {
		b.WriteString(". ")
		b.WriteString(brief)
	}
	b.WriteString(promptTail)
	return b.String()
}

// BuildReferencePrompt assembles a sheet with optional visual inspiration.
func BuildReferencePrompt(tier string, sel Selection, brief string) string {
	if IsAnimalTier(tier) {
		return "Use the reference image only as style inspiration for styling, colours, clothing and overall mood; do not lock the reference identity. The selected species takes priority over the brief and image inspiration. Do not copy the reference background, text or logos. " + BuildPrompt(tier, sel, brief)
	}
	const referenceInstruction = "Create one single character using the reference image only as inspiration for silhouette, styling, colours, clothing and overall mood. A new character and a different person are allowed; do not lock the reference identity. Explicit brief takes priority over selected traits; selected traits take priority over image inspiration. Preserve the requested exaggeration tier. Do not copy the reference background, text or logos. "
	for _, catID := range promptOrder {
		if len(sel[catID]) > 0 {
			return referenceInstruction + BuildPrompt(tier, sel, brief)
		}
	}
	prompt := strings.TrimSuffix(tierLead(tier), " a") + " an original character inspired by the reference image"
	if brief = strings.TrimSpace(brief); brief != "" {
		prompt += ". " + brief
	}
	return referenceInstruction + prompt + promptTail
}

// multiViewLayout is the fixed turnaround-sheet layout. It is character
// independent on purpose: the character description is appended separately so
// the panel geometry stays byte-identical across every multi-view request.
//
// The exaggeration tier is deliberately absent. A multi-view board is derived
// from an existing render, so the panel prompt asks the model to match the
// reference exactly; the tier lead ("Extreme caricature, ...") would fight that
// instruction and re-exaggerate an already-rendered character.
const multiViewLayout = `Create a professional character reference sheet of the exact same character from the reference image, keeping their face, skin, body shape, hair, eyes, facial features, and outfit identical in every panel. Ultra-photorealistic, hyper-detailed studio photography, plain seamless neutral grey background in every panel, consistent soft diffused studio lighting throughout.

LAYOUT: two rows on one wide 16:9 image with clean thin white dividing lines between panels, no text, no labels.
Top row – five full-body panels, head to toe, same scale, standing pose:
1. Front view
2. Three-quarter front view (turned slightly to side)
3. Side profile view
4. Three-quarter back view
5. Back view

Bottom row – three large close-up portraits from the chest up:
6. Front headshot, eyes gently closed, head tilted slightly down, with a serene closed-lip smile
7. Three-quarter headshot, eyes half-open, looking thoughtful
8. Front headshot, eyes open looking straight into the lens, completely deadpan and confident`

// multiViewQuality closes the prompt with the rendering bar shared by every
// panel.
const multiViewQuality = `QUALITY: shot on a full-frame camera with an 85mm lens, sharp focus in every panel, natural skin texture with visible pores, fine individual hair strands, realistic fabric drape and texture on the ensemble. Accurate, consistent colours across all panels. Clean white dividing lines between panels, no text, no logos.`

// BuildMultiViewPrompt assembles the turnaround-sheet prompt from the fixed
// layout plus the character description. The description is drawn from the same
// promptOrder/Fragment source as BuildPrompt, so one character reads identically
// in the sheet and in its multi-view board.
func BuildMultiViewPrompt(sel Selection, brief string) string {
	parts := make([]string, 0, 8)
	for _, catID := range promptOrder {
		for _, id := range sel[catID] {
			parts = append(parts, Fragment(catID, id))
		}
	}
	character := strings.Join(parts, ", ")
	if brief = strings.TrimSpace(brief); brief != "" {
		if character != "" {
			character += ". " + brief
		} else {
			character = brief
		}
	}
	if character == "" {
		character = "the exact character"
	}

	var b strings.Builder
	b.WriteString(multiViewLayout)
	b.WriteString("\n\nCHARACTER (must match reference image exactly in every panel):\n")
	b.WriteString(character)
	b.WriteString("\n\n")
	b.WriteString(multiViewQuality)
	return b.String()
}

// PickedLabels lists human labels for a selection.
func PickedLabels(sel Selection) []string {
	out := []string{}
	keys := make([]string, 0, len(sel))
	for k := range sel {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		for _, id := range sel[k] {
			if o := OptionByID(k, id); o != nil {
				out = append(out, o.LabelEN)
			}
		}
	}
	return out
}
