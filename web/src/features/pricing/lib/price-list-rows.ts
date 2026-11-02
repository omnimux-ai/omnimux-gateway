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
  DEFAULT_PRICING_GROUP,
  FILTER_ALL,
  MODALITY_FILTERS,
  PRICE_LIST_SPECS,
  QUOTA_TYPE_VALUES,
  type ModalityFilter,
  type PriceListSpec,
} from '../constants'
import type { PricingModel, TokenUnit } from '../types'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingSummary,
  isDynamicPricingModel,
} from './dynamic-price'
import { parseTags } from './filters'
import { formatPrice, formatRequestPrice, stripTrailingZeros } from './price'

export type PriceListModality =
  | 'text'
  | 'image'
  | 'video'
  | 'audio'
  | 'social'
  | 'other'

export type PriceListBillingKind = 'token' | 'request' | 'dynamic'

export type PriceListRow = {
  rowKey: string
  model: PricingModel
  modelName: string
  vendorName?: string
  vendorIcon?: string
  modality: PriceListModality
  spec: PriceListSpec
  /** English i18n source key for Spec column */
  specLabelKey: string
  ourPriceLabel: string
  unitHintKey: string
  billingKind: PriceListBillingKind
  /** First row for this model in the expanded list (UI can hide repeated names). */
  isFirstForModel: boolean
}

export type BuildPriceListRowsOptions = {
  tokenUnit?: TokenUnit
  showRechargePrice?: boolean
  priceRate?: number
  usdExchangeRate?: number
  /**
   * Active group filter from the sidebar. When "all", prices use
   * DEFAULT_PRICING_GROUP so the list is stable and matches ops default.
   */
  selectedGroup?: string
}

function hasFiniteRatio(value: number | null | undefined): boolean {
  return value !== undefined && value !== null && Number.isFinite(Number(value))
}

/**
 * Classify a model into a public price-list modality tab.
 * Order matters: social-data wins over Text Generation.
 */
export function getModelModality(model: PricingModel): PriceListModality {
  const tags = parseTags(model.tags).map((tag) => tag.toLowerCase())
  // Prefer operational / modality type tags (exact or substring for social-data).
  if (tags.some((tag) => tag === 'social-data' || tag.includes('social-data'))) {
    return 'social'
  }
  if (tags.some((tag) => tag === 'image generation')) {
    return 'image'
  }
  if (tags.some((tag) => tag === 'video generation')) {
    return 'video'
  }
  if (tags.some((tag) => tag === 'audio generation')) {
    return 'audio'
  }
  if (tags.some((tag) => tag === 'text generation')) {
    return 'text'
  }
  return 'other'
}

export function matchesModalityFilter(
  modality: PriceListModality,
  filter: ModalityFilter
): boolean {
  if (filter === MODALITY_FILTERS.ALL) return true
  return modality === filter
}

export function resolvePricingGroup(selectedGroup?: string): string {
  if (
    selectedGroup &&
    selectedGroup !== FILTER_ALL &&
    selectedGroup.trim() !== ''
  ) {
    return selectedGroup
  }
  return DEFAULT_PRICING_GROUP
}

function tokenUnitHint(tokenUnit: TokenUnit): string {
  return tokenUnit === 'K' ? '/ 1K tokens' : '/ 1M tokens'
}

function pushRow(
  rows: PriceListRow[],
  partial: Omit<PriceListRow, 'isFirstForModel' | 'rowKey'> & {
    rowKey?: string
  },
  firstModelName: { current: string | null }
) {
  const modelName = partial.modelName
  const isFirstForModel = firstModelName.current !== modelName
  firstModelName.current = modelName
  rows.push({
    ...partial,
    rowKey: partial.rowKey ?? `${modelName}:${partial.spec}`,
    isFirstForModel,
  })
}

/**
 * Expand filtered models into Spec rows for the APIMart-style price table.
 * All money strings go through existing formatPrice / formatRequestPrice.
 */
