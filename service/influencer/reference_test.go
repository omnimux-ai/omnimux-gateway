package influencer

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

const expectedDualViewSheetTail = `. Create ONE character reference image on a 16:9 landscape canvas with exactly two side-by-side views in a single row: LEFT HALF and RIGHT HALF, approximately equal width, separated only by a narrow white gap; no borders, labels, text, inset panels or additional views.
LEFT HALF: a large front-facing chest-up portrait, looking straight into the camera, with the face as the visual focus. Include the face, ears and recognisable hairstyle; keep both eyes and chin fully visible. Crop naturally through the upper chest at the bottom edge; shoulders may extend beyond this half. Keep the hairstyle inside the panel by adjusting the camera distance rather than distorting its shape.
RIGHT HALF: a front-facing full-body standing view of the same single character, looking straight into the camera, centred within this half. Show the entire silhouette from the highest point of the hairstyle to the soles of both shoes, including both hands. Fit the whole character within approximately 90-94% of the panel height, with clear white margins above the hair, below the shoes and on both sides. For unusually wide bodies or tall/wide hairstyles, scale the entire figure down enough to fit; never crop extremities, stretch the body or normalise the selected proportions.
CONSISTENCY: both views depict the same single character with identical face, age, skin tone, expression, hairstyle, selected proportions, clothing and accessories. Preserve every specified distinctive or exaggerated feature. Only the camera framing changes; do not create two different people.
SURFACE AND LIGHT: use the appearance of an unretouched camera photograph, preserving skin micro-texture and subtle local colour variation appropriate to the character's age and each view's distance. Show delicate pores on the nose and cheeks in the close-up without enlarging them across the full-body view. Keep skin reflections restrained, natural hair clumps with a few flyaways, and fabric drape and folds consistent with its material and pose. Use one broad soft directional light with gentle natural shading on the face and body, consistent in both views. Keep colours natural and details clearly readable without beauty-filter smoothing, waxy or plastic surfaces, glowing skin, excessive sharpening, artificial blur or heavy grain. Photographic material does not override the chosen character proportions or styling.
BACKGROUND: both views are isolated on a seamless pure white background (#FFFFFF), flat, even and shadow-free, with no gradient, no vignette, no coloured cast, no props and no environment behind the subject; only a subtle soft contact shadow directly beneath the feet in the right view. The shadow-free requirement applies to the backdrop, not to the character's own natural shading.
LAYOUT PRIORITY: these fixed canvas, two-view framing and background requirements take precedence over conflicting composition instructions in the character description. Interpret the character description as appearance and wardrobe guidance, not as permission to add panels or change the layout.`

func TestReferencePromptAllowsNewCharacterAndPreservesExaggeration(t *testing.T) {
	p := BuildReferencePrompt("total", Selection{"gender": {"male"}}, "wear a blue coat")
	assert.Contains(t, p, "Extreme caricature, exaggerated proportions")
	assert.Contains(t, p, "Male")
	assert.Contains(t, p, "wear a blue coat")
	assert.Contains(t, p, "Explicit brief takes priority over selected traits; selected traits take priority over image inspiration")
	assert.Contains(t, p, "a different person")
	assert.Contains(t, p, "silhouette, styling, colours, clothing and overall mood")
	assert.Contains(t, p, "Do not copy the reference background, text or logos")
	assert.Contains(t, p, "one single character")
	assert.Contains(t, p, "pure white background")
	assert.NotContains(t, p, "exact same character")
}

func TestReferenceOnlyPromptIsCompleteCharacterInstruction(t *testing.T) {
	const instruction = "Create one single character using the reference image only as inspiration for silhouette, styling, colours, clothing and overall mood. A new character and a different person are allowed; do not lock the reference identity. Explicit brief takes priority over selected traits; selected traits take priority over image inspiration. Preserve the requested exaggeration tier. Do not copy the reference background, text or logos. "
	for _, tc := range []struct {
		name, tier, brief, character string
		selection                    Selection
	}{
		{"reference only", "freak", "", "Stylized, slightly exaggerated, character reference portrait of an original character inspired by the reference image", nil},
		{"reference and brief", "normal", "  wear a blue coat  ", "Natural, realistic, character reference portrait of an original character inspired by the reference image. wear a blue coat", nil},
		{"reference and traits", "total", "wear a blue coat", "Extreme caricature, exaggerated proportions, character reference portrait of a Male. wear a blue coat", Selection{"gender": {"male"}}},
		{"empty category is not a subject", "normal", "", "Natural, realistic, character reference portrait of an original character inspired by the reference image", Selection{"gender": {}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, instruction+tc.character+expectedDualViewSheetTail, BuildReferencePrompt(tc.tier, tc.selection, tc.brief))
		})
	}
}

func TestNoReferencePromptHasExactDualViewInstruction(t *testing.T) {
	assert.Equal(t, "Natural, realistic, character reference portrait of a Female. freckles, smiling"+expectedDualViewSheetTail, BuildPrompt("normal", Selection{"gender": {"female"}}, "  freckles, smiling  "))
}
