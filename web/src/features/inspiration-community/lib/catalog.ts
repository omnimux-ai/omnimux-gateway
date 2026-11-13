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
 * Client for the statically served Inspiration Community catalog.
 *
 * The prompt pages used to read a 304-row TypeScript fixture bundled with the
 * app. The real corpus (64k+ rows) ships as static JSON instead, served from
 * the same origin:
 *
 *   GET {base}manifest.json                  zones, counts, facet vocabulary
 *   GET {base}{zone}/page-0000.json          one page of displayable rows
 *   GET {base}{zone}/quarantine-0000.json    one page of quarantined rows
 *
 * This module owns that contract and nothing else — URL shapes, payload
 * validation, and the configurable base path — so the hooks above it carry no
 * URL building and the contract is testable without rendering.
 */
import type { PromptItem, PromptMediaType, PromptSection } from '../types'
import { groupSections } from './zone-groups'

/** Same-origin mount point of the catalog in production (Caddy static route). */
export const DEFAULT_INSPIRATION_CATALOG_BASE = '/inspiration-catalog/'

/**
 * Placeholder card count for the loading skeleton, used only until the
 * manifest declares the real `pageSize`. It is never printed as a count.
 */
export const DEFAULT_CATALOG_PAGE_SIZE = 48

/** One model zone of the catalog, e.g. `seedance-2.5`. */
export interface CatalogZone {
  id: string
  label: string
  mediaType: PromptMediaType
  /** Displayable rows in the zone; quarantined rows are counted separately. */
  count: number
  pages: number
  quarantined: number
  quarantinePages: number
  /** Facet vocabulary of the zone, used for the filter chips. */
  categories: string[]
  styles: string[]
  subjects: string[]
  useCases: string[]
}

/** Provenance of one source feed, as reported by the builder. */
export interface CatalogSource {
  id: string
  rows: number
  quarantined: number
  dropped: number
}

export interface CatalogManifest {
  version: number
  generatedAt: string
  /** Rows per page file; the builder pads the page number to four digits. */
  pageSize: number
  /** Displayable rows across every zone. */
  total: number
  quarantinedTotal: number
  sources: CatalogSource[]
  zones: CatalogZone[]
}

/** One page file: `total`/`totalPages` describe the set this page belongs to. */
export interface CatalogPage {
  zone: string
  page: number
  pageSize: number
  total: number
  totalPages: number
  items: PromptItem[]
}

/**
 * Base path of the catalog, always with a leading and a trailing slash.
 *
 * `VITE_INSPIRATION_CATALOG_BASE` overrides the production default so local
 * verification can point at another static mount (or another origin) without
 * touching code.
 */
export function inspirationCatalogBase(): string {
  return normalizeCatalogBase(import.meta.env?.VITE_INSPIRATION_CATALOG_BASE)
}

export function normalizeCatalogBase(raw?: string | null): string {
  const value = (raw ?? '').trim()
  if (!value) return DEFAULT_INSPIRATION_CATALOG_BASE

  const hasScheme = /^[a-z][a-z0-9+.-]*:\/\//iu.test(value)
  const rooted = hasScheme || value.startsWith('/') ? value : `/${value}`
  return `${rooted.replace(/\/+$/u, '')}/`
}

export function catalogManifestUrl(base: string): string {
  return `${base}manifest.json`
}

/**
 * Path of one page file, e.g. `/inspiration-catalog/seedance-2.0/page-0007.json`
 * or `.../quarantine-0002.json` for the quarantined set.
 */
export function catalogPageUrl(
  base: string,
  zone: string,
  page: number,
  quarantined: boolean
): string {
  const name = quarantined ? 'quarantine' : 'page'
  return `${base}${zone}/${name}-${String(page).padStart(4, '0')}.json`
}

/** Rows the manifest declares for the requested set of a zone. */
export function zoneRowCount(
  zone: CatalogZone,
  quarantined: boolean
): number {
  return quarantined ? zone.quarantined : zone.count
}

/** Pages the manifest declares for the requested set of a zone. */
export function zonePageCount(
  zone: CatalogZone,
  quarantined: boolean
): number {
  return quarantined ? zone.quarantinePages : zone.pages
}

/**
 * Zones as the model sections the model pages group and look up by.
 *
 * Keyed by the catalog zone, because that is what a model page's URL names and
 * what its page files are addressed by. The 专區 used for navigation on the
 * gallery is a separate projection (`catalogGroupSections`).
 */
export function catalogSections(manifest: CatalogManifest): PromptSection[] {
  return manifest.zones.map((zone) => ({
    key: zone.id,
    label: zone.label,
    count: zone.count,
  }))
}

/**
 * The 专區 sections the gallery navigates by.
 *
 * One 专區 can merge several zones, so its count is the sum of theirs and its
 * key is the 专區 id rather than any single zone. The catalog itself stays
 * per-zone: paging, retries and error reporting keep operating on the ids the
 * manifest publishes.
 */
export function catalogGroupSections(
  manifest: CatalogManifest
): PromptSection[] {
  return groupSections(manifest.zones)
}

