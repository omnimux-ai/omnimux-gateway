import type { ParamControl, PlaygroundOperation } from '../types'
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
import { isDeclared, resolveDeclared } from './model-id'

/**
 * Frontend mirror of the gateway's mode contract.
 *
 * The authoritative table is `operationAllowed` in
 * `relay/channel/task/apimart/video_contract.go` (mode whitelist) together with
 * `validatePhaseOneOperationMedia` (per-mode media rules) and
 * `validatePhaseOneParameters` (per-mode parameter locks). This file repeats it
 * because the catalog payload carries no capability data yet; when a model is
 * absent here the form keeps its flat, modality-derived shape.
 *
 * Only models the apimart contract recognises are listed. A model that is not
 * declared must not gain a mode selector — that keeps this change additive.
 */
export const OPERATION_MATRIX: Record<string, PlaygroundOperation[]> = {
  'seedance-2-5': [
    'text_to_video',
    'first_frame',
    'first_last_frame',
    'video_multi_ref',
    'video_edit',
    'video_extend',
  ],
  'seedance-2-0': [
    'text_to_video',
    'first_frame',
    'first_last_frame',
    'video_multi_ref',
  ],
  'seedance-2-0-fast': [
    'text_to_video',
    'first_frame',
    'first_last_frame',
    'video_multi_ref',
  ],
  'seedance-2-0-mini': [
    'text_to_video',
    'first_frame',
    'first_last_frame',
    'video_multi_ref',
  ],
  'wan-3.0': [
    'text_to_video',
    'first_frame',
    'first_last_frame',
    'video_multi_ref',
  ],
  'minimax-h3': [
    'text_to_video',
    'first_frame',
    'first_last_frame',
    'video_multi_ref',
  ],
  'grok-imagine-video-1-5': ['text_to_video', 'video_multi_ref'],
}

/** i18n keys for the mode list; values never reach the wire. */
export const OPERATION_LABEL_KEYS: Record<PlaygroundOperation, string> = {
  text_to_video: 'Text to video',
  first_frame: 'First frame',
  first_last_frame: 'First & last frame',
  video_multi_ref: 'Reference to video',
  video_edit: 'Video edit',
  video_extend: 'Video extend',
}

export const DEFAULT_OPERATION: PlaygroundOperation = 'text_to_video'

type CapKind = 'image' | 'video' | 'audio'

/** Documented per-model reference limits (30/10/10 for 2.5, 9/3/3 below it). */
const MEDIA_CAPS: Record<string, Partial<Record<CapKind, number>>> = {
  'seedance-2-5': { image: 30, video: 10, audio: 10 },
  'seedance-2-0': { image: 9, video: 3, audio: 3 },
  'seedance-2-0-fast': { image: 9, video: 3, audio: 3 },
  'seedance-2-0-mini': { image: 9, video: 3, audio: 3 },
  'wan-3.0': { image: 10, video: 5, audio: 5 },
  'minimax-h3': { image: 9, video: 3, audio: 3 },
  'kling-v3': { image: 7, video: 1 },
  // The vendor publishes no image ceiling for Grok 1.5, allows up to 3 audio
  // preset voices, and rejects video references outright.
  'grok-imagine-video-1-5': { video: 0, audio: 3 },
}

const ACCEPT: Record<CapKind, string> = {
  image: 'image/png,image/jpeg,image/webp',
  video: 'video/mp4,video/quicktime',
  audio: 'audio/wav,audio/mpeg',
}

export type OperationMediaSpec = {
  key: string
  labelKey: string
  mediaRole: NonNullable<ParamControl['mediaRole']>
  accept: CapKind
  /** Marks the mode's primary slot; the form blocks submit while it is empty. */
  required: boolean
  /** Fixed single slot (a frame, a source clip) rather than a growing list. */
  single: boolean
  hintKey?: string
}

/**
 * Media slots each mode renders.
 *
 * Frame modes and reference mode are mutually exclusive upstream
 * ("frame and reference media families are mutually exclusive"), so no mode ever
 * lists both families — that is the whole reason the form is mode-first.
 */
