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

import type { ModelFamily, PricingModel } from '@/features/pricing/types'

import { buildParamSchema } from '../param-schema'

/**
 * The page must not deviate from the published contract.
 *
 * `modelcontracts/contracts.json` is the projection the gateway ships, generated
 * from the governance specs and drift-gated in Go. Nothing until now compared it
 * against what the form generator actually renders, so a hand-written table could
 * quietly disagree with the contract the catalog publishes. This is that gate: for
 * every model the payload covers, whatever axis it declares must be the axis the
 * form offers.
 *
 * An axis the contract does not declare is deliberately NOT asserted — the form is
 * allowed its previous behaviour there, and asserting otherwise would be inventing
 * a requirement.
 */
const payload = JSON.parse(
  readFileSync(
    resolve(
      dirname(fileURLToPath(import.meta.url)),
      '../../../../../..',
      'modelcontracts/contracts.json'
    ),
    'utf8'
  )
) as {
  models: Record<
    string,
    {
      model_id: string
      family?: string
      resolutions?: string[]
      aspect_ratios?: string[]
      output_formats?: string[]
    }
  >
}

/** Families whose form this generator serves. */
const FORM_FAMILIES = new Set(['video', 'image'])

function modelFor(contract: {
  model_id: string
  family?: string
  resolutions?: string[]
  aspect_ratios?: string[]
  output_formats?: string[]
}): PricingModel {
  return {
    id: 1,
    model_name: contract.model_id,
    quota_type: 1,
    model_ratio: 0,
    model_price: 1,
    completion_ratio: 0,
    enable_groups: ['default'],
    tags: contract.family === 'video' ? 'Video Generation' : 'Image Generation',
    contract: { ...contract, family: contract.family as ModelFamily },
  }
}

function declaredSubjects() {
  return Object.values(payload.models).filter(
    (contract) =>
      contract.family !== undefined && FORM_FAMILIES.has(contract.family)
  )
}

function valuesOf(schema: ReturnType<typeof buildParamSchema>, key: string) {
  return schema.controls
    .find((control) => control.key === key)
    ?.options?.map((option) => option.value)
}

/**
 * Deviations recorded on 2026-09-14, when this gate was first run.
 *
 * The gate asserts the computed set is a SUBSET of these, so a new deviation
 * fails the suite while the existing ones stay visible instead of being averaged
 * away. Fixing one is allowed and needs no edit here; deleting a line only
 * tightens the gate.
 *
 * These are the page's remaining hand-written domains outliving the contract —
 * the follow-up work this gate exists to track.
 */
const KNOWN_DEVIATIONS: Record<string, string[]> = {
  // Empty on purpose, and it must stay that way: every axis the contract
  // declares is rendered from the contract. The gate's first run reported 33 ids,
  // but it built the models without their contract, so it measured the fallback
  // path — an artefact of the test, not a defect in the form.
  resolution: [],
  aspect_ratio: [],
  output_format: [],
}

/** Model ids whose rendered values differ from the contract, for one axis. */
function deviatingIds(
  contractKey: 'resolutions' | 'aspect_ratios' | 'output_formats',
  controlKey: string
): string[] {
  const ids: string[] = []
  for (const contract of Object.values(payload.models)) {
    if (!contract.family || !FORM_FAMILIES.has(contract.family)) continue
    const declared = contract[contractKey]
    if (!declared?.length) continue
    const rendered =
      valuesOf(buildParamSchema(modelFor(contract)), controlKey) ?? []
    if (rendered.join(',') !== declared.join(',')) {
      ids.push(contract.model_id)
    }
  }
  return ids.sort()
}

describe('the rendered form and the published contract agree', () => {
  it('has contracts to compare', () => {
    expect(declaredSubjects().length).toBeGreaterThan(0)
  })

  it('introduces no new resolution deviation', () => {
    const known = new Set(KNOWN_DEVIATIONS.resolution)
    expect(
      deviatingIds('resolutions', 'resolution').filter((id) => !known.has(id))
    ).toEqual([])
  })

  it('introduces no new aspect-ratio deviation', () => {
    const known = new Set(KNOWN_DEVIATIONS.aspect_ratio)
    expect(
      deviatingIds('aspect_ratios', 'aspect_ratio').filter(
        (id) => !known.has(id)
      )
    ).toEqual([])
  })

  it('introduces no new output-format deviation', () => {
    const known = new Set(KNOWN_DEVIATIONS.output_format)
    expect(
      deviatingIds('output_formats', 'output_format').filter(
        (id) => !known.has(id)
      )
    ).toEqual([])
  })

  it('records every remaining deviation, so none is silently dropped', () => {
    // A recorded id that no longer deviates is not an error — that is the work
    // being finished — but the count must not grow unnoticed.
    expect(
      deviatingIds('resolutions', 'resolution').length
    ).toBeLessThanOrEqual(KNOWN_DEVIATIONS.resolution.length)
    expect(
      deviatingIds('aspect_ratios', 'aspect_ratio').length
    ).toBeLessThanOrEqual(KNOWN_DEVIATIONS.aspect_ratio.length)
    expect(
      deviatingIds('output_formats', 'output_format').length
    ).toBeLessThanOrEqual(KNOWN_DEVIATIONS.output_format.length)
  })
})

describe('the image form and the declared output sizes agree', () => {
  // Image specs state `output_sizes` rather than `resolutions`, and the parity
  // gate above only covered the latter — which is how a model could publish
  // 1024x1024 and still be offered the modality's 1K/2K/4K.
  const imageSubjects = Object.values(payload.models).filter(
    (contract) => contract.family === 'image'
  )

  it('has image contracts to compare', () => {
    expect(imageSubjects.length).toBeGreaterThan(0)
  })

  it('offers exactly the declared sizes, for every model that declares them', () => {
    const deviations: string[] = []
    for (const contract of Object.values(payload.models)) {
      const declared = (contract as { output_sizes?: string[] }).output_sizes
      if (!declared?.length) continue
      const rendered =
        valuesOf(buildParamSchema(modelFor(contract)), 'resolution') ?? []
      if (rendered.join(',') !== declared.join(',')) {
        deviations.push(
          `${contract.model_id}: contract ${declared.join('/')} vs form ${rendered.join('/')}`
        )
      }
    }
    expect(deviations).toEqual([])
  })
})
