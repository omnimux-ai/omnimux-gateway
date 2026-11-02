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

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { readExploreTab, writeExploreTab } from '../lib/explore-tab'

const KEY = 'influencer:explore-tab'

describe('readExploreTab', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('defaults to the character shelf on first visit', () => {
    expect(readExploreTab()).toBe('influencers')
  })

  it('reads back the shelf the user last picked', () => {
    writeExploreTab('trends')
    expect(readExploreTab()).toBe('trends')
    writeExploreTab('presets')
    expect(readExploreTab()).toBe('presets')
  })

  it('falls back to influencers for an unknown stored value', () => {
    localStorage.setItem(KEY, 'trending')
    expect(readExploreTab()).toBe('influencers')
  })

  it('falls back to influencers when storage throws', () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('storage disabled')
    })
    expect(readExploreTab()).toBe('influencers')
  })
})
