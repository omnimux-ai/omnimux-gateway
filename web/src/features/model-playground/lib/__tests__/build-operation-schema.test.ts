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

import { buildOperationSchema, defaultParams } from '../param-schema'
import type { ParamControl } from '../../types'

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

function control(schema: { controls: ParamControl[] }, key: string) {
  return schema.controls.find((item) => item.key === key)
}

describe('buildOperationSchema', () => {
  test('renders no media slot for text_to_video', () => {
    const schema = buildOperationSchema(seedance(), 'text_to_video')
    expect(schema.operation).toBe('text_to_video')
    expect(schema.controls.filter((item) => item.kind === 'media')).toEqual([])
  })

  test('renders exactly one frame slot for first_frame', () => {
    const schema = buildOperationSchema(seedance(), 'first_frame')
    const media = schema.controls.filter((item) => item.kind === 'media')
    expect(media.map((item) => item.key)).toEqual(['first_frame'])
    expect(media[0].maxCount).toBe(1)
    expect(media[0].mediaRole).toBe('first_frame')
  })

  test('renders a first and a last frame slot for first_last_frame', () => {
    const schema = buildOperationSchema(seedance(), 'first_last_frame')
    const media = schema.controls.filter((item) => item.kind === 'media')
    expect(media.map((item) => item.key)).toEqual(['first_frame', 'last_frame'])
    expect(media.map((item) => item.mediaRole)).toEqual([
      'first_frame',
      'last_frame',
    ])
  })

  test('carries the model media ceilings onto the reference slots', () => {
    const schema = buildOperationSchema(seedance(), 'video_multi_ref')
    expect(control(schema, 'reference_images')?.maxCount).toBe(30)
    expect(control(schema, 'reference_videos')?.maxCount).toBe(10)
    expect(control(schema, 'reference_audios')?.maxCount).toBe(10)
  })

  test('renders one source-video slot for edit and extend', () => {
    for (const operation of ['video_edit', 'video_extend'] as const) {
      const schema = buildOperationSchema(seedance(), operation)
      const media = schema.controls.filter((item) => item.kind === 'media')
      expect(media.map((item) => item.key)).toEqual(['source_videos'])
      expect(media[0].mediaRole).toBe('reference_video')
    }
  })

  test('locks the aspect ratio in the frame modes and explains why', () => {
    const schema = buildOperationSchema(seedance(), 'first_frame')
    const ratio = control(schema, 'aspect_ratio')
    expect(ratio?.defaultValue).toBe('adaptive')
    expect(ratio?.options).toEqual([{ value: 'adaptive' }])
    expect(ratio?.lockedReasonKey).toBeTruthy()
  })

  test('locks the duration to auto in video_edit', () => {
    const schema = buildOperationSchema(seedance(), 'video_edit')
    const duration = control(schema, 'duration')
    expect(duration?.defaultValue).toBe('-1')
    expect(duration?.options).toEqual([{ value: '-1', labelKey: 'Auto' }])
    expect(duration?.lockedReasonKey).toBeTruthy()
  })

  test('leaves the duration free in video_extend', () => {
    const schema = buildOperationSchema(seedance(), 'video_extend')
    const duration = control(schema, 'duration')
    expect(duration?.lockedReasonKey).toBeUndefined()
    expect((duration?.options ?? []).length).toBeGreaterThan(1)
  })

  test('keeps run-time parameters unlocked in text_to_video', () => {
    const schema = buildOperationSchema(seedance(), 'text_to_video')
    expect(control(schema, 'aspect_ratio')?.lockedReasonKey).toBeUndefined()
    expect(control(schema, 'duration')?.lockedReasonKey).toBeUndefined()
  })
})

describe('defaultParams with a mode schema', () => {
  test('starts every media slot empty and the locked controls pinned', () => {
    const schema = buildOperationSchema(seedance(), 'video_edit')
    const params = defaultParams(schema)
    expect(params.source_videos).toEqual([])
    expect(params.duration).toBe('-1')
    expect(params.aspect_ratio).toBe('adaptive')
  })
})
