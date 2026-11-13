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

import { randomizeSelection, type RandomSource } from '../lib/randomize'
import { conflictMessage } from '../lib/taxonomy'
import type {
  InfluencerCategory,
  InfluencerOption,
  InfluencerTaxonomy,
  Selection,
} from '../types'

const TIERS = ['normal', 'freak', 'total']
const FREAK_UP = ['freak', 'total']

function opt(
  id: string,
  extra: Partial<InfluencerOption> = {}
): InfluencerOption {
  return { id, label_en: id, visibleIn: TIERS, ...extra }
}

function cat(
  id: string,
  options: InfluencerOption[],
  max = 1
): InfluencerCategory {
  return { id, label_en: id, kind: 'media', max, options }
}

/**
 * Mirrors the shipped dataset closely enough to exercise all four hard
 * conflicts and the cross-category dependencies.
 */
const tax: InfluencerTaxonomy = {
  version: 1,
  source: 'test',
  tier_group: { options: [], comingSoonImageUrls: [] },
  category_priority: [],
  categories: [
    cat('gender', [
      opt('male'),
      opt('female'),
      opt('trans_man'),
      opt('trans_woman'),
      opt('non_binary'),
    ]),
    cat('ethnicity_origin_base', [opt('eth_a'), opt('eth_b'), opt('eth_c')]),
    cat('age', [opt('adult'), opt('mature'), opt('senior')]),
    cat('skin_tone', [opt('st_fair'), opt('st_deep')]),
    cat('eye_color', [opt('ec_brown'), opt('ec_blue')]),
    cat('height', [opt('h_average'), opt('h_tall'), opt('h_very_tall')]),
    cat('body_type', [
      opt('body_slim'),
      opt('body_athletic'),
      opt('body_heavy'),
      opt('body_muscular'),
      opt('body_curvy'),
      opt('body_ultra'),
    ]),
    cat(
      'proportions',
      [
        opt('pr_shortlegs'),
        opt('pr_longlimbs'),
        opt('pr_shoulders'),
        opt('pr_waist'),
        opt('pr_egg', { visibleIn: FREAK_UP }),
        opt('pr_potbelly', { visibleIn: FREAK_UP }),
      ],
      2
    ),
    cat('freak_neck', [
      opt('neck_normal'),
      opt('neck_column', { visibleIn: FREAK_UP }),
      opt('neck_long', { visibleIn: FREAK_UP }),
      opt('neck_short'),
    ]),
    cat(
      'freak_face',
      [
        opt('fn_freckles', { slot: 'marks' }),
        opt('fn_thickbrows', { slot: 'brows' }),
        opt('ff_brows_5', { visibleIn: FREAK_UP, slot: 'brows' }),
        opt('ff_teeth_11', { visibleIn: FREAK_UP, slot: 'teeth' }),
      ],
      4
    ),
    cat('facial_hair', [opt('fh_none'), opt('fh_beard')]),
    cat('hair', [
      opt('hs_lampshade', { visibleIn: FREAK_UP }),
      opt('hair_bob'),
    ]),
    cat('aesthetic', [opt('suits'), opt('retro'), opt('goth')]),
  ],
  prompt_map: { tiers: {}, categories: {} },
}

/** Deterministic generator so repeated runs are reproducible without mocking. */
function seeded(seed: number): RandomSource {
  let state = seed >>> 0
  return () => {
    state = (state * 1664525 + 1013904223) >>> 0
    return state / 4294967296
  }
}

function fixed(value: number): RandomSource {
  return () => value
}

function firstOf(
  sel: Record<string, string[] | undefined>,
  category: string
): string {
  return (sel[category] ?? [])[0] ?? ''
}

const BEARD_OPTIONS = ['fh_beard']
const BROW_OPTIONS = ['fn_thickbrows', 'ff_brows_5']

