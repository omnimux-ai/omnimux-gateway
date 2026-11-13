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

import officialBrandPricesData from './official-brand-prices.data.json'

/**
 * Curated official brand reference prices for the public pricing hall.
 *
 * Source of truth (ops ledger):
 * `scripts/ops/pricing-ledger/official-brand-prices.json`
 *
 * Token lanes are stored as USD / 1M tokens to match the public card default
 * token unit (`/M`). Request / media models use a representative primary tier.
 */

export type OfficialBrandPriceEntry = {
  /** Fallback / primary official USD used when no per-spec match exists. */
  usd: number
  /**
   * Optional ledger discount hint.
   * `0` means official 1:1 (same price); omit when unknown.
   */
  savingPercent?: number
  /** Per-spec official USD keyed by PublicPriceTier.spec labels. */
  bySpec?: Record<string, number>
}

/** Normalize catalog model ids so dated / preview variants share one ledger row. */
export function normalizeOfficialBrandModelId(modelId: string): string {
  const raw = modelId.trim().toLowerCase()
  if (!raw) return ''

  if (raw === 'gpt-image-1' || raw.startsWith('gpt-image-1-')) {
    return 'gpt-image-1-preview'
  }
  if (raw.startsWith('claude-3-5-sonnet')) return 'claude-3-5-sonnet'
  if (raw.startsWith('claude-3-7-sonnet')) return 'claude-3-7-sonnet'
  if (raw.startsWith('kling-v1.')) return 'kling-v1.5-pro'
  if (raw.startsWith('suno-v')) return 'suno-v3.5'
  if (raw.startsWith('minimax-video')) return 'minimax-video-01'
  if (raw.startsWith('eleven-multilingual')) return 'eleven-multilingual-v2'

  return raw
}

/**
 * Official brand price catalog keyed by normalized model id.
 * Compiled from `scripts/ops/pricing-ledger/official-brand-prices.json`.
 */
export const OFFICIAL_BRAND_PRICES: Record<string, OfficialBrandPriceEntry> =
  officialBrandPricesData as Record<string, OfficialBrandPriceEntry>

/**
 * Flat model → primary official USD map (backward-compatible with
 * `TransformPricingOptions.officialPriceMap`).
 */
export const OFFICIAL_BRAND_PRICE_MAP: Record<string, number> =
  Object.fromEntries(
    Object.entries(OFFICIAL_BRAND_PRICES).map(([modelId, entry]) => [
      modelId,
      entry.usd,
    ])
  )

export function lookupOfficialBrandPrice(
  modelId: string
): OfficialBrandPriceEntry | undefined {
  const normalized = normalizeOfficialBrandModelId(modelId)
  if (!normalized) return undefined
  return OFFICIAL_BRAND_PRICES[normalized]
}

/**
 * Resolve official USD for a model (+ optional tier spec).
 * Prefer per-spec lane prices for token rows; fall back to primary `usd`.
 */
export function resolveOfficialUsdForTier(
  modelId: string,
  spec?: string
): number | undefined {
  const entry = lookupOfficialBrandPrice(modelId)
  if (!entry) return undefined

  if (spec && entry.bySpec) {
    const direct = entry.bySpec[spec]
    if (typeof direct === 'number' && Number.isFinite(direct) && direct > 0) {
      return direct
    }

    // Dynamic tiers: "Tier · Input" / "Pro · Output"
    const lower = spec.toLowerCase()
    if (lower.includes('input') && entry.bySpec.Input) {
      return entry.bySpec.Input
    }
    if (lower.includes('output') && entry.bySpec.Output) {
      return entry.bySpec.Output
    }
    if (lower.includes('create cache') && entry.bySpec['Create cache']) {
      return entry.bySpec['Create cache']
    }
    if (lower.includes('cache') && entry.bySpec.Cache) {
      return entry.bySpec.Cache
    }
  }

  return Number.isFinite(entry.usd) && entry.usd > 0 ? entry.usd : undefined
}

/**
 * Build an `officialPriceMap` suitable for `transformPricingDataToCards`,
 * including common catalog aliases so live model ids resolve.
 */
export function buildOfficialPriceMap(
  extra?: Record<string, number>
): Record<string, number> {
  const map: Record<string, number> = { ...OFFICIAL_BRAND_PRICE_MAP }

  // Common live catalog aliases → ledger primary USD
  const aliases: Record<string, string> = {
    'gpt-image-1': 'gpt-image-1-preview',
    'claude-3-5-sonnet-20241022': 'claude-3-5-sonnet',
    'claude-3-7-sonnet-20250219': 'claude-3-7-sonnet',
    'kling-v1.6-pro': 'kling-v1.5-pro',
    'suno-v4': 'suno-v3.5',
  }

  for (const [alias, target] of Object.entries(aliases)) {
    const usd = OFFICIAL_BRAND_PRICE_MAP[target]
    if (typeof usd === 'number') {
      map[alias] = usd
    }
  }

  if (extra) {
    for (const [modelId, usd] of Object.entries(extra)) {
      if (typeof usd === 'number' && Number.isFinite(usd) && usd > 0) {
        map[modelId] = usd
      }
    }
  }

  return map
}
