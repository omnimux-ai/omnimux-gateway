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
 * Shared source layer for the Inspiration Community prompt datasets.
 *
 * Both builders read the same public crawls kept OUTSIDE this repo and map
 * them onto `PromptItem` (see `src/features/inspiration-community/types.ts`):
 *
 *   - `build-inspiration-mock.mjs`   -> a small typed TS fixture in the bundle
 *   - `build-inspiration-catalog.mjs` -> the static paginated JSON catalog
 *
 * Only the row mapping lives here. Everything that differs between the two
 * consumers (sample budget, prompt truncation, where the bytes land) is a
 * caller option, so the two can never drift apart on field semantics.
 *
 * Datasets (read-only, never written), relative to `DATASETS_ROOT`:
 *   - Topview: topview-prompt-library/data/community-feed.jsonl
 *   - EvoLink: evolink-prompt-library/data/community-feed-zh.jsonl
 *   - Loomi:   loomi-prompt-library/data/items.jsonl
 *   - Skillry: Github/awesome-opus5-5-videos/data/videos.json
 *
 * Poster aspect ratios are looked up per media URL:
 *   - Topview/EvoLink: checked-in `scripts/data/poster-ratios.json`
 *     ({ [posterUrl]: "1200 / 900" }), measured once off the image headers so
 *     the masonry can reserve each card's box before the image loads.
 *   - Loomi:           live-collected `raw/ratios.json`, same idea, written by
 *     the collector in that dataset repo.
 * A URL missing from a map (unmeasurable asset) falls back to
 * `FALLBACK_ASPECT_RATIO`.
 *
 * `limits` is how a consumer picks its fidelity budget. `MOCK_LIMITS` is the
 * default because the mock's artifacts are byte-checked into the repository:
 * its output must stay reproducible character for character. The catalog
 * passes `CATALOG_LIMITS`, where `null` means "no cap" — a catalog row carries
 * the source text as crawled, most importantly a never-truncated `prompt`.
 */
import fs from 'node:fs/promises'
import path from 'node:path'
import readline from 'node:readline'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

/** Absolute path of `web/`. */
export const WEB_ROOT = path.resolve(__dirname, '..', '..')
/** Absolute path of the repository root (the parent of `web/`). */
export const REPO_ROOT = path.resolve(WEB_ROOT, '..')

/**
 * Directory that holds the prompt-library checkouts. They live next to this
 * repository by default; `INSPIRATION_DATASETS_ROOT` points elsewhere.
 */
export const DATASETS_ROOT =
  process.env.INSPIRATION_DATASETS_ROOT || path.resolve(REPO_ROOT, '..')

export const TOPVIEW_FEED = path.join(
  DATASETS_ROOT,
  'topview-prompt-library/data/community-feed.jsonl'
)
export const EVOLINK_FEED = path.join(
  DATASETS_ROOT,
  'evolink-prompt-library/data/community-feed-zh.jsonl'
)
/**
 * Skillry community feed for the Claude zone: one JSON array of video rows.
 * Read-only; the path can be overridden with `SKILLRY_VIDEOS_FILE` for tests.
 */
export const SKILLRY_VIDEOS_FILE =
  process.env.SKILLRY_VIDEOS_FILE ||
  path.join(DATASETS_ROOT, 'Github/awesome-opus5-5-videos/data/videos.json')

/**
 * Checked-in URL -> `"<width> / <height>"` map of the poster pixel size, read
 * once per run. The value is the RAW pixel pair (e.g. `"9 / 16"`), never the
 * reduced display ratio, so the masonry adopts the media's own aspect ratio.
 */
export const POSTER_RATIOS_FILE = path.join(
  WEB_ROOT,
  'scripts/data/poster-ratios.json'
)

/** Ratio used when a media URL has no measured entry in the lookup. */
export const FALLBACK_ASPECT_RATIO = '16 / 9'

export const PROMPT_MAX_CHARS = 400

