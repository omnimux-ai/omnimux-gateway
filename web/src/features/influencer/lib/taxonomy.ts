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

import type { InfluencerCategory, InfluencerOption, Selection } from '../types'

export function catById(cats: InfluencerCategory[], id: string) {
  return cats.find((c) => c.id === id)
}

export function optById(
  cats: InfluencerCategory[],
  catId: string,
  optId: string
) {
  const c = catById(cats, catId)
  return c?.options.find((o) => o.id === optId)
}

export function visOpts(
  c: InfluencerCategory,
  tier: string
): InfluencerOption[] {
  return c.options.filter((o) => o.visibleIn.includes(tier))
}

function includes(arr: string[] | undefined, v: string): boolean {
  return !!arr && arr.includes(v)
}

function has(sel: Selection, cat: string, opt: string): boolean {
  return includes(sel[cat], opt)
}

function hasAny(sel: Selection, cat: string, opts: string[]): boolean {
  return (sel[cat] ?? []).some((v) => opts.includes(v))
}

export interface ConflictRule {
  /** Stable id for tests and diagnostics. */
  id: string
  /** English message key — the caller wraps it in t(). */
  message: string
  hits: (tier: string, sel: Selection) => boolean
  /**
   * What the randomizer re-rolls to clear this rule, and the options it must
   * avoid. Re-rolling keeps every card filled, which deleting cannot.
   */
  reroll: { category: string; avoid: string[] }
}

/**
 * 4 hard conflicts: 2 from the upstream conflict table plus 2 neck/teeth
 * exclusions from the same rules dataset. Single source of truth: the manual
 * picker reads `message`, the randomizer reads `reroll`.
 */
export const CONFLICT_RULES: ConflictRule[] = [
  {
    id: 'heavy_no_tall',
    message: 'Heavy build cannot be combined with Tall or Very tall.',
    hits: (_, sel) =>
      has(sel, 'body_type', 'body_heavy') &&
      hasAny(sel, 'height', ['h_tall', 'h_very_tall']),
    reroll: { category: 'height', avoid: ['h_tall', 'h_very_tall'] },
  },
  {
    id: 'build_no_shortlimb',
    message:
      'This build cannot be combined with Short legs or Long limbs in this mode.',
    hits: (tier, sel) =>
      tier !== 'normal' &&
      hasAny(sel, 'body_type', ['body_heavy', 'body_muscular', 'body_ultra']) &&
      hasAny(sel, 'proportions', ['pr_shortlegs', 'pr_longlimbs']),
    reroll: {
      category: 'proportions',
      avoid: ['pr_shortlegs', 'pr_longlimbs'],
    },
  },
  {
    id: 'ultra_no_longneck',
    message: 'Extreme muscular build cannot be combined with Long neck.',
    hits: (_, sel) =>
      has(sel, 'body_type', 'body_ultra') &&
      has(sel, 'freak_neck', 'neck_long'),
    reroll: { category: 'freak_neck', avoid: ['neck_long'] },
  },
  {
    id: 'longneck_no_buckteeth',
    message: 'Long neck cannot be combined with Buck teeth.',
    hits: (_, sel) =>
      has(sel, 'freak_neck', 'neck_long') &&
      has(sel, 'freak_face', 'ff_teeth_11'),
    reroll: { category: 'freak_face', avoid: ['ff_teeth_11'] },
  },
]

/** The first rule the selection trips, or null when the selection is valid. */
export function isCharacterTier(tier: unknown): tier is string {
  return (
    tier === 'normal' ||
    tier === 'freak' ||
    tier === 'total' ||
    tier === 'cats' ||
    tier === 'dogs'
  )
}

export function isAnimalTier(tier: string): boolean {
  return tier === 'cats' || tier === 'dogs'
}

export function findConflict(
  tier: string,
  sel: Selection
): ConflictRule | null {
  if (isAnimalTier(tier)) return null
  for (const rule of CONFLICT_RULES) {
    if (rule.hits(tier, sel)) return rule
  }
  return null
}

/** The rule's message, or null when the selection is valid. */
export function conflictMessage(tier: string, sel: Selection): string | null {
  return findConflict(tier, sel)?.message ?? null
}

/** A pick a hard conflict forced out, named the way the sidebar names it. */
export interface RemovedPick {
  categoryId: string
  optionId: string
  label: string
}

export interface ReconcileResult {
  selection: Selection
  removed: RemovedPick[]
}

