package influencer

import "strings"

var animalPromptOrder = []string{
	"animal_breed", "coat_color", "coat_pattern", "gender", "height", "body_type",
	"proportions", "hair", "hair_colour", "distinctive", "aesthetic", "accessory",
}

// animalDescription uses only traits visible for the fixed species. Defaults
// apply to original sheets, never to turnarounds derived from an existing image.
func animalDescription(tier string, sel Selection, defaults bool) string {
	parts := []string{}
	for _, catID := range animalPromptOrder {
		picks := sel[catID]
		if defaults && len(picks) == 0 {
			switch catID {
			case "animal_breed":
				id := "cat_generic"
				if tier == "dogs" {
					id = "dog_generic"
				}
				picks = []string{id}
			case "coat_color":
				picks = []string{"coat_natural"}
			case "coat_pattern":
				picks = []string{"pattern_natural"}
			case "hair":
				parts = append(parts, "natural head fur")
			case "aesthetic":
				picks = []string{"casual"}
			}
		}
		for _, id := range picks {
			o := OptionByID(catID, id)
			if o == nil || !contains(o.VisibleIn, tier) {
				continue
			}
			fragment := Fragment(catID, id)
			for _, override := range taxonomy.PromptMap.AnimalCategories[catID] {
				if override.ID == id {
					fragment = override.Fragment
					break
				}
			}
			parts = append(parts, fragment)
		}
	}
	return strings.Join(parts, ", ")
}

func animalIdentity(tier string) string {
	species := "cat"
	if tier == "dogs" {
		species = "dog"
	}
	return "an anthropomorphic bipedal " + species + " character with a true animal head, species-appropriate muzzle, nose, ears, fur, tail and paws; standing upright on two legs with expressive upper limbs. This is an animal character, not a person wearing a mask or a four-legged pet."
}

const animalPromptTail = `. Create ONE character reference image on a 16:9 landscape canvas with exactly two side-by-side views in a single row: LEFT HALF and RIGHT HALF, approximately equal width, separated only by a narrow white gap; no borders, labels, text, inset panels or additional views.
LEFT HALF: a large front-facing chest-up portrait, looking straight into the camera, showing the animal head, muzzle, nose, ears, eyes, fur and any selected hairstyle. Keep the ears and hairstyle inside the panel by adjusting camera distance. Crop naturally through the upper chest.
RIGHT HALF: a front-facing full-body standing view of the same single character, upright on two legs, centred within this half. Show the entire silhouette including ears, hairstyle, upper limbs, paws and complete species-appropriate tail without cropping. Fit the whole character within approximately 90-94% of the panel height with clear white margins; scale wide or tall shapes down without changing selected proportions. Footwear is optional and must suit the animal anatomy.
CONSISTENCY: identical animal face, muzzle, ears, coat colour, coat markings, tail, paws, expression, hairstyle, selected proportions, clothing and accessories in both views; only framing changes. A selected head-fur hairstyle or hairstyle colour does not change the ears or coat colour.
SURFACE AND LIGHT: natural readable fur strands and fur clumps, species-appropriate nose and paw material, realistic fabric drape and folds, one broad soft directional light with gentle natural shading consistent in both views. Preserve the animal anatomy without waxy smoothing or plastic surfaces.
BACKGROUND: seamless pure white background (#FFFFFF), flat, even and shadow-free, no gradient, vignette, props or environment; only a subtle soft contact shadow directly beneath the paws in the right view.
PRIORITY: selected species and bipedal anatomy, fixed two-view framing and white background take precedence over conflicting brief or reference instructions. Interpret the brief only as compatible appearance and wardrobe guidance; never replace the species or change the layout.`

// BuildMultiViewPromptForTier preserves a parent character's species and traits.
// Callers validate the tier and selection before assembly. Human tiers use the
// legacy prompt unchanged; animal turnarounds retain unspecified reference traits.
func BuildMultiViewPromptForTier(tier string, sel Selection, brief string) string {
	if !IsAnimalTier(tier) {
		return BuildMultiViewPrompt(sel, brief)
	}
	prompt := "Create a professional reference sheet of the exact same character from the reference image: " + animalIdentity(tier) + " Keep the animal face, muzzle, nose, ears, fur, coat colour, coat markings, tail, paws, body shape, hairstyle and outfit identical in every panel; retain the reference breed and all unspecified reference traits. Plain seamless neutral grey background in every panel, consistent soft diffused studio lighting.\n\n"
	// The panel geometry is shared with the human sheet; only anatomy and surface differ.
	prompt += multiViewLayout[strings.Index(multiViewLayout, "LAYOUT:"):]
	if character := animalDescription(tier, sel, false); character != "" {
		prompt += "\n\nCHARACTER (must match reference image exactly in every panel):\n" + character
	}
	if brief = strings.TrimSpace(brief); brief != "" {
		prompt += "\n\nCharacter brief: " + brief
	}
	return prompt + "\n\nQUALITY: natural fur strands and fur clumps, species-appropriate nose and paw material, realistic fabric drape, accurate consistent colours. Every full-body panel includes the complete ears, upper limbs, paws and tail without cropping; keep the same bipedal stance and animal anatomy. Selected hairstyle colour is limited to styled head fur, not the ears or body coat. Species, reference identity and fixed panel layout take precedence over conflicting brief instructions; never redesign or randomly replace the character. No text or logos."
}

func buildAnimalPrompt(tier string, sel Selection, brief string) string {
	prompt := "Create " + animalIdentity(tier) + " Character appearance: " + animalDescription(tier, sel, true)
	if brief = strings.TrimSpace(brief); brief != "" {
		prompt += ". Character brief: " + brief
	}
	return prompt + animalPromptTail
}
