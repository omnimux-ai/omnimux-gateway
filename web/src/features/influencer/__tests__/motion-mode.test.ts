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

import { readMotionMode, writeMotionMode } from '../lib/motion-mode'

const KEY = 'influencer:motion-mode'

afterEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

describe('readMotionMode', () => {
  it('defaults to motion transfer when nothing is stored', () => {
    expect(readMotionMode()).toBe('motion')
  })

  it('returns object swap once it was stored', () => {
    writeMotionMode('replace')
    expect(readMotionMode()).toBe('replace')
  })

  it('falls back to motion transfer for an unknown stored value', () => {
    localStorage.setItem(KEY, 'swap')
    expect(readMotionMode()).toBe('motion')
  })

  it('falls back to motion transfer when storage throws', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage disabled')
    })
    expect(readMotionMode()).toBe('motion')
  })
})

describe('writeMotionMode', () => {
  it('round-trips back to motion transfer', () => {
    writeMotionMode('replace')
    writeMotionMode('motion')
    expect(readMotionMode()).toBe('motion')
  })

  it('swallows a storage failure instead of breaking the session', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('storage disabled')
    })
    expect(() => writeMotionMode('replace')).not.toThrow()
  })
})
