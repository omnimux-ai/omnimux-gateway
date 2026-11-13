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
import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

import type {
  ContractOperation,
  ModelContract,
  ModelFamily,
  PricingModel,
} from '@/features/pricing/types'

import { contractOperations, executableOperations } from '../contract-schema'
import { operationsFor } from '../operation-matrix'
import { buildOperationSchema, buildParamSchema } from '../param-schema'

/**
 * The page must not deviate from the published contract.
 *
 * `modelcontracts/contracts.json` is the projection the gateway ships, generated
 * from the governance specs and drift-gated in Go. This gate compares every
 * executable operation's inputs and parameters against the controls the form
 * actually renders. It runs once the payload carries Contract v2 object
 * operations; a v1 projection has no operation schema to compare, so the gate
 * stays skipped rather than asserting against data that does not exist.
 */
type ContractDocument = {
  schemaVersion?: string
  models: Record<string, ModelContract>
}

const payload = JSON.parse(
  readFileSync(
    resolve(
      dirname(fileURLToPath(import.meta.url)),
      '../../../../../..',
      'modelcontracts/contracts.json'
    ),
    'utf8'
  )
) as ContractDocument

const isV2 = payload.schemaVersion === '2.0'

/** Families whose form this generator serves. */
const FORM_FAMILIES = new Set<ModelFamily>(['video', 'image'])

function modelFor(contract: ModelContract): PricingModel {
  return {
    id: 1,
    model_name: contract.model_id,
    quota_type: 1,
    model_ratio: 0,
    model_price: 1,
    completion_ratio: 0,
    enable_groups: ['default'],
    tags: contract.family === 'video' ? 'Video Generation' : 'Image Generation',
    contract,
  }
}

function formSubjects(): ModelContract[] {
  return Object.values(payload.models).filter(
    (contract) => contract.family && FORM_FAMILIES.has(contract.family)
  )
}

/** Bare contract formats sometimes arrive as extensions; the form uses MIME. */
const MIME_BY_FORMAT: Record<string, string> = {
  jpeg: 'image/jpeg',
  jpg: 'image/jpeg',
  png: 'image/png',
  webp: 'image/webp',
  gif: 'image/gif',
  bmp: 'image/bmp',
  tiff: 'image/tiff',
  heic: 'image/heic',
  heif: 'image/heif',
  mp4: 'video/mp4',
  mov: 'video/quicktime',
  quicktime: 'video/quicktime',
  wav: 'audio/wav',
  mp3: 'audio/mpeg',
}

function asMime(value: string): string {
  const token = value.trim().toLowerCase()
  return token.includes('/') ? token : (MIME_BY_FORMAT[token] ?? token)
}

function expectParameterRendered(
  control: ReturnType<typeof buildParamSchema>['controls'][number] | undefined,
  parameter: ContractOperation['parameters'][number],
  subject: string
) {
  expect(
    control,
    `${subject}: missing parameter ${parameter.name}`
  ).toBeDefined()
  if (!control) return

  if (parameter.fixed !== undefined) {
    expect(
      control.lockedReasonKey,
      `${subject}: ${parameter.name} not locked`
    ).toBeTruthy()
    expect(
      String(control.defaultValue),
      `${subject}: ${parameter.name} fixed value`
    ).toBe(String(parameter.fixed))
    return
  }

  if (parameter.type === 'enum') {
    expect(
      control.options?.map((option) => option.value),
      `${subject}: ${parameter.name} enum`
    ).toEqual((parameter.enum ?? []).map((value) => String(value)))
    if (parameter.default !== undefined) {
      expect(
        String(control.defaultValue),
        `${subject}: ${parameter.name} default`
      ).toBe(String(parameter.default))
    }
    return
  }

  if (parameter.type === 'integer' || parameter.type === 'number') {
    expect(control.kind, `${subject}: ${parameter.name} control`).toBe('slider')
    expect(control.min, `${subject}: ${parameter.name} minimum`).toBe(
      parameter.minimum
    )
    expect(control.max, `${subject}: ${parameter.name} maximum`).toBe(
      parameter.maximum
    )
    if (parameter.step !== undefined) {
      expect(control.step, `${subject}: ${parameter.name} step`).toBe(
        parameter.step
      )
    }
    if (parameter.default !== undefined) {
      expect(
        control.defaultValue,
        `${subject}: ${parameter.name} default`
      ).toBe(parameter.default)
    }
    return
  }

  if (parameter.type === 'boolean') {
    expect(control.kind, `${subject}: ${parameter.name} control`).toBe('switch')
    expect(control.defaultValue, `${subject}: ${parameter.name} default`).toBe(
      parameter.default ?? false
    )
  }
}

const describeContractParity = isV2 ? describe : describe.skip

