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
import { describe, expect, test } from 'vitest'

import type { CatalogZone } from '../catalog'
import {
  groupFacetValues,
  groupSections,
  preferredFacetLabels,
  type ZoneGroup,
  ZONE_GROUPS,
  zoneGroupById,
  zoneGroupOf,
  zonesOfGroup,
} from '../zone-groups'

/** A declared group, looked up by id; throws rather than asserting non-null. */
function requireGroup(id: string): ZoneGroup {
  const group = zoneGroupById(id)
  if (!group) throw new Error(`declared group ${id} is missing`)
  return group
}

function zone(id: string, overrides: Partial<CatalogZone> = {}): CatalogZone {
  return {
    id,
    label: id,
    mediaType: 'video',
    count: 10,
    pages: 1,
    quarantined: 0,
    quarantinePages: 0,
    categories: [],
    styles: [],
    subjects: [],
    useCases: [],
    ...overrides,
  }
}

const ALL_ZONES: CatalogZone[] = [
  zone('seedance-2.5', { count: 1933 }),
  zone('seedance-2.0', { count: 7120 }),
  zone('wan-3.0', { count: 105 }),
  zone('gpt-image-2', { count: 29507, mediaType: 'image' }),
  zone('gpt-image-2.5', { count: 132, mediaType: 'image' }),
  zone('nano-banana-pro', { count: 23730, mediaType: 'image' }),
  zone('grok-imagine', { count: 1615 }),
  zone('claude-opus-5.5', { count: 277 }),
]

describe('zoneGroupOf', () => {
  test('maps a merged zone to its 专區', () => {
    expect(zoneGroupOf('seedance-2.0')?.id).toBe('seedance')
    expect(zoneGroupOf('gpt-image-2')?.id).toBe('image2')
  })

  test('maps a single-zone 专區', () => {
    expect(zoneGroupOf('nano-banana-pro')?.id).toBe('nano')
  })

  test('leaves an unrecognised zone ungrouped rather than guessing', () => {
    expect(zoneGroupOf('some-future-model')).toBeUndefined()
  })

  test('resolves a group by id', () => {
    expect(zoneGroupById('wan30')?.zoneIds).toEqual(['wan-3.0'])
    expect(zoneGroupById('nope')).toBeUndefined()
  })
})

describe('groupSections', () => {
  test('emits one section per 专區 in the declared order', () => {
    expect(groupSections(ALL_ZONES).map((section) => section.key)).toEqual([
      'seedance',
      'image2',
      'nano',
      'wan30',
      'grok',
      'claude',
    ])
  })

  test('sums the counts of a merged 专區', () => {
    const seedance = groupSections(ALL_ZONES).find(
      (section) => section.key === 'seedance'
    )
    expect(seedance?.count).toBe(1933 + 7120)
  })

  test('keeps a single-zone 专區 at its own count', () => {
    const grok = groupSections(ALL_ZONES).find(
      (section) => section.key === 'grok'
    )
    expect(grok?.count).toBe(1615)
  })

  test('drops a 专區 whose zones the manifest does not carry yet', () => {
    const partial = ALL_ZONES.filter((entry) => entry.id !== 'grok-imagine')
    expect(groupSections(partial).map((section) => section.key)).not.toContain(
      'grok'
    )
    expect(groupSections(partial)).toHaveLength(ZONE_GROUPS.length - 1)
  })

  test('carries the i18n key as the label, not a brand name', () => {
    const seedance = groupSections(ALL_ZONES)[0]
    expect(seedance.label).toBe('Seedance zone')
  })

  test('returns nothing when the manifest has no zones', () => {
    expect(groupSections([])).toEqual([])
  })
})

describe('zonesOfGroup', () => {
  test('orders members as the group declares, not as the manifest lists', () => {
    const ordered = zonesOfGroup(requireGroup('image2'), ALL_ZONES).map(
      (entry) => entry.id
    )
    expect(ordered).toEqual(['gpt-image-2.5', 'gpt-image-2'])
  })

  test('skips a member the manifest does not carry', () => {
    const partial = ALL_ZONES.filter((entry) => entry.id !== 'seedance-2.0')
    const members = zonesOfGroup(requireGroup('seedance'), partial)
    expect(members.map((entry) => entry.id)).toEqual(['seedance-2.5'])
  })
})

describe('groupFacetValues', () => {
  test('unions the facet vocabulary of every member zone', () => {
    const zones = [
      zone('gpt-image-2.5', { styles: ['电影级写实风格'] }),
      zone('gpt-image-2', { styles: ['动漫', '电影级写实风格'] }),
    ]
    const image2 = requireGroup('image2')
    expect(groupFacetValues(image2, zones, 'styles')).toEqual([
      '电影级写实风格',
      '动漫',
    ])
  })

  test('reads the subjects facet from the same member set', () => {
    const zones = [zone('wan-3.0', { subjects: ['人物 / 角色'] })]
    const wan30 = requireGroup('wan30')
    expect(groupFacetValues(wan30, zones, 'subjects')).toEqual(['人物 / 角色'])
  })

  test('ignores empty values', () => {
    const zones = [zone('nano-banana-pro', { styles: ['', '动漫'] })]
    expect(groupFacetValues(requireGroup('nano'), zones, 'styles')).toEqual([
      '动漫',
    ])
  })
})

describe('preferredFacetLabels', () => {
  test('keeps only the localized labels when both are present', () => {
    expect(
      preferredFacetLabels(['documentary', '纪录片', 'anime', '动漫'])
    ).toEqual(['纪录片', '动漫'])
  })

  test('returns the values untouched when the vocabulary is slug-only', () => {
    expect(preferredFacetLabels(['documentary', 'anime'])).toEqual([
      'documentary',
      'anime',
    ])
  })

  test('returns nothing for an empty vocabulary', () => {
    expect(preferredFacetLabels([])).toEqual([])
  })
})
