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
import {
  catById,
  CONFLICT_RULES,
  filterSelectionForTier,
  optById,
  reconcileSelection,
  selectedCount,
  selectedLabels,
  totalSelected,
  trySelect,
  visOpts,
} from '../lib/taxonomy'
import type { InfluencerCategory } from '../types'

function cat(partial: Partial<InfluencerCategory> & { id: string }): InfluencerCategory {
  return { kind: 'media', label_en: partial.id, max: 1, options: [], ...partial }
}

const cats: InfluencerCategory[] = [
  cat({
    id: 'gender',
    max: 1,
    options: [
      { id: 'male', label_en: 'Male', visibleIn: ['normal', 'freak', 'total'] },
      { id: 'female', label_en: 'Female', visibleIn: ['normal', 'freak', 'total'] },
    ],
  }),
  cat({
    id: 'proportions',
    max: 2,
    options: [
      { id: 'p1', label_en: 'P1', visibleIn: ['normal', 'freak', 'total'] },
      { id: 'p2', label_en: 'P2', visibleIn: ['normal', 'freak', 'total'] },
      { id: 'p3', label_en: 'P3', visibleIn: ['freak', 'total'] },
    ],
  }),
  cat({
    id: 'accessory',
    max: 3,
    options: [
      { id: 'acc_none', label_en: 'None', visibleIn: ['normal', 'freak', 'total'] },
      { id: 'hat', label_en: 'Hat', visibleIn: ['normal', 'freak', 'total'], slot: 'head' },
      { id: 'cap', label_en: 'Cap', visibleIn: ['normal', 'freak', 'total'], slot: 'head' },
      { id: 'glasses', label_en: 'Glasses', visibleIn: ['normal', 'freak', 'total'], slot: 'eyes' },
    ],
  }),
]

describe('visOpts', () => {
  it('returns only options visible in the tier', () => {
    const c = cats[1]
    expect(visOpts(c, 'normal').map((o) => o.id)).toEqual(['p1', 'p2'])
    expect(visOpts(c, 'total').map((o) => o.id)).toEqual(['p1', 'p2', 'p3'])
  })
})

describe('trySelect', () => {
  it('single-max category replaces the previous pick', () => {
    const { selection } = trySelect(cats, 'normal', { gender: ['male'] }, 'gender', 'female')
    expect(selection.gender).toEqual(['female'])
  })

  it('respects max and keeps first picks', () => {
    let sel = { proportions: ['p1', 'p2'] }
    const { selection } = trySelect(cats, 'normal', sel, 'proportions', 'p3')
    expect(selection.proportions).toEqual(['p1', 'p2'])
  })

  it('acc_none is exclusive and clears other accessories', () => {
    const { selection } = trySelect(
      cats,
      'normal',
      { accessory: ['hat', 'glasses'] },
      'accessory',
      'acc_none'
    )
    expect(selection.accessory).toEqual(['acc_none'])
  })

  it('picking another accessory clears acc_none', () => {
    const { selection } = trySelect(
      cats,
      'normal',
      { accessory: ['acc_none'] },
      'accessory',
      'hat'
    )
    expect(selection.accessory).toEqual(['hat'])
  })

  it('same-slot last-wins', () => {
    const { selection } = trySelect(
      cats,
      'normal',
      { accessory: ['hat'] },
      'accessory',
      'cap'
    )
    expect(selection.accessory).toEqual(['cap'])
  })

  it('toggle off removes the pick and deletes empty category', () => {
    const { selection } = trySelect(cats, 'normal', { gender: ['male'] }, 'gender', 'male')
    expect(selection.gender).toBeUndefined()
  })
})

describe('filterSelectionForTier', () => {
  it('drops options invisible in the new tier and reports labels', () => {
    const { selection, dropped } = filterSelectionForTier(
      cats,
      'normal',
      { proportions: ['p1', 'p3'], gender: ['male'] }
    )
    expect(selection.proportions).toEqual(['p1'])
    expect(selection.gender).toEqual(['male'])
    expect(dropped).toEqual(['P3'])
  })
})

describe('counts', () => {
  it('selectedCount and totalSelected', () => {
    const sel = { gender: ['male'], accessory: ['hat', 'glasses'] }
    expect(selectedCount(sel, 'accessory')).toBe(2)
    expect(totalSelected(sel)).toBe(3)
  })

  it('selectedLabels returns labels in category order', () => {
    const labels = selectedLabels(cats, { accessory: ['hat'], gender: ['male'] })
    expect(labels).toContain('Male')
    expect(labels).toContain('Hat')
    expect(catById(cats, 'gender')?.id).toBe('gender')
    expect(optById(cats, 'gender', 'male')?.label_en).toBe('Male')
  })
})

