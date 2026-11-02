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
import { describe, expect, test } from 'vitest'

import {
  BILLING_DECLARATIONS,
  billingDeclarationFor,
  billingDimensionFor,
  durationEnvelopeFor,
  inferredDimensionFor,
} from '../billing-dimension'

describe('inferredDimensionFor', () => {
  test('reads a -task id as per-call, the repo rule for a per-call video id', () => {
    expect(inferredDimensionFor('seedance-2-5-task')).toBe('per_call')
  })

  test('keeps the historical per-second shape for an id without the suffix', () => {
    expect(inferredDimensionFor('seedance-2-5')).toBe('per_second')
  })

  test('reads the dotted spelling of a -task id as per-call', () => {
    expect(inferredDimensionFor('seedance-2.5-task')).toBe('per_call')
  })

  test('ignores surrounding whitespace and letter case', () => {
    expect(inferredDimensionFor('  SEEDANCE-2-5-TASK ')).toBe('per_call')
  })

  test('does not read a -task id inside the name as a per-call suffix', () => {
    expect(inferredDimensionFor('task-runner-2-5')).toBe('per_second')
  })
})

describe('billingDimensionFor', () => {
  test('uses the enrolled declaration when the model has one', () => {
    expect(billingDimensionFor('seedance-2-0-task')).toBe('per_call')
  })

  test('falls back to inference for a model that is not enrolled', () => {
    expect(billingDimensionFor('seedance-2-5')).toBe('per_second')
  })

  test('resolves an enrolled model through its dotted spelling', () => {
    expect(billingDeclarationFor('seedance-2.5-task')?.dimension).toBe(
      'per_call'
    )
  })
})

describe('durationEnvelopeFor', () => {
  test('leaves seedance-2-5-task to the contract, which states its envelope', () => {
    expect(durationEnvelopeFor('seedance-2-5-task')).toBeUndefined()
  })

  test('pins minimax-h3-task to the 15 seconds its description declares', () => {
    expect(durationEnvelopeFor('minimax-h3-task')).toEqual({
      kind: 'fixed',
      seconds: 15,
    })
  })

  test('records an unestablished envelope as unknown rather than guessing one', () => {
    expect(durationEnvelopeFor('seedance-2-0-task')).toEqual({
      kind: 'unknown',
    })
  })

  test('returns nothing for a model that is not enrolled', () => {
    expect(durationEnvelopeFor('seedance-2-5')).toBeUndefined()
  })
})

describe('BILLING_DECLARATIONS', () => {
  test('enrols the six in-service per-call video ids and nothing else', () => {
    expect(Object.keys(BILLING_DECLARATIONS).sort()).toEqual([
      'gemini-omni-1.1-flash-task',
      'grok-imagine-video-1-5-task',
      'minimax-h3-task',
      'seedance-2-0-fast-task',
      'seedance-2-0-task',
      'seedance-2-5-task',
    ])
  })

  test('enrols no per-second model, which needs no declaration', () => {
    for (const [id, declaration] of Object.entries(BILLING_DECLARATIONS)) {
      expect(declaration.dimension, id).toBe('per_call')
    }
  })

  test('narrows seedance-2-5-task to the resolutions and ratios its line accepts', () => {
    const declaration = billingDeclarationFor('seedance-2-5-task')
    expect(declaration?.resolutions).toEqual(['720p'])
    expect(declaration?.ratios).toEqual(['16:9', '9:16'])
  })

  test('claims no resolution or ratio domain for a model whose envelope is unestablished', () => {
    for (const id of [
      'seedance-2-0-task',
      'seedance-2-0-fast-task',
      'minimax-h3-task',
      'gemini-omni-1.1-flash-task',
      'grok-imagine-video-1-5-task',
    ]) {
      const declaration = billingDeclarationFor(id)
      expect(declaration?.resolutions, id).toBeUndefined()
      expect(declaration?.ratios, id).toBeUndefined()
    }
  })
})
