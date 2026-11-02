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

import type { PromptItem } from '../../types'
import {
  FILTER_ALL,
  buildPageItems,
  collectPromptCategories,
  filterPromptItems,
  filterPromptItemsByQuery,
  groupPromptsByModel,
  hasReportedViewCount,
  loadMoreSlice,
  resolveModelSection,
  sortPromptItems,
} from '../filter'

function promptItem(overrides: Partial<PromptItem>): PromptItem {
  return {
    id: 'topview:seedance-2.5:a',
    slug: 'a',
    modelKey: 'seedance-2.5',
    modelLabel: 'Seedance 2.5',
    mediaType: 'video',
    title: 'A',
    prompt: 'prompt a',
    category: 'cinematic',
    useCases: [],
    styles: [],
    tags: [],
    previewImageUrl: '',
    previewVideoUrl: '',
    posterUrl: '',
    aspectRatio: '16 / 9',
    authorName: 'author',
    sourcePlatform: 'x',
    sourceUrl: '',
    publishedAt: '2026-01-01T00:00:00',
    viewCount: 10,
    originalViewCount: 0,
    ...overrides,
  }
}

describe('buildPageItems', () => {
  it('lists every page when the range fits without truncation', () => {
    expect(buildPageItems(2, 5)).toEqual([1, 2, 3, 4, 5])
  })

  it('collapses both sides for a large page count', () => {
    expect(buildPageItems(50, 10_924)).toEqual([
      1,
      'ellipsis',
      49,
      50,
      51,
      'ellipsis',
      10_924,
    ])
  })

  it('omits the left ellipsis when the current page is near the start', () => {
    expect(buildPageItems(2, 100)).toEqual([1, 2, 3, 'ellipsis', 100])
  })
})

describe('filterPromptItems', () => {
  const items = [
    promptItem({ id: 'a', category: 'cinematic', mediaType: 'video' }),
    promptItem({ id: 'b', category: 'poster-flyer', mediaType: 'image' }),
  ]

  it('returns everything when both filters are "all"', () => {
    expect(
      filterPromptItems(items, { category: FILTER_ALL, mediaType: FILTER_ALL })
    ).toHaveLength(2)
  })

  it('filters by category and media type together', () => {
    expect(
      filterPromptItems(items, { category: 'poster-flyer', mediaType: 'image' })
    ).toEqual([expect.objectContaining({ id: 'b' })])
  })

  it('filters by style matching direct label or slug alias', () => {
    const styledItems = [
      promptItem({ id: 'a', styles: ['anime'] }),
      promptItem({ id: 'b', styles: ['cinematic-realistic'] }),
    ]
    expect(
      filterPromptItems(styledItems, {
        category: FILTER_ALL,
        mediaType: FILTER_ALL,
        style: '动漫',
      })
    ).toEqual([expect.objectContaining({ id: 'a' })])
  })

  it('filters by subject matching direct label or tag alias', () => {
    const taggedItems = [
      promptItem({ id: 'a', tags: ['person-character'] }),
      promptItem({ id: 'b', tags: ['animal-pet'] }),
    ]
    expect(
      filterPromptItems(taggedItems, {
        category: FILTER_ALL,
        mediaType: FILTER_ALL,
        subject: '人物 / 角色',
      })
    ).toEqual([expect.objectContaining({ id: 'a' })])
  })

  it('returns an empty list when nothing matches', () => {
    expect(
      filterPromptItems(items, { category: 'cinematic', mediaType: 'image' })
    ).toEqual([])
  })
})

describe('collectPromptCategories', () => {
  it('returns distinct categories alphabetically', () => {
    const items = [
      promptItem({ id: 'a', category: 'poster-flyer' }),
      promptItem({ id: 'b', category: 'cinematic' }),
      promptItem({ id: 'c', category: 'cinematic' }),
    ]
    expect(collectPromptCategories(items)).toEqual([
      'cinematic',
      'poster-flyer',
    ])
  })
})

