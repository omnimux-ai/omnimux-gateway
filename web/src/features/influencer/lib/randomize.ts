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

import type {
  InfluencerCategory,
  InfluencerOption,
  InfluencerTaxonomy,
  Selection,
} from '../types'
import {
  CONFLICT_RULES,
  catById,
  filterSelectionForTier,
  findConflict,
  isAnimalTier,
  visOpts,
} from './taxonomy'

/** Injectable randomness so tests can pin an outcome; production uses Math.random. */
export type RandomSource = () => number

/**
 * Weighted picks from the dataset's `rules.randomize` profile. Each entry maps
 * an option id to its probability; whatever is left over spreads uniformly
 * across the category's remaining eligible options.
 */
const WEIGHTS: Record<string, Record<string, number>> = {
  gender: { male: 0.67, female: 0.33 },
  aesthetic: { suits: 0.4, retro: 0.2 },
  hair: { hs_lampshade: 0.2 },
}

/** Options the profile never rolls, per category. */
const NEVER_ROLLED: Record<string, string[]> = {
  body_type: ['body_curvy'],
}

/** Options only the Bold and Extreme tiers roll. */
const NEVER_ROLLED_NORMAL: Record<string, string[]> = {
  body_type: ['body_ultra'],
}

/** Categories other categories read; rolled first so the value is already known. */
const DEPENDENCY_ROOTS = new Set(['gender'])

const BEARD_GENDERS = ['male', 'trans_man']

const BEARD_OPTIONS = [
  'fh_stubble',
  'fh_beard',
  'fh_goatee',
  'fh_moustache',
  'fh_pencil',
  'fh_pushbroom',
  'fh_braid',
]

/** The face features that occupy the brow slot. */
const BROW_OPTIONS = ['fn_thickbrows', 'ff_brows_5', 'ff_brows_6', 'ff_brows_7']

/**
 * Cross-category dependencies the dataset itself does not encode. Each rule
 * names a category and the options that stop being eligible once something
 * else is already rolled. The brow pair is listed in both directions so the
 * result does not depend on category order.
 */
interface Dependency {
  category: string
  /** Why the rule exists, kept beside the rule so it survives refactors. */
  reason: string
  /** Option ids to drop from this category's pool for the current selection. */
  drop: (sel: Selection) => string[]
}

const DEPENDENCIES: Dependency[] = [
  {
    category: 'facial_hair',
    reason: 'Beards and moustaches only grow on people who grow them.',
    drop: (sel) => (hasAny(sel, 'gender', BEARD_GENDERS) ? [] : BEARD_OPTIONS),
  },
  {
    category: 'distinctive',
    reason: '“No brows” and a brow feature describe the same feature twice.',
    drop: (sel) =>
      hasAny(sel, 'freak_face', BROW_OPTIONS) ? ['df_nobrows'] : [],
  },
  {
    category: 'freak_face',
    reason: '“No brows” leaves no brows to style.',
    drop: (sel) => (has(sel, 'distinctive', 'df_nobrows') ? BROW_OPTIONS : []),
  },
]

function includes(arr: string[] | undefined, value: string): boolean {
  return !!arr && arr.includes(value)
}

function has(sel: Selection, category: string, option: string): boolean {
  return includes(sel[category], option)
}

function hasAny(sel: Selection, category: string, options: string[]): boolean {
  return (sel[category] ?? []).some((value) => options.includes(value))
}

function rollOrder(categories: InfluencerCategory[]): InfluencerCategory[] {
  const roots = categories.filter((c) => DEPENDENCY_ROOTS.has(c.id))
  return [...roots, ...categories.filter((c) => !DEPENDENCY_ROOTS.has(c.id))]
}

/**
 * The options this category may roll right now: visible in the tier, not
 * banned by the profile, not in `avoid`, and not cancelled by a dependency.
 * Without locks, falls back to the tier's full visible set so a filtered-out
 * pool never leaves the card blank before conflict repair.
 */
