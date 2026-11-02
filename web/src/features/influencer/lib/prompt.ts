/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import type { InfluencerTaxonomy, Selection } from '../types'
import { isAnimalTier } from './taxonomy'

export const PROMPT_ORDER = [
  'gender',
  'ethnicity_origin_base',
  'age',
  'skin_tone',
  'height',
  'body_type',
  'proportions',
  'freak_head',
  'freak_neck',
  'eye_shape',
  'eye_color',
  'freak_face',
  'facial_hair',
  'hair',
  'hair_colour',
  'distinctive',
  'aesthetic',
  'accessory',
] as const

function fragment(
  tax: InfluencerTaxonomy,
  catId: string,
  optId: string
): string {
  const arr = tax.prompt_map.categories[catId]
  const f = arr?.find((o) => o.id === optId)
  return f?.fragment ?? optId
}

function lead(tier: string): string {
  if (tier === 'total') {
    return 'Extreme caricature, exaggerated proportions, character reference portrait of a'
  }
  if (tier === 'freak') {
    return 'Stylized, slightly exaggerated, character reference portrait of a'
  }
  return 'Natural, realistic, character reference portrait of a'
}

/** Mirrors promptTail in service/influencer/rules.go — keep both in step. */
export const PROMPT_TAIL = `. Create ONE character reference image on a 16:9 landscape canvas with exactly two side-by-side views in a single row: LEFT HALF and RIGHT HALF, approximately equal width, separated only by a narrow white gap; no borders, labels, text, inset panels or additional views.
LEFT HALF: a large front-facing chest-up portrait, looking straight into the camera, with the face as the visual focus. Include the face, ears and recognisable hairstyle; keep both eyes and chin fully visible. Crop naturally through the upper chest at the bottom edge; shoulders may extend beyond this half. Keep the hairstyle inside the panel by adjusting the camera distance rather than distorting its shape.
RIGHT HALF: a front-facing full-body standing view of the same single character, looking straight into the camera, centred within this half. Show the entire silhouette from the highest point of the hairstyle to the soles of both shoes, including both hands. Fit the whole character within approximately 90-94% of the panel height, with clear white margins above the hair, below the shoes and on both sides. For unusually wide bodies or tall/wide hairstyles, scale the entire figure down enough to fit; never crop extremities, stretch the body or normalise the selected proportions.
CONSISTENCY: both views depict the same single character with identical face, age, skin tone, expression, hairstyle, selected proportions, clothing and accessories. Preserve every specified distinctive or exaggerated feature. Only the camera framing changes; do not create two different people.
SURFACE AND LIGHT: use the appearance of an unretouched camera photograph, preserving skin micro-texture and subtle local colour variation appropriate to the character's age and each view's distance. Show delicate pores on the nose and cheeks in the close-up without enlarging them across the full-body view. Keep skin reflections restrained, natural hair clumps with a few flyaways, and fabric drape and folds consistent with its material and pose. Use one broad soft directional light with gentle natural shading on the face and body, consistent in both views. Keep colours natural and details clearly readable without beauty-filter smoothing, waxy or plastic surfaces, glowing skin, excessive sharpening, artificial blur or heavy grain. Photographic material does not override the chosen character proportions or styling.
BACKGROUND: both views are isolated on a seamless pure white background (#FFFFFF), flat, even and shadow-free, with no gradient, no vignette, no coloured cast, no props and no environment behind the subject; only a subtle soft contact shadow directly beneath the feet in the right view. The shadow-free requirement applies to the backdrop, not to the character's own natural shading.
LAYOUT PRIORITY: these fixed canvas, two-view framing and background requirements take precedence over conflicting composition instructions in the character description. Interpret the character description as appearance and wardrobe guidance, not as permission to add panels or change the layout.`

const ANIMAL_PROMPT_ORDER = [
  'animal_breed',
  'coat_color',
  'coat_pattern',
  'gender',
  'height',
  'body_type',
  'proportions',
  'hair',
  'hair_colour',
  'distinctive',
  'aesthetic',
  'accessory',
]

