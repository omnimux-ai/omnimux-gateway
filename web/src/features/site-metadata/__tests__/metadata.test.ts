import { describe, expect, it } from 'vitest'

import { landingStructuredData } from '../../../../scripts/landing-structured-data.mjs'
import translations from '../../../i18n/locales/en.json'
import { renderGuideDocuments } from '../../guides/render'
import { PRODUCT_DESCRIPTION } from '../content'
import { applyPageMetadata } from '../dom'
import {
  browserSearchPageManifest,
  createSearchPageManifest,
  renderSearchSitemap,
  resolveSearchPage,
} from '../metadata'

const brand = {
  brandName: 'OmniMux',
  seoTitle: 'OmniMux | AI Characters and Reference Videos',
  brandDescription: PRODUCT_DESCRIPTION,
  siteUrl: 'https://omnimux.ai',
  ogImageUrl: 'https://omnimux.ai/og.png',
}

const manifest = createSearchPageManifest(
  brand,
  landingStructuredData(brand),
  translations.translation,
  {
    privacy: '<p>Privacy document.</p>',
    terms: '<p>Terms document.</p>',
    refunds: '<p>Refund document.</p>',
    acceptableUse: '<p>Acceptable use document.</p>',
  }
)

describe('public page metadata', () => {
  it('lists exactly the published public pages in the sitemap', () => {
    const guideDocuments = renderGuideDocuments({
      brand,
      cssUrls: ['/static/css/guides.css'],
    })
    const fullManifest = {
      ...manifest,
      pages: {
        ...manifest.pages,
        ...Object.fromEntries(
          Object.entries(guideDocuments).map(([path, page]) => [
            path,
            { ...page, summary: '' },
          ])
        ),
      },
    }
    const sitemap = renderSearchSitemap(fullManifest, brand.siteUrl)
    const urls = [...sitemap.matchAll(/<loc>([^<]+)<\/loc>/g)].map(
      (match) => match[1]
    )
    expect(urls).toEqual(
      Object.keys(fullManifest.pages).map((path) => `${brand.siteUrl}${path}`)
    )
    expect(urls).toHaveLength(16)
    expect(sitemap).not.toContain('/influencer')
  })

  it('gives each public page its own canonical, title and matching fallback content', () => {
    expect(Object.keys(manifest.pages)).toEqual([
      '/',
      '/about',
      '/ai-influencer-studio',
      '/privacy',
      '/terms',
      '/refunds',
      '/acceptable-use',
      '/pricing',
      '/inspiration',
      '/tools/tiktok-video-downloader',
      '/zh/tools/tiktok-video-downloader',
      '/tools/tiktok-photo-downloader',
      '/zh/tools/tiktok-photo-downloader',
    ])
    const titles = new Set<string>()
    for (const [path, page] of Object.entries(manifest.pages)) {
      const head = new DOMParser().parseFromString(page.head, 'text/html')
      titles.add(head.title)
      expect(
        head.querySelector('link[rel="canonical"]')?.getAttribute('href')
      ).toBe(`https://omnimux.ai${path}`)
      expect(
        head.querySelector('meta[property="og:url"]')?.getAttribute('content')
      ).toBe(`https://omnimux.ai${path}`)
      expect(
        head.querySelector('meta[name="robots"]')?.getAttribute('content')
      ).toBe('index,follow')
      expect(page.summary).toContain('<noscript>')
      expect(page.summary).not.toMatch(/\bhidden\b|display:\s*none/)
      expect(page.summary).not.toContain('undefined')
    }
    expect(titles.size).toBe(13)
    expect(manifest.pages['/about'].summary).toContain(PRODUCT_DESCRIPTION)
    expect(manifest.pages['/about'].summary).not.toContain(
      'Put your character in any trend'
    )
    expect(manifest.pages['/privacy'].summary).toContain('Privacy document.')
  })

  it('links every TikTok tool page to its language alternates and publishes its FAQ', () => {
    const paths = {
      video: ['/tools/tiktok-video-downloader', '/zh/tools/tiktok-video-downloader'],
      photo: ['/tools/tiktok-photo-downloader', '/zh/tools/tiktok-photo-downloader'],
    }
    for (const [en, zh] of Object.values(paths)) {
      for (const path of [en, zh]) {
        const page = manifest.pages[path]
        const head = new DOMParser().parseFromString(page.head, 'text/html')
        const alternates = Object.fromEntries(
          [...head.querySelectorAll('link[rel="alternate"]')].map((link) => [
            link.getAttribute('hreflang'),
            link.getAttribute('href'),
          ])
        )
        expect(alternates).toEqual({
          en: `https://omnimux.ai${en}`,
          'zh-Hans': `https://omnimux.ai${zh}`,
          'x-default': `https://omnimux.ai${en}`,
        })
        const graph = JSON.parse(
          head.querySelector('script[type="application/ld+json"]')?.textContent ?? '{}'
        ) as { '@graph': { '@type': string; mainEntity?: unknown[] }[] }
        const types = graph['@graph'].map((node) => node['@type'])
        expect(types).toEqual(['WebPage', 'BreadcrumbList', 'WebApplication', 'FAQPage'])
        const faq = graph['@graph'][3].mainEntity ?? []
        expect(faq.length).toBeGreaterThan(0)
        expect(page.summary).toContain('<noscript>')
        expect(page.head.toLowerCase()).not.toMatch(/watermark|无水印/)
      }
    }
  })

  it('projects only heads into the browser metadata, including static tutorials', () => {
    const full = {
      ...manifest,
      pages: {
        ...manifest.pages,
        '/guides': {
          head: '<title>Guides</title>',
          summary: 'summary sentinel',
          documentHtml: '<main>article body sentinel</main>',
        },
      },
    }
    const browser = browserSearchPageManifest(full)
    const serialized = JSON.stringify(browser)
    expect(browser.pages['/guides'].head).toBe(full.pages['/guides'].head)
    expect(serialized).not.toContain('sentinel')
    expect(serialized).not.toContain('documentHtml')
    expect(
      Object.values(browser.pages).every((page) => page.summary === '')
    ).toBe(true)
  })

  it('normalizes public filters and trailing slashes without making workspaces indexable', () => {
    expect(resolveSearchPage(manifest, '/about/?group=test#prices')).toBe(
      manifest.pages['/about']
    )
    for (const path of [
      '/influencer',
      '/sign-in',
      '/wallet',
      '/privacy-policy',
      '/user-agreement',
      '/unknown',
    ]) {
      const page = resolveSearchPage(manifest, path)
      expect(page).toBe(manifest.private)
      expect(page.head).toContain('noindex,follow')
      expect(page.head).not.toContain('rel="canonical"')
      expect(page.summary).toBe('')
    }
  })

  it('escapes quoted brand values and script delimiters in metadata', () => {
    const hostile = createSearchPageManifest(
      { ...brand, seoTitle: 'Name "quoted" </title><script>alert(1)</script>' },
      {
        '@context': 'https://schema.org',
        name: '</script><script>alert(1)</script>',
      },
      translations.translation,
      { privacy: '', terms: '', refunds: '', acceptableUse: '' }
    )
    const parsed = new DOMParser().parseFromString(
      hostile.pages['/'].head,
      'text/html'
    )
    expect(parsed.title).toContain('Name "quoted" </title><script>')
    expect(parsed.querySelectorAll('script')).toHaveLength(1)
    expect(
      JSON.parse(parsed.querySelector('script')?.textContent ?? '')
    ).toEqual({
      '@context': 'https://schema.org',
      name: '</script><script>alert(1)</script>',
    })
  })

  it('replaces page metadata through public and private navigation without removing analytics', () => {
    document.head.innerHTML =
      '<title>Old</title><script id="analytics"></script><meta name="viewport" content="width=device-width">'
    applyPageMetadata(manifest, '/')
    applyPageMetadata(manifest, '/about')
    expect(document.title).toContain('About')
    expect(document.querySelectorAll('link[rel="canonical"]')).toHaveLength(1)
    expect(
      document.querySelector('link[rel="canonical"]')?.getAttribute('href')
    ).toBe('https://omnimux.ai/about')
    applyPageMetadata(manifest, '/wallet')
    expect(document.querySelector('link[rel="canonical"]')).toBeNull()
    expect(document.querySelector('meta[property="og:url"]')).toBeNull()
    expect(
      document.querySelector('meta[name="robots"]')?.getAttribute('content')
    ).toBe('noindex,follow')
    applyPageMetadata(manifest, '/about')
    expect(document.title).toContain('About OmniMux')
    expect(
      document.querySelectorAll('script[type="application/ld+json"]')
    ).toHaveLength(1)
    expect(document.querySelector('#analytics')).not.toBeNull()
    expect(document.querySelector('meta[name="viewport"]')).not.toBeNull()
  })
})
