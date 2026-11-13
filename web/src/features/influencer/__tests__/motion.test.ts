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

import {
  MOTION_VIDEO_MAX_BYTES,
  canSubmitMotion,
  composeMotionPrompt,
  isAcceptableVideoFile,
  motionModeMeta,
  presetsForMode,
} from '../lib/motion'
import { MOTION_PRESETS } from '../lib/motion-presets'

describe('reference video input formats', () => {
  it('rejects WebM that the server cannot use as a reference', () => {
    expect(
      isAcceptableVideoFile({ name: 'clip.webm', type: 'video/webm', size: 10 })
    ).toBe(false)
  })
})

describe('motionModeMeta', () => {
  it('treats the video as a pose source in motion transfer', () => {
    expect(motionModeMeta('motion').videoRole).toBe('driver')
  })

  it('treats the video as a scene plate in object swap', () => {
    expect(motionModeMeta('replace').videoRole).toBe('scene')
  })

  it('requires both slots in both modes', () => {
    expect(motionModeMeta('motion').requiresCharacter).toBe(true)
    expect(motionModeMeta('motion').requiresVideo).toBe(true)
    expect(motionModeMeta('replace').requiresCharacter).toBe(true)
    expect(motionModeMeta('replace').requiresVideo).toBe(true)
  })
})

describe('canSubmitMotion', () => {
  it('rejects while the character slot is empty', () => {
    expect(
      canSubmitMotion({ mode: 'motion', hasCharacter: false, hasVideo: true })
    ).toBe(false)
  })

  it('rejects while the video slot is empty', () => {
    expect(
      canSubmitMotion({ mode: 'replace', hasCharacter: true, hasVideo: false })
    ).toBe(false)
  })

  it('accepts once both slots are filled', () => {
    expect(
      canSubmitMotion({ mode: 'motion', hasCharacter: true, hasVideo: true })
    ).toBe(true)
  })
})

describe('composeMotionPrompt', () => {
  it('asks for the movement and pins the identity in motion transfer', () => {
    const prompt = composeMotionPrompt('motion', '')
    expect(prompt).toContain('motion transfer')
    expect(prompt).toContain('@Video')
    expect(prompt).toContain('@Image 1')
  })

  it('asks for the scene to survive in object swap', () => {
    const prompt = composeMotionPrompt('replace', '')
    expect(prompt).toContain('replace the primary person')
    expect(prompt).toContain('background scenery')
    expect(prompt).toContain('camera trajectory')
  })

  it('appends the user instruction when there is one', () => {
    expect(composeMotionPrompt('motion', '  keep the hat  ')).toMatch(
      /keep the hat$/
    )
  })

  it('adds no trailing space when the user instruction is blank', () => {
    expect(composeMotionPrompt('replace', '   ')).toBe(
      composeMotionPrompt('replace', '')
    )
  })
})

describe('isAcceptableVideoFile', () => {
  it('accepts an mp4 within the ceiling', () => {
    expect(
      isAcceptableVideoFile({ name: 'clip.mp4', type: 'video/mp4', size: 1024 })
    ).toBe(true)
  })

  it('accepts a mov whose type the browser left empty', () => {
    expect(
      isAcceptableVideoFile({ name: 'clip.MOV', type: '', size: 1024 })
    ).toBe(true)
  })

  it('rejects a clip over the gateway ceiling', () => {
    expect(
      isAcceptableVideoFile({
        name: 'clip.mp4',
        type: 'video/mp4',
        size: MOTION_VIDEO_MAX_BYTES + 1,
      })
    ).toBe(false)
  })

  it('rejects an empty file', () => {
    expect(
      isAcceptableVideoFile({ name: 'clip.mp4', type: 'video/mp4', size: 0 })
    ).toBe(false)
  })

  it('lets the server verify an MP4 extension when the browser MIME label differs', () => {
    expect(
      isAcceptableVideoFile({ name: 'clip.mp4', type: 'image/png', size: 1024 })
    ).toBe(true)
  })

  it('rejects an extension-less file with no declared type', () => {
    expect(
      isAcceptableVideoFile({ name: 'clip.bin', type: '', size: 1024 })
    ).toBe(false)
  })
})

describe('presetsForMode', () => {
  it('only returns templates belonging to the requested mode', () => {
    const motion = presetsForMode(MOTION_PRESETS, 'motion')
    expect(motion.length).toBeGreaterThan(0)
    expect(motion.every((preset) => preset.mode === 'motion')).toBe(true)
  })

  it('gives both modes something to offer', () => {
    expect(presetsForMode(MOTION_PRESETS, 'motion').length).toBeGreaterThan(0)
    expect(presetsForMode(MOTION_PRESETS, 'replace').length).toBeGreaterThan(0)
  })

  it('ships every mirrored template with a thumbnail and a public clip URL', () => {
    expect(MOTION_PRESETS.every((preset) => preset.thumbUrl)).toBe(true)
    expect(
      MOTION_PRESETS.every((preset) => preset.videoUrl?.startsWith('https://'))
    ).toBe(true)
  })
})
