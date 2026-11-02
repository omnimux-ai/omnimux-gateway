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
import type { CatalogZone } from './catalog'

/**
 * The 专區 the prompt library is organised into.
 *
 * One 专區 can merge several catalog zones: `seedance` holds 2.5 and 2.0, and
 * `image2` holds Image 2 and 2.5. Cards keep naming the model they actually
 * carry — the merge is a navigation decision, not a claim that the two models
 * are the same.
 *
 * The catalog still publishes per-zone data (counts, pages, facet vocabulary),
 * so a 专區 is a projection over zones rather than a new data source. That keeps
 * paging, retries and error reporting working exactly as they do per zone.
 */
export type ZoneGroupId =
  | 'seedance'
  | 'image2'
  | 'nano'
  | 'wan30'
  | 'grok'
  | 'claude'

export interface ZoneGroup {
  id: ZoneGroupId
  /**
   * i18n key. Every section label already renders through `t()` (the shell
   * does it for manifest labels too), so a key and a brand name travel the
   * same path.
   */
  label: string
  /** Catalog zones this 专區 merges, in display order. */
  zoneIds: string[]
  /**
   * Lobe icon name rendered before the tab label (`getLobeIcon` naming, e.g.
   * `Claude.Color`). Resolved by group id only — the label is localizable and
   * must never be used as the lookup key.
   */
  iconName: string
}

export const ZONE_GROUPS: readonly ZoneGroup[] = [
  {
    id: 'seedance',
    label: 'Seedance zone',
    zoneIds: ['seedance-2.5', 'seedance-2.0'],
    iconName: 'Doubao.Color',
  },
  {
    id: 'image2',
    label: 'Image 2 zone',
    zoneIds: ['gpt-image-2.5', 'gpt-image-2'],
    iconName: 'OpenAI.Color',
  },
  {
    id: 'nano',
    label: 'Nano zone',
    zoneIds: ['nano-banana-pro'],
    iconName: 'Gemini.Color',
  },
  {
    id: 'wan30',
    label: 'Wan 3.0 zone',
    zoneIds: ['wan-3.0'],
    iconName: 'Wan',
  },
  {
    id: 'grok',
    label: 'Grok zone',
    zoneIds: ['grok-imagine'],
    iconName: 'Grok.Color',
  },
  {
    id: 'claude',
    label: 'Claude zone',
    zoneIds: ['claude-opus-5.5'],
    iconName: 'Claude.Color',
  },
]

const GROUP_BY_ZONE = new Map<string, ZoneGroup>(
  ZONE_GROUPS.flatMap((group) =>
    group.zoneIds.map((zoneId) => [zoneId, group] as const)
  )
)

const GROUP_BY_ID = new Map<string, ZoneGroup>(
  ZONE_GROUPS.map((group) => [group.id, group] as const)
)

/** The 专區 a catalog zone belongs to, or undefined for an unlisted zone. */
export function zoneGroupOf(zoneId: string): ZoneGroup | undefined {
  return GROUP_BY_ZONE.get(zoneId)
}

export function zoneGroupById(id: string): ZoneGroup | undefined {
  return GROUP_BY_ID.get(id)
}

/** Catalog zones of one 专區, in the group's own order. */
export function zonesOfGroup(
  group: ZoneGroup,
  zones: readonly CatalogZone[]
): CatalogZone[] {
  const byId = new Map(zones.map((zone) => [zone.id, zone] as const))
  return group.zoneIds
    .map((zoneId) => byId.get(zoneId))
    .filter((zone): zone is CatalogZone => zone !== undefined)
}

/** Shape the shell and the anchor bar consume, mirroring a manifest section. */
export interface ZoneGroupSection {
  key: string
  label: string
  /** Sum of the member zones' displayable counts. */
  count: number
}

/**
 * Groups in display order, restricted to the zones the manifest actually
 * carries.
 *
 * A group with no live zone is dropped rather than rendered empty, so a catalog
 * that has not published one of the merged zones yet still produces a coherent
 * tab row. An unrecognised zone is not silently folded into a neighbour: it
 * simply has no group and disappears from the tab row, which is visible in the
 * manifest rather than hidden in the merge.
 */
export function groupSections(
  zones: readonly CatalogZone[]
): ZoneGroupSection[] {
  const sections: ZoneGroupSection[] = []
  for (const group of ZONE_GROUPS) {
    const members = zonesOfGroup(group, zones)
    if (members.length === 0) continue
    sections.push({
      key: group.id,
      label: group.label,
      count: members.reduce((sum, zone) => sum + zone.count, 0),
    })
  }
  return sections
}

/**
 * Facet chips for a 专區, in first-appearance order across its zones.
 *
 * The manifest emits each zone's vocabulary by descending coverage, so the head
 * of this list is the vocabulary most of the 专區's prompts actually carry —
 * which is what the capped chip row needs.
 */
export function groupFacetValues(
  group: ZoneGroup,
  zones: readonly CatalogZone[],
  facet: 'styles' | 'subjects' | 'categories'
): string[] {
  const seen = new Set<string>()
  const values: string[] = []
  for (const zone of zonesOfGroup(group, zones)) {
    for (const value of zone[facet] ?? []) {
      if (!value || seen.has(value)) continue
      seen.add(value)
      values.push(value)
    }
  }
  return values
}

/** True when a facet value carries a CJK character. */
function hasCjk(value: string): boolean {
  return /[\u3400-\u9fff]/u.test(value)
}

/**
 * Collapse a bilingual vocabulary onto one chip per concept.
 *
 * The builder emits each concept twice — once as its ASCII slug
 * (`documentary`) and once as its localized label (`纪录片`) — and the slug
 * block is concatenated ahead of the label block, so the row would print every
 * style twice. When the vocabulary carries localized labels at all, they are
 * the ones a visitor reads, so the slugs are dropped; a vocabulary that is
 * slug-only is returned untouched rather than emptied.
 */
export function preferredFacetLabels(values: readonly string[]): string[] {
  const localized = values.filter(hasCjk)
  return localized.length > 0 ? localized : [...values]
}
