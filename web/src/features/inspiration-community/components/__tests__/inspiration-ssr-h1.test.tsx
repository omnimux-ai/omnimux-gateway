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
 * The /inspiration crawler summary (noscript, built by `createSearchPageManifest`
 * and injected by the Go search-page handler) must print the same <h1> the
 * hydrated gallery renders. Both read one i18n key; this suite renders the real
 * gallery and the real manifest per language and compares the two headings.
 */
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

import { TooltipProvider } from '@/components/ui/tooltip'
import enBundle from '@/i18n/locales/en.json'
import zhBundle from '@/i18n/locales/zh.json'

import { landingStructuredData } from '../../../../../scripts/landing-structured-data.mjs'
import { PRODUCT_DESCRIPTION } from '../../../site-metadata/content'
import { createSearchPageManifest } from '../../../site-metadata/metadata'
import { PromptGallery } from '../prompt-gallery'

vi.mock('@/components/layout', () => ({
  PublicLayout: (props: { children: React.ReactNode }) => (
    <div>{props.children}</div>
  ),
}))

const brand = {
  brandName: 'OmniMux',
  seoTitle: 'OmniMux | AI Characters and Reference Videos',
  brandDescription: PRODUCT_DESCRIPTION,
  siteUrl: 'https://omnimux.ai',
  ogImageUrl: 'https://omnimux.ai/og.png',
}

const BUNDLES = {
  en: enBundle.translation,
  zh: zhBundle.translation,
} as const

let i18n: I18nInstance

beforeAll(async () => {
  i18n = createInstance()
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: 'en',
    supportedLngs: ['en', 'zh'],
    load: 'currentOnly',
    nsSeparator: false,
    resources: {
      en: { translation: BUNDLES.en },
      zh: { translation: BUNDLES.zh },
    },
    interpolation: { escapeValue: false },
  })
})

afterEach(async () => {
  vi.unstubAllGlobals()
  await i18n.changeLanguage('en')
})

/** The <h1>/<h2> texts of the /inspiration crawler summary for one bundle. */
function prerenderedHeadings(translations: Record<string, unknown>) {
  const manifest = createSearchPageManifest(
    brand,
    landingStructuredData(brand),
    translations,
    {
      privacy: '<p>Privacy document.</p>',
      terms: '<p>Terms document.</p>',
      refunds: '<p>Refund document.</p>',
      acceptableUse: '<p>Acceptable use document.</p>',
    }
  )
  const html = manifest.pages['/inspiration'].summary.replaceAll(
    /^<noscript>|<\/noscript>$/g,
    ''
  )
  const template = document.createElement('template')
  template.innerHTML = html
  return {
    h1: [...template.content.querySelectorAll('h1')].map((h) => h.textContent),
    h2: [...template.content.querySelectorAll('h2')].map((h) => h.textContent),
  }
}

async function renderedH1(lang: keyof typeof BUNDLES): Promise<string> {
  await i18n.changeLanguage(lang)
  // No catalog is needed for the static heading; every request answers 404.
  vi.stubGlobal(
    'fetch',
    vi.fn(
      async () =>
        ({ ok: false, status: 404, json: async () => ({}) }) as Response
    )
  )
  const root = createRootRoute()
  const inspiration = createRoute({
    getParentRoute: () => root,
    path: '/inspiration',
    component: () => <PromptGallery />,
  })
  const router = createRouter({
    routeTree: root.addChildren([inspiration]),
    history: createMemoryHistory({ initialEntries: ['/inspiration'] }),
  })
  render(
    <I18nextProvider i18n={i18n}>
      <TooltipProvider>
        <RouterProvider router={router} />
      </TooltipProvider>
    </I18nextProvider>
  )
  const heading = await screen.findByRole('heading', { level: 1 })
  return heading.textContent ?? ''
}

describe('/inspiration prerendered H1', () => {
  it('matches the rendered gallery H1 in English', async () => {
    const rendered = await renderedH1('en')
    expect(rendered).toBe('AI Video & Image Prompt Library')
    expect(prerenderedHeadings(BUNDLES.en).h1).toEqual([rendered])
  })

  it('matches the rendered gallery H1 in Simplified Chinese', async () => {
    const rendered = await renderedH1('zh')
    expect(rendered).toBe('AI 视频与图片提示词灵感库')
    expect(prerenderedHeadings(BUNDLES.zh).h1).toEqual([rendered])
  })

  it('does not reuse the gallery H2 as the prerendered H1', () => {
    for (const bundle of Object.values(BUNDLES)) {
      const { h1, h2 } = prerenderedHeadings(bundle)
      expect(h1).not.toContain(bundle['More prompt libraries'])
      expect(h2).not.toContain(h1[0])
    }
  })
})
