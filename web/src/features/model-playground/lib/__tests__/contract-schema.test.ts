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

import type { ContractOperation, ModelContract } from '@/features/pricing/types'

import {
  contractOperations,
  executableOperations,
  findContractOperation,
  operationInputs,
  operationParameters,
} from '../contract-schema'

const videoOperation: ContractOperation = {
  id: 'text_to_video',
  executable: true,
  inputs: [{ slot: 'prompt', type: 'text', required: true }],
  parameters: [{ name: 'duration', type: 'integer', minimum: 1, maximum: 15 }],
  output: { type: 'video' },
}

function contract(overrides: Partial<ModelContract> = {}): ModelContract {
  return { model_id: 'm', family: 'video', ...overrides }
}

describe('contractOperations', () => {
  test('returns nothing without a contract', () => {
    expect(contractOperations(undefined)).toEqual([])
  })

  test('turns a v1 string list into non-executable compatibility entries', () => {
    const operations = contractOperations(
      contract({ operations: ['text_to_video', 'first_frame'] })
    )

    expect(operations).toEqual([
      {
        id: 'text_to_video',
        executable: false,
        inputs: [],
        parameters: [],
        output: { type: 'video' },
      },
      {
        id: 'first_frame',
        executable: false,
        inputs: [],
        parameters: [],
        output: { type: 'video' },
      },
    ])
  })

  test('keeps a v2 object array intact', () => {
    expect(
      contractOperations(contract({ operations: [videoOperation] }))
    ).toEqual([videoOperation])
  })

  test('falls back to operation_ids when the object array is absent', () => {
    expect(
      contractOperations(contract({ operation_ids: ['text_to_video'] }))
    ).toEqual([
      {
        id: 'text_to_video',
        executable: false,
        inputs: [],
        parameters: [],
        output: { type: 'video' },
      },
    ])
  })
})

describe('executableOperations', () => {
  test('keeps only operations the gateway can execute', () => {
    const operations = executableOperations(
      contract({
        operations: [
          videoOperation,
          { ...videoOperation, id: 'first_frame', executable: false },
        ],
      })
    )
    expect(operations.map((operation) => operation.id)).toEqual([
      'text_to_video',
    ])
  })

  test('keeps nothing from a v1 payload', () => {
    expect(
      executableOperations(contract({ operations: ['text_to_video'] }))
    ).toEqual([])
  })
})

describe('findContractOperation', () => {
  test('finds one operation and exposes its inputs and parameters', () => {
    const subject = contract({ operations: [videoOperation] })

    expect(findContractOperation(subject, 'text_to_video')?.id).toBe(
      'text_to_video'
    )
    expect(operationInputs(subject, 'text_to_video')).toEqual([
      { slot: 'prompt', type: 'text', required: true },
    ])
    expect(operationParameters(subject, 'text_to_video')).toEqual([
      { name: 'duration', type: 'integer', minimum: 1, maximum: 15 },
    ])
  })

  test('returns nothing for an operation the contract does not publish', () => {
    const subject = contract({ operations: [videoOperation] })

    expect(findContractOperation(subject, 'video_edit')).toBeUndefined()
    expect(operationInputs(subject, 'video_edit')).toEqual([])
    expect(operationParameters(subject, 'video_edit')).toEqual([])
  })
})