describeContractParity(
  'the rendered form and the published contract agree',
  () => {
    it('has Contract v2 operations to compare', () => {
      const executable = formSubjects().flatMap((contract) =>
        executableOperations(contract)
      )
      expect(executable.length).toBeGreaterThan(0)
    })

    it('renders every executable video operation as a selectable mode', () => {
      for (const contract of formSubjects()) {
        if (contract.family !== 'video') continue
        const rendered = operationsFor(modelFor(contract))
        const executable = executableOperations(contract)
          .filter((operation) => operation.output.type === 'video')
          .map((operation) => operation.id)

        expect(rendered, contract.model_id).toEqual(executable)
      }
    })

    it('renders every video operation parameter with its contract value', () => {
      for (const contract of formSubjects()) {
        if (contract.family !== 'video') continue
        for (const operation of executableOperations(contract)) {
          const schema = buildOperationSchema(
            modelFor(contract),
            operation.id as Parameters<typeof buildOperationSchema>[1]
          )
          for (const parameter of operation.parameters ?? []) {
            expectParameterRendered(
              schema.controls.find((control) => control.key === parameter.name),
              parameter,
              `${contract.model_id}.${operation.id}`
            )
          }
        }
      }
    })

    it('renders a media control for every video operation input', () => {
      for (const contract of formSubjects()) {
        if (contract.family !== 'video') continue
        for (const operation of executableOperations(contract)) {
          const schema = buildOperationSchema(
            modelFor(contract),
            operation.id as Parameters<typeof buildOperationSchema>[1]
          )
          for (const input of operation.inputs ?? []) {
            const control = schema.controls.find(
              (candidate) => candidate.key === input.slot
            )
            expect(
              control,
              `${contract.model_id}.${operation.id}: missing input ${input.slot}`
            ).toBeDefined()
            if (!control) continue
            if (input.type === 'text' || input.type === 'url') {
              expect(control.kind).toBe('text')
              continue
            }
            expect(
              control.kind,
              `${contract.model_id}.${operation.id}: ${input.slot}`
            ).toBe('media')
            if (input.max !== undefined) {
              expect(control.maxCount).toBe(input.max)
            }
            if (input.maxSizeMb !== undefined) {
              expect(control.maxBytes).toBe(
                Math.round(input.maxSizeMb * 1024 * 1024)
              )
            }
            for (const mime of input.allowedMimes ?? []) {
              expect(
                control.accept?.split(',').map((entry) => entry.trim()),
                `${contract.model_id}.${operation.id}: ${input.slot} accepts ${mime}`
              ).toContain(asMime(mime))
            }
          }
        }
      }
    })

    it('renders every image operation parameter with its contract value', () => {
      for (const contract of formSubjects()) {
        if (contract.family !== 'image') continue
        const schema = buildParamSchema(modelFor(contract))
        for (const operation of executableOperations(contract)) {
          for (const parameter of operation.parameters ?? []) {
            expectParameterRendered(
              schema.controls.find((control) => control.key === parameter.name),
              parameter,
              `${contract.model_id}.${operation.id}`
            )
          }
        }
      }
    })

    it('carries the contract reference-image ceiling onto the flat image form', () => {
      for (const contract of formSubjects()) {
        if (contract.family !== 'image') continue
        const declared = contract.reference_images
        if (!declared) continue
        const schema = buildParamSchema(modelFor(contract))
        const control = schema.controls.find(
          (candidate) => candidate.key === 'reference_media'
        )
        if (declared.max === 0) {
          expect(
            control,
            `${contract.model_id}: no reference control`
          ).toBeUndefined()
          continue
        }
        expect(
          control,
          `${contract.model_id}: missing reference control`
        ).toBeDefined()
        expect(
          control?.maxCount,
          `${contract.model_id}: reference ceiling`
        ).toBe(declared.max ?? 1)
        if (declared.max_bytes_each !== undefined) {
          expect(
            control?.maxBytes,
            `${contract.model_id}: reference bytes`
          ).toBe(declared.max_bytes_each)
        }
      }
    })

    it('does not invent modes for video contracts whose operations are not executable', () => {
      for (const contract of formSubjects()) {
        if (contract.family !== 'video') continue
        const executable = executableOperations(contract).filter(
          (operation) => operation.output.type === 'video'
        )
        if (executable.length > 0) continue
        expect(operationsFor(modelFor(contract)), contract.model_id).toEqual([])
      }
    })
  }
)

describe('the contract operation payload', () => {
  it('publishes object operations once the v2 payload is generated', () => {
    if (!isV2) return
    for (const contract of formSubjects()) {
      for (const operation of contractOperations(contract)) {
        expect(typeof operation.id, contract.model_id).toBe('string')
        expect(typeof operation.executable, contract.model_id).toBe('boolean')
        expect(operation.output.type, contract.model_id).toBeDefined()
      }
    }
  })
})
