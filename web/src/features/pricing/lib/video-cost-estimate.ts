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
import {
  DESKTOP_PRICING_GROUPS,
  resolveSurface,
  type OmniMuxSurface,
} from '@/lib/surface'

import { EXCLUDED_GROUPS, QUOTA_TYPE_VALUES } from '../constants'
import type { PricingData, PricingModel } from '../types'
import { usdToCredits } from './credits'
import {
  getDynamicPricingSummary,
  isDynamicPricingModel,
} from './dynamic-price'
import { getModelModality } from './price-list-rows'

export type VideoEstimateRate = {
  id: string
  label: string
  creditsPerSecond: number
}

export type VideoEstimatorOption =
  | {
      id: string
      kind: 'per_second'
      creditsPerSecond: number
      isRange: boolean
    }
  | {
      id: string
      kind: 'per_video'
      creditsPerVideo: number
      isRange: boolean
    }
  | {
      id: string
      kind: 'unavailable'
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

function isVideoEstimateModel(model: PricingModel): boolean {
  if (model.contract?.family) {
    return model.contract.family === 'video'
  }
  const rawTags = (model.tags || '').toLowerCase()
  if (
    rawTags.includes('social-data') ||
    rawTags.includes('image generation') ||
    rawTags.includes('audio generation') ||
    rawTags.includes('text generation')
  ) {
    return false
  }
  if (
    rawTags.includes('video generation') ||
    getModelModality(model) === 'video'
  ) {
    return true
  }
  return (
    Array.isArray(model.output_modalities) &&
    model.output_modalities.includes('video')
  )
}

function resolveVideoModelGroupRatio(
  model: PricingModel,
  data: PricingData,
  selectedGroup: string | undefined,
  surface: OmniMuxSurface
): number | null {
  const usable = data.usable_group ?? {}
  const ratios = data.group_ratio ?? {}
  const desktopGroups = new Set<string>(DESKTOP_PRICING_GROUPS)

  const isUsableRatio = (group: string) => {
    if (EXCLUDED_GROUPS.includes(group)) return false
    if (!Object.hasOwn(usable, group)) return false
    if (surface === 'web' && desktopGroups.has(group)) return false
    const ratio = ratios[group]
    return typeof ratio === 'number' && Number.isFinite(ratio) && ratio >= 0
  }

  const enabled = Array.isArray(model.enable_groups) ? model.enable_groups : []
  const enabledAll = enabled.includes('all')

  if (
    selectedGroup &&
    isUsableRatio(selectedGroup) &&
    (enabledAll || enabled.includes(selectedGroup))
  ) {
    return ratios[selectedGroup]
  }

  const candidates = (enabledAll ? Object.keys(usable) : enabled).filter(
    isUsableRatio
  )
  if (candidates.length === 0) return null

  return Math.min(...candidates.map((group) => ratios[group]))
}

function resolveVideoOption(
  model: PricingModel,
  groupRatio: number | null
): VideoEstimatorOption {
  const id = model.model_name
  if (groupRatio === null) {
    return { id, kind: 'unavailable' }
  }

  const billing = model.contract?.billing
  const isPerVideoContract =
    billing?.unit === 'per_video' ||
    billing?.unit === 'per_request' ||
    billing?.pricing_basis === 'request' ||
    billing?.pricing_basis === 'request_count'
  const isPerUsageContract =
    billing?.unit === 'per_usage' || billing?.pricing_basis === 'reported_usage'

  if (isDynamicPricingModel(model)) {
    const summary = getDynamicPricingSummary(model, { tokenUnit: 'M' })
    const secondEntries =
      summary?.entries.filter((entry) => entry.unit === 'second') ?? []
    const secondBounds = secondEntries
      .map((entry) => ({
        min: entry.minValue ?? entry.value,
        max: entry.maxValue ?? entry.value,
      }))
      .filter(
        (bound) =>
          Number.isFinite(bound.min) &&
          Number.isFinite(bound.max) &&
          bound.min >= 0 &&
          bound.max > 0
      )
    if (secondBounds.length > 0) {
      const minUsd = Math.min(...secondBounds.map((bound) => bound.min))
      const maxUsd = Math.max(...secondBounds.map((bound) => bound.max))
      const creditsPerSecond = usdToCredits(minUsd * groupRatio)
      if (Number.isFinite(creditsPerSecond)) {
        return {
          id,
          kind: 'per_second',
          creditsPerSecond,
          isRange: maxUsd > minUsd || isPerUsageContract,
        }
      }
    }

    const requestEntries =
      summary?.entries.filter((entry) => entry.unit === 'request') ?? []
    const requestBounds = requestEntries
      .map((entry) => ({
        min: entry.minValue ?? entry.value,
        max: entry.maxValue ?? entry.value,
      }))
      .filter(
        (bound) =>
          Number.isFinite(bound.min) &&
          Number.isFinite(bound.max) &&
          bound.min >= 0 &&
          bound.max > 0
      )
    if (requestBounds.length > 0) {
      const minUsd = Math.min(...requestBounds.map((bound) => bound.min))
      const maxUsd = Math.max(...requestBounds.map((bound) => bound.max))
      const creditsPerVideo = usdToCredits(minUsd * groupRatio)
      if (Number.isFinite(creditsPerVideo)) {
        return {
          id,
          kind: 'per_video',
          creditsPerVideo,
          isRange: maxUsd > minUsd,
        }
      }
    }
  }

  let baseUsd: number | undefined
  if (
    model.quota_type === QUOTA_TYPE_VALUES.REQUEST &&
    typeof model.model_price === 'number' &&
    Number.isFinite(model.model_price) &&
    model.model_price > 0
  ) {
    baseUsd = model.model_price
  } else if (
    typeof billing?.unit_price_usd === 'number' &&
    Number.isFinite(billing.unit_price_usd) &&
    billing.unit_price_usd > 0
  ) {
    baseUsd = billing.unit_price_usd
  }

  if (baseUsd !== undefined) {
    if (isPerVideoContract) {
      const creditsPerVideo = usdToCredits(baseUsd * groupRatio)
      if (Number.isFinite(creditsPerVideo)) {
        return { id, kind: 'per_video', creditsPerVideo, isRange: false }
      }
    }
    if (
      billing?.unit === 'per_second' ||
      billing?.pricing_basis === 'duration_seconds' ||
      isPerUsageContract ||
      model.quota_type === QUOTA_TYPE_VALUES.REQUEST
    ) {
      const creditsPerSecond = usdToCredits(baseUsd * groupRatio)
      if (Number.isFinite(creditsPerSecond)) {
        return {
          id,
          kind: 'per_second',
          creditsPerSecond,
          isRange: isPerUsageContract,
        }
      }
    }
  }

  return { id, kind: 'unavailable' }
}

/**
 * Derive all live video estimator options from GET /api/pricing without a
 * hardcoded model whitelist.
 */
export function getVideoEstimatorOptions(
  data: PricingData | undefined,
  selectedGroup: string | undefined,
  surface: OmniMuxSurface = resolveSurface()
): readonly VideoEstimatorOption[] {
  if (!data?.success || !Array.isArray(data.data)) {
    return []
  }

  const seen = new Set<string>()
  const options: VideoEstimatorOption[] = []

  for (const model of data.data) {
    if (!model || typeof model.model_name !== 'string' || !model.model_name) {
      continue
    }
    if (seen.has(model.model_name) || !isVideoEstimateModel(model)) {
      continue
    }
    seen.add(model.model_name)

    const groupRatio = resolveVideoModelGroupRatio(
      model,
      data,
      selectedGroup,
      surface
    )
    options.push(resolveVideoOption(model, groupRatio))
  }

  const defaultVideoId = data.defaults?.text_to_video || 'seedance-2-5'
  options.sort((a, b) => {
    const aUnavailable = a.kind === 'unavailable' ? 1 : 0
    const bUnavailable = b.kind === 'unavailable' ? 1 : 0
    if (aUnavailable !== bUnavailable) {
      return aUnavailable - bUnavailable
    }
    const aDefault = a.id === defaultVideoId ? 0 : 1
    const bDefault = b.id === defaultVideoId ? 0 : 1
    if (aDefault !== bDefault) {
      return aDefault - bDefault
    }
    return a.id.localeCompare(b.id)
  })

  return options
}
