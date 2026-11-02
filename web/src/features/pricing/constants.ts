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
import type { TFunction } from 'i18next'

import type { TokenUnit } from './types'

// ----------------------------------------------------------------------------
// Pricing Constants
// ----------------------------------------------------------------------------

/** Sort options for pricing models */
export const SORT_OPTIONS = {
  UPDATED: 'updated',
  NAME: 'name',
  PRICE_LOW: 'price-low',
  PRICE_HIGH: 'price-high',
} as const

export type SortOption = (typeof SORT_OPTIONS)[keyof typeof SORT_OPTIONS]

export function getSortLabels(t: TFunction): Record<SortOption, string> {
  return {
    [SORT_OPTIONS.UPDATED]: t('Recently Updated'),
    [SORT_OPTIONS.NAME]: t('Name'),
    [SORT_OPTIONS.PRICE_LOW]: t('Price: Low to High'),
    [SORT_OPTIONS.PRICE_HIGH]: t('Price: High to Low'),
  }
}

/** Filter values */
export const FILTER_ALL = 'all'

/** Quota type options */
export const QUOTA_TYPES = {
  ALL: 'all',
  TOKEN: 'token',
  REQUEST: 'request',
  TASK: 'task',
} as const

export type QuotaTypeOption = (typeof QUOTA_TYPES)[keyof typeof QUOTA_TYPES]

/** Quota type labels */
export function getQuotaTypeLabels(
  t: TFunction
): Record<QuotaTypeOption, string> {
  return {
    [QUOTA_TYPES.ALL]: t('All Models'),
    [QUOTA_TYPES.TOKEN]: t('Token-based'),
    [QUOTA_TYPES.REQUEST]: t('Per Request'),
    [QUOTA_TYPES.TASK]: t('Task billing'),
  }
}

/** Endpoint type options */
export const ENDPOINT_TYPES = {
  ALL: 'all',
  OPENAI: 'openai',
  OPENAI_RESPONSE: 'openai-response',
  ANTHROPIC: 'anthropic',
  GEMINI: 'gemini',
  JINA_RERANK: 'jina-rerank',
  IMAGE_GENERATION: 'image-generation',
  EMBEDDINGS: 'embeddings',
  OPENAI_VIDEO: 'openai-video',
} as const

export type EndpointTypeOption =
  (typeof ENDPOINT_TYPES)[keyof typeof ENDPOINT_TYPES]

/** Endpoint type labels */
export function getEndpointTypeLabels(
  t: TFunction
): Record<EndpointTypeOption, string> {
  return {
    [ENDPOINT_TYPES.ALL]: t('All Types'),
    [ENDPOINT_TYPES.OPENAI]: 'Chat',
    [ENDPOINT_TYPES.OPENAI_RESPONSE]: 'Response',
    [ENDPOINT_TYPES.ANTHROPIC]: 'Anthropic',
    [ENDPOINT_TYPES.GEMINI]: 'Gemini',
    [ENDPOINT_TYPES.JINA_RERANK]: 'Rerank',
    [ENDPOINT_TYPES.IMAGE_GENERATION]: t('Image'),
    [ENDPOINT_TYPES.EMBEDDINGS]: t('Embeddings'),
    [ENDPOINT_TYPES.OPENAI_VIDEO]: t('Video'),
  }
}

/** Filter section keys */
export const FILTER_SECTIONS = {
  PRICING_TYPE: 'pricingType',
  ENDPOINT_TYPE: 'endpointType',
  VENDOR: 'vendor',
  GROUP: 'group',
  TAG: 'tag',
} as const

/** Maximum number of tags to display in model row */
export const MAX_TAGS_DISPLAY = 5

/** Maximum number of filter items to display before showing "More..." */
export const MAX_FILTER_ITEMS = 5

/** Sidebar width */
export const SIDEBAR_WIDTH = 'w-64'

/** Excluded groups */
export const EXCLUDED_GROUPS = ['', 'auto']