describe('sortPromptItems', () => {
  it('orders by publish time descending for "latest"', () => {
    const items = [
      promptItem({ id: 'old', publishedAt: '2025-01-01T00:00:00' }),
      promptItem({ id: 'new', publishedAt: '2026-08-01T00:00:00' }),
    ]
    expect(sortPromptItems(items, 'latest').map((i) => i.id)).toEqual([
      'new',
      'old',
    ])
  })

  it('keeps undated items at the bottom for "latest"', () => {
    const items = [
      promptItem({ id: 'undated', publishedAt: '' }),
      promptItem({ id: 'dated', publishedAt: '2026-08-01T00:00:00' }),
    ]
    expect(sortPromptItems(items, 'latest').map((i) => i.id)).toEqual([
      'dated',
      'undated',
    ])
  })

  it('orders rows with a reported counter by engagement for "hottest"', () => {
    const items = [
      promptItem({ id: 'cold', viewCount: 5, originalViewCount: 5 }),
      promptItem({ id: 'hot', viewCount: 900, originalViewCount: 900 }),
    ]
    expect(sortPromptItems(items, 'hottest').map((i) => i.id)).toEqual([
      'hot',
      'cold',
    ])
  })

  it('does not sink rows whose source reported no counter', () => {
    // The Loomi dataset reports no counters at all, so every one of its rows
    // arrives with `viewCount: 0`. A plain descending sort would place the
    // whole dataset below the single counted row.
    const items = [
      promptItem({ id: 'counted', viewCount: 40, originalViewCount: 40 }),
      promptItem({ id: 'loomi-a', viewCount: 0, originalViewCount: 0 }),
      promptItem({ id: 'loomi-b', viewCount: 0, originalViewCount: 0 }),
      promptItem({ id: 'loomi-c', viewCount: 0, originalViewCount: 0 }),
    ]

    expect(sortPromptItems(items, 'hottest').map((i) => i.id)).toEqual([
      'counted',
      'loomi-a',
      'loomi-b',
      'loomi-c',
    ])
  })

  it('ignores a fabricated stand-in count when ranking "hottest"', () => {
    // `derivedFields: ['viewCount']` marks a Topview/EvoLink row whose count the
    // catalog invented; `originalViewCount` stays 0 for those.
    const items = [
      promptItem({ id: 'fabricated', viewCount: 999_999, originalViewCount: 0 }),
      promptItem({ id: 'reported', viewCount: 12, originalViewCount: 12 }),
    ]

    expect(sortPromptItems(items, 'hottest').map((i) => i.id)).toEqual([
      'reported',
      'fabricated',
    ])
  })

  it('falls back to recency inside the uncounted cohort', () => {
    const items = [
      promptItem({
        id: 'older',
        viewCount: 0,
        originalViewCount: 0,
        publishedAt: '2026-01-01T00:00:00',
      }),
      promptItem({
        id: 'newer',
        viewCount: 0,
        originalViewCount: 0,
        publishedAt: '2026-08-01T00:00:00',
      }),
    ]

    expect(sortPromptItems(items, 'hottest').map((i) => i.id)).toEqual([
      'newer',
      'older',
    ])
  })

  it('does not mutate the input array', () => {
    const items = [
      promptItem({ id: 'a', viewCount: 1 }),
      promptItem({ id: 'b', viewCount: 2 }),
      promptItem({ id: 'c', viewCount: 3 }),
    ]
    sortPromptItems(items, 'hottest')
    expect(items.map((i) => i.id)).toEqual(['a', 'b', 'c'])
  })
})

describe('hasReportedViewCount', () => {
  it('treats a fabricated or absent counter as unknown, never as zero', () => {
    expect(
      hasReportedViewCount(
        promptItem({ id: 'loomi', viewCount: 0, originalViewCount: 0 })
      )
    ).toBe(false)
    expect(
      hasReportedViewCount(
        promptItem({ id: 'fake', viewCount: 420_000, originalViewCount: 0 })
      )
    ).toBe(false)
    expect(
      hasReportedViewCount(
        promptItem({ id: 'real', viewCount: 900, originalViewCount: 900 })
      )
    ).toBe(true)
  })
})

