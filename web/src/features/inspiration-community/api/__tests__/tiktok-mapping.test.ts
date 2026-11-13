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
 * Mapping of the inspiration service's payload onto the card's shape.
 *
 * `tiktok-page-live.json` is a verbatim response of
 * `GET /api/inspiration/v1/public/inspirations?page=1&page_size=30&sort=views`
 * captured from production on 2026-09-12, so the common path is pinned against
 * real rows rather than against rows written to match the mapper. The rows in
 * `fixtures/tiktok-rows.ts` cover what one page cannot show at once.
 */
import { describe, expect, it } from 'vitest'

import livePage from './fixtures/tiktok-page-live.json'
import {
  BARE_ROW,
  CONTRACT_PAGE,
  LONG_CAPTION,
  PHOTO_ROW,
  UNPLAYABLE_VIDEO_ROW,
  VIDEO_ROW,
} from './fixtures/tiktok-rows'
import {
  mapTikTokAdItem,
  mapTikTokFeedPage,
  type TikTokAdApiPage,
} from '../tiktok'

const live = livePage as unknown as {
  success: boolean
  data: TikTokAdApiPage
}

describe('mapTikTokAdItem', () => {
  it('keeps a real video row field for field', () => {
    const item = mapTikTokAdItem(VIDEO_ROW)

    expect(item).toMatchObject({
      id: '9001',
      type: 'video',
      title: 'A demo creative',
      country: 'US',
      category: 'Health & Wellness',
      durationSeconds: 24,
      views: 95600000,
      trafficType: 'organic',
      postedAt: '2025-08-08T16:00:00Z',
      coverUrl: '/api/inspiration/v1/public/media/inspiration-covers/9001',
      mediaUrls: ['/api/inspiration/v1/public/media/r2/videos/demo/video.mp4'],
      embedUrl: 'https://www.tiktok.com/player/v1/9001',
      sourceUrl: 'https://www.tiktok.com/@demo/video/9001',
      creator: { name: 'demo_creator', handle: 'demo_creator' },
    })
    expect(item.coverGradient).not.toBe('')
  })

  it('leaves a photo post without a clip length, a region or a publish date', () => {
    const item = mapTikTokAdItem(PHOTO_ROW)

    // `duration: 0` means the length is unknown, not that the post is instant.
    expect(item.durationSeconds).toBeNull()
    expect(item.country).toBe('')
    expect(item.postedAt).toBeNull()
    expect(item.sourceUrl).toBeNull()
    expect(item.embedUrl).toBeNull()
    // The slide images are still carried, even though nothing plays them.
    expect(item.mediaUrls).toEqual([
      '/api/inspiration/v1/public/media/r2/slideshows/demo/slide-1.jpg',
      '/api/inspiration/v1/public/media/r2/slideshows/demo/slide-2.jpg',
    ])
  })

  it('drops blank and unusable fields instead of passing them on', () => {
    const item = mapTikTokAdItem({
      ...BARE_ROW,
      postedAt: 'not-a-date',
      caption: LONG_CAPTION,
    })

    expect(item.coverUrl).toBeNull()
    expect(item.category).toBe('')
    expect(item.creator).toBeNull()
    // A media type and a traffic type outside the documented vocabulary are
    // unknown, not guessed into the nearest known value.
    expect(item.type).toBeNull()
    expect(item.trafficType).toBeNull()
    // Unparseable dates are absent rather than rendered as "Invalid Date".
    expect(item.postedAt).toBeNull()
    // A long caption survives mapping intact: it is not truncated for the UI.
    expect(LONG_CAPTION.length).toBeGreaterThan(400)
    expect(item.caption).toBe(LONG_CAPTION)
  })

  it('reports a real zero view count as zero, not as unknown', () => {
    const item = mapTikTokAdItem({ ...VIDEO_ROW, views: 0 })

    expect(item.views).toBe(0)
  })

  it('treats a missing view count as unknown rather than as zero', () => {
    const item = mapTikTokAdItem({ ...VIDEO_ROW, views: null })

    expect(item.views).toBeNull()
  })

  it('gives the same creative the same cover backdrop every time', () => {
    const first = mapTikTokAdItem(VIDEO_ROW)
    const second = mapTikTokAdItem({ ...VIDEO_ROW })

    expect(first.coverGradient).toBe(second.coverGradient)
    expect(mapTikTokAdItem({ ...VIDEO_ROW, id: 4242 }).coverGradient).not.toBe(
      undefined
    )
  })

  it('ignores a cover URL that is only whitespace', () => {
    expect(mapTikTokAdItem({ ...VIDEO_ROW, coverUrl: '   ' }).coverUrl).toBeNull()
  })
})

