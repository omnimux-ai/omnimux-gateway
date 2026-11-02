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

import type { PricingModel } from '@/features/pricing/types'

import { validateOperationMedia } from '../media-validation'
import { buildOperationSchema, defaultParams } from '../param-schema'
import type { PlaygroundOperation, PlaygroundParams } from '../../types'

function seedance(): PricingModel {
  return {
    id: 1,
    model_name: 'seedance-2-5',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    tags: 'Video Generation,async,task',
    output_modalities: ['video'],
  }
}

function paramsFor(
  operation: PlaygroundOperation,
  overrides: PlaygroundParams = {}
): PlaygroundParams {
  const schema = buildOperationSchema(seedance(), operation)
  return { ...defaultParams(schema), ...overrides }
}

describe('validateOperationMedia', () => {
  test('does not block text_to_video, which takes no media', () => {
    expect(validateOperationMedia('text_to_video', paramsFor('text_to_video'))).toBeNull()
  })

  test('reports the missing frame slot for first_frame', () => {
    expect(validateOperationMedia('first_frame', paramsFor('first_frame'))).toEqual({
      code: 'missing_required',
      slotKey: 'first_frame',
    })
  })

  test('passes once the first frame is filled', () => {
    const params = paramsFor('first_frame', { first_frame: ['https://cdn/f.png'] })
    expect(validateOperationMedia('first_frame', params)).toBeNull()
  })

  test('reports the last frame when only the first frame is set', () => {
    const params = paramsFor('first_last_frame', {
      first_frame: ['https://cdn/f.png'],
    })
    expect(validateOperationMedia('first_last_frame', params)).toEqual({
      code: 'missing_required',
      slotKey: 'last_frame',
    })
  })

  test('reports a reference set that has nothing to reference', () => {
    expect(
      validateOperationMedia('video_multi_ref', paramsFor('video_multi_ref'))
    ).toEqual({ code: 'missing_required', slotKey: 'reference_images' })
  })

  test('rejects reference audio without a visual reference', () => {
    const params = paramsFor('video_multi_ref', {
      reference_audios: ['https://cdn/t.mp3'],
    })
    expect(validateOperationMedia('video_multi_ref', params)).toEqual({
      code: 'audio_alone',
    })
  })

  test('accepts reference audio beside a reference image', () => {
    const params = paramsFor('video_multi_ref', {
      reference_images: ['https://cdn/a.png'],
      reference_audios: ['https://cdn/t.mp3'],
    })
    expect(validateOperationMedia('video_multi_ref', params)).toBeNull()
  })

  test('requires the source video for edit and extend', () => {
    for (const operation of ['video_edit', 'video_extend'] as const) {
      expect(validateOperationMedia(operation, paramsFor(operation))).toEqual({
        code: 'missing_required',
        slotKey: 'source_videos',
      })
      const filled = paramsFor(operation, {
        source_videos: ['https://cdn/s.mp4'],
      })
      expect(validateOperationMedia(operation, filled)).toBeNull()
    }
  })
})
