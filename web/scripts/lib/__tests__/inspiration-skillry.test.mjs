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
 * The Skillry datasource contract for the Claude zone: `videos.json` rows map
 * to catalog items whose cards render the poster still and link out via
 * `demoUrl`, curated short prompts stay displayable, and field-level defects
 * (`prompt_partial` or an empty prompt) are quarantined through `missing`
 * instead of dropped.
 */
import fs from 'node:fs'
import os from 'node:os'
import path from 'node:path'

import { afterAll, describe, expect, it } from 'vitest'

import {
  collectSkillry,
  derivePublishedAt,
  deriveXHandle,
  fromSkillry,
  truncateWords,
} from '../inspiration-sources.mjs'

/** The only `MODEL_SECTIONS` entry flagged `skillry: true`. */
const SECTION = { key: 'claude-opus-5.5', label: 'Claude zone' }

/** A row shaped like one `videos.json` entry; every field overridable. */
function rawRow(overrides = {}) {
  return {
    slug: 'author-123456',
    author: 'author',
    author_url: 'https://x.com/author',
    post_url: 'https://x.com/author/status/20',
    category: 'motion',
    tech_tags: ['canvas', 'gsap'],
    prompt: 'A prompt long enough to read like a real description.',
    prompt_partial: false,
    poster_url: 'https://media.skillry.dev/posters/author-123456.webp',
    skillry_url: 'https://skillry.dev/videos/author-123456',
    ...overrides,
  }
}

/** Write `rows` to a temp `videos.json` and return its path. */
function writeDataset(rows) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'skillry-test-'))
  const file = path.join(dir, 'videos.json')
  fs.writeFileSync(file, JSON.stringify(rows))
  tempDirs.push(dir)
  return file
}

const tempDirs = []

afterAll(() => {
  for (const dir of tempDirs) fs.rmSync(dir, { recursive: true, force: true })
})

describe('fromSkillry', () => {
  it('maps a videos.json row into a still-card catalog item', () => {
    const item = fromSkillry(rawRow(), SECTION)

    expect(item.id).toBe('skillry:claude-opus-5.5:author-123456')
    expect(item.slug).toBe('author-123456')
    expect(item.modelKey).toBe('claude-opus-5.5')
    expect(item.modelLabel).toBe('Claude zone')
    expect(item.mediaType).toBe('video')
    expect(item.title).toBe(rawRow().prompt)
    expect(item.titleEn).toBe(rawRow().prompt)
    expect(item.prompt).toBe(rawRow().prompt)
    expect(item.category).toBe('动效')
    expect(item.useCases).toEqual(['动效'])
    expect(item.styles).toEqual(['canvas', 'gsap'])
    expect(item.tags).toEqual(['canvas', 'gsap'])
    expect(item.posterUrl).toBe(rawRow().poster_url)
    expect(item.previewImageUrl).toBe(rawRow().poster_url)
    expect(item.previewVideoUrl).toBe('')
    expect(item.demoUrl).toBe(rawRow().skillry_url)
    expect(item.sourcePlatform).toBe('skillry')
    expect(item.sourceUrl).toBe(rawRow().post_url)
    expect(item.authorName).toBe('author')
    expect(item.viewCount).toBe(0)
    // No synthesized copy: descriptions stay empty for this zone.
    expect(item.description).toBe('')
    expect(item.descriptionEn).toBe('')
    expect(item.derivedFields).toEqual([
      'title',
      'titleEn',
      'publishedAt',
      'viewCount',
    ])
    expect(item.missing).toBeUndefined()
  })

  it('marks prompt_partial rows missing the prompt but keeps them shapable', () => {
    const item = fromSkillry(rawRow({ prompt_partial: true }), SECTION)

    expect(item.missing).toEqual(['prompt'])
    expect(item.prompt).toBe(rawRow().prompt)
    // Quarantine is field-level: no posterUnavailable vocabulary on the row.
    expect(Object.keys(item)).not.toContain('posterUnavailable')
    expect(Object.keys(item)).not.toContain('posterUnavailableReason')
  })

  it('recovers authorName from the status URL when the crawl has no author', () => {
    const item = fromSkillry(rawRow({ author: '' }), SECTION)

    expect(item.authorName).toBe('author')
    expect(item.derivedFields).toContain('authorName')
  })

  it('falls back to Unknown when neither author nor a handle is recoverable', () => {
    const item = fromSkillry(
      rawRow({ author: '', post_url: 'https://example.com/post/1' }),
      SECTION
    )

    expect(item.authorName).toBe('Unknown')
    expect(item.sourceUrl).toBe('https://example.com/post/1')
  })

  it('keeps an unrecognized category slug verbatim instead of re-bucketing', () => {
    const item = fromSkillry(rawRow({ category: 'fractal' }), SECTION)

    expect(item.category).toBe('fractal')
    expect(item.useCases).toEqual(['fractal'])
  })
})

