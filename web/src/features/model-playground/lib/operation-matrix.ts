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
import type { PricingModel } from '@/features/pricing/types'

import type { PlaygroundOperation } from '../types'
import { contractOperations } from './contract-schema'
import { isDeclared } from './model-id'

/** i18n keys for the mode list; values never reach the wire. */
export const OPERATION_LABEL_KEYS: Record<PlaygroundOperation, string> = {
  text_to_video: 'Text to video',
  first_frame: 'First frame',
  first_last_frame: 'First & last frame',
  video_multi_ref: 'Reference to video',
  video_edit: 'Video edit',
  video_extend: 'Video extend',
  end_frame: 'End frame',
  document_to_video: 'Document to video',
  webpage_to_video: 'Webpage to video',
  digital_human: 'Digital human',
  lip_sync: 'Lip sync',
}

export const DEFAULT_OPERATION: PlaygroundOperation = 'text_to_video'

const PLAYGROUND_OPERATIONS: ReadonlySet<string> = new Set(
  Object.keys(OPERATION_LABEL_KEYS)
)

function isPlaygroundOperation(id: string): id is PlaygroundOperation {
  return PLAYGROUND_OPERATIONS.has(id)
}

/**
 * Video modes a model exposes through its published contract.
 *
 * The contract is the only source: an operation the gateway marks
 * non-executable, produces something other than video, or names outside the
 * playground's vocabulary never becomes a mode. A model without a contract
 * keeps the flat, modality-derived form — the same behaviour it had before
 * contracts existed.
 */
export function operationsFor(model: PricingModel): PlaygroundOperation[] {
  const operations: PlaygroundOperation[] = []
  for (const operation of contractOperations(model.contract)) {
    if (!operation.executable) continue
    if (operation.output.type !== 'video') continue
    if (!isPlaygroundOperation(operation.id)) continue
    if (operations.includes(operation.id)) continue
    operations.push(operation.id)
  }
  return operations
}

/** True when the model declares modes and therefore earns a mode selector. */
export function hasOperations(model: PricingModel): boolean {
  return operationsFor(model).length > 0
}

/**
 * Controls a mode pins to a single value when no contract operation exists.
 *
 * Legacy fallback only: a contract operation states its own `fixed` parameters.
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

export function operationLocks(
  operation: PlaygroundOperation
): Record<string, string> {
  return OPERATION_LOCKS[operation] ?? {}
}

export function lockedValue(key: string): string | undefined {
  return LOCKED_VALUES[key]
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