/**
 * Cap for the item summary. The card clamps it to three lines, so a longer cut
 * only wastes fixture weight. The source descriptions run 37-240 characters.
 */
export const DESCRIPTION_MAX_CHARS = 300

export const MAX_SCAN_LINES = 120_000

/** Model sections rendered on /inspiration, in display order. */
export const MODEL_SECTIONS = [
  {
    key: 'seedance-2.5',
    label: 'Seedance 2.5',
    topview: 'seedance-2.5',
    evolink: 'seedance-2-5',
  },
  {
    key: 'seedance-2.0',
    label: 'Seedance 2.0',
    topview: 'seedance-2.0',
    evolink: 'seedance-2-0',
  },
  {
    key: 'wan-3.0',
    label: 'Wan 3.0',
    topview: 'wan-3.0',
  },
  {
    key: 'gpt-image-2',
    label: 'GPT Image 2',
    topview: 'gpt-image-2',
    evolink: 'gpt-image-2',
  },
  {
    key: 'gpt-image-2.5',
    label: 'GPT Image 2.5',
    topview: 'gpt-image-2.5',
  },
  {
    key: 'nano-banana-pro',
    label: 'Nano Banana Pro',
    topview: 'nano-banana-pro',
  },
  {
    key: 'grok-imagine',
    label: 'Grok Imagine',
    topview: 'grok-imagine',
  },
  {
    key: 'claude-opus-5.5',
    // Zone/tab label stays the product-facing zone name; `modelLabel` is the
    // model badge printed on each row and must name the actual model.
    label: 'Claude zone',
    modelLabel: 'Claude Opus 5.5',
    skillry: true,
  },
]

/**
 * Fidelity budget for the checked-in demo fixture: short enough that the
 * repository never carries a large volume of third-party long-form text, and
 * narrow enough that a card renders at most a handful of chips.
 */
export const MOCK_LIMITS = Object.freeze({
  title: 120,
  description: DESCRIPTION_MAX_CHARS,
  prompt: PROMPT_MAX_CHARS,
  useCases: 4,
  styles: 4,
  tags: 6,
})

/**
 * Fidelity budget for the static catalog: every cap is `null`, i.e. the
 * source text and the full facet lists are carried through unchanged.
 * `prompt` in particular is the product and MUST NOT be truncated.
 */
export const CATALOG_LIMITS = Object.freeze({
  title: null,
  description: null,
  prompt: null,
  useCases: null,
  styles: null,
  tags: null,
})

/**
 * Collapse whitespace and cut to `max` characters, appending an ellipsis when
 * text was dropped. `max == null` keeps the whole string (still whitespace
 * collapsed) so a long title stays a single line.
 */
export function truncate(text, max) {
  const value = String(text ?? '')
    .replaceAll(/\s+/gu, ' ')
    .trim()
  if (max == null || value.length <= max) return value
  return `${value.slice(0, max - 1).trimEnd()}…`
}

/**
 * Collapse whitespace and cut at a word boundary under `max` characters,
 * appending an ellipsis when text was dropped. Unlike `truncate` — which cuts
 * mid-word at an exact index — this backs the cut point off to the last space
 * so a title never ends on a broken word. A first word longer than `max` is
 * still hard-cut so the cap holds.
 */
export function truncateWords(text, max) {
  const value = String(text ?? '')
    .replaceAll(/\s+/gu, ' ')
    .trim()
  if (max == null || value.length <= max) return value
  const cut = value.slice(0, max - 1)
  const boundary = cut.lastIndexOf(' ')
  const head = (boundary > 0 ? cut.slice(0, boundary) : cut).trimEnd()
  return `${head}…`
}

/** Byte-faithful pass-through: no trimming, no whitespace collapsing. */
export function verbatimText(value) {
  return typeof value === 'string' ? value : ''
}

/**
 * Body text under `limits.maxChars`: verbatim when unlimited (the catalog
 * case, where the prompt must end where the source ends), whitespace-collapsed
 * and cut otherwise.
 */
