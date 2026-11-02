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
  buildModelPagePricing,
  priceLadder,
} from '../model-page-pricing'
import type { PricingModel } from '../../types'

function modelWith(overrides: Partial<PricingModel>): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

describe('priceLadder', () => {
  it('uses the fixed per-request price when the model has no billing expression', () => {
    const ladder = priceLadder(modelWith({ model_price: 0.01 }), 1)
    expect(ladder).toHaveLength(1)
    expect(ladder[0].usd).toBeCloseTo(0.01)
  })

  it('applies the group discount multiplier to unit prices', () => {
    const ladder = priceLadder(modelWith({ model_price: 0.01 }), 0.5)
    expect(ladder[0].usd).toBeCloseTo(0.005)
  })

  it('returns no entries when the catalog cannot price the model', () => {
    expect(priceLadder(modelWith({ model_price: 0 }), 1)).toEqual([])
    expect(priceLadder(modelWith({}), 1)).toEqual([])
  })
})

describe('buildModelPagePricing', () => {
  it('reports no pricing at all rather than a placeholder when unpriceable', () => {
    const view = buildModelPagePricing(modelWith({}), 1, 1)
    expect(view).toEqual({
      cheapestUsd: null,
      firstTest: null,
      budgetRows: [],
      examples: [],
    })
  })

  it('derives the first test cost and budget runs from the cheapest price', () => {
    const view = buildModelPagePricing(modelWith({ model_price: 0.01 }), 1, 1)

    expect(view.firstTest?.usd).toBeCloseTo(0.01)
    // 1 USD = 10 credits, so a one-cent request is 0.1 credits.
    expect(view.firstTest?.credits).toBeCloseTo(0.1)
    expect(view.budgetRows).toEqual([
      { budgetUsd: 10, runs: 1000 },
      { budgetUsd: 50, runs: 5000 },
      { budgetUsd: 100, runs: 10000 },
    ])
  })

  it('drops budget tiers that cannot afford a single run', () => {
    // $40 per request leaves only the $50 and $100 tiers affordable.
    const view = buildModelPagePricing(modelWith({ model_price: 40 }), 1, 1)
    expect(view.budgetRows.map((row) => row.budgetUsd)).toEqual([50, 100])
  })

  it('exposes only one example when the model has a single price point', () => {
    const view = buildModelPagePricing(modelWith({ model_price: 0.01 }), 1, 1)
    expect(view.examples).toHaveLength(1)
  })
})
