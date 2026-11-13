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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  useCatalogZoneRows,
  useInspirationCatalogPreview,
} from '../use-inspiration-catalog'
import { useInspirationManifest } from '../use-inspiration-manifest'

/* ── Fixture catalog ───────────────────────────────────────────────────── */

const ZONE = {
  id: 'wan-3.0',
  label: 'Wan 3.0',
  mediaType: 'video' as const,
  count: 3,
  pages: 2,
  quarantined: 1,
  quarantinePages: 1,
  categories: ['product-ad'],
  styles: [],
  subjects: [],
  useCases: [],
}

const MANIFEST = {
  version: 1,
  generatedAt: '2026-09-12T00:00:00.000Z',
  pageSize: 2,
  total: 3,
  quarantinedTotal: 1,
  sources: [],
  zones: [ZONE],
}

function page(zone: string, index: number, ids: string[], total: number) {
  return {
    zone,
    page: index,
    pageSize: 2,
    total,
    totalPages: Math.ceil(total / 2),
    items: ids.map((id) => ({ id, title: id })),
  }
}

const PAGES: Record<string, unknown> = {
  'manifest.json': MANIFEST,
  'wan-3.0/page-0000.json': page('wan-3.0', 0, ['a', 'b'], 3),
  'wan-3.0/page-0001.json': page('wan-3.0', 1, ['c'], 3),
  'wan-3.0/quarantine-0000.json': page('wan-3.0', 0, ['dirty'], 1),
}

/** Every catalog URL requested so far, with the signal it was given. */
let requests: Array<{ url: string; signal?: AbortSignal }> = []

function jsonResponse(payload: unknown): Response {
  return { ok: true, status: 200, json: async () => payload } as unknown as Response
}

/** Serve `PAGES`; unknown URLs answer 404 and aborted requests reject. */
function serveCatalog(
  overrides: Record<string, () => Promise<Response>> = {}
): void {
  requests = []
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input).replace('/inspiration-catalog/', '')
      requests.push({ url, signal: init?.signal ?? undefined })
      const override = overrides[url]
      if (override) return override()
      if (init?.signal?.aborted) {
        throw new DOMException('Aborted', 'AbortError')
      }
      const payload = PAGES[url]
      if (payload === undefined) {
        return { ok: false, status: 404, json: async () => ({}) } as Response
      }
      return jsonResponse(payload)
    })
  )
}

/** A response that only resolves when the test says so. */
function deferredResponse(): {
  promise: Promise<Response>
  resolve: (payload: unknown) => void
} {
  let settle: ((response: Response) => void) | undefined
  return {
    promise: new Promise<Response>((resolve) => {
      settle = resolve
    }),
    resolve: (payload: unknown) => settle?.(jsonResponse(payload)),
  }
}

function urls(): string[] {
  return requests.map((request) => request.url)
}

afterEach(() => {
  vi.unstubAllGlobals()
})

/* ── Probes ────────────────────────────────────────────────────────────── */

function ManifestProbe() {
  const state = useInspirationManifest()
  const [, setTick] = useState(0)

  return (
    <div>
      <span data-testid='total'>{state.manifest?.total ?? ''}</span>
      <span data-testid='error'>{state.error?.message ?? ''}</span>
      <span data-testid='loading'>{String(state.loading)}</span>
      <button type='button' onClick={() => setTick((value) => value + 1)}>
        rerender
      </button>
      <button type='button' onClick={state.reload}>
        reload
      </button>
    </div>
  )
}

function ZoneProbe(props: { zone: string | null; quarantined?: boolean }) {
  const rows = useCatalogZoneRows(props.zone, {
    quarantined: props.quarantined === true,
  })

  return (
    <div>
      <span data-testid='items'>{rows.items.map((item) => item.id).join(',')}</span>
      <span data-testid='pages'>{rows.loadedPages}</span>
      <span data-testid='total'>{rows.total}</span>
      <span data-testid='totalPages'>{rows.totalPages}</span>
      <span data-testid='hasMore'>{String(rows.hasMore)}</span>
      <span data-testid='loading'>{String(rows.loading)}</span>
      <span data-testid='error'>{rows.error?.message ?? ''}</span>
      <button type='button' onClick={rows.loadMore}>
        load more
      </button>
      <button type='button' onClick={rows.reload}>
        reload
      </button>
    </div>
  )
}

function PreviewProbe() {
  const preview = useInspirationCatalogPreview()
  return (
    <div>
      <span data-testid='zones'>
        {Object.keys(preview.rowsByZone).sort().join(',')}
      </span>
      <span data-testid='rows'>
        {Object.values(preview.rowsByZone)
          .flat()
          .map((item) => item.id)
          .join(',')}
      </span>
      <span data-testid='errors'>{Object.keys(preview.zoneErrors).join(',')}</span>
      <span data-testid='loading'>{String(preview.loading)}</span>
    </div>
  )
}

/* ── Tests ─────────────────────────────────────────────────────────────── */

