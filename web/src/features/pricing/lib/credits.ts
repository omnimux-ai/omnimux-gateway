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
import {
  CREDITS_PER_USD,
  DEFAULT_QUOTA_PER_UNIT,
  USD_PER_CREDIT,
} from '../constants'
import { compileBillingExpression } from './billing-expression/parser'
import { evaluateBillingExpression } from './billing-expression/runtime'

// ----------------------------------------------------------------------------
// One-way Credits ↔ USD ↔ raw_quota display helpers (pure, no side effects)
// ----------------------------------------------------------------------------

/**
 * Convert Credits to USD.
 * Hard rule: 1 Credit = $0.1 → credits * 0.1
 */
export function creditsToUsd(credits: number): number {
  if (!Number.isFinite(credits)) return Number.NaN
  return credits * USD_PER_CREDIT
}

/**
 * Convert USD to Credits.
 * Hard rule: 1 USD = 10 Credits → usd * 10
 */
export function usdToCredits(usd: number): number {
  if (!Number.isFinite(usd)) return Number.NaN
  return usd * CREDITS_PER_USD
}

/**
 * Convert raw_quota units to USD.
 * Hard rule: usd = raw_quota / quota_per_unit (default 500000).
 * Never label raw_quota as "Credits".
 */
export function rawQuotaToUsd(
  rawQuota: number,
  quotaPerUnit: number = DEFAULT_QUOTA_PER_UNIT
): number {
  if (!Number.isFinite(rawQuota)) return Number.NaN
  const unit =
    Number.isFinite(quotaPerUnit) && quotaPerUnit > 0
      ? quotaPerUnit
      : DEFAULT_QUOTA_PER_UNIT
  return rawQuota / unit
}

/**
 * Format Credits for display: supports decimals and large numbers with grouping.
 * Trailing zeros are stripped (e.g. 1.50 → "1.5", 1000 → "1,000").
 */
export function formatCredits(credits: number): string {
  if (!Number.isFinite(credits)) return '—'

  const abs = Math.abs(credits)
  let maximumFractionDigits = 0
  if (abs > 0 && abs < 1) {
    maximumFractionDigits = 4
  } else if (abs > 0 && abs < 100) {
    maximumFractionDigits = 2
  }

  const formatted = new Intl.NumberFormat('en-US', {
    maximumFractionDigits,
    minimumFractionDigits: 0,
  }).format(credits)

  return stripTrailingFractionalZeros(formatted)
}

/**
 * Format a USD numeric value, stripping redundant trailing zeros.
 * Does not prepend a currency symbol — callers compose "~$…" as needed.
 */
export function formatUsdValue(val: number): string {
  if (!Number.isFinite(val)) return '—'

  const abs = Math.abs(val)
  let maximumFractionDigits = 2
  if (abs > 0 && abs < 0.01) {
    maximumFractionDigits = 6
  } else if (abs > 0 && abs < 1) {
    maximumFractionDigits = 4
  }

  const formatted = new Intl.NumberFormat('en-US', {
    maximumFractionDigits,
    minimumFractionDigits: 0,
  }).format(val)

  return stripTrailingFractionalZeros(formatted)
}

/**
 * Compute savings percent vs official USD price.
 * Prefer explicitSaving when finite (including 0 for official 1:1).
 * Otherwise derive from credits / officialUsd.
 * Returns null when saving cannot be determined or our price is higher.
 */
export function calculateSaving(
  credits: number,
  officialUsd?: number,
  explicitSaving?: number
): number | null {
  if (Number.isFinite(explicitSaving)) {
    const rounded = Math.round(Number(explicitSaving))
    return rounded >= 0 ? rounded : null
  }

  if (!Number.isFinite(credits) || !Number.isFinite(officialUsd)) {
    return null
  }

  const official = Number(officialUsd)
  if (official <= 0) return null

  const ourUsd = creditsToUsd(credits)
  if (!Number.isFinite(ourUsd) || ourUsd < 0) return null

  const saving = ((official - ourUsd) / official) * 100
  if (!Number.isFinite(saving)) return null
  // Absorb Credits↔USD float noise around official 1:1.
  if (Math.abs(saving) < 0.5) return 0
  if (saving < 0) return null

  return Math.round(saving)
}

/**
 * Resolve a displayable saving value for a tier that already has official USD.
 * Prefer explicitSaving (including 0); otherwise derive from credits.
 */
export function resolveSavingPercent(
  credits: number,
  officialUsd?: number,
  explicitSaving?: number
): number | null {
  return calculateSaving(credits, officialUsd, explicitSaving)
}

function stripTrailingFractionalZeros(formatted: string): string {
  if (!formatted.includes('.')) return formatted
  return formatted.replace(/(\.\d*?[1-9])0+$|\.0+$/, '$1')
}

/** Single-image service quote, with the native positive quota rounding order. */
export function quoteFixedImageExpression(
  expression: string,
  groupRatio: number
): { usd: number; credits: number } | null {
  if (!Number.isFinite(groupRatio) || groupRatio < 0) return null
  const compiled = compileBillingExpression(expression)
  if (compiled.status !== 'ready') return null
  const product = compiled.ast
  if (
    product.kind !== 'binary' ||
    product.operator !== '*' ||
    product.right.kind !== 'variable' ||
    product.right.name !== 'image_count'
  ) {
    return null
  }
  const tier = product.left
  if (
    tier.kind !== 'call' ||
    tier.name !== 'tier' ||
    tier.args.length !== 2 ||
    tier.args[0]?.kind !== 'literal' ||
    tier.args[0].value !== 'image'
  ) {
    return null
  }
  const fixed = tier.args[1]
  if (
    fixed?.kind !== 'call' ||
    fixed.name !== 'fixed' ||
    fixed.args.length !== 1 ||
    fixed.args[0]?.kind !== 'literal' ||
    typeof fixed.args[0].value !== 'number'
  ) {
    return null
  }
  const evaluated = evaluateBillingExpression(compiled, { imageCount: 1 })
  if (evaluated.status !== 'success' || evaluated.billingUnit !== 'request') {
    return null
  }
  // Keep division, quota conversion and the group multiplier in native order.
  const raw = Math.round(
    (evaluated.cost / 1_000_000) * DEFAULT_QUOTA_PER_UNIT * groupRatio
  )
  if (!Number.isFinite(raw) || raw < 0 || raw > 2_147_483_647) return null
  const usd = rawQuotaToUsd(raw)
  return { usd, credits: usdToCredits(usd) }
}
