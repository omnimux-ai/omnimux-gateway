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

import { readWorkbenchMode, writeWorkbenchMode } from '../lib/workbench-mode'

const KEY = 'influencer:workbench-mode'

afterEach(() => {
  localStorage.clear()
  vi.restoreAllMocks()
})

describe('readWorkbenchMode', () => {
  it('lands on character design when nothing is stored', () => {
    expect(readWorkbenchMode()).toBe('image')
  })

  it('returns the video surface once it was stored', () => {
    writeWorkbenchMode('video')
    expect(readWorkbenchMode()).toBe('video')
  })

  it('falls back to character design for an unknown stored value', () => {
    localStorage.setItem(KEY, 'motion')
    expect(readWorkbenchMode()).toBe('image')
  })

  it('falls back to character design when storage throws', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage disabled')
    })
    expect(readWorkbenchMode()).toBe('image')
  })
})

describe('writeWorkbenchMode', () => {
  it('round-trips back to character design', () => {
    writeWorkbenchMode('video')
    writeWorkbenchMode('image')
    expect(readWorkbenchMode()).toBe('image')
  })

  it('swallows a storage failure instead of breaking the session', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('storage disabled')
    })
    expect(() => writeWorkbenchMode('video')).not.toThrow()
  })
})