describe('conflict rules i18n coverage', () => {
  it('covers all hard conflict messages in static keys and localization', async () => {
    // The locale JSON types expose literal keys; the rule messages are looked
    // up dynamically, so index them as a plain string map.
    type Catalog = { translation: Record<string, string> }
    const zh = (await import('@/i18n/locales/zh.json')).default as unknown as Catalog
    const zhTW = (await import('@/i18n/locales/zh-TW.json')).default as unknown as Catalog
    const en = (await import('@/i18n/locales/en.json')).default as unknown as Catalog

    for (const rule of CONFLICT_RULES) {
      expect(en.translation[rule.message]).toBe(rule.message)
      expect(zh.translation[rule.message]).toBeDefined()
      expect(zh.translation[rule.message].length).toBeGreaterThan(0)
      expect(zhTW.translation[rule.message]).toBeDefined()
      expect(zhTW.translation[rule.message].length).toBeGreaterThan(0)
    }
  })
})

describe('reconcileSelection', () => {
  /** Only the categories the four hard rules read, using their real option ids. */
  const ruleCats: InfluencerCategory[] = [
    cat({
      id: 'body_type',
      options: [
        { id: 'body_muscular', label_en: 'Muscular', visibleIn: ['normal', 'freak', 'total'] },
        { id: 'body_heavy', label_en: 'Heavy', visibleIn: ['normal', 'freak', 'total'] },
        { id: 'body_ultra', label_en: 'Ultra Muscular', visibleIn: ['normal', 'freak', 'total'] },
      ],
    }),
    cat({
      id: 'proportions',
      max: 2,
      options: [
        { id: 'pr_shortlegs', label_en: 'Short legs', visibleIn: ['normal', 'freak', 'total'] },
        { id: 'pr_longlimbs', label_en: 'Long limbs', visibleIn: ['normal', 'freak', 'total'] },
        { id: 'pr_shoulders', label_en: 'Broad shoulders', visibleIn: ['normal', 'freak', 'total'] },
      ],
    }),
    cat({
      id: 'height',
      options: [
        { id: 'h_tall', label_en: 'Tall', visibleIn: ['normal', 'freak', 'total'] },
        { id: 'h_very_tall', label_en: 'Very tall', visibleIn: ['normal', 'freak', 'total'] },
      ],
    }),
    cat({
      id: 'freak_neck',
      options: [{ id: 'neck_long', label_en: 'Long neck', visibleIn: ['normal', 'freak', 'total'] }],
    }),
    cat({
      id: 'freak_face',
      options: [{ id: 'ff_teeth_11', label_en: 'Buck teeth', visibleIn: ['normal', 'freak', 'total'] }],
    }),
  ]

  it('leaves a selection alone when the rule does not apply in this tier', () => {
    const sel = { body_type: ['body_muscular'], proportions: ['pr_shortlegs'] }

    const { selection, removed } = reconcileSelection(ruleCats, 'normal', sel)

    expect(selection).toEqual(sel)
    expect(removed).toEqual([])
  })

  it('drops only the named pick and keeps the rest of a multi-select category', () => {
    const { selection, removed } = reconcileSelection(ruleCats, 'total', {
      body_type: ['body_muscular'],
      proportions: ['pr_shortlegs', 'pr_shoulders'],
    })

    expect(selection.proportions).toEqual(['pr_shoulders'])
    expect(selection.body_type).toEqual(['body_muscular'])
    expect(removed).toEqual([
      { categoryId: 'proportions', optionId: 'pr_shortlegs', label: 'Short legs' },
    ])
  })

  it('clears the category when the named pick was its only value', () => {
    const { selection, removed } = reconcileSelection(ruleCats, 'freak', {
      body_type: ['body_heavy'],
      height: ['h_very_tall'],
    })

    expect(selection.height).toBeUndefined()
    expect(removed.map((r) => r.optionId)).toEqual(['h_very_tall'])
  })

  it('resolves the neck and teeth pair as well', () => {
    const { selection, removed } = reconcileSelection(ruleCats, 'total', {
      freak_neck: ['neck_long'],
      freak_face: ['ff_teeth_11'],
    })

    expect(selection.freak_neck).toEqual(['neck_long'])
    expect(selection.freak_face).toBeUndefined()
    expect(removed.map((r) => r.optionId)).toEqual(['ff_teeth_11'])
  })

  it('returns a legal selection unchanged and reports nothing', () => {
    const sel = { body_type: ['body_muscular'], proportions: ['pr_shoulders'] }

    const { selection, removed } = reconcileSelection(ruleCats, 'total', sel)

    expect(selection).toEqual(sel)
    expect(removed).toEqual([])
  })
})
