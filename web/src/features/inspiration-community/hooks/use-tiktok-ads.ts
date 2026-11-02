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
 * Data source of the TikTok ad library page (`/inspiration/tiktok`).
 *
 * The feed is paged by the service, so the page cursor is part of the server
 * query rather than a slice of a local array. That makes "which page am I on"
 * part of the filter state: any edit to the filters or the sort defines a new
 * result set, and the cursor must go back to page 1 with it — otherwise the
 * visitor would land on page 7 of a *different* query. The reducer below is the
 * single place that rule lives.
 */
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useCallback, useEffect, useReducer } from 'react'

import { fetchTikTokFeed } from '../api/tiktok'
import { isAbortError } from '../lib/catalog'
import { EMPTY_TIKTOK_FILTERS, buildTikTokQueryParams } from '../lib/tiktok-query'
import type { TikTokFilters, TikTokAdItem, TikTokSort } from '../types'

/** Root of every ad-library query key. */
export const TIKTOK_ADS_QUERY_KEY = ['inspiration', 'tiktok', 'ads'] as const

/** What the grid needs on screen, plus the cursor that produced it. */
export interface TikTokFeedState {
  filters: TikTokFilters
  sort: TikTokSort
  page: number
}

export const TIKTOK_FEED_INITIAL_STATE: TikTokFeedState = {
  filters: EMPTY_TIKTOK_FILTERS,
  sort: 'views',
  page: 1,
}

export type TikTokFeedAction =
  | { type: 'filters'; filters: TikTokFilters }
  | { type: 'sort'; sort: TikTokSort }
  | { type: 'page'; page: number }
  | { type: 'reset' }

/**
 * Filter state of the ad library.
 *
 * `filters` and `sort` describe the result set, so both reset the cursor; only
 * `page` moves it. Without that, a filter edited on page 7 would ask the
 * service for page 7 of the filtered feed.
 */
export function tiktokFeedReducer(
  state: TikTokFeedState,
  action: TikTokFeedAction
): TikTokFeedState {
  switch (action.type) {
    case 'filters':
      return { ...state, filters: action.filters, page: 1 }
    case 'sort':
      return { ...state, sort: action.sort, page: 1 }
    case 'page':
      return { ...state, page: Math.max(1, action.page) }
    default:
      return TIKTOK_FEED_INITIAL_STATE
  }
}

export interface TikTokAdsController {
  state: TikTokFeedState
  items: TikTokAdItem[]
  /** Result count of the *current* query, from the service's `total`. */
  total: number | null
  /** Number of pages the service reports for the current query. */
  pageCount: number | null
  /** True only for the first load; page and filter changes keep the old rows. */
  isLoading: boolean
  /** A request is in flight, including background page/filter refetches. */
  isFetching: boolean
  error: Error | null
  setFilters: (filters: TikTokFilters) => void
  setSort: (sort: TikTokSort) => void
  setPage: (page: number) => void
  reset: () => void
  refetch: () => void
}

/** One page of the public feed, driven by the filter state above. */
export function useTikTokAds(): TikTokAdsController {
  const [state, dispatch] = useReducer(
    tiktokFeedReducer,
    TIKTOK_FEED_INITIAL_STATE
  )
  const params = buildTikTokQueryParams(state.filters, state.sort, state.page)

  const query = useQuery({
    queryKey: [...TIKTOK_ADS_QUERY_KEY, params],
    queryFn: ({ signal }) => fetchTikTokFeed(params, signal),
    // Paging must not throw the grid away: the previous page stays on screen
    // while the next one loads, so the layout does not collapse and come back.
    placeholderData: keepPreviousData,
  })

  // A cancelled request (route change, param change mid-flight) must not
  // surface as a page-level failure — hide it and re-issue the query.
  const cancelled = isAbortError(query.error)
  const { refetch: refetchQuery } = query

  useEffect(() => {
    if (!cancelled) return
    void refetchQuery()
  }, [cancelled, refetchQuery])

  const setFilters = useCallback((filters: TikTokFilters) => {
    dispatch({ type: 'filters', filters })
  }, [])
  const setSort = useCallback((sort: TikTokSort) => {
    dispatch({ type: 'sort', sort })
  }, [])
  const setPage = useCallback((page: number) => {
    dispatch({ type: 'page', page })
  }, [])
  const reset = useCallback(() => {
    dispatch({ type: 'reset' })
  }, [])
  const refetch = useCallback(() => {
    void query.refetch()
  }, [query])

  return {
    state,
    items: query.data?.items ?? [],
    total: query.data?.total ?? null,
    pageCount: query.data?.pageCount ?? null,
    isLoading: query.isPending || cancelled,
    isFetching: query.isFetching || cancelled,
    error: cancelled ? null : query.error,
    setFilters,
    setSort,
    setPage,
    reset,
    refetch,
  }
}