/** Mirrors animalPromptTail in service/influencer/animal.go. */
export const ANIMAL_PROMPT_TAIL = `. Create ONE character reference image on a 16:9 landscape canvas with exactly two side-by-side views in a single row: LEFT HALF and RIGHT HALF, approximately equal width, separated only by a narrow white gap; no borders, labels, text, inset panels or additional views.
LEFT HALF: a large front-facing chest-up portrait, looking straight into the camera, showing the animal head, muzzle, nose, ears, eyes, fur and any selected hairstyle. Keep the ears and hairstyle inside the panel by adjusting camera distance. Crop naturally through the upper chest.
RIGHT HALF: a front-facing full-body standing view of the same single character, upright on two legs, centred within this half. Show the entire silhouette including ears, hairstyle, upper limbs, paws and complete species-appropriate tail without cropping. Fit the whole character within approximately 90-94% of the panel height with clear white margins; scale wide or tall shapes down without changing selected proportions. Footwear is optional and must suit the animal anatomy.
CONSISTENCY: identical animal face, muzzle, ears, coat colour, coat markings, tail, paws, expression, hairstyle, selected proportions, clothing and accessories in both views; only framing changes. A selected head-fur hairstyle or hairstyle colour does not change the ears or coat colour.
SURFACE AND LIGHT: natural readable fur strands and fur clumps, species-appropriate nose and paw material, realistic fabric drape and folds, one broad soft directional light with gentle natural shading consistent in both views. Preserve the animal anatomy without waxy smoothing or plastic surfaces.
BACKGROUND: seamless pure white background (#FFFFFF), flat, even and shadow-free, no gradient, vignette, props or environment; only a subtle soft contact shadow directly beneath the paws in the right view.
PRIORITY: selected species and bipedal anatomy, fixed two-view framing and white background take precedence over conflicting brief or reference instructions. Interpret the brief only as compatible appearance and wardrobe guidance; never replace the species or change the layout.`

function buildAnimalPrompt(
  tax: InfluencerTaxonomy | null,
  tier: string,
  sel: Selection,
  brief: string
): string {
  const species = tier === 'dogs' ? 'dog' : 'cat'
  const parts: string[] = []
  const defaults: Selection = {
    animal_breed: [`${species}_generic`],
    coat_color: ['coat_natural'],
    coat_pattern: ['pattern_natural'],
    aesthetic: ['casual'],
  }
  for (const catId of ANIMAL_PROMPT_ORDER) {
    const picks = sel[catId]?.length ? sel[catId] : (defaults[catId] ?? [])
    if (catId === 'hair' && !picks.length) parts.push('natural head fur')
    for (const id of picks) {
      const option = tax?.categories
        .find((c) => c.id === catId)
        ?.options.find((o) => o.id === id && o.visibleIn.includes(tier))
      if (!tax || !option) continue
      const override = tax.prompt_map.animal_categories?.[catId]?.find(
        (o) => o.id === id
      )
      parts.push(override?.fragment ?? fragment(tax, catId, id))
    }
  }
  let prompt = `Create an anthropomorphic bipedal ${species} character with a true animal head, species-appropriate muzzle, nose, ears, fur, tail and paws; standing upright on two legs with expressive upper limbs. This is an animal character, not a person wearing a mask or a four-legged pet. Character appearance: ${parts.join(', ')}`
  if (brief.trim()) prompt += `. Character brief: ${brief.trim()}`
  return prompt + ANIMAL_PROMPT_TAIL
}

/** Mirrors the backend BuildPrompt — available for prompt consumers. */
export function buildPrompt(
  tax: InfluencerTaxonomy | null,
  tier: string,
  sel: Selection,
  brief: string
): string {
  if (isAnimalTier(tier)) return buildAnimalPrompt(tax, tier, sel, brief)
  const parts: string[] = []
  for (const catId of PROMPT_ORDER) {
    for (const id of sel[catId] ?? []) {
      if (tax) parts.push(fragment(tax, catId, id))
    }
  }
  let p = lead(tier)
  if (parts.length) p += ` ${parts.join(', ')}`
  const b = brief.trim()
  if (b) p += `. ${b}`
  p += PROMPT_TAIL
  return p
}
