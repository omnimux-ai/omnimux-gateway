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
  buildMotionSubmitPayload,
  parseStoredMotionPayload,
} from '../lib/submit-payload'

/**
 * Simulated inputs that mirror a filled form: a character photo and a driving
 * clip the user uploaded, plus the same form once they pick a built-in
 * template instead of an upload.
 */
const UPLOADED = {
  prompt: '  the same person walks forward and turns to wave  ',
  character: {
    url: 'https://files.example.com/face-01.png',
    fileName: 'face.png',
  },
  video: {
    kind: 'upload' as const,
    url: 'https://files.example.com/clip-01.mp4',
    fileName: 'clip.mp4',
    metadata: {
      version: 1 as const,
      browserMetadata: {
        duration: 10,
        video_duration: 10,
        width: 720,
        height: 1280,
        frame_rate: 30,
        video_codec: 'h264',
        audio_codec: 'aac',
        container: 'mp4',
        size_bytes: 1024,
      },
      rawDurationSec: 10,
      durationSec: 10,
      width: 720,
      height: 1280,
      aspectRatio: '9:16' as const,
    },
  },
  durationSec: 10,
  resolution: '720p',
  outputAspect: '9:16' as const,
}

const PRESET = {
  ...UPLOADED,
  video: {
    kind: 'preset' as const,
    presetId: 'motion-club-dance',
    nameKey: 'Club dance',
  },
}

describe('historical motion payload', () => {
  it('restores an object-edit record with adaptive framing and the original duration', () => {
    const body = buildMotionSubmitPayload({ ...UPLOADED, mode: 'replace' })
    expect(body).toMatchObject({ aspect_ratio: '9:16', duration_sec: 10 })
    expect(parseStoredMotionPayload(JSON.stringify(body))).toEqual(body)
  })
  it('restores a local accepted record with aspect_ratio before server enrichment', () => {
    const body = buildMotionSubmitPayload({ ...UPLOADED, mode: 'motion' })
    expect(body).not.toBeNull()
    expect(parseStoredMotionPayload(JSON.stringify(body))).toEqual(body)
  })
  it('keeps stored size authoritative when a record also carries aspect_ratio', () => {
    const body = buildMotionSubmitPayload({ ...UPLOADED, mode: 'motion' })
    expect(
      parseStoredMotionPayload(JSON.stringify({ ...body, size: '16:9' }))
        ?.aspect_ratio
    ).toBe('16:9')
  })
})

