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

import {
  hasOperations,
  operationsFor,
  vendorSubTaskFields,
} from '../operation-matrix'

function operation(
  id: string,
  overrides: Partial<ContractOperation> = {}
): ContractOperation {
  return {
    id,
    executable: true,
    inputs: [],
    parameters: [],
    output: { type: 'video' },
    ...overrides,
  }
}

function model(
  operations: ModelContract['operations'] | undefined,
  modelName = 'test-model'
): PricingModel {
  return {
    id: 1,
    model_name: modelName,
    quota_type: 1,
    model_ratio: 0,
    completion_ratio: 0,
    enable_groups: ['default'],
    tags: 'Video Generation',
    contract: operations
      ? { model_id: modelName, family: 'video', operations }
      : undefined,
  }
}

describe('operationsFor', () => {
  test('returns the executable video operations the contract publishes', () => {
    const subject = model(
      [
        operation('text_to_video'),
        operation('first_frame'),
        operation('first_last_frame'),
        operation('video_multi_ref'),
        operation('video_edit'),
        operation('video_extend'),
      ],
      'seedance-2-5'
    )

    expect(operationsFor(subject)).toEqual([
      'text_to_video',
      'first_frame',
      'first_last_frame',
      'video_multi_ref',
      'video_edit',
      'video_extend',
    ])
  })

  test('skips non-executable operations', () => {
    const subject = model([
      operation('text_to_video', { executable: false }),
      operation('video_multi_ref'),
    ])

    expect(operationsFor(subject)).toEqual(['video_multi_ref'])
  })

  test('skips operations whose output is not video', () => {
    const subject = model([
      operation('text_to_video'),
      operation('text_to_image', { output: { type: 'image' } }),
    ])

    expect(operationsFor(subject)).toEqual(['text_to_video'])
  })

  test('skips operations outside the playground vocabulary', () => {
    const subject = model([
      operation('text_to_video'),
      operation('motion_control'),
    ])

    expect(operationsFor(subject)).toEqual(['text_to_video'])
  })

  test('returns nothing for a v1 string list or a model without a contract', () => {
    expect(operationsFor(model(['text_to_video'], 'seedance-2-5'))).toEqual([])
    expect(operationsFor(model(undefined))).toEqual([])
  })

  test('deduplicates repeated operation ids', () => {
    const subject = model([
      operation('text_to_video'),
      operation('text_to_video'),
    ])

    expect(operationsFor(subject)).toEqual(['text_to_video'])
  })

  test('reports whether a model declares any mode', () => {
    expect(hasOperations(model([operation('first_frame')]))).toBe(true)
    expect(hasOperations(model(undefined))).toBe(false)
  })
})

describe('vendorSubTaskFields', () => {
  test('maps the seedance-2-5 reference family onto omni_reference_task_type', () => {
    expect(vendorSubTaskFields('seedance-2-5', 'video_multi_ref')).toEqual({
      omni_reference_task_type: 'reference',
    })
    expect(vendorSubTaskFields('seedance-2-5', 'video_edit')).toEqual({
      omni_reference_task_type: 'edit',
    })
    expect(vendorSubTaskFields('seedance-2-5', 'video_extend')).toEqual({
      omni_reference_task_type: 'extend',
    })
  })

  test('omits the field for the frame and text modes', () => {
    expect(vendorSubTaskFields('seedance-2-5', 'text_to_video')).toEqual({})
    expect(vendorSubTaskFields('seedance-2-5', 'first_frame')).toEqual({})
    expect(vendorSubTaskFields('seedance-2-5', 'first_last_frame')).toEqual({})
  })

  test('maps wan-3.0 onto generation_type instead', () => {
    expect(vendorSubTaskFields('wan-3.0', 'first_frame')).toEqual({
      generation_type: 'frame',
    })
    expect(vendorSubTaskFields('wan-3.0', 'video_multi_ref')).toEqual({
      generation_type: 'reference',
    })
  })

  test('sends nothing for a model that carries neither field', () => {
    expect(vendorSubTaskFields('minimax-h3', 'video_multi_ref')).toEqual({})
  })
})
