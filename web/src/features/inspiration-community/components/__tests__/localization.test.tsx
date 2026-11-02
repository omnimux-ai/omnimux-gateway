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
 * Copy the ad library builds from feature constants rather than from a literal
 * `t('...')` call site: the country names behind `REGION_LABEL_KEYS`, the
 * content types behind `CONTENT_TYPE_LABEL_KEYS` and the traffic-type labels
 * behind `TRAFFIC_LABEL_KEYS`. A missing or English-only entry there would print
 * hardcoded English in every language.
 *
 * These keys are asserted as strings on purpose: for them the translation IS
 * the contract, so a locale that silently falls back to English must fail.
 *
 * The dropdown options themselves live inside a Base UI popup that does not
 * open under jsdom, so the rendered popup copy is verified in browser QA
 * instead; here the rendered filter-bar labels plus the label lookups are
 * covered.
 *
 * A dedicated i18next instance is used instead of `@/i18n/config`, because the
 * app module initializes asynchronously and `src/test-setup.ts` already
 * installs an empty `en` bundle for the unrelated component tests. The feed's
 * network boundary is mocked, so the gallery itself renders without a service.
 */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen } from '@testing-library/react'
import { createInstance, type i18n as I18nInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, beforeAll, describe, expect, it, vi } from 'vitest'

import enBundle from '@/i18n/locales/en.json'
import frBundle from '@/i18n/locales/fr.json'
import jaBundle from '@/i18n/locales/ja.json'
import ruBundle from '@/i18n/locales/ru.json'
import viBundle from '@/i18n/locales/vi.json'
import zhTWBundle from '@/i18n/locales/zh-TW.json'
import zhBundle from '@/i18n/locales/zh.json'

import {
  CONTENT_TYPE_LABEL_KEYS,
  DURATION_LABEL_KEYS,
  PUBLISHED_LABEL_KEYS,
  REGION_CODES,
  REGION_LABEL_KEYS,
  SORT_LABEL_KEYS,
  TRAFFIC_LABEL_KEYS,
  VIEWS_LABEL_KEYS,
  regionLabelKey,
} from '../../constants'
import { TikTokAdGallery } from '../tiktok-ad-gallery'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/lib/http-client', () => ({ api: { get } }))

// The page chrome (site header, auth store, status query) is outside this
// feature; the copy contract lives in the gallery body.
vi.mock('@/components/layout', () => ({
  PublicLayout: (props: { children: React.ReactNode }) => (
    <div>{props.children}</div>
  ),
}))

const LOCALES = ['en', 'zh', 'zh-TW', 'fr', 'ja', 'ru', 'vi'] as const

/**
 * A locale catalog is a flat key -> copy map in practice, but the checked-in
 * JSON also carries a few nested groups, so reads go through `copyOf`.
 */
const CATALOGS: Record<string, Record<string, unknown>> = {
  en: enBundle.translation,
  zh: zhBundle.translation,
  'zh-TW': zhTWBundle.translation,
  fr: frBundle.translation,
  ja: jaBundle.translation,
  ru: ruBundle.translation,
  vi: viBundle.translation,
}

let i18n: I18nInstance

beforeAll(async () => {
  i18n = createInstance()
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    supportedLngs: [...LOCALES],
    nonExplicitSupportedLngs: true,
    load: 'currentOnly',
    nsSeparator: false,
    resources: Object.fromEntries(
      Object.entries(CATALOGS).map(([code, translation]) => [
        code,
        { translation },
      ])
    ),
    interpolation: { escapeValue: false },
  })
})

/** Flat copy lookup; a nested group never counts as a translation. */
function copyOf(locale: string, key: string): string | undefined {
  const value = CATALOGS[locale][key]
  return typeof value === 'string' ? value : undefined
}

/** Region codes the dropdown offers, and the country key behind each. */
const REGION_NAMES: Record<string, string> = {
  US: 'United States',
  GB: 'United Kingdom',
  CA: 'Canada',
  DE: 'Germany',
  NG: 'Nigeria',
  PH: 'Philippines',
  FR: 'France',
  AU: 'Australia',
  NL: 'Netherlands',
  IT: 'Italy',
  SE: 'Sweden',
  ES: 'Spain',
  BR: 'Brazil',
}

