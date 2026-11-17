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

import type { BrowserVideoMetadata } from '../lib/browser-video-metadata'
import {
  MOTION_MODES,
  RECREATE_VIDEO_MAX_SECONDS,
  RECREATE_VIDEO_MIN_SECONDS,
  composeMotionPrompt,
  isMotionMode,
  motionModeMeta,
  motionVideoFailureMessage,
  validateBrowserMotionMetadata,
  validateMotionReference,
} from '../lib/motion'
import { buildMotionSubmitPayload } from '../lib/submit-payload'
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

  it('states the same 4–30 second window it enforces when a clip is refused', () => {
    expect([RECREATE_VIDEO_MIN_SECONDS, RECREATE_VIDEO_MAX_SECONDS]).toEqual([
      4, 30,
    ])
    expect(motionVideoFailureMessage('duration', 'recreate')).toBe(
      'Video duration must be between 4 and 30 seconds.'
    )
    expect(motionVideoFailureMessage('duration-long', 'recreate')).toBe(
      'The reference video is too long. Choose a clip of 30 seconds or fewer.'
    )
  })
})

describe('recreate submit payload', () => {
  const clipAt = (seconds: number): MotionVideoMetadata => ({
    ...recreateClip,
    browserMetadata: browserMetadata({
      duration: seconds,
      video_duration: seconds,
    }),
    rawDurationSec: seconds,
    durationSec: Math.round(seconds),
  })
  const build = (prompt = '', clip: MotionVideoMetadata = recreateClip) =>
    buildMotionSubmitPayload({
      mode: 'recreate',
      prompt,
      character: characters[0],
      characters,
      video: {
        kind: 'upload',
        url: 'https://files.example.com/reference.mp4',
        fileName: 'reference.mp4',
        metadata: clip,
      },
      resolution: '480p',
    })

  it('sends the raw intent, clip and browser metadata as one queued request', () => {
    const body = build('keep the turn')
    expect(body).toMatchObject({
      mode: 'recreate',
      prompt: 'keep the turn',
      subject_image_urls: [
        'https://files.example.com/front.png',
        'https://files.example.com/side.png',
      ],
      video_url: 'https://files.example.com/reference.mp4',
      video_metadata: recreateClip.browserMetadata,
      duration_sec: 6,
      resolution: '480p',
      aspect_ratio: '9:16',
    })
  })

  it('allows an empty prompt because the intent is optional', () => {
    expect(build('   ')).toMatchObject({ prompt: '' })
  })

  it('rejects a prompt over the public rune ceiling', () => {
    expect(() => build('字'.repeat(4001))).toThrow(
      'Prompt must be 4000 characters or fewer.'
    )
  })

  it('rejects a duration outside the reference-video range', () => {
    expect(build('keep the turn', clipAt(3))).toBeNull()
    expect(build('keep the turn', clipAt(31))).toBeNull()
  })

  it('rounds a fractional clip to the whole output seconds the worker derives', () => {
    expect(build('keep the turn', clipAt(7.6))?.duration_sec).toBe(8)
  })
})