function shapeBody(value, maxChars) {
  return maxChars == null ? verbatimText(value) : truncate(value, maxChars)
}

/** Facet list under `limits[name]`: copied whole when unlimited. */
function shapeList(value, maxEntries) {
  if (!Array.isArray(value)) return []
  return maxEntries == null ? [...value] : value.slice(0, maxEntries)
}

/** `x.com/<handle>/status/<id>` and `twitter.com` equivalents, nothing else. */
export const X_STATUS_PATTERN =
  /^https?:\/\/(?:www\.)?(?:x|twitter)\.com\/([A-Za-z0-9_]{1,15})\/status\/(\d+)/u

/** Twitter's snowflake epoch (2010-11-04T01:42:54.657Z) in milliseconds. */
export const SNOWFLAKE_EPOCH_MS = 1288834974657n

/** Handle of an `x.com`/`twitter.com` status URL, `''` for anything else. */
export function deriveXHandle(sourceUrl) {
  if (!isHttpUrl(sourceUrl)) return ''
  const match = X_STATUS_PATTERN.exec(sourceUrl)
  if (!match || match[1] === 'i') return ''
  return match[1]
}

/**
 * Publish time of an `x.com` status URL, from the snowflake id that encodes
 * it. Formatted as a naive UTC `YYYY-MM-DDTHH:mm:ss`, the same shape the
 * Topview crawl uses, so the two can sort against each other.
 */
export function derivePublishedAt(sourceUrl) {
  if (!isHttpUrl(sourceUrl)) return ''
  const match = X_STATUS_PATTERN.exec(sourceUrl)
  if (!match) return ''
  const milliseconds = (BigInt(match[2]) >> 22n) + SNOWFLAKE_EPOCH_MS
  const stamp = new Date(Number(milliseconds))
  if (Number.isNaN(stamp.getTime())) return ''
  return stamp.toISOString().slice(0, 19)
}

/** Deterministic 32-bit hash — used for stable demo view counts only. */
export function hash32(input) {
  let hash = 2166136261
  for (let index = 0; index < input.length; index += 1) {
    hash ^= input.charCodeAt(index)
    hash = Math.imul(hash, 16777619)
  }
  return hash >>> 0
}

export function isHttpUrl(value) {
  return typeof value === 'string' && /^https?:\/\//u.test(value)
}

export function mediaTypeOf(raw) {
  if (raw.taskType === 'video_generation' || raw.isVideo === true) {
    return 'video'
  }
  return 'image'
}

/**
 * The public Topview crawl does not expose engagement counts (every row reads
 * `viewCount: 0`), so "hottest" sorting would be a no-op without a stand-in.
 * Derive a stable pseudo count from the item id instead of inventing per-item
 * numbers by hand.
 */
export function demoViewCount(id, sourceViewCount) {
  if (typeof sourceViewCount === 'number' && sourceViewCount > 0) {
    return Math.round(sourceViewCount)
  }
  return 420 + (hash32(id) % 986_000)
}

/** A row is only usable when it carries a prompt a human could retype. */
export function readableRequirement(item) {
  return (
    item &&
    typeof item === 'object' &&
    typeof item.prompt === 'string' &&
    item.prompt.length > 40
  )
}