/** Bounded so a pathological ruleset can never spin. */
const MAX_RECONCILE_PASSES = 8

/**
 * Drops the picks a hard conflict names and keeps the rest of the selection.
 *
 * The picker refuses these combinations as they are made, but a whole look
 * arrives in one step — a mirrored preset, a saved task, or a tier switch that
 * makes an already-legal pair illegal. Those paths have no per-option moment to
 * refuse at, so they reconcile instead: the look stays generatable and the
 * caller reports which pick went.
 *
 * Every rule is a conjunction of positive tests, so removing an option can
 * never create a new conflict; the loop only guards a future rule whose
 * `reroll.avoid` does not name the pick that trips it.
 */
export function reconcileSelection(
  cats: InfluencerCategory[],
  tier: string,
  sel: Selection
): ReconcileResult {
  const next: Selection = { ...sel }
  const removed: RemovedPick[] = []

  for (let pass = 0; pass < MAX_RECONCILE_PASSES; pass += 1) {
    const rule = findConflict(tier, next)
    if (!rule) break
    const { category, avoid } = rule.reroll
    const picks = next[category] ?? []
    const offending = picks.filter((id) => avoid.includes(id))
    if (!offending.length) break

    for (const optionId of offending) {
      removed.push({
        categoryId: category,
        optionId,
        label: optById(cats, category, optionId)?.label_en ?? optionId,
      })
    }
    const kept = picks.filter((id) => !avoid.includes(id))
    if (kept.length) next[category] = kept
    else delete next[category]
  }

  return { selection: next, removed }
}

export interface TrySelectResult {
  selection: Selection
  message: string | null
}

/**
 * Toggle an option inside the current selection with the same semantics as
 * the demo: category max cap (first picks win), same-slot replacement inside
 * multi-select categories, acc_none exclusivity, and the hard conflict gate.
 */
export function trySelect(
  cats: InfluencerCategory[],
  tier: string,
  sel: Selection,
  catId: string,
  optId: string
): TrySelectResult {
  const c = catById(cats, catId)
  if (!c) return { selection: sel, message: 'Unknown category.' }
  const opt = optById(cats, catId, optId)
  if (!opt) return { selection: sel, message: 'Unknown option.' }
  if (!opt.visibleIn.includes(tier))
    {return { selection: sel, message: 'Unknown option.' }}

  const cur = [...(sel[catId] ?? [])]
  let next: string[]
  if (cur.includes(optId)) {
    next = cur.filter((x) => x !== optId)
  } else {
    const maxN = c.max > 0 ? c.max : 1
    if (maxN === 1 || (catId === 'accessory' && optId === 'acc_none')) {
      next = [optId]
    } else {
      const kept = cur.filter((id) => {
        if (catId === 'accessory' && id === 'acc_none') return false
        if (!opt.slot) return true
        const po = optById(cats, catId, id)
        return po?.slot !== opt.slot
      })
      next = kept.length >= maxN ? kept : [...kept, optId]
    }
  }
  const trial: Selection = { ...sel }
  if (next.length) trial[catId] = next
  else delete trial[catId]

  const msg = conflictMessage(tier, trial)
  if (msg) return { selection: sel, message: msg }
  return { selection: trial, message: null }
}

/**
 * Changing tier drops any selected option not visible in the new tier and
 * returns the dropped labels for a toast.
 */
export function filterSelectionForTier(
  cats: InfluencerCategory[],
  tier: string,
  sel: Selection
): { selection: Selection; dropped: string[] } {
  const out: Selection = {}
  const dropped: string[] = []
  for (const c of cats) {
    const picks = sel[c.id]
    if (!picks?.length) continue
    const ok = picks.filter((id) => {
      const o = optById(cats, c.id, id)
      const visible = !!o && o.visibleIn.includes(tier)
      if (!visible) dropped.push(o?.label_en ?? id)
      return visible
    })
    if (ok.length) out[c.id] = ok
  }
  return { selection: out, dropped }
}

export function selectedCount(sel: Selection, catId: string): number {
  return (sel[catId] ?? []).length
}

export function totalSelected(sel: Selection): number {
  return Object.values(sel).reduce((n, arr) => n + arr.length, 0)
}

export function selectedLabels(
  cats: InfluencerCategory[],
  sel: Selection
): string[] {
  const out: string[] = []
  for (const c of cats) {
    for (const id of sel[c.id] ?? []) {
      out.push(optById(cats, c.id, id)?.label_en ?? id)
    }
  }
  return out
}
