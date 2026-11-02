import { DOCS_URL, X_URL } from '../omnimux-landing/lib/landing-urls'
import {
  ABOUT_PRICING,
  ABOUT_RIGHTS,
  ABOUT_STEPS,
  ABOUT_TITLE,
  ABOUT_TUTORIAL_TITLE,
  HOMEPAGE_DESCRIPTION,
  HOMEPAGE_STEPS,
  PRODUCT_DESCRIPTION,
} from './content'
import { serializeStructuredData } from './structured-data.mjs'
import { toolSearchPages } from './tool-pages'

export interface SiteBrand {
  brandName: string
  seoTitle: string
  brandDescription: string
  siteUrl: string
  ogImageUrl: string
}

export interface SearchPageFragments {
  head: string
  summary: string
  /** Server-only complete documents; never serialize these into application JS. */
  documentHtml?: string
}

export interface SearchPageManifest {
  pages: Record<string, SearchPageFragments>
  private: SearchPageFragments
}

export interface SearchPage {
  path?: string
  title: string
  description: string
  summary: string
  graph?: Record<string, unknown>
  /** Language alternates of the same page, rendered as hreflang links. */
  alternates?: { hreflang: string; path: string }[]
  ogLocale?: string
}

export function escapeHtml(value: string): string {
  return value
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

function renderHead(brand: SiteBrand, page: SearchPage): string {
  const title = escapeHtml(page.title)
  const description = escapeHtml(page.description)
  const canonical = page.path ? `${brand.siteUrl}${page.path}` : undefined
  const graph = page.graph ?? {
    '@context': 'https://schema.org',
    '@type': 'WebPage',
    ...(canonical ? { '@id': `${canonical}#webpage`, url: canonical } : {}),
    name: page.title,
    description: page.description,
    ...(canonical
      ? {
          isPartOf: {
            '@type': 'WebSite',
            name: brand.brandName,
            url: `${brand.siteUrl}/`,
          },
        }
      : {}),
  }
  return [
    `<title>${title}</title>`,
    `<meta name="title" content="${title}" />`,
    `<meta name="omnimux-seo-title" content="${title}" />`,
    `<meta name="description" content="${description}" />`,
    `<meta name="robots" content="${canonical ? 'index,follow' : 'noindex,follow'}" />`,
    ...(canonical
      ? [`<link rel="canonical" href="${escapeHtml(canonical)}" />`]
      : []),
    ...(page.alternates ?? []).map(
      (alternate) =>
        `<link rel="alternate" hreflang="${escapeHtml(alternate.hreflang)}" href="${escapeHtml(`${brand.siteUrl}${alternate.path}`)}" />`
    ),
    '<meta property="og:type" content="website" />',
    ...(canonical
      ? [`<meta property="og:url" content="${escapeHtml(canonical)}" />`]
      : []),
    `<meta property="og:title" content="${title}" />`,
    `<meta property="og:description" content="${description}" />`,
    `<meta property="og:site_name" content="${escapeHtml(brand.brandName)}" />`,
    `<meta property="og:image" content="${escapeHtml(brand.ogImageUrl)}" />`,
    `<meta property="og:locale" content="${escapeHtml(page.ogLocale ?? 'en_US')}" />`,
    '<meta name="twitter:card" content="summary_large_image" />',
    `<meta name="twitter:title" content="${title}" />`,
    `<meta name="twitter:description" content="${description}" />`,
    `<meta name="twitter:image" content="${escapeHtml(brand.ogImageUrl)}" />`,
    `<script type="application/ld+json">${serializeStructuredData(graph)}</script>`,
  ].join('\n')
}

function paragraph(text: string): string {
  return `<p>${escapeHtml(text)}</p>`
}

function summary(title: string, body: string, brand: SiteBrand): string {
  return `<noscript><div id="geo-ssr"><header>${paragraph(`${brand.brandName} · ${new URL(brand.siteUrl).hostname}`)}<nav><a href="${escapeHtml(brand.siteUrl)}/">Home</a> · <a href="${escapeHtml(brand.siteUrl)}/about">About</a> · <a href="${DOCS_URL}/">Docs</a> · <a href="${escapeHtml(brand.siteUrl)}/guides">Guides (English)</a></nav></header><main><h1>${escapeHtml(title)}</h1>${body}</main></div></noscript>`
}

/** Build and browser navigation consume this same explicit public-page allowlist. */
export function createSearchPageManifest(
  brand: SiteBrand,
  landingGraph: Record<string, unknown>,
  translations: Record<string, unknown>,
  legal: { privacy: string; terms: string; refunds: string; acceptableUse: string }
): SearchPageManifest {
  const translatedText = (key: string): string => {
    const value = translations[key]
    if (typeof value !== 'string') throw new Error(`Missing page text: ${key}`)
    return value
  }
  // fork: the homepage is the desktop landing page. Its noscript summary must
  // describe the same workflow a crawler would otherwise only see as a shell.
  const homepage = summary(
    brand.seoTitle,
    `${paragraph(
      HOMEPAGE_DESCRIPTION
    )}<h2>How it works</h2><ol>${HOMEPAGE_STEPS.map((step) => `<li>${escapeHtml(step)}</li>`).join('')}</ol>${paragraph(
      ABOUT_PRICING
    )}<h2>Start creating</h2><p><a href="${escapeHtml(brand.siteUrl)}/pricing">Model pricing</a> · <a href="${escapeHtml(brand.siteUrl)}/inspiration">Inspiration</a> · <a href="${escapeHtml(brand.siteUrl)}/ai-influencer-studio">Character studio</a></p>`,
    brand
  )
  const about = summary(
    ABOUT_TITLE,
    `${paragraph(
      PRODUCT_DESCRIPTION
    )}<h2>${ABOUT_TUTORIAL_TITLE}</h2><ol>${ABOUT_STEPS.map((step) => `<li>${escapeHtml(step)}</li>`).join('')}</ol>${paragraph(
      ABOUT_RIGHTS
    )}${paragraph(
      ABOUT_PRICING
    )}<h2>Official links</h2><ul><li><a href="${escapeHtml(brand.siteUrl)}/">${escapeHtml(brand.siteUrl)}/</a></li><li><a href="${DOCS_URL}/">${DOCS_URL}/</a></li><li><a href="${X_URL}">X · @OmniMuxai</a></li></ul>`,
    brand
  )
  const pages: (SearchPage & { path: string })[] = [
    {
      path: '/',
      title: brand.seoTitle,
      description: brand.brandDescription,
      summary: homepage,
      graph: landingGraph,
    },
    {
      path: '/about',
      title: `${ABOUT_TITLE} | AI Characters and Reference Videos`,
      description: PRODUCT_DESCRIPTION,
      summary: about,
    },
    {
      path: '/ai-influencer-studio',
      title: `${brand.brandName} AI Influencer Studio | AI Characters and Reference Videos`,
      description: PRODUCT_DESCRIPTION,
      summary: summary(
        'AI Influencer Studio',
        `${paragraph(
          PRODUCT_DESCRIPTION
        )}<h2>${ABOUT_TUTORIAL_TITLE}</h2><ol>${ABOUT_STEPS.map((step) => `<li>${escapeHtml(step)}</li>`).join('')}</ol>${paragraph(ABOUT_RIGHTS)}${paragraph(ABOUT_PRICING)}<h2>Start creating</h2><p><a href="${escapeHtml(brand.siteUrl)}/influencer">Open the studio</a></p>`,
        brand
      ),
    },
    {
      path: '/privacy',
      title: `${brand.brandName} Privacy Policy`,
      description:
        'Read how OmniMux collects, uses, shares, and protects personal data and how to exercise your privacy rights.',
      summary: summary('Privacy Policy', legal.privacy, brand),
    },
    {
      path: '/terms',
      title: `${brand.brandName} Terms of Service`,
      description:
        'Read the terms for using OmniMux, including accounts, generated content, payments, credits, and acceptable use.',
      summary: summary('Terms of Service', legal.terms, brand),
    },
    {
      path: '/refunds',
      title: `${brand.brandName} Refund Policy`,
      description:
        'Read when OmniMux returns generation credits: automatic refunds for failed generations, broken results, credits, and subscriptions.',
      summary: summary('Refund Policy', legal.refunds, brand),
    },
    {
      path: '/acceptable-use',
      title: `${brand.brandName} Acceptable Use Policy`,
      description:
        'Read the acceptable use rules for OmniMux: permitted content, likeness and media authorization, and prohibited uses.',
      summary: summary('Acceptable Use Policy', legal.acceptableUse, brand),
    },
    {
      path: '/pricing',
      title: `${brand.brandName} Model Pricing | Generation Rates and Credits`,
      description:
        'Browse current OmniMux model generation rates, token and credit pricing, recharge options, optional subscriptions, and pay-per-generation costs.',
      summary: summary(
        'Model pricing and generation credits',
        `${paragraph(
          'Browse the live catalog of generation rates across text, image, video, audio, and social models. Prices and credit balances update from your wallet and the current price list.'
        )}${paragraph(
          'Choose an optional pack or subscription, or top up and pay per generation. Check the pricing page for the exact rate before you commit to a generation.'
        )}<h2>FAQ</h2><h3>${escapeHtml(translatedText('influencer_landing.faq.q2'))}</h3>${paragraph(translatedText('influencer_landing.faq.a2'))}<h3>${escapeHtml(translatedText('influencer_landing.faq.q3'))}</h3>${paragraph(translatedText('influencer_landing.faq.a3'))}`,
        brand
      ),
    },
    {
      path: '/inspiration',
      title: `${brand.brandName} Inspiration | AI Character and Video Ideas`,
      description:
        'Browse community-shared AI characters, prompts, and reference video recreations on OmniMux. Discover ideas and start your own character in the studio.',
      summary: summary(
        'Inspiration community',
        `${paragraph(
          'A gallery of AI characters, prompts, and recreated videos shared by OmniMux creators. Browse looks, study prompts, and start your own character in the studio.'
        )}${paragraph(
          'Sign in to publish your own result to the gallery or to generate a character video from a reference clip.'
        )}<h2>Start creating</h2><p><a href="${escapeHtml(brand.siteUrl)}/guides">Read the character and video guides</a> or <a href="${escapeHtml(brand.siteUrl)}/influencer">open the studio</a>.</p>`,
        brand
      ),
    },
    // fork: TikTok tool pages (en + zh-CN). Copy is shared with the page UI.
    ...toolSearchPages(brand, { escapeHtml, paragraph, summary }),
  ]
  return {
    pages: Object.fromEntries(
      pages.map((page) => [
        page.path,
        { head: renderHead(brand, page), summary: page.summary },
      ])
    ),
    private: {
      head: renderHead(brand, {
        title: `${brand.brandName} Workspace`,
        description: PRODUCT_DESCRIPTION,
        summary: '',
      }),
      summary: '',
    },
  }
}

/** The application only updates metadata. Bodies stay in the server manifest. */
export function browserSearchPageManifest(
  manifest: SearchPageManifest
): SearchPageManifest {
  return {
    pages: Object.fromEntries(
      Object.entries(manifest.pages).map(([path, page]) => [
        path,
        { head: page.head, summary: '' },
      ])
    ),
    private: { head: manifest.private.head, summary: '' },
  }
}

/** Sitemap and public documents share the same reviewed page registry. */
export function renderSearchSitemap(
  manifest: SearchPageManifest,
  siteUrl: string
): string {
  return `<?xml version="1.0" encoding="UTF-8"?>\n<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">\n${Object.keys(
    manifest.pages
  )
    .map((path) => `  <url><loc>${escapeHtml(`${siteUrl}${path}`)}</loc></url>`)
    .join('\n')}\n</urlset>\n`
}

export function resolveSearchPage(
  manifest: SearchPageManifest,
  pathname: string
): SearchPageFragments {
  const path = pathname.split(/[?#]/, 1)[0].replace(/\/+$/, '') || '/'
  // Inspiration share links reuse the gallery metadata; the server renders a
  // share-specific head on the initial load.
  if (path.startsWith('/s/')) {
    return manifest.pages['/inspiration'] ?? manifest.private
  }
  return manifest.pages[path] ?? manifest.private
}
