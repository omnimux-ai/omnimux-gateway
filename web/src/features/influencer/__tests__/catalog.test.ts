import { describe, expect, it } from 'vitest'

import {
  pickImageQuoteGroup,
  quoteCredits,
  resolveImageRoute,
} from '../lib/catalog'
import type { PricingRow } from '../types'

describe('one-image quote', () => {
  const row: PricingRow = {
    model_name: 'gpt-image-2.5-sunburst',
    vendor_id: 2,
    quota_type: 1,
    model_price: 0.3,
    billing_mode: 'tiered_expr',
    billing_expr: 'tier("image", fixed(0.013072)) * image_count',
    enable_groups: ['gpt-image-2.5-sunburst-image'],
  }
  it('quotes the active image expression rather than its inactive legacy price', () => {
    const quote = quoteCredits(row, 'gpt-image-2.5-sunburst-image', {
      'gpt-image-2.5-sunburst-image': 0.112455,
    })
    expect(quote?.usd).toBe(0.00147)
    expect(quote?.credits).toBe(0.0147)
  })
  it.each([
    '',
    'tier("image", ???)',
    'tier("tokens", p * 2)',
    'tier("image", fixed(0.01))',
    'tier("image", fixed(0.01)) * image_count * image_count',
    'true ? tier("image", fixed(0.01)) * image_count : tier("image", fixed(0.02)) * image_count',
    'tier("request", fixed(0.01)) * image_count',
    'tier("image", fixed(0.01)) * (param("quality") == "high" ? 2 : 1)',
  ])(
    'leaves an unknown expression price blank rather than using a stale flat price: %s',
    (billing_expr) => {
      expect(
        quoteCredits({ ...row, billing_expr }, 'gpt-image-2.5-sunburst-image', {
          'gpt-image-2.5-sunburst-image': 1,
        })
      ).toBeNull()
    }
  )
  it('preserves an explicitly free expression and rejects invalid group ratios', () => {
    const free = {
      ...row,
      billing_expr: 'tier("image", fixed(0)) * image_count',
    }
    expect(
      quoteCredits(free, 'gpt-image-2.5-sunburst-image', {
        'gpt-image-2.5-sunburst-image': 1,
      })
    ).toEqual({ usd: 0, credits: 0 })
    expect(
      quoteCredits(row, 'gpt-image-2.5-sunburst-image', {
        'gpt-image-2.5-sunburst-image': -1,
      })
    ).toBeNull()
  })
  it.each([
    [1, 0.013072],
    [0.9, 0.011764],
    [0.8, 0.010458],
    [0.112455, 0.00147],
  ])('uses native quota rounding at group ratio %s', (ratio, usd) => {
    expect(quoteCredits(row, 'image', { image: ratio })?.usd).toBe(usd)
  })
  it('rounds a positive half quota upward and rejects saturated quotes', () => {
    expect(
      quoteCredits(
        {
          ...row,
          billing_expr: 'tier("image", fixed(0.000003)) * image_count',
        },
        'image',
        { image: 1 }
      )?.usd
    ).toBe(0.000004)
    expect(
      quoteCredits(
        { ...row, billing_expr: 'tier("image", fixed(5000)) * image_count' },
        'image',
        { image: 1 }
      )
    ).toBeNull()
  })
  it('keeps legacy per-call pricing when expression billing is not active', () => {
    expect(
      quoteCredits(
        { ...row, billing_mode: 'ratio' },
        'gpt-image-2.5-sunburst-image',
        {
          'gpt-image-2.5-sunburst-image': 0.112455,
        }
      )?.credits
    ).toBeCloseTo(0.337365, 6)
  })
  it('leaves the quote blank when its anchor price is missing rather than borrowing a cheaper supplier price', () => {
    expect(
      quoteCredits(row, 'gpt-image-2.5-sunburst-image', { default: 0.01 })
    ).toBeNull()
    expect(quoteCredits(row, undefined, { default: 0.01 })).toBeNull()
  })
})