export function fromTopview(raw, section, limits = MOCK_LIMITS) {
  const slug = raw.slug || raw.candidateId || 'item'
  const id = `topview:${section.key}:${slug}`
  const mediaType = mediaTypeOf(raw)
  const videoUrl = isHttpUrl(raw.previewVideoUrl) ? raw.previewVideoUrl : ''
  const imageUrl = isHttpUrl(raw.previewImageUrl) ? raw.previewImageUrl : ''
  const coverUrl = isHttpUrl(raw.coverUrl) ? raw.coverUrl : ''
  const posterUrl = isHttpUrl(raw.previewPosterUrl)
    ? raw.previewPosterUrl
    : coverUrl

  return {
    id,
    slug,
    modelKey: section.key,
    modelLabel: section.modelLabel || section.label,
    mediaType,
    title: truncate(raw.titleOriginal || raw.titleEn || slug, limits.title),
    // English counterpart of `title`. It falls back to the Chinese original so
    // the field is never empty: the UI picks by locale, and a row that exists in
    // only one language must still render in the other rather than go blank.
    titleEn: truncate(raw.titleEn || raw.titleOriginal || slug, limits.title),
    // The crawl carries a human-written summary per item; it is NOT derived
    // from the prompt body, and stays empty when the row has none.
    description: truncate(
      raw.descriptionOriginal || raw.descriptionEn,
      limits.description
    ),
    descriptionEn: truncate(
      raw.descriptionEn || raw.descriptionOriginal,
      limits.description
    ),
    prompt: shapeBody(raw.promptOriginal || raw.promptEn, limits.prompt),
    category: raw.categoryPrimary || 'uncategorized',
    useCases: shapeList(raw.facetUseCases, limits.useCases),
    styles: shapeList(raw.facetStyles, limits.styles),
    tags: shapeList(raw.tagsSource, limits.tags),
    previewVideoUrl: videoUrl,
    previewImageUrl: imageUrl || coverUrl,
    posterUrl: mediaType === 'video' ? posterUrl : imageUrl || coverUrl,
    authorName: truncate(raw.sourceAuthorName || 'Unknown', 60),
    sourcePlatform: raw.sourcePlatform || 'topview',
    sourceUrl: isHttpUrl(raw.sourceUrl) ? raw.sourceUrl : '',
    publishedAt: String(raw.publishedAt || '').slice(0, 19),
    viewCount: demoViewCount(id, raw.viewCount),
    originalViewCount:
      typeof raw.viewCount === 'number' ? Math.round(raw.viewCount) : 0,
  }
}

export function evolinkMediaType(raw) {
  if (raw.mediaType === 'video') return 'video'
  return 'image'
}

export function fromEvolink(raw, section, limits = MOCK_LIMITS) {
  const slug = raw.slug || raw.id || 'item'
  const id = `evolink:${section.key}:${slug}`
  const mediaType = evolinkMediaType(raw)
  const mediaUrl = isHttpUrl(raw.mediaUrl) ? raw.mediaUrl : ''
  const posterUrl = isHttpUrl(raw.mediaPoster) ? raw.mediaPoster : ''

  return {
    id,
    slug,
    modelKey: section.key,
    modelLabel: section.modelLabel || section.label,
    mediaType,
    title: truncate(raw.title || slug, limits.title),
    // This feed is the Chinese locale (`community-feed-zh.jsonl`) and carries no
    // English counterpart, so the English field mirrors the Chinese one rather
    // than leaving the row unreadable outside Chinese.
    titleEn: truncate(raw.title || slug, limits.title),
    description: truncate(raw.summary, limits.description),
    descriptionEn: truncate(raw.summary, limits.description),
    prompt: shapeBody(raw.prompt, limits.prompt),
    category: (raw.category || 'uncategorized')
      .toLowerCase()
      .replaceAll(' & ', '-')
      .replaceAll(/[\s_]+/gu, '-'),
    useCases: [],
    styles: [],
    tags: shapeList(raw.tags, limits.tags),
    previewVideoUrl: mediaType === 'video' ? mediaUrl : '',
    previewImageUrl: mediaType === 'image' ? mediaUrl : posterUrl,
    posterUrl: posterUrl || mediaUrl,
    authorName: truncate(raw.author || 'Unknown', 60),
    sourcePlatform: 'evolink',
    sourceUrl: isHttpUrl(raw.sourceUrl) ? raw.sourceUrl : '',
    publishedAt: '',
    viewCount: demoViewCount(id, 0),
    originalViewCount: 0,
  }
}

function emptyDrops() {
  return {
    unparsable: 0,
    unpublished: 0,
    capped: 0,
    prompt: 0,
    media: 0,
    duplicate: 0,
  }
}

