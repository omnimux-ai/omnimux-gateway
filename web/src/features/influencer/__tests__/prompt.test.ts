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

import { describe, expect, it } from 'vitest'

import { buildPrompt, PROMPT_TAIL } from '../lib/prompt'
import type { InfluencerTaxonomy } from '../types'

const tax: InfluencerTaxonomy = {
  version: 1,
  source: 'test',
  tier_group: { options: [], comingSoonImageUrls: [] },
  category_priority: [],
  categories: [],
  prompt_map: {
    tiers: {},
    categories: {
      gender: [
        { id: 'male', fragment: 'Male' },
        { id: 'female', fragment: 'Female' },
      ],
      age: [{ id: 'adult', fragment: 'Adult' }],
      skin_tone: [{ id: 'st_fair', fragment: 'fair skin' }],
      height: [{ id: 'h_average', fragment: 'average height' }],
      aesthetic: [{ id: 'goth', fragment: 'goth outfit' }],
    },
  },
}

describe('buildPrompt', () => {
  it('normal tier starts with the natural lead', () => {
    const p = buildPrompt(tax, 'normal', {}, '')
    expect(p.startsWith('Natural, realistic')).toBe(true)
  })

  it('total tier starts with the extreme lead', () => {
    const p = buildPrompt(tax, 'total', {}, '')
    expect(p.startsWith('Extreme caricature')).toBe(true)
  })

  it('fragments follow PROMPT_ORDER not insertion order', () => {
    const sel = {
      skin_tone: ['st_fair'],
      gender: ['male'],
      height: ['h_average'],
      age: ['adult'],
    }
    const p = buildPrompt(tax, 'normal', sel, '')
    expect(p).toContain('Male, Adult, fair skin, average height')
  })

  it('appends brief and the white-backdrop suffix', () => {
    const p = buildPrompt(
      tax,
      'normal',
      { gender: ['male'] },
      'a corporate product reviewer'
    )
    expect(p).toContain('a corporate product reviewer.')
    expect(p.endsWith(PROMPT_TAIL)).toBe(true)
    expect(p).toContain('pure white background')
    expect(p).not.toContain('Studio background')
  })

  it('unknown option ids fall back to the raw id', () => {
    const p = buildPrompt(tax, 'normal', { gender: ['unknown_id'] }, '')
    expect(p).toContain('unknown_id')
  })

  it.each(['normal', 'freak', 'total'])(
    'keeps %s character details in a landscape two-view prompt',
    (tier) => {
      const p = buildPrompt(tax, tier, { gender: ['male'] }, 'red bob haircut')
      for (const fragment of [
        '16:9 landscape canvas',
        'exactly two side-by-side views',
        'LEFT HALF',
        'front-facing chest-up portrait',
        'RIGHT HALF',
        'front-facing full-body standing view',
        'same single character',
        'selected proportions',
        'red bob haircut',
        'skin micro-texture',
      ]) {
        expect(p).toContain(fragment)
      }
      expect(p).not.toContain('9:16')
      expect(p).not.toContain('high detail, sharp focus')
    }
  )

  it('null taxonomy still produces a prompt', () => {
    const p = buildPrompt(null, 'normal', { gender: ['male'] }, '')
    expect(p.startsWith('Natural, realistic')).toBe(true)
  })
})
