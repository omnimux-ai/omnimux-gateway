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

import type {
  ContractOperation,
  ModelContract,
  PricingModel,
} from '@/features/pricing/types'

import type { ParamControl } from '../../types'
import { buildOperationSchema, defaultParams } from '../param-schema'

const IMAGE_MIMES = ['image/jpeg', 'image/png', 'image/webp']
const VIDEO_MIMES = ['video/mp4', 'video/quicktime']

const VIDEO_PARAMETERS = [
  {
    name: 'duration',
    type: 'integer' as const,
    minimum: 4,
    maximum: 30,
    step: 1,
  },
  {
    name: 'resolution',
    type: 'enum' as const,
    enum: ['480p', '720p', '1080p'],
  },
  {
    name: 'aspect_ratio',
    type: 'enum' as const,
    enum: ['16:9', '9:16', '1:1'],
  },
  { name: 'generate_audio', type: 'boolean' as const, default: false },
]

function fixedAdaptive(): ContractOperation['parameters'] {
  return [
    {
      name: 'aspect_ratio',
      type: 'enum',
      enum: ['adaptive'],
      default: 'adaptive',
      fixed: 'adaptive',
    },
  ]
}

function seedanceContract(): ModelContract {
  return {
    model_id: 'seedance-2-5',
    family: 'video',
    prompt_max_chars: 5000,
    operations: [
      {
        id: 'text_to_video',
        executable: true,
        inputs: [{ slot: 'prompt', type: 'text', required: true }],
        parameters: VIDEO_PARAMETERS,
        output: { type: 'video' },
      },
      {
        id: 'first_frame',
        executable: true,
        inputs: [
          { slot: 'prompt', type: 'text', required: true },
          {
            slot: 'first_frame',
            type: 'image',
            role: 'first_frame',
            required: true,
            min: 1,
            max: 1,
            allowedMimes: IMAGE_MIMES,
            maxSizeMb: 30,
          },
        ],
        parameters: [
          VIDEO_PARAMETERS[0],
          VIDEO_PARAMETERS[1],
          ...fixedAdaptive(),
          VIDEO_PARAMETERS[3],
        ],
        output: { type: 'video' },
      },
      {
        id: 'first_last_frame',
        executable: true,
        inputs: [
          { slot: 'prompt', type: 'text', required: true },
          {
            slot: 'first_frame',
            type: 'image',
            role: 'first_frame',
            required: true,
            min: 1,
            max: 1,
            allowedMimes: IMAGE_MIMES,
            maxSizeMb: 30,
          },
          {
            slot: 'last_frame',
            type: 'image',
            role: 'last_frame',
            required: true,
            min: 1,
            max: 1,
            allowedMimes: IMAGE_MIMES,
            maxSizeMb: 30,
          },
        ],
        parameters: [
          VIDEO_PARAMETERS[0],
          VIDEO_PARAMETERS[1],
          ...fixedAdaptive(),
          VIDEO_PARAMETERS[3],
        ],
        output: { type: 'video' },
      },
      {
        id: 'video_multi_ref',
        executable: true,
        inputs: [
          { slot: 'prompt', type: 'text', required: true },
          {
            slot: 'reference_images',
            type: 'image',
            role: 'reference',
            required: false,
            min: 1,
            max: 30,
            allowedMimes: IMAGE_MIMES,
            maxSizeMb: 30,
          },
          {
            slot: 'reference_videos',
            type: 'video',
            role: 'reference',
            required: false,
            min: 0,
            max: 10,
            allowedMimes: VIDEO_MIMES,
            maxSizeMb: 200,
          },
          {
            slot: 'reference_audios',
            type: 'audio',
            role: 'reference',
            required: false,
            min: 0,
            max: 10,
            allowedMimes: ['audio/wav', 'audio/mpeg'],
            maxSizeMb: 15,
          },
        ],
        parameters: VIDEO_PARAMETERS,
        output: { type: 'video' },
      },
      {
        id: 'video_edit',
        executable: true,
        inputs: [
          { slot: 'prompt', type: 'text', required: true },
          {
            slot: 'source_videos',
            type: 'video',
            role: 'source',
            required: true,
            min: 1,
            max: 1,
            allowedMimes: VIDEO_MIMES,
            maxSizeMb: 200,
          },
        ],
        parameters: [
          {
            name: 'duration',
            type: 'integer',
            minimum: -1,
            maximum: -1,
            default: -1,
            fixed: -1,
          },
          VIDEO_PARAMETERS[1],
          ...fixedAdaptive(),
          VIDEO_PARAMETERS[3],
        ],
        output: { type: 'video' },
      },
      {
        id: 'video_extend',
        executable: true,
        inputs: [
          { slot: 'prompt', type: 'text', required: true },
          {
            slot: 'source_videos',
            type: 'video',
            role: 'source',
            required: true,
            min: 1,
            max: 1,
            allowedMimes: VIDEO_MIMES,
            maxSizeMb: 200,
          },
        ],
        parameters: [
          VIDEO_PARAMETERS[0],
          VIDEO_PARAMETERS[1],
          ...fixedAdaptive(),
          VIDEO_PARAMETERS[3],
        ],
        output: { type: 'video' },
      },
    ],
  }
}

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
    contract: seedanceContract(),
  }
}

