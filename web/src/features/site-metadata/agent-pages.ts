import {
  AGENT_LOCALES,
  AGENT_LOCALE_TAGS,
  AGENT_PATHS,
  type AgentKind,
  type AgentLocale,
  type AgentPageContent,
} from '../omnimux-landing/desktop/agent/content'
import { AGENT_PAGES } from '../omnimux-landing/desktop/agent/locales'
import type { SearchPage, SearchPageRenderers, SiteBrand } from './metadata'

/**
 * fork: the Agent entry pages (/cli and /skills) publish one URL per language.
 * Each page names all six language alternates plus x-default (English) and
 * carries steps, core cards, and FAQ so crawler-visible HTML cannot drift.
 */
export function agentSearchPages(
  brand: SiteBrand,
  render: SearchPageRenderers
): (SearchPage & { path: string })[] {
  const pages: (SearchPage & { path: string })[] = []
  for (const kind of ['cli', 'skills'] as AgentKind[]) {
    const alternates = [
      ...AGENT_LOCALES.map((locale) => ({
        hreflang: AGENT_LOCALE_TAGS[locale].hreflang,
        path: AGENT_PATHS[locale][kind],
      })),
      { hreflang: 'x-default', path: AGENT_PATHS.en[kind] },
    ]
    for (const locale of AGENT_LOCALES) {
      const page = AGENT_PAGES[locale][kind]
      pages.push({
        path: page.path,
        title: page.title,
        description: page.description,
        alternates,
        ogLocale: AGENT_LOCALE_TAGS[locale].og,
        graph: agentGraph(
          brand,
          page,
          kind,
          AGENT_LOCALE_TAGS[locale].hreflang
        ),
        summary: render.summary(
          page.h1,
          agentBody(brand, page, kind, locale, render),
          brand
        ),
      })
    }
  }
  return pages
}

function agentBody(
  brand: SiteBrand,
  page: AgentPageContent,
  kind: AgentKind,
  locale: AgentLocale,
  render: SearchPageRenderers
): string {
  const { escapeHtml, paragraph } = render
  const steps = page.steps
    .map(
      (step) =>
        `<li><strong>${escapeHtml(step.title)}</strong>${step.command ? ` <code>${escapeHtml(step.command)}</code>` : ''} ${escapeHtml(step.body)}</li>`
    )
    .join('')
  const cards = page.cards
    .map(
      (card) =>
        `<li><strong>${escapeHtml(card.title)}</strong> ${escapeHtml(card.body)}</li>`
    )
    .join('')
  const faq = page.faq
    .map((item) => `<h3>${escapeHtml(item.q)}</h3>${paragraph(item.a)}`)
    .join('')
  const siblingPath =
    kind === 'cli' ? AGENT_PATHS[locale].skills : AGENT_PATHS[locale].cli
  const siblingLabel = kind === 'cli' ? page.kindSkills : page.kindCli
  const related = [
    { label: siblingLabel, href: siblingPath },
    { label: 'AI Influencer Studio', href: '/ai-influencer-studio' },
  ]
    .map(
      (link) =>
        `<a href="${escapeHtml(`${brand.siteUrl}${link.href}`)}">${escapeHtml(link.label)}</a>`
    )
    .join(' | ')
  return `${paragraph(page.subtitle)}<h2>${escapeHtml(page.stepLabel)}</h2><ol>${steps}</ol><h2>${escapeHtml(page.coreTitle)}</h2><ul>${cards}</ul><h2>${escapeHtml(page.faqTitle)}</h2>${faq}<p>${related}</p>`
}

function agentGraph(
  brand: SiteBrand,
  page: AgentPageContent,
  kind: AgentKind,
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
        '@type': 'SoftwareApplication',
        name:
          kind === 'cli'
            ? `${brand.brandName} CLI`
            : `${brand.brandName} Skills`,
        url,
        applicationCategory: 'DeveloperApplication',
        operatingSystem: 'macOS, Linux',
        inLanguage,
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
