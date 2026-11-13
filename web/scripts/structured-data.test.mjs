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
import { readFileSync } from 'node:fs'
import path from 'node:path'

import { describe, expect, it } from 'vitest'

import { en as landingCopy } from '../src/features/omnimux-landing/desktop/i18n/locales/en'
import { enFaq } from '../src/features/omnimux-landing/desktop/i18n/locales/en-faq'
import {
  landingStructuredData,
  serializeStructuredData,
} from './landing-structured-data.mjs'

const brand = {
  brandName: 'OmniMux',
  seoTitle: 'OmniMux | AI Characters and Reference Videos',
  brandDescription:
    'Create AI characters, build multi-view references, and generate character videos from reference clips.',
  siteUrl: 'https://omnimux.ai',
}

describe('landing structured data', () => {
  it('serializes one graph whose entity references all resolve', () => {
    const graph = JSON.parse(
      serializeStructuredData(
        landingStructuredData({ ...brand, faqItems: landingCopy.faq.items })
      )
    )['@graph']
    expect(graph.map((node) => node['@type'])).toEqual([
      'Organization',
      'WebSite',
      'WebPage',
      'SoftwareApplication',
      'FAQPage',
    ])

    const ids = new Set(graph.map((node) => node['@id']))
    for (const node of graph) {
      for (const key of ['publisher', 'isPartOf', 'about', 'mainEntity']) {
        if (node[key]?.['@id']) expect(ids.has(node[key]['@id'])).toBe(true)
      }
      for (const key of [
        'author',
        'datePublished',
        'dateModified',
        'aggregateRating',
        'offers',
        'sameAs',
      ]) {
        expect(node[key]).toBeUndefined()
      }
    }
  })

  it('builds the FAQPage from the same items the homepage <CnFaq /> renders', () => {
    const items = landingCopy.faq.items
    const faq = landingStructuredData({ ...brand, faqItems: items })[
      '@graph'
    ].find((node) => node['@type'] === 'FAQPage')

    expect(items.length).toBeGreaterThan(0)
    // The build passes enFaq.items; the homepage renders landingCopy.faq.
    expect(enFaq.items).toBe(items)
    expect(
      faq.mainEntity.map((question) => ({
        q: question.name,
        a: question.acceptedAnswer.text,
      }))
    ).toEqual(items.map(({ q, a }) => ({ q, a })))
  })

  it('keeps subscription and free-credit claims out of the homepage FAQ', () => {
    const text = JSON.stringify(landingCopy.faq.items).replaceAll(
      /no subscriptions?/gi,
      ''
    )

    expect(text).not.toMatch(
      /subscri|free trial|free credit|start free|\bfree\b|订阅|免费/i
    )
  })

  it('publishes no FAQPage without visible FAQ items', () => {
    const graph = landingStructuredData(brand)['@graph']

    expect(graph.some((node) => node['@type'] === 'FAQPage')).toBe(false)
    expect(JSON.stringify(graph)).not.toContain('"Question"')
  })

  it('keeps quoted brand copy and script delimiters intact', () => {
    const graph = landingStructuredData({
      ...brand,
      brandName: 'Brand "quoted"',
      brandDescription: '</script><script>alert(1)</script>&',
    })
    const serialized = serializeStructuredData(graph)

    expect(serialized).not.toContain('<')
    expect(JSON.parse(serialized)).toEqual(graph)
  })

  it('leaves the HTML template one generated metadata block and no hand-maintained FAQ', () => {
    const template = readFileSync(
      path.resolve(process.cwd(), 'index.html'),
      'utf8'
    )

    expect(template).toContain('<!--omnimux-page-head-->')
    expect(template).toContain('<%= pageHead %>')
    expect(template).toContain('<%= pageSummary %>')
    expect(template).not.toContain('"@type": "Question"')
  })
})