describe('buildMotionSubmitPayload — simulated form → gateway body', () => {
  it('rejects an overlong combined motion prompt without truncating it', () => {
    expect(() =>
      buildMotionSubmitPayload({
        ...UPLOADED,
        mode: 'motion',
        prompt: '字'.repeat(4001),
      })
    ).toThrow('Prompt must be 4000 characters or fewer.')
  })

  it('accepts 4000 Unicode code points including supplementary characters', () => {
    const prompt = '🎨'.repeat(4000)
    expect(
      buildMotionSubmitPayload({ ...UPLOADED, mode: 'motion', prompt })?.prompt
    ).toBe(prompt)
  })

  it('maps an uploaded clip to the gateway body in motion mode', () => {
    expect(buildMotionSubmitPayload({ mode: 'motion', ...UPLOADED })).toEqual({
      mode: 'motion',
      prompt: 'the same person walks forward and turns to wave',
      subject_image_url: 'https://files.example.com/face-01.png',
      subject_image_urls: ['https://files.example.com/face-01.png'],
      video_url: 'https://files.example.com/clip-01.mp4',
      duration_sec: 10,
      aspect_ratio: '9:16',
      resolution: '720p',
      video_metadata: UPLOADED.video.metadata.browserMetadata,
    })
  })

  it('maps multiple characters to subject_image_urls and sets the first as subject_image_url', () => {
    const chars = [
      { url: 'https://files.example.com/face-01.png', fileName: 'face-01.png' },
      { url: 'https://files.example.com/face-02.png', fileName: 'face-02.png' },
      { url: 'https://files.example.com/face-03.png', fileName: 'face-03.png' },
    ]
    const out = buildMotionSubmitPayload({
      mode: 'motion',
      ...UPLOADED,
      character: chars[0],
      characters: chars,
    })
    expect(out?.subject_image_url).toBe('https://files.example.com/face-01.png')
    expect(out?.subject_image_urls).toEqual([
      'https://files.example.com/face-01.png',
      'https://files.example.com/face-02.png',
      'https://files.example.com/face-03.png',
    ])
  })

  it('preserves the original frame and duration for object swap', () => {
    const out = buildMotionSubmitPayload({
      mode: 'replace',
      ...UPLOADED,
      resolution: '480p',
    })
    expect(out?.mode).toBe('replace')
    expect(out?.aspect_ratio).toBe('9:16')
    expect(out?.duration_sec).toBe(10)
    expect(out?.resolution).toBe('480p')
    expect(out?.video_url).toBe('https://files.example.com/clip-01.mp4')
  })

  it('refuses a built-in template, which has no public clip yet', () => {
    expect(buildMotionSubmitPayload({ mode: 'motion', ...PRESET })).toBeNull()
  })

  it('maps a library clip (remote URL) to the same body as an upload', () => {
    const out = buildMotionSubmitPayload({
      mode: 'motion',
      ...UPLOADED,
      video: {
        kind: 'remote' as const,
        url: 'https://cdn.example.com/community-presets/abc.mp4',
        name: 'Dance Duo',
        metadata: UPLOADED.video.metadata,
      },
    })
    expect(out?.video_url).toBe(
      'https://cdn.example.com/community-presets/abc.mp4'
    )
    expect(out?.mode).toBe('motion')
  })

  it('refuses a remote URL until actual metadata is available', () => {
    expect(
      buildMotionSubmitPayload({
        mode: 'motion',
        ...UPLOADED,
        video: {
          kind: 'remote',
          url: 'https://cdn.example/clip.mp4',
          name: 'Clip',
        },
      })
    ).toBeNull()
  })

  it('derives duration from the reference regardless of legacy form fields', () => {
    const out = buildMotionSubmitPayload({
      mode: 'motion',
      ...UPLOADED,
    })
    expect(out?.duration_sec).toBe(10)
    expect(out?.aspect_ratio).toBe('9:16')
  })

  it('rejects fabricated metadata with out-of-range raw duration', () => {
    expect(
      buildMotionSubmitPayload({
        mode: 'motion',
        ...UPLOADED,
        video: {
          ...UPLOADED.video,
          metadata: {
            ...UPLOADED.video.metadata,
            browserMetadata: {
              ...UPLOADED.video.metadata.browserMetadata,
              duration: 30.01,
              video_duration: 30.01,
            },
            rawDurationSec: 30.01,
          },
        },
      })
    ).toBeNull()
  })

  it.each(['motion', 'replace'] as const)(
    'rejects a legal 5:4 reference that the %s route cannot preserve',
    (mode) => {
      const raw = {
        ...UPLOADED.video.metadata.browserMetadata,
        width: 1200,
        height: 960,
      }
      const video = {
        ...UPLOADED.video,
        metadata: {
          ...UPLOADED.video.metadata,
          browserMetadata: raw,
          width: 1200,
          height: 960,
          aspectRatio: null,
        },
      }
      expect(buildMotionSubmitPayload({ ...UPLOADED, mode, video })).toBeNull()
    }
  )
  it.each(['motion', 'replace'] as const)(
    'preserves 3:4 at 480p but never silently changes it at 720p for %s',
    (mode) => {
      const raw = {
        ...UPLOADED.video.metadata.browserMetadata,
        width: 960,
        height: 1280,
      }
      const video = {
        ...UPLOADED.video,
        metadata: {
          ...UPLOADED.video.metadata,
          browserMetadata: raw,
          width: 960,
          height: 1280,
          aspectRatio: '3:4' as const,
        },
      }
      expect(
        buildMotionSubmitPayload({
          ...UPLOADED,
          mode,
          video,
          resolution: '480p',
        })
      ).toMatchObject({ aspect_ratio: '3:4', duration_sec: 10 })
      expect(
        buildMotionSubmitPayload({
          ...UPLOADED,
          mode,
          video,
          resolution: '720p',
        })
      ).toBeNull()
    }
  )
  it.each([2, 3.999])(
    'refuses a %s-second raw duration that cannot reach the four-second output minimum',
    (duration) => {
      const raw = {
        ...UPLOADED.video.metadata.browserMetadata,
        duration,
        video_duration: duration,
      }
      const video = {
        ...UPLOADED.video,
        metadata: {
          ...UPLOADED.video.metadata,
          browserMetadata: raw,
          rawDurationSec: duration,
          durationSec: Math.round(duration),
        },
      }
      expect(
        buildMotionSubmitPayload({ ...UPLOADED, mode: 'motion', video })
      ).toBeNull()
    }
  )
  it('requests the rounded whole second for a consistent fractional duration', () => {
    const raw = {
      ...UPLOADED.video.metadata.browserMetadata,
      duration: 7.2,
      video_duration: 7.2,
    }
    const video = {
      ...UPLOADED.video,
      metadata: {
        ...UPLOADED.video.metadata,
        browserMetadata: raw,
        rawDurationSec: 7.2,
        durationSec: 7,
      },
    }
    expect(
      buildMotionSubmitPayload({ ...UPLOADED, mode: 'motion', video })
    ).toMatchObject({ duration_sec: 7, video_metadata: raw })
  })
  it.each(['motion', 'replace'] as const)(
    'submits 6 seconds for the reported 5.708333/5.717007 reference in %s mode',
    (mode) => {
      const raw = Object.freeze({
        ...UPLOADED.video.metadata.browserMetadata,
        duration: 5.717007,
        video_duration: 5.708333,
        width: 720,
        height: 1280,
        frame_rate: 24,
        size_bytes: 2863940,
      })
      const video = {
        ...UPLOADED.video,
        metadata: {
          ...UPLOADED.video.metadata,
          browserMetadata: raw,
          rawDurationSec: 5.717007,
          durationSec: 6,
          width: 720,
          height: 1280,
        },
      }
      const body = buildMotionSubmitPayload({ ...UPLOADED, mode, video })
      expect(body).toMatchObject({
        duration_sec: 6,
        aspect_ratio: '9:16',
        video_metadata: raw,
      })
      expect(body?.video_metadata).toBe(raw)
      expect(raw).toMatchObject({
        duration: 5.717007,
        video_duration: 5.708333,
      })
    }
  )
  it.each(['motion', 'replace'] as const)(
    'submits 7 seconds for the reported supported audio tail in %s mode without rewriting raw durations',
    (mode) => {
      const raw = Object.freeze({
        ...UPLOADED.video.metadata.browserMetadata,
        duration: 7.072,
        video_duration: 7.041667,
        width: 480,
        height: 854,
        frame_rate: 24,
        size_bytes: 6650581,
      })
      const video = {
        ...UPLOADED.video,
        metadata: {
          ...UPLOADED.video.metadata,
          browserMetadata: raw,
          rawDurationSec: 7.072,
          durationSec: 7,
          width: 480,
          height: 854,
        },
      }
      const body = buildMotionSubmitPayload({ ...UPLOADED, mode, video })
      expect(body).toMatchObject({
        duration_sec: 7,
        aspect_ratio: '9:16',
        video_metadata: raw,
      })
      expect(body?.video_metadata).toBe(raw)
      expect(raw).toMatchObject({ duration: 7.072, video_duration: 7.041667 })
    }
  )
  it('refuses an old summary without complete current browser metadata', () => {
    const {
      browserMetadata: _raw,
      version: _version,
      ...metadata
    } = UPLOADED.video.metadata
    expect(
      buildMotionSubmitPayload({
        ...UPLOADED,
        mode: 'motion',
        video: { ...UPLOADED.video, metadata },
      })
    ).toBeNull()
  })

  it('returns null when a required slot is empty', () => {
    expect(
      buildMotionSubmitPayload({ mode: 'motion', ...UPLOADED, character: null })
    ).toBeNull()
    expect(
      buildMotionSubmitPayload({ mode: 'motion', ...UPLOADED, video: null })
    ).toBeNull()
  })
})
