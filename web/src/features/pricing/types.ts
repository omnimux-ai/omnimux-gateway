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
// ----------------------------------------------------------------------------
// Pricing Types
// ----------------------------------------------------------------------------

export type PricingVendor = {
  id: number
  name: string
  icon?: string
  description?: string
}

export type BillingUsageUnit = 'second' | 'count' | 'token' | 'credit'

export type BillingUsageFieldSchema = {
  type?: 'number' | 'boolean'
  unit?: BillingUsageUnit
  unitLabel?: string | Record<string, string>
  enum?: string[]
  enumLabels?: Record<string, string | Record<string, string>>
  description?: string | Record<string, string>
}

export type BillingUsageSchema = Record<string, BillingUsageFieldSchema>

export type BillingUsageExample = {
  label: string
  facts: Record<string, string | number>
}

export type BillingPluginVariant = {
  plugin_key: string
  plugin_name: string
  icon?: string
  billing_expr: string
  billing_mode?: 'ratio' | 'tiered_expr'
  billing_usage_schema: BillingUsageSchema
  billing_usage_examples?: BillingUsageExample[]
}

export type PricingModel = {
  billing_plugin_variants?: BillingPluginVariant[]
  id: number
  model_name: string
  description?: string
  icon?: string
  vendor_id?: number
  vendor_name?: string
  vendor_icon?: string
  vendor_description?: string
  quota_type: number
  model_ratio: number
  completion_ratio: number
  model_price?: number
  cache_ratio?: number | null
  create_cache_ratio?: number | null
  image_ratio?: number | null
  audio_ratio?: number | null
  audio_completion_ratio?: number | null
  enable_groups: string[]
  tags?: string
  supported_endpoint_types?: string[]
  created_time?: number
  updated_time?: number
  key?: string
  group_ratio?: Record<string, number>
  /** Billing mode (e.g. "tiered_expr") used to flag dynamic pricing */
  billing_mode?: string
  /** Raw expression describing dynamic / tiered billing */
  billing_expr?: string
  /** Task-plugin usage facts and their billing units. */
  billing_usage_schema?: BillingUsageSchema
  /** Display-only labeled usage vectors for pricing examples. */
  billing_usage_examples?: BillingUsageExample[]
  /** Pricing version returned by backend, useful for cache busting */
  pricing_version?: string
  /**
   * Optional model metadata fields reserved for backend-provided catalog data.
   * Keep them data-driven; do not synthesize display values on the client.
   */
  context_length?: number
  max_output_tokens?: number
  knowledge_cutoff?: string
  release_date?: string
  parameter_count?: string
  input_modalities?: Modality[]
  output_modalities?: Modality[]
  capabilities?: ModelCapability[]
  /**
   * The model's declared capability contract, projected from the gateway's
   * governance specs. Absent for a model that has no spec, which means "not
   * established" — never "no such axis".
   */
  contract?: ModelContract
}

/**
 * Per-model capability envelope published by the catalog.
 *
 * Every field is optional because the gateway omits an axis the governance spec
 * does not declare. A missing axis must stay missing: filling it in here would
 * recreate the invented contract this data exists to replace.
 */
export type ModelContract = {
  model_id: string
  family?: ModelFamily
  status?: string
  operations?: string[]
  input_modalities?: string[]
  durations_seconds?: {
    min?: number
    max?: number
    default?: number
    allowed?: number[]
  }
  resolutions?: string[]
  aspect_ratios?: string[]
  audio_supported?: boolean
  output_formats?: string[]
  output_sizes?: string[]
  reference_images?: ModelMediaLimits
  reference_videos?: ModelMediaLimits
  reference_audios?: ModelMediaLimits
  prompt_max_chars?: number
  context_window_tokens?: number
  max_output_tokens?: number
}

/** Ceiling and accepted formats for one reference family. */
export type ModelMediaLimits = {
  min?: number
  max?: number
  formats?: string[]
  max_bytes_each?: number
  max_seconds_each?: number
}

/** The capability family a governance spec assigns to a model. */
export type ModelFamily = 'video' | 'image' | 'llm' | 'data' | 'audio'

/** Input/output modalities supported by a model. */
export type Modality = 'text' | 'image' | 'audio' | 'video' | 'file'

/** Functional capabilities a model exposes. */
export type ModelCapability =
  | 'function_calling'
  | 'streaming'
  | 'vision'
  | 'json_mode'
  | 'structured_output'
  | 'reasoning'
  | 'tools'
  | 'system_prompt'
  | 'web_search'
  | 'code_interpreter'
  | 'caching'
  | 'embeddings'

export type PricingData = {
  success: boolean
  message?: string
  data: PricingModel[]
  vendors: PricingVendor[]
  group_ratio: Record<string, number>
  usable_group: Record<string, string>
  supported_endpoint: Record<string, string>
  auto_groups: string[]
  /** Top-level pricing hash from GET /api/pricing; preferred for L2 cache bust */
  pricing_version?: string
}

/** Public toolbar / card modality (wider than backend Modality). */
export type PublicModelCategory =
  | 'text'
  | 'image'
  | 'video'
  | 'audio'
  | 'social'
  | 'other'

/** Public card tier row (OUR / OFFICIAL / SAVING view model). */
export interface PublicPriceTier {
  spec: string
  credits: number
  unitSuffix?: string
  officialUsd?: number
  officialCredits?: number
  savingPercent?: number
  note?: string
}

/** Public model card view model for the conversion-oriented pricing grid. */
export interface PublicModelCardItem {
  id: string
  name: string
  modelId: string
  provider: string
  vendorIcon?: string
  category: PublicModelCategory
  tiers: PublicPriceTier[]
  tierCount: number
  rawModel: PricingModel
}

/** L2 localStorage envelope for pricing + optional status snapshot. */
export interface PricingCacheEnvelope {
  version: string
  timestamp: number
  pricingData: PricingData
  statusData?: {
    price?: number
    usd_exchange_rate?: number
    quota_per_unit?: number
  }
}

export type TokenUnit = 'M' | 'K'
export type PriceType =
  | 'input'
  | 'output'
  | 'cache'
  | 'create_cache'
  | 'image'
  | 'audio_input'
  | 'audio_output'
export type QuotaType = 0 | 1 // 0: token-based, 1: per-request
