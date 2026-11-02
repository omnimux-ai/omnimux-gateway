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
      serializeStructuredData(landingStructuredData(brand))
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

  it('mirrors the five FAQ questions and answers the landing page renders', () => {
    const translations = JSON.parse(
      readFileSync(
        path.resolve(process.cwd(), 'src/i18n/locales/en.json'),
        'utf8'
      )
    ).translation
    const faq = landingStructuredData(brand)['@graph'].find(
      (node) => node['@type'] === 'FAQPage'
    )

    expect(faq.mainEntity).toHaveLength(5)
    faq.mainEntity.forEach((question, index) => {
      const key = `q${index + 1}`
      expect(question.name).toBe(translations[`influencer_landing.faq.${key}`])
      expect(question.acceptedAnswer.text).toBe(
        translations[`influencer_landing.faq.a${index + 1}`]
      )
      expect(question.name).toBeTruthy()
      expect(question.acceptedAnswer.text).toBeTruthy()
    })
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
