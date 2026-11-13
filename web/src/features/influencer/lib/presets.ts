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

import snapshot from '../data/presets.json'
import type {
  ExplorePreset,
  InfluencerTaxonomy,
  PresetParamRow,
  PublishedPresetRow,
  Selection,
} from '../types'
import { selectionSchema } from './builder-draft'
import {
  isAnimalTier,
  isCharacterTier,
  reconcileSelection,
  type RemovedPick,
} from './taxonomy'

interface PresetSnapshot {
  version: number
  source: string
  fetched_at: string
  items: ExplorePreset[]
}

const SNAPSHOT = snapshot as PresetSnapshot

/**
 * The preset's picks, resolved against the live taxonomy, or `null` when the
 * preset cannot be reproduced exactly.
 *
 * All-or-nothing on purpose: a preset is one whole look, and applying the
 * resolvable half of it would build a different character than the card shows.
 * The snapshot script already gates on the same taxonomy file, so a `null`
 * here means the two have drifted apart — drop the card rather than mislead.
 */
export function resolvePresetSelection(
  preset: ExplorePreset,
  taxonomy: InfluencerTaxonomy
): Selection | null {
  const tiers = new Set(taxonomy.tier_group.options.map((o) => o.id))
  if (!tiers.has(preset.tier)) return null

  const categories = new Map(taxonomy.categories.map((c) => [c.id, c]))
  const resolved: Selection = {}

  const selection = selectionSchema.safeParse(
    preset.selection === undefined ? {} : preset.selection
  )
  if (!selection.success) return null
  for (const [categoryId, optionIds] of Object.entries(selection.data)) {
    const category = categories.get(categoryId)
    if (!category) return null

    const kept: string[] = []
    for (const optionId of optionIds) {
      const option = category.options.find((o) => o.id === optionId)
      // 档位过滤同样要过：选项若不在该预设自己的档位里可见，
      // 套用后左侧面板根本看不到这一项。
      if (!option || !option.visibleIn.includes(preset.tier)) return null
      kept.push(optionId)
    }
    if (!kept.length) continue
    if (kept.length > category.max) return null
    resolved[categoryId] = kept
  }

  return Object.keys(resolved).length || isAnimalTier(preset.tier)
    ? resolved
    : null
}

/** The presets that can be applied as-is; unusable ones never reach the grid. */
export function usablePresets(
  taxonomy: InfluencerTaxonomy | null
): ExplorePreset[] {
  if (!taxonomy) return []
  return SNAPSHOT.items.filter(
    (p) => resolvePresetSelection(p, taxonomy) !== null
  )
}

export interface AppliedPreset {
  selection: Selection
  removed: RemovedPick[]
}

/**
 * What a card actually applies: the preset's own picks, minus any pick the
 * hard conflicts refuse.
 *
 * The snapshot is mirrored from another product, so a few cards carry a
 * combination this one forbids — the card is still worth offering, but
 * "Recreate" must not land the user on a Generate that cannot succeed.
 * `resolvePresetSelection` still decides whether a card is offered at all; this
 * only reconciles what is written into the sidebar.
 */
export function applyPresetSelection(
  preset: ExplorePreset,
  taxonomy: InfluencerTaxonomy
): AppliedPreset | null {
  const resolved = resolvePresetSelection(preset, taxonomy)
  if (!resolved) return null
  return reconcileSelection(taxonomy.categories, preset.tier, resolved)
}

/**
 * The characters an administrator published into the library, as grid entries.
 *
 * Same all-or-nothing gate as the mirrored snapshot: a row whose tier or option
 * ids no longer resolve against the live taxonomy is dropped, because a card
 * that cannot be applied would build a different character than it shows. The
 * newest publication leads, and equal timestamps fall back to the row id so the
 * order never depends on the gateway's sort.
 */
export function publishedPresets(
  rows: PublishedPresetRow[] | undefined,
  taxonomy: InfluencerTaxonomy | null
): ExplorePreset[] {
  if (!taxonomy || !rows?.length) return []
  return rows
    .map((row): ExplorePreset => ({
      id: row.id,
      name: row.name,
      preview: row.preview,
      thumbnail: row.thumbnail,
      sheet: row.sheet,
      tier: row.tier,
      selection: row.selection,
      brief: row.brief,
      seed: row.seed,
      published: true,
      createdAt: row.created_at,
    }))
    .filter(
      (preset) =>
        !isCharacterTier(preset.tier) ||
        resolvePresetSelection(preset, taxonomy) !== null
    )
    .sort(
      (a, b) =>
        (b.createdAt ?? 0) - (a.createdAt ?? 0) || comparePresetIds(b.id, a.id)
    )
}

/** `pub-9` is older than `pub-12`, so the numeric tail decides before the text. */
function comparePresetIds(a: string, b: string): number {
  const left = Number.parseInt(a.replace(/^\D+/, ''), 10)
  const right = Number.parseInt(b.replace(/^\D+/, ''), 10)
  if (Number.isFinite(left) && Number.isFinite(right) && left !== right) {
    return left - right
  }
  return a.localeCompare(b)
}

/**
 * The preset's parameters as readable rows: category and option labels from the
 * taxonomy, ordered the same way the left sidebar lists them. Raw option ids
 * would mean nothing to the user.
 *
 * Reads the applied selection, not the raw snapshot, so the summary lists
 * exactly what "Recreate" writes into the sidebar.
 */
export function presetParamRows(
  preset: ExplorePreset,
  taxonomy: InfluencerTaxonomy
): PresetParamRow[] {
  const resolved = applyPresetSelection(preset, taxonomy)?.selection
  if (!resolved) return []

  const categories = new Map(taxonomy.categories.map((c) => [c.id, c]))
  const rank = (categoryId: string): number => {
    const index = taxonomy.category_priority.indexOf(categoryId)
    return index < 0 ? 99 : index
  }

  return Object.entries(resolved)
    .map(([categoryId, optionIds]) => {
      const category = categories.get(categoryId)
      return {
        categoryId,
        label: category?.label_en ?? categoryId,
        options: optionIds.map(
          (id) => category?.options.find((o) => o.id === id)?.label_en ?? id
        ),
      }
    })
    .sort((a, b) => rank(a.categoryId) - rank(b.categoryId))
}

/** Snapshot provenance, for diagnostics and the workbench's own reporting. */
export const PRESET_SNAPSHOT = {
  version: SNAPSHOT.version,
  source: SNAPSHOT.source,
  fetchedAt: SNAPSHOT.fetched_at,
  total: SNAPSHOT.items.length,
} as const
