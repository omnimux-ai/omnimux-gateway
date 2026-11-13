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

import type { InfluencerTaxonomy, PublishedPresetRow } from '../types'

const VERSION = 1
const TAXONOMY_KEY = 'influencer:taxonomy-cache:v1'
const PRESETS_KEY = 'influencer:presets-cache:v1'

/**
 * Taxonomy is static option metadata bundled into the server build.
 * It is valid for 24 hours in local cache; TanStack Query still validates
 * on mount via initialDataUpdatedAt: 0.
 */
export const TAXONOMY_TTL = 24 * 60 * 60 * 1000

/**
 * Published presets list changes when admins promote or withdraw characters.
 * Cached for 60 seconds to provide an instant second paint without stale ghosts.
 */
export const PRESETS_TTL = 60 * 1000

interface TaxonomyPayload {
  taxonomy: InfluencerTaxonomy
  etag: string
}

interface TaxonomyCacheEnvelope {
  v: number
  savedAt: number
  data: TaxonomyPayload
}

interface PresetsCacheEnvelope {
  v: number
  savedAt: number
  items: PublishedPresetRow[]
}

function storage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage
  } catch {
    return null
  }
}

export function readTaxonomyCache(): TaxonomyPayload | null {
  const store = storage()
  if (!store) return null
  try {
    const raw = store.getItem(TAXONOMY_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<TaxonomyCacheEnvelope>
    if (
      parsed?.v !== VERSION ||
      typeof parsed.savedAt !== 'number' ||
      Date.now() - parsed.savedAt > TAXONOMY_TTL ||
      !parsed.data?.taxonomy ||
      !Array.isArray(parsed.data.taxonomy.categories)
    ) {
      store.removeItem(TAXONOMY_KEY)
      return null
    }
    return parsed.data
  } catch {
    try {
      store.removeItem(TAXONOMY_KEY)
    } catch {
      /* nothing left to recover */
    }
    return null
  }
}

export function writeTaxonomyCache(data: TaxonomyPayload): void {
  const store = storage()
  if (!store || !data?.taxonomy || !Array.isArray(data.taxonomy.categories)) return
  const envelope: TaxonomyCacheEnvelope = {
    v: VERSION,
    savedAt: Date.now(),
    data,
  }
  try {
    store.setItem(TAXONOMY_KEY, JSON.stringify(envelope))
  } catch {
    /* Quota exceeded or private browsing: network still loads normally. */
  }
}

export function clearTaxonomyCache(): void {
  const store = storage()
  if (!store) return
  try {
    store.removeItem(TAXONOMY_KEY)
  } catch {
    /* ignore */
  }
}

export function readPresetsCache(): PublishedPresetRow[] | null {
  const store = storage()
  if (!store) return null
  try {
    const raw = store.getItem(PRESETS_KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<PresetsCacheEnvelope>
    if (
      parsed?.v !== VERSION ||
      typeof parsed.savedAt !== 'number' ||
      Date.now() - parsed.savedAt > PRESETS_TTL ||
      !Array.isArray(parsed.items)
    ) {
      store.removeItem(PRESETS_KEY)
      return null
    }
    return parsed.items
  } catch {
    try {
      store.removeItem(PRESETS_KEY)
    } catch {
      /* nothing left to recover */
    }
    return null
  }
}

export function writePresetsCache(items: PublishedPresetRow[]): void {
  const store = storage()
  if (!store || !Array.isArray(items)) return
  const envelope: PresetsCacheEnvelope = {
    v: VERSION,
    savedAt: Date.now(),
    items,
  }
  try {
    store.setItem(PRESETS_KEY, JSON.stringify(envelope))
  } catch {
    /* Quota exceeded or private browsing: network still loads normally. */
  }
}

export function clearPresetsCache(): void {
  const store = storage()
  if (!store) return
  try {
    store.removeItem(PRESETS_KEY)
  } catch {
    /* ignore */
  }
}
