import { resolveSearchPage, type SearchPageManifest } from './metadata'

const PAGE_METADATA_SELECTOR = [
  'title',
  'meta[name="title"]',
  'meta[name="omnimux-seo-title"]',
  'meta[name="description"]',
  'meta[name="robots"]',
  'link[rel="canonical"]',
  'meta[property^="og:"]',
  'meta[name^="twitter:"]',
  'script[type="application/ld+json"]',
].join(',')

/** Remove the previous page's canonical and graph before applying the next page. */
export function applyPageMetadata(
  manifest: SearchPageManifest,
  pathname: string
): void {
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