describe('mapTikTokFeedPage', () => {
  it('normalizes the contract fixture page', () => {
    const page = mapTikTokFeedPage(CONTRACT_PAGE)

    expect(page.total).toBe(4)
    expect(page.page).toBe(1)
    expect(page.pageSize).toBe(30)
    expect(page.pageCount).toBe(1)
    expect(page.items.map((item) => item.id)).toEqual([
      '9001',
      '9002',
      '9003',
      '9004',
    ])
  })

  it('maps every row of the captured production page', () => {
    const page = mapTikTokFeedPage(live.data)

    expect(page.total).toBe(2954)
    expect(page.page).toBe(1)
    expect(page.pageSize).toBe(30)
    expect(page.pageCount).toBe(99)
    expect(page.items).toHaveLength(30)
    expect(page.items.every((item) => item.coverGradient !== '')).toBe(true)
  })

  it('maps a captured photo post with a zero duration', () => {
    const page = mapTikTokFeedPage(live.data)
    const [first] = page.items

    expect(first).toMatchObject({
      id: '2789',
      type: 'image',
      country: 'DE',
      views: 95600000,
      durationSeconds: null,
      trafficType: 'organic',
      sourceUrl: null,
      embedUrl: null,
    })
    expect(first.coverUrl).toBe(
      '/api/inspiration/v1/public/media/inspiration-covers/2789'
    )
  })

  it('maps a captured row whose region is empty', () => {
    const page = mapTikTokFeedPage(live.data)
    const row = page.items.find((item) => item.id === '1683')

    expect(row?.country).toBe('')
    expect(row?.views).toBe(64000000)
  })

  it('maps a captured row that carries both a source link and a creator', () => {
    const page = mapTikTokFeedPage(live.data)
    const row = page.items.find((item) => item.id === '148')

    // The captured row has an empty `mediaUrls` and still links to the post.
    expect(row?.sourceUrl).toBe(
      'https://www.tiktok.com/@shi.learn/video/7535235994035997966?sync=1787813616_4'
    )
    expect(row?.creator).toEqual({ name: 'Matthew Shi', handle: 'shi.learn' })
    expect(row?.mediaUrls).toEqual([])
  })

  it('derives the page count when the payload omits it', () => {
    const page = mapTikTokFeedPage({
      total: 2954,
      page_size: 30,
      items: [],
    })

    expect(page.pageCount).toBe(99)
    expect(page.page).toBe(1)
  })

  it('rejects a page without a total or without items', () => {
    expect(() =>
      mapTikTokFeedPage({ page_size: 30, items: [] } as unknown as TikTokAdApiPage)
    ).toThrow(/unexpected/i)
    expect(() =>
      mapTikTokFeedPage({ total: 10 } as unknown as TikTokAdApiPage)
    ).toThrow(/unexpected/i)
  })
})

describe('the unplayable video case', () => {
  it('keeps an mp4 row without an embed player, so no page can mount one', () => {
    const item = mapTikTokAdItem(UNPLAYABLE_VIDEO_ROW)

    expect(item.mediaUrls).toHaveLength(1)
    expect(item.mediaUrls[0]?.endsWith('.mp4')).toBe(true)
    expect(item.embedUrl).toBeNull()
  })
})
