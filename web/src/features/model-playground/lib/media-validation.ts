import type { ModelContract } from '@/features/pricing/types'

import type { PlaygroundOperation, PlaygroundParams } from '../types'
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
import { operationMedia } from './operation-matrix'

/**
 * Client-side mirror of the gateway's per-mode media rules.
 *
 * This is not a second source of truth: the gateway re-checks everything and
 * stays the last line of defence. The point is that the gateway rejects the
 * request *after* it has been submitted, while the form can refuse to submit at
 * all — the difference between an error toast and a disabled button.
 */
export type MediaIssue =
  | { code: 'missing_required'; slotKey: string }
  | { code: 'audio_alone' }
  | { code: 'too_many'; slotKey: string; max: number }
  | { code: 'too_few'; slotKey: string; min: number }

/** Ceiling and floor a model declares for one reference family. */
export type ReferenceLimit = { min?: number; max?: number }

/** Reference limits keyed by the parameter slot they constrain. */
export type ReferenceLimits = Partial<Record<string, ReferenceLimit>>

/**
 * The declared limits for a model's reference slots.
 *
 * A model that publishes no contract, or a contract whose spec established no
 * limit for a family, contributes nothing — the form then keeps its previous
 * behaviour instead of inventing a ceiling.
 */
export function referenceLimitsFrom(
  contract: ModelContract | undefined
): ReferenceLimits {
  if (!contract) return {}
  const limits: ReferenceLimits = {}
  for (const slotKey of [
    'reference_images',
    'reference_videos',
    'reference_audios',
  ] as const) {
    const declared = contract[slotKey]
    if (!declared) continue
    if (typeof declared.min === 'number' || typeof declared.max === 'number') {
      limits[slotKey] = { min: declared.min, max: declared.max }
    }
  }
  return limits
}

function filled(params: PlaygroundParams, key: string): string[] {
  const value = params[key]
  if (!Array.isArray(value)) return []
  return value.filter(
    (item): item is string => typeof item === 'string' && item !== ''
  )
}

/** The first rule the current parameter set breaks, or null when it is valid. */
export function validateOperationMedia(
  operation: PlaygroundOperation,
  params: PlaygroundParams,
  limits: ReferenceLimits = {}
): MediaIssue | null {
  const specs = operationMedia(operation)

  for (const spec of specs) {
    if (!spec.required) continue
    if (filled(params, spec.key).length === 0) {
      return { code: 'missing_required', slotKey: spec.key }
    }
  }

  // What the model declares it accepts, checked here so an over-full or partial
  // set is refused by the form rather than by the gateway after submission.
  for (const spec of specs) {
    const limit = limits[spec.key]
    if (!limit) continue
    const count = filled(params, spec.key).length

    if (limit.max !== undefined && count > limit.max) {
      return { code: 'too_many', slotKey: spec.key, max: limit.max }
    }
    // A partial set is unambiguous; an untouched optional slot is not, so it is
    // left to the required check above.
    if (limit.min !== undefined && count > 0 && count < limit.min) {
      return { code: 'too_few', slotKey: spec.key, min: limit.min }
    }
  }

  // Reference mode has no single required slot, but the gateway refuses a
  // reference operation with nothing to reference.
  if (operation === 'video_multi_ref') {
    const images = filled(params, 'reference_images')
    const videos = filled(params, 'reference_videos')
    const audios = filled(params, 'reference_audios')
    if (images.length + videos.length + audios.length === 0) {
      return { code: 'missing_required', slotKey: 'reference_images' }
    }
    // Reference audio needs a visual reference beside it; the gateway refuses an
    // audio-only reference set outright.
    if (audios.length > 0 && images.length + videos.length === 0) {
      return { code: 'audio_alone' }
    }
  }

  return null
}

/** True when the mode has no usable input yet, so submit stays disabled. */
export function hasBlockingMediaIssue(
  operation: PlaygroundOperation,
  params: PlaygroundParams,
  limits: ReferenceLimits = {}
): boolean {
  return validateOperationMedia(operation, params, limits) !== null
}

/** The slot rules the form itself can enforce, taken from the control. */
export type MediaSelectionLimits = {
  /** Accepted MIME types; an empty list means the model documents none. */
  accept: string[]
  /** Ceiling for the slot; undefined means the model documents none. */
  maxCount?: number
  /** Per-file byte ceiling; undefined means the model documents none. */
  maxBytes?: number
}

export type MediaSelectionIssue =
  | { code: 'too_many'; limit: number }
  | { code: 'too_large'; limitBytes: number }
  | { code: 'unsupported_type'; mimeType: string }

/**
 * Splits a picked selection into what the slot takes, plus the first reason it
 * refuses the rest.
 *
 * Format and size are checked before the count, so the count message describes
 * the files that were actually eligible. The caller uploads `accepted` and
 * nothing else: an over-limit, over-sized or wrong-format file must never reach
 * the network, because the gateway precharges before the upstream ever sees the
 * request.
 */
export function screenMediaSelection(
  limits: MediaSelectionLimits,
  existing: number,
  files: File[]
): { accepted: File[]; issue: MediaSelectionIssue | null } {
  const takes = (file: File): boolean =>
    limits.accept.length === 0 || limits.accept.includes(file.type)
  const fits = (file: File): boolean =>
    limits.maxBytes === undefined || file.size <= limits.maxBytes

  const wrongType = files.find((file) => !takes(file))
  const tooLarge = files.find((file) => takes(file) && !fits(file))
  const eligible = files.filter((file) => takes(file) && fits(file))
  const room =
    limits.maxCount === undefined
      ? eligible.length
      : Math.max(0, limits.maxCount - existing)
  const accepted = eligible.slice(0, room)

  if (wrongType) {
    return {
      accepted,
      issue: { code: 'unsupported_type', mimeType: wrongType.type },
    }
  }
  if (tooLarge && limits.maxBytes !== undefined) {
    return {
      accepted,
      issue: { code: 'too_large', limitBytes: limits.maxBytes },
    }
  }
  if (limits.maxCount !== undefined && eligible.length > room) {
    return { accepted, issue: { code: 'too_many', limit: limits.maxCount } }
  }
  return { accepted, issue: null }
}

/** Splits an `accept` attribute value back into the MIME types it lists. */
export function acceptTypes(accept: string | undefined): string[] {
  if (!accept) return []
  return accept
    .split(',')
    .map((entry) => entry.trim())
    .filter((entry) => entry !== '')
}
