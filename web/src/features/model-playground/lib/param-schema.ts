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
import type {
  BillingUsageSchema,
  ModelContract,
  PricingModel,
} from '@/features/pricing/types'

import type {
  ParamControl,
  ParamOption,
  PlaygroundModality,
  PlaygroundOperation,
  PlaygroundParamSchema,
  PlaygroundParams,
} from '../types'
import {
  billingDeclarationFor,
  isPerCall,
  type DurationEnvelope,
} from './billing-dimension'
import {
  lockedValue,
  mediaAccept,
  mediaCap,
  operationLocks,
  operationMedia,
} from './operation-matrix'
import { tierResolutions } from './tier-group'

/**
 * Capability chips shown above the playground form.
 *
 * Values are i18n keys. A single public model ID can expose several input
 * shapes (text / first frame / references), so chips describe capabilities and
 * prefill a parameter combination — they never expose derivative model IDs.
 */
export const CAPABILITY_KEYS = {
  textToImage: 'Text to image',
  imageToImage: 'Image to image',
  maskEdit: 'Mask edit',
  textToVideo: 'Text to video',
  imageToVideo: 'Image to video',
  referenceToVideo: 'Reference to video',
  videoEdit: 'Video edit',
  videoAudio: 'Video + audio',
} as const

/**
 * Models whose tier limits are not expressible from the catalog payload yet.
 *
 * This is the minimal D-3(a) declaration: only the few public IDs the page
 * ships with, each overriding a single axis. Deliberately NOT a per-model
 * parameter matrix — the schema/modality derivation below owns everything it
 * can express on its own.
 */
type ParamOverride = {
  minDuration?: number
  maxDuration?: number
  ratios?: string[]
  resolutions?: string[]
  qualities?: string[]
}

const PARAM_OVERRIDES: Record<string, ParamOverride> = {
  'seedance-2-5': {
    minDuration: 4,
    maxDuration: 30,
    ratios: ['auto', '16:9', '9:16', '1:1', '4:3', '3:4', '21:9'],
    resolutions: ['480p', '720p', '1080p'],
  },
  'seedance-2-0': {
    minDuration: 4,
    maxDuration: 15,
    ratios: ['auto', '16:9', '9:16', '1:1'],
    resolutions: ['480p', '720p', '1080p'],
  },
  'kling-v3': {
    minDuration: 5,
    maxDuration: 10,
    ratios: ['16:9', '9:16', '1:1'],
    resolutions: ['720p', '1080p'],
  },
  'minimax-h3': {
    minDuration: 4,
    maxDuration: 15,
    ratios: ['16:9', '9:16', '1:1', '4:3', '3:4', '21:9'],
    resolutions: ['768P', '2K'],
  },
  'minimax-h3-task': {
    minDuration: 4,
    maxDuration: 15,
    ratios: ['16:9', '9:16', '1:1', '4:3', '3:4', '21:9'],
    resolutions: ['768P', '2K'],
  },
  'gpt-image-2.5': {
    ratios: ['auto', '1:1', '3:2', '2:3', '16:9', '9:16', '4:3', '3:4'],
    resolutions: ['1K', '2K', '4K'],
    qualities: ['low', 'medium', 'high'],
  },
}

const IMAGE_RATIOS = ['1:1', '3:2', '2:3', '16:9', '9:16', '4:3', '3:4']
const IMAGE_RESOLUTIONS = ['1K', '2K', '4K']
const IMAGE_QUALITIES = ['low', 'medium', 'high']
const IMAGE_FORMATS = ['png', 'jpeg', 'webp']
const VIDEO_RATIOS = ['auto', '16:9', '9:16', '1:1']
const VIDEO_RESOLUTIONS = ['480p', '720p', '1080p']
const VIDEO_FORMATS = ['mp4', 'mov']

