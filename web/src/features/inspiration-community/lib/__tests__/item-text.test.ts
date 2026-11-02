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
import { itemDescription, itemTitle, readsChinese } from '../item-text'

function promptItem(overrides: Partial<PromptItem>): PromptItem {
  return {
    id: 'topview:seedance-2.5:a',
    slug: 'a',
    modelKey: 'seedance-2.5',
    modelLabel: 'Seedance 2.5',
    mediaType: 'video',
    title: '冰龙骑士电影级画面',
    titleEn: 'Ice Dragon Knight Cinematic Visuals',
    description: '描述',
    descriptionEn: 'Description',
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

describe('readsChinese', () => {
  it('treats every Chinese locale as reading the Chinese column', () => {
    expect(readsChinese('zh')).toBe(true)
    expect(readsChinese('zh-CN')).toBe(true)
    expect(readsChinese('zh-TW')).toBe(true)
    expect(readsChinese('ZH-HANS')).toBe(true)
  })

  it('treats every other language as reading the English column', () => {
    expect(readsChinese('en')).toBe(false)
    expect(readsChinese('fr')).toBe(false)
    expect(readsChinese('ja')).toBe(false)
    expect(readsChinese('ru')).toBe(false)
    expect(readsChinese('vi')).toBe(false)
  })
})

describe('itemTitle', () => {
  it('returns the Chinese title for a Chinese locale and the English one otherwise', () => {
    const item = promptItem({})
    expect(itemTitle(item, 'zh')).toBe('冰龙骑士电影级画面')
    expect(itemTitle(item, 'zh-TW')).toBe('冰龙骑士电影级画面')
    expect(itemTitle(item, 'en')).toBe('Ice Dragon Knight Cinematic Visuals')
    expect(itemTitle(item, 'fr')).toBe('Ice Dragon Knight Cinematic Visuals')
  })

  it('falls back to the other language when the asked-for one is missing', () => {
    // Rows predating the bilingual catalog carry no English text; an English
    // reader must still see a caption rather than a blank tile.
    const legacy = promptItem({ titleEn: undefined })
    expect(itemTitle(legacy, 'en')).toBe('冰龙骑士电影级画面')

    const onlyEnglish = promptItem({ title: '', titleEn: 'English only' })
    expect(itemTitle(onlyEnglish, 'zh')).toBe('English only')
  })
})

describe('itemDescription', () => {
  it('returns the summary in the language the locale asks for', () => {
    const item = promptItem({})
    expect(itemDescription(item, 'zh')).toBe('描述')
    expect(itemDescription(item, 'en')).toBe('Description')
  })

  it('returns an empty string when the row carries no summary at all', () => {
    const bare = promptItem({ description: undefined, descriptionEn: undefined })
    expect(itemDescription(bare, 'zh')).toBe('')
    expect(itemDescription(bare, 'en')).toBe('')
  })
})
