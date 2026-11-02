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
 * The cursor rules of the ad library.
 *
 * Filters and sort describe *which* result set is on screen, so editing either
 * has to put the cursor back to page 1: without it, a filter edited on page 7
 * would request page 7 of a different query. `page` alone may move the cursor.
 */
import { describe, expect, it } from 'vitest'

import {
  TIKTOK_FEED_INITIAL_STATE,
  tiktokFeedReducer,
  type TikTokFeedState,
} from '../use-tiktok-ads'

const FILTERED: TikTokFeedState = {
  filters: {
    country: 'US',
    contentType: 'video',
    duration: 'lt15',
    views: 'gt10m',
    trafficType: 'ad',
    published: 'm6',
  },
  sort: 'new',
  page: 5,
}

describe('tiktokFeedReducer', () => {
  it('returns to page 1 when a filter changes', () => {
    const next = tiktokFeedReducer(FILTERED, {
      type: 'filters',
      filters: { ...FILTERED.filters, country: 'DE' },
    })

    expect(next.filters.country).toBe('DE')
    expect(next.page).toBe(1)
    expect(next.sort).toBe('new')
  })

  it('returns to page 1 when the sort changes', () => {
    const next = tiktokFeedReducer(FILTERED, { type: 'sort', sort: 'views' })

    expect(next.sort).toBe('views')
    expect(next.page).toBe(1)
    expect(next.filters).toBe(FILTERED.filters)
  })

  it('keeps the filters and the sort when only the page moves', () => {
    const next = tiktokFeedReducer(FILTERED, { type: 'page', page: 9 })

    expect(next.page).toBe(9)
    expect(next.filters).toBe(FILTERED.filters)
    expect(next.sort).toBe('new')
  })

  it('refuses a page below the first one', () => {
    expect(tiktokFeedReducer(FILTERED, { type: 'page', page: 0 }).page).toBe(1)
    expect(tiktokFeedReducer(FILTERED, { type: 'page', page: -3 }).page).toBe(1)
  })

  it('clears every dimension and the cursor on reset', () => {
    const next = tiktokFeedReducer(FILTERED, { type: 'reset' })

    expect(next).toEqual(TIKTOK_FEED_INITIAL_STATE)
    expect(next.page).toBe(1)
    expect(Object.values(next.filters).every((value) => value === null)).toBe(
      true
    )
  })
})
