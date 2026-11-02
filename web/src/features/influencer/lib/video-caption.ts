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
import type { TFunction } from 'i18next'

import type { MotionVideoSource } from '../types'
import { communityTrendPresets, motionExplorePresets } from './motion-explore'

/**
 * The caption under a filled video slot: only catalogue clips have one.
 * Uploaded files keep their name in the picker, never under the slot.
 */
export function motionVideoCaption(
  source: MotionVideoSource | null,
  t: TFunction
): string | undefined {
  if (source?.kind === 'preset') return t(source.nameKey)
  if (source?.kind === 'remote') {
    const catalogueVideo = [
      ...motionExplorePresets(),
      ...communityTrendPresets(),
    ].some(
      (preset) =>
        preset.title === source.name &&
        (preset.referenceVideoUrl ?? preset.videoUrl) === source.url
    )
    return catalogueVideo ? t(source.name) : undefined
  }
  return undefined
}
