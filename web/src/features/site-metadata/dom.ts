import { resolveSearchPage, type SearchPageManifest } from './metadata'

const PAGE_METADATA_SELECTOR = [
  'title',
  'meta[name="title"]',
  'meta[name="omnimux-seo-title"]',
  'meta[name="omnimux-server-head-path"]',
  'meta[name="description"]',
  'meta[name="robots"]',
  'link[rel="canonical"]',
  'link[rel="alternate"][hreflang]',
  'meta[property^="og:"]',
  'meta[name^="twitter:"]',
  'script[type="application/ld+json"]',
].join(',')

function isQuarantineSearch(search: string): boolean {
  const value = new URLSearchParams(search)
    .get('quarantine')
    ?.trim()
    .toLowerCase()
  return value === '1' || value === 'true'
}

/** Remove the previous page's canonical, alternates, and graph before applying the next page. */
export function applyPageMetadata(
  manifest: SearchPageManifest,
  pathname: string,
  search = typeof window !== 'undefined' ? window.location.search : ''
): void {
  const normalizedPath = pathname.split(/[?#]/, 1)[0].replace(/\/+$/, '') || '/'
  const serverHeadPath = document.head
    .querySelector('meta[name="omnimux-server-head-path"]')
    ?.getAttribute('content')
  // Preserve dynamic server-rendered heads (e.g. /inspiration/models/$slug)
  // across initial SPA mount unless ?quarantine=1 triggers the private fallback.
  if (serverHeadPath === normalizedPath && !isQuarantineSearch(search)) {
    return
  }
  const fragments = resolveSearchPage(manifest, pathname)
  const parsed = new DOMParser().parseFromString(
    `<head>${fragments.head}</head>`,
    'text/html'
  )
  document.head
    .querySelectorAll(PAGE_METADATA_SELECTOR)
    .forEach((node) => node.remove())
  for (const node of parsed.head.children) {
    document.head.appendChild(document.importNode(node, true))
  }
}
