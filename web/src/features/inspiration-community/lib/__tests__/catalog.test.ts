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
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  DEFAULT_INSPIRATION_CATALOG_BASE,
  catalogCategories,
  catalogGroupSections,
  catalogManifestUrl,
  catalogPageUrl,
  catalogSections,
  fetchCatalogManifest,
  fetchCatalogPage,
  inspirationCatalogBase,
  isAbortError,
  normalizeCatalogBase,
  visibleFacetValues,
  zonePageCount,
  zoneRowCount,
  type CatalogManifest,
} from '../catalog'

const ZONE = {
  id: 'seedance-2.0',
  label: 'Seedance 2.0',
  mediaType: 'video' as const,
  count: 7120,
  pages: 149,
  quarantined: 12,
  quarantinePages: 1,
  categories: ['product-ad'],
  styles: [],
  subjects: [],
  useCases: [],
}

const MANIFEST: CatalogManifest = {
  version: 1,
  generatedAt: '2026-09-12T00:00:00.000Z',
  pageSize: 48,
  total: 64142,
  quarantinedTotal: 1370,
  sources: [{ id: 'topview', rows: 35500, quarantined: 0, dropped: 1773 }],
  zones: [ZONE],
}

function jsonResponse(payload: unknown, ok = true, status = 200): Response {
  return { ok, status, json: async () => payload } as unknown as Response
}

afterEach(() => {
  vi.unstubAllEnvs()
  vi.unstubAllGlobals()
})

describe('catalog base path', () => {
  it('defaults to the same-origin mount point', () => {
    expect(normalizeCatalogBase(undefined)).toBe(
      DEFAULT_INSPIRATION_CATALOG_BASE
    )
    expect(normalizeCatalogBase('   ')).toBe(DEFAULT_INSPIRATION_CATALOG_BASE)
  })

  it('normalizes slashes and keeps absolute URLs intact', () => {
    expect(normalizeCatalogBase('catalog')).toBe('/catalog/')
    expect(normalizeCatalogBase('/catalog')).toBe('/catalog/')
    expect(normalizeCatalogBase('/catalog/v1//')).toBe('/catalog/v1/')
    expect(normalizeCatalogBase('https://cdn.example.com/catalog')).toBe(
      'https://cdn.example.com/catalog/'
    )
  })

  it('reads VITE_INSPIRATION_CATALOG_BASE so local runs can point elsewhere', () => {
    vi.stubEnv('VITE_INSPIRATION_CATALOG_BASE', '/tmp-catalog')
    expect(inspirationCatalogBase()).toBe('/tmp-catalog/')

    vi.stubEnv('VITE_INSPIRATION_CATALOG_BASE', '')
    expect(inspirationCatalogBase()).toBe(DEFAULT_INSPIRATION_CATALOG_BASE)
  })
})

describe('catalog URLs', () => {
  it('builds the manifest and four-digit page paths', () => {
    expect(catalogManifestUrl('/inspiration-catalog/')).toBe(
      '/inspiration-catalog/manifest.json'
    )
    expect(catalogPageUrl('/inspiration-catalog/', 'seedance-2.0', 0, false)).toBe(
      '/inspiration-catalog/seedance-2.0/page-0000.json'
    )
    expect(
      catalogPageUrl('/inspiration-catalog/', 'seedance-2.0', 148, false)
    ).toBe('/inspiration-catalog/seedance-2.0/page-0148.json')
    expect(catalogPageUrl('/inspiration-catalog/', 'nano-banana-pro', 2, true)).toBe(
      '/inspiration-catalog/nano-banana-pro/quarantine-0002.json'
    )
  })
})

describe('manifest mapping', () => {
  it('exposes zones as model sections so a model page keeps its own key', () => {
    expect(catalogSections(MANIFEST)).toEqual([
      { key: 'seedance-2.0', label: 'Seedance 2.0', count: 7120 },
    ])
  })

  it('projects the same zones onto the 专區 sections the gallery navigates by', () => {
    expect(catalogGroupSections(MANIFEST)).toEqual([
      { key: 'seedance', label: 'Seedance zone', count: 7120 },
    ])
  })

  it('reads the displayed counts from the requested set of a zone', () => {
    expect(zoneRowCount(ZONE, false)).toBe(7120)
    expect(zoneRowCount(ZONE, true)).toBe(12)
    expect(zonePageCount(ZONE, false)).toBe(149)
    expect(zonePageCount(ZONE, true)).toBe(1)
  })

  it('keeps the manifest order when unioning the facets of every zone', () => {
    expect(
      catalogCategories([
        { ...ZONE, categories: ['product-ad', 'cinematic-film'] },
        { ...ZONE, id: 'other', categories: ['cinematic-film', ''] },
      ])
    ).toEqual(['product-ad', 'cinematic-film'])
  })

  it('unions zones in zone order, not alphabetical order', () => {
    // `zzz` and `aaa` are deliberately anti-alphabetical: an alphabetical
    // re-sort would reverse them and throw away the coverage ordering the
    // manifest publishes.
    expect(
      catalogCategories([
        { ...ZONE, categories: ['zzz-last-zone-first'] },
        { ...ZONE, id: 'other', categories: ['aaa-second-zone'] },
      ])
    ).toEqual(['zzz-last-zone-first', 'aaa-second-zone'])
  })
})

