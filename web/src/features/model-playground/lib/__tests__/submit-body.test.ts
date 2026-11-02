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

import { buildOperationSchema, buildParamSchema, defaultParams } from '../param-schema'
import { toSubmitBody } from '../submit-body'
import type { PlaygroundParams } from '../../types'

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

function imageModel(): PricingModel {
  return {
    id: 2,
    model_name: 'gpt-image-2.5',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    model_price: 0.01,
    enable_groups: ['default'],
    input_modalities: ['text', 'image'],
    output_modalities: ['image'],
  }
}

describe('toSubmitBody', () => {
  test('moves a first frame into image_with_roles with its role', () => {
    const schema = buildOperationSchema(seedance(), 'first_frame')
    const params: PlaygroundParams = {
      ...defaultParams(schema),
      prompt: 'a kite',
      first_frame: ['https://cdn/first.png'],
    }

    const body = toSubmitBody('seedance-2-5', 'first_frame', schema, params)

    expect(body.image_with_roles).toEqual([
      { url: 'https://cdn/first.png', role: 'first_frame' },
    ])
    expect(body.first_frame).toBeUndefined()
  })

  test('orders the frame pair first then last regardless of slot order', () => {
    const schema = buildOperationSchema(seedance(), 'first_last_frame')
    const params: PlaygroundParams = {
      ...defaultParams(schema),
      last_frame: ['https://cdn/last.png'],
      first_frame: ['https://cdn/first.png'],
    }

    const body = toSubmitBody('seedance-2-5', 'first_last_frame', schema, params)

    expect(body.image_with_roles).toEqual([
      { url: 'https://cdn/first.png', role: 'first_frame' },
      { url: 'https://cdn/last.png', role: 'last_frame' },
    ])
  })

  test('sends reference material to the three url arrays', () => {
    const schema = buildOperationSchema(seedance(), 'video_multi_ref')
    const params: PlaygroundParams = {
      ...defaultParams(schema),
      reference_images: ['https://cdn/a.png', 'https://cdn/b.png'],
      reference_videos: ['https://cdn/clip.mp4'],
      reference_audios: ['https://cdn/track.mp3'],
    }

    const body = toSubmitBody('seedance-2-5', 'video_multi_ref', schema, params)

    expect(body.image_urls).toEqual(['https://cdn/a.png', 'https://cdn/b.png'])
    expect(body.video_urls).toEqual(['https://cdn/clip.mp4'])
    expect(body.audio_urls).toEqual(['https://cdn/track.mp3'])
    expect(body.reference_images).toBeUndefined()
    expect(body.reference_videos).toBeUndefined()
    expect(body.reference_audios).toBeUndefined()
  })

  test('sends the source video to video_urls for edit', () => {
    const schema = buildOperationSchema(seedance(), 'video_edit')
    const params: PlaygroundParams = {
      ...defaultParams(schema),
      source_videos: ['https://cdn/source.mp4'],
    }

    const body = toSubmitBody('seedance-2-5', 'video_edit', schema, params)

    expect(body.video_urls).toEqual(['https://cdn/source.mp4'])
    expect(body.source_videos).toBeUndefined()
  })

  test('carries the operation and the vendor sub-task field for seedance', () => {
    const schema = buildOperationSchema(seedance(), 'video_edit')
    const body = toSubmitBody(
      'seedance-2-5',
      'video_edit',
      schema,
      defaultParams(schema)
    )

    expect(body.operation).toBe('video_edit')
    expect(body.omni_reference_task_type).toBe('edit')
  })

  test('omits the vendor sub-task field for the frame modes', () => {
    const schema = buildOperationSchema(seedance(), 'first_frame')
    const body = toSubmitBody(
      'seedance-2-5',
      'first_frame',
      schema,
      defaultParams(schema)
    )

    expect(body.operation).toBe('first_frame')
    expect(body.omni_reference_task_type).toBeUndefined()
  })

  test('coerces the numeric duration on the way out', () => {
    const schema = buildOperationSchema(seedance(), 'text_to_video')
    const params: PlaygroundParams = {
      ...defaultParams(schema),
      duration: '8',
    }

    const body = toSubmitBody('seedance-2-5', 'text_to_video', schema, params)

    expect(body.duration).toBe(8)
  })

  test('sends duration -1 as a number in video_edit', () => {
    const schema = buildOperationSchema(seedance(), 'video_edit')
    const body = toSubmitBody(
      'seedance-2-5',
      'video_edit',
      schema,
      defaultParams(schema)
    )

    expect(body.duration).toBe(-1)
  })

  test('never rewrites the wan-3.0 model id and drops a legacy quality tier', () => {
    const wanModel: PricingModel = {
      id: 99,
      model_name: 'wan-3.0',
      quota_type: 1,
      model_ratio: 1,
      completion_ratio: 1,
      tags: 'Video Generation,async,task',
      enable_groups: ['wan-3.0-720p', 'wan-3.0-1080p'],
    }
    const schema = buildOperationSchema(wanModel, 'text_to_video')

    // The tier is the request's group, not a second model id: whichever tier a
    // pasted document names, the body keeps the public id and carries no tier.
    for (const tier of ['standard', 'prime']) {
      const body = toSubmitBody('wan-3.0', 'text_to_video', schema, {
        ...defaultParams(schema),
        quality_tier: tier,
      })

      expect(body.quality_tier).toBeUndefined()
      expect(body.model).toBeUndefined()
      expect(JSON.stringify(body)).not.toContain('wan-3.0-prime')
    }
  })

  test('leaves a flat schema without any operation, renaming only the image reference', () => {
    const schema = buildParamSchema(imageModel())
    const params: PlaygroundParams = {
      ...defaultParams(schema),
      prompt: 'a cat',
      reference_media: ['https://cdn/ref.png'],
    }

    const body = toSubmitBody('gpt-image-2.5', undefined, schema, params)

    expect(body.operation).toBeUndefined()
    expect(body.images).toEqual(['https://cdn/ref.png'])
    expect(body).not.toHaveProperty('reference_media')
  })
})
