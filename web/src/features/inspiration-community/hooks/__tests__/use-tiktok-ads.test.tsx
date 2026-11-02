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
 * The page cursor of the ad library, exercised through the real query hook.
 *
 * The network boundary (`@/lib/http-client`) is the only thing mocked, so the
 * reducer, the parameter builder and the query key all run for real: what is
 * asserted is the request the service actually receives.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { EMPTY_TIKTOK_FILTERS } from '../../lib/tiktok-query'
import { useTikTokAds } from '../use-tiktok-ads'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/lib/http-client', () => ({ api: { get } }))

interface Params {
  page: number
  page_size: number
  type?: string
  country?: string
}

/** Every request the hook makes, in order, with its query parameters. */
function requestedParams(): Params[] {
  return get.mock.calls.map(
    ([, config]) => (config as { params: Params }).params
  )
}

function feedResponse(params: Params) {
  return {
    data: {
      success: true,
      message: '',
      data: {
        total: 2954,
        page: params.page,
        page_size: params.page_size,
        page_count: 99,
        items:
          params.page_size === 1
            ? []
            : [
                {
                  id: 2789,
                  type: 'image',
                  title: 'A creative',
                  caption: '',
                  country: 'DE',
                  category: 'Health & Wellness',
                  duration: 0,
                  views: 95600000,
                  trafficType: 'organic',
                  postedAt: '2025-08-08T16:00:00Z',
                  coverUrl: '/cover/2789',
                  mediaUrls: [],
                  embedUrl: null,
                  sourceUrl: null,
                  creator: null,
                },
              ],
      },
    },
  }
}

function wrapper(props: { children: ReactNode }) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
}

function renderAds() {
  return renderHook(() => useTikTokAds(), { wrapper })
}

afterEach(() => {
  get.mockReset()
})

describe('useTikTokAds', () => {
  it('requests the first page of the feed on the default sort', async () => {
    get.mockImplementation((_url: string, config: { params: Params }) =>
      Promise.resolve(feedResponse(config.params))
    )

    const { result } = renderAds()

    await waitFor(() => expect(result.current.items).toHaveLength(1))
    expect(requestedParams()[0]).toMatchObject({ page: 1, page_size: 30 })
    expect(result.current.total).toBe(2954)
    expect(result.current.pageCount).toBe(99)
  })

  it('asks the service for the page the visitor picked', async () => {
    get.mockImplementation((_url: string, config: { params: Params }) =>
      Promise.resolve(feedResponse(config.params))
    )

    const { result } = renderAds()
    await waitFor(() => expect(result.current.items).toHaveLength(1))

    result.current.setPage(2)

    await waitFor(() =>
      expect(requestedParams().some((params) => params.page === 2)).toBe(true)
    )
    expect(result.current.state.page).toBe(2)
  })

  it('returns to page 1 when a filter changes after paging', async () => {
    get.mockImplementation((_url: string, config: { params: Params }) =>
      Promise.resolve(feedResponse(config.params))
    )

    const { result } = renderAds()
    await waitFor(() => expect(result.current.items).toHaveLength(1))

    result.current.setPage(4)
    await waitFor(() => expect(result.current.state.page).toBe(4))

    const before = requestedParams().length
    result.current.setFilters({ ...EMPTY_TIKTOK_FILTERS, country: 'US' })

    await waitFor(() => expect(requestedParams().length).toBeGreaterThan(before))
    const latest = requestedParams().at(-1)
    expect(latest).toMatchObject({ page: 1, country: 'US' })
    expect(result.current.state.page).toBe(1)
  })

  it('drops the region parameter when the filter is cleared', async () => {
    get.mockImplementation((_url: string, config: { params: Params }) =>
      Promise.resolve(feedResponse(config.params))
    )

    const { result } = renderAds()
    await waitFor(() => expect(result.current.items).toHaveLength(1))

    result.current.setFilters({ ...EMPTY_TIKTOK_FILTERS, country: 'US' })
    await waitFor(() =>
      expect(requestedParams().some((params) => params.country === 'US')).toBe(
        true
      )
    )

    result.current.reset()

    await waitFor(() => {
      const latest = requestedParams().at(-1)
      expect(latest).not.toHaveProperty('country')
    })
  })

  it('surfaces a failing feed as an error instead of an empty grid', async () => {
    get.mockRejectedValue(new Error('Request failed with status code 503'))

    const { result } = renderAds()

    await waitFor(() => expect(result.current.error).not.toBeNull())
    expect(result.current.items).toEqual([])
    expect(result.current.total).toBeNull()
  })
})