describe('groupPromptsByModel', () => {
  it('keeps section order and drops empty sections', () => {
    const items = [
      promptItem({ id: 'a', modelKey: 'seedance-2.5' }),
      promptItem({ id: 'b', modelKey: 'wan-3.0' }),
    ]
    const groups = groupPromptsByModel(items, [
      { key: 'seedance', label: 'Seedance zone', count: 2 },
      { key: 'image2', label: 'Image 2 zone', count: 1 },
      { key: 'wan30', label: 'Wan 3.0 zone', count: 1 },
    ])
    expect(groups.map((group) => group.section.key)).toEqual([
      'seedance',
      'wan30',
    ])
    expect(groups[1].items.map((i) => i.id)).toEqual(['b'])
  })

  it('folds two zones of one 专區 into a single group', () => {
    const items = [
      promptItem({ id: 'a', modelKey: 'seedance-2.5' }),
      promptItem({ id: 'b', modelKey: 'seedance-2.0' }),
    ]
    const groups = groupPromptsByModel(items, [
      { key: 'seedance', label: 'Seedance zone', count: 2 },
    ])
    expect(groups).toHaveLength(1)
    expect(groups[0].items.map((item) => item.id)).toEqual(['a', 'b'])
  })

  it('keeps an unlisted zone under its own key', () => {
    const items = [promptItem({ id: 'a', modelKey: 'some-future-model' })]
    const groups = groupPromptsByModel(items, [
      { key: 'some-future-model', label: 'Some future model', count: 1 },
    ])
    expect(groups.map((group) => group.section.key)).toEqual([
      'some-future-model',
    ])
  })
})






describe('filterPromptItemsByQuery', () => {
  const library = [
    promptItem({
      id: 'a',
      title: 'Ice dragon knight',
      prompt: 'cinematic flight over snowy mountains',
      authorName: 'Saif Ai',
      tags: ['fantasy-magical'],
    }),
    promptItem({
      id: 'b',
      title: 'Garlic bread tutorial',
      prompt: 'a 45 second how-to video',
      authorName: 'Rduck',
      tags: ['food-drink'],
    }),
  ]

  it('matches the title case-insensitively', () => {
    expect(
      filterPromptItemsByQuery(library, 'ICE DRAGON').map((item) => item.id)
    ).toEqual(['a'])
  })

  it('matches the prompt body, the author and the tags', () => {
    expect(filterPromptItemsByQuery(library, 'snowy').map((i) => i.id)).toEqual(
      ['a']
    )
    expect(filterPromptItemsByQuery(library, 'rduck').map((i) => i.id)).toEqual(
      ['b']
    )
    expect(
      filterPromptItemsByQuery(library, 'food-drink').map((i) => i.id)
    ).toEqual(['b'])
  })

  it('returns everything for an empty or whitespace-only query', () => {
    expect(filterPromptItemsByQuery(library, '')).toHaveLength(2)
    expect(filterPromptItemsByQuery(library, '   ')).toHaveLength(2)
  })

  it('returns nothing when no field matches', () => {
    expect(filterPromptItemsByQuery(library, 'submarine')).toEqual([])
  })
})

describe('resolveModelSection', () => {
  const sections = [
    { key: 'seedance-2.5', label: 'Seedance 2.5', count: 48 },
    { key: 'gpt-image-2', label: 'GPT Image 2', count: 40 },
  ]

  it('resolves a known model key to its section', () => {
    expect(resolveModelSection(sections, 'seedance-2.5')?.label).toBe(
      'Seedance 2.5'
    )
  })

  it('resolves an unknown slug to null', () => {
    expect(resolveModelSection(sections, 'kling-3.0')).toBeNull()
  })

  it('resolves an empty slug to null', () => {
    expect(resolveModelSection(sections, '  ')).toBeNull()
  })
})

describe('loadMoreSlice', () => {
  const items = Array.from({ length: 30 }, (_, index) => index + 1)

  it('reveals the first batch and reports what is left', () => {
    const slice = loadMoreSlice(items, 24, 24)
    expect(slice.items).toHaveLength(24)
    expect(slice.items[0]).toBe(1)
    expect(slice.hasMore).toBe(true)
    expect(slice.nextBatchSize).toBe(6)
  })

  it('reveals the remainder once the window grows past the list', () => {
    const slice = loadMoreSlice(items, 48, 24)
    expect(slice.items).toHaveLength(30)
    expect(slice.visibleCount).toBe(30)
    expect(slice.hasMore).toBe(false)
    expect(slice.nextBatchSize).toBe(0)
  })

  it('never shows fewer than one batch when the count is clamped to the total', () => {
    const slice = loadMoreSlice(items, 0, 24)
    expect(slice.items).toHaveLength(24)
    expect(slice.visibleCount).toBe(24)
  })

  it('reports an exhausted list for empty input', () => {
    const slice = loadMoreSlice([], 24, 24)
    expect(slice.items).toEqual([])
    expect(slice.total).toBe(0)
    expect(slice.hasMore).toBe(false)
  })
})
