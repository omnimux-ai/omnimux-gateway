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
 * Facet vocabulary of one catalog zone, derived from the zone's displayable
 * rows.
 *
 * This is the ordered contract the frontend relies on: the arrays are emitted
 * in DESCENDING COVERAGE order (the number of displayable rows carrying the
 * value), tie-broken by `localeCompare`, and never alphabetically. A zone in
 * the merged catalog can carry enough free-text facet values that rendering
 * every one of them as a filter chip is unusable, so the frontend caps the
 * chip row at `FACET_CHIP_LIMIT` — and a cap is only defensible when the head
 * of the list is the part of the vocabulary that actually covers the zone.
 * Alphabetical order throws exactly that signal away.
 *
 * Only displayable rows are counted: a quarantined row's poster is gone, so a
 * chip no visible row could match must not be offered. `zoneFacets` is a
 * separate module from the builder so this contract can be tested without
 * running a build.
 */

function isNonEmptyString(value) {
  return typeof value === 'string' && value.length > 0
}

/**
 * `value -> number of displayable rows carrying it`, in descending coverage
 * order with a `localeCompare` tie-break so equal-coverage values still emit
 * in a stable, human-alphabetical order.
 */
function orderByCoverage(coverage) {
  return [...coverage.entries()]
    .sort(
      (left, right) => right[1] - left[1] || left[0].localeCompare(right[0])
    )
    .map(([value]) => value)
}

/**
 * Content-only facet arrays for one zone, derived from the rows themselves.
 *
 * `tags` minus the two dimensions that already have a field of their own give
 * `subjects`; every other array maps to the row field of the same name.
 */
export function zoneFacets(items) {
  const categories = new Map()
  const useCases = new Map()
  const styles = new Map()
  const tags = new Map()

  for (const item of items) {
    countRow(categories, [item.category])
    countRow(useCases, item.useCases)
    countRow(styles, item.styles)
    countRow(tags, item.tags)
  }

  const subjects = new Map()
  for (const [value, coverages] of tags) {
    if (!useCases.has(value) && !styles.has(value)) {
      subjects.set(value, coverages)
    }
  }

  return {
    categories: orderByCoverage(categories),
    useCases: orderByCoverage(useCases),
    styles: orderByCoverage(styles),
    subjects: orderByCoverage(subjects),
  }
}

/**
 * Add one row's worth of coverage to `coverages`: coverage counts ROWS
 * carrying a value, so a value a single row repeats still adds one.
 */
function countRow(coverages, values) {
  const seen = new Set()
  for (const value of values) {
    if (!isNonEmptyString(value) || seen.has(value)) continue
    seen.add(value)
    coverages.set(value, (coverages.get(value) ?? 0) + 1)
  }
}