describe('collectSkillry', () => {
  it('collects every row for the claude-opus-5.5 section', async () => {
    const file = writeDataset([rawRow(), rawRow({ slug: 'other-654321' })])
    const { wanted, scanned, modelled } = await collectSkillry({ file })
    const bucket = wanted.get('claude-opus-5.5')

    expect(scanned).toBe(2)
    expect(modelled).toBe(2)
    expect(bucket.items).toHaveLength(2)
    expect(bucket.items.map((item) => item.id)).toEqual([
      'skillry:claude-opus-5.5:author-123456',
      'skillry:claude-opus-5.5:other-654321',
    ])
  })

  it('keeps a terse real prompt displayable instead of dropping it', async () => {
    // Regression: the feed-level readableRequirement (prompt > 40 chars) does
    // not apply to this curated dataset — "Go all out." must survive.
    const file = writeDataset([rawRow({ prompt: 'Go all out.' })])
    const { wanted, dropped } = await collectSkillry({ file })

    const items = wanted.get('claude-opus-5.5').items
    expect(items).toHaveLength(1)
    expect(items[0].prompt).toBe('Go all out.')
    expect(items[0].missing).toBeUndefined()
    expect(dropped.prompt).toBe(0)
  })

  it('quarantines an empty prompt via missing rather than dropping the row', async () => {
    const file = writeDataset([rawRow({ prompt: '' })])
    const { wanted, dropped, modelled } = await collectSkillry({ file })

    const items = wanted.get('claude-opus-5.5').items
    expect(items).toHaveLength(1)
    expect(items[0].missing).toEqual(['prompt'])
    expect(modelled).toBe(1)
    expect(dropped.media).toBe(0)
  })

  it('routes prompt_partial rows into the same missing quarantine', async () => {
    const file = writeDataset([
      rawRow({ prompt: 'partial crawl', prompt_partial: true }),
    ])
    const { wanted } = await collectSkillry({ file })

    const items = wanted.get('claude-opus-5.5').items
    expect(items).toHaveLength(1)
    expect(items[0].missing).toEqual(['prompt'])
  })

  it('drops a row only when it has no poster and no video to render', async () => {
    const file = writeDataset([
      rawRow({ poster_url: '' }),
      rawRow({ slug: 'kept-111111' }),
    ])
    const { wanted, dropped, modelled } = await collectSkillry({ file })

    expect(wanted.get('claude-opus-5.5').items).toHaveLength(1)
    expect(dropped.media).toBe(1)
    expect(modelled).toBe(2)
  })

  it('drops a quarantined row that has no media to render', async () => {
    // Regression: the media check must run even when the row already carries
    // a `missing` flag — a quarantined card still cannot render without a
    // poster or a video.
    const file = writeDataset([
      rawRow({ prompt: 'partial crawl', prompt_partial: true, poster_url: '' }),
      rawRow({ slug: 'kept-111111' }),
    ])
    const { wanted, dropped, modelled } = await collectSkillry({ file })

    expect(wanted.get('claude-opus-5.5').items).toHaveLength(1)
    expect(dropped.media).toBe(1)
    expect(modelled).toBe(2)
  })

  it('counts a repeated slug once as a duplicate drop', async () => {
    const file = writeDataset([rawRow(), rawRow()])
    const { wanted, dropped, scanned } = await collectSkillry({ file })

    expect(scanned).toBe(2)
    expect(wanted.get('claude-opus-5.5').items).toHaveLength(1)
    expect(dropped.duplicate).toBe(1)
  })

  it('counts non-object rows as unparsable', async () => {
    const file = writeDataset([null, 'junk', rawRow()])
    const { wanted, dropped, scanned } = await collectSkillry({ file })

    expect(scanned).toBe(3)
    expect(dropped.unparsable).toBe(2)
    expect(wanted.get('claude-opus-5.5').items).toHaveLength(1)
  })

  it('throws on a non-array document instead of emitting an empty zone', async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'skillry-test-'))
    tempDirs.push(dir)
    const file = path.join(dir, 'videos.json')
    fs.writeFileSync(file, JSON.stringify({ rows: [] }))

    await expect(collectSkillry({ file })).rejects.toThrow(
      /must be a JSON array/u
    )
  })

  it('throws when the dataset file does not exist', async () => {
    await expect(
      collectSkillry({ file: '/nonexistent/skillry/videos.json' })
    ).rejects.toThrow()
  })
})

