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
along with this program. If not, see <http://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, it } from 'vitest'

import type { ContractInput, ModelContract } from '@/features/pricing/types'

import {
  referenceLimitsFrom,
  validateOperationMedia,
} from '../media-validation'

const contractWith = (
  images?: ModelContract['reference_images']
): ModelContract => ({
  model_id: 'm',
  family: 'video',
  reference_images: images,
})

const REFERENCE_INPUTS: ContractInput[] = [
  { slot: 'prompt', type: 'text', required: true },
  {
    slot: 'reference_images',
    type: 'image',
    role: 'reference',
    required: false,
    min: 1,
    max: 2,
  },
]

describe('referenceLimitsFrom', () => {
  it('reports nothing when no contract is published', () => {
    expect(referenceLimitsFrom(undefined)).toEqual({})
  })

  it('reports nothing for a spec that established no limit', () => {
    expect(referenceLimitsFrom(contractWith({}))).toEqual({})
  })

  it('carries the declared ceiling', () => {
    expect(referenceLimitsFrom(contractWith({ min: 1, max: 9 }))).toEqual({
      reference_images: { min: 1, max: 9 },
    })
  })
})

describe('validateOperationMedia with declared limits', () => {
  it('refuses more references than the model accepts', () => {
    const issue = validateOperationMedia(
      REFERENCE_INPUTS,
      { reference_images: ['a', 'b', 'c'] },
      'video_multi_ref'
    )
    expect(issue).toEqual({
      code: 'too_many',
      slotKey: 'reference_images',
      max: 2,
    })
  })

  it('accepts a set inside the declared range', () => {
    expect(
      validateOperationMedia(
        REFERENCE_INPUTS,
        { reference_images: ['a', 'b'] },
        'video_multi_ref'
      )
    ).toBeNull()
  })

  it('leaves an untouched optional slot to the required check', () => {
    expect(
      validateOperationMedia(
        [{ slot: 'prompt', type: 'text', required: true }],
        {},
        'text_to_video'
      )
    ).toBeNull()
  })

  it('disables submit when the declared ceiling is exceeded', () => {
    const issue = validateOperationMedia(
      REFERENCE_INPUTS,
      { reference_images: ['a', 'b', 'c'] },
      'video_multi_ref'
    )
    expect(issue).not.toBeNull()
  })
})
