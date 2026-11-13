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
  /**
   * Human-readable model name for the page heading. The pricing API does not
   * send it yet, so the heading falls back to `model_name`.
   */
  display_name?: string
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
  /** Shape of the published contract. The gateway publishes "2.0". */
  schemaVersion?: string
  /** Deterministic content hash of one contract, excluding this field. */
  contractVersion?: string
  /** Whether this model has a verified executable gateway path. */
  execution_status?: 'live' | 'stub' | 'unavailable'
  /** Why a non-live model is not executable; required for stub/unavailable. */
  execution_reason?: string
  family?: ModelFamily
  status?: string
  /**
   * Compatibility view of the v1 payload, where operations were bare ids. A
   * v2 payload carries `ContractOperation[]` instead; a client must accept
   * both because a cached or older gateway response can still surface here.
   */
  operations?: string[] | ContractOperation[]
  /** The same operation ids as strings, for clients that cannot read objects. */
  operation_ids?: string[]
  /**
   * Field paths whose value the governance spec did not establish. Published as
   * data so a client can tell "not stated" apart from "no such axis".
   */
  unknowns?: string[]
  input_modalities?: string[]
  tool_calling?: boolean
  streaming?: boolean
  prompt_caching?: boolean
  audio_kind?: string
  input_formats?: string[]
  max_input_bytes?: number
  max_input_seconds?: number
  max_output_seconds?: number
  voices?: number
  languages?: string[]
  endpoints?: ContractEndpoint[]
  auth?: string
  rate_limit?: unknown
  pagination?: string
  response_fields?: string[]
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

/** One callable route a data-family contract publishes. */
export type ContractEndpoint = {
  name?: string
  method?: string
  path?: string
}

/** How a parameter bound was established by the governance spec. */
export type ContractLimitSource = {
  kind: 'official_docs' | 'measured' | 'policy_conservative' | string
  note?: string
}

/** One input slot an executable operation consumes. */
export type ContractInput = {
  slot: string
  type: 'text' | 'image' | 'video' | 'audio' | 'file' | 'url' | string
  role?: string
  required: boolean
  min?: number
  max?: number
  allowedMimes?: string[]
  maxSizeMb?: number
  minDurationSec?: number
  maxDurationSec?: number
}

/** One request parameter an executable operation accepts. */
export type ContractParameter = {
  name: string
  type: 'enum' | 'integer' | 'number' | 'boolean' | 'string'
  enum?: Array<string | number | boolean>
  minimum?: number
  maximum?: number
  step?: number
  default?: string | number | boolean
  fixed?: string | number | boolean
  limitSource?: ContractLimitSource
}

/** The media shape one operation produces. */
export type ContractOperationOutput = {
  type: 'text' | 'image' | 'video' | 'audio'
  formats?: string[]
  sizes?: string[]
}

/** One mode a model can actually execute through the gateway. */
export type ContractOperation = {
  id: string
  executable: boolean
  inputs: ContractInput[]
  parameters: ContractParameter[]
  output: ContractOperationOutput
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
