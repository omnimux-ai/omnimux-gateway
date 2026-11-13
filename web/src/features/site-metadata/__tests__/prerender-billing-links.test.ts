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
 * Crawler copy guard for /pricing, /inspiration and /ai-influencer-studio.
 *
 * TEMPORARY DUPLICATE: `ALLOWED_BILLING_NEGATIVES` and `FORBIDDEN_BILLING_CLAIMS`
 * are copied verbatim from PR #764's `metadata.test.ts` so this PR does not
 * conflict with it. Once #764 is merged, fold this word list back into one copy
 * and extend the assertion to every public page in a follow-up task.
 */
import { describe, expect, it } from 'vitest'

import { landingStructuredData } from '../../../../scripts/landing-structured-data.mjs'
import translations from '../../../i18n/locales/en.json'
import { PRODUCT_DESCRIPTION } from '../content'
import { createSearchPageManifest } from '../metadata'

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

const ALLOWED_BILLING_NEGATIVES = /\bno subscriptions?\b|没有订阅/gi
const FORBIDDEN_BILLING_CLAIMS = new RegExp(
  [
    'subscri',
    '订阅',
    'monthly plan',
    'per month',
    '包月',
    '\\/month\\b',
    'billed monthly',
    'monthly membership',
    '\\bstart(?:\\s+for)?\\s+free\\b',
    '\\bstart-free\\b',
    'try(?:\\s+it)?\\s+free',
    'get started free',
    'free trial',
    'free credit',
    'free plan',
    'free tokens',
    '免费试用',
    '免费额度',
    '免费积分',
    '免费体验',
    '赠送积分',
    '送积分',
    '开通会员',
    '按月付费',
    '按月收费',
    '\\/mo\\b',
    'free to start',
    'forever free',
  ].join('|'),
  'i'
)

function billingClaimRemainder(text: string): string {
  return text.replace(ALLOWED_BILLING_NEGATIVES, '')
}

/**
 * An href whose path is exactly `/influencer` (optionally followed by a query,
 * fragment or sub-path). `/ai-influencer-studio` never matches.
 */
const INFLUENCER_HREF = /href="(?:https?:\/\/[^/"]+)?\/influencer(?=["?#/])/

const SEARCH_PRICING_SENTENCE =
  'Top up credits and pay per generation. No subscription; trial packs are paid. Check current prices before you generate.'

describe('prerendered billing copy and studio links', () => {
  it('keeps /pricing and /inspiration heads and summaries free of subscription and free-credit claims', () => {
    for (const path of ['/pricing', '/inspiration']) {
      const page = manifest.pages[path]
      expect(page, path).toBeDefined()
      for (const [part, text] of [
        ['head', page.head],
        ['summary', page.summary],
      ] as const) {
        expect(billingClaimRemainder(text), `${path} ${part}`).not.toMatch(
          FORBIDDEN_BILLING_CLAIMS
        )
      }
    }
  })

  it('states the approved pricing sentence on /pricing', () => {
    const page = manifest.pages['/pricing']
    expect(page.head).toContain(
      `<meta name="description" content="${SEARCH_PRICING_SENTENCE}" />`
    )
    expect(page.summary).toContain(`<p>${SEARCH_PRICING_SENTENCE}</p>`)
  })

  it('does not link /pricing, /inspiration or /ai-influencer-studio to /influencer', () => {
    for (const path of ['/pricing', '/inspiration', '/ai-influencer-studio']) {
      const page = manifest.pages[path]
      expect(page, path).toBeDefined()
      expect(`${page.head}\n${page.summary}`, path).not.toMatch(INFLUENCER_HREF)
    }
  })

  it('does not point prerendered tool pages at /influencer', () => {
    const toolPaths = Object.keys(manifest.pages).filter((path) =>
      path.includes('/tools/')
    )
    expect(toolPaths.length).toBeGreaterThan(0)
    for (const path of toolPaths) {
      const page = manifest.pages[path]
      expect(`${page.head}\n${page.summary}`, path).not.toMatch(INFLUENCER_HREF)
    }
  })

  it('matches /influencer exactly and never /ai-influencer-studio', () => {
    for (const hit of [
      'href="https://omnimux.ai/influencer"',
      'href="/influencer"',
      'href="https://omnimux.ai/influencer?tab=video"',
      'href="https://omnimux.ai/influencer#start"',
      'href="https://omnimux.ai/influencer/new"',
    ]) {
      expect(hit).toMatch(INFLUENCER_HREF)
    }
    for (const miss of [
      'href="https://omnimux.ai/ai-influencer-studio"',
      'href="/ai-influencer-studio"',
      'href="https://omnimux.ai/influencers"',
      'href="https://omnimux.ai/guides/influencer-tips"',
    ]) {
      expect(miss).not.toMatch(INFLUENCER_HREF)
    }
  })
})
