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
import { QUOTA_TYPE_VALUES, TOKEN_UNIT_DIVISORS } from '../constants'
import type {
  PricingData,
  PricingModel,
  PublicModelCardItem,
  PublicModelCategory,
  PublicPriceTier,
  TokenUnit,
} from '../types'
import {
  calculateSaving,
  quoteFixedImageExpression,
  usdToCredits,
} from './credits'
import {
  getDynamicDisplayGroupRatio,
  getDynamicPricingTiers,
  isDynamicPricingModel,
} from './dynamic-price'
import { getDisplayGroupRatio } from './model-helpers'
import {
  buildOfficialPriceMap,
  lookupOfficialBrandPrice,
  resolveOfficialUsdForTier,
} from './official-brand-prices'
import { calculateRequestPrice, calculateTokenPrice } from './price'
import { getModelModality, resolvePricingGroup } from './price-list-rows'

// ----------------------------------------------------------------------------
// Anti-Corruption Layer: backend PricingData → public card ViewModel
// ----------------------------------------------------------------------------

export type TransformPricingOptions = {
  selectedGroup?: string
  showWithRecharge?: boolean
  /** Token display unit; public cards default to /M. */
  tokenUnit?: TokenUnit
  /** Optional official USD reference prices keyed by model_name. */
  officialPriceMap?: Record<string, number>
}

type StatusSnapshot = {
  price?: number
  usd_exchange_rate?: number
  quota_per_unit?: number
}

const TOKEN_SPEC_ROWS: {
  type:
    | 'input'
    | 'output'
    | 'cache'
    | 'create_cache'
    | 'image'
    | 'audio_input'
    | 'audio_output'
  spec: string
  requires?: (model: PricingModel) => boolean
}[] = [
  { type: 'input', spec: 'Input' },
  { type: 'output', spec: 'Output' },
  {
    type: 'cache',
    spec: 'Cache',
    requires: (model) => hasFiniteRatio(model.cache_ratio),
  },
  {
    type: 'create_cache',
    spec: 'Create cache',
    requires: (model) => hasFiniteRatio(model.create_cache_ratio),
  },
  {
    type: 'image',
    spec: 'Image',
    requires: (model) => hasFiniteRatio(model.image_ratio),
  },
  {
    type: 'audio_input',
    spec: 'Audio input',
    requires: (model) => hasFiniteRatio(model.audio_ratio),
  },
  {
    type: 'audio_output',
    spec: 'Audio output',
    requires: (model) =>
      hasFiniteRatio(model.audio_ratio) &&
      hasFiniteRatio(model.audio_completion_ratio),
  },
]

/**
 * Pure ACL: map backend pricing (+ optional status rates) into public card VMs.
 * No window / storage / network side effects.
 */
export function transformPricingDataToCards(
  pricingData?: PricingData,
  statusData?: StatusSnapshot,
  options?: TransformPricingOptions
): PublicModelCardItem[] {
  if (!pricingData || !Array.isArray(pricingData.data)) {
    return []
  }

  const showWithRecharge = options?.showWithRecharge ?? false
  const tokenUnit = options?.tokenUnit ?? 'M'
  // Merge curated ledger defaults with any caller overrides.
  const officialPriceMap = buildOfficialPriceMap(options?.officialPriceMap)
  const pricingGroup = resolvePricingGroup(options?.selectedGroup)

  const priceRate = safeRate(statusData?.price, 1)
  const usdExchangeRate = safeRate(
    statusData?.usd_exchange_rate,
    Math.max(priceRate, 0.001)
  )

  const vendorMap = new Map(
    (pricingData.vendors ?? []).map((vendor) => [vendor.id, vendor])
  )

  const cards: PublicModelCardItem[] = []

  for (const raw of pricingData.data) {
    if (!raw || typeof raw.model_name !== 'string' || raw.model_name === '') {
      continue
    }

    const vendor = raw.vendor_id ? vendorMap.get(raw.vendor_id) : undefined
    let description: PricingModel['description']
    if (raw.description && raw.description !== 'undefined') {
      description = raw.description
    } else if (raw.model_name === 'jina-reader-v1') {
      description =
        'Jina Reader 网页内容提取与搜索（将任意 URL 转换为大模型友好结构化 Markdown 文本）'
    } else {
      description = raw.description
    }
    const model: PricingModel = {
      ...raw,
      key: raw.model_name,
      description,
      vendor_name: vendor?.name ?? raw.vendor_name,
      vendor_icon: vendor?.icon ?? raw.vendor_icon,
      vendor_description: vendor?.description ?? raw.vendor_description,
      group_ratio: pricingData.group_ratio ?? raw.group_ratio ?? {},
    }

    const modality = resolveCardCategory(model)
    let groupRatio = isDynamicPricingModel(model)
      ? getDynamicDisplayGroupRatio(model, pricingGroup)
      : getDisplayGroupRatio(model, pricingGroup)

    if (
      model.model_name === 'gpt-image-2.5-sunburst' &&
      model.billing_mode === 'tiered_expr'
    ) {
      const groups = model.enable_groups?.includes(pricingGroup)
        ? [pricingGroup]
        : (model.enable_groups ?? [])
      const ratios = groups
        .filter((group) => Object.hasOwn(pricingData.usable_group ?? {}, group))
        .map((group) => model.group_ratio?.[group])
      groupRatio =
        ratios.length > 0 &&
        ratios.every(
          (ratio) =>
            typeof ratio === 'number' && Number.isFinite(ratio) && ratio >= 0
        )
          ? Math.min(...(ratios as number[]))
          : Number.NaN
    }

    const tiers = buildTiersForModel(model, {
      modality,
      groupRatio,
      tokenUnit,
      showWithRecharge,
      priceRate,
      usdExchangeRate,
      modelId: model.model_name,
      officialUsd: officialPriceMap[model.model_name],
    })

    cards.push({
      id: String(model.id ?? model.model_name),
      name: model.model_name,
      modelId: model.model_name,
      provider: model.vendor_name || 'Unknown',
      vendorIcon: model.icon || model.vendor_icon,
      category: modality,
      tiers,
      tierCount: tiers.length,
      rawModel: model,
    })
  }

  return cards
}

