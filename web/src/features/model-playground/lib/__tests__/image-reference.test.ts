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

import {
  buildParamSchema,
  CAPABILITY_KEYS,
  defaultParams,
} from '../param-schema'
import { toSubmitBody } from '../submit-body'
import type { ParamControl } from '../../types'

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

/** The catalog payload the served image models actually carry. */
function imageModel(): PricingModel {
  return model({ model_name: 'gpt-image-2.5', tags: 'Image Generation' })
}

function referenceControl(controls: ParamControl[]): ParamControl | undefined {
  return controls.find((control) => control.key === 'reference_media')
}

describe('the image-generation reference slot', () => {
  test('renders for a model that declares only the image-generation tag', () => {
    const schema = buildParamSchema(imageModel())

    const control = referenceControl(schema.controls)

    expect(control?.kind).toBe('media')
    expect(control?.labelKey).toBe('Reference images')
  })

  test('carries the model’s own formats and ceiling', () => {
    const schema = buildParamSchema(imageModel())

    const control = referenceControl(schema.controls)

    expect(control?.accept).toBe('image/png,image/jpeg,image/webp')
    // OpenAI documents 16 images; it publishes no per-reference-file byte cap,
    // so none is asserted.
    expect(control?.maxCount).toBe(16)
    expect(control?.maxBytes).toBeUndefined()
  })

  test('carries each vendor’s own documented ceiling', () => {
    const ceilings: [string, number][] = [
      ['nano-banana-pro', 14],
      ['nano-banana-2', 14],
      ['seedream-5-0-pro', 10],
      ['qwen-image-3-0', 3],
    ]

    for (const [modelName, expected] of ceilings) {
      const schema = buildParamSchema(
        model({ model_name: modelName, tags: 'Image Generation' })
      )

      expect(referenceControl(schema.controls)?.maxCount, modelName).toBe(
        expected
      )
    }
  })

  test('carries the documented per-file ceiling where the vendor publishes one', () => {
    const schema = buildParamSchema(
      model({ model_name: 'qwen-image-3-0', tags: 'Image Generation' })
    )

    expect(referenceControl(schema.controls)?.maxBytes).toBe(10 * 1024 * 1024)
  })

  test('claims image-to-image only where the slot is offered', () => {
    const schema = buildParamSchema(imageModel())

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
})

describe('an image submission carrying a reference', () => {
  test('sends the reference under the field the edit surface defines', () => {
    const schema = buildParamSchema(imageModel())
    const params = defaultParams(schema)
    params.reference_media = ['https://cdn/a.png']

    const body = toSubmitBody('gpt-image-2.5', undefined, schema, params)

    expect(body.images).toEqual(['https://cdn/a.png'])
    expect(body).not.toHaveProperty('reference_media')
  })

  test('adds no reference field when the slot is empty', () => {
    const schema = buildParamSchema(imageModel())

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
