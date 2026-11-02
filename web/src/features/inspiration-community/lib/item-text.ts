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
 * The row text a given locale should read.
 *
 * The catalog publishes every row twice — Chinese in `title` / `description`
 * and English in `titleEn` / `descriptionEn` — because the crawls themselves are
 * written in different languages and one field can only serve one of them.
 *
 * The split follows the same rule the facet vocabulary already uses
 * (`facetDisplayName`): `zh` and `zh-TW` read Chinese, every other language
 * reads English. A row that carries no text in the asked-for language falls back
 * to the other one rather than rendering blank, so a partially covered row stays
 * readable instead of losing its caption.
 */
import type { PromptItem } from '../types'

/** True for the locales that read the Chinese column of the catalog. */
export function readsChinese(language: string): boolean {
  return language.toLowerCase().startsWith('zh')
}

function pick(zh: string | undefined, en: string | undefined, language: string): string {
  const chinese = (zh ?? '').trim()
  const english = (en ?? '').trim()
  return readsChinese(language) ? chinese || english : english || chinese
}

/** Display title of a row in `language`. */
export function itemTitle(item: PromptItem, language: string): string {
  return pick(item.title, item.titleEn, language)
}

/** Display summary of a row in `language`; empty when the row carries none. */
export function itemDescription(item: PromptItem, language: string): string {
  return pick(item.description, item.descriptionEn, language)
}
