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
 * Build the static paginated JSON catalog for the Inspiration Community.
 *
 * The catalog is the production replacement for the bundled TypeScript demo
 * fixture (`mock/prompt-items.ts`, see `build-inspiration-mock.mjs`): the same
 * `PromptItem` rows, but tens of thousands of them, served as a static
 * directory of JSON files rather than shipped inside the web bundle. The shape
 * mirrors the source site's own public layout (a `manifest.json` plus
 * `{zone}/page-NNNN.json` on object storage).
 *
 * ## Inputs (read-only, all outside this repo)
 *
 * Paths are relative to `DATASETS_ROOT` (the parent of this repository unless
 * `INSPIRATION_DATASETS_ROOT` is set):
 *
 *   - Topview: topview-prompt-library/data/community-feed.jsonl
 *   - EvoLink: evolink-prompt-library/data/community-feed-zh.jsonl
 *   - Skillry: Github/awesome-opus5-5-videos/data/videos.json
 *     (one JSON array; override with `--skillry-file` or `SKILLRY_VIDEOS_FILE`)
 *   - Loomi:   loomi-prompt-library/data/items.jsonl
 *   - Topview/EvoLink poster sizes: the checked-in `scripts/data/poster-ratios.json`
 *   - Loomi poster sizes:           `<loomi>/raw/ratios.json`, written by that
 *     dataset's own collector. It may be missing or still growing while the
 *     crawl runs: a missing/unreadable map is reported loudly and every
 *     unmeasured poster falls back to `FALLBACK_ASPECT_RATIO`, so the build
 *     still succeeds and re-running once the map lands is the whole fix.
 *   - Loomi zone labels:            `<loomi>/raw/templates/manifest.json`, the
 *     manifest the collector already cached. Only if that cache is absent does
 *     this script fetch the public URL
 *     (`https://storage.googleapis.com/loomi-open-prompts/catalog/templates/manifest.json`)
 *     and cache the response in the OS temp dir — the dataset repo is
 *     read-only for this builder.
 *
 * Field mapping for Topview, EvoLink and Skillry lives in
 * `lib/inspiration-sources.mjs` and is shared with the mock builder, so the
 * two can never disagree about what a `PromptItem` means. Only the Loomi
 * mapping is new here.
 *
 * ## Output contract
 *
 *   <out>/manifest.json
 *   <out>/<zone>/page-0000.json        (48 displayable rows, 4-digit padding)
 *   <out>/<zone>/page-0001.json
 *   ...
 *   <out>/<zone>/quarantine-0000.json  (48 quarantined rows, same padding)
 *   ...
 *
 * Never point `--out` inside the repository: the directory is hundreds of
 * megabytes of third-party prompt text and MUST NOT be committed.
 *
 * `manifest.json` carries `version`, `generatedAt`, `pageSize`, `total`,
 * `quarantinedTotal`, per-source row accounting, and one entry per zone with
 * its label, media type, displayable and quarantined row/page counts and the
 * distinct facet values present in that zone.
 *
 * The facet arrays are derived from the DISPLAYABLE rows only, so every chip
 * the frontend offers is guaranteed to match at least one row the user can
 * actually see. They are emitted in DESCENDING COVERAGE order — the number of
 * displayable rows in that zone carrying the value, ties broken by
 * `localeCompare` — and NOT alphabetically. That ordering is a contract, not a
 * style choice: one zone can carry 61 free-text category values (the merged
 * catalog unions three sources' vocabularies), so the frontend caps the chip
 * row and reveals the rest on demand, and a cap is only honest when the head
 * of the array is the vocabulary that actually covers the zone. Re-sorting the
 * arrays alphabetically on either side discards that signal. The ordering
 * itself lives in `lib/inspiration-facets.mjs`, next to the tests that pin it.
 *
 * A facet key names the row
 * field the frontend must filter on: `categories` -> `category`, `useCases` ->
 * `useCases`, `styles` -> `styles`, and `subjects` -> `tags`. The last one is
 * not a typo: `PromptItem` has no dedicated subjects field, so the subject
 * dimension a source exposes rides in `tags` (for Loomi, `tags` is the whole
 * filter map minus nothing, i.e. subject and scene included; for Topview,
 * `tags` is `tagsSource`, which unions every facet dimension). The `subjects`
 * array therefore lists the values that appear in a displayable row's `tags`
 * and in neither its `useCases` nor its `styles` — the labels beyond the two
 * dimensions that already have a field of their own.
 *
 * A page file is `{ zone, page, pageSize, total, totalPages, items }`, i.e.
 * `PageSlice<PromptItem>` plus the zone id, so a page is self-describing and
 * can be fetched by URL alone. The envelope is IDENTICAL for both sets and
 * `total`/`totalPages` always describe the set that file belongs to: in a
 * `quarantine-NNNN.json` they are the quarantine numbers, not the zone's.
 *
 * ## Displayable vs quarantined rows
 *
 * A row whose displayed media is permanently gone (`--missing=<file|dir>`, a
 * `{ url: reason }` map such as the crawler's `*missing*.json`) is dirty data:
 * its poster 404s, so it must not be shown, but it must stay retrievable for
 * auditing. It is therefore neither displayed NOR dropped — it is quarantined:
 *
 *   - `page-NNNN.json` holds displayable rows only, and the zone's `count`,
 *     `pages` and facet arrays describe exactly those rows, so a normally
 *     paging viewer never downloads a quarantined byte and never counts one.
 *   - `quarantine-NNNN.json` holds the dirty rows — same 48 per page, same row
 *     shape plus `posterUnavailable: true` and
 *     `posterUnavailableReason: <reason>` — as a separate, opt-in set.
 *
 * Membership in the missing map is decided on `displayedMediaUrl(row)` (the
 * poster when there is one, the still frame otherwise), the same precedence the
 * cards use. Quarantining is a third, separate bucket: it never mixes into the
 * gate drops below, and `--missing` is optional — with no flag the catalog is
 * built exactly as before, with zero quarantined rows.
 *
 * ## Row shape
 *
 * Every row is a `PromptItem` (see
 * `src/features/inspiration-community/types.ts`) plus one additive field on
 * every row, and two more on quarantined rows only:
 *
 *   - `derivedFields: string[]` — which fields this builder COMPUTED instead
 *     of reading them from the source row or from a source-provided lookup.
 *     Empty for a row that is entirely supplied. Values are `'viewCount'`
 *     (Topview/EvoLink, where the crawl carries no counter and the demo
 *     stand-in from the shared mapper is used) and `'authorName'` /
 *     `'publishedAt'` (Loomi, where they are parsed out of an `x.com`
 *     permalink). `aspectRatio` is NOT listed: it is read from a measured
 *     lookup, and the unmeasured fallback is reported as a coverage statistic
 *     instead.
 *   - `posterUnavailable: true` and `posterUnavailableReason: <reason>` —
 *     quarantined rows only, copied verbatim from the `--missing` map.
 *
 * ## Loomi -> PromptItem
 *
 *   - `title`        <- `titleZh || title`
 *   - `titleEn`      <- `title || titleZh`
 *   - `description`  <- `descriptionZh || description`
 *   - `descriptionEn` <- `description || descriptionZh`
 *   - `prompt`       <- the full `prompt` text, never truncated (verified: the
 *                       mapper asserts it is byte-identical to the source)
 *   - `modelKey`     <- `zoneId` (identical to `modelId` on every row)
 *   - `modelLabel`   <- the zone label from Loomi's manifest
 *   - `mediaType`    <- the row's `mediaType` (`image` unless `video`)
 *   - `category`     <- first entry of `categoryPath`
 *   - `useCases`     <- `filters.use_case`
 *   - `styles`       <- `filters.style`
 *   - `tags`         <- every value of every `filters` dimension, deduped, in
 *                       sorted-key order (mirrors Topview's `tagsSource`)
 *   - `previewImageUrl` / `posterUrl` <- `posterUrl`; `previewVideoUrl` <-
 *                       `videoUrl` when present
 *   - `aspectRatio`  <- the measured size of `posterUrl`
 *   - `sourceUrl`    <- the row's `sourceUrl`
 *   - `sourcePlatform` <- `'loomi'`, matching the EvoLink convention of
 *                       naming the dataset rather than the origin host
 *   - `authorName`   <- derived from the `x.com` handle in `sourceUrl`,
 *                       `''` otherwise
 *   - `publishedAt`  <- derived from the tweet snowflake id in `sourceUrl`
 *                       (UTC, second precision), `''` otherwise
 *   - `viewCount` / `originalViewCount` <- `0`; the upstream payload has no
 *                       counters at all and this builder invents none
 *
 * Loomi rows carrying neither a usable prompt nor a poster are dropped, as are
 * Topview/EvoLink rows the shared mapper already rejects; every drop is
 * counted by reason and printed.
 *
 * ## Determinism
 *
 * Identical inputs produce byte-identical output apart from `generatedAt`,
 * which is overridable via `--generated-at=<iso>` or `SOURCE_DATE_EPOCH`, so
 * two runs can be diffed. Rows inside a zone are ordered newest-first and then
 * by id; facet arrays are ordered by descending coverage with a
 * `localeCompare` tie-break.
 *
 * Usage (from web/):
 *   bun scripts/build-inspiration-catalog.mjs
 *   bun scripts/build-inspiration-catalog.mjs --out=/tmp/inspiration-catalog
 *   bun scripts/build-inspiration-catalog.mjs --generated-at=2026-09-12T00:00:00.000Z
 *   bun scripts/build-inspiration-catalog.mjs --missing=/path/to/ratios-missing.json
 *   bun scripts/build-inspiration-catalog.mjs \
 *     --missing=../loomi-prompt-library/raw/ratios-missing.json \
 *     --missing=../inspiration-catalog-build/topview-missing.json
 *
 * `--missing` may be repeated and each value may be a FILE or a DIRECTORY. A
 * directory is scanned for `*missing*.json` only — never every `*.json`, because
 * a directory that also holds a poster-size map would otherwise mark every
 * measured URL as dead. Each file must be `{ url: reason }` with STRING values;
 * a value of another type aborts the build with the offending file named (that
 * shape means the file is a poster-size map, not a missing-media map).
 * No `--missing` flag means no quarantine set at all.
 */
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import readline from 'node:readline'

import { zoneFacets } from './lib/inspiration-facets.mjs'
import {
  CATALOG_LIMITS,
  DATASETS_ROOT,
  EVOLINK_FEED,
  FALLBACK_ASPECT_RATIO,
  MODEL_SECTIONS,
  POSTER_RATIOS_FILE,
  REPO_ROOT,
  SKILLRY_VIDEOS_FILE,
  TOPVIEW_FEED,
  attachAspectRatios,
  collectEvolink,
  collectSkillry,
  collectTopview,
  derivePublishedAt,
  deriveXHandle,
  displayedMediaUrl,
  isHttpUrl,
  loadPosterRatios,
  normalizeAspectRatio,
  readableRequirement,
  truncate,
  verbatimText,
} from './lib/inspiration-sources.mjs'

const DEFAULT_OUT_DIR = '/tmp/inspiration-catalog'
const DEFAULT_LOOMI_DIR = path.join(DATASETS_ROOT, 'loomi-prompt-library')

const PAGE_SIZE = 48
const MANIFEST_VERSION = 1

const LOOMI_ITEMS_FILE = 'data/items.jsonl'
const LOOMI_RATIOS_FILE = 'raw/ratios.json'
const LOOMI_MANIFEST_CACHES = [
  'raw/templates/manifest.json',
  'raw/evidence/manifest.snapshot.json',
]
const LOOMI_MANIFEST_URL =
  'https://storage.googleapis.com/loomi-open-prompts/catalog/templates/manifest.json'

/** Zone ids become directory names, so they are restricted to a safe subset. */
const ZONE_ID_PATTERN = /^[a-z0-9][a-z0-9._-]*$/u

/** Reason recorded when a missing-media map gives an entry an empty string. */
const QUARANTINE_REASON_FALLBACK = 'unknown'

/** A directory passed to `--missing` contributes only clearly-missing files. */
const MISSING_FILE_PATTERN = /missing/iu

const PAGE_PREFIX = 'page'
const QUARANTINE_PREFIX = 'quarantine'

/**
 * The `PromptItem` contract, as `types.ts` declares it: every field except
 * `description` is required, and every value written by this builder must have
 * the declared type. `derivedFields` is the one field this builder adds.
 */
const ROW_FIELD_TYPES = {
  id: 'string',
  slug: 'string',
  modelKey: 'string',
  modelLabel: 'string',
  mediaType: 'mediaType',
  title: 'string',
  titleEn: 'string',
  description: 'string',
  descriptionEn: 'string',
  prompt: 'string',
  category: 'string',
  useCases: 'array',
  styles: 'array',
  tags: 'array',
  previewImageUrl: 'string',
  previewVideoUrl: 'string',
  posterUrl: 'string',
  aspectRatio: 'string',
  authorName: 'string',
  sourcePlatform: 'string',
  sourceUrl: 'string',
  publishedAt: 'string',
  viewCount: 'number',
  originalViewCount: 'number',
  // Optional in `types.ts` (`demoUrl?`), serialized unconditionally so the
  // byte layout never depends on which source produced the row: sources
  // without a demo link write `''`.
  demoUrl: 'string',
  derivedFields: 'array',
}

/** Field order inside a serialized row — fixed so output bytes are stable. */
const ROW_FIELD_ORDER = Object.keys(ROW_FIELD_TYPES)

function readFlag(argv, name) {
  const prefix = `--${name}=`
  const hit = argv.find((arg) => arg.startsWith(prefix))
  return hit === undefined ? undefined : hit.slice(prefix.length)
}

/** Every occurrence of a repeatable `--name=value` flag, in order. */
function readFlags(argv, name) {
  const prefix = `--${name}=`
  return argv
    .filter((arg) => arg.startsWith(prefix))
    .map((arg) => arg.slice(prefix.length))
    .filter((value) => value.length > 0)
}

function compareStrings(left, right) {
  if (left === right) return 0
  return left < right ? -1 : 1
}

/** Newest first, then by id, so two runs emit the same byte order. */
function compareRows(left, right) {
  const a = left.publishedAt
  const b = right.publishedAt
  if (a !== b) {
    if (!a) return 1
    if (!b) return -1
    return b < a ? -1 : 1
  }
  return compareStrings(left.id, right.id)
}

function dedupePreservingOrder(values) {
  return [...new Set(values)]
}

function isNonEmptyString(value) {
  return typeof value === 'string' && value.length > 0
}

/** `filters` is a map of dimension -> string[]; a missing dimension is empty. */
function facetValues(filters, key) {
  const value = filters[key]
  if (!Array.isArray(value)) return []
  return value.filter(isNonEmptyString)
}

/** Every value of every filter dimension, in sorted-key order. */
function allFilterValues(filters) {
  const values = []
  for (const key of Object.keys(filters).sort(compareStrings)) {
    values.push(...facetValues(filters, key))
  }
  return dedupePreservingOrder(values)
}

/**
 * Re-order a row to the fixed field order and attach its provenance.
 *
 * `demoUrl` is the only `ROW_FIELD_TYPES` field a source may legitimately
 * leave unset — it is optional on `PromptItem` — so it defaults to `''`
 * here instead of serializing `undefined` and failing the row contract.
 */
function shapeRow(row, derivedFields) {
  const ordered = {}
  for (const field of ROW_FIELD_ORDER) {
    if (field === 'derivedFields') {
      ordered[field] = derivedFields
    } else {
      ordered[field] = field === 'demoUrl' ? (row[field] ?? '') : row[field]
    }
  }
  return ordered
}

/**
 * Row provenance for a Topview/EvoLink row: the shared mapper fabricates a
 * `viewCount` when the crawl carries no counter (so "hottest" sorting is not a
 * no-op — see `demoViewCount`). A fabricated count is exactly the kind of
 * computed value `derivedFields` exists to expose.
 */
function sharedRowDerivedFields(row) {
  if (row.originalViewCount === 0 && row.viewCount > 0) return ['viewCount']
  return []
}

function loomiZoneId(row) {
  return String(row.zoneId || row.modelId || '')
}

function fromLoomi(raw, zone) {
  const slug = String(raw.id || 'item')
  const filters =
    raw.filters &&
    typeof raw.filters === 'object' &&
    !Array.isArray(raw.filters)
      ? raw.filters
      : {}
  const categoryPath = Array.isArray(raw.categoryPath) ? raw.categoryPath : []
  const sourceUrl = isHttpUrl(raw.sourceUrl) ? raw.sourceUrl : ''

  const derivedFields = []
  const authorName = deriveXHandle(sourceUrl)
  const publishedAt = derivePublishedAt(sourceUrl)
  if (authorName) derivedFields.push('authorName')
  if (publishedAt) derivedFields.push('publishedAt')

  return {
    id: `loomi:${zone.key}:${slug}`,
    slug,
    modelKey: zone.key,
    modelLabel: zone.label,
    mediaType: raw.mediaType === 'video' ? 'video' : 'image',
    title: truncate(raw.titleZh || raw.title || slug, CATALOG_LIMITS.title),
    // Loomi ships both languages on the row (`title` / `titleZh`); the English
    // field mirrors the Chinese one when only the latter exists.
    titleEn: truncate(raw.title || raw.titleZh || slug, CATALOG_LIMITS.title),
    description: truncate(
      raw.descriptionZh || raw.description,
      CATALOG_LIMITS.description
    ),
    descriptionEn: truncate(
      raw.description || raw.descriptionZh,
      CATALOG_LIMITS.description
    ),
    prompt: verbatimText(raw.prompt),
    category: String(categoryPath[0] || 'uncategorized'),
    useCases: dedupePreservingOrder(facetValues(filters, 'use_case')),
    styles: dedupePreservingOrder(facetValues(filters, 'style')),
    tags: allFilterValues(filters),
    previewImageUrl: isHttpUrl(raw.posterUrl) ? raw.posterUrl : '',
    previewVideoUrl: isHttpUrl(raw.videoUrl) ? raw.videoUrl : '',
    posterUrl: isHttpUrl(raw.posterUrl) ? raw.posterUrl : '',
    // Filled in by the shared `attachAspectRatios` pass, which sees the merged
    // Topview/EvoLink and Loomi ratio maps at once.
    aspectRatio: '',
    authorName,
    sourcePlatform: 'loomi',
    sourceUrl,
    publishedAt,
    viewCount: 0,
    originalViewCount: 0,
    // Loomi has no demo link; `''` keeps every row's key set identical.
    demoUrl: '',
    derivedFields,
  }
}

/** Loomi drops, counted by reason so the build report can explain them. */
function emptyLoomiDrops() {
  return {
    unparsable: 0,
    unknownZone: 0,
    prompt: 0,
    media: 0,
    duplicate: 0,
  }
}

async function collectLoomi(datasetDir, zonesById, log) {
  const itemsFile = path.join(datasetDir, LOOMI_ITEMS_FILE)
  const byZone = new Map()
  const dropped = emptyLoomiDrops()
  let scanned = 0
  let sourcePromptChars = 0
  let verbatimPrompts = 0
  const stream = readline.createInterface({
    input: (await fs.open(itemsFile, 'r')).createReadStream(),
    crlfDelay: Infinity,
  })

  for await (const line of stream) {
    if (!line.trim()) continue
    scanned += 1

    let raw
    try {
      raw = JSON.parse(line)
    } catch {
      dropped.unparsable += 1
      continue
    }

    const zone = zonesById.get(loomiZoneId(raw))
    if (!zone) {
      dropped.unknownZone += 1
      continue
    }
    const item = fromLoomi(raw, zone)

    // `prompt` is the product: it is copied verbatim, and this is the assertion
    // that says so rather than trusting `verbatimText` to stay verbatim.
    if (item.prompt !== verbatimText(raw.prompt)) {
      throw new Error(
        `loomi ${raw.id}: prompt was altered by the mapper (${item.prompt.length} chars vs ${verbatimText(raw.prompt).length})`
      )
    }
    verbatimPrompts += 1

    if (!readableRequirement(item)) {
      dropped.prompt += 1
      continue
    }
    if (!item.previewImageUrl && !item.previewVideoUrl) {
      dropped.media += 1
      continue
    }

    if (!byZone.has(zone.key)) {
      byZone.set(zone.key, { zone, items: [], seen: new Set() })
    }
    const bucket = byZone.get(zone.key)
    if (bucket.seen.has(item.slug)) {
      dropped.duplicate += 1
      continue
    }
    bucket.seen.add(item.slug)
    bucket.items.push(item)
    sourcePromptChars += verbatimText(raw.prompt).length
  }

  const mappedPromptChars = [...byZone.values()]
    .flatMap((bucket) => bucket.items)
    .reduce((sum, item) => sum + item.prompt.length, 0)
  if (mappedPromptChars !== sourcePromptChars) {
    throw new Error(
      `loomi prompt text lost characters: ${mappedPromptChars} mapped vs ${sourcePromptChars} read`
    )
  }
  log(
    `loomi prompt integrity: ${verbatimPrompts} rows mapped verbatim, ${mappedPromptChars} chars in / ${mappedPromptChars} chars out`
  )

  return {
    byZone,
    scanned,
    modelled: scanned - dropped.unknownZone,
    dropped,
    sourcePromptChars,
  }
}

/** Source id of a row, taken from its `source:modelKey:slug` primary key. */
function sourceOfRow(row) {
  const index = row.id.indexOf(':')
  return index === -1 ? row.id : row.id.slice(0, index)
}

/**
 * Tag a row as quarantined. These two fields exist on quarantined rows only —
 * a displayable row stays exactly the `PromptItem` shape plus `derivedFields`.
 */
function quarantineRow(row, reason) {
  return { ...row, posterUnavailable: true, posterUnavailableReason: reason }
}

/**
 * Read the `{ url: reason }` maps of media that is permanently gone.
 *
 * Each input is a file or a directory; a directory contributes only its
 * `*missing*.json` files (never every `*.json` — a directory that also holds a
 * poster-size map would otherwise mark every measured URL as dead). String
 * values are required: a non-string value means the caller pointed this at a
 * size map, which is a mistake worth failing loudly on rather than silently
 * quarantining the entire catalog.
 */
async function loadMissingMedia(inputs, log) {
  const missing = new Map()
  let files = 0

  for (const input of inputs) {
    const resolved = path.resolve(input)
    const stats = await fs.stat(resolved).catch(() => null)
    if (!stats) {
      throw new Error(`--missing=${input}: no such file or directory`)
    }

    let paths = []
    if (stats.isDirectory()) {
      const entries = await fs.readdir(resolved, { withFileTypes: true })
      paths = entries
        .filter(
          (entry) => entry.isFile() && MISSING_FILE_PATTERN.test(entry.name)
        )
        .map((entry) => path.join(resolved, entry.name))
        .sort(compareStrings)
      if (paths.length === 0) {
        log(`missing media: ${input} holds no *missing*.json file`)
      }
    } else {
      paths = [resolved]
    }

    for (const file of paths) {
      let parsed
      try {
        parsed = JSON.parse(await fs.readFile(file, 'utf8'))
      } catch (error) {
        throw new Error(
          `--missing=${input}: cannot read ${file} (${error.message}); if its producer is still writing, re-run once it has finished`
        )
      }
      if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
        throw new Error(
          `--missing=${input}: ${file} must be a JSON object of { url: reason }`
        )
      }

      const reasons = new Map()
      for (const [url, reason] of Object.entries(parsed)) {
        if (typeof reason !== 'string') {
          throw new Error(
            `--missing=${input}: ${file} is not a missing-media map (${url} maps to ${Array.isArray(reason) ? 'an array' : typeof reason})`
          )
        }
        const value = reason.trim() || QUARANTINE_REASON_FALLBACK
        missing.set(url, value)
        reasons.set(value, (reasons.get(value) ?? 0) + 1)
      }
      files += 1
      log(
        `missing media: ${file} (${Object.keys(parsed).length} entries; ${[
          ...reasons,
        ]
          .map(([reason, count]) => `${reason}=${count}`)
          .join(', ')})`
      )
    }
  }

  return { missing, files }
}

function pageFileName(prefix, page) {
  return `${prefix}-${String(page).padStart(4, '0')}.json`
}

function serializePage(zoneId, page, total, totalPages, items) {
  return JSON.stringify({
    zone: zoneId,
    page,
    pageSize: PAGE_SIZE,
    total,
    totalPages,
    items,
  })
}

function serializeManifest(manifest) {
  return `${JSON.stringify(manifest, null, 2)}\n`
}

/**
 * Write one page set (`page` or `quarantine`) of a zone and fold its byte
 * sizes into the run statistics. `pages == 0` writes nothing at all, so a zone
 * without quarantined rows has no quarantine files rather than an empty one.
 */
async function writePageSet(context) {
  const { outDir, zoneId, prefix, items, total, pages, stats, log } = context
  for (let page = 0; page < pages; page += 1) {
    const slice = items.slice(page * PAGE_SIZE, (page + 1) * PAGE_SIZE)
    const text = serializePage(zoneId, page, total, pages, slice)
    const file = path.join(outDir, zoneId, pageFileName(prefix, page))
    await fs.writeFile(file, text, 'utf8')
    const bytes = Buffer.byteLength(text)
    stats.totalBytes += bytes
    const isQuarantine = prefix === QUARANTINE_PREFIX
    const sizes = isQuarantine ? stats.quarantineSizes : stats.pageSizes
    sizes.push(bytes)
    const key = isQuarantine ? 'largestQuarantine' : 'largestPage'
    if (!stats[key] || bytes > stats[key].bytes) {
      stats[key] = { bytes, file: path.relative(outDir, file) }
    }
  }
  if (pages === 0 && items.length === 0) return
  log(
    `wrote ${path.join(zoneId, `${prefix}-*.json`)}: ${pages} page(s), ${items.length} rows`
  )
}

/**
 * Resolve the Loomi zone labels. The dataset's own collector already caches
 * the manifest under `raw/`; the network is only touched when that cache is
 * gone, and the fetched copy lands in the OS temp dir because the dataset repo
 * is read-only for this builder.
 */
async function loadLoomiManifest(datasetDir, explicitPath, log) {
  const candidates = explicitPath
    ? [explicitPath]
    : LOOMI_MANIFEST_CACHES.map((relative) => path.join(datasetDir, relative))

  for (const candidate of candidates) {
    try {
      const parsed = JSON.parse(await fs.readFile(candidate, 'utf8'))
      if (Array.isArray(parsed?.zones)) {
        log(`loomi manifest: ${candidate}`)
        return parsed
      }
      throw new Error('manifest has no `zones` array')
    } catch (error) {
      if (explicitPath) throw error
    }
  }

  const cachePath = path.join(
    os.tmpdir(),
    'omnimux-inspiration-cache',
    'loomi-templates-manifest.json'
  )
  try {
    const parsed = JSON.parse(await fs.readFile(cachePath, 'utf8'))
    if (Array.isArray(parsed?.zones)) {
      log(`loomi manifest: ${cachePath} (fetched cache)`)
      return parsed
    }
  } catch {
    // Fall through to the network.
  }

  log(`loomi manifest: no cache found, fetching ${LOOMI_MANIFEST_URL}`)
  const response = await fetch(LOOMI_MANIFEST_URL)
  if (!response.ok) {
    throw new Error(
      `cannot load the loomi manifest (${LOOMI_MANIFEST_URL}): HTTP ${response.status}`
    )
  }
  const parsed = await response.json()
  if (!Array.isArray(parsed?.zones)) {
    throw new Error('fetched loomi manifest has no `zones` array')
  }
  await fs.mkdir(path.dirname(cachePath), { recursive: true })
  await fs.writeFile(cachePath, JSON.stringify(parsed), 'utf8')
  log(`loomi manifest: cached at ${cachePath}`)
  return parsed
}

/**
 * Read the Loomi poster-size map. Optional by design: another job may still be
 * writing it, and a partial map is still better than none. Both shapes the
 * collector has used are accepted (`"W / H"` strings and `[width, height]`
 * pairs) via `normalizeAspectRatio`.
 */
async function loadLoomiRatios(file, log) {
  try {
    const parsed = await loadPosterRatios(file)
    log(`loomi ratios: ${file} (${Object.keys(parsed).length} entries)`)
    return { ratios: parsed, status: 'loaded' }
  } catch (error) {
    const status =
      error.code === 'ENOENT' || error.code === 'ENOTFOUND'
        ? 'missing'
        : 'unreadable'
    log(
      `loomi ratios: ${status} at ${file} — every Loomi poster will use the fallback '${FALLBACK_ASPECT_RATIO}' (${error.message})`
    )
    return { ratios: {}, status }
  }
}

/**
 * Prepare the output directory. An existing catalog is replaced wholesale; a
 * directory that this script did not write is never touched, so a mistyped
 * `--out` cannot delete unrelated files.
 */
async function prepareOutDir(outDir, log) {
  const entries = await fs.readdir(outDir).catch(() => null)
  if (entries === null) {
    await fs.mkdir(outDir, { recursive: true })
    return
  }
  if (entries.length === 0) return

  const manifestPath = path.join(outDir, 'manifest.json')
  let previous = null
  try {
    previous = JSON.parse(await fs.readFile(manifestPath, 'utf8'))
  } catch {
    previous = null
  }
  if (
    previous?.version !== MANIFEST_VERSION ||
    !Array.isArray(previous.zones)
  ) {
    throw new Error(
      `${outDir} is not empty and holds no catalog manifest; point --out at a new or previously built catalog directory`
    )
  }

  for (const zone of previous.zones) {
    const id = String(zone?.id ?? '')
    if (!ZONE_ID_PATTERN.test(id) || id === '.' || id === '..') {
      throw new Error(
        `refusing to clean ${outDir}: previous manifest lists an unsafe zone id ${JSON.stringify(id)}`
      )
    }
    await fs.rm(path.join(outDir, id), { recursive: true, force: true })
  }
  await fs.rm(manifestPath, { force: true })
  log(`cleaned the previous catalog in ${outDir}`)
}

/** Guard against ever writing the catalog into the repository. */
function assertOutsideRepo(outDir) {
  const resolved = path.resolve(outDir)
  const roots = new Set([REPO_ROOT])
  // A worktree lives at <main checkout>/.worktrees/<task>; both halves of the
  // repository are off limits, not just the one this script runs from.
  const marker = `${path.sep}.worktrees${path.sep}`
  const index = REPO_ROOT.indexOf(marker)
  if (index > 0) roots.add(REPO_ROOT.slice(0, index))

  for (const root of roots) {
    if (resolved === root || resolved.startsWith(`${root}${path.sep}`)) {
      throw new Error(
        `--out=${resolved} is inside the repository (${root}); the catalog is generated data and must not be committed`
      )
    }
  }
  return resolved
}

function resolveGeneratedAt(argv) {
  const explicit = readFlag(argv, 'generated-at')
  if (explicit !== undefined) {
    const stamp = new Date(explicit)
    if (Number.isNaN(stamp.getTime())) {
      throw new Error(`--generated-at=${explicit} is not a valid ISO-8601 date`)
    }
    return stamp.toISOString()
  }
  const epoch = process.env.SOURCE_DATE_EPOCH
  if (epoch) {
    const seconds = Number(epoch)
    if (!Number.isFinite(seconds)) {
      throw new Error(`SOURCE_DATE_EPOCH=${epoch} is not a number of seconds`)
    }
    return new Date(seconds * 1000).toISOString()
  }
  return new Date().toISOString()
}

/** A tiny deterministic PRNG, so the read-back sample is reproducible. */
function mulberry32(seed) {
  let state = seed >>> 0
  return () => {
    state = (state + 0x6d2b79f5) >>> 0
    let value = state
    value = Math.imul(value ^ (value >>> 15), value | 1)
    value ^= value + Math.imul(value ^ (value >>> 7), value | 61)
    return ((value ^ (value >>> 14)) >>> 0) / 4294967296
  }
}

function typeOfValue(value) {
  if (Array.isArray(value)) return 'array'
  if (value === null) return 'null'
  return typeof value
}

/**
 * Validate one written row against `ROW_FIELD_TYPES`. Returns the list of
 * problems; an empty list means the row satisfies the `PromptItem` contract.
 */
function rowProblems(row) {
  const problems = []
  for (const [field, expected] of Object.entries(ROW_FIELD_TYPES)) {
    if (!(field in row)) {
      problems.push(`missing ${field}`)
      continue
    }
    const actual = typeOfValue(row[field])
    if (expected === 'mediaType') {
      if (actual !== 'string' || !['image', 'video'].includes(row[field])) {
        problems.push(
          `${field}=${JSON.stringify(row[field])} is not a media type`
        )
      }
      continue
    }
    if (actual !== expected) {
      problems.push(`${field} is ${actual}, expected ${expected}`)
    }
  }
  for (const value of row.useCases ?? []) {
    if (typeof value !== 'string') problems.push('useCases holds a non-string')
  }
  for (const value of row.tags ?? []) {
    if (typeof value !== 'string') problems.push('tags holds a non-string')
  }
  for (const value of row.styles ?? []) {
    if (typeof value !== 'string') problems.push('styles holds a non-string')
  }
  return problems
}

/** Rows on `page` of a set holding `total` rows across `pages` pages. */
function expectedPageRows(total, page, pages) {
  if (page !== pages - 1) return PAGE_SIZE
  return total - page * PAGE_SIZE
}

/**
 * Read pages back off disk and check them the way a client would: page
 * arithmetic, the `PromptItem` contract on every row, the split itself (no
 * quarantined row in a display page, no live row in a quarantine page), and —
 * for the longest Loomi prompt in the catalog — that the written bytes are the
 * source text from beginning to end.
 */
async function verifyWrites(context) {
  const { outDir, zones, random, log, missing, longestLoomi } = context
  const problems = []
  const isDirty = (row) => missing.has(displayedMediaUrl(row))

  const samplePages = async (prefix) => {
    const all = zones.flatMap((zone) =>
      Array.from(
        {
          length:
            prefix === QUARANTINE_PREFIX ? zone.quarantinePages : zone.pages,
        },
        (_, page) => ({ zone, page, prefix })
      )
    )
    const sample = []
    const taken = new Set()
    const wanted = Math.min(prefix === QUARANTINE_PREFIX ? 3 : 5, all.length)
    while (sample.length < wanted) {
      const pick = Math.floor(random() * all.length)
      if (taken.has(pick)) continue
      taken.add(pick)
      sample.push(all[pick])
    }

    for (const { zone, page } of sample) {
      const quarantine = prefix === QUARANTINE_PREFIX
      const file = path.join(outDir, zone.id, pageFileName(prefix, page))
      const parsed = JSON.parse(await fs.readFile(file, 'utf8'))
      const expectedTotal = quarantine ? zone.quarantined : zone.count
      const expectedPages = quarantine ? zone.quarantinePages : zone.pages
      const expectedCount = expectedPageRows(expectedTotal, page, expectedPages)
      if (parsed.zone !== zone.id) problems.push(`${file}: zone=${parsed.zone}`)
      if (parsed.page !== page) problems.push(`${file}: page=${parsed.page}`)
      if (parsed.pageSize !== PAGE_SIZE) {
        problems.push(`${file}: pageSize=${parsed.pageSize}`)
      }
      if (parsed.total !== expectedTotal) {
        problems.push(
          `${file}: total=${parsed.total}, expected ${expectedTotal}`
        )
      }
      if (parsed.totalPages !== expectedPages) {
        problems.push(
          `${file}: totalPages=${parsed.totalPages}, expected ${expectedPages}`
        )
      }
      if (parsed.items.length !== expectedCount) {
        problems.push(
          `${file}: ${parsed.items.length} rows, expected ${expectedCount}`
        )
      }

      let shortestDirty = null
      for (const row of parsed.items) {
        if (row.modelKey !== zone.id) {
          problems.push(`${file}: row ${row.id} is in the wrong zone`)
        }
        for (const problem of rowProblems(row)) {
          problems.push(`${file}: row ${row.id} ${problem}`)
        }
        const dirty = isDirty(row)
        // Field-level quarantine rows (a non-empty `missing` list, e.g. a
        // partial prompt) are quarantined as-is: no URL-keyed reason, no
        // `posterUnavailable` fields. They bypass this whole block.
        const fieldQuarantined =
          Array.isArray(row.missing) && row.missing.length > 0
        if (quarantine) {
          if (fieldQuarantined) {
            if ('posterUnavailable' in row || 'posterUnavailableReason' in row) {
              problems.push(
                `${file}: field-quarantined row ${row.id} carries poster fields`
              )
            }
          } else {
            if (!dirty) {
              problems.push(`${file}: row ${row.id} is not in the missing map`)
            }
            if (row.posterUnavailable !== true) {
              problems.push(`${file}: row ${row.id} lacks posterUnavailable`)
            }
            if (typeof row.posterUnavailableReason !== 'string') {
              problems.push(`${file}: row ${row.id} lacks a reason string`)
            } else if (
              row.posterUnavailableReason !== missing.get(displayedMediaUrl(row))
            ) {
              problems.push(
                `${file}: row ${row.id} reason=${row.posterUnavailableReason} != map value`
              )
            }
            if (
              !shortestDirty ||
              row.prompt.length < shortestDirty.prompt.length
            ) {
              shortestDirty = row
            }
          }
        } else {
          if (fieldQuarantined) {
            problems.push(
              `${file}: row ${row.id} is field-quarantined but sits in a display page`
            )
          }
          if ('missing' in row) {
            problems.push(
              `${file}: displayable row ${row.id} carries a missing list`
            )
          }
          if (dirty) {
            problems.push(
              `${file}: row ${row.id} is quarantined but sits in a display page`
            )
          }
          if ('posterUnavailable' in row || 'posterUnavailableReason' in row) {
            problems.push(
              `${file}: displayable row ${row.id} carries quarantine fields`
            )
          }
        }
      }
      log(
        `read back ${path.relative(outDir, file)}: ${parsed.items.length} rows, all PromptItem fields present, ${quarantine ? 'all quarantined' : 'none quarantined'}`
      )
      if (shortestDirty) {
        log(
          `  shortest-prompt dirty row: ${JSON.stringify(shortestDirty, null, 2)}`
        )
      }
    }
    return sample
  }

  const pageSamples = await samplePages(PAGE_PREFIX)
  const quarantineSamples = await samplePages(QUARANTINE_PREFIX)
  if (missing.size > 0 && quarantineSamples.length === 0) {
    // Not an error: the supplied map may simply not match any row's media.
    log('no quarantine page to read back; the missing map matched no row')
  }

  if (longestLoomi) {
    const file = path.join(
      outDir,
      longestLoomi.zone,
      pageFileName(longestLoomi.prefix, longestLoomi.page)
    )
    const parsed = JSON.parse(await fs.readFile(file, 'utf8'))
    const written = parsed.items.find((row) => row.id === longestLoomi.id)
    if (!written) {
      problems.push(`${file}: ${longestLoomi.id} is not on its own page`)
    } else {
      const source = longestLoomi.prompt
      const intact =
        written.prompt === source &&
        written.prompt.length === source.length &&
        written.prompt.endsWith(source.slice(-40))
      if (!intact) {
        problems.push(`${file}: ${longestLoomi.id} prompt was truncated`)
      }
      log(
        `longest loomi prompt: ${longestLoomi.id} (${longestLoomi.prefix}) ${source.length} chars, ends ${JSON.stringify(source.slice(-40))} -> ${intact ? 'intact' : 'TRUNCATED'}`
      )
    }
  }

  if (problems.length > 0) {
    throw new Error(`catalog self-check failed:\n  ${problems.join('\n  ')}`)
  }
  log(
    `self-check passed: ${pageSamples.length} display pages and ${quarantineSamples.length} quarantine pages read back`
  )
}

async function main() {
  const argv = process.argv.slice(2)
  const known = new Set([
    'out',
    'generated-at',
    'ratios',
    'missing',
    'loomi-dir',
    'loomi-manifest',
    'skillry-file',
  ])
  for (const arg of argv) {
    const name = arg.startsWith('--') ? arg.slice(2).split('=')[0] : null
    if (!name || !known.has(name)) {
      throw new Error(
        `unknown argument ${arg}\nusage: bun scripts/build-inspiration-catalog.mjs [--out=DIR] [--generated-at=ISO] [--ratios=FILE] [--missing=FILE|DIR]... [--loomi-dir=DIR] [--loomi-manifest=FILE] [--skillry-file=FILE]`
      )
    }
  }

  const log = (message) => console.log(message)
  const outDir = assertOutsideRepo(readFlag(argv, 'out') ?? DEFAULT_OUT_DIR)
  const datasetDir = path.resolve(
    readFlag(argv, 'loomi-dir') ?? DEFAULT_LOOMI_DIR
  )
  const ratiosFile = path.resolve(
    readFlag(argv, 'ratios') ?? path.join(datasetDir, LOOMI_RATIOS_FILE)
  )
  const generatedAt = resolveGeneratedAt(argv)
  const skillryFile = path.resolve(
    readFlag(argv, 'skillry-file') ?? SKILLRY_VIDEOS_FILE
  )

  log(`catalog output: ${outDir}`)
  log(`generatedAt:    ${generatedAt}`)

  const loomiManifest = await loadLoomiManifest(
    datasetDir,
    readFlag(argv, 'loomi-manifest'),
    log
  )
  const loomiRatios = await loadLoomiRatios(ratiosFile, log)
  const checkedInRatios = await loadPosterRatios(POSTER_RATIOS_FILE)
  const ratios = { ...checkedInRatios, ...loomiRatios.ratios }
  const { missing, files: missingFiles } = await loadMissingMedia(
    readFlags(argv, 'missing'),
    log
  )
  if (missing.size === 0) {
    log('missing media: none supplied, so nothing will be quarantined')
  } else {
    log(
      `missing media: ${missing.size} URLs from ${missingFiles} file(s) will be quarantined`
    )
  }

  // Zone list: the model sections already rendered by the frontend, in their
  // display order, then any Loomi zone that does not map onto one of them.
  const sections = MODEL_SECTIONS.map((section) => ({
    key: section.key,
    label: section.label,
    section,
  }))
  const loomiZones = new Map()
  for (const zone of loomiManifest.zones) {
    const id = String(zone?.id ?? '')
    if (!ZONE_ID_PATTERN.test(id)) {
      log(`skipping loomi zone ${JSON.stringify(id)}: unusable zone id`)
      continue
    }
    const section = sections.find((entry) => entry.key === id)
    loomiZones.set(id, {
      key: id,
      label:
        zone.labelEn ||
        zone.labelZh ||
        section?.label ||
        String(zone.modelId || id),
      section,
    })
  }
  const zones = [
    ...sections,
    ...[...loomiZones.values()].filter((zone) => !zone.section),
  ]

  const topview = await collectTopview({
    perModel: null,
    maxScanLines: null,
    limits: CATALOG_LIMITS,
  })
  const evolink = await collectEvolink({
    perModel: null,
    limits: CATALOG_LIMITS,
    requireMedia: true,
  })
  const loomi = await collectLoomi(datasetDir, loomiZones, log)
  const skillry = await collectSkillry({ file: skillryFile })

  // How much of the Loomi poster set the (possibly still growing) ratio map
  // actually covers — the number that decides whether a re-run is worth it.
  let loomiPosters = 0
  let loomiMeasured = 0
  for (const bucket of loomi.byZone.values()) {
    for (const item of bucket.items) {
      loomiPosters += 1
      if (normalizeAspectRatio(ratios[item.posterUrl])) loomiMeasured += 1
    }
  }
  const byZone = new Map()
  const addRows = (key, label, rows) => {
    if (rows.length === 0) return
    if (!byZone.has(key)) byZone.set(key, { key, label, items: [] })
    byZone.get(key).items.push(...rows)
  }

  let topviewRows = 0
  for (const section of sections) {
    const bucket = topview.wanted.get(section.key)
    const rows = (bucket?.items ?? []).map((item) =>
      shapeRow(item, sharedRowDerivedFields(item))
    )
    topviewRows += rows.length
    addRows(section.key, section.label, rows)
  }

  let evolinkRows = 0
  for (const section of sections) {
    const key = section.section.evolink
    if (!key) continue
    const bucket = evolink.wanted.get(key)
    const rows = (bucket?.items ?? []).map((item) =>
      shapeRow(item, sharedRowDerivedFields(item))
    )
    evolinkRows += rows.length
    addRows(section.key, section.label, rows)
  }

  let loomiRows = 0
  for (const bucket of loomi.byZone.values()) {
    const zone = loomiZones.get(bucket.zone.key) ?? bucket.zone
    loomiRows += bucket.items.length
    addRows(bucket.zone.key, zone.label, bucket.items)
  }

  for (const section of sections) {
    if (section.section.skillry !== true) continue
    const bucket = skillry.wanted.get(section.key)
    const rows = (bucket?.items ?? []).map((item) => {
      // The row's own list, verbatim: Skillry derives a different set of
      // fields than the community feeds.
      const shaped = shapeRow(item, item.derivedFields)
      // `shapeRow` drops every key outside ROW_FIELD_ORDER, so the
      // field-level quarantine marker has to be re-attached by hand.
      shaped.missing = item.missing
      return shaped
    })
    addRows(section.key, section.label, rows)
  }

  for (const bucket of byZone.values()) {
    bucket.items.sort(compareRows)
  }

  const ratioCoverage = attachAspectRatios(
    new Map([...byZone].map(([key, bucket]) => [key, bucket.items])),
    ratios
  )

  // Split every zone into the displayable set and the quarantined set. Both
  // inherit the order established above, so each set stays stable on its own.
  const zoneEntries = zones
    .filter((zone) => byZone.has(zone.key))
    .map((zone) => {
      const all = byZone.get(zone.key).items
      const items = []
      const quarantine = []
      for (const item of all) {
        // Field-level quarantine (e.g. a Skillry row whose prompt is
        // partial): the media is intact, the row itself is incomplete, so it
        // lands in the quarantine set as-is with no URL-keyed reason.
        if (Array.isArray(item.missing) && item.missing.length > 0) {
          quarantine.push(item)
          continue
        }
        const reason = missing.get(displayedMediaUrl(item))
        if (reason === undefined) items.push(item)
        else quarantine.push(quarantineRow(item, reason))
      }
      if (items.length === 0) {
        throw new Error(
          `zone ${zone.key} would publish 0 displayable rows out of ${all.length}`
        )
      }
      // A model section is overwhelmingly one media kind, and the zone has to
      // declare exactly one. Take the dominant kind from the displayable rows
      // and report any minority instead of dropping it: the crawl does contain
      // the odd row whose `isVideo` disagrees with its section (one
      // `gpt-image-2` row is flagged video), and rewriting the row's own flag
      // would be a lie.
      const mediaCounts = new Map()
      for (const item of items) {
        mediaCounts.set(
          item.mediaType,
          (mediaCounts.get(item.mediaType) ?? 0) + 1
        )
      }
      const ranked = [...mediaCounts].sort(
        (left, right) => right[1] - left[1] || compareStrings(left[0], right[0])
      )
      const mediaType = ranked[0][0]
      const minority = ranked.slice(1)
      if (minority.length > 0) {
        log(
          `zone ${zone.key} declares ${mediaType} but ${items.length} rows include ${minority
            .map(([kind, count]) => `${count} ${kind}`)
            .join(', ')} (kept as-is)`
        )
      }
      const ids = new Set(items.map((item) => item.id))
      if (ids.size !== items.length) {
        throw new Error(
          `zone ${zone.key} has ${items.length - ids.size} duplicate row ids`
        )
      }
      // Facets come from the displayable rows only: a chip no visible row
      // matches would be dead UI.
      const facets = zoneFacets(items)
      if (quarantine.length > 0) {
        log(
          `zone ${zone.key}: ${quarantine.length} of ${all.length} rows quarantined (media permanently gone)`
        )
      }
      return {
        id: zone.key,
        label: zone.label,
        mediaType,
        count: items.length,
        pages: Math.ceil(items.length / PAGE_SIZE),
        quarantined: quarantine.length,
        quarantinePages: Math.ceil(quarantine.length / PAGE_SIZE),
        categories: facets.categories,
        styles: facets.styles,
        subjects: facets.subjects,
        useCases: facets.useCases,
        items,
        quarantine,
      }
    })

  // Per-source accounting. Every row a source contributed to a published
  // section must land in exactly one of three buckets, so the three numbers
  // reconcile against the rows read for that source:
  //     modelled = displayable (rows) + quarantined + dropped
  const perSource = new Map()
  const bucketOf = (id) => {
    if (!perSource.has(id)) {
      perSource.set(id, { rows: 0, quarantined: 0 })
    }
    return perSource.get(id)
  }
  for (const zone of zoneEntries) {
    for (const row of zone.items) bucketOf(sourceOfRow(row)).rows += 1
    for (const row of zone.quarantine) {
      bucketOf(sourceOfRow(row)).quarantined += 1
    }
  }
  const sumDrops = (dropped) =>
    Object.values(dropped).reduce((sum, count) => sum + count, 0)
  const sources = [
    {
      id: 'topview',
      modelled: topview.modelled,
      dropped: sumDrops(topview.dropped),
    },
    {
      id: 'evolink',
      modelled: evolink.modelled,
      dropped: sumDrops(evolink.dropped),
    },
    { id: 'loomi', modelled: loomi.modelled, dropped: sumDrops(loomi.dropped) },
    {
      id: 'skillry',
      modelled: skillry.modelled,
      dropped: sumDrops(skillry.dropped),
    },
  ].map((source) => {
    const bucket = bucketOf(source.id)
    if (source.modelled !== bucket.rows + bucket.quarantined + source.dropped) {
      throw new Error(
        `${source.id}: ${source.modelled} modelled rows != ${bucket.rows} displayable + ${bucket.quarantined} quarantined + ${source.dropped} dropped`
      )
    }
    return {
      id: source.id,
      rows: bucket.rows,
      quarantined: bucket.quarantined,
      dropped: source.dropped,
    }
  })

  const total = zoneEntries.reduce((sum, zone) => sum + zone.count, 0)
  const quarantinedTotal = zoneEntries.reduce(
    (sum, zone) => sum + zone.quarantined,
    0
  )
  const sourceTotal = sources.reduce((sum, source) => sum + source.rows, 0)
  if (total !== sourceTotal) {
    throw new Error(
      `row accounting mismatch: ${total} displayable rows in zones vs ${sourceTotal} from the sources`
    )
  }
  const sourceQuarantined = sources.reduce(
    (sum, source) => sum + source.quarantined,
    0
  )
  if (quarantinedTotal !== sourceQuarantined) {
    throw new Error(
      `quarantine accounting mismatch: ${quarantinedTotal} quarantined rows in zones vs ${sourceQuarantined} from the sources`
    )
  }
  for (const zone of zoneEntries) {
    if (zone.pages !== Math.ceil(zone.count / PAGE_SIZE)) {
      throw new Error(`zone ${zone.id}: page arithmetic is off`)
    }
    if (zone.items.length - (zone.pages - 1) * PAGE_SIZE === 0) {
      throw new Error(
        `zone ${zone.id}: the last displayable page would be empty`
      )
    }
    if (zone.quarantinePages !== Math.ceil(zone.quarantined / PAGE_SIZE)) {
      throw new Error(`zone ${zone.id}: quarantine page arithmetic is off`)
    }
    if (
      zone.quarantined > 0 &&
      zone.quarantine.length - (zone.quarantinePages - 1) * PAGE_SIZE === 0
    ) {
      throw new Error(
        `zone ${zone.id}: the last quarantine page would be empty`
      )
    }
  }

  const missingSections = sections
    .filter((section) => !byZone.has(section.key))
    .map((section) => section.key)
  if (missingSections.length > 0) {
    log(`zones with no rows: ${missingSections.join(', ')}`)
  }

  await prepareOutDir(outDir, log)

  const stats = {
    pageSizes: [],
    quarantineSizes: [],
    totalBytes: 0,
    largestPage: null,
    largestQuarantine: null,
  }
  for (const zone of zoneEntries) {
    await fs.mkdir(path.join(outDir, zone.id), { recursive: true })
    await writePageSet({
      outDir,
      zoneId: zone.id,
      prefix: PAGE_PREFIX,
      items: zone.items,
      total: zone.count,
      pages: zone.pages,
      stats,
      log,
    })
    await writePageSet({
      outDir,
      zoneId: zone.id,
      prefix: QUARANTINE_PREFIX,
      items: zone.quarantine,
      total: zone.quarantined,
      pages: zone.quarantinePages,
      stats,
      log,
    })
  }

  const manifest = {
    version: MANIFEST_VERSION,
    generatedAt,
    pageSize: PAGE_SIZE,
    total,
    quarantinedTotal,
    sources,
    zones: zoneEntries.map((zone) => ({
      id: zone.id,
      label: zone.label,
      mediaType: zone.mediaType,
      count: zone.count,
      pages: zone.pages,
      quarantined: zone.quarantined,
      quarantinePages: zone.quarantinePages,
      categories: zone.categories,
      styles: zone.styles,
      subjects: zone.subjects,
      useCases: zone.useCases,
    })),
  }
  const manifestText = serializeManifest(manifest)
  await fs.writeFile(path.join(outDir, 'manifest.json'), manifestText, 'utf8')
  stats.totalBytes += Buffer.byteLength(manifestText)
  stats.manifestBytes = Buffer.byteLength(manifestText)

  // The longest Loomi prompt is the strongest available proof that a catalog
  // row is not a shortened copy: whichever set it landed in, it must survive
  // the page write intact.
  let longestLoomi = null
  for (const zone of zoneEntries) {
    for (const [prefix, set] of [
      [PAGE_PREFIX, zone.items],
      [QUARANTINE_PREFIX, zone.quarantine],
    ]) {
      for (const [index, item] of set.entries()) {
        if (!item.id.startsWith('loomi:')) continue
        if (longestLoomi && item.prompt.length <= longestLoomi.prompt.length) {
          continue
        }
        longestLoomi = {
          id: item.id,
          zone: zone.id,
          prefix,
          page: Math.floor(index / PAGE_SIZE),
          prompt: item.prompt,
        }
      }
    }
  }
  const random = mulberry32(
    zoneEntries.reduce(
      (seed, zone) =>
        (seed * 31 + zone.count + zone.pages * 7 + zone.quarantined * 3) >>> 0,
      0x5eed
    )
  )
  await verifyWrites({
    outDir,
    zones: zoneEntries,
    random,
    log,
    missing,
    longestLoomi,
  })

  const pageSizes = stats.pageSizes
  const quarantineSizes = stats.quarantineSizes
  pageSizes.sort((left, right) => left - right)
  quarantineSizes.sort((left, right) => left - right)
  const median = (sizes) => sizes[Math.floor(sizes.length / 2)]

  // aspectRatio fallbacks over the displayable set: a quarantined row will
  // never render, so its reserved box does not matter to the UI.
  let fallbackDisplayable = 0
  const fallbackByZone = new Map()
  for (const zone of zoneEntries) {
    let missingRatio = 0
    for (const row of zone.items) {
      if (row.aspectRatio === FALLBACK_ASPECT_RATIO) missingRatio += 1
    }
    if (missingRatio > 0) fallbackByZone.set(zone.id, missingRatio)
    fallbackDisplayable += missingRatio
  }

  log('')
  log(
    `wrote ${total} displayable rows + ${quarantinedTotal} quarantined rows in ${zoneEntries.length} zones -> ${outDir}`
  )
  for (const source of sources) {
    log(
      `  source ${source.id}: ${source.rows} displayable + ${source.quarantined} quarantined + ${source.dropped} dropped = ${source.rows + source.quarantined + source.dropped} modelled rows`
    )
  }
  for (const zone of zoneEntries) {
    log(
      `  zone ${zone.id} (${zone.label}, ${zone.mediaType}): ${zone.count} displayable rows, ${zone.pages} pages | ${zone.quarantined} quarantined, ${zone.quarantinePages} pages`
    )
  }
  log(
    `displayable pages: ${pageSizes.length} files, min ${pageSizes[0]} B, median ${median(pageSizes)} B, max ${stats.largestPage.bytes} B (${stats.largestPage.file})`
  )
  if (quarantineSizes.length > 0) {
    log(
      `quarantine pages: ${quarantineSizes.length} files, min ${quarantineSizes[0]} B, median ${median(quarantineSizes)} B, max ${stats.largestQuarantine.bytes} B (${stats.largestQuarantine.file})`
    )
  } else {
    log('quarantine pages: none')
  }
  log(
    `total output: ${stats.totalBytes} bytes (${(stats.totalBytes / 1024 / 1024).toFixed(1)} MiB, manifest ${stats.manifestBytes} B)`
  )
  log(
    `aspectRatio fallbacks: ${fallbackDisplayable} of ${total} displayable rows (${ratioCoverage.total} of ${total + quarantinedTotal} including quarantine; ratio maps: ${Object.keys(ratios).length} entries)`
  )
  for (const [zone, count] of fallbackByZone) {
    const zoneTotal = zoneEntries.find((entry) => entry.id === zone).count
    log(
      `  ${zone}: ${count} of ${zoneTotal} displayable rows used '${FALLBACK_ASPECT_RATIO}'`
    )
  }
  log(
    `dropped topview: ${JSON.stringify(topview.dropped)} (scanned ${topview.scanned}, modelled ${topview.modelled})`
  )
  log(
    `dropped evolink: ${JSON.stringify(evolink.dropped)} (scanned ${evolink.scanned}, modelled ${evolink.modelled})`
  )
  log(
    `dropped loomi: ${JSON.stringify(loomi.dropped)} (scanned ${loomi.scanned}, modelled ${loomi.modelled})`
  )
  if (loomiRatios.status !== 'loaded') {
    log(
      `NOTE: the loomi poster-size map was ${loomiRatios.status}; re-run this build once ${ratiosFile} is complete to pick up real aspect ratios.`
    )
  }
  log(
    `loomi aspectRatio coverage: ${loomiMeasured} of ${loomiPosters} posters measured (${((loomiMeasured / Math.max(loomiPosters, 1)) * 100).toFixed(1)}%)`
  )
  log(`topview feed: ${TOPVIEW_FEED}`)
  log(`evolink feed: ${EVOLINK_FEED}`)
}

main().catch((error) => {
  console.error(error)
  process.exitCode = 1
})
