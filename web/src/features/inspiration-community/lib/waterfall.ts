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
/**
 * Waterfall layout shared by both prompt levels.
 *
 * The browsing tile sizes its media box from the item's own `aspectRatio`
 * (`PromptDetailCard`), so a batch of tiles is ragged by construction: in a CSS
 * grid every card in a row would stretch to the tallest one and leave a hole
 * under the short ones. Both levels therefore cut the batch into columns and
 * lay each column out as its own flex stack.
 *
 * Hand-built columns cannot be re-counted by CSS alone — the batch has to be
 * re-cut for a different column count — so the count is resolved in JS from the
 * same breakpoints the rest of the page uses.
 */
import { useMediaQuery } from '@/hooks/use-media-query'

/** One rung of a waterfall ladder, widest first. */
export interface WaterfallStep {
  /** CSS media query, e.g. `(min-width: 1280px)`. */
  query: string
  /** Columns the batch is cut into while the query matches. */
  columns: number
}

/**
 * Exactly three rungs, widest first. The tuple type is what lets
 * `useWaterfallColumnCount` call its hooks unconditionally: the ladder length
 * is a compile-time fact rather than a runtime assumption.
 */
export type WaterfallLadder = readonly [
  WaterfallStep,
  WaterfallStep,
  WaterfallStep,
]

/**
 * L2 listing ladder: one column below `md` (768), then 2 / 3 / 4 at
 * `md` / `xl` (1280) / `2xl` (1536).
 */
export const LISTING_COLUMN_STEPS: WaterfallLadder = [
  { query: '(min-width: 1536px)', columns: 4 },
  { query: '(min-width: 1280px)', columns: 3 },
  { query: '(min-width: 768px)', columns: 2 },
]

/**
 * L1 section-preview ladder: the same treatment, one rung denser, because a
 * section is a ten-tile teaser above the "View all" pill rather than a library
 * wall — 2 / 3 / 4 / 5 at `sm` (640) / `md` (768) / `lg` (1024).
 */
export const PREVIEW_COLUMN_STEPS: WaterfallLadder = [
  { query: '(min-width: 1024px)', columns: 5 },
  { query: '(min-width: 768px)', columns: 4 },
  { query: '(min-width: 640px)', columns: 3 },
]

/** Columns of the live viewport's ladder rung; one column below every rung. */
export function useWaterfallColumnCount(steps: WaterfallLadder): number {
  const [widest, wide, medium] = steps
  const matchesWidest = useMediaQuery(widest.query)
  const matchesWide = useMediaQuery(wide.query)
  const matchesMedium = useMediaQuery(medium.query)

  if (matchesWidest) return widest.columns
  if (matchesWide) return wide.columns
  if (matchesMedium) return medium.columns
  return 1
}

/**
 * Estimate normalized card height from its aspect ratio.
 * `aspectRatio` is width / height (e.g. "9 / 16", "16 / 9", "1 / 1").
 * Normalized height = (height / width) + base padding for title & badges.
 */
function estimateItemHeight(item: unknown): number {
  if (item && typeof item === 'object' && 'aspectRatio' in item) {
    const raw = (item as { aspectRatio?: unknown }).aspectRatio
    if (typeof raw === 'string') {
      const parts = raw.split(/[/:]/).map((s) => Number.parseFloat(s.trim()))
      if (
        parts.length === 2 &&
        !Number.isNaN(parts[0]) &&
        !Number.isNaN(parts[1]) &&
        parts[0] > 0 &&
        parts[1] > 0
      ) {
        return parts[1] / parts[0] + 0.3
      }
    }
  }
  return 1.0
}

/**
 * Distribute items into `count` waterfall columns using a greedy shortest-column
 * balancing algorithm.
 *
 * Rather than naive contiguous slicing, each item is placed into the column with
 * the minimum current cumulative height. This ensures columns grow evenly and prevents
 * jagged voids or empty trailing columns when cards have mixed aspect ratios.
 */
export function splitIntoColumns<T>(
  items: readonly T[],
  count: number
): Array<{ id: string; items: T[] }> {
  if (count <= 1) {
    return [{ id: 'column-1', items: [...items] }]
  }

  const columns: Array<{ id: string; items: T[] }> = Array.from(
    { length: count },
    (_, index) => ({
      id: `column-${index + 1}`,
      items: [],
    })
  )
  const columnHeights = Array.from({ length: count }, () => 0)

  for (const item of items) {
    let shortestIndex = 0
    let minHeight = columnHeights[0]
    for (let i = 1; i < count; i++) {
      if (columnHeights[i] < minHeight) {
        minHeight = columnHeights[i]
        shortestIndex = i
      }
    }
    columns[shortestIndex].items.push(item)
    columnHeights[shortestIndex] += estimateItemHeight(item)
  }

  return columns
}