function control(schema: { controls: ParamControl[] }, key: string) {
  return schema.controls.find((item) => item.key === key)
}

describe('buildOperationSchema from a contract operation', () => {
  test('renders no media slot for text_to_video', () => {
    const schema = buildOperationSchema(seedance(), 'text_to_video')
    expect(schema.operation).toBe('text_to_video')
    expect(schema.controls.filter((item) => item.kind === 'media')).toEqual([])
  })

  test('carries the declared prompt ceiling', () => {
    const schema = buildOperationSchema(seedance(), 'text_to_video')
    expect(control(schema, 'prompt')?.maxChars).toBe(5000)
  })

  test('renders exactly one frame slot for first_frame', () => {
    const schema = buildOperationSchema(seedance(), 'first_frame')
    const media = schema.controls.filter((item) => item.kind === 'media')
    expect(media.map((item) => item.key)).toEqual(['first_frame'])
    expect(media[0].maxCount).toBe(1)
    expect(media[0].mediaRole).toBe('first_frame')
    expect(media[0].accept).toBe('image/jpeg,image/png,image/webp')
    expect(media[0].maxBytes).toBe(30 * 1024 * 1024)
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

  test('carries the contract media ceilings onto the reference slots', () => {
    const schema = buildOperationSchema(seedance(), 'video_multi_ref')
    expect(control(schema, 'reference_images')?.maxCount).toBe(30)
    expect(control(schema, 'reference_videos')?.maxCount).toBe(10)
    expect(control(schema, 'reference_audios')?.maxCount).toBe(10)
    expect(control(schema, 'reference_audios')?.maxBytes).toBe(15 * 1024 * 1024)
  })

  test('renders one source-video slot for edit and extend', () => {
    for (const operation of ['video_edit', 'video_extend'] as const) {
      const schema = buildOperationSchema(seedance(), operation)
      const media = schema.controls.filter((item) => item.kind === 'media')
      expect(media.map((item) => item.key)).toEqual(['source_videos'])
      expect(media[0].mediaRole).toBe('reference_video')
    }
  })

  test('locks a fixed aspect ratio and says it is the contract', () => {
    const schema = buildOperationSchema(seedance(), 'first_frame')
    const ratio = control(schema, 'aspect_ratio')
    expect(ratio?.defaultValue).toBe('adaptive')
    expect(ratio?.options).toEqual([{ value: 'adaptive' }])
    expect(ratio?.lockedReasonKey).toBe('Locked by the service contract')
  })

  test('locks a fixed duration to auto and labels the sentinel', () => {
    const schema = buildOperationSchema(seedance(), 'video_edit')
    const duration = control(schema, 'duration')
    expect(duration?.defaultValue).toBe('-1')
    expect(duration?.options).toEqual([{ value: '-1', labelKey: 'Auto' }])
    expect(duration?.lockedReasonKey).toBe('Locked by the service contract')
  })

  test('leaves a bounded duration free in video_extend', () => {
    const schema = buildOperationSchema(seedance(), 'video_extend')
    const duration = control(schema, 'duration')
    expect(duration?.kind).toBe('slider')
    expect(duration?.min).toBe(4)
    expect(duration?.max).toBe(30)
    expect(duration?.lockedReasonKey).toBeUndefined()
  })

  test('keeps run-time parameters unlocked in text_to_video', () => {
    const schema = buildOperationSchema(seedance(), 'text_to_video')
    expect(control(schema, 'aspect_ratio')?.lockedReasonKey).toBeUndefined()
    expect(control(schema, 'duration')?.lockedReasonKey).toBeUndefined()
  })

  test('renders every declared parameter and no undeclared control', () => {
    const schema = buildOperationSchema(seedance(), 'text_to_video')
    expect(schema.controls.map((item) => item.key)).toEqual([
      'prompt',
      'duration',
      'resolution',
      'aspect_ratio',
      'generate_audio',
    ])
  })
})

describe('buildOperationSchema fallback without a contract operation', () => {
  test('keeps the previous operation template', () => {
    const legacy: PricingModel = {
      id: 1,
      model_name: 'legacy-video',
      quota_type: 0,
      model_ratio: 1,
      completion_ratio: 1,
      enable_groups: ['default'],
      tags: 'Video Generation',
      output_modalities: ['video'],
    }
    const schema = buildOperationSchema(legacy, 'first_frame')
    const media = schema.controls.filter((item) => item.kind === 'media')
    expect(media.map((item) => item.key)).toEqual(['first_frame'])
    expect(media[0].maxCount).toBe(1)
    expect(control(schema, 'aspect_ratio')?.defaultValue).toBe('adaptive')
    expect(control(schema, 'aspect_ratio')?.lockedReasonKey).toBeTruthy()
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
