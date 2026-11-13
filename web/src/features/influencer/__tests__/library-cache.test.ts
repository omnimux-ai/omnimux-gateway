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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  clearPresetsCache,
  clearTaxonomyCache,
  PRESETS_TTL,
  readPresetsCache,
  readTaxonomyCache,
  TAXONOMY_TTL,
  writePresetsCache,
  writeTaxonomyCache,
} from '../lib/library-cache'
import type { InfluencerTaxonomy, PublishedPresetRow } from '../types'

const SAMPLE_TAXONOMY: InfluencerTaxonomy = {
  version: 1,
  source: 'test',
  tier_group: {
    options: [{ id: 'average', label_en: 'Average', visibleIn: ['average'] }],
    comingSoonImageUrls: [],
  },
  category_priority: ['gender'],
  categories: [
    {
      id: 'gender',
      label_en: 'Gender',
      kind: 'text',
      max: 1,
      options: [{ id: 'male', label_en: 'Male', visibleIn: ['average'] }],
    },
  ],
  prompt_map: {
    tiers: {},
    categories: {},
  },
}

const SAMPLE_PRESET_ROW: PublishedPresetRow = {
  id: 'pub-1',
  name: 'Test Creator',
  tier: 'average',
  selection: { gender: ['male'] },
  brief: 'A test character',
  seed: 1234,
  preview: { path: '/asset/preview.webp', width: 400, height: 600 },
  sheet: { path: '/asset/sheet.webp', width: 800, height: 600 },
  created_at: 1700000000,
  published: true,
}

describe('library-cache', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
  })

  afterEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
  })

  describe('taxonomy cache', () => {
    it('returns null when the cache is empty', () => {
      expect(readTaxonomyCache()).toBeNull()
    })

    it('writes and reads valid taxonomy cache', () => {
      writeTaxonomyCache({ taxonomy: SAMPLE_TAXONOMY, etag: 'etag-123' })
      const cached = readTaxonomyCache()
      expect(cached).not.toBeNull()
      expect(cached?.etag).toBe('etag-123')
      expect(cached?.taxonomy.categories).toHaveLength(1)
    })

    it('expires after TAXONOMY_TTL', () => {
      vi.useFakeTimers()
      writeTaxonomyCache({ taxonomy: SAMPLE_TAXONOMY, etag: 'etag-123' })
      expect(readTaxonomyCache()).not.toBeNull()

      vi.advanceTimersByTime(TAXONOMY_TTL + 1000)
      expect(readTaxonomyCache()).toBeNull()
      vi.useRealTimers()
    })

    it('clears corrupted taxonomy cache safely', () => {
      localStorage.setItem('influencer:taxonomy-cache:v1', 'invalid json {{{')
      expect(readTaxonomyCache()).toBeNull()
      expect(localStorage.getItem('influencer:taxonomy-cache:v1')).toBeNull()
    })

    it('clears cache with wrong envelope version or structure', () => {
      localStorage.setItem(
        'influencer:taxonomy-cache:v1',
        JSON.stringify({ v: 999, savedAt: Date.now(), data: {} })
      )
      expect(readTaxonomyCache()).toBeNull()
      expect(localStorage.getItem('influencer:taxonomy-cache:v1')).toBeNull()
    })

    it('can be cleared explicitly', () => {
      writeTaxonomyCache({ taxonomy: SAMPLE_TAXONOMY, etag: 'etag-123' })
      clearTaxonomyCache()
      expect(readTaxonomyCache()).toBeNull()
    })
  })

  describe('presets cache', () => {
    it('returns null when the cache is empty', () => {
      expect(readPresetsCache()).toBeNull()
    })

    it('writes and reads valid presets cache', () => {
      writePresetsCache([SAMPLE_PRESET_ROW])
      const cached = readPresetsCache()
      expect(cached).not.toBeNull()
      expect(cached).toHaveLength(1)
      expect(cached?.[0].id).toBe('pub-1')
    })

    it('expires after PRESETS_TTL', () => {
      vi.useFakeTimers()
      writePresetsCache([SAMPLE_PRESET_ROW])
      expect(readPresetsCache()).not.toBeNull()

      vi.advanceTimersByTime(PRESETS_TTL + 1000)
      expect(readPresetsCache()).toBeNull()
      vi.useRealTimers()
    })

    it('clears corrupted presets cache safely', () => {
      localStorage.setItem('influencer:presets-cache:v1', 'not valid json')
      expect(readPresetsCache()).toBeNull()
      expect(localStorage.getItem('influencer:presets-cache:v1')).toBeNull()
    })

    it('can be cleared explicitly', () => {
      writePresetsCache([SAMPLE_PRESET_ROW])
      clearPresetsCache()
      expect(readPresetsCache()).toBeNull()
    })
  })

  describe('storage failure tolerance', () => {
    it('handles localStorage throwing on access', () => {
      vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
        throw new Error('SecurityError: Access denied')
      })
      expect(readTaxonomyCache()).toBeNull()
      expect(readPresetsCache()).toBeNull()
    })

    it('handles localStorage throwing on setItem gracefully', () => {
      vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
        throw new Error('QuotaExceededError')
      })
      expect(() => {
        writeTaxonomyCache({ taxonomy: SAMPLE_TAXONOMY, etag: 'etag' })
      }).not.toThrow()
      expect(() => {
        writePresetsCache([SAMPLE_PRESET_ROW])
      }).not.toThrow()
    })
  })
})
