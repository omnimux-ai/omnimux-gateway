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

import type { MotionMode, MotionStyleChoice } from '../types'

/**
 * A style contributes its own instruction to the job without taking the prompt
 * box hostage: the box stays the caller's own words, and the two are joined
 * here, at the last moment before submission.
 *
 * The style instruction leads because it describes the look the clip should
 * land in; the caller's sentence refines it. The gateway then prepends its
 * mode framing (`influencerMotionPrompts`) to whatever this returns.
 */
export function combineStylePrompt(
  styleInstruction: string | null | undefined,
  userPrompt: string,
  mode: MotionMode = 'replace'
): string {
  const style = (styleInstruction ?? '').trim()
  const own = userPrompt.trim()
  if (!style) return own
  if (!own) return style
  const separator = mode === 'motion' ? '\n\n' : '. '
  return `${style}${separator}${own}`
}

/**
 * The thumbnails stacked on the style row: a glance at what the pack holds
 * without opening the picker. Templates whose assets have not landed yet are
 * skipped rather than rendered as empty frames.
 */
export function styleThumbnails(
  presets: MotionStyleChoice[],
  limit = 3
): { id: string; thumbUrl: string }[] {
  return presets
    .flatMap((preset) => {
      const thumbUrl =
        'summaryKey' in preset ? preset.previewImageUrl : preset.thumbUrl
      return thumbUrl ? [{ id: preset.id, thumbUrl }] : []
    })
    .slice(0, limit)
}