function eligiblePool(
  cat: InfluencerCategory,
  tier: string,
  sel: Selection,
  avoid: string[],
  hasLocks = false
): InfluencerOption[] {
  const visible = visOpts(cat, tier)
  if (isAnimalTier(tier)) return visible
  const banned = new Set<string>([
    ...(NEVER_ROLLED[cat.id] ?? []),
    ...(tier === 'normal' ? (NEVER_ROLLED_NORMAL[cat.id] ?? []) : []),
    ...avoid,
  ])
  for (const dep of DEPENDENCIES) {
    if (dep.category !== cat.id) continue
    for (const id of dep.drop(sel)) banned.add(id)
  }
  const pool = visible.filter((o) => !banned.has(o.id))
  if (!hasLocks) return pool.length ? pool : visible

  // Conflicts a lock already carries belong to the caller; only reject new ones.
  const rules = CONFLICT_RULES.filter((rule) => !rule.hits(tier, sel))
  return pool.filter((option) => {
    if (
      cat.id === 'gender' &&
      hasAny(sel, 'facial_hair', BEARD_OPTIONS) &&
      !BEARD_GENDERS.includes(option.id)
    ) {
      return false
    }
    const trial: Selection = { ...sel, [cat.id]: [option.id] }
    return !rules.some((rule) => rule.hits(tier, trial))
  })
}

/** Draws from `pool` following WEIGHTS; unweighted options share the remainder. */
function pickWeighted(
  catId: string,
  pool: InfluencerOption[],
  random: RandomSource
): InfluencerOption {
  const weights = WEIGHTS[catId] ?? {}
  const roll = random()
  let acc = 0
  for (const [id, weight] of Object.entries(weights)) {
    acc += weight
    if (roll < acc) {
      const hit = pool.find((o) => o.id === id)
      // A weighted option may be excluded in this tier; the remainder takes over.
      if (hit) return hit
      break
    }
  }
  const rest = pool.filter((o) => !(o.id in weights))
  const draw = rest.length ? rest : pool
  return draw[Math.floor(random() * draw.length)]
}

/** Bounded so a pathological ruleset can never spin. */
const MAX_REPAIR_PASSES = 8

/**
 * Clears every conflict by re-rolling the rule's soft category, which keeps
 * each card filled. Only a category with no compatible option left is dropped.
 *
 * A locked category is never re-rolled: ownership outranks a tidy sheet, so a
 * conflict a lock is part of stays put for the submit gate to report instead.
 */
function repair(
  tax: InfluencerTaxonomy,
  tier: string,
  sel: Selection,
  random: RandomSource,
  locked: ReadonlySet<string>
): Selection {
  const next: Selection = { ...sel }
  for (let pass = 0; pass < MAX_REPAIR_PASSES; pass++) {
    const conflict = findConflict(tier, next)
    if (!conflict) return next
    const { category, avoid } = conflict.reroll
    if (locked.has(category)) break
    const cat = catById(tax.categories, category)
    if (!cat) break
    const usable = eligiblePool(cat, tier, next, avoid).filter(
      (o) => !avoid.includes(o.id)
    )
    if (usable.length) {
      next[category] = [pickWeighted(category, usable, random).id]
    } else if (next[category]) {
      delete next[category]
    } else {
      break
    }
  }
  return next
}

/**
 * Weighted randomizer matching the upstream product profile: male 67%,
 * female 33%, suits 40%, retro 20%, lampshade hair 20%, curvy excluded, ultra
 * excluded from the Average tier.
 *
 * `locks` are the categories the visitor pinned by hand: they are copied after
 * tier filtering and never rolled, and every other category fills around them
 * so a roll cannot create a hard conflict with a locked pick. A category with
 * no compatible candidate stays empty; conflicting locks stay untouched for
 * the caller to report.
 */
export function randomizeSelection(
  tax: InfluencerTaxonomy,
  tier: string,
  random: RandomSource = Math.random,
  locks: Selection = {}
): Selection {
  const sel = filterSelectionForTier(tax.categories, tier, locks).selection
  const lockedCategories = new Set(Object.keys(sel))
  const hasLocks = lockedCategories.size > 0
  for (const cat of rollOrder(tax.categories)) {
    if (lockedCategories.has(cat.id)) continue
    const pool = eligiblePool(cat, tier, sel, [], hasLocks)
    if (!pool.length) continue
    sel[cat.id] = [pickWeighted(cat.id, pool, random).id]
  }
  return repair(tax, tier, sel, random, lockedCategories)
}

/**
 * How many categories the dice would re-roll right now. The caller disables the
 * dice when this is zero, so "nothing happened" never needs explaining.
 */
export function rollableCount(
  tax: InfluencerTaxonomy,
  tier: string,
  locks: Selection = {}
): number {
  const locked = new Set(
    Object.keys(filterSelectionForTier(tax.categories, tier, locks).selection)
  )
  let count = 0
  for (const cat of tax.categories) {
    if (locked.has(cat.id) || !visOpts(cat, tier).length) continue
    count += 1
  }
  return count
}
