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
import { normalizeInterfaceLanguage, toIntlLocale } from '@/i18n/languages'

import type {
  LegalDocumentContent,
  LocalizedText,
  ResolvedLegalDocument,
} from '../types'

/** Every interface language has an authored legal translation. */
export type LegalContentLanguage = keyof LocalizedText

export function resolveContentLanguage(
  language?: string | null
): LegalContentLanguage {
  const normalized = normalizeInterfaceLanguage(language)
  return normalized === 'zhCN' ? 'zh' : (normalized as LegalContentLanguage)
}

export function resolveLocalizedText(
  text: LocalizedText,
  language?: string | null
): string {
  return text[resolveContentLanguage(language)]
}

/**
 * `YYYY-MM-DD` rendered in the reader's language. Parsed as UTC so a reader
 * behind or ahead of UTC never sees the previous or next day.
 */
function formatUpdatedDate(updated: string, language?: string | null): string {
  const parsed = new Date(`${updated}T00:00:00Z`)
  if (Number.isNaN(parsed.getTime())) return updated
  return new Intl.DateTimeFormat(toIntlLocale(language), {
    dateStyle: 'long',
    timeZone: 'UTC',
  }).format(parsed)
}

export function resolveLegalDocument(
  content: LegalDocumentContent,
  language?: string | null
): ResolvedLegalDocument {
  return {
    title: resolveLocalizedText(content.title, language),
    updated: formatUpdatedDate(content.updated, language),
    body: resolveLocalizedText(content.body, language),
  }
}
