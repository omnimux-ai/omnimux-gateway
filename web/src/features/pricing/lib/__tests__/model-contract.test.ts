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

import type { PricingModel } from '../../types'
import {
  declaredContract,
  declaredFamily,
  hasPlayground,
} from '../model-contract'

function model(contract?: PricingModel['contract']): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    contract,
  }
}

describe('hasPlayground', () => {
  it('mounts the form for a video model', () => {
    expect(hasPlayground(model({ model_id: 'm', family: 'video' }))).toBe(true)
  })

  it('mounts the form for an image model', () => {
    expect(hasPlayground(model({ model_id: 'm', family: 'image' }))).toBe(true)
  })

  it('withholds the form from a text model', () => {
    expect(hasPlayground(model({ model_id: 'm', family: 'llm' }))).toBe(false)
  })

  it('withholds the form from a data model', () => {
    expect(hasPlayground(model({ model_id: 'm', family: 'data' }))).toBe(false)
  })

  it('withholds the form from an audio model', () => {
    expect(hasPlayground(model({ model_id: 'm', family: 'audio' }))).toBe(false)
  })

  it('keeps the previous behaviour when no contract was published', () => {
    expect(hasPlayground(model())).toBe(true)
  })

  it('keeps the previous behaviour when the contract declares no family', () => {
    expect(hasPlayground(model({ model_id: 'm' }))).toBe(true)
  })
})

describe('declaredFamily', () => {
  it('reports the published family', () => {
    expect(declaredFamily(model({ model_id: 'm', family: 'video' }))).toBe(
      'video'
    )
  })

  it('reports nothing when no contract was published', () => {
    expect(declaredFamily(model())).toBeUndefined()
  })
})

describe('declaredContract', () => {
  it('passes the published contract through unchanged', () => {
    const published = { model_id: 'm', family: 'video' as const }
    expect(declaredContract(model(published))).toBe(published)
  })

  it('reports nothing rather than an empty contract', () => {
    expect(declaredContract(model())).toBeUndefined()
  })
})