/** The labels this suite treats as copy contracts beyond the option tables. */
const CHROME_KEYS = [
  'Region',
  'Content type',
  'Clip length',
  'Views',
  'Traffic',
  'Published',
  'Sort',
  'Creator',
  'Play video',
  'View original post',
  'Last 6 months',
  'Last year',
  'Over a year ago',
  'No creatives in the ad library yet',
  'New creatives appear here as soon as they are collected.',
  'Ad creatives come from the OmniMux TikTok data service.',
  'The TikTok ad library is not enabled on this instance.',
  'The TikTok ad library is temporarily unavailable.',
]

/** One empty page: this suite checks copy, not rows. */
function emptyPage() {
  return {
    data: {
      success: true,
      message: '',
      data: {
        total: 2954,
        page: 1,
        page_size: 30,
        page_count: 99,
        items: [],
      },
    },
  }
}

function renderAdLibrary() {
  get.mockReset()
  get.mockImplementation(() => Promise.resolve(emptyPage()))

  const root = createRootRoute()
  const home = createRoute({
    getParentRoute: () => root,
    path: '/',
    component: () => null,
  })
  const tiktok = createRoute({
    getParentRoute: () => root,
    path: '/inspiration/tiktok',
    component: () => <TikTokAdGallery />,
  })
  const router = createRouter({
    routeTree: root.addChildren([home, tiktok]),
    history: createMemoryHistory({ initialEntries: ['/inspiration/tiktok'] }),
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  return render(
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>
        <RouterProvider router={router} />
      </I18nextProvider>
    </QueryClientProvider>
  )
}

afterEach(async () => {
  await i18n.changeLanguage('en')
})

describe('TikTok ad library localization', () => {
  it('maps every offered region to a country key with copy in all 7 locales', () => {
    expect(REGION_CODES.length).toBeGreaterThan(10)

    for (const code of REGION_CODES) {
      const key = regionLabelKey(code)
      expect(key, `${code} has no region key`).toBe(REGION_NAMES[code])

      for (const locale of LOCALES) {
        const copy = copyOf(locale, key)
        // A handful of country names are legitimately identical across
        // languages (`Canada` in en/fr, `Philippines` in en/fr/vi), so identity
        // with the key only proves a fallback for the locales whose script
        // differs.
        expect(copy, `${locale} is missing "${key}"`).toBeTruthy()
        const sameScript = locale === 'en' || locale === 'fr' || locale === 'vi'
        if (!sameScript) {
          expect(
            copy,
            `${locale} falls back to the English country name "${key}"`
          ).not.toBe(key)
        }
      }
    }
  })

  it('renders the region filter with a per-locale country name, not English', () => {
    const expected: Array<[string, string]> = [
      ['en', 'US · United States'],
      ['zh', 'US · 美国'],
      ['zh-TW', 'US · 美國'],
      ['fr', 'US · États-Unis'],
      ['ja', 'US · アメリカ合衆国'],
      ['ru', 'US · США'],
      ['vi', 'US · Hoa Kỳ'],
    ]

    for (const [code, label] of expected) {
      const key = REGION_LABEL_KEYS.US
      expect(
        `${REGION_CODES[0]} · ${copyOf(code, key)}`,
        `${code} region option label`
      ).toBe(label)
    }
  })

  it('keeps the paid-traffic label distinct from the billing Paid key', () => {
    // The bare `Paid` key is a settled-payment state in the billing surfaces;
    // the traffic bucket must not reuse it.
    expect(TRAFFIC_LABEL_KEYS.ad).toBe('Paid traffic')

    const expected: Array<[string, string, string]> = [
      ['en', 'Paid', 'Paid'],
      ['zh', '付费流量', '已支付'],
      ['zh-TW', '付費流量', '已支付'],
      ['fr', 'Trafic payant', 'Payé'],
      ['ja', '有料トラフィック', '支払済み'],
      ['ru', 'Платный трафик', 'Оплачено'],
      ['vi', 'Lưu lượng trả phí', 'Đã thanh toán'],
    ]

    for (const [code, traffic, billing] of expected) {
      expect(
        copyOf(code, TRAFFIC_LABEL_KEYS.ad),
        `${code} paid-traffic copy`
      ).toBe(traffic)
      expect(copyOf(code, 'Paid'), `${code} billing Paid copy`).toBe(billing)
      if (code !== 'en') {
        expect(
          traffic,
          `${code} paid traffic must differ from billing`
        ).not.toBe(billing)
      }
    }
  })

  it('has copy for every option label the ad library renders', () => {
    const labelKeys = [
      ...Object.values(REGION_LABEL_KEYS),
      ...Object.values(CONTENT_TYPE_LABEL_KEYS),
      ...Object.values(DURATION_LABEL_KEYS),
      ...Object.values(VIEWS_LABEL_KEYS),
      ...Object.values(TRAFFIC_LABEL_KEYS),
      ...Object.values(PUBLISHED_LABEL_KEYS),
      ...Object.values(SORT_LABEL_KEYS),
    ]

    expect(labelKeys.length).toBeGreaterThan(20)

    for (const locale of LOCALES) {
      for (const key of labelKeys) {
        expect(
          copyOf(locale, key),
          `${locale} is missing option label "${key}"`
        ).toBeTruthy()
      }
    }
  })

  it('has per-locale copy for the filter labels and the detail layer', () => {
    for (const locale of LOCALES) {
      for (const key of CHROME_KEYS) {
        expect(copyOf(locale, key), `${locale} is missing "${key}"`).toBeTruthy()
        if (locale === 'en') continue
        // The label itself is the contract: a locale that still prints the
        // English source has missing copy, not "close enough".
        expect(copyOf(locale, key), `${locale} still prints "${key}"`).not.toBe(
          key
        )
      }
    }
  })

  it('renders the filter bar in the active language', async () => {
    const samples: Array<[string, string[]]> = [
      [
        'en',
        [
          'Region',
          'Content type',
          'Clip length',
          'Views',
          'Traffic',
          'Published',
          'Sort',
        ],
      ],
      ['zh', ['地区', '内容类型', '时长', '播放量', '流量类型', '已发布', '排序']],
      [
        'zh-TW',
        ['地區', '內容類型', '時長', '播放量', '流量類型', '已發佈', '排序'],
      ],
      [
        'fr',
        [
          'Région',
          'Type de contenu',
          'Durée du clip',
          'Vues',
          'Type de trafic',
          'Publié',
          'Trier',
        ],
      ],
      [
        'ja',
        [
          '地域',
          'コンテンツ種別',
          'クリップの長さ',
          '再生数',
          'トラフィック種別',
          '公開済み',
          '並べ替え',
        ],
      ],
      [
        'ru',
        [
          'Регион',
          'Тип контента',
          'Длина клипа',
          'Просмотры',
          'Тип трафика',
          'Опубликовано',
          'Сортировать',
        ],
      ],
      [
        'vi',
        [
          'Khu vực',
          'Loại nội dung',
          'Độ dài clip',
          'Lượt xem',
          'Loại lưu lượng',
          'Đã xuất bản',
          'Sắp xếp',
        ],
      ],
    ]

    for (const [code, labels] of samples) {
      await i18n.changeLanguage(code)
      const { unmount } = renderAdLibrary()

      for (const label of labels) {
        expect(
          await screen.findByRole('button', { name: label }),
          `${code} filter label "${label}"`
        ).toBeInTheDocument()
      }

      unmount()
    }
  }, 30_000)

  it('never prints a raw key or a missing-translation placeholder', async () => {
    for (const code of LOCALES) {
      await i18n.changeLanguage(code)
      const { unmount } = renderAdLibrary()
      await screen.findByRole('button', { name: i18n.t('Region') })
      const text = document.body.textContent ?? ''

      expect(text, `${code} leaked a raw i18n key`).not.toMatch(
        /\binspiration\.[a-z]+\.[a-zA-Z]/
      )
      expect(text, `${code} showed a missing placeholder`).not.toMatch(
        /\[missing|\[translation missing|i18n key/i
      )

      unmount()
    }
  }, 30_000)
})
