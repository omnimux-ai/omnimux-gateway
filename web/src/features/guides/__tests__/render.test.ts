import { existsSync } from 'node:fs'
import { resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

import { GUIDE_REGISTRY } from '../registry'
import { renderGuideBody, renderGuideDocuments } from '../render'

const brand = {
  brandName: 'OmniMux',
  seoTitle: 'OmniMux | AI Characters and Reference Videos',
  brandDescription: 'Create AI characters and recreate reference videos.',
  siteUrl: 'https://omnimux.ai',
  ogImageUrl: 'https://omnimux.ai/og.png',
}
const options = {
  brand,
  cssUrls: ['/static/css/guides.test.css'],
  scriptUrls: ['/static/js/guides.test.js'],
}
const documents = renderGuideDocuments(options)
const parse = (html: string) =>
  new DOMParser().parseFromString(html, 'text/html')

describe('standalone guide documents', () => {
  it('publishes the hub and both full English articles outside noscript', () => {
    expect(Object.keys(documents)).toEqual([
      '/guides',
      '/guides/create-ai-character',
      '/guides/recreate-video-with-your-character',
    ])
    for (const article of GUIDE_REGISTRY) {
      const document = parse(documents[article.path].documentHtml)
      expect(document.documentElement.lang).toBe('en')
      expect(document.querySelectorAll('h1')).toHaveLength(1)
      expect(document.querySelector('.article-body')?.textContent).toContain(
        'Questions & limits'
      )
      expect(
        document.querySelector('.article-body')?.textContent?.split(/\s+/)
          .length
      ).toBeGreaterThan(500)
      expect(document.querySelector('noscript')).toBeNull()
      expect(document.querySelector('main [hidden]')).toBeNull()
    }
  })

  it('provides unique canonical metadata and exact compiled resources without application hydration', () => {
    const titles = new Set<string>()
    for (const [path, item] of Object.entries(documents)) {
      const document = parse(item.documentHtml)
      titles.add(document.title)
      expect(document.querySelectorAll('title')).toHaveLength(1)
      expect(
        document.querySelector('link[rel=canonical]')?.getAttribute('href')
      ).toBe(`${brand.siteUrl}${path}`)
      expect(
        document.querySelector('meta[name=robots]')?.getAttribute('content')
      ).toBe('index,follow')
      expect(
        [...document.querySelectorAll('link[rel=stylesheet]')].map((link) =>
          link.getAttribute('href')
        )
      ).toEqual(options.cssUrls)
      expect(
        [...document.querySelectorAll('script[src]')].map((script) =>
          script.getAttribute('src')
        )
      ).toEqual(options.scriptUrls)
      expect(item.documentHtml).not.toMatch(
        /react|hydrate|id="root"|data-nav=/i
      )
      expect(item.documentHtml.match(/<!--umami-->\n/g)).toHaveLength(1)
    }
    expect(titles.size).toBe(3)
  })

  it('derives directory summaries and article links from the registry', () => {
    const hub = parse(documents['/guides'].documentHtml)
    for (const article of GUIDE_REGISTRY) {
      const card = hub.querySelector(`.guide-card[href="${article.path}"]`)
      expect(card?.textContent).toContain(article.title)
      expect(card?.textContent).toContain(article.summary)
    }
  })

  it('provides native section, related-article, and image links when JavaScript is absent', () => {
    for (const article of GUIDE_REGISTRY) {
      const document = parse(documents[article.path].documentHtml)
      const links = [...document.querySelectorAll('.article-toc nav a')]
      expect(links.length).toBeGreaterThan(5)
      for (const link of links) {
        expect(
          document.querySelector(link.getAttribute('href') ?? '#missing')
        ).not.toBeNull()
      }
      const sibling = GUIDE_REGISTRY.find(
        (item) => item.slug === article.next.relatedSlug
      )
      expect(
        document.querySelector('.related-link')?.getAttribute('href')
      ).toBe(sibling?.path)
      expect(document.querySelectorAll('a[data-guide-image]')).toHaveLength(
        article.media.length
      )
      for (const media of article.media) {
        const link = document.querySelector(
          `a[data-guide-image][href="${media.url}"]`
        )
        expect(link?.querySelector('img')?.alt).toBe(media.alt)
        expect(link?.querySelector('img')?.getAttribute('width')).toBe(
          String(media.width)
        )
        expect(link?.querySelector('img')?.getAttribute('height')).toBe(
          String(media.height)
        )
        expect(
          existsSync(resolve('src/features/guides/assets', media.fileName))
        ).toBe(true)
      }
    }
  })

  it('records only guide slug and button position on working studio links', () => {
    for (const article of GUIDE_REGISTRY) {
      const document = parse(documents[article.path].documentHtml)
      const links = [...document.querySelectorAll('[data-umami-event]')]
      expect(links.length).toBeGreaterThan(0)
      for (const link of links) {
        expect(link.getAttribute('href')).toBe('/influencer')
        expect(link.getAttribute('data-umami-event')).toBe('guide_studio_click')
        expect(link.getAttribute('data-umami-event-slug')).toBe(article.slug)
        expect(link.getAttribute('data-umami-event-position')).toMatch(
          /^(header|sidebar|article-bottom)$/
        )
        expect(
          link
            .getAttributeNames()
            .filter((name) => name.startsWith('data-umami-event-'))
            .sort()
        ).toEqual(['data-umami-event-position', 'data-umami-event-slug'])
      }
    }
  })

  it('uses the real organization and visible breadcrumb labels in structured data', () => {
    for (const article of GUIDE_REGISTRY) {
      const document = parse(documents[article.path].documentHtml)
      const graph = JSON.parse(
        document.querySelector('script[type="application/ld+json"]')
          ?.textContent ?? '{}'
      )['@graph']
      const organization = graph.find(
        (node: { '@type': string }) => node['@type'] === 'Organization'
      )
      const articleNode = graph.find(
        (node: { '@type': string }) => node['@type'] === 'Article'
      )
      const breadcrumb = graph.find(
        (node: { '@type': string }) => node['@type'] === 'BreadcrumbList'
      )
      expect(organization.name).toBe('OmniMux')
      expect(organization.url).toBe('https://omnimux.ai/')
      expect(articleNode.author).toEqual({ '@id': organization['@id'] })
      expect(articleNode.datePublished).toBe(article.publishedAt)
      expect(articleNode.dateModified).toBe(article.reviewedAt)
      expect(
        breadcrumb.itemListElement.map((entry: { name: string }) => entry.name)
      ).toEqual(
        [
          ...document.querySelectorAll(
            '.breadcrumb a,.breadcrumb [aria-current=page]'
          ),
        ].map((element) => element.textContent)
      )
    }
  })

  it('allows head-only projection before CSS compilation without putting article text in metadata', () => {
    const projected = renderGuideDocuments({ brand, cssUrls: [] })
    for (const item of Object.values(projected)) {
      expect(item.head).toContain('<title>')
      expect(item.head).not.toContain('Before you begin')
      expect(item.head).not.toContain('bodyMarkdown')
      expect(
        parse(item.documentHtml).querySelector('link[rel=stylesheet]')
      ).toBeNull()
    }
  })

  it('fails building an article with missing media or duplicate section anchors', () => {
    const article = GUIDE_REGISTRY[0]
    expect(() => renderGuideBody({ ...article, media: [] })).toThrow(
      'Unknown guide media'
    )
    expect(() =>
      renderGuideBody({
        ...article,
        bodyMarkdown: `${article.bodyMarkdown}\n## Duplicate {#c-prepare}\n`,
      })
    ).toThrow('Duplicate guide anchor')
  })

  it('keeps configured branding inside HTML and JSON boundaries', () => {
    const name = '</script><script>alert("brand")</script>'
    const escaped = renderGuideDocuments({
      ...options,
      brand: { ...brand, brandName: name },
    })
    const document = parse(escaped[GUIDE_REGISTRY[0].path].documentHtml)
    expect(document.querySelectorAll('script')).toHaveLength(2)
    const graph = JSON.parse(
      document.querySelector('script[type="application/ld+json"]')
        ?.textContent ?? '{}'
    )['@graph']
    expect(
      graph.find(
        (node: { '@type': string }) => node['@type'] === 'Organization'
      ).name
    ).toBe(name)
  })
})
