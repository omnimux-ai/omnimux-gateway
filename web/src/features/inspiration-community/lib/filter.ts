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
 * Pure filtering / sorting / pagination for the Inspiration Community pages.
 *
 * Everything here is side-effect free so the behaviour is unit-testable
 * without rendering (`lib/__tests__/`). Components own only the state; the
 * derivation lives in this module.
 */
import type {
  PromptItem,
  PromptMediaType,
  PromptSection,
  PromptSort,
} from '../types'
import { zoneGroupOf } from './zone-groups'

/**
 * Preview cards per model section on L1 (`/inspiration`).
 *
 * L1 is a *preview* of each model library, not the library itself: the section
 * header prints the model's full total and the section tail links to L2 for
 * the rest. There is deliberately no pagination on L1.
 */
export const PROMPT_PREVIEW_COUNT = 20

/** Sentinel used by the single-select dropdowns for "no constraint". */
export const FILTER_ALL = 'all'

export interface PromptFilterState {
  /** `category` facet, or `FILTER_ALL`. */
  category: string
  /** `FILTER_ALL` | `image` | `video`. */
  mediaType: PromptMediaType | typeof FILTER_ALL
  /** `style` facet (e.g. '2D 动画', '动漫'), or `FILTER_ALL`. */
  style?: string
  /** `subject` facet (e.g. '人物 / 角色', '产品'), or `FILTER_ALL`. */
  subject?: string
}

export const DEFAULT_PROMPT_FILTERS: PromptFilterState = {
  category: FILTER_ALL,
  mediaType: FILTER_ALL,
  style: FILTER_ALL,
  subject: FILTER_ALL,
}

/**
 * Page numbers with an ellipsis on each side of the current window, matching
 * the reference library's `1 2 3 … 10924` control.
 */
export function buildPageItems(
  current: number,
  totalPages: number,
  siblings = 1
): Array<number | 'ellipsis'> {
  const total = Math.max(1, totalPages)
  const active = Math.min(Math.max(1, current), total)
  const windowSize = siblings * 2 + 5

  if (total <= windowSize) {
    return Array.from({ length: total }, (_, index) => index + 1)
  }

  const left = Math.max(2, active - siblings)
  const right = Math.min(total - 1, active + siblings)
  const items: Array<number | 'ellipsis'> = [1]

  if (left > 2) items.push('ellipsis')
  for (let page = left; page <= right; page += 1) items.push(page)
  if (right < total - 1) items.push('ellipsis')
  items.push(total)

  return items
}

/** Match a facet value against a filter, where `all` means "no constraint". */
export function matchesFacet(value: string, filter: string): boolean {
  return filter === FILTER_ALL || value === filter
}

/** Distinct `category` facets present in the data, alphabetically sorted. */
export function collectPromptCategories(
  items: readonly PromptItem[]
): string[] {
  const seen = new Set<string>()
  for (const item of items) {
    if (item.category) seen.add(item.category)
  }
  return [...seen].sort((a, b) => a.localeCompare(b))
}

const STYLE_ALIASES: Record<string, string[]> = {
  '电影级写实风格': ['cinematic-realistic', 'cinematic'],
  '复古 / 怀旧胶片': ['vintage-retro', 'retro-vintage'],
  '纪录片': ['documentary'],
  '超现实 / 梦境': ['surreal-dreamlike'],
  '赛博朋克 / 科幻': ['cyberpunk-scifi', 'cyberpunk-sci-fi'],
  '奇幻 / 魔法': ['fantasy-magical', 'fantasy'],
  '动漫': ['anime', 'anime-manga'],
  '3D 卡通': ['3d-cartoon', '3d-render'],
  '2D 动画': ['2d-animation'],
  '中国水墨': ['chinese-ink', '水墨 / 国风'],
  '定格动画': ['stop-motion'],
}