/** Field name candidates, in priority order, for each derived control. */
const SCHEMA_FIELDS = {
  ratio: ['aspect_ratio', 'ratio'],
  resolution: ['resolution', 'quality_tier', 'image_size'],
  quality: ['quality'],
  duration: ['duration'],
  background: ['background'],
  format: ['output_format', 'format'],
  count: ['n', 'count'],
  audio: ['generate_audio', 'audio'],
  sizeMode: ['size'],
} as const

function parseTags(model: PricingModel): string[] {
  return (model.tags ?? '')
    .split(',')
    .map((tag) => tag.trim())
    .filter(Boolean)
}

/** Output modality decides the playground shape; tags are the documented fallback. */
export function resolveModality(model: PricingModel): PlaygroundModality {
  const outputs = model.output_modalities ?? []
  if (outputs.includes('video')) return 'video'
  if (outputs.includes('image')) return 'image'

  const tags = parseTags(model)
  if (tags.includes('Video Generation')) return 'video'
  if (tags.includes('Image Generation')) return 'image'
  return 'image'
}

/**
 * Model accepts reference material when any input modality is not plain text.
 *
 * Image-generation models are included on their own merits: the gateway gives
 * them an images edit surface, so the reference slot is meaningful even when the
 * catalog declares only `tags: "Image Generation"`.
 */
function acceptsReferenceMedia(model: PricingModel): boolean {
  const inputs = model.input_modalities
  if (inputs?.length) {
    return inputs.some((modality) => modality !== 'text')
  }
  const tags = parseTags(model)
  return (
    tags.includes('async') ||
    tags.includes('task') ||
    tags.includes('Image Generation')
  )
}

/** Formats the image edit surface takes; a model not listed falls back to this. */
const DEFAULT_REFERENCE_IMAGE_ACCEPT = 'image/png,image/jpeg,image/webp'

/**
 * Reference-image limits, per model, from each vendor's own documentation.
 *
 * - `gpt-image-2.5` — OpenAI images edit: up to **16** images, prompt up to
 *   32000 characters. OpenAI publishes **no per-reference-file byte cap**: the
 *   only "50MB" figure in its image guide sits under "Mask requirements" and
 *   covers the image being edited together with its mask, and the API reference
 *   that defines the `image` array mentions no size at all. No cap is asserted
 *   here rather than shipping a number the vendor never stated.
 * - `nano-banana-pro` / `nano-banana-2` — Google Gemini 3 image models: up to
 *   **14** reference images.
 * - `seedream-5-0-pro` — Volcengine Doubao Seedream 5.0 Pro: multi-image to a
 *   single image takes **2–10** references. (The 2–14 figure on the same page
 *   belongs to Seedream 5.0 lite / 4.5 / 4.0, and our model explicitly does not
 *   support grouped generation.)
 * - `qwen-image-3-0` — Alibaba Model Studio image edit: **1–3** images, each
 *   **no larger than 10MB**.
 * - `grok-imagine-image-*` — xAI Imagine: up to **5** reference images.
 *
 * These are vendor ceilings. A reseller channel may accept fewer, so a request
 * that passes here can still be refused upstream; the limit exists to stop the
 * browser from uploading files no documented upstream would take. A model with
 * no documented byte ceiling therefore carries no `maxBytes`.
 */
type ReferenceImageLimit = {
  accept: string
  maxCount: number
  /** Per-file ceiling in bytes, only where the vendor documents one. */
  maxBytes?: number
}

const MEGABYTE = 1024 * 1024

const REFERENCE_IMAGE_LIMITS: Record<string, ReferenceImageLimit> = {
  'gpt-image-2.5': {
    accept: DEFAULT_REFERENCE_IMAGE_ACCEPT,
    maxCount: 16,
  },
  'nano-banana-pro': {
    accept: DEFAULT_REFERENCE_IMAGE_ACCEPT,
    maxCount: 14,
  },
  'nano-banana-2': {
    accept: DEFAULT_REFERENCE_IMAGE_ACCEPT,
    maxCount: 14,
  },
  'seedream-5-0-pro': {
    accept: DEFAULT_REFERENCE_IMAGE_ACCEPT,
    maxCount: 10,
  },
  'qwen-image-3-0': {
    accept: DEFAULT_REFERENCE_IMAGE_ACCEPT,
    maxCount: 3,
    maxBytes: 10 * MEGABYTE,
  },
}

