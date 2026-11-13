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
import { describe, expect, test } from 'vitest'

import type { ContractInput } from '@/features/pricing/types'

import type { PlaygroundOperation, PlaygroundParams } from '../../types'
import { validateOperationMedia } from '../media-validation'

const PROMPT: ContractInput = { slot: 'prompt', type: 'text', required: true }

const FIRST_FRAME: ContractInput[] = [
  PROMPT,
  {
    slot: 'first_frame',
    type: 'image',
    role: 'first_frame',
    required: true,
    min: 1,
    max: 1,
  },
]

const FIRST_LAST_FRAME: ContractInput[] = [
  ...FIRST_FRAME,
  {
    slot: 'last_frame',
    type: 'image',
    role: 'last_frame',
    required: true,
    min: 1,
    max: 1,
  },
]

const VIDEO_MULTI_REF: ContractInput[] = [
  PROMPT,
  {
    slot: 'reference_images',
    type: 'image',
    role: 'reference',
    required: false,
    min: 1,
    max: 30,
  },
  {
    slot: 'reference_videos',
    type: 'video',
    role: 'reference',
    required: false,
    min: 0,
    max: 10,
  },
  {
    slot: 'reference_audios',
    type: 'audio',
    role: 'reference',
    required: false,
    min: 0,
    max: 10,
  },
]

const VIDEO_SOURCE: ContractInput[] = [
  PROMPT,
  {
    slot: 'source_videos',
    type: 'video',
    role: 'source',
    required: true,
    min: 1,
    max: 1,
  },
]

function validate(
  inputs: ContractInput[],
  params: PlaygroundParams,
  operation?: PlaygroundOperation
) {
  return validateOperationMedia(inputs, params, operation)
}

describe('validateOperationMedia', () => {
  test('does not block text_to_video, which takes no media', () => {
    expect(validate([PROMPT], {})).toBeNull()
  })

  test('reports the missing frame slot for first_frame', () => {
    expect(validate(FIRST_FRAME, {})).toEqual({
      code: 'missing_required',
      slotKey: 'first_frame',
    })
  })

  test('passes once the first frame is filled', () => {
    expect(
      validate(FIRST_FRAME, { first_frame: ['https://cdn/f.png'] })
    ).toBeNull()
  })

  test('reports the last frame when only the first frame is set', () => {
    expect(
      validate(FIRST_LAST_FRAME, { first_frame: ['https://cdn/f.png'] })
    ).toEqual({ code: 'missing_required', slotKey: 'last_frame' })
  })

  test('reports a reference set that has nothing to reference', () => {
    expect(validate(VIDEO_MULTI_REF, {}, 'video_multi_ref')).toEqual({
      code: 'missing_required',
      slotKey: 'reference_images',
    })
  })

  test('rejects reference audio without a visual reference', () => {
    expect(
      validate(
        VIDEO_MULTI_REF,
        { reference_audios: ['https://cdn/t.mp3'] },
        'video_multi_ref'
      )
    ).toEqual({ code: 'audio_alone' })
  })

  test('accepts reference audio beside a reference image', () => {
    expect(
      validate(
        VIDEO_MULTI_REF,
        {
          reference_images: ['https://cdn/a.png'],
          reference_audios: ['https://cdn/t.mp3'],
        },
        'video_multi_ref'
      )
    ).toBeNull()
  })

  test('refuses more references than the contract ceiling', () => {
    expect(
      validate(
        VIDEO_MULTI_REF,
        {
          reference_videos: Array.from(
            { length: 11 },
            (_, index) => `https://cdn/${index}.mp4`
          ),
        },
        'video_multi_ref'
      )
    ).toEqual({ code: 'too_many', slotKey: 'reference_videos', max: 10 })
  })

  test('refuses a partial set below the contract minimum', () => {
    expect(
      validate(
        [
          PROMPT,
          {
            slot: 'reference_images',
            type: 'image',
            required: false,
            min: 2,
            max: 4,
          },
        ],
        { reference_images: ['https://cdn/a.png'] },
        'video_multi_ref'
      )
    ).toEqual({ code: 'too_few', slotKey: 'reference_images', min: 2 })
  })

  test('requires the source video for edit and extend', () => {
    for (const operation of ['video_edit', 'video_extend'] as const) {
      expect(validate(VIDEO_SOURCE, {}, operation)).toEqual({
        code: 'missing_required',
        slotKey: 'source_videos',
      })
      expect(
        validate(
          VIDEO_SOURCE,
          { source_videos: ['https://cdn/s.mp4'] },
          operation
        )
      ).toBeNull()
    }
  })

  test('requires a declared URL slot to be filled', () => {
    const inputs: ContractInput[] = [
      { slot: 'url', type: 'url', required: true },
    ]
    expect(validate(inputs, { url: '' })).toEqual({
      code: 'missing_required',
      slotKey: 'url',
    })
    expect(validate(inputs, { url: 'https://example.com' })).toBeNull()
  })
})
