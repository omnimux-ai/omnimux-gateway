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

For commercial licensing, please contact support@quantumnous.com.
*/
import { z } from 'zod'

import type { InfluencerTaxonomy, LookLocks, Selection } from '../types'
import { reconcileSelection } from './taxonomy'

/**
 * The workbench's two state layers, kept apart on purpose:
 *
 * - `selection` is what the sheet currently shows; every writer may change it.
 * - `locks` is the visitor's ownership of a category, and only a manual pick may
 *   write it. A whole look (preset, saved task, landing draft) must never be
 *   translated into locks — that is what turned the dice into a no-op.
 *
 * The session lives in memory only (see `workbench-session.ts`), so this schema
 * also covers an intentionally empty reset: `initialized` says the panel was
 * already set up.
 */
export const selectionSchema = z.record(z.string(), z.array(z.string()).max(50))
const sessionSchema = z.object({
  version: z.literal(1),
  tier: z.string(),
  selection: selectionSchema.default({}),
  locks: selectionSchema.default({}),
  initialized: z.literal(true),
})
export type WorkbenchSession = z.infer<typeof sessionSchema>

/** Accept only current visible options, respecting caps and exclusive slots. */
export function normalizeSelection(
  tax: InfluencerTaxonomy,
  tier: string,
  input: Selection
): Selection {
  const result: Selection = {}
  for (const cat of tax.categories) {
    const kept: string[] = []
    const slots = new Set<string>()
    for (const id of input[cat.id] ?? []) {
      const option = cat.options.find(
        (o) => o.id === id && o.visibleIn.includes(tier)
      )
      if (
        !option ||
        kept.includes(id) ||
        (option.slot && slots.has(option.slot))
      ) {
        continue
      }
      if (cat.id === 'accessory' && id === 'acc_none') {
        if (!kept.length) kept.push(id)
        break
      }
      kept.push(id)
      if (option.slot) slots.add(option.slot)
      if (kept.length >= Math.max(1, cat.max)) break
    }
    if (kept.length) result[cat.id] = kept
  }
  return result
}

/**
 * Keeps only the locks the current selection still holds, so a category whose
 * manual pick was dropped by a tier switch or a conflict never stays pinned to
 * a value the panel no longer shows.
 */
export function lockSubset(selection: Selection, locks: LookLocks): LookLocks {
  const result: LookLocks = {}
  for (const [category, ids] of Object.entries(locks)) {
    const kept = (selection[category] ?? []).filter((id) => ids.includes(id))
    if (kept.length) result[category] = kept
  }
  return result
}

/**
 * Re-applies the taxonomy's rules to a restored session: only options this
 * build still shows survive, and a hard conflict the restored look created is
 * reconciled the same way a fresh look would be.
 */
export function normalizeSession(
  tax: InfluencerTaxonomy,
  session: WorkbenchSession
): WorkbenchSession {
  const tier = session.tier
  const normalized = normalizeSelection(tax, tier, session.selection)
  const selection = reconcileSelection(
    tax.categories,
    tier,
    normalized
  ).selection
  return {
    version: 1,
    tier,
    selection,
    locks: lockSubset(selection, session.locks),
    initialized: true,
  }
}
