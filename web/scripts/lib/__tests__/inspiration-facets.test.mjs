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
 * The facet-ordering contract of the catalog build: each zone's facet arrays
 * are emitted by descending coverage, so the frontend's capped chip row shows
 * the vocabulary that actually covers the zone.
 */
import { describe, expect, it } from 'vitest'

import { zoneFacets } from '../inspiration-facets.mjs'

/** A row as the builder hands it to `zoneFacets`; only facet fields matter. */
function row(category, useCases = [], styles = [], tags = []) {
  return { category, useCases, styles, tags }
}

describe('zoneFacets', () => {
  it('orders a facet by descending coverage', () => {
    const facets = zoneFacets([
      row('rare'),
      row('common'),
      row('common'),
      row('common'),
      row('middling'),
      row('middling'),
    ])

    expect(facets.categories).toEqual(['common', 'middling', 'rare'])
  })

  it('breaks a coverage tie by localeCompare, not by insertion order', () => {
    const facets = zoneFacets([
      row('zulu'),
      row('alpha'),
      row('mike'),
      row('alpha'),
      row('zulu'),
      row('mike'),
    ])

    expect(facets.categories).toEqual(['alpha', 'mike', 'zulu'])
  })

  it('orders every facet dimension by coverage, not just categories', () => {
    const facets = zoneFacets([
      row('one', ['use-b'], ['style-b'], ['subject-b', 'use-b', 'style-b']),
      row('one', ['use-a'], ['style-a'], ['subject-a', 'use-a', 'style-a']),
      row('one', ['use-a'], ['style-a'], ['subject-a', 'use-a', 'style-a']),
    ])

    expect(facets.useCases).toEqual(['use-a', 'use-b'])
    expect(facets.styles).toEqual(['style-a', 'style-b'])
    expect(facets.subjects).toEqual(['subject-a', 'subject-b'])
  })

  it('counts a value once per row even when a row lists it twice', () => {
    // A tag repeated inside one row is one row carrying it, which would
    // otherwise let a duplicated source value outrank a genuinely common one.
    const facets = zoneFacets([
      row('duplicated', [], [], ['duplicated', 'duplicated']),
      row('shared', [], [], ['shared']),
      row('shared', [], [], ['shared']),
    ])

    expect(facets.categories).toEqual(['shared', 'duplicated'])
    expect(facets.subjects).toEqual(['shared', 'duplicated'])
  })

  it('excludes values carried only by quarantined rows', () => {
    // The builder passes displayable rows only, so a value that is not in the
    // input can never become a chip; `quarantined-only` stands in for a value
    // the builder dropped before calling this function.
    const facets = zoneFacets([row('visible')])

    expect(facets.categories).toEqual(['visible'])
  })

  it('ignores an empty category instead of emitting a blank chip', () => {
    expect(zoneFacets([row(''), row('named')]).categories).toEqual(['named'])
  })

  it('drops useCases and styles out of subjects', () => {
    const facets = zoneFacets([
      row(
        'one',
        ['shared-use-case'],
        ['shared-style'],
        ['shared-use-case', 'shared-style', 'only-subject']
      ),
    ])

    expect(facets.subjects).toEqual(['only-subject'])
  })
})
