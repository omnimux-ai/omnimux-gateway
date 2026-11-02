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

import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import {
  clearPrefetchCache,
  isPrefetched,
  prefetchImage,
} from '../lib/prefetch'

describe('prefetchImage', () => {
  beforeEach(() => {
    clearPrefetchCache()
  })

  afterEach(() => {
    clearPrefetchCache()
  })

  it('drops empty, whitespace, and null URLs without prefetching', () => {
    expect(prefetchImage(null)).toBe(false)
    expect(prefetchImage(undefined)).toBe(false)
    expect(prefetchImage('')).toBe(false)
    expect(prefetchImage('   ')).toBe(false)
  })

  it('prefetches a valid URL on first encounter', () => {
    const url = 'https://example.com/sheet.webp'
    expect(isPrefetched(url)).toBe(false)
    expect(prefetchImage(url)).toBe(true)
    expect(isPrefetched(url)).toBe(true)
  })

  it('deduplicates repeat prefetch requests for the same URL', () => {
    const url = 'https://example.com/sheet.webp'
    expect(prefetchImage(url)).toBe(true)
    expect(prefetchImage(url)).toBe(false)
    expect(prefetchImage(`  ${url}  `)).toBe(false)
  })
})
