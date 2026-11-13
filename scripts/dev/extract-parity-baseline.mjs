#!/usr/bin/env node
/**
 * Extract a compact parity baseline from EvoLink model-page HTML snapshots.
 *
 * Why this exists: the 1:1 replica is verified against the benchmark page, so
 * the benchmark's module order and copy must survive as reviewable repo
 * evidence instead of living in a scratch directory. Raw snapshots are ~320KB
 * each and are NOT committed; this script distils them into a small JSON that
 * IS committed, and it can be re-run any time the benchmark changes.
 *
 * Usage:
 *   node scripts/dev/extract-parity-baseline.mjs <snapshot.html>[:<id>] ...
 *
 * Refresh the snapshots first (they are SSR HTML; no JavaScript execution, so
 * anything client-rendered is absent — see model-page-parity-checklist.json
 * entries marked "source": "live"):
 *
 *   curl -sSL -A "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7)" \
 *     https://evolink.ai/zh/seedance-2-5 -o /tmp/seedance.html
 *
 * Output: docs/product/parity-baseline.json
 */
import { readFileSync, writeFileSync } from 'node:fs'
import { basename, dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const OUTPUT = join(ROOT, 'docs', 'product', 'parity-baseline.json')

/** Page-level anchors the benchmark page navigates to. */
const ANCHORS = ['playground', 'pricing', 'about', 'api']

function decodeEntities(input) {
  return input
    .replace(/&nbsp;/g, ' ')
    .replace(/&amp;/g, '&')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&quot;/g, '"')
    .replace(/&#x27;|&#39;/g, "'")
}

function stripTags(segment) {
  const withoutBlocks = segment.replace(
    /<(script|style|noscript)[^>]*>[\s\S]*?<\/\1>/gi,
    ' '
  )
  const text = decodeEntities(withoutBlocks.replace(/<[^>]+>/g, ' '))
  return text.replace(/\s+/g, ' ').trim()
}

/** Ordered heading + control outline, matching the module order on the page. */
function extractOutline(html) {
  const pattern =
    /<(h1|h2|h3|h4|button|label|th)\b([^>]*)>([\s\S]*?)<\/\1>/gi
  const outline = []
  for (const match of html.matchAll(pattern)) {
    const tag = match[1].toLowerCase()
    const text = stripTags(match[3])
    if (!text || text.length > 160) continue
    const entry = { tag, text }
    if (tag === 'button') {
      const aria = /aria-label="([^"]{0,60})"/i.exec(match[2])
      if (aria) entry.ariaLabel = aria[1]
    }
    const previous = outline.at(-1)
    if (previous && previous.tag === tag && previous.text === text) continue
    outline.push(entry)
  }
  return outline
}

/** Anchor hrefs, preserving document order and de-duplicating. */
function extractAnchors(html) {
  const anchors = []
  for (const match of html.matchAll(/<a\b([^>]*)>([\s\S]*?)<\/a>/gi)) {
    const href = /href="([^"]*#[^"]*)"/i.exec(match[1])
    if (!href) continue
    const text = stripTags(match[2])
    if (!text || text.length >= 30) continue
    if (anchors.some((a) => a.href === href[1] && a.text === text)) continue
    anchors.push({ href: href[1], text })
  }
  return anchors
}

/** Text of each anchored section, bounded by the next anchor position. */
function extractSections(html) {
  const marks = []
  for (const anchor of ANCHORS) {
    const at = html.indexOf(`id="${anchor}"`)
    if (at >= 0) marks.push({ anchor, at })
  }
  marks.sort((a, b) => a.at - b.at)

  const sections = {}
  marks.forEach((mark, index) => {
    const end =
      index + 1 < marks.length
        ? marks[index + 1].at
        : Math.min(mark.at + 40_000, html.length)
    sections[mark.anchor] = stripTags(html.slice(mark.at, end))
  })

  const missing = ANCHORS.filter((anchor) => !(anchor in sections))
  return { sections, missing }
}

function loadSnapshots(args) {
  if (args.length === 0) {
    throw new Error(
      'no snapshots given; pass <file.html>[:<id>] arguments (see script header)'
    )
  }
  return args.map((arg) => {
    const [path, explicitId] = arg.split(':')
    const html = readFileSync(path, 'utf8')
    const id = explicitId || basename(path).replace(/\.html?$/i, '')
    return { id, path, html }
  })
}

function main() {
  const snapshots = loadSnapshots(process.argv.slice(2))
  const baseline = {
    generated_at: new Date().toISOString().slice(0, 10),
    note: 'Baseline distilled from SSR HTML snapshots. Client-rendered sections (see checklist entries with source=live) are intentionally absent.',
    pages: {},
  }

  for (const snapshot of snapshots) {
    const { sections, missing } = extractSections(snapshot.html)
    baseline.pages[snapshot.id] = {
      source_file: basename(snapshot.path),
      source_bytes: Buffer.byteLength(snapshot.html, 'utf8'),
      anchors: extractAnchors(snapshot.html),
      outline: extractOutline(snapshot.html),
      sections,
      sections_missing_from_ssr: missing,
    }
  }

  writeFileSync(OUTPUT, `${JSON.stringify(baseline, null, 2)}\n`, 'utf8')
  const summary = Object.entries(baseline.pages)
    .map(
      ([id, page]) =>
        `${id}: ${page.outline.length} outline nodes, anchors=[${page.anchors
          .map((a) => a.href)
          .join(' ')}], missing_ssr_sections=[${page.sections_missing_from_ssr.join(
          ' '
        )}]`
    )
    .join('\n')
  process.stdout.write(`wrote ${OUTPUT}\n${summary}\n`)
}

main()
