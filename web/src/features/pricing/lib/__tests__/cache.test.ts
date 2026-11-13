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
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { PRICING_CACHE_KEY, PRICING_CACHE_TTL } from '../../constants'
import type { PricingCacheEnvelope, PricingData } from '../../types'
import {
  clearPricingCache,
  loadPricingCache,
  savePricingCache,
} from '../cache'

function makePricingData(overrides: Partial<PricingData> = {}): PricingData {
  return {
    success: true,
    data: [],
    vendors: [],
    group_ratio: {},
    usable_group: {},
    supported_endpoint: {},
    auto_groups: [],
    pricing_version: 'v-test',
    ...overrides,
  }
}

function makeEnvelope(
  overrides: Partial<PricingCacheEnvelope> = {}
): PricingCacheEnvelope {
  return {
    version: 'v-test',
    timestamp: Date.now(),
    pricingData: makePricingData(),
    statusData: {
      price: 1,
      usd_exchange_rate: 6.8,
      quota_per_unit: 500000,
    },
    ...overrides,
  }
}

/** In-memory Storage — Node may ship a broken localStorage stub. */
function createMemoryStorage(): Storage {
  const store = new Map<string, string>()
  return {
    get length() {
      return store.size
    },
    clear() {
      store.clear()
    },
    getItem(key: string) {
      return store.has(key) ? (store.get(key) ?? null) : null
    },
    key(index: number) {
      return [...store.keys()][index] ?? null
    },
    removeItem(key: string) {
      store.delete(key)
    },
    setItem(key: string, value: string) {
      store.set(key, String(value))
    },
  }
}

let memoryStorage: Storage

beforeEach(() => {
  memoryStorage = createMemoryStorage()
  Object.defineProperty(window, 'localStorage', {
    configurable: true,
    writable: true,
    value: memoryStorage,
  })
})

afterEach(() => {
  memoryStorage.clear()
  vi.restoreAllMocks()
})

describe('pricing L2 cache', () => {
  test('save then load round-trips a valid envelope', () => {
    const envelope = makeEnvelope({ version: 'abc123' })
    expect(savePricingCache(envelope)).toBe(true)

    const loaded = loadPricingCache()
    expect(loaded).not.toBeNull()
    expect(loaded?.version).toBe('abc123')
    expect(loaded?.pricingData.success).toBe(true)
    expect(loaded?.statusData?.quota_per_unit).toBe(500000)
    expect(window.localStorage.getItem(PRICING_CACHE_KEY)).toBeTruthy()
  })

  test('loadPricingCache returns null when key is missing', () => {
    expect(loadPricingCache()).toBeNull()
  })

  test('loadPricingCache clears and returns null for malformed JSON', () => {
    window.localStorage.setItem(PRICING_CACHE_KEY, '{not-json')
    expect(loadPricingCache()).toBeNull()
    expect(window.localStorage.getItem(PRICING_CACHE_KEY)).toBeNull()
  })

  test('loadPricingCache clears and returns null for structurally invalid envelope', () => {
    window.localStorage.setItem(
      PRICING_CACHE_KEY,
      JSON.stringify({ version: 1, timestamp: Date.now() })
    )
    expect(loadPricingCache()).toBeNull()
    expect(window.localStorage.getItem(PRICING_CACHE_KEY)).toBeNull()
  })

  test('loadPricingCache expires envelopes older than PRICING_CACHE_TTL', () => {
    const stale = makeEnvelope({
      timestamp: Date.now() - PRICING_CACHE_TTL - 1,
    })
    expect(savePricingCache(stale)).toBe(true)
    expect(loadPricingCache()).toBeNull()
    expect(window.localStorage.getItem(PRICING_CACHE_KEY)).toBeNull()
  })

  test('clearPricingCache removes the stored envelope', () => {
    expect(savePricingCache(makeEnvelope())).toBe(true)
    clearPricingCache()
    expect(loadPricingCache()).toBeNull()
    expect(window.localStorage.getItem(PRICING_CACHE_KEY)).toBeNull()
  })

  test('savePricingCache returns false for invalid envelopes without writing', () => {
    const invalid = {
      version: '',
      timestamp: Date.now(),
      pricingData: makePricingData(),
    } as PricingCacheEnvelope

    expect(savePricingCache(invalid)).toBe(false)
    expect(window.localStorage.getItem(PRICING_CACHE_KEY)).toBeNull()
  })

  test('savePricingCache returns false when localStorage.setItem throws', () => {
    memoryStorage.setItem = () => {
      throw new Error('quota exceeded')
    }

    expect(savePricingCache(makeEnvelope())).toBe(false)
  })
})