/** What an undocumented model falls back to: one image, the common formats. */
const FALLBACK_REFERENCE_IMAGE_LIMIT: ReferenceImageLimit = {
  accept: DEFAULT_REFERENCE_IMAGE_ACCEPT,
  maxCount: 1,
}

/** The reference slot's own limits, resolved per model. */
function referenceMediaLimits(model: PricingModel): ReferenceImageLimit {
  return (
    REFERENCE_IMAGE_LIMITS[model.model_name] ?? FALLBACK_REFERENCE_IMAGE_LIMIT
  )
}

/** Enum values declared by the catalog win over the modality defaults. */
function enumOptions(
  schema: BillingUsageSchema | undefined,
  fields: readonly string[]
): ParamOption[] | undefined {
  if (!schema) return undefined
  for (const field of fields) {
    const entry = schema[field]
    if (!entry?.enum?.length) continue
    return entry.enum.map((value) => {
      const label = entry.enumLabels?.[value]
      return typeof label === 'string' ? { value, labelKey: label } : { value }
    })
  }
  return undefined
}

function optionValues(options: ParamOption[]): string[] {
  return options.map((option) => option.value)
}

function chips(
  key: string,
  labelKey: string,
  values: string[],
  defaultValue: string,
  schemaOptions?: ParamOption[]
): ParamControl {
  const options =
    schemaOptions && schemaOptions.length > 0
      ? schemaOptions
      : values.map((value) => ({ value }))
  const values_ = optionValues(options)
  return {
    key,
    labelKey,
    kind: 'chips',
    options,
    defaultValue: values_.includes(defaultValue) ? defaultValue : values_[0],
  }
}

function segmented(
  key: string,
  labelKey: string,
  values: string[],
  defaultValue: string,
  schemaOptions?: ParamOption[]
): ParamControl {
  const options =
    schemaOptions && schemaOptions.length > 0
      ? schemaOptions
      : values.map((value) => ({ value }))
  const values_ = optionValues(options)
  return {
    key,
    labelKey,
    kind: 'segmented',
    options,
    defaultValue: values_.includes(defaultValue) ? defaultValue : values_[0],
  }
}

function buildDurationOptions(
  min: number,
  max: number,
  override?: ParamOverride
): ParamOption[] {
  const lower = override?.minDuration ?? min
  const upper = override?.maxDuration ?? max
  if (!Number.isFinite(lower) || !Number.isFinite(upper) || upper < lower) {
    return [
      { value: '4' },
      { value: '5' },
      { value: '8' },
      { value: '10' },
      { value: '12' },
    ]
  }
  const candidates = new Set<number>([lower, upper])
  for (const step of [4, 5, 6, 8, 10, 12, 15]) {
    if (step >= lower && step <= upper) candidates.add(step)
  }
  return [...candidates]
    .sort((a, b) => a - b)
    .map((value) => ({ value: String(value) }))
}