type TierBuildContext = {
  modality: PublicModelCategory
  groupRatio: number
  tokenUnit: TokenUnit
  showWithRecharge: boolean
  priceRate: number
  usdExchangeRate: number
  modelId: string
  officialUsd?: number
}

function resolveTierOfficialUsd(
  ctx: Pick<TierBuildContext, 'modelId' | 'officialUsd'>,
  spec: string
): number | undefined {
  const fromLedger = resolveOfficialUsdForTier(ctx.modelId, spec)
  if (fromLedger !== undefined) return fromLedger
  if (
    typeof ctx.officialUsd === 'number' &&
    Number.isFinite(ctx.officialUsd) &&
    ctx.officialUsd > 0
  ) {
    return ctx.officialUsd
  }
  return undefined
}

function buildTiersForModel(
  model: PricingModel,
  ctx: TierBuildContext
): PublicPriceTier[] {
  if (
    model.model_name === 'gpt-image-2.5-sunburst' &&
    model.billing_mode === 'tiered_expr'
  ) {
    const quote = quoteFixedImageExpression(
      model.billing_expr ?? '',
      ctx.groupRatio
    )
    if (!quote) return []
    const usd = applyRechargeRate(
      quote.usd,
      ctx.showWithRecharge,
      ctx.priceRate,
      ctx.usdExchangeRate
    )
    return [
      finalizeTier({
        spec: 'Per request',
        credits: usdToCredits(usd),
        unitSuffix: requestUnitSuffix(ctx.modality, ctx.modelId),
      }),
    ]
  }
  if (isDynamicPricingModel(model)) {
    return buildDynamicTiers(model, ctx)
  }

  if (model.quota_type === QUOTA_TYPE_VALUES.REQUEST) {
    return buildRequestTiers(model, ctx)
  }

  return buildTokenTiers(model, ctx)
}

function buildRequestTiers(
  model: PricingModel,
  ctx: TierBuildContext
): PublicPriceTier[] {
  let usd = calculateRequestPrice(model, ctx.groupRatio)
  usd = applyRechargeRate(
    usd,
    ctx.showWithRecharge,
    ctx.priceRate,
    ctx.usdExchangeRate
  )

  const credits = finiteOrZero(usdToCredits(usd))
  const spec = 'Per request'
  return [
    finalizeTier({
      spec,
      credits,
      unitSuffix: requestUnitSuffix(ctx.modality, ctx.modelId),
      officialUsd: resolveTierOfficialUsd(ctx, spec),
      modelId: ctx.modelId,
    }),
  ]
}

