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
 * Client of the inspiration service's public TikTok ad feed.
 *
 *   GET /api/inspiration/v1/public/inspirations   one page of creatives
 *   GET /api/inspiration/v1/public/media/*key     anonymous cover/video bytes
 *
 * The endpoint is anonymous and read-only. This module owns three things and
 * nothing else: the wire shape, the row mapping, and the request. Mapping is
 * deliberately explicit rather than a spread of the payload, because the two
 * vocabularies differ — the service says `postedAt`/`duration`/`creator`, the
 * cards render `postedAt`/`durationSeconds`/`creator`, and every field that can
 * be absent has to be marked absent instead of defaulted into a value the
 * payload never carried.
 *
 * What the media fields really deliver, measured against production:
 * `coverUrl` answers `200 image/jpeg` for every row, `mediaUrls` currently
 * answers `404` for every object under `/r2/…` (upstream, not the gateway), and
 * `embedUrl` carries TikTok's own player for the ~24% of rows that have a real
 * TikTok video. So the cards paint `coverUrl`, and only a row with `embedUrl`
 * offers playback; `mediaUrls` is mapped because it is contract data, not
 * because anything can play it.
 */
import { api } from '@/lib/api'
import { createServerError, requireServerSuccess } from '@/lib/server-error-message'

import { coverGradientFor } from '../lib/media'
import { TIKTOK_PAGE_SIZE, type TikTokQueryParams } from '../lib/tiktok-query'
import type {
  TikTokAdCreator,
  TikTokAdItem,
  TikTokMediaType,
  TikTokTrafficType,
} from '../types'

/** Path of the public feed. */
export const TIKTOK_INSPIRATIONS_PATH =
  '/api/inspiration/v1/public/inspirations'

/** One creative exactly as the service sends it. */
export interface TikTokAdApiItem {
  id: number | string
  type?: string | null
  title?: string | null
  caption?: string | null
  country?: string | null
  category?: string | null
  duration?: number | null
  views?: number | null
  trafficType?: string | null
  postedAt?: string | null
  coverUrl?: string | null
  mediaUrls?: string[] | null
  /**
   * TikTok's own player URL, projected from the collected post. Present for the
   * creatives that really have a playable TikTok video (measured 120 of 500
   * rows), null for the rest.
   */
  embedUrl?: string | null
  sourceUrl?: string | null
  creator?: { name?: string | null; handle?: string | null } | null
}

/** One page of the feed, as the service sends it. */
export interface TikTokAdApiPage {
  total: number
  page?: number
  page_size?: number
  page_count?: number
  items: TikTokAdApiItem[]
}

/** The service's response envelope. */
export interface TikTokInspirationsResponse {
  success: boolean
  message?: string
  data?: TikTokAdApiPage
}

/** One page of the feed, as the grid consumes it. */
export interface TikTokFeedPage {
  total: number
  page: number
  pageSize: number
  pageCount: number
  items: TikTokAdItem[]
}

function asText(value: unknown): string {
  return typeof value === 'string' ? value.trim() : ''
}

function asNumber(value: unknown): number | null {
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function asMediaType(value: unknown): TikTokMediaType | null {
  if (value === 'video' || value === 'image' || value === 'link') return value
  return null
}

function asTrafficType(value: unknown): TikTokTrafficType | null {
  if (value === 'ad' || value === 'organic') return value
  return null
}

/** A count the payload really reported; missing stays missing, never `0`. */
function asCount(value: unknown): number | null {
  const count = asNumber(value)
  if (count === null || count < 0) return null
  return Math.round(count)
}

/**
 * Clip length in whole seconds, or null when the service has none. The service
 * reports `0` for photo posts, and that 0 means unknown rather than instant.
 */
function asDuration(value: unknown): number | null {
  const seconds = asNumber(value)
  if (seconds === null || seconds <= 0) return null
  return Math.round(seconds)
}

/** An ISO timestamp the browser can actually format; anything else is absent. */
function asDateIso(value: unknown): string | null {
  const text = asText(value)
  if (!text || Number.isNaN(Date.parse(text))) return null
  return text
}

function asUrlList(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.map(asText).filter((entry) => entry !== '')
}

function asCreator(value: unknown): TikTokAdCreator | null {
  if (!value || typeof value !== 'object') return null
  const record = value as { name?: unknown; handle?: unknown }
  const name = asText(record.name)
  const handle = asText(record.handle)
  if (!name && !handle) return null
  return { name, handle }
}

/** Map one payload row onto the card's shape. */
export function mapTikTokAdItem(raw: TikTokAdApiItem): TikTokAdItem {
  const id = String(raw.id ?? '')
  return {
    id,
    type: asMediaType(raw.type),
    title: asText(raw.title),
    caption: asText(raw.caption),
    country: asText(raw.country),
    category: asText(raw.category),
    durationSeconds: asDuration(raw.duration),
    views: asCount(raw.views),
    trafficType: asTrafficType(raw.trafficType),
    postedAt: asDateIso(raw.postedAt),
    coverUrl: asText(raw.coverUrl) || null,
    mediaUrls: asUrlList(raw.mediaUrls),
    embedUrl: asText(raw.embedUrl) || null,
    sourceUrl: asText(raw.sourceUrl) || null,
    creator: asCreator(raw.creator),
    coverGradient: coverGradientFor(id),
  }
}

/**
 * Normalize one payload page. `total` and `items` are the two fields the page
 * cannot work without: without them the response is not the contract, and
 * answering with an empty grid would print "0 creatives" for a feed that has
 * 2,954 — a wrong number instead of an error.
 */
export function mapTikTokFeedPage(raw: TikTokAdApiPage): TikTokFeedPage {
  const total = asNumber(raw?.total)
  if (total === null || !Array.isArray(raw?.items)) {
    throw createServerError(raw, 'The ad library returned an unexpected response.')
  }

  const pageSize = asCount(raw.page_size) ?? TIKTOK_PAGE_SIZE
  const page = asCount(raw.page) ?? 1
  const pageCount =
    asCount(raw.page_count) ??
    Math.max(1, Math.ceil(total / Math.max(1, pageSize)))

  return {
    total,
    page,
    pageSize,
    pageCount,
    items: raw.items.map(mapTikTokAdItem),
  }
}

/** Fetch one page of the public feed. */
export async function fetchTikTokFeed(
  params: TikTokQueryParams,
  signal?: AbortSignal
): Promise<TikTokFeedPage> {
  const res = await api.get<TikTokInspirationsResponse>(
    TIKTOK_INSPIRATIONS_PATH,
    { params, signal }
  )
  const body = requireServerSuccess(res.data)
  if (!body?.data) {
    throw createServerError(res.data, 'The ad library returned an unexpected response.')
  }
  return mapTikTokFeedPage(body.data)
}

/**
 * `total` of one count probe, used for the headline figures. The probe asks for
 * a single row, so it costs the same as a stat call and needs no extra contract.
 */
export async function fetchTikTokFeedTotal(
  params: TikTokQueryParams,
  signal?: AbortSignal
): Promise<number> {
  return (await fetchTikTokFeed(params, signal)).total
}
