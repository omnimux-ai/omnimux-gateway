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
  MOTION_MODES,
  composeMotionPrompt,
  isMotionMode,
  motionModeMeta,
  validateBrowserMotionMetadata,
  validateMotionReference,
} from '../lib/motion'
import { recreateQuoteFor } from '../lib/recreate'
import { buildMotionSubmitPayload } from '../lib/submit-payload'
import type { BrowserVideoMetadata } from '../lib/browser-video-metadata'
import type { MotionVideoMetadata } from '../types'

const browserMetadata = (
  overrides: Partial<BrowserVideoMetadata> = {}
): BrowserVideoMetadata => ({
  duration: 6,
  video_duration: 6,
  width: 1080,
  height: 1080,
  frame_rate: 30,
  video_codec: 'h264',
  audio_codec: 'aac',
  container: 'mp4',
  size_bytes: 2048,
  ...overrides,
})

const recreateClip: MotionVideoMetadata = {
  version: 1,
  browserMetadata: browserMetadata(),
  rawDurationSec: 6,
  durationSec: 6,
  width: 1080,
  height: 1080,
  aspectRatio: null,
}

const characters = [
  { url: 'https://files.example.com/front.png', fileName: 'front.png' },
  { url: 'https://files.example.com/side.png', fileName: 'side.png' },
]

describe('recreate mode', () => {
  it('is the third declared mode', () => {
    expect(MOTION_MODES).toEqual(['motion', 'replace', 'recreate'])
    expect(isMotionMode('recreate')).toBe(true)
  })

  it('uses the clip only as decomposition material', () => {
    const meta = motionModeMeta('recreate')
    expect(meta.videoRole).toBe('reference-only')
    expect(meta.requiresVideo).toBe(true)
    expect(meta.requiresCharacter).toBe(false)
  })

  it('never adds a video reference to the compiled prompt', () => {
    const prompt =
      'Reference image 1 is the hero product. The frame stays free of captions, lettering, logos and watermarks.'
    expect(composeMotionPrompt('recreate', prompt)).toBe(prompt)
    expect(composeMotionPrompt('recreate', prompt)).not.toMatch(
      /video|footage|clip/i
    )
  })

  it('accepts a decomposition clip that is not a usable 9:16 source', () => {
    expect(validateMotionReference(recreateClip, 'recreate', '720p').ok).toBe(
      true
    )
  })

  it('reports a reference clip shorter than four seconds', () => {
    expect(
      validateBrowserMotionMetadata(
        browserMetadata({ duration: 3, video_duration: 3 }),
        'recreate'
      )
    ).toMatchObject({ ok: false, reason: 'duration-short' })
  })

  it('reports a reference clip longer than the 30-second output cap', () => {
    expect(
      validateBrowserMotionMetadata(
        browserMetadata({ duration: 31, video_duration: 31 }),
        'recreate'
      )
    ).toMatchObject({ ok: false, reason: 'duration-long' })
  })
})

describe('recreate submit payload', () => {
  const build = (prompt: string, durationSec = 8) =>
    buildMotionSubmitPayload({
      mode: 'recreate',
      prompt,
      character: characters[0],
      characters,
      video: {
        kind: 'upload',
        url: 'https://files.example.com/reference.mp4',
        fileName: 'reference.mp4',
        metadata: recreateClip,
      },
      resolution: '480p',
      recreate: { prompt, durationSec },
    })

  it('sends the compiled prompt and images without the reference video', () => {
    const body = build('Reference image 1 is the hero product. ...')
    expect(body).toMatchObject({
      mode: 'recreate',
      prompt: 'Reference image 1 is the hero product. ...',
      subject_image_urls: [
        'https://files.example.com/front.png',
        'https://files.example.com/side.png',
      ],
      duration_sec: 8,
      resolution: '480p',
      aspect_ratio: '9:16',
    })
    expect(body).not.toHaveProperty('video_url')
  })

  it('rejects an empty compiled prompt', () => {
    expect(build('   ')).toBeNull()
  })

  it('rejects a prompt over the public rune ceiling', () => {
    expect(() => build('字'.repeat(4001))).toThrow(
      'Prompt must be 4000 characters or fewer.'
    )
  })

  it('rejects a duration outside the reference-video range', () => {
    expect(build('Reference image 1 is the hero product.', 3)).toBeNull()
    expect(build('Reference image 1 is the hero product.', 31)).toBeNull()
  })

  it('accepts a fractional clip rounded to whole seconds inside the range', () => {
    expect(
      build('Reference image 1 is the hero product.', 30)?.duration_sec
    ).toBe(30)
    expect(build('Reference image 1 is the hero product.', 5)?.duration_sec).toBe(
      5
    )
  })
})

describe('recreate quote', () => {
  const meta = {
    pricing: { '480p': 6.47, '720p': 13.89 },
  }

  it('reads the credit price of the selected quality from the server map', () => {
    expect(recreateQuoteFor(meta, '480p')).toEqual({ usd: 0, credits: 6.47 })
    expect(recreateQuoteFor(meta, '720p')).toEqual({ usd: 0, credits: 13.89 })
  })

  it('returns null when the server sends no price for that quality', () => {
    expect(recreateQuoteFor(meta, '1080p')).toBeNull()
    expect(recreateQuoteFor({ pricing: { '720p': 13.89 } }, '480p')).toBeNull()
    expect(recreateQuoteFor(null, '480p')).toBeNull()
  })
})