function buildTokenTiers(
  model: PricingModel,
  ctx: TierBuildContext
): PublicPriceTier[] {
  const unitSuffix = tokenUnitSuffix(ctx.tokenUnit)
  const tiers: PublicPriceTier[] = []

  for (const row of TOKEN_SPEC_ROWS) {
    if (row.requires && !row.requires(model)) continue

    let usd = calculateTokenPrice(model, row.type, ctx.groupRatio)
    if (!Number.isFinite(usd)) continue

    usd = usd / TOKEN_UNIT_DIVISORS[ctx.tokenUnit]
    usd = applyRechargeRate(
      usd,
      ctx.showWithRecharge,
      ctx.priceRate,
      ctx.usdExchangeRate
    )

    const credits = finiteOrZero(usdToCredits(usd))
    tiers.push(
      finalizeTier({
        spec: row.spec,
        credits,
        unitSuffix,
        officialUsd: resolveTierOfficialUsd(ctx, row.spec),
        modelId: ctx.modelId,
      })
    )
  }

  if (tiers.length === 0) {
    return [
      finalizeTier({
        spec: 'Default',
        credits: 0,
        unitSuffix,
        officialUsd: resolveTierOfficialUsd(ctx, 'Default'),
        modelId: ctx.modelId,
        note: 'Price unavailable',
      }),
    ]
  }

  return tiers
}

function buildDynamicTiers(
  model: PricingModel,
  ctx: TierBuildContext
): PublicPriceTier[] {
  const parsed = getDynamicPricingTiers(model)
  const unitSuffix = tokenUnitSuffix(ctx.tokenUnit)

  if (parsed.length === 0) {
    return [
      finalizeTier({
        spec: 'Dynamic pricing',
        credits: 0,
        unitSuffix,
        officialUsd: resolveTierOfficialUsd(ctx, 'Dynamic pricing'),
        modelId: ctx.modelId,
        note: 'Expression parse failed',
      }),
    ]
  }

  const tiers: PublicPriceTier[] = []

  for (const tier of parsed) {
    if ('unitPrices' in tier && tier.unitPrices) {
      for (const [field, val] of Object.entries(
        tier.unitPrices as Record<string, number>
      )) {
        const valUsd = Number(val)
        if (Number.isFinite(valUsd) && valUsd > 0) {
          const spec = `${tier.label || 'Task'} · ${field}`
          tiers.push(
            finalizeTier({
              spec,
              credits: finiteOrZero(usdToCredits(scaleDynamicUsd(valUsd, ctx))),
              unitSuffix,
              officialUsd: resolveTierOfficialUsd(ctx, spec),
              modelId: ctx.modelId,
            })
          )
        }
      }
      continue
    }

    const fixedPrice = Number((tier as { fixedPrice?: number }).fixedPrice)
    if (Number.isFinite(fixedPrice) && fixedPrice > 0) {
      let usd = fixedPrice * ctx.groupRatio
      usd = applyRechargeRate(
        usd,
        ctx.showWithRecharge,
        ctx.priceRate,
        ctx.usdExchangeRate
      )
      const credits = finiteOrZero(usdToCredits(usd))
      const spec =
        tier.label === 'request' || tier.label === 'image'
          ? 'Per request'
          : tier.label || 'Per request'
      tiers.push(
        finalizeTier({
          spec,
          credits,
          unitSuffix: requestUnitSuffix(ctx.modality, ctx.modelId),
          officialUsd: resolveTierOfficialUsd(ctx, spec),
          modelId: ctx.modelId,
        })
      )
      continue
    }

    const tokenTier = tier as {
      inputPrice?: number
      outputPrice?: number
      label?: string
    }
    const inputUsd = Number(tokenTier.inputPrice)
    const outputUsd = Number(tokenTier.outputPrice)
    const hasInput = Number.isFinite(inputUsd) && inputUsd > 0
    const hasOutput = Number.isFinite(outputUsd) && outputUsd > 0
    const label = (tier.label || 'Tier').trim() || 'Tier'

    if (hasInput) {
      const spec = `${label} · Input`
      tiers.push(
        finalizeTier({
          spec,
          credits: finiteOrZero(usdToCredits(scaleDynamicUsd(inputUsd, ctx))),
          unitSuffix,
          officialUsd: resolveTierOfficialUsd(ctx, spec),
          modelId: ctx.modelId,
        })
      )
    }

    if (hasOutput) {
      const spec = `${label} · Output`
      tiers.push(
        finalizeTier({
          spec,
          credits: finiteOrZero(usdToCredits(scaleDynamicUsd(outputUsd, ctx))),
          unitSuffix,
          officialUsd: resolveTierOfficialUsd(ctx, spec),
          modelId: ctx.modelId,
        })
      )
    }

    // If neither primary lane has a price, still emit a placeholder for the tier label.
    if (!hasInput && !hasOutput) {
      tiers.push(
        finalizeTier({
          spec: label,
          credits: 0,
          unitSuffix,
          officialUsd: resolveTierOfficialUsd(ctx, label),
          modelId: ctx.modelId,
          note: 'Price unavailable',
        })
      )
    }
  }

  return tiers
}