function buildImageControls(
  schema: BillingUsageSchema | undefined,
  override: ParamOverride | undefined,
  contract?: ModelContract
): ParamControl[] {
  const controls: ParamControl[] = []
  const ratioOptions = enumOptions(schema, SCHEMA_FIELDS.ratio)
  const resolutionOptions = enumOptions(schema, SCHEMA_FIELDS.resolution)
  const qualityOptions = enumOptions(schema, SCHEMA_FIELDS.quality)
  const backgroundOptions = enumOptions(schema, SCHEMA_FIELDS.background)
  const formatOptions = enumOptions(schema, SCHEMA_FIELDS.format)

  controls.push({
    key: 'size_mode',
    labelKey: 'Size mode',
    kind: 'segmented',
    options: [{ value: 'auto' }, { value: 'ratio' }, { value: 'custom' }],
    defaultValue: 'auto',
  })
  controls.push(
    chips(
      'aspect_ratio',
      'Aspect ratio',
      override?.ratios ?? IMAGE_RATIOS,
      '1:1',
      ratioOptions
    )
  )
  controls.push(
    chips(
      'resolution',
      'Resolution',
      // An image model states its output sizes; a video model states
      // resolutions. Either outranks the modality default.
      declaredList(contract?.output_sizes) ??
        declaredList(contract?.resolutions) ??
        override?.resolutions ??
        IMAGE_RESOLUTIONS,
      '1K',
      resolutionOptions
    )
  )
  controls.push(
    segmented(
      'quality',
      'Quality',
      override?.qualities ?? IMAGE_QUALITIES,
      'medium',
      qualityOptions
    )
  )
  controls.push(
    segmented(
      'background',
      'Background',
      ['opaque', 'transparent'],
      'opaque',
      backgroundOptions
    )
  )
  // The image path read the contract too: an image model that publishes its
  // accepted containers must not be offered the modality defaults instead.
  controls.push(
    chips(
      'output_format',
      'Output format',
      declaredList(contract?.output_formats) ?? IMAGE_FORMATS,
      'png',
      formatOptions
    )
  )

  controls.push({
    key: 'seed',
    labelKey: 'Seed',
    kind: 'slider',
    min: 0,
    max: 2147483647,
    step: 1,
    defaultValue: 0,
    advanced: true,
  })
  controls.push({
    key: 'n',
    labelKey: 'Images per request',
    kind: 'slider',
    min: 1,
    max: 4,
    step: 1,
    defaultValue: 1,
    advanced: true,
  })
  return controls
}

/**
 * The length control for a video model.
 *
 * A per-call model charges one price per invocation, so a picker over seconds
 * would claim a billing effect the model does not have. Where the envelope is
 * declared the control collapses to the values the model actually accepts;
 * where it is not declared the control is omitted and no length reaches the
 * request body, rather than shipping a guess as a real parameter.
 */
function buildDurationControl(
  perCall: boolean,
  envelope: DurationEnvelope | undefined,
  schema: BillingUsageSchema | undefined,
  override: ParamOverride | undefined,
  durations?: ModelContract['durations_seconds']
): ParamControl | undefined {
  if (!perCall) {
    // A declared enum is the model's own domain; absent one, the modality
    // ladder is the only shape available.
    const declared = enumOptions(schema, SCHEMA_FIELDS.duration)
    const options =
      declaredSeconds(durations) ??
      declared ??
      buildDurationOptions(4, 12, override)
    return {
      key: 'duration',
      labelKey: 'Duration (seconds)',
      kind: 'chips',
      numeric: true,
      options,
      defaultValue: options[0]?.value ?? '5',
      hintKey: 'Duration is billed per output second.',
    }
  }

  if (envelope?.kind === 'fixed') {
    const seconds = String(envelope.seconds)
    // `numeric` has to survive the lock: without it the value reaches the body
    // as a string and the gateway rejects the type.
    return {
      ...lockControl(
        chips('duration', 'Duration (seconds)', [seconds], seconds),
        seconds,
        'Fixed length — this model is charged once per generation'
      ),
      numeric: true,
    }
  }

  if (envelope?.kind === 'selectable' && envelope.seconds.length > 0) {
    const values = envelope.seconds.map((seconds) => String(seconds))
    return {
      ...chips('duration', 'Duration (seconds)', values, values[0]),
      numeric: true,
      hintKey: 'Charged once per generation, whatever the length',
    }
  }

  return undefined
}

/**
 * Duration chips from a declared envelope: exact values when the spec lists
 * them, otherwise the declared range. Returns nothing when the spec established
 * no envelope, so the caller keeps its previous behaviour.
 */
function declaredSeconds(
  envelope: ModelContract['durations_seconds']
): ParamOption[] | undefined {
  if (!envelope) return undefined
  if (envelope.allowed && envelope.allowed.length > 0) {
    return envelope.allowed.map((seconds) => ({ value: String(seconds) }))
  }
  if (typeof envelope.min === 'number' && typeof envelope.max === 'number') {
    return buildDurationOptions(envelope.min, envelope.max)
  }
  return undefined
}

