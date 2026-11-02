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

import { existsSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'

import { describe, expect, it } from 'vitest'

import {
  applyPresetSelection,
  PRESET_SNAPSHOT,
  presetParamRows,
  resolvePresetSelection,
  usablePresets,
} from '../lib/presets'
import { conflictMessage } from '../lib/taxonomy'
import type { ExplorePreset, InfluencerTaxonomy } from '../types'

/**
 * The taxonomy the gateway actually serves, so the snapshot is verified against
 * the same option ids the sidebar renders. Located by walking up from wherever
 * vitest was started, which is `web/` under the package scripts.
 */
function servedTaxonomy(): InfluencerTaxonomy {
  let dir = process.cwd()
  for (let depth = 0; depth < 4; depth += 1) {
    const candidate = join(dir, 'service/influencer/data/taxonomy.json')
    if (existsSync(candidate)) {
      return JSON.parse(readFileSync(candidate, 'utf8')) as InfluencerTaxonomy
    }
    dir = dirname(dir)
  }
  throw new Error('service/influencer/data/taxonomy.json not found')
}

/** A preset's params are only usable when the sidebar can show every pick. */
function preset(overrides: Partial<ExplorePreset> = {}): ExplorePreset {
  return {
    id: 'p1',
    name: 'Preset 1',
    preview: { path: '/influencer/presets/a.webp', width: 400, height: 600 },
    sheet: { path: '/influencer/presets/b.webp', width: 1200, height: 700 },
    tier: 'freak',
    selection: { hair: ['hair_bowl'] },
    ...overrides,
  }
}

function taxonomy(): InfluencerTaxonomy {
  return {
    version: 1,
    source: 'test',
    tier_group: {
      options: [
        { id: 'normal', label_en: 'Average', visibleIn: ['normal'] },
        { id: 'freak', label_en: 'Bold', visibleIn: ['freak'] },
      ],
      comingSoonImageUrls: [],
    },
    category_priority: ['gender'],
    categories: [
      {
        id: 'hair',
        label_en: 'Hair',
        kind: 'media',
        max: 1,
        options: [
          { id: 'hair_bowl', label_en: 'Bowl cut', visibleIn: ['freak'] },
          { id: 'hair_buzz', label_en: 'Buzz cut', visibleIn: ['normal'] },
        ],
      },
      {
        id: 'gender',
        label_en: 'Gender',
        kind: 'media',
        max: 1,
        options: [{ id: 'male', label_en: 'Male', visibleIn: ['normal', 'freak'] }],
      },
    ],
    prompt_map: { tiers: {}, categories: {} },
  }
}

describe('resolvePresetSelection', () => {
  it('keeps every pick when the category, option and tier all resolve', () => {
    const resolved = resolvePresetSelection(
      preset({ selection: { gender: ['male'], hair: ['hair_bowl'] } }),
      taxonomy()
    )
    expect(resolved).toEqual({ gender: ['male'], hair: ['hair_bowl'] })
  })

  it('drops an empty pick list instead of carrying it into the sidebar', () => {
    const resolved = resolvePresetSelection(
      preset({ selection: { hair: ['hair_bowl'], gender: [] } }),
      taxonomy()
    )
    expect(resolved).toEqual({ hair: ['hair_bowl'] })
  })

  it('refuses the whole preset when a category is unknown', () => {
    const resolved = resolvePresetSelection(
      preset({ selection: { hair: ['hair_bowl'], species: ['elf'] } }),
      taxonomy()
    )
    expect(resolved).toBeNull()
  })

  it('refuses the whole preset when an option is not visible in its own tier', () => {
    // `hair_buzz` only shows in `normal`, but this preset claims `freak`.
    const resolved = resolvePresetSelection(
      preset({ tier: 'freak', selection: { hair: ['hair_buzz'] } }),
      taxonomy()
    )
    expect(resolved).toBeNull()
  })

  it('refuses a preset whose tier the taxonomy does not offer', () => {
    const resolved = resolvePresetSelection(
      preset({ tier: 'legendary' }),
      taxonomy()
    )
    expect(resolved).toBeNull()
  })

  it('refuses a preset that exceeds a category pick limit', () => {
    const resolved = resolvePresetSelection(
      preset({ selection: { hair: ['hair_bowl', 'hair_buzz'] } }),
      taxonomy()
    )
    expect(resolved).toBeNull()
  })

  it('refuses a preset with nothing picked at all', () => {
    expect(resolvePresetSelection(preset({ selection: {} }), taxonomy())).toBeNull()
  })
})

describe('usablePresets', () => {
  it('returns nothing until the taxonomy has loaded', () => {
    expect(usablePresets(null)).toEqual([])
  })

  it('keeps only the presets that resolve against the served taxonomy', () => {
    const tax = servedTaxonomy()
    const all = usablePresets(tax)
    expect(all.length).toBe(PRESET_SNAPSHOT.total)
    for (const item of all) {
      expect(resolvePresetSelection(item, tax)).not.toBeNull()
    }
  })
})

describe('applyPresetSelection', () => {
  it('applies every offered preset to a selection no hard conflict refuses', () => {
    // The grid promises "Recreate" leads to a Generate that works. Mirrored
    // presets may carry a combination this product forbids, so the applied
    // selection — not the raw snapshot — is what has to satisfy the rules.
    const tax = servedTaxonomy()
    const offered = usablePresets(tax)
    expect(offered.length).toBe(PRESET_SNAPSHOT.total)

    for (const item of offered) {
      const applied = applyPresetSelection(item, tax)
      if (!applied) throw new Error(`${item.name} should resolve against the served taxonomy`)
      expect(conflictMessage(item.tier, applied.selection), item.name).toBeNull()
    }
  })

  it('drops the one pick the conflict names and keeps the rest of the look', () => {
    const tax = servedTaxonomy()
    const preset = usablePresets(tax).find((p) => p.name === 'AI Influencer 13')
    if (!preset) throw new Error('AI Influencer 13 should be offered by the grid')

    const applied = applyPresetSelection(preset, tax)
    if (!applied) throw new Error('AI Influencer 13 should resolve against the served taxonomy')

    expect(applied.selection.proportions).toEqual(['pr_shoulders'])
    expect(applied.removed).toEqual([
      { categoryId: 'proportions', optionId: 'pr_shortlegs', label: 'Short legs' },
    ])
    expect(conflictMessage(preset.tier, applied.selection)).toBeNull()
  })

  it('reports nothing when the preset already satisfies the hard conflicts', () => {
    const tax = servedTaxonomy()
    const preset = usablePresets(tax).find((p) => p.name === 'AI Influencer 38')
    if (!preset) throw new Error('AI Influencer 38 should be offered by the grid')

    const applied = applyPresetSelection(preset, tax)

    expect(applied?.removed).toEqual([])
    expect(applied?.selection).toEqual(resolvePresetSelection(preset, tax))
  })

  it('returns null for a preset that cannot be resolved at all', () => {
    expect(applyPresetSelection(preset({ selection: { species: ['elf'] } }), taxonomy())).toBeNull()
  })
})

describe('presetParamRows', () => {
  it('orders rows by the taxonomy priority and shows labels, not raw ids', () => {
    const rows = presetParamRows(
      preset({ selection: { gender: ['male'], hair: ['hair_bowl'] } }),
      taxonomy()
    )
    expect(rows.map((r) => r.categoryId)).toEqual(['gender', 'hair'])
    expect(rows[0]).toEqual({ categoryId: 'gender', label: 'Gender', options: ['Male'] })
    expect(rows[1]).toEqual({ categoryId: 'hair', label: 'Hair', options: ['Bowl cut'] })
  })

  it('returns no rows for a preset that cannot be applied', () => {
    expect(presetParamRows(preset({ selection: {} }), taxonomy())).toEqual([])
  })
})

describe('preset snapshot', () => {
  const tax = servedTaxonomy()
  const items = usablePresets(tax)

  it('carries the mirrored catalogue with unique ids', () => {
    expect(PRESET_SNAPSHOT.total).toBeGreaterThan(0)
    expect(new Set(items.map((i) => i.id)).size).toBe(items.length)
  })

  it('points every asset at our own origin, with real dimensions', () => {
    // Two homes, both ours: the mirrored catalogue ships its webp files with the
    // build, and the hand-authored Popino pack is served from our R2 bucket.
    const mirrored = /^\/influencer-presets\/[0-9a-f]{16}\.webp$/
    const r2 = /^https:\/\/files\.omnimux\.ai\/influencer\/presets\/[0-9a-f]{16}\.webp$/
    for (const item of items) {
      for (const asset of [item.preview, item.sheet]) {
        expect(asset.path).toMatch(new RegExp(`${mirrored.source}|${r2.source}`))
        expect(asset.width).toBeGreaterThan(0)
        expect(asset.height).toBeGreaterThan(0)
      }
    }
  })

  it('keeps the Popino template pack complete, R2-hosted, and ahead of the mirror', () => {
    const pack = items.filter((i) => i.id.startsWith('popino-'))
    expect(pack.map((i) => i.id)).toEqual([
      'popino-jean-phil',
      'popino-jerry',
      'popino-brigitte-macaron',
      'popino-leopold',
      'popino-jimmy',
    ])
    // The grid renders in array order, so leading the snapshot is what puts the
    // newest pack in the first column of the inspiration library.
    expect(items.slice(0, pack.length).map((i) => i.id)).toEqual(pack.map((i) => i.id))
    for (const item of pack) {
      for (const asset of [item.preview, item.sheet]) {
        expect(asset.path).toMatch(
          /^https:\/\/files\.omnimux\.ai\/influencer\/presets\/[0-9a-f]{16}\.webp$/
        )
      }
      // The upstream generation format travels in the brief: description first,
      // then the character lock the source product uses to keep a face stable.
      expect(item.brief).toContain('Character lock — face:')
      expect(item.brief).toContain('never change:')
    }
  })

  it('ships a tier and at least one pick for every preset', () => {
    for (const item of items) {
      expect(tax.tier_group.options.map((o) => o.id)).toContain(item.tier)
      expect(Object.keys(item.selection).length).toBeGreaterThan(0)
    }
  })
})
