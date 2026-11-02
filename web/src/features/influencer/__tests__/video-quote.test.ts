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
  quoteVideoCredits,
  VIDEO_MODEL,
  VIDEO_QUOTE_GROUPS,
  videoQuoteGroup,
} from '../lib/video-quote'
import type { PricingResponse, PricingRow } from '../types'

const ALL_GROUPS = ['seedance-2-5-480p', 'seedance-2-5-720p']

function videoPricing(overrides: Partial<PricingRow> = {}): PricingResponse {
  return {
    success: true,
    data: [
      {
        model_name: VIDEO_MODEL,
        vendor_id: 23,
        quota_type: 1,
        model_price: 0.8,
        enable_groups: ALL_GROUPS,
        ...overrides,
      },
    ],
    vendors: [],
    group_ratio: { 'seedance-2-5-480p': 0.524, 'seedance-2-5-720p': 0.7206 },
  }
}

const USER_GROUPS = {
  'seedance-2-5-480p': { ratio: 0.524 },
  'seedance-2-5-720p': { ratio: 0.7206 },
}

describe('videoQuoteGroup', () => {
  it('maps the workbench resolutions to the gateway tier groups', () => {
    expect(videoQuoteGroup('480p')).toBe(VIDEO_QUOTE_GROUPS['480p'])
    expect(videoQuoteGroup('720p')).toBe(VIDEO_QUOTE_GROUPS['720p'])
  })

  it('rejects a resolution the workbench does not sell', () => {
    expect(videoQuoteGroup('1080p')).toBeNull()
  })
})

describe('quoteVideoCredits', () => {
  it('prices a 20s 480p clip from the account group ratio', () => {
    const quote = quoteVideoCredits(videoPricing(), USER_GROUPS, '480p', 20)
    expect(quote).not.toBeNull()
    expect(quote?.usd).toBeCloseTo(8.384, 6)
    expect(quote?.credits).toBeCloseTo(83.84, 6)
  })

  it('prices a 7s 720p clip at the 720p tier', () => {
    const quote = quoteVideoCredits(videoPricing(), USER_GROUPS, '720p', 7)
    expect(quote?.credits).toBeCloseTo(40.3536, 6)
  })

  it('uses the account special ratio instead of applying the discount twice', () => {
    const quote = quoteVideoCredits(
      videoPricing(),
      { 'seedance-2-5-480p': { ratio: 0.4192 } },
      '480p',
      20
    )
    // 0.8 × 0.4192 × 20 × 10; a second ×0.8 would wrongly give 53.66.
    expect(quote?.credits).toBeCloseTo(67.072, 6)
  })

  it('ignores the catalog-wide group_ratio when the account ratio is supplied', () => {
    const pricing = videoPricing()
    pricing.group_ratio = { 'seedance-2-5-480p': 1, 'seedance-2-5-720p': 1 }
    const quote = quoteVideoCredits(pricing, USER_GROUPS, '480p', 20)
    expect(quote?.credits).toBeCloseTo(83.84, 6)
  })

  it('accepts both duration boundaries', () => {
    expect(
      quoteVideoCredits(videoPricing(), USER_GROUPS, '480p', 4)?.credits
    ).toBeCloseTo(16.768, 6)
    expect(
      quoteVideoCredits(videoPricing(), USER_GROUPS, '720p', 30)?.credits
    ).toBeCloseTo(172.944, 6)
  })

  it.each([3, 31, 7.5, 0, -4])(
    'returns null for an out-of-range duration of %s',
    (seconds) => {
      expect(
        quoteVideoCredits(videoPricing(), USER_GROUPS, '480p', seconds)
      ).toBeNull()
    }
  )

  it('returns null when the video model is not in the catalog', () => {
    const pricing = videoPricing({ model_name: 'gpt-image-2.5' })
    expect(quoteVideoCredits(pricing, USER_GROUPS, '480p', 20)).toBeNull()
  })

  it('returns null when the tier group is not enabled for the model', () => {
    const pricing = videoPricing({ enable_groups: ['seedance-2-5-480p'] })
    expect(quoteVideoCredits(pricing, USER_GROUPS, '720p', 20)).toBeNull()
  })

  it('returns null when the video model moves to expression billing', () => {
    const pricing = videoPricing({ billing_mode: 'tiered_expr' })
    expect(quoteVideoCredits(pricing, USER_GROUPS, '480p', 20)).toBeNull()
  })

  it.each([0, -0.5, Number.NaN, Number.POSITIVE_INFINITY])(
    'returns null for an unusable group ratio of %s',
    (ratio) => {
      expect(
        quoteVideoCredits(
          videoPricing(),
          { 'seedance-2-5-480p': { ratio } },
          '480p',
          20
        )
      ).toBeNull()
    }
  )

  it.each([0, -1, Number.NaN, Number.POSITIVE_INFINITY, undefined])(
    'returns null for an unusable per-second price of %s',
    (modelPrice) => {
      expect(
        quoteVideoCredits(
          videoPricing({ model_price: modelPrice }),
          USER_GROUPS,
          '480p',
          20
        )
      ).toBeNull()
    }
  )

  it('returns null when the pricing or groups payload is missing', () => {
    expect(quoteVideoCredits(undefined, USER_GROUPS, '480p', 20)).toBeNull()
    expect(quoteVideoCredits(videoPricing(), undefined, '480p', 20)).toBeNull()
    expect(quoteVideoCredits(videoPricing(), {}, '480p', 20)).toBeNull()
  })
})