/**
 * A per-call package envelope the contract already pins.
 *
 * Reading it here keeps the register from having to restate a length the model
 * declares: the register supplies an envelope only where the contract is silent.
 */
function declaredEnvelope(
  durations: ModelContract['durations_seconds']
): DurationEnvelope | undefined {
  const declared = declaredSeconds(durations)
  if (!declared) return undefined
  if (declared.length === 1) {
    return { kind: 'fixed', seconds: Number(declared[0].value) }
  }
  return {
    kind: 'selectable',
    seconds: declared.map((option) => Number(option.value)),
  }
}

/**
 * The declared prompt length, or undefined when the model established none.
 *
 * The gateway refuses an over-long prompt, so the field caps its input instead of
 * letting the user compose a request that cannot succeed.
 */
function declaredPromptMax(contract?: ModelContract): number | undefined {
  const declared = contract?.prompt_max_chars
  return typeof declared === 'number' && declared > 0 ? declared : undefined
}

/** A declared list, or undefined when the model declared none for that axis. */
function declaredList(values: string[] | undefined): string[] | undefined {
  return values && values.length > 0 ? values : undefined
}

function buildVideoControls(
  schema: BillingUsageSchema | undefined,
  override: ParamOverride | undefined,
  model: PricingModel
): ParamControl[] {
  const controls: ParamControl[] = []
  const modelName = model.model_name
  const contract = model.contract
  const ratioOptions = enumOptions(schema, SCHEMA_FIELDS.ratio)
  const resolutionOptions = enumOptions(schema, SCHEMA_FIELDS.resolution)
  const formatOptions = enumOptions(schema, SCHEMA_FIELDS.format)

  // A declared axis outranks every hand-written table: the contract is the
  // model's own statement of what it accepts, and those tables exist only
  // because it was previously unavailable.
  const declaredRatios = declaredList(contract?.aspect_ratios)
  const declaredResolutions = declaredList(contract?.resolutions)
  const declaredFormats = declaredList(contract?.output_formats)

  // A model whose tiers are provisioned offers exactly those tiers: the tier is
  // the group the gateway routes and bills on, so an unprovisioned resolution
  // would be a request with no group behind it. Absent tiers keep the declared
  // contract, which is the axis the model itself states.
  const provisionedResolutions = tierResolutions(model)

  // The billing dimension, not the modality, decides the shape of the length
  // control and which parameter domains are safe to offer.
  const declaration = billingDeclarationFor(modelName)
  const perCall = isPerCall(modelName)

  controls.push({
    ...chips(
      'aspect_ratio',
      'Aspect ratio',
      declaredRatios ?? declaration?.ratios ?? override?.ratios ?? VIDEO_RATIOS,
      'auto',
      ratioOptions
    ),
    // The video providers name the automatic choice `adaptive`; the chip keeps
    // reading "auto" but the body has to carry the vendor's term.
    valueAliases: { auto: 'adaptive' },
  })
  controls.push(
    chips(
      'resolution',
      'Resolution',
      provisionedResolutions ??
        declaredResolutions ??
        declaration?.resolutions ??
        override?.resolutions ??
        VIDEO_RESOLUTIONS,
      provisionedResolutions?.[0] ?? '480p',
      resolutionOptions
    )
  )

  const durationControl = buildDurationControl(
    perCall,
    declaration?.duration ?? declaredEnvelope(contract?.durations_seconds),
    schema,
    override,
    contract?.durations_seconds
  )
  if (durationControl) controls.push(durationControl)

  // Only offer audio where the model declares it. A silent video model must not
  // be shown a switch that promises synchronised sound.
  const audioControl: ParamControl | undefined =
    contract?.audio_supported === false
      ? undefined
      : {
          key: 'generate_audio',
          labelKey: 'Generate audio',
          kind: 'switch',
          defaultValue: false,
          hintKey: 'Synchronized audio, no extra charge.',
        }
  if (audioControl) controls.push(audioControl)
  controls.push(
    chips(
      'output_format',
      'Output format',
      declaredFormats ?? VIDEO_FORMATS,
      'mp4',
      formatOptions
    )
  )
  controls.push({
    key: 'seed',
    labelKey: 'Seed',
    kind: 'slider',
    min: 0,
    max: 2147483647,
    step: 1,
    defaultValue: 0,
    advanced: true,
  })
  return controls
}