const SUBJECT_ALIASES: Record<string, string[]> = {
  '人物 / 角色': [
    'person-character',
    'character',
    'portrait-selfie',
    '人像 / 自拍',
    '角色',
  ],
  '运动 / 动作': ['sports-action', 'action'],
  '城市 / 街景': ['cityscape-urban', 'cityscape-street', 'cityscape'],
  '自然 / 风景': ['nature-landscape', 'landscape-nature', '风景 / 自然'],
  '人群 / 群体': ['crowd-group', 'group-couple', '团体 / 情侣'],
  '建筑 / 室内': ['architecture-interior', 'interior'],
  '动物 / 宠物': ['animal-pet', 'animal-creature', '动物 / 生物'],
  '美食 / 饮品': ['food-drink-v', 'food-drink', '食物 / 饮品'],
  产品: ['product-v', 'product'],
  交通工具: ['vehicle-v', 'vehicle', '载具'],
  '抽象 / VFX': ['abstract-vfx-v', 'vfx', '抽象 / 背景'],
  '文字 / Logo': ['text-logo-v', 'text-typography', '文字 / 排版'],
}

function matchesFilterFacet(
  itemValues: readonly string[] | undefined,
  filter: string | undefined,
  aliases: Record<string, string[]>
): boolean {
  if (!filter || filter === FILTER_ALL) return true
  if (!itemValues || itemValues.length === 0) return false
  const target = filter.trim().toLowerCase()
  const allowed = new Set([
    target,
    ...(aliases[filter] ?? []).map((s) => s.toLowerCase()),
  ])
  return itemValues.some((val) => allowed.has(val.trim().toLowerCase()))
}

/** Apply the shared (all-section) filters of the prompt gallery. */
export function filterPromptItems(
  items: readonly PromptItem[],
  filters: PromptFilterState
): PromptItem[] {
  return items.filter(
    (item) =>
      matchesFacet(item.category, filters.category) &&
      matchesFacet(item.mediaType, filters.mediaType) &&
      matchesFilterFacet(item.styles, filters.style, STYLE_ALIASES) &&
      matchesFilterFacet(item.tags, filters.subject, SUBJECT_ALIASES)
  )
}

/**
 * Whether the row's source reported an engagement counter at all.
 *
 * `originalViewCount` is the raw crawl value and is `0` whenever the source
 * feed carries no counter — the whole Loomi dataset (28,406 displayable rows)
 * has none, and the Topview/EvoLink mappers fabricate a stand-in `viewCount`
 * for another 31,599 rows while leaving `originalViewCount` at `0`. Only a
 * value above zero is a number a human can compare, so ranking keys off this
 * predicate rather than off `viewCount`, where a missing counter arrives as a
 * `0` that would read as "nobody looked".
 */
export function hasReportedViewCount(item: PromptItem): boolean {
  return item.originalViewCount > 0
}

function byPublishedAtDesc(a: PromptItem, b: PromptItem): number {
  if (!a.publishedAt) return 1
  if (!b.publishedAt) return -1
  return b.publishedAt.localeCompare(a.publishedAt)
}

/**
 * Interleave two already-ordered runs by relative position: the k-th item of a
 * run lands where its own pace puts it, so a short run is never buried at the
 * tail and its head always leads. Ties keep the primary (ranked) item, which
 * is what keeps a real top count on top.
 *
 * Neither run is buried at the tail nor promoted above the other, which is the
 * whole point of using it for "hottest" — see `sortPromptItems`.
 */
export function mergeByRelativeRank(
  primary: readonly PromptItem[],
  secondary: readonly PromptItem[]
): PromptItem[] {
  if (primary.length === 0) return [...secondary]
  if (secondary.length === 0) return [...primary]

  const merged: PromptItem[] = []
  let left = 0
  let right = 0

  while (left < primary.length || right < secondary.length) {
    const leftRank = left / primary.length
    const rightRank = right / secondary.length
    const takePrimary =
      left < primary.length &&
      (right >= secondary.length || leftRank <= rightRank)
    if (takePrimary) {
      merged.push(primary[left])
      left += 1
    } else {
      merged.push(secondary[right])
      right += 1
    }
  }

  return merged
}

/**
 * `latest` orders by `publishedAt` (missing timestamps sink to the bottom).
 *
 * `hottest` ranks only the rows whose source reported a counter, and orders
 * the rest by recency, then merges the two runs by relative position. Comparing
 * a fabricated or absent counter against a real one is what made the Loomi rows
 * look cold: every one of them reports `viewCount: 0`, so a plain descending
 * sort put a 23,730-row dataset below every row of the source that does report
 * numbers.
 */
