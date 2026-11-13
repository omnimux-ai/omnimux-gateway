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
import i18next from 'i18next'
import { describe, expect, it } from 'vitest'

import { facetDisplayName } from '../facets'

describe('facetDisplayName', () => {
  it('returns the Chinese name for Chinese locales', () => {
    expect(facetDisplayName('cinematic-showcase', 'zh')).toBe('电影级展示')
    expect(facetDisplayName('cinematic-showcase', 'zhTW')).toBe('电影级展示')
    expect(facetDisplayName('brand-commercial', 'zh-CN')).toBe('品牌商业广告')
    expect(facetDisplayName('ugc-talking-head', 'zh')).toBe('UGC 口播视频')
    expect(facetDisplayName('food-drink', 'zh')).toBe('餐饮')
    expect(facetDisplayName('cinematic-realistic', 'zh')).toBe('电影级写实')
  })

  it('returns the English name for every other locale', () => {
    expect(facetDisplayName('cinematic-showcase', 'en')).toBe(
      'Cinematic showcase'
    )
    expect(facetDisplayName('brand-commercial', 'fr')).toBe('Brand commercial')
    expect(facetDisplayName('ugc-talking-head', 'ja')).toBe('UGC talking head')
  })

  it('humanizes a key outside the table instead of leaking it raw', () => {
    expect(facetDisplayName('some-new-facet', 'zh')).toBe('Some new facet')
    expect(facetDisplayName('some-new-facet', 'en')).toBe('Some new facet')
  })

  it('translates a canonical Chinese label instead of echoing it', () => {
    // The browse surfaces hand the anchor bar the Chinese label, not the slug,
    // so an English reader used to be shown Chinese chips.
    expect(facetDisplayName('电影级写实风格', 'zh')).toBe('电影级写实风格')
    expect(facetDisplayName('电影级写实风格', 'en')).toBe('Cinematic realism')
    expect(facetDisplayName('文字 / Logo', 'en')).toBe('Text & logo')
    expect(facetDisplayName('交通工具', 'zh-CN')).toBe('交通工具')
    expect(facetDisplayName('交通工具', 'ja')).toBe('Vehicle')
  })

  it('resolves a Chinese name back to the slug it belongs to', () => {
    expect(facetDisplayName('电影级写实', 'en')).toBe('Cinematic realistic')
    expect(facetDisplayName('电影级写实', 'zh')).toBe('电影级写实')
  })

  it('passes a Chinese label with no English name through unchanged', () => {
    // `humanizeFacet` is written for slugs; running a Chinese label through it
    // would mangle it, so an unmapped label keeps its own text.
    expect(facetDisplayName('某个未登记分类', 'en')).toBe('某个未登记分类')
  })

  /**
   * Every Chinese value the published catalog actually ships in its style and
   * subject vocabularies, captured from
   * `https://omnimux.ai/inspiration-catalog/manifest.json`.
   *
   * This is the contract behind the bug: the anchor bar renders these labels
   * verbatim, so any one of them without an English name is a Chinese chip in an
   * English UI. The list is exhaustive on purpose — a new label in the crawl
   * should fail here rather than leak.
   */
  const CATALOG_CHINESE_LABELS = [
    // styles
    '电影级写实风格',
    '复古 / 怀旧胶片',
    '纪录片',
    '超现实 / 梦境',
    '赛博朋克 / 科幻',
    '奇幻 / 魔法',
    '动漫',
    '3D 卡通',
    '2D 动画',
    '中国水墨',
    '定格动画',
    '摄影',
    '插画',
    '电影剧照',
    '动漫 / 漫画',
    '极简主义',
    '3D 渲染',
    '复古 / 怀旧',
    '素描 / 线稿',
    '水彩',
    '漫画小说',
    '水墨 / 国风',
    'Q版 / 萌系',
    '等轴测',
    '油画',
    '像素画',
    // subjects
    '人物 / 角色',
    '运动 / 动作',
    '城市 / 街景',
    '自然 / 风景',
    '人群 / 群体',
    '建筑 / 室内',
    '动物 / 宠物',
    '美食 / 饮品',
    '产品',
    '交通工具',
    '抽象 / VFX',
    '文字 / Logo',
    '角色',
    '人像 / 自拍',
    '文字 / 排版',
    '网红 / 模特',
    '风景 / 自然',
    '团体 / 情侣',
    '动物 / 生物',
    '时尚单品',
    '图表',
    '抽象 / 背景',
    '食物 / 饮品',
    '载具',
  ]

  it('gives every published Chinese label an English name', () => {
    const untranslated = CATALOG_CHINESE_LABELS.filter((label) =>
      /[\u4e00-\u9fa5]/u.test(facetDisplayName(label, 'en'))
    )
    expect(untranslated).toEqual([])
  })

  it('leaves every published Chinese label untouched for Chinese locales', () => {
    for (const label of CATALOG_CHINESE_LABELS) {
      expect(facetDisplayName(label, 'zh')).toBe(label)
      expect(facetDisplayName(label, 'zh-TW')).toBe(label)
    }
  })

  it('covers every facet key the checked-in fixture prints', () => {
    const mapped = [
      'cinematic-showcase',
      'short-film',
      'social-lifestyle',
      'product-marketing',
      'product-ads',
      'profile-avatar',
      'comic-storyboard',
      'poster-flyer',
      'uncategorized',
      'person-character',
      'photography',
      'influencer-model',
      '3d-cartoon',
      'cyberpunk-scifi',
      'fantasy-magical',
    ]

    for (const key of mapped) {
      expect(facetDisplayName(key, 'zh'), `${key} needs a zh name`).not.toBe('')
      expect(facetDisplayName(key, 'zh')).not.toBe(facetDisplayName(key, 'en'))
    }
  })
})

describe('localized facet labels', () => {
  it('uses the requested interface language for slugs and Chinese source labels', async () => {
    const labels = {
      en: 'Poster',
      zhCN: '海报',
      zhTW: '海報',
      fr: 'Affiche',
      ja: 'ポスター',
      ru: 'Плакат',
      vi: 'Áp phích',
    }
    await i18next.init({
      lng: 'en',
      fallbackLng: false,
      resources: Object.fromEntries(
        Object.entries(labels).map(([language, label]) => [
          language,
          { translation: { Poster: label } },
        ])
      ),
    })
    for (const [language, label] of Object.entries(labels)) {
      expect(facetDisplayName('poster', language)).toBe(label)
      expect(facetDisplayName('海报', language)).toBe(label)
    }
    expect(facetDisplayName('poster', 'zh-TW')).toBe('海報')
    expect(facetDisplayName('poster', 'fr')).toBe('Affiche')
    expect(facetDisplayName('poster', 'ja')).toBe('ポスター')
  })
})
