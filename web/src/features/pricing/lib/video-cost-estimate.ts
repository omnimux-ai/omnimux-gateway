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
import { formatNumber } from '@/lib/format'

import { QUOTA_TYPE_VALUES } from '../constants'
import type { PricingData } from '../types'
import { usdToCredits } from './credits'

export type VideoEstimateRate = {
  id: string
  label: string
  creditsPerSecond: number
}

// Published duration_seconds base rates in modelcontracts/contracts.json.
// Keep the full precision until formatting the final budget estimate.
const BASE_ESTIMATE_RATES: readonly VideoEstimateRate[] = [
  {
    id: 'seedance-2-5',
    label: 'Seedance 2.5',
    creditsPerSecond: usdToCredits(0.8),
  },
  { id: 'wan-3.0', label: 'Wan 3.0', creditsPerSecond: usdToCredits(0.176471) },
  {
    id: 'seedance-2-0',
    label: 'Seedance 2.0',
    creditsPerSecond: usdToCredits(0.0971),
  },
]

export const DEFAULT_ESTIMATE_SECONDS = 15
export const REFERENCE_CLIP_CREDITS =
  BASE_ESTIMATE_RATES[0].creditsPerSecond * DEFAULT_ESTIMATE_SECONDS

export function formatVideoCreditsPerSecond(
  credits: number,
  locale: Intl.LocalesArgument
) {
  if (credits >= 1) return formatNumber(credits, locale)
  return new Intl.NumberFormat(locale, { maximumFractionDigits: 3 }).format(
    credits
  )
}

export function getVideoEstimateRates(
  data: PricingData | undefined,
  selectedGroup: string | undefined
): readonly VideoEstimateRate[] {
  if (
    !data?.success ||
    !selectedGroup ||
    !Object.hasOwn(data.usable_group ?? {}, selectedGroup)
  ) {
    return BASE_ESTIMATE_RATES
  }

  const ratio = data.group_ratio?.[selectedGroup]
  if (typeof ratio !== 'number' || !Number.isFinite(ratio) || ratio < 0) {
    return BASE_ESTIMATE_RATES
  }

  return BASE_ESTIMATE_RATES.map((base) => {
    const model = data.data?.find((row) => row.model_name === base.id)
    const billing = model?.contract?.billing
    if (
      model?.quota_type !== QUOTA_TYPE_VALUES.REQUEST ||
      !model.enable_groups?.includes(selectedGroup) ||
      billing?.unit !== 'per_second' ||
      billing.pricing_basis !== 'duration_seconds'
    ) {
      return base
    }

    // The catalog model price is the unadjusted scalar. The ratio already
    // incorporates the caller's membership, so apply it once.
    const baseUsd = model.model_price ?? billing.unit_price_usd
    if (
      typeof baseUsd !== 'number' ||
      !Number.isFinite(baseUsd) ||
      baseUsd < 0
    ) {
      return base
    }

    const creditsPerSecond = usdToCredits(baseUsd * ratio)
    return Number.isFinite(creditsPerSecond)
      ? { ...base, creditsPerSecond }
      : base
  })
}