export function sortPromptItems(
  items: readonly PromptItem[],
  sort: PromptSort
): PromptItem[] {
  if (sort === 'hottest') {
    const ranked = [...items]
      .filter(hasReportedViewCount)
      .sort((a, b) => b.viewCount - a.viewCount)
    const unranked = [...items]
      .filter((item) => !hasReportedViewCount(item))
      .sort(byPublishedAtDesc)
    return mergeByRelativeRank(ranked, unranked)
  }
  return [...items].sort(byPublishedAtDesc)
}

export interface PromptSectionGroup {
  section: PromptSection
  items: PromptItem[]
}

/**
 * Split the filtered items back into one group per known section, preserving the
 * section order. Sections with no matching item are dropped so the anchor bar
 * never points at an empty region.
 *
 * An item is bucketed by the key the given sections actually use: the 专區 id
 * where the section list is grouped (the gallery navigates by 专區), the zone id
 * where it is per-zone (a model page is addressed by zone). Both shapes reach
 * this function, so the key is chosen from the sections rather than assumed.
 */
export function groupPromptsByModel(
  items: readonly PromptItem[],
  sections: readonly PromptSection[]
): PromptSectionGroup[] {
  const sectionKeys = new Set(sections.map((section) => section.key))
  const keyOf = (item: PromptItem): string => {
    const group = zoneGroupOf(item.modelKey)
    if (group && sectionKeys.has(group.id)) return group.id
    return item.modelKey
  }

  const byModel = new Map<string, PromptItem[]>()
  for (const item of items) {
    const key = keyOf(item)
    const bucket = byModel.get(key)
    if (bucket) bucket.push(item)
    else byModel.set(key, [item])
  }

  const groups: PromptSectionGroup[] = []
  for (const section of sections) {
    const bucket = byModel.get(section.key)
    if (bucket && bucket.length > 0) {
      groups.push({ section, items: bucket })
    }
  }
  return groups
}

/**
 * Free-text search over the fields a visitor can actually see on a card or
 * open in the preview: title, prompt body, author, category and tags.
 *
 * The query is trimmed and case-folded; an empty query matches everything, so
 * the caller can pass the raw input value straight through.
 */
export function filterPromptItemsByQuery(
  items: readonly PromptItem[],
  query: string
): PromptItem[] {
  const needle = query.trim().toLowerCase()
  if (!needle) return [...items]

  return items.filter((item) => {
    // Both language columns are searched on purpose: the row carries its text
    // twice, and a visitor typing in either language expects the row to match
    // regardless of which locale the UI happens to be in.
    const haystack = [
      item.title,
      item.titleEn,
      item.description,
      item.descriptionEn,
      item.prompt,
      item.authorName,
      item.modelLabel,
      item.category,
      ...item.tags,
    ]
    return haystack.some(
      (field) => (field ?? '').toLowerCase().includes(needle)
    )
  })
}

/**
 * Resolve a `/inspiration/models/$modelSlug` path segment to its model
 * section. Returns `null` for an unknown slug so L2 can render a not-found
 * state instead of an empty grid.
 */
export function resolveModelSection(
  sections: readonly PromptSection[],
  slug: string
): PromptSection | null {
  const wanted = slug.trim().toLowerCase()
  if (!wanted) return null
  return sections.find((section) => section.key === wanted) ?? null
}

/**
 * The visible window of a "Load more" list: the first `visibleCount` items
 * plus whether more remain. `visibleCount` is clamped to the list length so a
 * shrinking filter can never leave a stale oversized window.
 */
export interface LoadMoreSlice<T> {
  items: T[]
  /** How many items are currently revealed. */
  visibleCount: number
  total: number
  hasMore: boolean
  /** How many items the next "Load more" click reveals. */
  nextBatchSize: number
}

export function loadMoreSlice<T>(
  items: readonly T[],
  visibleCount: number,
  batchSize: number
): LoadMoreSlice<T> {
  const size = Math.max(1, Math.floor(batchSize))
  const total = items.length
  const visible = Math.min(Math.max(size, Math.floor(visibleCount)), total)

  return {
    items: items.slice(0, visible),
    visibleCount: visible,
    total,
    hasMore: visible < total,
    nextBatchSize: Math.min(size, total - visible),
  }
}