describe('image quote group', () => {
  const mockRow: PricingRow = {
    model_name: 'gpt-image-2.5-sunburst',
    vendor_id: 1,
    quota_type: 1,
    model_price: 0.05,
    enable_groups: ['default', 'gpt-image-2.5-sunburst-image', 'vip'],
  }

  it('keeps the workbench quote anchor when another supplier group has a lower sale ratio', () => {
    const userGroups = {
      default: { ratio: 0.01 },
      'gpt-image-2.5-sunburst-image': { ratio: 0.2 },
      vip: { ratio: 2.0 },
    }
    const group = pickImageQuoteGroup(mockRow, userGroups)
    expect(group).toBe('gpt-image-2.5-sunburst-image')
    expect(
      quoteCredits(mockRow, group ?? undefined, {
        default: 0.01,
        'gpt-image-2.5-sunburst-image': 0.2,
      })?.credits
    ).toBeCloseTo(0.1)
  })

  it('does not infer permission while caller groups are still loading', () => {
    expect(pickImageQuoteGroup(mockRow, undefined)).toBeNull()
    expect(pickImageQuoteGroup(mockRow, {})).toBeNull()
  })

  it('does not fall back to an unrelated group when the quote anchor is unavailable', () => {
    expect(pickImageQuoteGroup(mockRow, { default: { ratio: 1 } })).toBeNull()
    expect(
      pickImageQuoteGroup(
        { ...mockRow, enable_groups: ['default'] },
        { 'gpt-image-2.5-sunburst-image': { ratio: 1 } }
      )
    ).toBeNull()
  })

  it('does not infer routing for a historical unsupported model', () => {
    expect(
      pickImageQuoteGroup(
        { ...mockRow, model_name: 'historical-image-model' },
        { 'gpt-image-2.5-sunburst-image': { ratio: 1 } }
      )
    ).toBeNull()
    expect(pickImageQuoteGroup(undefined, {})).toBeNull()
  })
})

describe('image route resolution', () => {
  const sunburstRow: PricingRow = {
    model_name: 'gpt-image-2.5-sunburst',
    vendor_id: 2,
    quota_type: 1,
    billing_mode: 'tiered_expr',
    billing_expr: 'tier("image", fixed(0.014706)) * image_count',
    enable_groups: ['gpt-image-2.5-sunburst-image'],
  }
  const economyRow: PricingRow = {
    model_name: 'gpt-image-2.5',
    vendor_id: 2,
    quota_type: 1,
    billing_mode: 'tiered_expr',
    billing_expr: 'tier("image", fixed(0.013072)) * image_count',
    enable_groups: ['gpt-image-2.5-economy'],
  }
  const userGroups = {
    'gpt-image-2.5-sunburst-image': { ratio: 1 },
    'gpt-image-2.5-economy': { ratio: 0.112455 },
  }
  const groupRatio = {
    'gpt-image-2.5-sunburst-image': 1,
    'gpt-image-2.5-economy': 0.112455,
  }

  it('prefers the verified Sunburst route when the gateway sells it', () => {
    expect(
      resolveImageRoute([economyRow, sunburstRow], userGroups, groupRatio)
    ).toEqual({
      model: 'gpt-image-2.5-sunburst',
      group: 'gpt-image-2.5-sunburst-image',
    })
  })

  it('falls back to the audited economy route when Sunburst is not advertised', () => {
    expect(resolveImageRoute([economyRow], userGroups, groupRatio)).toEqual({
      model: 'gpt-image-2.5',
      group: 'gpt-image-2.5-economy',
    })
  })

  it('falls back when the Sunburst group is not usable for this account', () => {
    expect(
      resolveImageRoute(
        [sunburstRow, economyRow],
        { 'gpt-image-2.5-economy': { ratio: 0.112455 } },
        groupRatio
      )
    ).toEqual({ model: 'gpt-image-2.5', group: 'gpt-image-2.5-economy' })
  })

  it('closes image generation when no route can be quoted', () => {
    expect(resolveImageRoute([], userGroups, groupRatio)).toBeNull()
    expect(
      resolveImageRoute(
        [{ ...economyRow, enable_groups: ['default'] }],
        { default: { ratio: 1 } },
        { default: 1 }
      )
    ).toBeNull()
  })

  it('does not borrow the Sunburst anchor for the economy row', () => {
    expect(
      pickImageQuoteGroup(
        economyRow,
        { 'gpt-image-2.5-sunburst-image': { ratio: 1 } },
        'gpt-image-2.5-economy'
      )
    ).toBeNull()
  })
})
