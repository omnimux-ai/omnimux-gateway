import {
  TOOL_PAGES,
  type ToolKind,
  type ToolLocale,
  type ToolPageContent,
} from '../omnimux-landing/desktop/tools/content'
import type { SearchPage, SiteBrand } from './metadata'

/** Renderers owned by metadata.ts; passed in to avoid an import cycle. */
export interface ToolPageRenderers {
  escapeHtml: (value: string) => string
  paragraph: (text: string) => string
  summary: (title: string, body: string, brand: SiteBrand) => string
}

const LOCALE_TAGS: Record<ToolLocale, { hreflang: string; og: string }> = {
  en: { hreflang: 'en', og: 'en_US' },
  'zh-CN': { hreflang: 'zh-Hans', og: 'zh_CN' },
}

/**
 * fork: the TikTok tool pages publish one URL per language. Each page names
 * every language alternate plus x-default (English), and carries the same
 * steps and FAQ the page renders so the crawler-visible text cannot drift.
 */
export function toolSearchPages(
  brand: SiteBrand,
  render: ToolPageRenderers
): (SearchPage & { path: string })[] {
  const pages: (SearchPage & { path: string })[] = []
  for (const kind of ['video', 'photo'] as ToolKind[]) {
    const alternates = [
      ...(Object.keys(TOOL_PAGES) as ToolLocale[]).map((locale) => ({
        hreflang: LOCALE_TAGS[locale].hreflang,
        path: TOOL_PAGES[locale][kind].path,
      })),
      { hreflang: 'x-default', path: TOOL_PAGES.en[kind].path },
    ]
    for (const locale of Object.keys(TOOL_PAGES) as ToolLocale[]) {
      const page = TOOL_PAGES[locale][kind]
      pages.push({
        path: page.path,
        title: page.title,
        description: page.description,
        alternates,
        ogLocale: LOCALE_TAGS[locale].og,
        graph: toolGraph(brand, page, LOCALE_TAGS[locale].hreflang),
        summary: render.summary(page.h1, toolBody(brand, page, render), brand),
      })
    }
  }
  return pages
}

function toolBody(
  brand: SiteBrand,
  page: ToolPageContent,
  render: ToolPageRenderers
): string {
  const { escapeHtml, paragraph } = render
  const steps = page.steps
    .map(
      (step) =>
        `<li><strong>${escapeHtml(step.title)}</strong> ${escapeHtml(step.body)}</li>`
    )
    .join('')
  const faq = page.faq
    .map((item) => `<h3>${escapeHtml(item.q)}</h3>${paragraph(item.a)}`)
    .join('')
  const related = page.related
    .map(
      (link) =>
        `<a href="${escapeHtml(`${brand.siteUrl}${link.href}`)}">${escapeHtml(link.label)}</a>`
    )
    .join(' · ')
  return `${paragraph(page.subtitle)}<ol>${steps}</ol><h2>FAQ</h2>${faq}<p>${related}</p>`
}

function toolGraph(
  brand: SiteBrand,
  page: ToolPageContent,
  inLanguage: string
): Record<string, unknown> {
  const url = `${brand.siteUrl}${page.path}`
  return {
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'WebPage',
        '@id': `${url}#webpage`,
        url,
        name: page.title,
        description: page.description,
        inLanguage,
        isPartOf: {
          '@type': 'WebSite',
          name: brand.brandName,
          url: `${brand.siteUrl}/`,
        },
        breadcrumb: { '@id': `${url}#breadcrumb` },
      },
      {
        '@type': 'BreadcrumbList',
        '@id': `${url}#breadcrumb`,
        itemListElement: [
          {
            '@type': 'ListItem',
            position: 1,
            name: brand.brandName,
            item: `${brand.siteUrl}/`,
          },
          { '@type': 'ListItem', position: 2, name: page.h1, item: url },
        ],
      },
      {
        '@type': 'WebApplication',
        name: page.h1,
        url,
        applicationCategory: 'MultimediaApplication',
        operatingSystem: 'Web',
        browserRequirements: 'Requires JavaScript',
        inLanguage,
        isAccessibleForFree: true,
        offers: { '@type': 'Offer', price: '0', priceCurrency: 'USD' },
        publisher: {
          '@type': 'Organization',
          name: brand.brandName,
          url: `${brand.siteUrl}/`,
        },
      },
      {
        '@type': 'FAQPage',
        mainEntity: page.faq.map((item) => ({
          '@type': 'Question',
          name: item.q,
          acceptedAnswer: { '@type': 'Answer', text: item.a },
        })),
      },
    ],
  }
}
