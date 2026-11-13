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

import type { ModelContract, PricingModel } from '@/features/pricing/types'

import type { ParamControl } from '../../types'
import {
  buildParamSchema,
  CAPABILITY_KEYS,
  defaultParams,
} from '../param-schema'
import { toSubmitBody } from '../submit-body'

function model(overrides: Partial<PricingModel>): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

function imageContract(overrides: Partial<ModelContract>): ModelContract {
  return {
    model_id: 'test-model',
    family: 'image',
    operations: [
      {
        id: 'text_to_image',
        executable: true,
        inputs: [{ slot: 'prompt', type: 'text', required: true }],
        parameters: [],
        output: { type: 'image' },
      },
    ],
    ...overrides,
  }
}

/** The catalog payload the served image models actually carry. */
function gptImageModel(): PricingModel {
  return model({
    model_name: 'gpt-image-2.5',
    tags: 'Image Generation',
    contract: imageContract({
      model_id: 'gpt-image-2.5',
      reference_images: {
        min: 0,
        max: 1,
        formats: ['jpeg', 'png', 'webp'],
        max_bytes_each: 5 * 1024 * 1024,
      },
    }),
  })
}

function referenceControl(controls: ParamControl[]): ParamControl | undefined {
  return controls.find((control) => control.key === 'reference_media')
}

describe('the image-generation reference slot', () => {
  test('renders for a model that declares only the image-generation tag', () => {
    const schema = buildParamSchema(
      model({ model_name: 'gpt-image-2.5', tags: 'Image Generation' })
    )

    const control = referenceControl(schema.controls)

    expect(control?.kind).toBe('media')
    expect(control?.labelKey).toBe('Reference images')
  })

  test('takes the contract ceiling instead of the old hand-written table', () => {
    const schema = buildParamSchema(gptImageModel())

    const control = referenceControl(schema.controls)

    expect(control?.accept).toBe('image/jpeg,image/png,image/webp')
    expect(control?.maxCount).toBe(1)
    expect(control?.maxBytes).toBe(5 * 1024 * 1024)
  })

  test('takes a larger declared ceiling from another contract', () => {
    const schema = buildParamSchema(
      model({
        model_name: 'nano-banana-2-lite',
        tags: 'Image Generation',
        contract: imageContract({
          model_id: 'nano-banana-2-lite',
          reference_images: {
            min: 0,
            max: 14,
            formats: ['image/png', 'image/jpeg', 'image/webp', 'image/heic'],
            max_bytes_each: 7 * 1024 * 1024,
          },
        }),
      })
    )

    const control = referenceControl(schema.controls)

    expect(control?.maxCount).toBe(14)
    expect(control?.accept).toBe('image/png,image/jpeg,image/webp,image/heic')
    expect(control?.maxBytes).toBe(7 * 1024 * 1024)
  })

  test('keeps the one-image fallback when no contract is published', () => {
    const schema = buildParamSchema(
      model({ model_name: 'uncontracted-image', tags: 'Image Generation' })
    )

    expect(referenceControl(schema.controls)?.maxCount).toBe(1)
  })

  test('claims image-to-image only where the slot is offered', () => {
    const schema = buildParamSchema(gptImageModel())

    expect(schema.capabilities).toContain(CAPABILITY_KEYS.textToImage)
    expect(schema.capabilities).toContain(CAPABILITY_KEYS.imageToImage)
  })

  test('leaves a video model with the single reference slot it already had', () => {
    const schema = buildParamSchema(
      model({
        model_name: 'seedance-2-0',
        tags: 'Video Generation,async,task',
      })
    )

    const slots = schema.controls.filter(
      (control) => control.key === 'reference_media'
    )

    expect(slots).toHaveLength(1)
    expect(schema.capabilities).not.toContain(CAPABILITY_KEYS.imageToImage)
  })

  test('carries a video contract reference ceiling onto the flat slot', () => {
    const schema = buildParamSchema(
      model({
        model_name: 'kling-v3',
        tags: 'Video Generation,async,task',
        contract: {
          model_id: 'kling-v3',
          family: 'video',
          reference_images: { min: 0, max: 7 },
        },
      })
    )

    expect(referenceControl(schema.controls)?.maxCount).toBe(7)
  })
})

describe('an image submission carrying a reference', () => {
  test('sends the reference under the field the edit surface defines', () => {
    const schema = buildParamSchema(gptImageModel())
    const params = defaultParams(schema)
    params.reference_media = ['https://cdn/a.png']

    const body = toSubmitBody('gpt-image-2.5', undefined, schema, params)

    expect(body.images).toEqual(['https://cdn/a.png'])
    expect(body).not.toHaveProperty('reference_media')
  })

  test('adds no reference field when the slot is empty', () => {
    const schema = buildParamSchema(gptImageModel())

    const body = toSubmitBody(
      'gpt-image-2.5',
      undefined,
      schema,
      defaultParams(schema)
    )

    expect(body).not.toHaveProperty('images')
    expect(body).not.toHaveProperty('reference_media')
  })
})