function scaleDynamicUsd(
  valuePerMillionTokens: number,
  ctx: {
    groupRatio: number
    tokenUnit: TokenUnit
    showWithRecharge: boolean
    priceRate: number
    usdExchangeRate: number
  }
): number {
  const usd =
    (valuePerMillionTokens * ctx.groupRatio) /
    TOKEN_UNIT_DIVISORS[ctx.tokenUnit]
  return applyRechargeRate(
    usd,
    ctx.showWithRecharge,
    ctx.priceRate,
    ctx.usdExchangeRate
  )
}

function resolveExplicitSavingPercent(
  modelId: string | undefined
): number | undefined {
  if (!modelId) return undefined
  const entry = lookupOfficialBrandPrice(modelId)
  if (!entry) return undefined
  // Prefer ledger/hint explicit values (including 0 for Official 1:1)
  // so float-derived sell↔official diffs do not drift same-price models.
  if (
    typeof entry.savingPercent === 'number' &&
    Number.isFinite(entry.savingPercent)
  ) {
    return entry.savingPercent
  }
  return undefined
}

function finalizeTier(input: {
  spec: string
  credits: number
  unitSuffix?: string
  officialUsd?: number
  note?: string
  modelId?: string
  /** Explicit discount hint from OFFICIAL_BRAND_PRICES / caller (incl. 0). */
  savingPercent?: number
}): PublicPriceTier {
  const credits = Number.isFinite(input.credits) ? input.credits : 0
  const officialUsd =
    input.officialUsd !== undefined && Number.isFinite(input.officialUsd)
      ? Number(input.officialUsd)
      : undefined

  const officialCredits =
    officialUsd !== undefined ? usdToCredits(officialUsd) : undefined

  const explicitSaving =
    typeof input.savingPercent === 'number' &&
    Number.isFinite(input.savingPercent)
      ? input.savingPercent
      : resolveExplicitSavingPercent(input.modelId)

  // Prefer explicit ledger/hint savingPercent (incl. 0) over float derivation.
  // Keep 0 so UI can render the "Official 1:1" badge.
  const saving = calculateSaving(credits, officialUsd, explicitSaving)
  const savingPercent = saving === null ? undefined : saving

  return {
    spec: input.spec,
    credits,
    unitSuffix: input.unitSuffix,
    officialUsd,
    officialCredits:
      officialCredits !== undefined && Number.isFinite(officialCredits)
        ? officialCredits
        : undefined,
    savingPercent,
    note: input.note,
  }
}

function resolveCardCategory(model: PricingModel): PublicModelCategory {
  // parseTags() splits on whitespace, which breaks multi-word catalog tags
  // like "Image Generation". Prefer substring match on the raw tags string.
  const rawTags = (model.tags || '').toLowerCase()
  if (rawTags.includes('social-data')) return 'social'
  if (rawTags.includes('image generation')) return 'image'
  if (rawTags.includes('video generation')) return 'video'
  if (rawTags.includes('audio generation')) return 'audio'
  if (rawTags.includes('text generation')) return 'text'
  if (model.model_name.startsWith('jina-')) return 'text'

  const fromHelpers = getModelModality(model)
  if (fromHelpers !== 'other') {
    return fromHelpers
  }

  const modalities = new Set([
    ...(model.input_modalities ?? []),
    ...(model.output_modalities ?? []),
  ])
  if (modalities.has('image')) return 'image'
  if (modalities.has('video')) return 'video'
  if (modalities.has('audio')) return 'audio'
  if (modalities.has('text')) return 'text'
  return 'other'
}

function requestUnitSuffix(
  modality: PublicModelCategory,
  modelName?: string
): string {
  if (modelName && modelName.endsWith('-task')) {
    return '/task'
  }
  switch (modality) {
    case 'image':
      return '/pic'
    case 'video':
      return '/sec'
    case 'audio':
      return '/sec'
    case 'social':
      return '/gen'
    default:
      return '/gen'
  }
}

function tokenUnitSuffix(tokenUnit: TokenUnit): string {
  return tokenUnit === 'K' ? '/K' : '/M'
}

function applyRechargeRate(
  price: number,
  showWithRecharge: boolean,
  priceRate: number,
  usdExchangeRate: number
): number {
  if (!Number.isFinite(price)) return Number.NaN
  if (!showWithRecharge) return price
  return (price * priceRate) / usdExchangeRate
}

function hasFiniteRatio(value: number | null | undefined): boolean {
  return value !== undefined && value !== null && Number.isFinite(Number(value))
}

function safeRate(value: number | undefined, fallback: number): number {
  if (typeof value === 'number' && Number.isFinite(value) && value > 0) {
    return value
  }
  return Math.max(fallback, 0.001)
}

function finiteOrZero(value: number): number {
  return Number.isFinite(value) ? value : 0
}
