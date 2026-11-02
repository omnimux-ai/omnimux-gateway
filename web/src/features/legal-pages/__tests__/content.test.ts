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

import { acceptableUseContent } from '../content/acceptable-use'
import { privacyContent } from '../content/privacy'
import { refundsContent } from '../content/refunds'
import { termsContent } from '../content/terms'
import {
  resolveContentLanguage,
  resolveLegalDocument,
  resolveLocalizedText,
} from '../lib/content'
import type { LegalDocumentContent } from '../types'

const document: LegalDocumentContent = {
  title: {
    en: 'Terms of Service',
    zh: '服务条款',
    zhTW: '服務條款',
    fr: 'Conditions',
    ja: '利用規約',
    ru: 'Условия',
    vi: 'Điều khoản',
  },
  updated: '2026-10-08',
  body: {
    en: 'English body',
    zh: '中文正文',
    zhTW: '繁體正文',
    fr: 'Corps français',
    ja: '日本語の本文',
    ru: 'Русский текст',
    vi: 'Nội dung tiếng Việt',
  },
}

const ALL_DOCUMENTS: [string, LegalDocumentContent][] = [
  ['terms', termsContent],
  ['privacy', privacyContent],
  ['refunds', refundsContent],
  ['acceptable-use', acceptableUseContent],
]

describe('resolveContentLanguage', () => {
  it('distinguishes simplified and traditional Chinese', () => {
    expect(resolveContentLanguage('zhCN')).toBe('zh')
    expect(resolveContentLanguage('zh-CN')).toBe('zh')
    expect(resolveContentLanguage('zhTW')).toBe('zhTW')
    expect(resolveContentLanguage('zh-TW')).toBe('zhTW')
  })

  it('resolves each non-Chinese interface language directly', () => {
    for (const language of ['en', 'fr', 'ru', 'ja', 'vi']) {
      expect(resolveContentLanguage(language)).toBe(language)
    }
  })

  it('falls back to English for a missing or unsupported language', () => {
    expect(resolveContentLanguage(undefined)).toBe('en')
    expect(resolveContentLanguage(null)).toBe('en')
    expect(resolveContentLanguage('')).toBe('en')
    expect(resolveContentLanguage('klingon')).toBe('en')
  })
})

describe('resolveLocalizedText', () => {
  it('returns the text matching the resolved language', () => {
    expect(resolveLocalizedText(document.title, 'zhCN')).toBe('服务条款')
    expect(resolveLocalizedText(document.title, 'ja')).toBe('利用規約')
  })
})

describe('resolveLegalDocument', () => {
  it('resolves title, body and date together', () => {
    const zh = resolveLegalDocument(document, 'zhCN')
    expect(zh.title).toBe('服务条款')
    expect(zh.body).toBe('中文正文')

    const en = resolveLegalDocument(document, 'fr')
    expect(en.title).toBe('Conditions')
    expect(en.body).toBe('Corps français')
  })

  it('renders the same calendar day in every language', () => {
    const en = resolveLegalDocument(document, 'en').updated
    const zhCN = resolveLegalDocument(document, 'zhCN').updated
    const zhTW = resolveLegalDocument(document, 'zhTW').updated

    // Parsed as UTC, so no reader sees the neighbouring day.
    for (const rendered of [en, zhCN, zhTW]) {
      expect(rendered).toContain('2026')
      expect(rendered).toContain('8')
      expect(rendered).not.toBe(document.updated)
    }
    // The Chinese interfaces share a date format, but not with English.
    expect(en).not.toBe(zhCN)
  })

  it('keeps an unparseable date verbatim instead of rendering Invalid Date', () => {
    const broken: LegalDocumentContent = { ...document, updated: 'not-a-date' }
    expect(resolveLegalDocument(broken, 'en').updated).toBe('not-a-date')
  })
})

describe('published legal documents', () => {
  it.each(ALL_DOCUMENTS)(
    '%s ships every interface translation with the same sections and links',
    (_name, content) => {
      expect(content.title.en.length).toBeGreaterThan(0)
      expect(content.title.zh.length).toBeGreaterThan(0)
      expect(content.updated).toMatch(/^\d{4}-\d{2}-\d{2}$/)

      for (const language of ['en', 'zhCN', 'zhTW', 'fr', 'ja', 'ru', 'vi']) {
        const resolved = resolveLegalDocument(content, language)
        expect(resolved.title.length).toBeGreaterThan(0)
        expect(resolved.body.length).toBeGreaterThan(500)
        if (language !== 'en') expect(resolved.body).not.toBe(content.body.en)
        const structure = (body: string) => ({
          sections: [...body.matchAll(/^## (\d+)\./gm)].map(
            (match) => match[1]
          ),
          links: [...body.matchAll(/\]\(([^)]+)\)/g)]
            .map((match) => match[1])
            .sort(),
          bullets: [...body.matchAll(/^\s*- /gm)].length,
          emails: [...body.matchAll(/hello@omnimux\.ai/g)].length,
        })
        expect(structure(resolved.body)).toEqual(structure(content.body.en))
        // Section headings only; the page owns the H1.
        expect(resolved.body).toContain('## 1.')
        expect(resolved.body).not.toContain('\n# ')
      }
    }
  )
})
