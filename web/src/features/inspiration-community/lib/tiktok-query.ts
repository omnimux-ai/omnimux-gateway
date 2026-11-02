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
 * Query contract of the inspiration service's TikTok ad feed.
 *
 * The service takes one single value per categorical dimension (`country`,
 * `type`, `traffic_type`), an inclusive integer range per numeric one
 * (`duration_min`/`_max`, `views_min`/`_max`) and a publish window
 * (`posted_after`/`posted_before`). Every filter on this page is therefore a
 * single choice: two countries, or two non-adjacent duration buckets, cannot be
 * told apart in one request, and the bounding box of such a selection would
 * answer with rows the visitor never selected.
 *
 * The module is pure — filter state in, request parameters out — so the bucket
 * edges and the publish-window arithmetic are pinned without rendering or
 * network.
 */
import type {
  TikTokContentType,
  TikTokDurationBucket,
  TikTokFilters,
  TikTokPublishedBucket,
  TikTokSort,
  TikTokTrafficType,
  TikTokViewsBucket,
} from '../types'

/** Creatives per request, and per grid page. */
export const TIKTOK_PAGE_SIZE = 30

/** Largest `page_size` the service accepts. */
export const TIKTOK_MAX_PAGE_SIZE = 100

/** No dimension narrowing the feed. */
export const EMPTY_TIKTOK_FILTERS: TikTokFilters = {
  country: null,
  contentType: null,
  duration: null,
  views: null,
  trafficType: null,
  published: null,
}

/** Request parameters of `GET /api/inspiration/v1/public/inspirations`. */
export interface TikTokQueryParams {
  page: number
  page_size: number
  country?: string
  type?: TikTokContentType
  duration_min?: number
  duration_max?: number
  views_min?: number
  views_max?: number
  traffic_type?: TikTokTrafficType
  posted_after?: string
  posted_before?: string
  sort?: TikTokSort
}

/**
 * Inclusive integer-second edges of the clip-length buckets.
 *
 * `<15s` starts at 1 because the service reports `duration: 0` for photo posts
 * and that 0 means "unknown", not "zero seconds" — a bucket starting at 0 would
 * answer `<15s` with every photo post in the corpus.
 */
const DURATION_RANGES: Record<
  TikTokDurationBucket,
  { min: number; max?: number }
> = {
  lt15: { min: 1, max: 14 },
  s15to30: { min: 15, max: 30 },
  s30to60: { min: 31, max: 60 },
  gt60: { min: 61 },
}

/** Inclusive view-count edges of the view buckets. */
const VIEWS_RANGES: Record<
  TikTokViewsBucket,
  { min?: number; max?: number }
> = {
  lt10k: { max: 9_999 },
  k10to100k: { min: 10_000, max: 99_999 },
  k100to1m: { min: 100_000, max: 999_999 },
  m1to10m: { min: 1_000_000, max: 9_999_999 },
  gt10m: { min: 10_000_000 },
}

/** `YYYY-MM-DD` on the visitor's own calendar, as the service expects it. */
export function toDateParam(date: Date): string {
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${date.getFullYear()}-${month}-${day}`
}

/**
 * `date` moved back by whole months, clamped to the target month's last day, so
 * 2026-03-31 minus 6 months is 2025-09-30 rather than an overflow into October.
 */
export function subtractMonths(date: Date, months: number): Date {
  const firstOfTarget = new Date(date.getFullYear(), date.getMonth() - months, 1)
  const lastDayOfTarget = new Date(
    firstOfTarget.getFullYear(),
    firstOfTarget.getMonth() + 1,
    0
  ).getDate()
  firstOfTarget.setDate(Math.min(date.getDate(), lastDayOfTarget))
  return firstOfTarget
}

/**
 * Publish window of a bucket, as the inclusive `posted_after`/`posted_before`
 * pair the service takes.
 *
 * A creative whose `postedAt` is null carries no publish date, so it falls
 * outside every window and is reachable only through "All". That is the
 * service's semantics, and this page does not paper over it.
 */
export function publishedWindow(
  bucket: TikTokPublishedBucket,
  now: Date = new Date()
): { after?: string; before?: string } {
  if (bucket === 'older') {
    return { before: toDateParam(subtractMonths(now, 12)) }
  }
  if (bucket === 'm6') {
    return { after: toDateParam(subtractMonths(now, 6)) }
  }
  return { after: toDateParam(subtractMonths(now, 12)) }
}

/** Request parameters for one page of the feed. */
export function buildTikTokQueryParams(
  filters: TikTokFilters,
  sort: TikTokSort,
  page: number,
  now: Date = new Date()
): TikTokQueryParams {
  const params: TikTokQueryParams = {
    page: Number.isFinite(page) ? Math.max(1, Math.floor(page)) : 1,
    page_size: TIKTOK_PAGE_SIZE,
    sort,
  }

  if (filters.country) params.country = filters.country
  if (filters.contentType) params.type = filters.contentType

  if (filters.duration) {
    const range = DURATION_RANGES[filters.duration]
    params.duration_min = range.min
    if (range.max !== undefined) params.duration_max = range.max
  }

  if (filters.views) {
    const range = VIEWS_RANGES[filters.views]
    if (range.min !== undefined) params.views_min = range.min
    if (range.max !== undefined) params.views_max = range.max
  }

  if (filters.trafficType) params.traffic_type = filters.trafficType

  if (filters.published) {
    const window = publishedWindow(filters.published, now)
    if (window.after) params.posted_after = window.after
    if (window.before) params.posted_before = window.before
  }

  return params
}

/**
 * Parameters of a count probe: one row of a type, so the response's `total`
 * answers "how many" without pulling a page of creatives.
 */
export function buildTikTokCountParams(
  type?: TikTokContentType
): TikTokQueryParams {
  return { page: 1, page_size: 1, ...(type ? { type } : {}) }
}
