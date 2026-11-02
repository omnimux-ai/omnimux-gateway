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

import { calculateSaving } from './credits'
import type { PublicPriceTier } from '../types'

export type SavingBadgeKind = 'off' | 'same' | 'none'

export type SavingBadgeModel = {
  kind: SavingBadgeKind
  /** Positive integer percent when kind === 'off'. */
  percent: number | null
}

/**
 * Classify a saving value for UI badges.
 * - off: positive discount vs official
 * - same: official 1:1 (savingPercent === 0 with an official price present)
 * - none: no comparable official price / unavailable
 */
export function classifySavingBadge(
  saving: number | null | undefined,
  hasOfficialPrice: boolean
): SavingBadgeModel {
  if (!hasOfficialPrice) {
    return { kind: 'none', percent: null }
  }

  if (typeof saving === 'number' && Number.isFinite(saving)) {
    if (saving > 0) {
      return { kind: 'off', percent: Math.round(saving) }
    }
    if (saving === 0) {
      return { kind: 'same', percent: 0 }
    }
  }

  return { kind: 'none', percent: null }
}

/**
 * Resolve saving badge model for a public price tier.
 */
export function resolveTierSavingBadge(
  tier: Pick<PublicPriceTier, 'credits' | 'officialUsd' | 'savingPercent'>
): SavingBadgeModel {
  const hasOfficial =
    typeof tier.officialUsd === 'number' &&
    Number.isFinite(tier.officialUsd) &&
    tier.officialUsd > 0

  if (!hasOfficial) {
    return { kind: 'none', percent: null }
  }

  const saving = calculateSaving(
    tier.credits,
    tier.officialUsd,
    tier.savingPercent
  )

  return classifySavingBadge(saving, true)
}

/**
 * Pick the best (highest) positive saving across tiers for a card-level badge.
 * Falls back to "same" when every comparable tier is official 1:1.
 */
export function resolveCardSavingBadge(
  tiers: Array<
    Pick<PublicPriceTier, 'credits' | 'officialUsd' | 'savingPercent'>
  >
): SavingBadgeModel {
  let bestOff: number | null = null
  let sawSame = false
  let sawOfficial = false

  for (const tier of tiers) {
    const badge = resolveTierSavingBadge(tier)
    if (badge.kind === 'off' && badge.percent !== null) {
      sawOfficial = true
      bestOff =
        bestOff === null ? badge.percent : Math.max(bestOff, badge.percent)
      continue
    }
    if (badge.kind === 'same') {
      sawOfficial = true
      sawSame = true
    }
  }

  if (bestOff !== null && bestOff > 0) {
    return { kind: 'off', percent: bestOff }
  }
  if (sawSame && sawOfficial) {
    return { kind: 'same', percent: 0 }
  }
  return { kind: 'none', percent: null }
}