/**
 * Scan the Topview community feed for the models behind `MODEL_SECTIONS`.
 *
 * `perModel` caps how many items a section keeps (source scan order, so the
 * sample is stable); `null` keeps every row. The cap is checked before the
 * quality gates, so a rejected row never consumes budget.
 */
export async function collectTopview(options = {}) {
  const {
    perModel = null,
    maxScanLines = MAX_SCAN_LINES,
    limits = MOCK_LIMITS,
  } = options
  const wanted = new Map(
    MODEL_SECTIONS.filter((section) => section.topview).map((section) => [
      section.topview,
      { section, items: [], seen: new Set() },
    ])
  )
  const dropped = emptyDrops()
  const stream = readline.createInterface({
    input: (await fs.open(TOPVIEW_FEED, 'r')).createReadStream(),
    crlfDelay: Infinity,
  })

  let scanned = 0
  let outOfScope = 0
  for await (const line of stream) {
    if (!line.trim()) continue
    scanned += 1
    if (maxScanLines != null && scanned > maxScanLines) break

    let raw
    try {
      raw = JSON.parse(line)
    } catch {
      dropped.unparsable += 1
      continue
    }

    const bucket = wanted.get(raw.model)
    if (!bucket) {
      // A model this build does not publish — out of scope, never a drop.
      outOfScope += 1
      continue
    }
    if (raw.status && raw.status !== 'published') {
      dropped.unpublished += 1
      continue
    }
    if (perModel != null && bucket.items.length >= perModel) {
      dropped.capped += 1
      continue
    }
    const item = fromTopview(raw, bucket.section, limits)
    if (!readableRequirement(item)) {
      dropped.prompt += 1
      continue
    }
    if (!item.previewImageUrl && !item.previewVideoUrl) {
      dropped.media += 1
      continue
    }
    if (bucket.seen.has(item.slug)) {
      dropped.duplicate += 1
      continue
    }
    bucket.seen.add(item.slug)
    bucket.items.push(item)
  }

  // Drop buckets that never filled up so the emitted fixture has no thin section.
  for (const [model, bucket] of wanted) {
    if (bucket.items.length === 0) wanted.delete(model)
  }
  for (const bucket of wanted.values()) delete bucket.seen
  // `modelled` = rows that belong to a published section, i.e. the rows this
  // build actually accounts for: kept + dropped. Everything else is out of
  // scope (a model the catalog does not list) and is not a drop.
  return { wanted, scanned, modelled: scanned - outOfScope, dropped }
}

/** Scan the EvoLink community feed for the models behind `MODEL_SECTIONS`. */
export async function collectEvolink(options = {}) {
  const {
    perModel = null,
    limits = MOCK_LIMITS,
    requireMedia = false,
  } = options
  const wanted = new Map(
    MODEL_SECTIONS.filter((section) => section.evolink).map((section) => [
      section.evolink,
      { section, items: [], seen: new Set() },
    ])
  )
  const dropped = emptyDrops()

  let rows = []
  try {
    const content = await fs.readFile(EVOLINK_FEED, 'utf8')
    rows = content.split('\n')
  } catch {
    return { wanted, scanned: 0, modelled: 0, dropped }
  }

  let scanned = 0
  let outOfScope = 0
  for (const line of rows) {
    if (!line.trim()) continue
    scanned += 1
    let raw
    try {
      raw = JSON.parse(line)
    } catch {
      dropped.unparsable += 1
      continue
    }
    const bucket = wanted.get(raw.modelKey)
    if (!bucket) {
      // A model this build does not publish — out of scope, never a drop.
      outOfScope += 1
      continue
    }
    if (perModel != null && bucket.items.length >= perModel) {
      dropped.capped += 1
      continue
    }
    const item = fromEvolink(raw, bucket.section, limits)
    if (!readableRequirement(item)) {
      dropped.prompt += 1
      continue
    }
    if (requireMedia && !item.previewImageUrl && !item.previewVideoUrl) {
      dropped.media += 1
      continue
    }
    if (bucket.seen.has(item.slug)) {
      dropped.duplicate += 1
      continue
    }
    bucket.seen.add(item.slug)
    bucket.items.push(item)
  }

  for (const bucket of wanted.values()) delete bucket.seen
  return { wanted, scanned, modelled: scanned - outOfScope, dropped }
}