describe('visibleFacetValues', () => {
  const VALUES = ['a', 'b', 'c', 'd']

  it('shows the first values in order when nothing is selected', () => {
    expect(visibleFacetValues(VALUES, 2)).toEqual(['a', 'b'])
    expect(visibleFacetValues(VALUES, 2, null)).toEqual(['a', 'b'])
  })

  it('returns every value when the cap exceeds the list', () => {
    expect(visibleFacetValues(VALUES, 10)).toEqual(VALUES)
  })

  it('leaves the visible values untouched when the selection is already in them', () => {
    expect(visibleFacetValues(VALUES, 2, 'b')).toEqual(['a', 'b'])
  })

  it('appends a selection that falls outside the cap', () => {
    expect(visibleFacetValues(VALUES, 2, 'd')).toEqual(['a', 'b', 'd'])
  })

  it('ignores a selection that is not in the vocabulary at all', () => {
    // `all` is the sentinel for "no category filter", not a facet value.
    expect(visibleFacetValues(VALUES, 2, 'all')).toEqual(['a', 'b'])
  })
})

describe('fetchCatalogManifest', () => {
  it('returns the parsed manifest', async () => {
    const fetchMock = vi.fn(async () => jsonResponse(MANIFEST))
    vi.stubGlobal('fetch', fetchMock)

    await expect(
      fetchCatalogManifest('/inspiration-catalog/')
    ).resolves.toEqual(MANIFEST)
    expect(fetchMock).toHaveBeenCalledWith('/inspiration-catalog/manifest.json', {
      signal: undefined,
    })
  })

  it('rejects a non-OK response with the status', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({}, false, 503)))

    await expect(fetchCatalogManifest('/c/')).rejects.toThrow('503')
  })

  it('rejects a manifest without zones', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ pageSize: 48 })))

    await expect(fetchCatalogManifest('/c/')).rejects.toThrow('no zones')
  })
})

describe('fetchCatalogPage', () => {
  it('validates the envelope and falls back to sane defaults', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        jsonResponse({ zone: 'wan-3.0', items: [{ id: 'a' }] })
      )
    )

    await expect(
      fetchCatalogPage({ base: '/c/', zone: 'wan-3.0', page: 0 })
    ).resolves.toEqual({
      zone: 'wan-3.0',
      page: 0,
      pageSize: 1,
      total: 1,
      totalPages: 1,
      items: [{ id: 'a' }],
    })
  })

  it('reads the quarantined page when asked for it', async () => {
    const fetchMock = vi.fn(async (_input: RequestInfo | URL) =>
      jsonResponse({ zone: 'nano-banana-pro', items: [] })
    )
    vi.stubGlobal('fetch', fetchMock)

    await fetchCatalogPage({
      base: '/c/',
      zone: 'nano-banana-pro',
      page: 3,
      quarantined: true,
    })

    expect(String(fetchMock.mock.calls[0]?.[0])).toBe(
      '/c/nano-banana-pro/quarantine-0003.json'
    )
  })

  it('rejects a page without items', async () => {
    vi.stubGlobal('fetch', vi.fn(async () => jsonResponse({ zone: 'x' })))

    await expect(
      fetchCatalogPage({ base: '/c/', zone: 'x', page: 0 })
    ).rejects.toThrow('no items')
  })
})

describe('isAbortError', () => {
  it('recognizes a cancelled request and nothing else', () => {
    expect(isAbortError(new DOMException('Aborted', 'AbortError'))).toBe(true)
    expect(
      isAbortError(Object.assign(new Error('canceled'), { name: 'CanceledError' }))
    ).toBe(true)
    expect(isAbortError(new Error('network down'))).toBe(false)
    expect(isAbortError(null)).toBe(false)
  })
})
