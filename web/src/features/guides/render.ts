import { Marked, Renderer } from 'marked'

import { escapeHtml, type SiteBrand } from '../site-metadata/metadata'
import { serializeStructuredData } from '../site-metadata/structured-data.mjs'
import {
  GUIDE_HUB,
  GUIDE_REGISTRY,
  type GuideArticle,
  type GuideMedia,
} from './registry'

export interface GuideDocument {
  head: string
  documentHtml: string
}

export interface GuideDocumentOptions {
  brand: SiteBrand
  cssUrls: readonly string[]
  scriptUrls?: readonly string[]
  publishedAt?: string
}

interface TocEntry {
  id: string
  label: string
}

function mediaById(article: GuideArticle, id: string): GuideMedia {
  const media = article.media.find((item) => item.id === id)
  if (!media) throw new Error(`Unknown guide media: ${article.slug}/${id}`)
  return media
}

function renderMedia(media: GuideMedia): string {
  const className = media.shape ? ` class="figure-${media.shape}"` : ''
  return `<figure${className}><a class="image-zoom" href="${escapeHtml(media.url)}" data-guide-image aria-label="Enlarge image: ${escapeHtml(media.alt)}"><img src="${escapeHtml(media.url)}" alt="${escapeHtml(media.alt)}" width="${media.width}" height="${media.height}" loading="lazy" decoding="async"><span aria-hidden="true">↗</span></a><figcaption>${escapeHtml(media.caption)}</figcaption></figure>`
}

