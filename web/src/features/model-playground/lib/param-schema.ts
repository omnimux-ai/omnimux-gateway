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
  ContractInput,
  ContractParameter,
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
import { findContractOperation } from './contract-schema'
import { lockedValue, operationLocks } from './operation-matrix'
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

/** Formats the image edit surface takes when a contract declares none. */
const DEFAULT_IMAGE_ACCEPT = 'image/png,image/jpeg,image/webp'
const DEFAULT_VIDEO_ACCEPT = 'video/mp4,video/quicktime'
const DEFAULT_AUDIO_ACCEPT = 'audio/wav,audio/mpeg'
const DEFAULT_FILE_ACCEPT = 'application/pdf'

const MIME_BY_FORMAT: Record<string, string> = {
  jpg: 'image/jpeg',
  jpeg: 'image/jpeg',
  png: 'image/png',
  webp: 'image/webp',
  gif: 'image/gif',
  bmp: 'image/bmp',
  tif: 'image/tiff',
  tiff: 'image/tiff',
  heic: 'image/heic',
  heif: 'image/heif',
  avif: 'image/avif',
  mp4: 'video/mp4',
  mov: 'video/quicktime',
  quicktime: 'video/quicktime',
  webm: 'video/webm',
  mpeg: 'video/mpeg',
  mpg: 'video/mpeg',
  wav: 'audio/wav',
  mp3: 'audio/mpeg',
  m4a: 'audio/mp4',
  ogg: 'audio/ogg',
  flac: 'audio/flac',
  pdf: 'application/pdf',
  doc: 'application/msword',
  docx: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document',
  xls: 'application/vnd.ms-excel',
  xlsx: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet',
  ppt: 'application/vnd.ms-powerpoint',
  pptx: 'application/vnd.openxmlformats-officedocument.presentationml.presentation',
  txt: 'text/plain',
  md: 'text/markdown',
}

type MediaInputKind = 'image' | 'video' | 'audio' | 'file'

function defaultAccept(kind: MediaInputKind): string {
  if (kind === 'video') return DEFAULT_VIDEO_ACCEPT
  if (kind === 'audio') return DEFAULT_AUDIO_ACCEPT
  if (kind === 'file') return DEFAULT_FILE_ACCEPT
  return DEFAULT_IMAGE_ACCEPT
}

/**
 * The `accept` attribute for one slot.
 *
 * Contract formats may be bare extensions (`jpeg`, `png`) or MIME types; the
 * browser and the selection guard both need MIME types, so bare tokens are
 * mapped where the vocabulary is known. An unknown token is dropped rather than
 * turned into a guessed MIME type.
 */
function acceptAttribute(
  allowedMimes: string[] | undefined,
  kind: MediaInputKind
): string {
  if (!allowedMimes?.length) return defaultAccept(kind)
  const mimes: string[] = []
  for (const raw of allowedMimes) {
    const token = raw.trim().toLowerCase()
    if (!token) continue
    const mime = token.includes('/') ? token : (MIME_BY_FORMAT[token] ?? '')
    if (mime && !mimes.includes(mime)) mimes.push(mime)
  }
  return mimes.length > 0 ? mimes.join(',') : defaultAccept(kind)
}

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

/**
 * The flat form's single image-reference slot, limited by the contract.
 *
 * The ceiling is `contract.reference_images`; a model without a contract keeps
 * its previous one-image fallback instead of gaining a limit nobody stated.
 */
