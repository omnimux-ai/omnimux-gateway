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

import { describe, expect, it } from 'vitest'

import { combineStylePrompt, styleThumbnails } from '../lib/motion-style'
import type { MotionPreset, MotionVisualStyle } from '../types'

function preset(over: Partial<MotionPreset> = {}): MotionPreset {
  return {
    id: 'p1',
    nameKey: 'Patchwork Runway',
    mode: 'motion',
    prompt: 'Keep the patchwork runway look',
    durationSec: 5,
    ...over,
  }
}

describe('combineStylePrompt', () => {
  it('leads with the style instruction and keeps the caller sentence after it', () => {
    expect(combineStylePrompt('Style words', 'my own note')).toBe(
      'Style words. my own note'
    )
  })

  it('sends the style instruction alone when the prompt box is empty', () => {
    expect(combineStylePrompt('Style words', '   ')).toBe('Style words')
  })

  it('sends the caller sentence alone when no style is chosen', () => {
    expect(combineStylePrompt(null, '  my own note ')).toBe('my own note')
  })

  it('sends nothing when neither a style nor a prompt is set', () => {
    expect(combineStylePrompt(undefined, '')).toBe('')
  })
})

describe('styleThumbnails', () => {
  it('skips templates whose art has not landed and keeps the requested count', () => {
    const withArt = preset({ id: 'a', thumbUrl: 'https://cdn/a.webp' })
    const withoutArt = preset({ id: 'b' })
    const second = preset({ id: 'c', thumbUrl: 'https://cdn/c.webp' })

    expect(
      styleThumbnails([withoutArt, withArt, second]).map((p) => p.id)
    ).toEqual(['a', 'c'])
  })

  it('caps the row at three tiles', () => {
    const many = ['a', 'b', 'c', 'd'].map((id) =>
      preset({ id, thumbUrl: `https://cdn/${id}.webp` })
    )

    expect(styleThumbnails(many)).toHaveLength(3)
  })

  it('reads visual covers without treating previews as replacement inputs', () => {
    const style: MotionVisualStyle = {
      id: 'visual',
      nameKey: 'Visual style',
      summaryKey: 'Visual summary',
      prompt: 'Visual instruction',
      mode: 'motion',
      previewImageUrl: '/influencer-styles/visual.webp',
      previewVideoUrl: '/influencer-styles/visual.mp4',
    }
    expect(styleThumbnails([style])).toEqual([
      { id: 'visual', thumbUrl: '/influencer-styles/visual.webp' },
    ])
    expect(style).not.toHaveProperty('videoUrl')
    expect(style).not.toHaveProperty('thumbUrl')
  })
})
