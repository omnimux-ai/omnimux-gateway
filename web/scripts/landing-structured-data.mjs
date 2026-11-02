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

// `web/` is the package root both the build and Vitest run in.
const translations = JSON.parse(
  readFileSync(path.resolve(process.cwd(), 'src/i18n/locales/en.json'), 'utf8')
).translation

/** FAQ entries the landing page renders, keyed q1..q5 / a1..a5. */
const FAQ_KEYS = ['q1', 'q2', 'q3', 'q4', 'q5']

/**
 * Homepage JSON-LD graph. Brand values come from the build-time brand config;
 * FAQ copy is read from the landing translations so the crawler-visible
 * questions cannot drift from the visible page.
 */
export function landingStructuredData({
  brandName,
  seoTitle,
  brandDescription,
  siteUrl,
}) {
  const ref = (fragment) => ({ '@id': `${siteUrl}/#${fragment}` })
  return {
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'Organization',
        ...ref('organization'),
        name: brandName,
        url: `${siteUrl}/`,
        logo: `${siteUrl}/logo.png`,
        description: brandDescription,
      },
      {
        '@type': 'WebSite',
        ...ref('website'),
        url: `${siteUrl}/`,
        name: brandName,
        description: brandDescription,
        inLanguage: 'en',
        publisher: ref('organization'),
      },
      {
        '@type': 'WebPage',
        ...ref('webpage'),
        url: `${siteUrl}/`,
        name: seoTitle,
        isPartOf: ref('website'),
        about: ref('organization'),
        mainEntity: ref('software'),
        description: brandDescription,
        inLanguage: 'en',
      },
      {
        '@type': 'SoftwareApplication',
        ...ref('software'),
        name: brandName,
        url: `${siteUrl}/`,
        applicationCategory: 'MultimediaApplication',
        operatingSystem: 'Web',
        publisher: ref('organization'),
        description: brandDescription,
      },
      {
        '@type': 'FAQPage',
        ...ref('faq'),
        isPartOf: ref('webpage'),
        inLanguage: 'en',
        mainEntity: FAQ_KEYS.map((key) => ({
          '@type': 'Question',
          name: translations[`influencer_landing.faq.${key}`],
          acceptedAnswer: {
            '@type': 'Answer',
            text: translations[`influencer_landing.faq.${key.replace('q', 'a')}`],
          },
        })),
      },
    ],
  }
}

export { serializeStructuredData } from '../src/features/site-metadata/structured-data.mjs'
