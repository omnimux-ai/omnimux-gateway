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
// fork: shared TikTok tool page types, URL map, and SEO locale tags. Kept in
// a leaf module so ./locales/* never form a circular import with ./content.

export type ToolKind = 'video' | 'photo' | 'transcript'
export type ToolLocale = 'en' | 'zh-CN' | 'zh-TW' | 'ja' | 'es' | 'id'

export interface ToolPageContent {
  path: string
  /** SEO <title>. */
  title: string
  /** SEO meta description. */
  description: string
  h1: string
  subtitle: string
  stepsTitle: string
  steps: { title: string; body: string }[]
  faqTitle: string
  faq: { q: string; a: string }[]
  related: { label: string; href: string }[]
}

export const TOOL_PATHS: Record<ToolLocale, Record<ToolKind, string>> = {
  en: {
    video: '/tools/tiktok-video-downloader',
    photo: '/tools/tiktok-photo-downloader',
    transcript: '/tools/tiktok-transcript-generator',
  },
  'zh-CN': {
    video: '/zh/tools/tiktok-video-downloader',
    photo: '/zh/tools/tiktok-photo-downloader',
    transcript: '/zh/tools/tiktok-transcript-generator',
  },
  'zh-TW': {
    video: '/zh-tw/tools/tiktok-video-downloader',
    photo: '/zh-tw/tools/tiktok-photo-downloader',
    transcript: '/zh-tw/tools/tiktok-transcript-generator',
  },
  ja: {
    video: '/ja/tools/tiktok-video-downloader',
    photo: '/ja/tools/tiktok-photo-downloader',
    transcript: '/ja/tools/tiktok-transcript-generator',
  },
  es: {
    video: '/es/tools/tiktok-video-downloader',
    photo: '/es/tools/tiktok-photo-downloader',
    transcript: '/es/tools/tiktok-transcript-generator',
  },
  id: {
    video: '/id/tools/tiktok-video-downloader',
    photo: '/id/tools/tiktok-photo-downloader',
    transcript: '/id/tools/tiktok-transcript-generator',
  },
}

export const TOOL_LOCALES: ToolLocale[] = [
  'en',
  'zh-CN',
  'zh-TW',
  'ja',
  'es',
  'id',
]

export const TOOL_LOCALE_TAGS: Record<
  ToolLocale,
  { hreflang: string; og: string }
> = {
  en: { hreflang: 'en', og: 'en_US' },
  'zh-CN': { hreflang: 'zh-Hans', og: 'zh_CN' },
  'zh-TW': { hreflang: 'zh-Hant', og: 'zh_TW' },
  ja: { hreflang: 'ja', og: 'ja_JP' },
  es: { hreflang: 'es', og: 'es_ES' },
  id: { hreflang: 'id', og: 'id_ID' },
}
