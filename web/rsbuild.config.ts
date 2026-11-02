import { readFileSync, readdirSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import {
  defineConfig,
  loadEnv,
  type RsbuildPlugin,
  type Rspack,
} from '@rsbuild/core'
import { pluginReact } from '@rsbuild/plugin-react'
import { pluginTailwindcss } from '@rsbuild/plugin-tailwindcss'
import { tanstackRouter } from '@tanstack/router-plugin/rspack'
import { marked } from 'marked'

import { landingStructuredData } from './scripts/landing-structured-data.mjs'
import { renderGuideDocuments } from './src/features/guides/render'
import { acceptableUseContent } from './src/features/legal-pages/content/acceptable-use'
import { privacyContent } from './src/features/legal-pages/content/privacy'
import { refundsContent } from './src/features/legal-pages/content/refunds'
import { termsContent } from './src/features/legal-pages/content/terms'
import { appRouteManifestBuild } from './src/features/site-metadata/app-route-manifest'
import {
  assertGuideDocuments,
  assertSearchPageShell,
} from './src/features/site-metadata/build-contract'
import { HOMEPAGE_DESCRIPTION } from './src/features/site-metadata/content'
import {
  browserSearchPageManifest,
  createSearchPageManifest,
  renderSearchSitemap,
} from './src/features/site-metadata/metadata'
import translations from './src/i18n/locales/en.json'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

export default defineConfig(({ envMode }) => {
  const env = loadEnv({ mode: envMode, prefixes: ['VITE_'] })
  const serverUrl =
    process.env.VITE_REACT_APP_SERVER_URL ||
    env.rawPublicVars.VITE_REACT_APP_SERVER_URL ||
    'http://localhost:3000'

  const isProd = envMode === 'production'
  const appRoutes = appRouteManifestBuild()

  // Build-time brand injection for the HTML shell (title / meta / OG / GEO).
  // Override with env vars when rebranding a deployment; runtime UI text
  // still comes from the DB-backed system_name option (single source of
  // truth). These values cover public metadata before and after app navigation:
  //   VITE_BRAND_NAME        short product name (default "OmniMux") — og:site_name
  //   VITE_SEO_TITLE         full document/OG title (40–60 chars recommended)
  //   VITE_BRAND_DESCRIPTION meta description (140–160 chars recommended)
  //   VITE_SITE_URL          (default "https://omnimux.ai", no trailing slash)
  //   VITE_OG_IMAGE_URL      (default `${siteUrl}/og.png`)
  const brandName =
    process.env.VITE_BRAND_NAME ||
    env.rawPublicVars.VITE_BRAND_NAME ||
    'OmniMux'
  // fork: the homepage is the desktop landing page (2026-10-27).
  const seoTitle =
    process.env.VITE_SEO_TITLE ||
    env.rawPublicVars.VITE_SEO_TITLE ||
    'OmniMux Desktop — Find, Recreate & Scale Viral Shoppable Videos'
  const brandDescription =
    process.env.VITE_BRAND_DESCRIPTION ||
    env.rawPublicVars.VITE_BRAND_DESCRIPTION ||
    HOMEPAGE_DESCRIPTION
  const siteUrl = (
    process.env.VITE_SITE_URL ||
    env.rawPublicVars.VITE_SITE_URL ||
    'https://omnimux.ai'
  ).replace(/\/$/, '')
  const ogImageUrl =
    process.env.VITE_OG_IMAGE_URL ||
    env.rawPublicVars.VITE_OG_IMAGE_URL ||
    `${siteUrl}/og.png`
  const searchPages = createSearchPageManifest(
    { brandName, seoTitle, brandDescription, siteUrl, ogImageUrl },
    landingStructuredData({ brandName, seoTitle, brandDescription, siteUrl }),
    translations.translation,
    {
      privacy: marked.parse(privacyContent.body.en, { async: false }),
      terms: marked.parse(termsContent.body.en, { async: false }),
      refunds: marked.parse(refundsContent.body.en, { async: false }),
      acceptableUse: marked.parse(acceptableUseContent.body.en, {
        async: false,
      }),
    }
  )
  const brand = { brandName, seoTitle, brandDescription, siteUrl, ogImageUrl }
  const guideHeads = renderGuideDocuments({ brand, cssUrls: [] })
  const browserPages = browserSearchPageManifest({
    ...searchPages,
    pages: {
      ...searchPages.pages,
      ...Object.fromEntries(
        Object.entries(guideHeads).map(([url, page]) => [
          url,
          { head: page.head, summary: '' },
        ])
      ),
    },
  })
  const searchPagesPlugin: Rspack.Plugin = {
    apply(compiler: Rspack.Compiler) {
      compiler.hooks.thisCompilation.tap(
        'OmniMuxSearchPages',
        (compilation) => {
          compilation.hooks.processAssets.tap(
            {
              name: 'OmniMuxSearchPages',
              stage: compiler.webpack.Compilation.PROCESS_ASSETS_STAGE_REPORT,
            },
            () => {
              const files =
                compilation.entrypoints.get('guides')?.getFiles() ?? []
              const cssUrls = files
                .filter((file) => file.endsWith('.css'))
                .map((file) => `/${file}`)
              if (isProd && cssUrls.length === 0) {
                throw new Error('Guides require compiled CSS assets.')
              }
              const documents = renderGuideDocuments({
                brand,
                cssUrls,
                scriptUrls: files
                  .filter((file) => file.endsWith('.js'))
                  .map((file) => `/${file}`),
              })
              assertGuideDocuments(documents)
              const manifest = {
                ...searchPages,
                pages: {
                  ...searchPages.pages,
                  ...Object.fromEntries(
                    Object.entries(documents).map(([url, page]) => [
                      url,
                      { ...page, summary: '' },
                    ])
                  ),
                },
              }
              compilation.emitAsset(
                'search-pages.json',
                new compiler.webpack.sources.RawSource(JSON.stringify(manifest))
              )
              compilation.emitAsset(
                'sitemap.xml',
                new compiler.webpack.sources.RawSource(
                  renderSearchSitemap(manifest, siteUrl)
                )
              )
              const mediaPath = path.resolve(
                __dirname,
                'src/features/guides/assets'
              )
              for (const file of readdirSync(mediaPath).filter((file) =>
                file.endsWith('.webp')
              )) {
                compilation.emitAsset(
                  `static/guides/${file}`,
                  new compiler.webpack.sources.RawSource(
                    readFileSync(path.join(mediaPath, file))
                  )
                )
              }
            }
          )
        }
      )
    },
  }
  const searchPageContract: RsbuildPlugin = {
    name: 'omnimux-search-page-contract',
    setup(api) {
      api.modifyHTML((html) => {
        assertSearchPageShell(html)
        return html
      })
    },
  }
  // Mount point of the statically served Inspiration Community catalog
  // (`VITE_INSPIRATION_CATALOG_BASE`, e.g. `/catalog-alt/` for a local run).
  // Empty keeps the feature's same-origin `/inspiration-catalog/` default; the
  // value is defined below so a shell variable works alongside a .env entry,
  // the same way the brand vars above are resolved.
  const inspirationCatalogBase =
    process.env.VITE_INSPIRATION_CATALOG_BASE ||
    env.rawPublicVars.VITE_INSPIRATION_CATALOG_BASE ||
    ''
  const devProxy = Object.fromEntries(
    (['/api', '/v1', '/mj', '/pg'] as const).map((key) => [
      key,
      { target: serverUrl, changeOrigin: true },
    ])
  ) as Record<string, { target: string; changeOrigin: boolean }>

  return {
    plugins: [
      pluginReact(),
      pluginTailwindcss({ optimize: false }),
      searchPageContract,
    ],
    // Rsbuild 2: replaces deprecated `performance.chunkSplit` (RSPack 2 aligned)
    splitChunks: {
      preset: 'default',
      cacheGroups: {
        'vendor-react': {
          test: /node_modules[\\/](react|react-dom)[\\/]/,
          name: 'vendor-react',
          chunks: 'all',
          priority: 0,
          enforce: true,
        },
        'vendor-ui-primitives': {
          test: /node_modules[\\/](@base-ui|@radix-ui)[\\/]/,
          name: 'vendor-ui-primitives',
          chunks: 'all',
          priority: 0,
          enforce: true,
        },
        'vendor-tanstack': {
          test: /node_modules[\\/]@tanstack[\\/]/,
          name: 'vendor-tanstack',
          chunks: 'all',
          priority: 0,
          enforce: true,
        },
        // Heavy libraries that only ever load on demand (KaTeX/marked via the
        // lazily imported markdown renderer, the calendar stack via date
        // pickers inside routes). Sharing a chunk with a vendor the app shell
        // needs — sonner is rendered by the root route — would put them back in
        // the initial download, so give them their own groups.
        'vendor-markdown': {
          test: /node_modules[\\/](katex|marked)[\\/]/,
          name: 'vendor-markdown',
          chunks: 'all',
          priority: 10,
          enforce: true,
        },
        'vendor-pickers': {
          test: /node_modules[\\/](react-day-picker|date-fns)[\\/]/,
          name: 'vendor-pickers',
          chunks: 'all',
          priority: 10,
          enforce: true,
        },
      },
    },
    source: {
      entry: {
        index: './src/main.tsx',
        guides: { import: './src/features/guides/browser.ts', html: false },
      },
      define: {
        'import.meta.env.VITE_INSPIRATION_CATALOG_BASE': JSON.stringify(
          inspirationCatalogBase
        ),
        'import.meta.env.OMNIMUX_SEARCH_PAGES': JSON.stringify(browserPages),
      },
    },
    resolve: {
      alias: {
        '@': path.resolve(__dirname, './src'),
        // MediaInfo's browser bundle references this adjacent WASM asset, while
        // its published package exports the binary from dist/ instead.
        'MediaInfoModule.wasm$': path.resolve(
          __dirname,
          'node_modules/mediainfo.js/dist/MediaInfoModule.wasm'
        ),
      },
    },
    html: {
      // The template receives its title through pageHead; suppress automatic injection.
      title: '',
      template: './index.html',
      templateParameters: {
        pageHead: searchPages.pages['/'].head,
        pageSummary: searchPages.pages['/'].summary,
      },
      favicon: './public/favicon.ico',
    },
    server: {
      host: '0.0.0.0',
      strictPort: false,
      proxy: devProxy,
    },
    output: {
      // Production optimizations
      minify: isProd,
      target: 'web',
      distPath: {
        root: 'dist',
      },
      // Rely on Rsbuild default legalComments ("linked" → per-chunk *.LICENSE.txt) in all modes.
      // Do not set "none" in production: that strips minifier-preserved third-party notices and
      // extracted license files, which some distributions require for open-source compliance.
    },
    performance: {
      // Remove console in production
      removeConsole: isProd ? ['log'] : false,
      buildCache: false,
    },
    tools: {
      rspack: {
        plugins: [
          searchPagesPlugin,
          appRoutes.rspack,
          tanstackRouter({
            target: 'react',
            plugins: [appRoutes.generator],
            // Dev: avoid per-route async chunks (reduces white flash on navigation + faster HMR feedback).
            // Prod: keep route-based code splitting.
            autoCodeSplitting: isProd,
          }),
        ],
      },
    },
  }
})
