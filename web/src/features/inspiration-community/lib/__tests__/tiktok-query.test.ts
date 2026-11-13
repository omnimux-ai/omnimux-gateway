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
import { describe, expect, it } from 'vitest'

import {
  EMPTY_TIKTOK_FILTERS,
  TIKTOK_MAX_PAGE_SIZE,
  TIKTOK_PAGE_SIZE,
  buildTikTokCountParams,
  buildTikTokQueryParams,
  publishedWindow,
  subtractMonths,
  toDateParam,
} from '../tiktok-query'
import type { TikTokFilters } from '../../types'

function filters(overrides: Partial<TikTokFilters> = {}): TikTokFilters {
  return { ...EMPTY_TIKTOK_FILTERS, ...overrides }
}

describe('buildTikTokQueryParams', () => {
  it('asks only for the page when nothing is filtered', () => {
    const params = buildTikTokQueryParams(EMPTY_TIKTOK_FILTERS, 'views', 1)

    expect(params).toEqual({
      page: 1,
      page_size: TIKTOK_PAGE_SIZE,
      sort: 'views',
    })
  })

  it('keeps the page size within the service cap', () => {
    expect(TIKTOK_PAGE_SIZE).toBeLessThanOrEqual(TIKTOK_MAX_PAGE_SIZE)
  })

  it('passes a single region and a single content type through', () => {
    const params = buildTikTokQueryParams(
      filters({ country: 'DE', contentType: 'image' }),
      'new',
      3
    )

    expect(params).toMatchObject({
      country: 'DE',
      type: 'image',
      sort: 'new',
      page: 3,
    })
  })

  it('turns a region-free filter into an absent parameter, not an empty one', () => {
    const params = buildTikTokQueryParams(filters({ country: null }), 'views', 1)

    expect(params).not.toHaveProperty('country')
    expect(params).not.toHaveProperty('type')
    expect(params).not.toHaveProperty('posted_after')
  })

  it('sends the clip-length bucket as an inclusive second range', () => {
    // `<15s` starts at 1: a duration of 0 means "unknown" for photo posts, so a
    // bucket starting at 0 would answer with every photo post in the corpus.
    expect(
      buildTikTokQueryParams(filters({ duration: 'lt15' }), 'views', 1)
    ).toMatchObject({ duration_min: 1, duration_max: 14 })

    expect(
      buildTikTokQueryParams(filters({ duration: 's15to30' }), 'views', 1)
    ).toMatchObject({ duration_min: 15, duration_max: 30 })

    expect(
      buildTikTokQueryParams(filters({ duration: 's30to60' }), 'views', 1)
    ).toMatchObject({ duration_min: 31, duration_max: 60 })

    // The open-ended bucket carries no upper bound at all.
    const open = buildTikTokQueryParams(filters({ duration: 'gt60' }), 'views', 1)
    expect(open).toMatchObject({ duration_min: 61 })
    expect(open).not.toHaveProperty('duration_max')
  })

  it('sends the view bucket as an inclusive count range', () => {
    expect(
      buildTikTokQueryParams(filters({ views: 'lt10k' }), 'views', 1)
    ).toMatchObject({ views_max: 9_999 })

    expect(
      buildTikTokQueryParams(filters({ views: 'k10to100k' }), 'views', 1)
    ).toMatchObject({ views_min: 10_000, views_max: 99_999 })

    expect(
      buildTikTokQueryParams(filters({ views: 'k100to1m' }), 'views', 1)
    ).toMatchObject({ views_min: 100_000, views_max: 999_999 })

    expect(
      buildTikTokQueryParams(filters({ views: 'm1to10m' }), 'views', 1)
    ).toMatchObject({ views_min: 1_000_000, views_max: 9_999_999 })

    const open = buildTikTokQueryParams(filters({ views: 'gt10m' }), 'views', 1)
    expect(open).toMatchObject({ views_min: 10_000_000 })
    expect(open).not.toHaveProperty('views_max')
  })

  it('sends the traffic bucket with the service vocabulary', () => {
    expect(
      buildTikTokQueryParams(filters({ trafficType: 'ad' }), 'views', 1)
        .traffic_type
    ).toBe('ad')
    expect(
      buildTikTokQueryParams(filters({ trafficType: 'organic' }), 'views', 1)
        .traffic_type
    ).toBe('organic')
  })

  it('clamps a page below the first one', () => {
    expect(buildTikTokQueryParams(filters(), 'views', 0).page).toBe(1)
    expect(buildTikTokQueryParams(filters(), 'views', -4).page).toBe(1)
  })
})

describe('publishedWindow', () => {
  const now = new Date(2026, 8, 12) // 2026-09-12, browser time

  it('bounds the last six months with an inclusive lower date', () => {
    expect(publishedWindow('m6', now)).toEqual({ after: '2026-03-12' })
  })

  it('bounds the last year with an inclusive lower date', () => {
    expect(publishedWindow('y1', now)).toEqual({ after: '2025-09-12' })
  })

  it('bounds everything older than a year with an inclusive upper date', () => {
    expect(publishedWindow('older', now)).toEqual({ before: '2025-09-12' })
  })

  it('clamps a month-end subtraction to the target month last day', () => {
    // 2026-03-31 minus six months is 2025-09-30, not an overflow into October.
    expect(toDateParam(subtractMonths(new Date(2026, 2, 31), 6))).toBe(
      '2025-09-30'
    )
  })

  it('keeps a day that exists in the target month', () => {
    expect(toDateParam(subtractMonths(new Date(2026, 8, 12), 6))).toBe(
      '2026-03-12'
    )
    expect(toDateParam(subtractMonths(new Date(2026, 8, 12), 12))).toBe(
      '2025-09-12'
    )
  })

  it('crosses a year boundary backwards', () => {
    expect(toDateParam(subtractMonths(new Date(2026, 0, 15), 6))).toBe(
      '2025-07-15'
    )
  })

  it('zero-pads single-digit months and days', () => {
    expect(toDateParam(new Date(2026, 0, 5))).toBe('2026-01-05')
  })

  it('reaches the request parameters from the filter state', () => {
    const params = buildTikTokQueryParams(
      filters({ published: 'older' }),
      'views',
      1,
      now
    )

    expect(params).toMatchObject({ posted_before: '2025-09-12' })
    expect(params).not.toHaveProperty('posted_after')
  })
})

describe('buildTikTokCountParams', () => {
  it('asks for a single row so the answer is the total', () => {
    expect(buildTikTokCountParams()).toEqual({ page: 1, page_size: 1 })
  })

  it('narrows a count probe to one content type', () => {
    expect(buildTikTokCountParams('video')).toEqual({
      page: 1,
      page_size: 1,
      type: 'video',
    })
  })
})
