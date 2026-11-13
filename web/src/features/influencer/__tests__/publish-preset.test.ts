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

import { publishedPresets } from '../lib/presets'
import type { InfluencerTaxonomy, PublishedPresetRow } from '../types'

/** A minimal taxonomy with one resolvable pick and one that is not offered. */
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
    category_priority: ['hair'],
    categories: [
      {
        id: 'hair',
        label_en: 'Hair',
        kind: 'media',
        max: 1,
        options: [
          { id: 'hair_bowl', label_en: 'Bowl cut', visibleIn: ['freak'] },
        ],
      },
    ],
    prompt_map: { tiers: {}, categories: {} },
  }
}

function row(over: Partial<PublishedPresetRow> = {}): PublishedPresetRow {
  return {
    id: 'pub-1',
    name: 'Street silver',
    tier: 'freak',
    selection: { hair: ['hair_bowl'] },
    brief: 'silver hair on a night street',
    seed: 12345,
    preview: {
      path: '/api/influencer/presets/1/asset/preview',
      width: 1080,
      height: 1920,
    },
    sheet: {
      path: '/api/influencer/presets/1/asset/sheet',
      width: 1920,
      height: 1080,
    },
    created_at: 1759600000,
    published: true,
    ...over,
  }
}

describe('publishedPresets', () => {
  it('maps a published row into a grid preset that keeps the gateway assets', () => {
    const [preset] = publishedPresets([row()], taxonomy())

    expect(preset).toEqual({
      id: 'pub-1',
      name: 'Street silver',
      preview: {
        path: '/api/influencer/presets/1/asset/preview',
        width: 1080,
        height: 1920,
      },
      sheet: {
        path: '/api/influencer/presets/1/asset/sheet',
        width: 1920,
        height: 1080,
      },
      tier: 'freak',
      selection: { hair: ['hair_bowl'] },
      brief: 'silver hair on a night street',
      seed: 12345,
      published: true,
      createdAt: 1759600000,
    })
  })

  it('carries the derivative without replacing the full-size assets', () => {
    const published = row({
      thumbnail: { path: '/thumb.webp', width: 640, height: 1138 },
    })
    const [preset] = publishedPresets([published], taxonomy())
    expect(preset.thumbnail).toEqual(published.thumbnail)
    expect(preset.preview).toEqual(published.preview)
    expect(preset.sheet).toEqual(published.sheet)
  })

  it('drops a row whose option ids are not in the taxonomy', () => {
    const unresolvable = row({ selection: { hair: ['hair_mohawk'] } })

    expect(publishedPresets([unresolvable], taxonomy())).toEqual([])
  })

  it('returns nothing until the taxonomy has loaded', () => {
    expect(publishedPresets([row()], null)).toEqual([])
    expect(publishedPresets(undefined, taxonomy())).toEqual([])
  })

  it('sorts the newest publication first', () => {
    const rows = [
      row({ id: 'pub-1', created_at: 100 }),
      row({ id: 'pub-2', created_at: 300 }),
      row({ id: 'pub-3', created_at: 200 }),
    ]

    expect(publishedPresets(rows, taxonomy()).map((p) => p.id)).toEqual([
      'pub-2',
      'pub-3',
      'pub-1',
    ])
  })

  it('breaks a publication-time tie by the newest row id', () => {
    const rows = [
      row({ id: 'pub-9', created_at: 100 }),
      row({ id: 'pub-12', created_at: 100 }),
    ]

    expect(publishedPresets(rows, taxonomy()).map((p) => p.id)).toEqual([
      'pub-12',
      'pub-9',
    ])
  })
})