function referenceMediaControl(model: PricingModel): ParamControl | undefined {
  if (!acceptsReferenceMedia(model)) return undefined
  const declared = model.contract?.reference_images
  if (declared && declared.max === 0) return undefined

  const maxCount = declared?.max ?? 1
  const accept = acceptAttribute(declared?.formats, 'image')
  const control: ParamControl = {
    key: 'reference_media',
    labelKey: 'Reference images',
    kind: 'media',
    defaultValue: [],
    accept,
    maxCount,
  }
  if (declared?.max_bytes_each !== undefined) {
    control.maxBytes = declared.max_bytes_each
  }
  if (!declared?.formats?.length) {
    control.hintKey =
      maxCount === 1
        ? 'JPG / PNG / WebP — one file'
        : 'JPG / PNG / WebP — up to {{count}} files'
  }
  return control
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

function buildDurationOptions(min: number, max: number): ParamOption[] {
  if (!Number.isFinite(min) || !Number.isFinite(max) || max < min) {
    return [
      { value: '4' },
      { value: '5' },
      { value: '8' },
      { value: '10' },
      { value: '12' },
    ]
  }
  const candidates = new Set<number>([min, max])
  for (const step of [4, 5, 6, 8, 10, 12, 15]) {
    if (step >= min && step <= max) candidates.add(step)
  }
  return [...candidates]
    .sort((a, b) => a - b)
    .map((value) => ({ value: String(value) }))
}

/** Fallback image controls for a model whose contract states no parameters. */
function buildImageControls(
  schema: BillingUsageSchema | undefined,
  contract: ModelContract | undefined
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
      declaredList(contract?.aspect_ratios) ?? IMAGE_RATIOS,
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
        IMAGE_RESOLUTIONS,
      '1K',
      resolutionOptions
    )
  )
  controls.push(
    segmented('quality', 'Quality', IMAGE_QUALITIES, 'medium', qualityOptions)
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
  durations?: ModelContract['durations_seconds']
): ParamControl | undefined {
  if (!perCall) {
    const declared = enumOptions(schema, SCHEMA_FIELDS.duration)
    const options =
      declaredSeconds(durations) ?? declared ?? buildDurationOptions(4, 12)
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

/** Fallback video controls for a model whose contract states no parameters. */
function buildVideoControls(
  schema: BillingUsageSchema | undefined,
  model: PricingModel
): ParamControl[] {
  const controls: ParamControl[] = []
  const modelName = model.model_name
  const contract = model.contract
  const ratioOptions = enumOptions(schema, SCHEMA_FIELDS.ratio)
  const resolutionOptions = enumOptions(schema, SCHEMA_FIELDS.resolution)
  const formatOptions = enumOptions(schema, SCHEMA_FIELDS.format)

  // A declared axis outranks every hand-written default: the contract is the
  // model's own statement of what it accepts.
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
      declaredRatios ?? declaration?.ratios ?? VIDEO_RATIOS,
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
        VIDEO_RESOLUTIONS,
      provisionedResolutions?.[0] ?? '480p',
      resolutionOptions
    )
  )

  const durationControl = buildDurationControl(
    perCall,
    declaration?.duration ?? declaredEnvelope(contract?.durations_seconds),
    schema,
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

/** Slot labels for contract inputs, as i18n keys. */
const SLOT_LABEL_KEYS: Record<string, string> = {
  prompt: 'Prompt',
  first_frame: 'First frame',
  last_frame: 'Last frame',
  reference_images: 'Reference images',
  reference_videos: 'Reference videos',
  reference_audios: 'Reference audio',
  source_videos: 'Source video',
  reference_image: 'Reference image',
  reference_audio: 'Reference audio',
  file: 'Document',
  url: 'Webpage URL',
}

/** Parameter labels for contract parameters, as i18n keys. */
const PARAM_LABEL_KEYS: Record<string, string> = {
  seed: 'Seed',
  n: 'Images per request',
  quality: 'Quality',
  background: 'Background',
  aspect_ratio: 'Aspect ratio',
  resolution: 'Resolution',
  duration: 'Duration (seconds)',
  generate_audio: 'Generate audio',
  output_format: 'Output format',
}

function slotLabelKey(slot: string): string {
  return SLOT_LABEL_KEYS[slot] ?? slot.replaceAll('_', ' ')
}

function paramLabelKey(name: string): string {
  return PARAM_LABEL_KEYS[name] ?? name.replaceAll('_', ' ')
}

/** The control shape one contract parameter maps onto. */
function contractParameterKind(param: ContractParameter): ParamControl['kind'] {
  if (param.name === 'quality' || param.name === 'background') {
    return 'segmented'
  }
  if (param.type === 'integer' || param.type === 'number') return 'slider'
  if (param.type === 'enum') return 'chips'
  if (param.type === 'boolean') return 'switch'
  return 'text'
}

/**
 * Narrow a resolution control to the tiers the model's catalog provisions.
 *
 * Tier groups are a billing/routing fact the contract does not carry; offering
 * a resolution without a provisioned group would pin the request to a group
 * that cannot serve it. A model with no provisioned tiers keeps the contract's
 * full list.
 */
function applyProvisionedResolutions(
  model: PricingModel,
  control: ParamControl
): ParamControl {
  if (control.key !== 'resolution') return control
  if (control.kind !== 'chips' && control.kind !== 'segmented') return control

  const tiers = tierResolutions(model)
  if (!tiers?.length) return control
  const options = (control.options ?? []).filter((option) =>
    tiers.some((tier) => tier.toLowerCase() === option.value.toLowerCase())
  )
  if (options.length === 0) return control
  const keepsDefault = options.some(
    (option) => option.value === control.defaultValue
  )
  return {
    ...control,
    options,
    defaultValue: keepsDefault ? control.defaultValue : options[0].value,
  }
}

/**
 * One control per contract parameter.
 *
 * `fixed` collapses the control to the single value the service accepts and
 * explains why; `enum` is a chip row (quality and background read better as a
 * segmented control); numbers are sliders and booleans switches. Bounds and
 * defaults come from the contract, never from a client-side table.
 */
function contractParameterControl(param: ContractParameter): ParamControl {
  const labelKey = paramLabelKey(param.name)
  const advanced = param.name === 'seed'

  if (param.fixed !== undefined) {
    const value = String(param.fixed)
    const option: ParamOption = { value }
    if (param.name === 'duration' && value === '-1') option.labelKey = 'Auto'
    return {
      key: param.name,
      labelKey,
      kind: contractParameterKind(param),
      options: [option],
      defaultValue: value,
      numeric: param.type === 'integer' || param.type === 'number',
      advanced,
      lockedReasonKey: 'Locked by the service contract',
    }
  }

  if (param.type === 'enum') {
    const values = (param.enum ?? []).map((value) => String(value))
    const options = values.map((value) => ({ value }))
    const declaredDefault =
      param.default === undefined ? values[0] : String(param.default)
    return {
      key: param.name,
      labelKey,
      kind:
        param.name === 'quality' || param.name === 'background'
          ? 'segmented'
          : 'chips',
      options,
      defaultValue: values.includes(declaredDefault)
        ? declaredDefault
        : values[0],
      advanced,
    }
  }

  if (param.type === 'integer' || param.type === 'number') {
    const min = param.minimum ?? 0
    const max = param.maximum ?? min
    return {
      key: param.name,
      labelKey,
      kind: 'slider',
      min,
      max,
      step: param.step ?? 1,
      defaultValue: typeof param.default === 'number' ? param.default : min,
      numeric: true,
      advanced,
    }
  }

  if (param.type === 'boolean') {
    return {
      key: param.name,
      labelKey,
      kind: 'switch',
      defaultValue: param.default === true,
      advanced,
    }
  }

  return {
    key: param.name,
    labelKey,
    kind: 'text',
    defaultValue: param.default === undefined ? '' : String(param.default),
    advanced,
  }
}

/** The upload role one contract input plays, when it is a media slot. */
function mediaRoleFor(
  input: ContractInput
): ParamControl['mediaRole'] | undefined {
  if (input.slot === 'first_frame') return 'first_frame'
  if (input.slot === 'last_frame') return 'last_frame'
  if (input.type === 'video') return 'reference_video'
  if (input.type === 'audio') return 'reference_audio'
  if (input.type === 'image') return 'reference_image'
  return undefined
}

/** One control per contract input slot. */
function contractInputControl(
  model: PricingModel,
  input: ContractInput
): ParamControl | undefined {
  if (input.type === 'text') {
    return {
      key: input.slot,
      labelKey: slotLabelKey(input.slot),
      kind: 'text',
      defaultValue: '',
      maxChars: declaredPromptMax(model.contract),
    }
  }
  if (input.type === 'url') {
    return {
      key: input.slot,
      labelKey: slotLabelKey(input.slot),
      kind: 'text',
      defaultValue: '',
      placeholderKey: 'Enter a URL',
    }
  }
  if (
    input.type !== 'image' &&
    input.type !== 'video' &&
    input.type !== 'audio' &&
    input.type !== 'file'
  ) {
    return undefined
  }

  const mediaRole = mediaRoleFor(input)
  const control: ParamControl = {
    key: input.slot,
    labelKey: slotLabelKey(input.slot),
    kind: 'media',
    defaultValue: [],
    accept: acceptAttribute(input.allowedMimes, input.type),
  }
  if (mediaRole) control.mediaRole = mediaRole
  if (input.max !== undefined) control.maxCount = input.max
  // A document slot is a single file even when the contract states no count:
  // the wire field is one `file_url`, not a list.
  if (input.type === 'file' && input.max === undefined) control.maxCount = 1
  if (input.maxSizeMb !== undefined) {
    control.maxBytes = Math.round(input.maxSizeMb * 1024 * 1024)
  }
  return control
}

/** Generic operation slots used only when no contract operation is published. */
function legacyOperationInputs(
  operation: PlaygroundOperation
): ContractInput[] {
  switch (operation) {
    case 'first_frame':
      return [
        {
          slot: 'first_frame',
          type: 'image',
          role: 'first_frame',
          required: true,
          min: 1,
          max: 1,
        },
      ]
    case 'first_last_frame':
      return [
        {
          slot: 'first_frame',
          type: 'image',
          role: 'first_frame',
          required: true,
          min: 1,
          max: 1,
        },
        {
          slot: 'last_frame',
          type: 'image',
          role: 'last_frame',
          required: true,
          min: 1,
          max: 1,
        },
      ]
    case 'video_multi_ref':
      return [
        {
          slot: 'reference_images',
          type: 'image',
          role: 'reference',
          required: false,
          min: 0,
        },
        {
          slot: 'reference_videos',
          type: 'video',
          role: 'reference',
          required: false,
          min: 0,
        },
        {
          slot: 'reference_audios',
          type: 'audio',
          role: 'reference',
          required: false,
          min: 0,
        },
      ]
    case 'video_edit':
    case 'video_extend':
      return [
        {
          slot: 'source_videos',
          type: 'video',
          role: 'source',
          required: true,
          min: 1,
          max: 1,
        },
      ]
    case 'end_frame':
      return [
        {
          slot: 'last_frame',
          type: 'image',
          role: 'last_frame',
          required: true,
          min: 1,
          max: 1,
        },
      ]
    case 'document_to_video':
      return [
        {
          slot: 'file',
          type: 'file',
          required: true,
          min: 1,
          max: 1,
        },
      ]
    case 'webpage_to_video':
      return [{ slot: 'url', type: 'url', required: true }]
    case 'digital_human':
      return [
        {
          slot: 'reference_image',
          type: 'image',
          required: true,
          min: 1,
          max: 1,
        },
        {
          slot: 'reference_audio',
          type: 'audio',
          required: true,
          min: 1,
          max: 1,
        },
      ]
    case 'text_to_video':
    default:
      return []
  }
}

/** Contract-declared controls for one operation, or undefined when absent. */
function contractOperationControls(
  model: PricingModel,
  operation: PlaygroundOperation
): ParamControl[] | undefined {
  const declared = findContractOperation(model.contract, operation)
  if (!declared?.executable) return undefined

  const controls: ParamControl[] = []
  for (const input of declared.inputs ?? []) {
    const control = contractInputControl(model, input)
    if (control) controls.push(control)
  }
  for (const param of declared.parameters ?? []) {
    controls.push(
      applyProvisionedResolutions(model, contractParameterControl(param))
    )
  }
  return controls
}

/** Contract parameters of the image operation the flat image form renders. */
function imageContractControls(
  model: PricingModel
): ParamControl[] | undefined {
  const contract = model.contract
  if (!contract) return undefined

  const operation = ['text_to_image', 'image_to_image']
    .map((id) => findContractOperation(contract, id))
    .find((candidate) => candidate?.executable === true)
  if (!operation) return undefined

  const controls: ParamControl[] = [
    {
      key: 'size_mode',
      labelKey: 'Size mode',
      kind: 'segmented',
      options: [{ value: 'auto' }, { value: 'ratio' }, { value: 'custom' }],
      defaultValue: 'auto',
    },
    ...(operation.parameters ?? []).map(contractParameterControl),
  ]
  const sizes = operation.output?.sizes
  if (
    sizes?.length &&
    !controls.some((control) => control.key === 'resolution')
  ) {
    controls.push({
      key: 'resolution',
      labelKey: 'Resolution',
      kind: 'chips',
      options: sizes.map((value) => ({ value })),
      defaultValue: sizes[0],
    })
  }

  const formats = operation.output?.formats
  if (
    formats?.length &&
    !controls.some((control) => control.key === 'output_format')
  ) {
    controls.push({
      key: 'output_format',
      labelKey: 'Output format',
      kind: 'chips',
      options: formats.map((value) => ({ value })),
      defaultValue: formats[0],
    })
  }
  return controls
}

/**
 * Derives the playground form schema for one model.
 *
 * An image model whose contract declares parameters renders those controls; a
 * model whose contract states none keeps the modality-derived fallback.
 */
export function buildParamSchema(model: PricingModel): PlaygroundParamSchema {
  const modality = resolveModality(model)
  const schema = model.billing_usage_schema

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

  const reference = referenceMediaControl(model)
  if (reference) controls.push(reference)

  if (modality === 'image') {
    const contracted = imageContractControls(model)
    controls.push(...(contracted ?? buildImageControls(schema, model.contract)))
  } else {
    controls.push(...buildVideoControls(schema, model))
  }

  return {
    modelId: model.model_name,
    modality,
    capabilities,
    controls,
    advancedCount: controls.filter((control) => control.advanced === true)
      .length,
  }
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
 * Contract-declared inputs and parameters are the authority; a model without a
 * contract operation keeps the previous operation template so the fallback path
 * does not regress.
 */
export function buildOperationSchema(
  model: PricingModel,
  operation: PlaygroundOperation
): PlaygroundParamSchema {
  const modality = resolveModality(model)
  const schema = model.billing_usage_schema
  const contracted = contractOperationControls(model, operation)

  if (contracted) {
    return {
      modelId: model.model_name,
      modality,
      capabilities: [],
      controls: contracted,
      advancedCount: contracted.filter((control) => control.advanced === true)
        .length,
      operation,
    }
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

  for (const input of legacyOperationInputs(operation)) {
    const control = contractInputControl(model, input)
    if (!control) continue
    if (operation === 'video_multi_ref' && input.slot === 'reference_audios') {
      control.hintKey = 'Needs at least one reference image or video'
    }
    controls.push(control)
  }

  const parameterControls =
    modality === 'image'
      ? buildImageControls(schema, model.contract)
      : buildVideoControls(schema, model)

  const locks = operationLocks(operation)
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