export function buildPriceListRows(
  models: PricingModel[],
  options: BuildPriceListRowsOptions = {}
): PriceListRow[] {
  const tokenUnit = options.tokenUnit ?? 'M'
  const showRechargePrice = options.showRechargePrice ?? false
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1
  const pricingGroup = resolvePricingGroup(options.selectedGroup)

  const rows: PriceListRow[] = []
  const firstModelName = { current: null as string | null }

  for (const model of models) {
    const modality = getModelModality(model)
    const base = {
      model,
      modelName: model.model_name,
      vendorName: model.vendor_name,
      vendorIcon: model.icon || model.vendor_icon,
      modality,
    }

    if (isDynamicPricingModel(model)) {
      const summary = getDynamicPricingSummary(model, {
        tokenUnit,
        showRechargePrice,
        priceRate,
        usdExchangeRate,
        groupRatioMultiplier: getDynamicDisplayGroupRatio(model, pricingGroup),
      })
      const label =
        summary && !summary.isSpecialExpression && summary.primaryEntries.length
          ? summary.primaryEntries
              .map((entry) => stripTrailingZeros(entry.formatted))
              .join(' / ')
          : ''
      pushRow(
        rows,
        {
          ...base,
          spec: PRICE_LIST_SPECS.DYNAMIC,
          specLabelKey: 'Dynamic',
          ourPriceLabel: label || '—',
          unitHintKey: 'Dynamic pricing',
          billingKind: 'dynamic',
        },
        firstModelName
      )
      continue
    }

    if (model.quota_type === QUOTA_TYPE_VALUES.REQUEST) {
      pushRow(
        rows,
        {
          ...base,
          spec: PRICE_LIST_SPECS.PER_REQUEST,
          specLabelKey: 'Per request',
          ourPriceLabel: stripTrailingZeros(
            formatRequestPrice(
              model,
              showRechargePrice,
              priceRate,
              usdExchangeRate,
              pricingGroup
            )
          ),
          unitHintKey: 'request',
          billingKind: 'request',
        },
        firstModelName
      )
      continue
    }

    // Token-based
    const tokenSpecs: {
      spec: PriceListSpec
      type: 'input' | 'output' | 'cache' | 'create_cache'
      labelKey: string
    }[] = [
      {
        spec: PRICE_LIST_SPECS.INPUT,
        type: 'input',
        labelKey: 'Input',
      },
      {
        spec: PRICE_LIST_SPECS.OUTPUT,
        type: 'output',
        labelKey: 'Output',
      },
    ]

    if (hasFiniteRatio(model.cache_ratio)) {
      tokenSpecs.push({
        spec: PRICE_LIST_SPECS.CACHE,
        type: 'cache',
        labelKey: 'Cache',
      })
    }
    if (hasFiniteRatio(model.create_cache_ratio)) {
      tokenSpecs.push({
        spec: PRICE_LIST_SPECS.CREATE_CACHE,
        type: 'create_cache',
        labelKey: 'Create cache',
      })
    }

    for (const tokenSpec of tokenSpecs) {
      pushRow(
        rows,
        {
          ...base,
          spec: tokenSpec.spec,
          specLabelKey: tokenSpec.labelKey,
          ourPriceLabel: stripTrailingZeros(
            formatPrice(
              model,
              tokenSpec.type,
              tokenUnit,
              showRechargePrice,
              priceRate,
              usdExchangeRate,
              pricingGroup
            )
          ),
          unitHintKey: tokenUnitHint(tokenUnit),
          billingKind: 'token',
        },
        firstModelName
      )
    }
  }

  return rows
}

export function filterPriceListRowsByModality(
  rows: PriceListRow[],
  modality: ModalityFilter
): PriceListRow[] {
  if (modality === MODALITY_FILTERS.ALL) return rows
  return rows.filter((row) => row.modality === modality)
}