/**
 * Derives the playground form schema for one model.
 *
 * Precedence: catalog `billing_usage_schema` enums → modality defaults →
 * the small PARAM_OVERRIDES table above.
 */
export function buildParamSchema(model: PricingModel): PlaygroundParamSchema {
  const modality = resolveModality(model)
  const schema = model.billing_usage_schema
  const override = PARAM_OVERRIDES[model.model_name]

  const capabilities: string[] = []
  if (modality === 'image') {
    capabilities.push(CAPABILITY_KEYS.textToImage)
    // Claimed only where the form renders a reference slot: the chip is a
    // promise the slot has to be able to keep, so the two share one predicate.
    if (acceptsReferenceMedia(model)) {
      capabilities.push(CAPABILITY_KEYS.imageToImage)
    }
    if (schema?.mask_url) capabilities.push(CAPABILITY_KEYS.maskEdit)
  } else {
    capabilities.push(CAPABILITY_KEYS.textToVideo)
    if (acceptsReferenceMedia(model)) {
      capabilities.push(
        CAPABILITY_KEYS.imageToVideo,
        CAPABILITY_KEYS.referenceToVideo
      )
    }
    capabilities.push(CAPABILITY_KEYS.videoAudio)
  }

  const controls: ParamControl[] = [
    {
      key: 'prompt',
      labelKey: 'Prompt',
      kind: 'text',
      defaultValue: '',
      maxChars: declaredPromptMax(model.contract),
    },
  ]

  if (acceptsReferenceMedia(model)) {
    const limits = referenceMediaLimits(model)
    controls.push({
      key: 'reference_media',
      labelKey: 'Reference images',
      kind: 'media',
      defaultValue: [],
      accept: limits.accept,
      maxCount: limits.maxCount,
      maxBytes: limits.maxBytes,
      hintKey:
        limits.maxCount === 1
          ? 'JPG / PNG / WebP — one file'
          : 'JPG / PNG / WebP — up to {{count}} files',
    })
  }

  controls.push(
    ...(modality === 'image'
      ? buildImageControls(schema, override, model.contract)
      : buildVideoControls(schema, override, model))
  )

  return {
    modelId: model.model_name,
    modality,
    capabilities,
    controls,
    advancedCount: controls.filter((control) => control.advanced === true)
      .length,
  }
}

/**
 * Media slot for one mode, carrying the ceiling the model documents.
 *
 * A single slot (`first_frame`, `source_videos`) renders as one fixed box
 * instead of a growing list, because the gateway wants exactly one there.
 */
function buildMediaControl(
  modelId: string,
  spec: ReturnType<typeof operationMedia>[number]
): ParamControl {
  const control: ParamControl = {
    key: spec.key,
    labelKey: spec.labelKey,
    kind: 'media',
    defaultValue: [],
    mediaRole: spec.mediaRole,
    accept: mediaAccept(spec.accept),
    hintKey: spec.hintKey,
  }
  const cap = spec.single ? 1 : mediaCap(modelId, spec.accept)
  if (cap !== undefined) control.maxCount = cap
  return control
}

/** Pins a control to the mode's forced value and explains why it cannot move. */
function lockControl(
  control: ParamControl,
  value: string,
  reasonKey: string
): ParamControl {
  const option: ParamOption = { value }
  // Duration -1 means "the model picks the length"; the wire value is not what
  // the user should read. Any other pinned duration is a length the user should
  // see as written, so only the sentinel gets a label.
  if (control.key === 'duration' && value === '-1') option.labelKey = 'Auto'
  return {
    ...control,
    options: [option],
    defaultValue: value,
    lockedReasonKey: reasonKey,
  }
}