/** Quota type values */
export const QUOTA_TYPE_VALUES = {
  TOKEN: 0,
  REQUEST: 1,
} as const

/** Token unit divisors */
export const TOKEN_UNIT_DIVISORS = {
  M: 1,
  K: 1000,
} as const

/** Default token unit for pricing display */
export const DEFAULT_TOKEN_UNIT: TokenUnit = 'M'

/** View mode options */
export const VIEW_MODES = {
  CARD: 'card',
  TABLE: 'table',
} as const

export type ViewMode = (typeof VIEW_MODES)[keyof typeof VIEW_MODES]

/** Default page size for pricing table */
export const DEFAULT_PRICING_PAGE_SIZE = 20

/**
 * Default billing group for price-list display when the sidebar group
 * filter is "all". Matches production default group.
 */
export const DEFAULT_PRICING_GROUP = 'default'

/** Modality tabs on the public price list (APIMart-style). */
export const MODALITY_FILTERS = {
  ALL: 'all',
  TEXT: 'text',
  IMAGE: 'image',
  VIDEO: 'video',
  AUDIO: 'audio',
  SOCIAL: 'social',
} as const

export type ModalityFilter =
  (typeof MODALITY_FILTERS)[keyof typeof MODALITY_FILTERS]

/** Spec keys emitted by price-list row expansion. */
export const PRICE_LIST_SPECS = {
  INPUT: 'input',
  OUTPUT: 'output',
  CACHE: 'cache',
  CREATE_CACHE: 'create_cache',
  PER_REQUEST: 'per_request',
  DYNAMIC: 'dynamic',
  DEFAULT: 'default',
} as const

export type PriceListSpec =
  (typeof PRICE_LIST_SPECS)[keyof typeof PRICE_LIST_SPECS]

// ----------------------------------------------------------------------------
// Public pricing cache + credits conversion
// ----------------------------------------------------------------------------

/** L2 localStorage key for pricing envelope (bump suffix on schema change). */
export const PRICING_CACHE_KEY = 'omnimux.pricing.cache.v1'

/** TanStack Query staleTime / silent refetch cadence (5 minutes). */
export const PRICING_STALE_TIME = 5 * 60 * 1000

/** Max age for L2 envelope before loadPricingCache treats it as expired (24h). */
export const PRICING_CACHE_TTL = 24 * 60 * 60 * 1000

/** Hard billing display rule: 1 USD = 10 Credits. */
export const CREDITS_PER_USD = 10

/** Inverse of CREDITS_PER_USD. */
export const USD_PER_CREDIT = 0.1

/** Default raw_quota units that equal 1 USD (ops / status may override). */
export const DEFAULT_QUOTA_PER_UNIT = 500000

export type TopupRung = {
  /** USD usage value the preset sells (matches backend `amount_options`). */
  amountUsd: number
  /** Credits granted; equals amountUsd / USD_PER_CREDIT, unaffected by discount. */
  credits: number
  /** Multiplier applied to money paid; 1 means no discount. */
  discount: number
}

/**
 * Public mirror of the top-up volume ladder.
 *
 * Canonical source is `scripts/ops/payment/amount-ladder.json` (applied to
 * production by `scripts/ops/apply-payment-amount-ladder.js`). A regression
 * test reads that JSON and fails if this list drifts from it, so the public
 * page cannot advertise a discount the backend will not honour.
 *
 * Discounts only match exact preset amounts server-side; custom amounts are
 * charged at full price.
 */
export const TOPUP_LADDER: readonly TopupRung[] = [
  { amountUsd: 10, credits: 100, discount: 1 },
  { amountUsd: 50, credits: 500, discount: 1 },
  { amountUsd: 100, credits: 1000, discount: 1 },
  { amountUsd: 300, credits: 3000, discount: 0.95 },
  { amountUsd: 1000, credits: 10000, discount: 0.9 },
  { amountUsd: 3000, credits: 30000, discount: 0.85 },
]
