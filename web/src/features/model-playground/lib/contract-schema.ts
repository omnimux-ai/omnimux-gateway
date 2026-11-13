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
import type { ContractOperation, ModelContract } from '@/features/pricing/types'

/**
 * Compatibility view of a v1 operation list.
 *
 * A v1 payload named operations as bare strings. It never said what those
 * operations consume or produce, so the view marks them non-executable and
 * invents no inputs or parameters: absence of data must stay absent.
 */
function compatibilityOperation(id: string): ContractOperation {
  return {
    id,
    executable: false,
    inputs: [],
    parameters: [],
    output: { type: 'video' },
  }
}

function isOperationObject(value: unknown): value is ContractOperation {
  return (
    typeof value === 'object' &&
    value !== null &&
    typeof (value as ContractOperation).id === 'string'
  )
}

/** Every operation a contract publishes, in either payload shape. */
export function contractOperations(
  contract: ModelContract | undefined
): ContractOperation[] {
  if (!contract) return []

  const declared = contract.operations
  if (Array.isArray(declared)) {
    const objects = declared.filter(isOperationObject)
    if (objects.length > 0) return objects

    const ids = declared.filter(
      (operation): operation is string => typeof operation === 'string'
    )
    if (ids.length > 0) return ids.map(compatibilityOperation)
  }

  return (contract.operation_ids ?? []).map(compatibilityOperation)
}

/** Operations the gateway can actually execute today. */
export function executableOperations(
  contract: ModelContract | undefined
): ContractOperation[] {
  return contractOperations(contract).filter(
    (operation) => operation.executable
  )
}

/** One contract operation by id, or undefined when the contract omits it. */
export function findContractOperation(
  contract: ModelContract | undefined,
  operationId: string
): ContractOperation | undefined {
  return contractOperations(contract).find(
    (operation) => operation.id === operationId
  )
}

/** Input slots one operation consumes. */
export function operationInputs(
  contract: ModelContract | undefined,
  operationId: string
): ContractOperation['inputs'] {
  return findContractOperation(contract, operationId)?.inputs ?? []
}

/** Parameters one operation accepts. */
export function operationParameters(
  contract: ModelContract | undefined,
  operationId: string
): ContractOperation['parameters'] {
  return findContractOperation(contract, operationId)?.parameters ?? []
}
