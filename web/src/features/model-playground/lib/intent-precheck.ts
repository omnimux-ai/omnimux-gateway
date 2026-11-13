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
import type { PlaygroundOperation } from '../types'

/**
 * Prompt-intent pre-check for the two modes the model infers.
 *
 * Seedance documents that an edit or extend task is decided from the prompt
 * intent, and that a prompt without the matching wording fails *asynchronously* —
 * after the task has been queued. The gateway does not check for these words, so
 * warning before submit is the only thing standing between the user and a
 * ten-minute wait for a failure.
 *
 * The check is advisory: the list can never be complete, so it warns rather than
 * blocks.
 */
const INTENT_WORDS: Partial<Record<PlaygroundOperation, string[]>> = {
  video_edit: [
    'edit',
    'add',
    'delete',
    'remove',
    'modify',
    'replace',
    'change',
    'restyle',
    '编辑',
    '修改',
    '替换',
    '删除',
    '去掉',
    '添加',
    '加入',
    '改成',
    '换成',
  ],
  video_extend: [
    'extend',
    'continue',
    'forward',
    'backward',
    'prolong',
    '延长',
    '续',
    '继续',
    '向前',
    '向后',
  ],
}

/** Sentence appended by the "complete the prompt" affordance. */
const INTENT_SUFFIX: Partial<Record<PlaygroundOperation, string>> = {
  video_edit: 'Edit the video: ',
  video_extend: 'Continue the video forward: ',
}

function words(operation: PlaygroundOperation): string[] {
  return INTENT_WORDS[operation] ?? []
}

/** True for the modes whose prompt intent decides the task type upstream. */
export function needsIntentHint(operation: PlaygroundOperation): boolean {
  return words(operation).length > 0
}

/** True when the prompt already carries wording the model can act on. */
export function promptHasIntent(
  operation: PlaygroundOperation,
  prompt: string
): boolean {
  const list = words(operation)
  if (list.length === 0) return true
  const haystack = prompt.toLowerCase()
  return list.some((word) => haystack.includes(word.toLowerCase()))
}

/** Prefix that gives the model an unambiguous task intent. */
export function intentSuffix(operation: PlaygroundOperation): string {
  return INTENT_SUFFIX[operation] ?? ''
}