/**
 * Skillry category slugs -> the Chinese labels the Claude zone exposes. An
 * unrecognized slug stays verbatim rather than being silently re-bucketed.
 */
const SKILLRY_CATEGORY_LABELS = {
  motion: '动效',
  explainer: '讲解',
  '3d': '3D',
  interactive: '交互',
}

/**
 * Shape one `videos.json` row into a catalog item for a `skillry` section.
 *
 * The dataset carries no media stream URL — cards render the poster as a
 * still (`previewVideoUrl` stays empty, `posterUrl`/`previewImageUrl` both
 * point at the WebP poster). `skillry_url` maps to `demoUrl`, the "watch the
 * render" link the modal shows for this zone; `author_url` is intentionally
 * not carried into the row.
 *
 * Rows flagged `prompt_partial` in the crawl keep their partial prompt but
 * get `missing: ['prompt']`: the build routes them to quarantine unchanged
 * (no `posterUnavailable*` markers — that vocabulary is reserved for dead
 * media URLs). Everything about the row stays serializable either way.
 */
export function fromSkillry(raw, section) {
  const slug = raw.slug || 'item'
  const id = `skillry:${section.key}:${slug}`
  const sourceUrl = isHttpUrl(raw.post_url) ? raw.post_url : ''
  const posterUrl = isHttpUrl(raw.poster_url) ? raw.poster_url : ''
  const category =
    SKILLRY_CATEGORY_LABELS[raw.category] ?? String(raw.category ?? '')
  const techTags = shapeList(raw.tech_tags, null)
  const derivedFields = ['title', 'titleEn', 'publishedAt', 'viewCount']
  let authorName = truncate(raw.author, 60)
  if (!authorName) {
    // No `author` string in the crawl: recover the handle from the status
    // URL the row links to, and mark the field as derived.
    authorName = deriveXHandle(sourceUrl) || 'Unknown'
    derivedFields.push('authorName')
  }
  const item = {
    id,
    slug,
    modelKey: section.key,
    modelLabel: section.modelLabel || section.label,
    mediaType: 'video',
    title: truncateWords(raw.prompt || slug, 120),
    titleEn: truncateWords(raw.prompt || slug, 120),
    description: '',
    descriptionEn: '',
    prompt: verbatimText(raw.prompt),
    category,
    useCases: category ? [category] : [],
    styles: techTags,
    tags: [...techTags],
    previewImageUrl: posterUrl,
    previewVideoUrl: '',
    posterUrl,
    aspectRatio: '',
    authorName,
    sourcePlatform: 'skillry',
    sourceUrl,
    publishedAt: derivePublishedAt(sourceUrl),
    viewCount: 0,
    originalViewCount: 0,
    demoUrl: isHttpUrl(raw.skillry_url) ? raw.skillry_url : '',
    derivedFields,
  }
  if (raw.prompt_partial === true) item.missing = ['prompt']
  return item
}

/**
 * Read the Skillry `videos.json` dataset for the sections marked
 * `skillry: true` in `MODEL_SECTIONS`.
 *
 * Unlike the JSONL feeds this file is one pretty-printed JSON array, and
 * unlike the community feeds it is a required input for the zone it backs:
 * a missing file or a non-array document throws rather than silently
 * emitting an empty zone.
 *
 * `missing`-flagged rows (partial prompts) stay in `bucket.items`; the build
 * decides displayable vs quarantine downstream so the two sets page against
 * the same ordering.
 */