function headingParts(text: string): { id: string; label: string } {
  const match = text.match(/\s+\{#([a-z0-9-]+)\}$/)
  if (!match) throw new Error(`Guide section needs an explicit anchor: ${text}`)
  return { id: match[1], label: text.slice(0, match.index) }
}

/** Markdown is authored locally and rendered only while building the site. */
export interface GuideStep {
  position: number
  name: string
  anchor: string
}

export function renderGuideBody(article: GuideArticle): {
  html: string
  toc: TocEntry[]
  steps: GuideStep[]
} {
  const renderer = new Renderer()
  renderer.heading = function (token) {
    if (token.depth === 1) {
      throw new Error(`Guide body cannot contain another h1: ${article.slug}`)
    }
    const inline = this.parser.parseInline(token.tokens)
    if (token.depth !== 2) {
      return `<h${token.depth}>${inline}</h${token.depth}>\n`
    }
    const heading = headingParts(token.text)
    return `<h2 id="${heading.id}">${inline.replace(/\s+\{#[a-z0-9-]+\}$/, '')}</h2>\n`
  }
  renderer.image = function (token) {
    if (!token.href.startsWith('media:')) {
      throw new Error(`Guide image must use registered media: ${token.href}`)
    }
    return renderMedia(mediaById(article, token.href.slice('media:'.length)))
  }
  renderer.paragraph = function (token) {
    const inline = this.parser.parseInline(token.tokens)
    if (token.tokens.length === 1 && token.tokens[0].type === 'image') {
      return `${inline}\n`
    }
    return `<p>${inline}</p>\n`
  }
  const markdown = new Marked({ renderer, gfm: true, async: false })
  const tokens = markdown.lexer(article.bodyMarkdown)
  const toc: TocEntry[] = []
  const steps: GuideStep[] = []
  let step: string | undefined
  for (const token of tokens) {
    if (token.type === 'html') {
      step = token.text.match(/class="step-label">Step (\d+)<\/p>/)?.[1]
    }
    if (token.type === 'heading' && token.depth === 2) {
      const heading = headingParts(token.text)
      toc.push({
        id: heading.id,
        label: step ? `${step}. ${heading.label}` : heading.label,
      })
      if (step) {
        steps.push({
          position: Number(step),
          name: heading.label,
          anchor: heading.id,
        })
      }
      step = undefined
    }
  }
  const ids = toc.map((entry) => entry.id)
  if (new Set(ids).size !== ids.length || ids.includes(article.next.id)) {
    throw new Error(`Duplicate guide anchor: ${article.slug}`)
  }
  return { html: markdown.parser(tokens), toc, steps }
}

function canonicalUrl(brand: SiteBrand, path: string): string {
  return `${brand.siteUrl.replace(/\/+$/, '')}${path}`
}

function breadcrumbs(
  brand: SiteBrand,
  article?: GuideArticle
): {
  html: string
  graph: Record<string, unknown>
} {
  const items = [
    { name: 'Home', path: '/' },
    { name: 'Guides', path: GUIDE_HUB.path },
    ...(article
      ? [
          {
            name:
              article.slug === 'create-ai-character'
                ? 'Create a character'
                : 'Recreate a reference video',
            path: article.path,
          },
        ]
      : []),
  ]
  return {
    html: `<nav class="breadcrumb" aria-label="Breadcrumb">${items
      .map((item, index) => {
        const separator = index > 0 ? '<span aria-hidden="true">/</span>' : ''
        const text = escapeHtml(item.name)
        return `${separator}${index === items.length - 1 ? `<span aria-current="page">${text}</span>` : `<a href="${item.path}">${text}</a>`}`
      })
      .join('')}</nav>`,
    graph: {
      '@type': 'BreadcrumbList',
      '@id': `${canonicalUrl(brand, article?.path ?? GUIDE_HUB.path)}#breadcrumb`,
      itemListElement: items.map((item, index) => ({
        '@type': 'ListItem',
        position: index + 1,
        name: item.name,
        item: canonicalUrl(brand, item.path),
      })),
    },
  }
}

function guideHead(
  brand: SiteBrand,
  path: string,
  title: string,
  description: string,
  graph: Record<string, unknown>,
  article?: GuideArticle
): string {
  const canonical = canonicalUrl(brand, path)
  const cover = article ? mediaById(article, article.coverId) : undefined
  const ogImage = cover
    ? canonicalUrl(brand, cover.url)
    : brand.ogImageUrl
  return [
    `<title>${escapeHtml(title)}</title>`,
    `<meta name="title" content="${escapeHtml(title)}">`,
    `<meta name="omnimux-seo-title" content="${escapeHtml(title)}">`,
    `<meta name="description" content="${escapeHtml(description)}">`,
    '<meta name="robots" content="index,follow">',
    `<link rel="canonical" href="${escapeHtml(canonical)}">`,
    `<meta property="og:type" content="${article ? 'article' : 'website'}">`,
    `<meta property="og:url" content="${escapeHtml(canonical)}">`,
    `<meta property="og:title" content="${escapeHtml(title)}">`,
    `<meta property="og:description" content="${escapeHtml(description)}">`,
    `<meta property="og:site_name" content="${escapeHtml(brand.brandName)}">`,
    `<meta property="og:image" content="${escapeHtml(ogImage)}">`,
    '<meta property="og:locale" content="en_US">',
    '<meta name="twitter:card" content="summary_large_image">',
    `<meta name="twitter:title" content="${escapeHtml(title)}">`,
    `<meta name="twitter:description" content="${escapeHtml(description)}">`,
    `<meta name="twitter:image" content="${escapeHtml(ogImage)}">`,
    `<script type="application/ld+json">${serializeStructuredData(graph)}</script>`,
  ].join('\n')
}

function studioLink(
  slug: string,
  position: string,
  label = 'Open studio'
): string {
  return `<a class="button primary" href="/influencer" data-umami-event="guide_studio_click" data-umami-event-slug="${escapeHtml(slug)}" data-umami-event-position="${escapeHtml(position)}">${escapeHtml(label)} <span aria-hidden="true">→</span></a>`
}

function header(brand: SiteBrand, slug: string): string {
  return `<a class="skip-link" href="#page-main">Skip to content</a><header class="site-header"><nav class="site-nav" aria-label="Main navigation"><a href="/" class="brand" aria-label="${escapeHtml(brand.brandName)} home"><img src="/logo-wordmark.webp" alt="${escapeHtml(brand.brandName)}" width="440" height="98"></a><div class="header-links"><a href="/#pricing">Pricing</a><a href="/#works">Inspiration</a><a class="nav-login" href="/sign-in">Log in</a>${studioLink(slug, 'header', 'Start')}</div></nav></header>`
}

function footer(brand: SiteBrand): string {
  const links = [
    ['Guides', '/guides'],
    ['Pricing', '/pricing'],
    ['About', '/about'],
    ['Documentation', 'https://docs.omnimux.ai'],
    ['Trends', '/influencer'],
    ['Dashboard', '/dashboard'],
    ['Support', 'mailto:hello@omnimux.ai'],
    ['Terms of Service', '/terms'],
    ['Privacy Policy', '/privacy'],
    ['Refunds', '/refunds'],
    ['Acceptable use', '/acceptable-use'],
  ]
  return `<footer class="site-footer"><div class="footer-shell"><div class="footer-top"><a class="brand" href="/" aria-label="${escapeHtml(brand.brandName)} home"><img src="/logo-wordmark.webp" alt="${escapeHtml(brand.brandName)}" width="440" height="98"></a><nav class="footer-links" aria-label="Footer navigation">${links.map(([label, href]) => `<a href="${href}">${label}</a>`).join('')}</nav></div><p class="language-explanation">Guide articles are in English. The app supports seven interface languages.</p><div class="footer-bottom"><p>Create your own AI influencer</p><p>© 2026 ${escapeHtml(brand.brandName)} · All rights reserved. · Powered by <a href="https://github.com/QuantumNous/new-api">New API</a></p></div></div></footer>`
}

function imageDialog(): string {
  return '<dialog class="image-dialog" aria-label="Enlarged guide image" data-guide-dialog><form method="dialog"><button class="close-button" aria-label="Close enlarged image">×</button></form><img alt=""><p>Press Escape to close</p></dialog>'
}

function documentHtml(
  options: GuideDocumentOptions,
  head: string,
  main: string,
  slug: string
): string {
  const styles = options.cssUrls
    .map((url) => `<link rel="stylesheet" href="${escapeHtml(url)}">`)
    .join('\n')
  const scripts = (options.scriptUrls ?? [])
    .map((url) => `<script src="${escapeHtml(url)}" defer></script>`)
    .join('\n')
  return `<!doctype html>\n<html lang="en"><head>\n<meta charset="utf-8">\n<meta name="viewport" content="width=device-width, initial-scale=1">\n<!--omnimux-page-head-->\n${head}\n<!--/omnimux-page-head-->\n${styles}\n<!--umami-->\n${scripts}\n</head><body class="omnimux-landing omnimux-guides">${header(options.brand, slug)}<main id="page-main">${main}</main>${footer(options.brand)}${imageDialog()}</body></html>\n`
}

function articleGraph(
  options: GuideDocumentOptions,
  article: GuideArticle,
  breadcrumb: Record<string, unknown>,
  steps: GuideStep[]
): Record<string, unknown> {
  const canonical = canonicalUrl(options.brand, article.path)
  const organizationId = `${canonicalUrl(options.brand, '/')}#organization`
  const publishedAt = options.publishedAt ?? article.publishedAt
  const articleNode: Record<string, unknown> = {
    '@type': 'Article',
    '@id': `${canonical}#article`,
    url: canonical,
    headline: article.title,
    description: article.summary,
    inLanguage: 'en',
    datePublished: publishedAt,
    dateModified:
      article.reviewedAt < publishedAt ? publishedAt : article.reviewedAt,
    author: { '@id': organizationId },
    publisher: { '@id': organizationId },
    mainEntityOfPage: canonical,
    image: article.media.map((media) =>
      canonicalUrl(options.brand, media.url)
    ),
  }
  const graph: Record<string, unknown>[] = [
    {
      '@type': 'Organization',
      '@id': organizationId,
      name: options.brand.brandName,
      url: canonicalUrl(options.brand, '/'),
    },
    articleNode,
    breadcrumb,
  ]
  if (steps.length > 0) {
    // Guides mark instructional sections with an explicit Step label; expose
    // them as a HowTo so search engines and answer engines can cite the steps.
    articleNode['about'] = { '@id': `${canonical}#howto` }
    graph.push({
      '@type': 'HowTo',
      '@id': `${canonical}#howto`,
      url: canonical,
      name: article.title,
      description: article.summary,
      inLanguage: 'en',
      totalTime: 'PT5M',
      step: steps.map((entry) => ({
        '@type': 'HowToStep',
        position: entry.position,
        name: entry.name,
        url: `${canonical}#${entry.anchor}`,
      })),
    })
  }
  return {
    '@context': 'https://schema.org',
    '@graph': graph,
  }
}

function renderArticle(
  options: GuideDocumentOptions,
  article: GuideArticle
): GuideDocument {
  const related = GUIDE_REGISTRY.find(
    (item) => item.slug === article.next.relatedSlug
  )
  if (!related) throw new Error(`Unknown related guide: ${article.slug}`)
  const body = renderGuideBody(article)
  const breadcrumb = breadcrumbs(options.brand, article)
  const toc = [
    ...body.toc,
    { id: article.next.id, label: article.next.tocLabel },
  ]
  const reviewDate = new Intl.DateTimeFormat('en', {
    day: 'numeric',
    month: 'long',
    year: 'numeric',
    timeZone: 'UTC',
  }).format(new Date(`${article.reviewedAt}T00:00:00Z`))
  const next = `<section class="article-next" id="${article.next.id}"><p class="eyebrow">${escapeHtml(article.next.eyebrow)}</p><h2>${escapeHtml(article.next.title)}</h2><p>${escapeHtml(article.next.description)}</p><a class="related-link" href="${related.path}"><span>${escapeHtml(related.title)}</span><span aria-hidden="true">→</span></a><div class="article-bottom-actions">${studioLink(article.slug, 'article-bottom')}<a class="text-link" href="/guides">All guides</a></div></section>`
  const main = `<section class="article-page"><div class="page-shell">${breadcrumb.html}<header class="article-header"><p class="eyebrow">${escapeHtml(article.category)} <span class="language-tag">English</span></p><h1>${escapeHtml(article.headingLead)}<br><em>${escapeHtml(article.headingAccent)}</em></h1><p class="lead">${escapeHtml(article.intro)}</p><p class="article-byline"><span>By ${escapeHtml(options.brand.brandName)}</span><span>Reviewed: ${reviewDate}</span></p><p class="article-provenance">${escapeHtml(article.provenance)}</p></header><div class="article-layout"><aside class="article-toc"><nav aria-label="On this page"><p>On this page</p>${toc.map((entry) => `<a href="#${entry.id}">${escapeHtml(entry.label)}</a>`).join('')}</nav>${studioLink(article.slug, 'sidebar')}</aside><article class="article-body">${body.html}${next}</article></div></div></section>`
  const head = guideHead(
    options.brand,
    article.path,
    `${article.title} | ${options.brand.brandName} Guides`,
    article.summary,
    articleGraph(options, article, breadcrumb.graph, body.steps),
    article
  )
  return { head, documentHtml: documentHtml(options, head, main, article.slug) }
}

function renderHub(options: GuideDocumentOptions): GuideDocument {
  const breadcrumb = breadcrumbs(options.brand)
  const cards = GUIDE_REGISTRY.map((article) => {
    const cover = mediaById(article, article.coverId)
    return `<a class="guide-card" href="${article.path}"><div class="guide-cover"><img src="${cover.url}" alt="${escapeHtml(cover.alt)}" width="${cover.width}" height="${cover.height}"></div><div class="guide-card-copy"><span class="eyebrow muted">${article.slug === 'create-ai-character' ? 'Create a character' : 'Recreate a reference'}</span><h2>${escapeHtml(article.title)} <span aria-hidden="true">↗</span></h2><p>${escapeHtml(article.summary)}</p><span class="read-guide">Read the guide <span aria-hidden="true">→</span></span></div></a>`
  }).join('')
  const main = `<section class="hub-page"><div class="page-shell">${breadcrumb.html}<div class="hub-intro"><p class="eyebrow">${escapeHtml(options.brand.brandName)} guides <span class="language-tag">English</span></p><h1>Create a character.<br><em>Make it move.</em></h1><p class="lead">${escapeHtml(GUIDE_HUB.summary)}</p></div><div class="guide-grid">${cards}</div><aside class="preparation-note"><div><span class="note-symbol" aria-hidden="true">↳</span><h2>Already have a character?</h2></div><p>Start with the reference video guide. You can use a suitable character image as video material, including a photo you have permission to use.</p><a class="text-link" href="/guides/recreate-video-with-your-character">Prepare your video inputs →</a></aside><div class="hub-bottom"><p>These guides are in English. The app supports seven interface languages.</p>${studioLink('guides', 'hub-bottom')}</div></div></section>`
  const graph = {
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'CollectionPage',
        '@id': `${canonicalUrl(options.brand, GUIDE_HUB.path)}#webpage`,
        url: canonicalUrl(options.brand, GUIDE_HUB.path),
        name: `${options.brand.brandName} Guides`,
        description: GUIDE_HUB.summary,
        inLanguage: 'en',
        hasPart: GUIDE_REGISTRY.map((article) => ({
          '@type': 'Article',
          '@id': `${canonicalUrl(options.brand, article.path)}#article`,
          url: canonicalUrl(options.brand, article.path),
          headline: article.title,
        })),
      },
      breadcrumb.graph,
    ],
  }
  const head = guideHead(
    options.brand,
    GUIDE_HUB.path,
    GUIDE_HUB.title,
    GUIDE_HUB.summary,
    graph
  )
  return { head, documentHtml: documentHtml(options, head, main, 'guides') }
}

/** Standalone documents contain no application shell or hydration entrypoint. */
export function renderGuideDocuments(
  options: GuideDocumentOptions
): Record<string, GuideDocument> {
  const documents: Record<string, GuideDocument> = {
    [GUIDE_HUB.path]: renderHub(options),
  }
  for (const article of GUIDE_REGISTRY) {
    if (documents[article.path]) {
      throw new Error(`Duplicate guide path: ${article.path}`)
    }
    documents[article.path] = renderArticle(options, article)
  }
  return documents
}
