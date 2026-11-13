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
import { usdToCredits } from './credits'
import { getTaskPricingDisplayTiers } from './task-matrix-display'
import type { PricingModel } from '../types'

/**
 * Pricing blocks for the model page (benchmark: "first test cost", "budget
 * reference", "cost examples").
 *
 * Every figure traces back to a catalog price — either a tier unit price from
 * the billing expression or the model's fixed per-request price. When neither
 * exists the block reports empty instead of inventing a number, and the page
 * hides it. Blocks are never computed from a hardcoded rate.
 */

/** Test budget tiers offered by the benchmark page. */
const BUDGET_TIERS_USD = [10, 50, 100] as const

export type PriceLadderEntry = {
  label: string
  /** Unit price in USD before the group/discount multiplier. */
  usd: number
}

export type PricedLine = {
  label: string
  usd: number
  credits: number
}

export type ModelPagePricingView = {
  /** Cheapest unit price found, or null when the catalog cannot price this model. */
  cheapestUsd: number | null
  firstTest: PricedLine | null
  budgetRows: { budgetUsd: number; runs: number }[]
  /** Up to three ladder entries; empty when no ladder exists. */
  examples: PricedLine[]
}

/**
 * Cheapest-to-priciest unit prices this model exposes.
 *
 * A tiered expression yields one entry per tier (its cheapest unit price); a
 * fixed per-request model yields a single entry. `priceRate` is applied so the
 * ladder matches what the pricing table shows.
 */
export function priceLadder(
  model: PricingModel,
  priceRate: number
): PriceLadderEntry[] {
  const tiers = getTaskPricingDisplayTiers(
    model.billing_expr,
    model.billing_usage_schema
  )
  const entries: PriceLadderEntry[] = []
  for (const tier of tiers) {
    const unitPrices = Object.values(tier.unitPrices ?? {}).filter(
      (value) => Number.isFinite(value) && value > 0
    )
    if (unitPrices.length === 0) continue
    entries.push({
      label: tier.label,
      usd: Math.min(...unitPrices) * priceRate,
    })
  }

  if (entries.length === 0) {
    const fixed = model.model_price
    if (Number.isFinite(fixed) && (fixed ?? 0) > 0) {
      entries.push({ label: 'per_request', usd: (fixed as number) * priceRate })
    }
  }

  return entries.sort((a, b) => a.usd - b.usd)
}

function toLine(
  label: string,
  usd: number,
  _usdExchangeRate: number
): PricedLine {
  return { label, usd, credits: usdToCredits(usd) }
}

export function buildModelPagePricing(
  model: PricingModel,
  priceRate: number,
  usdExchangeRate: number
): ModelPagePricingView {
  const ladder = priceLadder(model, priceRate)
  if (ladder.length === 0) {
    return {
      cheapestUsd: null,
      firstTest: null,
      budgetRows: [],
      examples: [],
    }
  }

  const cheapest = ladder[0]
  const budgetRows = BUDGET_TIERS_USD.map((budgetUsd) => ({
    budgetUsd,
    runs: Math.floor(budgetUsd / cheapest.usd),
  })).filter((row) => row.runs > 0)

  return {
    cheapestUsd: cheapest.usd,
    firstTest: toLine(cheapest.label, cheapest.usd, usdExchangeRate),
    budgetRows,
    examples: ladder
      .slice(0, 3)
      .map((entry) => toLine(entry.label, entry.usd, usdExchangeRate)),
  }
}