describe('randomizeSelection with fixed selections', () => {
  it('keeps fixed tall height and chooses a compatible build in either category order', () => {
    const categories = [
      cat('body_type', [opt('body_heavy'), opt('body_slim')]),
      cat('height', [opt('h_average'), opt('h_tall')]),
    ]
    for (const ordered of [categories, [...categories].reverse()]) {
      const sel = randomizeSelection(
        { ...tax, categories: ordered },
        'total',
        fixed(0),
        {
          height: ['h_tall'],
        }
      )
      expect(sel.height).toEqual(['h_tall'])
      expect(sel.body_type).toEqual(['body_slim'])
      expect(conflictMessage('total', sel)).toBeNull()
    }
  })

  it('keeps the exact fixed multi-select subset without adding a random option or mutating input', () => {
    const picks = ['pr_waist', 'pr_shoulders']
    Object.freeze(picks)
    const manual: Selection = { proportions: picks }
    Object.freeze(manual)
    const sel = randomizeSelection(tax, 'total', fixed(0), manual)
    expect(sel.proportions).toEqual(['pr_waist', 'pr_shoulders'])
    expect(sel.proportions).not.toBe(picks)
    expect(manual).toEqual({ proportions: ['pr_waist', 'pr_shoulders'] })
    sel.proportions.push('pr_egg')
    expect(picks).toEqual(['pr_waist', 'pr_shoulders'])
  })

  it('keeps fixed short-leg proportions and excludes heavy, muscular and ultra builds', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('body_type', [
          opt('body_heavy'),
          opt('body_muscular'),
          opt('body_ultra'),
          opt('body_slim'),
        ]),
        cat('proportions', [opt('pr_shortlegs'), opt('pr_shoulders')]),
      ],
    }
    const sel = randomizeSelection(only, 'total', fixed(0), {
      proportions: ['pr_shortlegs'],
    })
    expect(sel.proportions).toEqual(['pr_shortlegs'])
    expect(sel.body_type).toEqual(['body_slim'])
  })

  it('keeps fixed female gender and selects no beard even when facial hair precedes gender', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('facial_hair', [opt('fh_beard'), opt('fh_none')]),
        cat('gender', [opt('male'), opt('female')]),
      ],
    }
    const sel = randomizeSelection(only, 'total', fixed(0), {
      gender: ['female'],
    })
    expect(sel.gender).toEqual(['female'])
    expect(sel.facial_hair).toEqual(['fh_none'])
  })

  it('keeps a fixed beard and only rolls a beard-capable gender', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('gender', [opt('female'), opt('male'), opt('trans_man')]),
        cat('facial_hair', [opt('fh_none'), opt('fh_beard')]),
      ],
    }
    const sel = randomizeSelection(only, 'total', fixed(0.8), {
      facial_hair: ['fh_beard'],
    })
    expect(sel.facial_hair).toEqual(['fh_beard'])
    expect(['male', 'trans_man']).toContain(sel.gender?.[0])
  })

  it('keeps conflicting fixed female and beard selections instead of repairing either', () => {
    const manual = { gender: ['female'], facial_hair: ['fh_beard'] }
    const sel = randomizeSelection(tax, 'total', fixed(0), manual)
    expect(sel.gender).toEqual(['female'])
    expect(sel.facial_hair).toEqual(['fh_beard'])
    expect(manual).toEqual({ gender: ['female'], facial_hair: ['fh_beard'] })
    expect(sel.age).toHaveLength(1)
  })

  it('keeps fixed no-brows and excludes brow features regardless of category order', () => {
    const categories = [
      cat('freak_face', [opt('fn_thickbrows'), opt('fn_freckles')]),
      cat('distinctive', [opt('df_scar'), opt('df_nobrows')]),
    ]
    for (const ordered of [categories, [...categories].reverse()]) {
      const sel = randomizeSelection(
        { ...tax, categories: ordered },
        'total',
        fixed(0),
        {
          distinctive: ['df_nobrows'],
        }
      )
      expect(sel.distinctive).toEqual(['df_nobrows'])
      expect(sel.freak_face).toEqual(['fn_freckles'])
    }
  })

  it('keeps fixed brow features and excludes no-brows regardless of category order', () => {
    const categories = [
      cat('distinctive', [opt('df_nobrows'), opt('df_scar')]),
      cat('freak_face', [opt('fn_freckles'), opt('fn_thickbrows')]),
    ]
    for (const ordered of [categories, [...categories].reverse()]) {
      const sel = randomizeSelection(
        { ...tax, categories: ordered },
        'total',
        fixed(0),
        {
          freak_face: ['fn_thickbrows'],
        }
      )
      expect(sel.freak_face).toEqual(['fn_thickbrows'])
      expect(sel.distinctive).toEqual(['df_scar'])
    }
  })

  it('keeps fixed long neck and excludes ultra build and buck teeth', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('body_type', [opt('body_ultra'), opt('body_slim')]),
        cat('freak_face', [opt('ff_teeth_11'), opt('fn_freckles')]),
        cat('freak_neck', [opt('neck_short'), opt('neck_long')]),
      ],
    }
    const sel = randomizeSelection(only, 'total', fixed(0), {
      freak_neck: ['neck_long'],
    })
    expect(sel.freak_neck).toEqual(['neck_long'])
    expect(sel.body_type).toEqual(['body_slim'])
    expect(sel.freak_face).toEqual(['fn_freckles'])
  })

  it('keeps fixed buck teeth and excludes long neck', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('freak_neck', [opt('neck_long'), opt('neck_short')]),
        cat('freak_face', [opt('fn_freckles'), opt('ff_teeth_11')]),
      ],
    }
    const sel = randomizeSelection(only, 'total', fixed(0), {
      freak_face: ['ff_teeth_11'],
    })
    expect(sel.freak_face).toEqual(['ff_teeth_11'])
    expect(sel.freak_neck).toEqual(['neck_short'])
  })

  it('leaves an unfixed build empty when fixed tall height has no compatible candidate', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('body_type', [opt('body_heavy')]),
        cat('height', [opt('h_tall')]),
      ],
    }
    const sel = randomizeSelection(only, 'total', fixed(0), {
      height: ['h_tall'],
    })
    expect(sel.height).toEqual(['h_tall'])
    expect(sel.body_type).toBeUndefined()
    expect(conflictMessage('total', sel)).toBeNull()
  })

  it('leaves beard and brow categories empty rather than falling back to incompatible pools', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('gender', [opt('female')]),
        cat('facial_hair', [opt('fh_beard')]),
        cat('distinctive', [opt('df_nobrows')]),
        cat('freak_face', [opt('fn_thickbrows')]),
      ],
    }
    const sel = randomizeSelection(only, 'total', fixed(0), {
      gender: ['female'],
      distinctive: ['df_nobrows'],
    })
    expect(sel.facial_hair).toBeUndefined()
    expect(sel.freak_face).toBeUndefined()
    expect(sel.gender).toEqual(['female'])
    expect(sel.distinctive).toEqual(['df_nobrows'])
  })

  it('preserves conflicting fixed picks while still filling unrelated categories without retrying', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('body_type', [opt('body_heavy')]),
        cat('height', [opt('h_average'), opt('h_tall')]),
        cat('age', [opt('adult')]),
      ],
    }
    let calls = 0
    const random = () => {
      calls += 1
      if (calls > 2) throw new Error('Unexpected random retry')
      return 0
    }
    const sel = randomizeSelection(only, 'total', random, {
      body_type: ['body_heavy'],
      height: ['h_tall'],
    })
    expect(sel).toEqual({
      body_type: ['body_heavy'],
      height: ['h_tall'],
      age: ['adult'],
    })
    expect(conflictMessage('total', sel)).not.toBeNull()
  })

  it('returns all legal fixed categories without calling randomness, including profile-excluded picks', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('body_type', [opt('body_curvy')]),
        cat('height', [opt('h_tall')]),
      ],
    }
    const manual = { body_type: ['body_curvy'], height: ['h_tall'] }
    const sel = randomizeSelection(
      only,
      'normal',
      () => {
        throw new Error('All categories are fixed')
      },
      manual
    )
    expect(sel).toEqual(manual)
  })

  it('ignores unknown, invisible and empty fixed items and rolls those categories normally', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('height', [
          opt('h_average'),
          opt('h_tall', { visibleIn: FREAK_UP }),
        ]),
        cat('age', [opt('adult')]),
        cat('gender', [opt('male'), opt('female')]),
      ],
    }
    const invalid = {
      height: ['missing', 'h_tall'],
      age: [],
      gender: ['missing'],
      stale: ['value'],
    }
    const sel = randomizeSelection(only, 'normal', fixed(0), invalid)
    expect(sel).toEqual({
      gender: ['male'],
      height: ['h_average'],
      age: ['adult'],
    })
    expect(invalid).toEqual({
      height: ['missing', 'h_tall'],
      age: [],
      gender: ['missing'],
      stale: ['value'],
    })
  })

  it('keeps valid fixed items from a mixed array without carrying unknown or invisible items', () => {
    const manual = {
      proportions: ['missing', 'pr_waist', 'pr_egg', 'pr_shoulders'],
    }
    const sel = randomizeSelection(tax, 'normal', fixed(0), manual)
    expect(sel.proportions).toEqual(['pr_waist', 'pr_shoulders'])
    expect(manual.proportions).toEqual([
      'missing',
      'pr_waist',
      'pr_egg',
      'pr_shoulders',
    ])
  })

  it('matches the existing deterministic result when fixed selections contain no valid items', () => {
    expect(randomizeSelection(tax, 'total', seeded(41), {})).toEqual(
      randomizeSelection(tax, 'total', seeded(41))
    )
    expect(
      randomizeSelection(tax, 'total', seeded(41), {
        stale: ['missing'],
        age: [],
      })
    ).toEqual(randomizeSelection(tax, 'total', seeded(41)))
  })
})