/**
 * Derives the form for one (model, mode) pair.
 *
 * The mode-first counterpart to `buildParamSchema`: the same parameter controls,
 * restricted to the slots the mode accepts and pinned wherever the gateway pins
 * them. `capabilities` stays empty because the mode selector already says more
 * than chips could, and the chips component is not rendered here.
 */
export function buildOperationSchema(
  model: PricingModel,
  operation: PlaygroundOperation
): PlaygroundParamSchema {
  const modality = resolveModality(model)
  const schema = model.billing_usage_schema
  const override = PARAM_OVERRIDES[model.model_name]
  const locks = operationLocks(operation)

  const controls: ParamControl[] = [
    {
      key: 'prompt',
      labelKey: 'Prompt',
      kind: 'text',
      defaultValue: '',
      maxChars: declaredPromptMax(model.contract),
    },
  ]

  for (const spec of operationMedia(operation)) {
    controls.push(buildMediaControl(model.model_name, spec))
  }

  const parameterControls =
    modality === 'image'
      ? buildImageControls(schema, override, model.contract)
      : buildVideoControls(schema, override, model)

  for (const control of parameterControls) {
    const reasonKey = locks[control.key]
    const forced = lockedValue(control.key)
    controls.push(
      reasonKey && forced ? lockControl(control, forced, reasonKey) : control
    )
  }

  return {
    modelId: model.model_name,
    modality,
    capabilities: [],
    controls,
    advancedCount: controls.filter((control) => control.advanced === true)
      .length,
    operation,
  }
}

/** Default parameter values for a freshly opened playground. */
export function defaultParams(schema: PlaygroundParamSchema): PlaygroundParams {
  const params: PlaygroundParams = {}
  for (const control of schema.controls) {
    params[control.key] = Array.isArray(control.defaultValue)
      ? [...control.defaultValue]
      : control.defaultValue
  }
  return params
}

/**
 * Parameters shaped for the submit body.
 *
 * Controls flagged `numeric` hold strings in the form — option values double as
 * i18n keys, so they must stay strings for display — but the gateway validates
 * their JSON type. Coercing here, at the boundary, keeps both contracts intact.
 */
export function submitParams(
  schema: PlaygroundParamSchema,
  params: PlaygroundParams
): PlaygroundParams {
  const shaped: PlaygroundParams = { ...params }
  for (const control of schema.controls) {
    const aliases = control.valueAliases
    if (aliases) {
      const current = shaped[control.key]
      if (typeof current === 'string' && aliases[current]) {
        shaped[control.key] = aliases[current]
      }
    }
  }
  for (const control of schema.controls) {
    if (!control.numeric) continue
    const raw = shaped[control.key]
    if (typeof raw !== 'string' || raw.trim() === '') continue
    const numeric = Number(raw)
    if (Number.isFinite(numeric)) shaped[control.key] = numeric
  }
  return shaped
}

/**
 * Parameter combination a capability chip prefills.
 *
 * Chips are capabilities, not model IDs: selecting one only nudges the form
 * towards the matching input shape.
 */
export function capabilityPatch(
  capability: string,
  schema: PlaygroundParamSchema
): PlaygroundParams {
  const patch: PlaygroundParams = {}
  const hasMedia = schema.controls.some(
    (control) => control.key === 'reference_media'
  )

  switch (capability) {
    case CAPABILITY_KEYS.imageToVideo:
    case CAPABILITY_KEYS.referenceToVideo:
    case CAPABILITY_KEYS.maskEdit:
    case CAPABILITY_KEYS.imageToImage:
      if (hasMedia) patch.reference_media = []
      break
    case CAPABILITY_KEYS.videoAudio:
      patch.generate_audio = true
      break
    default:
      break
  }
  return patch
}