describe('useInspirationManifest', () => {
  it('loads the manifest once and keeps it across re-renders', async () => {
    serveCatalog()
    render(<ManifestProbe />)

    expect(await screen.findByText('3')).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'rerender' }))
    await userEvent.click(screen.getByRole('button', { name: 'rerender' }))

    expect(urls()).toEqual(['manifest.json'])
  })

  it('reports a failure and retries it', async () => {
    let attempts = 0
    serveCatalog({
      'manifest.json': async () => {
        attempts += 1
        if (attempts === 1) return { ok: false, status: 500, json: async () => ({}) } as Response
        return jsonResponse(MANIFEST)
      },
    })
    render(<ManifestProbe />)

    expect(await screen.findByText(/Catalog request failed/)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'reload' }))

    expect(await screen.findByText('3')).toBeInTheDocument()
    expect(urls()).toEqual(['manifest.json', 'manifest.json'])
  })
})

describe('useCatalogZoneRows', () => {
  it('requests nothing until the zone is known', async () => {
    serveCatalog()
    render(<ZoneProbe zone={null} />)

    await waitFor(() => expect(screen.getByTestId('loading')).toHaveTextContent('false'))
    expect(urls()).toEqual([])
  })

  it('holds page 0 and appends the next page only when load more runs', async () => {
    serveCatalog()
    render(<ZoneProbe zone='wan-3.0' />)

    await waitFor(() => expect(screen.getByTestId('items')).toHaveTextContent('a,b'))
    expect(screen.getByTestId('hasMore')).toHaveTextContent('true')
    expect(urls()).toEqual(['wan-3.0/page-0000.json'])

    await userEvent.click(screen.getByRole('button', { name: 'load more' }))

    await waitFor(() =>
      expect(screen.getByTestId('items')).toHaveTextContent('a,b,c')
    )
    expect(screen.getByTestId('pages')).toHaveTextContent('2')
    expect(screen.getByTestId('total')).toHaveTextContent('3')
    expect(screen.getByTestId('hasMore')).toHaveTextContent('false')
    expect(urls()).toEqual([
      'wan-3.0/page-0000.json',
      'wan-3.0/page-0001.json',
    ])
  })

  it('drops a second load-more click while a page is in flight', async () => {
    const deferred = deferredResponse()
    serveCatalog({ 'wan-3.0/page-0001.json': () => deferred.promise })
    render(<ZoneProbe zone='wan-3.0' />)
    await waitFor(() => expect(screen.getByTestId('items')).toHaveTextContent('a,b'))

    await userEvent.click(screen.getByRole('button', { name: 'load more' }))
    await userEvent.click(screen.getByRole('button', { name: 'load more' }))

    deferred.resolve(page('wan-3.0', 1, ['c'], 3))
    await waitFor(() =>
      expect(screen.getByTestId('items')).toHaveTextContent('a,b,c')
    )

    expect(urls().filter((url) => url.endsWith('page-0001.json'))).toHaveLength(1)
  })

  it('loads the quarantined set only when it is asked for', async () => {
    serveCatalog()
    render(<ZoneProbe zone='wan-3.0' quarantined />)

    await waitFor(() =>
      expect(screen.getByTestId('items')).toHaveTextContent('dirty')
    )
    expect(urls()).toEqual(['wan-3.0/quarantine-0000.json'])
    expect(screen.getByTestId('hasMore')).toHaveTextContent('false')
  })

  it('surfaces a page failure and recovers on reload', async () => {
    let attempts = 0
    serveCatalog({
      'wan-3.0/page-0000.json': async () => {
        attempts += 1
        if (attempts === 1) return { ok: false, status: 500, json: async () => ({}) } as Response
        return jsonResponse(page('wan-3.0', 0, ['a', 'b'], 3))
      },
    })
    render(<ZoneProbe zone='wan-3.0' />)

    expect(await screen.findByText(/Catalog request failed/)).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'reload' }))

    await waitFor(() => expect(screen.getByTestId('items')).toHaveTextContent('a,b'))
    expect(screen.getByTestId('error')).toHaveTextContent('')
  })

  it('aborts the in-flight request when the page unmounts', async () => {
    const deferred = deferredResponse()
    serveCatalog({ 'wan-3.0/page-0000.json': () => deferred.promise })
    const view = render(<ZoneProbe zone='wan-3.0' />)
    await waitFor(() => expect(requests).toHaveLength(1))

    view.unmount()

    expect(requests[0]?.signal?.aborted).toBe(true)
    deferred.resolve(page('wan-3.0', 0, ['a'], 3))
  })
})

describe('useInspirationCatalogPreview', () => {
  it('loads the first page of every zone in the manifest', async () => {
    serveCatalog()
    render(<PreviewProbe />)

    expect(await screen.findByText('a,b')).toBeInTheDocument()
    expect(screen.getByTestId('zones')).toHaveTextContent('wan-3.0')
    expect(screen.getByTestId('errors')).toHaveTextContent('')
    expect(urls()).toEqual(['manifest.json', 'wan-3.0/page-0000.json'])
  })

  it('reports a zone whose page failed instead of dropping it silently', async () => {
    serveCatalog({
      'wan-3.0/page-0000.json': async () =>
        ({ ok: false, status: 500, json: async () => ({}) }) as Response,
    })
    render(<PreviewProbe />)

    await waitFor(() =>
      expect(screen.getByTestId('errors')).toHaveTextContent('wan-3.0')
    )
    expect(screen.getByTestId('loading')).toHaveTextContent('false')
    expect(screen.getByTestId('rows')).toHaveTextContent('')
  })
})
