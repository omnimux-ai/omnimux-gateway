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

import { afterEach, describe, expect, it, vi } from 'vitest'

import { readSource, writeSource } from '../lib/source'

const KEY = 'influencer:source'

afterEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

describe('readSource', () => {
  it('defaults to explore when nothing is stored', () => {
    expect(readSource()).toBe('explore')
  })

  it('returns history once history was stored', () => {
    writeSource('history')
    expect(readSource()).toBe('history')
  })

  it('falls back to explore for an unknown stored value', () => {
    localStorage.setItem(KEY, 'trending')
    expect(readSource()).toBe('explore')
  })

  it('falls back to explore when storage throws', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage disabled')
    })
    expect(readSource()).toBe('explore')
  })
})

describe('writeSource', () => {
  it('round-trips back to explore', () => {
    writeSource('history')
    writeSource('explore')
    expect(readSource()).toBe('explore')
  })

  it('stays silent when storage rejects the write', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('quota exceeded')
    })
    expect(() => writeSource('history')).not.toThrow()
  })
})
