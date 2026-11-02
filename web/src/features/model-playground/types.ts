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

/**
 * Model page playground contracts.
 *
 * The model page (/{pricing}/$modelId) hosts a Playground section that submits
 * generation tasks. Submission is expressed as a transport so the frontend can
 * run without any backend change: MockTransport simulates task lifecycles,
 * GatewayTransport talks to the relay API. See
 * docs/product/model-page-implementation-plan-zh.md.
 */
import type { PricingModel } from '@/features/pricing/types'

/** Modalities the playground can drive. */
export type PlaygroundModality = 'image' | 'video'

/**
 * Generation modes one public model ID can drive.
 *
 * Values are the gateway's own discriminators, not vendor route names: the
 * request carries `operation`, and the apimart contract validates it against a
 * per-model whitelist. The authoritative table is `operationAllowed` in
 * `relay/channel/task/apimart/video_contract.go`; `lib/operation-matrix.ts`
 * mirrors it for the form.
 */
export type PlaygroundOperation =
  | 'text_to_video'
  | 'first_frame'
  | 'first_last_frame'
  | 'video_multi_ref'
  | 'video_edit'
  | 'video_extend'

/** Role a frame image plays in the vendor's `image_with_roles` pair. */
export type MediaRole = 'first_frame' | 'last_frame'

/** One entry of the vendor's `image_with_roles` frame pair. */
export type ImageWithRole = {
  url: string
  role: MediaRole
}

/** How a parameter is rendered. Mirrors the benchmark page's control vocabulary. */
export type ParamControlKind =
  | 'chips'
  | 'segmented'
  | 'slider'
  | 'switch'
  | 'text'
  | 'media'

export type ParamOption = {
  value: string
  /** i18n key; falls back to `value` when absent. */
  labelKey?: string
}

export type ParamControl = {
  key: string
  /** i18n key for the field label. */
  labelKey: string
  kind: ParamControlKind
  /**
   * The control carries a number even though its options are strings. Option
   * values stay strings so they can double as i18n keys; `submitParams` coerces
   * this key on the way out, because the gateway validates its type.
   */
  numeric?: boolean
  /**
   * Wire value for a UI value, when the gateway names the same choice
   * differently. The form keeps showing the UI value; only the body is adapted.
   */
  valueAliases?: Record<string, string>
  options?: ParamOption[]
  min?: number
  max?: number
  /**
   * Ceiling a text control declares, taken from the model's contract. Absent
   * means the model established none, so the input stays uncapped rather than
   * gaining a limit nothing stated.
   */
  maxChars?: number
  step?: number
  defaultValue: string | number | boolean | string[]
  /** Rendered inside the collapsed "advanced settings" group. */
  advanced?: boolean
  /** Shows alongside the control when the model documents one. */
  hintKey?: string
  /**
   * Media controls only: the role this slot's uploads play on the wire, and the
   * accept list / ceiling the model documents. `maxCount: 1` renders a single
   * fixed slot (first frame, source video) instead of a growing list.
   */
  mediaRole?:
    | MediaRole
    | 'reference_image'
    | 'reference_video'
    | 'reference_audio'
  accept?: string
  maxCount?: number
  /** Per-file byte ceiling the model documents, refused before any upload. */
  maxBytes?: number
  /**
   * Set when the mode forces this control to one value. The form renders it
   * disabled and explains why, instead of offering a choice the gateway (or the
   * upstream) would reject.
   */
  lockedReasonKey?: string
}

export type PlaygroundParamSchema = {
  modelId: string
  modality: PlaygroundModality
  /** Capability chips shown above the form. */
  capabilities: string[]
  controls: ParamControl[]
  advancedCount: number
  /**
   * Present only for models whose modes are declared. Its absence is what keeps
   * every other model on the flat, modality-derived form.
   */
  operation?: PlaygroundOperation
}

/**
 * Form state and, once shaped by `toSubmitBody`, the request body.
 *
 * The two share a type because the body is the parameters with media moved into
 * the gateway's field names; `image_with_roles` is the one entry that is not a
 * bare string.
 */
export type PlaygroundParams = Record<
  string,
  string | number | boolean | string[] | ImageWithRole[]
>

export type TaskStatus =
  | 'queued'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'timeout'

export type TaskMedia = {
  url: string
  kind: 'image' | 'video'
}

export type GenerationTask = {
  taskId: string
  modelId: string
  status: TaskStatus
  /** 0-100 */
  progress: number
  params: PlaygroundParams
  /** Estimated charge in USD, null when the model cannot be estimated. */
  estimatedUsd: number | null
  settledUsd: number | null
  createdAt: number
  finishedAt?: number
  failReason?: string
  media: TaskMedia[]
}

export type SubmitRequest = {
  modelId: string
  params: PlaygroundParams
  modality?: PlaygroundModality
  /**
   * Token group the request must be pinned to, sent as `X-Omnimux-Group`.
   *
   * Absent means the caller expresses no group: the gateway then keeps its own
   * selection. A model whose quality tiers are token groups is the case this
   * exists for — the tier is the group, not a second model id.
   */
  group?: string
}

export type UploadResult = {
  url: string
}

/**
 * Single seam between UI and the execution gateway. Implementations must not
 * leak transport-specific fields into components.
 */
export interface GenerationTransport {
  /** True when tasks are simulated locally (no gateway call). */
  readonly mocked: boolean
  uploadReference(file: File): Promise<UploadResult>
  submit(request: SubmitRequest): Promise<{ taskId: string }>
  getTask(taskId: string): Promise<GenerationTask>
  listHistory(modelId: string): Promise<GenerationTask[]>
}

/** Narrow view-model the page needs from the pricing catalog. */
export type PlaygroundModel = Pick<
  PricingModel,
  | 'model_name'
  | 'description'
  | 'vendor_name'
  | 'quota_type'
  | 'model_price'
  | 'billing_expr'
  | 'billing_mode'
  | 'tags'
  | 'input_modalities'
  | 'output_modalities'
  | 'billing_usage_schema'
  | 'billing_usage_examples'
>
