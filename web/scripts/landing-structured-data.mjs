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
 * Homepage JSON-LD graph. Brand values come from the build-time brand config.
 * fork: seo-audit-batch2 — the FAQPage is generated from `faqItems`, the same
 * `enFaq.items` (desktop/i18n/locales/en-faq.ts, referenced by `en.ts` as
 * `faq`) that `<CnFaq />` renders on the
 * homepage, so structured data cannot drift from the visible FAQ. The former
 * FAQPage came from the character landing (`influencer_landing.faq.*`) and did
 * not match the homepage FAQ. Without items the graph carries no FAQPage.
 */
export function landingStructuredData({
  brandName,
  seoTitle,
  brandDescription,
  siteUrl,
  faqItems = [],
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
        operatingSystem: 'macOS, Windows, Web',
        publisher: ref('organization'),
        description: brandDescription,
      },
      ...(faqItems.length > 0
        ? [
            {
              '@type': 'FAQPage',
              ...ref('faq'),
              url: `${siteUrl}/`,
              isPartOf: ref('website'),
              inLanguage: 'en',
              mainEntity: faqItems.map(({ q, a }) => ({
                '@type': 'Question',
                name: q,
                acceptedAnswer: { '@type': 'Answer', text: a },
              })),
            },
          ]
        : []),
    ],
  }
}

export { serializeStructuredData } from '../src/features/site-metadata/structured-data.mjs'