describe('deriveXHandle', () => {
  it('extracts the handle from x.com and twitter.com status URLs', () => {
    expect(deriveXHandle('https://x.com/jack/status/20')).toBe('jack')
    expect(deriveXHandle('https://twitter.com/jack/status/20')).toBe('jack')
    expect(deriveXHandle('https://www.x.com/Some_User/status/123')).toBe(
      'Some_User'
    )
  })

  it('rejects the reserved i/ handle and non-status URLs', () => {
    expect(deriveXHandle('https://x.com/i/status/20')).toBe('')
    expect(deriveXHandle('https://x.com/jack')).toBe('')
    expect(deriveXHandle('https://example.com/jack/status/20')).toBe('')
    expect(deriveXHandle('')).toBe('')
  })
})

describe('derivePublishedAt', () => {
  it('decodes a tweet snowflake id into a naive UTC timestamp', () => {
    // Status id 20 is Jack Dorsey's first tweet; its snowflake decodes to
    // 2010-11-04T01:42:54.657Z, sliced to seconds like the Topview crawl.
    expect(derivePublishedAt('https://x.com/jack/status/20')).toBe(
      '2010-11-04T01:42:54'
    )
  })

  it('returns empty for non-status or malformed URLs', () => {
    expect(derivePublishedAt('https://x.com/jack')).toBe('')
    expect(derivePublishedAt('https://x.com/jack/status/abc')).toBe('')
    expect(derivePublishedAt('')).toBe('')
  })
})

describe('truncateWords', () => {
  it('returns text under the cap unchanged after whitespace collapsing', () => {
    expect(truncateWords('hello   world', 50)).toBe('hello world')
    expect(truncateWords('exactly10!', 10)).toBe('exactly10!')
  })

  it('backs an over-cap cut off to the last word boundary', () => {
    // max 10 → cut at index 9 ("one two t") → back to "one two" + ellipsis.
    expect(truncateWords('one two three', 10)).toBe('one two…')
  })

  it('hard-cuts a first word longer than the cap', () => {
    expect(truncateWords('abcdefghij', 5)).toBe('abcd…')
  })

  it('normalizes non-strings to empty', () => {
    expect(truncateWords(null, 10)).toBe('')
    expect(truncateWords(undefined, 10)).toBe('')
  })
})