const OPERATION_MEDIA: Record<PlaygroundOperation, OperationMediaSpec[]> = {
  text_to_video: [],
  first_frame: [
    {
      key: 'first_frame',
      labelKey: 'First frame',
      mediaRole: 'first_frame',
      accept: 'image',
      required: true,
      single: true,
    },
  ],
  first_last_frame: [
    {
      key: 'first_frame',
      labelKey: 'First frame',
      mediaRole: 'first_frame',
      accept: 'image',
      required: true,
      single: true,
    },
    {
      key: 'last_frame',
      labelKey: 'Last frame',
      mediaRole: 'last_frame',
      accept: 'image',
      required: true,
      single: true,
    },
  ],
  video_multi_ref: [
    {
      key: 'reference_images',
      labelKey: 'Reference images',
      mediaRole: 'reference_image',
      accept: 'image',
      required: false,
      single: false,
    },
    {
      key: 'reference_videos',
      labelKey: 'Reference videos',
      mediaRole: 'reference_video',
      accept: 'video',
      required: false,
      single: false,
    },
    {
      key: 'reference_audios',
      labelKey: 'Reference audio',
      mediaRole: 'reference_audio',
      accept: 'audio',
      required: false,
      single: false,
      // The gateway refuses reference audio on its own; it needs a visual
      // reference beside it.
      hintKey: 'Needs at least one reference image or video',
    },
  ],
  video_edit: [
    {
      key: 'source_videos',
      labelKey: 'Source video',
      mediaRole: 'reference_video',
      accept: 'video',
      required: true,
      single: true,
      hintKey: '4–30 seconds',
    },
  ],
  video_extend: [
    {
      key: 'source_videos',
      labelKey: 'Source video',
      mediaRole: 'reference_video',
      accept: 'video',
      required: true,
      single: true,
    },
  ],
}

/**
 * Controls a mode pins to a single value.
 *
 * Editing and extension follow the input video's geometry, and the frame modes
 * follow the first frame's, so an aspect-ratio picker there is a control that
 * cannot do anything.
 */
const OPERATION_LOCKS: Partial<
  Record<PlaygroundOperation, Record<string, string>>
> = {
  first_frame: { aspect_ratio: 'Aspect ratio follows the input frame' },
  first_last_frame: { aspect_ratio: 'Aspect ratio follows the input frame' },
  video_edit: {
    aspect_ratio: 'Aspect ratio follows the input video',
    duration: 'Duration follows the input video',
  },
  video_extend: { aspect_ratio: 'Aspect ratio follows the input video' },
}

/** Forced values for the controls listed in OPERATION_LOCKS. */
const LOCKED_VALUES: Record<string, string> = {
  aspect_ratio: 'adaptive',
  duration: '-1',
}

/** The vendor field that selects the sub-task inside the reference family. */
const REFERENCE_TASK_TYPE: Partial<Record<PlaygroundOperation, string>> = {
  video_multi_ref: 'reference',
  video_edit: 'edit',
  video_extend: 'extend',
}

/** Wan names the same idea differently. */
const GENERATION_TYPE: Partial<Record<PlaygroundOperation, string>> = {
  first_frame: 'frame',
  first_last_frame: 'frame',
  video_multi_ref: 'reference',
}

/** Models whose request carries the vendor sub-task field. */
const REFERENCE_TASK_TYPE_MODELS = new Set(['seedance-2-5'])
const GENERATION_TYPE_MODELS = new Set(['wan-3.0'])

/** Modes a model exposes; empty means "keep the flat form". */
export function operationsFor(modelId: string): PlaygroundOperation[] {
  return resolveDeclared(modelId, OPERATION_MATRIX) ?? []
}

/** True when the model declares modes and therefore earns a mode selector. */
export function hasOperations(modelId: string): boolean {
  return operationsFor(modelId).length > 0
}

export function operationMedia(
  operation: PlaygroundOperation
): OperationMediaSpec[] {
  return OPERATION_MEDIA[operation]
}

export function operationLocks(
  operation: PlaygroundOperation
): Record<string, string> {
  return OPERATION_LOCKS[operation] ?? {}
}

export function lockedValue(key: string): string | undefined {
  return LOCKED_VALUES[key]
}

/** Upload ceiling for one slot, or undefined when the vendor publishes none. */
export function mediaCap(
  modelId: string,
  kind: 'image' | 'video' | 'audio'
): number | undefined {
  return resolveDeclared(modelId, MEDIA_CAPS)?.[kind]
}

export function mediaAccept(kind: CapKind): string {
  return ACCEPT[kind]
}

/**
 * Vendor sub-task fields derived from the chosen mode.
 *
 * The gateway's apimart contract deletes `operation` before forwarding, so the
 * request has to carry the vendor's own field as well; without it the upstream
 * falls back to inferring the task from the prompt and can fail asynchronously.
 */
export function vendorSubTaskFields(
  modelId: string,
  operation: PlaygroundOperation
): Record<string, string> {
  if (isDeclared(modelId, REFERENCE_TASK_TYPE_MODELS)) {
    const value = REFERENCE_TASK_TYPE[operation]
    if (value) return { omni_reference_task_type: value }
    return {}
  }
  if (isDeclared(modelId, GENERATION_TYPE_MODELS)) {
    const value = GENERATION_TYPE[operation]
    if (value) return { generation_type: value }
    return {}
  }
  return {}
}
