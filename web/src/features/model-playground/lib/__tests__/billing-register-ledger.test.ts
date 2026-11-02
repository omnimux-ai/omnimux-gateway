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
import { readdirSync, readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, test } from 'vitest'

import {
  BILLING_DECLARATIONS,
  LEDGER_BILLING_CONFLICTS,
  billingDimensionFor,
} from '../billing-dimension'

/**
 * Register ⇄ ledger drift guard.
 *
 * `web/src/features/model-playground/lib/billing-dimension.ts` is the only place
 * the playground reads a model's billing dimension from, and the pricing ledger
 * is the ops record that knows it. Nothing links the two at runtime — the
 * catalog payload carries no billing dimension — so this suite is the link: a
 * per-call video model added to the ledger fails here until it is enrolled, and
 * an enrolment the ledger does not back fails here too.
 */
const REGISTER_PATH =
  'web/src/features/model-playground/lib/billing-dimension.ts'

const LEDGER_DIR = resolve(
  dirname(fileURLToPath(import.meta.url)),
  '../../../../../..',
  'scripts/ops/pricing-ledger/models'
)

/** Ledger modes that mean "one charge per invocation". */
const PER_CALL_MODES = new Set(['per_task', 'per_action'])

/** Ledger statuses that mean the model is in service. */
const IN_SERVICE = new Set(['active', 'on'])

type LedgerRow = {
  public_id?: string
  modality?: string
  billing_mode?: string
  status?: string
}

function ledgerRows(): LedgerRow[] {
  const rows: LedgerRow[] = []
  for (const file of readdirSync(LEDGER_DIR)) {
    if (!file.endsWith('.json')) continue
    const parsed: unknown = JSON.parse(
      readFileSync(resolve(LEDGER_DIR, file), 'utf8')
    )
    const models = Array.isArray(parsed)
      ? parsed
      : ((parsed as { models?: LedgerRow[] }).models ?? [])
    for (const model of models) {
      rows.push(model)
    }
  }
  return rows
}

function videoRows(rows: LedgerRow[]): LedgerRow[] {
  return rows.filter(
    (row) =>
      row.modality === 'video' && Boolean(row.public_id) && row.billing_mode
  )
}

function modesByPublicId(rows: LedgerRow[]): Map<string, Set<string>> {
  const modes = new Map<string, Set<string>>()
  for (const row of videoRows(rows)) {
    const id = row.public_id as string
    const found = modes.get(id) ?? new Set<string>()
    found.add(row.billing_mode as string)
    modes.set(id, found)
  }
  return modes
}

describe('billing register vs pricing ledger', () => {
  test('enrols every in-service per-call video model as per_call', () => {
    const missing = new Set<string>()
    for (const row of videoRows(ledgerRows())) {
      if (!PER_CALL_MODES.has(row.billing_mode as string)) continue
      if (!IN_SERVICE.has(row.status ?? '')) continue
      const id = row.public_id as string
      if (billingDimensionFor(id) !== 'per_call') missing.add(id)
    }

    const list = [...missing].sort()
    expect(
      list,
      `Enrol these in BILLING_DECLARATIONS (${REGISTER_PATH}): ${list.join(', ')}`
    ).toEqual([])
  })

  test('records every ledger id whose billing modes disagree with each other', () => {
    const unrecorded = [...modesByPublicId(ledgerRows())]
      .filter(([, modes]) => modes.size > 1)
      .map(([id]) => id)
      .filter((id) => !LEDGER_BILLING_CONFLICTS.has(id))
      .sort()

    expect(
      unrecorded,
      `Ledger ids with conflicting billing modes must be listed in LEDGER_BILLING_CONFLICTS, or reconciled in the ledger: ${unrecorded.join(', ')}`
    ).toEqual([])
  })

  test('enrols no id the ledger contradicts', () => {
    const modes = modesByPublicId(ledgerRows())
    const contradicted = Object.keys(BILLING_DECLARATIONS)
      .filter((id) => {
        const known = modes.get(id)
        // A model the ledger has never heard of cannot contradict anything.
        if (!known) return false
        return ![...known].some((mode) => PER_CALL_MODES.has(mode))
      })
      .sort()

    expect(
      contradicted,
      `The ledger records these as per-second only, so enrolling them as per_call in ${REGISTER_PATH} is unsupported: ${contradicted.join(', ')}`
    ).toEqual([])
  })
})