export async function collectSkillry(options = {}) {
  const { file = SKILLRY_VIDEOS_FILE } = options
  const wanted = new Map(
    MODEL_SECTIONS.filter((section) => section.skillry === true).map(
      (section) => [section.key, { section, items: [], seen: new Set() }]
    )
  )
  const dropped = emptyDrops()

  const rows = JSON.parse(await fs.readFile(file, 'utf8'))
  if (!Array.isArray(rows)) {
    throw new Error(`${file} must be a JSON array of Skillry video rows`)
  }

  let scanned = 0
  for (const raw of rows) {
    scanned += 1
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) {
      dropped.unparsable += 1
      continue
    }
    for (const bucket of wanted.values()) {
      const item = fromSkillry(raw, bucket.section)
      if (bucket.seen.has(item.slug)) {
        dropped.duplicate += 1
        continue
      }
      if (item.missing === undefined && !item.prompt) {
        // The dataset is curated, so prompts are only unfit when empty — a
        // terse real prompt ("make a Tron game.") is displayable. An empty
        // prompt that the crawl did not flag partial is still a field-level
        // defect: quarantine it instead of dropping silently.
        item.missing = ['prompt']
      }
      if (!item.previewImageUrl && !item.previewVideoUrl) {
        // No poster and no video leaves the card nothing to render. This
        // check runs even for rows already flagged missing, so a quarantined
        // row can never reach a page without media either.
        dropped.media += 1
        continue
      }
      bucket.seen.add(item.slug)
      bucket.items.push(item)
    }
  }

  for (const bucket of wanted.values()) delete bucket.seen
  // Every row in the file belongs to a skillry section, so `modelled` is the
  // whole scan: kept + dropped, nothing out of scope.
  return { wanted, scanned, modelled: scanned, dropped }
}

/**
 * Read a poster pixel-size lookup. A missing file is fatal rather than a
 * silent all-fallback run: the map is an artifact of the crawl, so its absence
 * means the build would be emitted with wrong proportions. Callers that can
 * legitimately run without one catch the error and pass `{}`.
 */
export async function loadPosterRatios(file = POSTER_RATIOS_FILE) {
  const raw = await fs.readFile(file, 'utf8')
  const parsed = JSON.parse(raw)
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
    throw new Error(
      `${path.relative(WEB_ROOT, file)} must be a JSON object of { url: "w / h" }`
    )
  }
  return parsed
}

/**
 * Normalize one lookup entry to a CSS `aspect-ratio` value.
 *
 * The checked-in Topview map stores strings (`"1200 / 900"`); the Loomi
 * collector stores `[width, height]` pixel pairs. Both are accepted so either
 * map can feed either builder. Anything unmeasurable returns `''` and the
 * caller falls back.
 */
export function normalizeAspectRatio(value) {
  if (typeof value === 'string') {
    const trimmed = value.trim()
    return trimmed || ''
  }
  const width = Array.isArray(value) ? value[0] : value?.width
  const height = Array.isArray(value) ? value[1] : value?.height
  if (!Number.isFinite(width) || !Number.isFinite(height)) return ''
  if (width <= 0 || height <= 0) return ''
  return `${width} / ${height}`
}

/**
 * The URL a card actually renders, i.e. the poster when there is one and the
 * still frame otherwise — the same precedence both row mappers emit.
 */
export function displayedMediaUrl(item) {
  return item.posterUrl || item.previewImageUrl || ''
}

/**
 * Bake `aspectRatio` onto every item of every bucket. Returns the total number
 * of items with no measured entry plus that count per bucket key, so a caller
 * can report it once (mock) or per zone (catalog) instead of per item.
 */
export function attachAspectRatios(byKey, ratios) {
  const missingByKey = new Map()
  let total = 0
  for (const [key, items] of byKey) {
    let missing = 0
    for (const item of items) {
      const ratio = normalizeAspectRatio(ratios[displayedMediaUrl(item)])
      if (ratio) {
        item.aspectRatio = ratio
        continue
      }
      item.aspectRatio = FALLBACK_ASPECT_RATIO
      missing += 1
    }
    if (missing > 0) missingByKey.set(key, missing)
    total += missing
  }
  return { total, missingByKey }
}