/**
 * Union of the facet vocabulary of every zone, in FIRST-APPEARANCE order:
 * zone order first, then each zone's own order.
 *
 * The manifest emits each zone's `categories` by descending coverage (see the
 * builder's schema comment), so the union keeps the most-covered values of the
 * most-covered zone first. It is deliberately NOT re-sorted alphabetically:
 * the chip row is capped, and an alphabetical re-sort would throw away the one
 * signal that makes a cap presentable.
 */
export function catalogCategories(zones: readonly CatalogZone[]): string[] {
  const seen = new Set<string>()
  for (const zone of zones) {
    for (const category of zone.categories) {
      if (category) seen.add(category)
    }
  }
  return [...seen]
}

/**
 * Facet chips a capped row shows before the visitor asks for the rest.
 *
 * Twelve fills two or three wrapped rows on a desktop listing and one or two
 * on a phone, which is the size of a filter row a visitor actually reads. The
 * value is a starting point, not a measured optimum: what matters is that the
 * head of the list is the coverage-ordered vocabulary, so the visible chips
 * are the ones most of the zone's prompts actually carry.
 */
export const FACET_CHIP_LIMIT = 12

/**
 * The chips a capped facet row shows: the first `limit` values in the
 * manifest's coverage order, plus `selected` when the visitor's own choice
 * falls outside them.
 *
 * Keeping the selected value is the point of the helper. A visitor who picked
 * category 40 and then sees the row collapse back to the first twelve would
 * have no way to see — let alone undo — the filter they are looking at, and
 * the listing would be narrowed by a chip that is nowhere on screen.
 */
export function visibleFacetValues(
  values: readonly string[],
  limit: number,
  selected?: string | null
): string[] {
  const visible = values.slice(0, limit)
  if (!selected || visible.includes(selected)) return visible
  return values.includes(selected) ? [...visible, selected] : visible
}

/**
 * A request that was cancelled rejects with `AbortError` (fetch) or
 * `CanceledError` (axios); callers must swallow it instead of reporting a
 * failure the visitor never saw.
 */
export function isAbortError(error: unknown): boolean {
  if (typeof error !== 'object' || error === null || !('name' in error)) {
    return false
  }
  const name = (error as { name?: unknown }).name
  return name === 'AbortError' || name === 'CanceledError'
}

/** Narrow an unknown rejection to an `Error` without losing its message. */
export function toCatalogError(error: unknown): Error {
  if (error instanceof Error) return error
  return new Error(String(error))
}

export async function fetchCatalogManifest(
  base: string,
  signal?: AbortSignal
): Promise<CatalogManifest> {
  const payload = await requestJson(catalogManifestUrl(base), signal)
  return readManifest(payload)
}

export interface CatalogPageRequest {
  base: string
  zone: string
  page: number
  /** Quarantined rows are off by default; see `useCatalogZoneRows`. */
  quarantined?: boolean
  signal?: AbortSignal
}

export async function fetchCatalogPage(
  request: CatalogPageRequest
): Promise<CatalogPage> {
  const url = catalogPageUrl(
    request.base,
    request.zone,
    request.page,
    request.quarantined === true
  )
  const payload = await requestJson(url, request.signal)
  return readPage(payload, url)
}

async function requestJson(url: string, signal?: AbortSignal): Promise<unknown> {
  const response = await fetch(url, { signal })
  if (!response.ok) {
    throw new Error(`Catalog request failed (${response.status}): ${url}`)
  }
  return response.json()
}

function readManifest(payload: unknown): CatalogManifest {
  if (typeof payload !== 'object' || payload === null) {
    throw new Error('Inspiration catalog manifest is not an object.')
  }
  const manifest = payload as Partial<CatalogManifest>
  if (!Array.isArray(manifest.zones) || manifest.zones.length === 0) {
    throw new Error('Inspiration catalog manifest carries no zones.')
  }
  if (typeof manifest.pageSize !== 'number' || manifest.pageSize < 1) {
    throw new Error('Inspiration catalog manifest carries no page size.')
  }
  return {
    version: manifest.version ?? 0,
    generatedAt: manifest.generatedAt ?? '',
    pageSize: manifest.pageSize,
    total: manifest.total ?? 0,
    quarantinedTotal: manifest.quarantinedTotal ?? 0,
    sources: manifest.sources ?? [],
    zones: manifest.zones,
  }
}

function readPage(payload: unknown, url: string): CatalogPage {
  if (typeof payload !== 'object' || payload === null) {
    throw new Error(`Inspiration catalog page is not an object: ${url}`)
  }
  const page = payload as Partial<CatalogPage>
  if (!Array.isArray(page.items)) {
    throw new Error(`Inspiration catalog page carries no items: ${url}`)
  }
  return {
    zone: page.zone ?? '',
    page: page.page ?? 0,
    pageSize: page.pageSize ?? page.items.length,
    total: page.total ?? page.items.length,
    totalPages: page.totalPages ?? 1,
    items: page.items,
  }
}