describe('randomizeSelection', () => {
  it('fills every category that has an option in the tier', () => {
    for (const tier of TIERS) {
      const sel = randomizeSelection(tax, tier, seeded(7))
      for (const c of tax.categories) {
        expect(sel[c.id], `${tier} ${c.id}`).toHaveLength(1)
      }
    }
  })

  it('never returns a selection that trips a hard conflict', () => {
    for (const tier of TIERS) {
      const random = seeded(11)
      for (let run = 0; run < 4000; run++) {
        const sel = randomizeSelection(tax, tier, random)
        expect(conflictMessage(tier, sel), `${tier} run ${run}`).toBeNull()
      }
    }
  })

  it('re-rolls the height instead of blanking the card when heavy meets tall', () => {
    const heavyTall: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('body_type', [opt('body_heavy')]),
        cat('height', [opt('h_average'), opt('h_tall')]),
      ],
    }
    const sel = randomizeSelection(heavyTall, 'normal', seeded(3))
    expect(sel.height).toEqual(['h_average'])
    expect(conflictMessage('normal', sel)).toBeNull()
  })

  it('re-rolls the neck instead of the build when ultra meets a long neck', () => {
    const ultraNeck: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('body_type', [opt('body_ultra')]),
        cat('freak_neck', [opt('neck_long'), opt('neck_short')]),
      ],
    }
    const sel = randomizeSelection(ultraNeck, 'total', seeded(5))
    expect(sel.body_type).toEqual(['body_ultra'])
    expect(sel.freak_neck).toEqual(['neck_short'])
    expect(conflictMessage('total', sel)).toBeNull()
  })

  it('re-rolls the face feature instead of the neck when a long neck meets buck teeth', () => {
    const neckTeeth: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('freak_neck', [opt('neck_long')]),
        cat('freak_face', [
          opt('ff_teeth_11', { slot: 'teeth' }),
          opt('fn_freckles'),
        ]),
      ],
    }
    const sel = randomizeSelection(neckTeeth, 'total', seeded(9))
    expect(sel.freak_neck).toEqual(['neck_long'])
    expect(sel.freak_face).toEqual(['fn_freckles'])
    expect(conflictMessage('total', sel)).toBeNull()
  })

  it('never gives a beard to a gender that does not grow one', () => {
    const genders = ['female', 'trans_woman', 'non_binary']
    for (const gender of genders) {
      const only: InfluencerTaxonomy = {
        ...tax,
        categories: [
          cat('gender', [opt(gender)]),
          cat('facial_hair', [opt('fh_none'), opt('fh_beard')]),
        ],
      }
      const sel = randomizeSelection(only, 'total', seeded(13))
      expect(firstOf(sel, 'facial_hair'), gender).toBe('fh_none')
    }
  })

  it('lets a beard-capable gender roll facial hair', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('gender', [opt('male')]),
        cat('facial_hair', [opt('fh_none'), opt('fh_beard')]),
      ],
    }
    expect(randomizeSelection(only, 'total', fixed(0.5)).facial_hair).toEqual([
      'fh_beard',
    ])
  })

  it('drops "no brows" when a brow feature is already picked', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('freak_face', [opt('fn_thickbrows', { slot: 'brows' })]),
        cat('distinctive', [opt('df_nobrows'), opt('df_scar')]),
      ],
    }
    expect(randomizeSelection(only, 'total', fixed(0.1)).distinctive).toEqual([
      'df_scar',
    ])
  })

  it('drops the brow feature when "no brows" is already picked', () => {
    const only: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('distinctive', [opt('df_nobrows')]),
        cat('freak_face', [
          opt('fn_thickbrows', { slot: 'brows' }),
          opt('fn_freckles'),
        ]),
      ],
    }
    const sel = randomizeSelection(only, 'total', fixed(0.1))
    expect(sel.distinctive).toEqual(['df_nobrows'])
    expect(sel.freak_face).toEqual(['fn_freckles'])
  })

  it('keeps "no brows" and a brow feature apart on every run', () => {
    const random = seeded(17)
    for (let run = 0; run < 4000; run++) {
      const sel = randomizeSelection(tax, 'total', random)
      const hasNoBrows = firstOf(sel, 'distinctive') === 'df_nobrows'
      const brow = firstOf(sel, 'freak_face')
      if (hasNoBrows) expect(BROW_OPTIONS, `run ${run}`).not.toContain(brow)
    }
  })

  it('excludes curvy in every tier and ultra in the average tier', () => {
    for (const tier of TIERS) {
      const random = seeded(19)
      for (let run = 0; run < 2000; run++) {
        const body = firstOf(randomizeSelection(tax, tier, random), 'body_type')
        expect(body, `${tier} run ${run}`).not.toBe('body_curvy')
        if (tier === 'normal') {
          expect(body, `normal run ${run}`).not.toBe('body_ultra')
        }
      }
    }
  })

  it('follows the upstream weighting for gender, aesthetic and hair', () => {
    const runs = 20000
    const gender = { male: 0, female: 0 }
    const aesthetic = { suits: 0, retro: 0 }
    let lampshade = 0
    const random = seeded(23)
    for (let run = 0; run < runs; run++) {
      const sel = randomizeSelection(tax, 'total', random)
      gender[firstOf(sel, 'gender') as 'male' | 'female'] += 1
      aesthetic[firstOf(sel, 'aesthetic') as 'suits' | 'retro'] += 1
      if (firstOf(sel, 'hair') === 'hs_lampshade') lampshade += 1
    }
    expect(gender.male / runs).toBeGreaterThan(0.65)
    expect(gender.male / runs).toBeLessThan(0.69)
    expect(aesthetic.suits / runs).toBeGreaterThan(0.38)
    expect(aesthetic.suits / runs).toBeLessThan(0.42)
    expect(aesthetic.retro / runs).toBeGreaterThan(0.18)
    expect(aesthetic.retro / runs).toBeLessThan(0.22)
    expect(lampshade / runs).toBeGreaterThan(0.18)
    expect(lampshade / runs).toBeLessThan(0.22)
  })

  it('never picks a category whose options are all invisible in the tier', () => {
    const freakOnly: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('gender', [opt('male'), opt('female')]),
        cat('aesthetic', [opt('goth', { visibleIn: ['freak'] })]),
      ],
    }
    const sel = randomizeSelection(freakOnly, 'normal', seeded(29))
    expect(sel.aesthetic).toBeUndefined()
    expect(sel.gender).toHaveLength(1)
  })

  it('drops a category only when the tier leaves no compatible option', () => {
    const impossible: InfluencerTaxonomy = {
      ...tax,
      categories: [
        cat('body_type', [opt('body_heavy')]),
        cat('height', [opt('h_tall')]),
      ],
    }
    const sel = randomizeSelection(impossible, 'normal', seeded(31))
    expect(sel.height).toBeUndefined()
    expect(conflictMessage('normal', sel)).toBeNull()
  })

  it('keeps beards off every non-beard gender across many runs', () => {
    const random = seeded(37)
    for (let run = 0; run < 4000; run++) {
      const sel = randomizeSelection(tax, 'total', random)
      const gender = firstOf(sel, 'gender')
      if (gender !== 'male' && gender !== 'trans_man') {
        expect(BEARD_OPTIONS, `run ${run} gender ${gender}`).not.toContain(
          firstOf(sel, 'facial_hair')
        )
      }
    }
  })
})
