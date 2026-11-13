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
 * Hand-built payload rows for the situations the live page does not always
 * show in one request: an absent region, an absent publish date, an absent
 * cover, an absent link, an absent creator, a caption long enough to break a
 * layout, and a creative whose two media fields disagree (an `mp4` in
 * `mediaUrls` with no `embedUrl` to play it with).
 *
 * The values are shaped like the service's, not invented numbers: the ids are
 * synthetic only so they cannot be confused with a real row.
 */
import type { TikTokAdApiItem, TikTokAdApiPage } from '../../tiktok'

/** A payload row with every field the service documents, overridable per case. */
export function apiRow(
  overrides: Partial<TikTokAdApiItem> = {}
): TikTokAdApiItem {
  return {
    id: 9001,
    type: 'video',
    title: 'A demo creative',
    caption: 'caption text',
    country: 'US',
    category: 'Health & Wellness',
    duration: 24,
    views: 95600000,
    trafficType: 'organic',
    postedAt: '2025-08-08T16:00:00Z',
    coverUrl: '/api/inspiration/v1/public/media/inspiration-covers/9001',
    mediaUrls: ['/api/inspiration/v1/public/media/r2/videos/demo/video.mp4'],
    embedUrl: 'https://www.tiktok.com/player/v1/9001',
    sourceUrl: 'https://www.tiktok.com/@demo/video/9001',
    creator: { name: 'demo_creator', handle: 'demo_creator' },
    ...overrides,
  }
}

/**
 * The extra-long caption case, in the same mixed script and emoji runs as the
 * longest rows in production, and long enough that a card or dialog without a
 * wrapping rule would overflow.
 */
export const LONG_CAPTION = `${'daily routine 🥺 家人们谁懂啊 '.repeat(24)}`.trimEnd()

export const VIDEO_ROW = apiRow({ id: 9001 })

/** A photo post: no clip length, a slide image, and no publish date. */
export const PHOTO_ROW = apiRow({
  id: 9002,
  type: 'image',
  title: 'Photo carousel',
  caption: '',
  coverUrl: '/api/inspiration/v1/public/media/inspiration-covers/9002',
  country: '',
  duration: 0,
  views: 61100000,
  mediaUrls: [
    '/api/inspiration/v1/public/media/r2/slideshows/demo/slide-1.jpg',
    '/api/inspiration/v1/public/media/r2/slideshows/demo/slide-2.jpg',
  ],
  embedUrl: null,
  sourceUrl: null,
  postedAt: null,
})

/** No cover, no creator, an unknown traffic value and an unknown media type. */
export const BARE_ROW = apiRow({
  id: 9003,
  type: 'story',
  title: '',
  caption: LONG_CAPTION,
  country: '',
  category: '',
  duration: 9,
  views: 0,
  trafficType: 'mixed',
  coverUrl: null,
  mediaUrls: [],
  embedUrl: null,
  sourceUrl: null,
  postedAt: 'not-a-date',
  creator: null,
})

/** A video row with an `mp4` but no `embedUrl`: nothing can play it. */
export const UNPLAYABLE_VIDEO_ROW = apiRow({
  id: 9004,
  embedUrl: null,
  sourceUrl: null,
})

export const CONTRACT_PAGE: TikTokAdApiPage = {
  total: 4,
  page: 1,
  page_size: 30,
  page_count: 1,
  items: [VIDEO_ROW, PHOTO_ROW, BARE_ROW, UNPLAYABLE_VIDEO_ROW],
}
