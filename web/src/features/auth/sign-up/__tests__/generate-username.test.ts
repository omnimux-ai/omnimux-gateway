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
import { describe, expect, it } from 'vitest'

import { generateRandomUsername } from '../lib/generate-username'

describe('generateRandomUsername', () => {
  it('generates a non-empty username with u_ prefix', () => {
    const username = generateRandomUsername()
    expect(username.startsWith('u_')).toBe(true)
    expect(username.length).toBeGreaterThan(5)
  })

  it('strictly adheres to backend length limit of max 20 characters', () => {
    for (let i = 0; i < 50; i++) {
      const username = generateRandomUsername()
      expect(username.length).toBeLessThanOrEqual(20)
    }
  })

  it('generates distinct usernames across calls', () => {
    const set = new Set<string>()
    for (let i = 0; i < 50; i++) {
      set.add(generateRandomUsername())
    }
    expect(set.size).toBe(50)
  })
})
