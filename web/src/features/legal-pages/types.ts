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
 * Public marketing-site legal documents (`/terms`, `/privacy`, `/refunds`,
 * `/acceptable-use`).
 *
 * These ship with the frontend, so they are versioned with the product and
 * readable without an API call. That is deliberately different from
 * `@/features/legal`, which renders the administrator-configured documents
 * served by `/api/user-agreement` and `/api/privacy-policy`.
 */

/** A string authored once per language the public site promises. */
export interface LocalizedText {
  /** English — the authoritative wording for every legal document. */
  en: string
  /** Simplified Chinese. */
  zh: string
  zhTW: string
  fr: string
  ja: string
  ru: string
  vi: string
}

export interface LegalDocumentContent {
  /** Rendered as the page's single `<h1>`. */
  title: LocalizedText
  /** ISO `YYYY-MM-DD`; the page formats it for the reader's language. */
  updated: string
  /** Markdown body: numbered `##` sections, paragraphs and `-` lists. */
  body: LocalizedText
}

/** A document resolved for one reader: no language variants left. */
export interface ResolvedLegalDocument {
  title: string
  updated: string
  body: string
}
