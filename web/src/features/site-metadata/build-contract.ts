/** Validate the emitted shell, after the bundler has injected its own tags. */
export function assertSearchPageShell(html: string): void {
  const titles = [...html.matchAll(/<title(?:\s[^>]*)?>[\s\S]*?<\/title\s*>/gi)]
  const start = html.indexOf('<!--omnimux-page-head-->')
  const end = html.indexOf('<!--/omnimux-page-head-->')
  const title = titles[0]
  if (
    titles.length !== 1 ||
    start < 0 ||
    end <= start ||
    !title ||
    title.index <= start ||
    title.index + title[0].length > end
  ) {
    throw new Error(
      'Search page shell must contain exactly one title inside its page-head block.'
    )
  }
}

/** A missing tutorial must fail the release instead of shipping a SPA shell. */
export function assertGuideDocuments(
  documents: Record<string, { documentHtml: string }>
): void {
  const paths = [
    '/guides',
    '/guides/create-ai-character',
    '/guides/recreate-video-with-your-character',
  ]
  for (const path of paths) {
    const html = documents[path]?.documentHtml ?? ''
    const visible = html.replaceAll(/<noscript\b[^>]*>[\s\S]*?<\/noscript>/gi, '')
    if (
      !/^<!doctype html>/i.test(html) ||
      !/<main\b/i.test(visible) ||
      !/<h1\b/i.test(visible) ||
      !/<\/html>/i.test(html)
    ) {
      throw new Error(`Missing complete guide document: ${path}`)
    }
  }
}
