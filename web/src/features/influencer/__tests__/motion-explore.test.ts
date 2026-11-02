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

import { assert, describe, expect, it } from 'vitest'

import {
  communityTrendPresets,
  exploreToMotionPreset,
  motionExplorePresets,
  motionTemplatePresets,
} from '../lib/motion-explore'

describe('motion template generation instructions', () => {
  it('keeps the replacement shelf descriptions visible without sending them as task instructions', () => {
    const replacement = motionTemplatePresets().filter(
      (preset) => preset.mode === 'replace'
    )

    expect(replacement).toHaveLength(10)
    for (const preset of replacement) {
      expect(preset.description).toBe('Explore this object replacement preset.')
      expect(preset.prompt).toBe('')
      const original = motionExplorePresets().find(
        (item) => item.id === preset.id
      )
      expect(preset.videoUrl).toBe(original?.videoUrl)
      expect(preset.referenceVideoUrl).toBe(original?.referenceVideoUrl)
    }
  })

  it('retains community replacement instructions when mapping a community clip', () => {
    const community = communityTrendPresets().find(
      (preset) => preset.mode === 'replace'
    )
    assert(community, 'The community replacement fixture must exist')

    expect(exploreToMotionPreset(community).prompt).toBe(community.description)
  })

  it('retains motion template instructions when no separate generation instruction is provided', () => {
    const motion = motionExplorePresets().find(
      (preset) => preset.mode === 'motion'
    )
    assert(motion, 'The motion template fixture must exist')

    expect(exploreToMotionPreset(motion).prompt).toBe(motion.description)
  })
})
