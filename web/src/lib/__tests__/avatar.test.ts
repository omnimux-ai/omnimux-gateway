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
import { describe, expect, test } from 'vitest'

import {
  USER_AVATAR_PALETTE,
  getUserAvatarFallback,
  getUserAvatarStyle,
} from '../avatar'

function relativeLuminance(hex: string): number {
  const channels = [1, 3, 5].map((offset) => {
    const channel = Number.parseInt(hex.slice(offset, offset + 2), 16) / 255
    return channel <= 0.03928
      ? channel / 12.92
      : ((channel + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * channels[0] + 0.7152 * channels[1] + 0.0722 * channels[2]
}

function contrastRatio(hexA: string, hexB: string): number {
  const lumA = relativeLuminance(hexA)
  const lumB = relativeLuminance(hexB)
  return (Math.max(lumA, lumB) + 0.05) / (Math.min(lumA, lumB) + 0.05)
}

function hueOf(hex: string): number {
  const [r, g, b] = [1, 3, 5].map(
    (i) => Number.parseInt(hex.slice(i, i + 2), 16) / 255
  )
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const delta = max - min
  if (delta === 0) return 0
  let hue = 0
  if (max === r) hue = (((g - b) / delta) % 6) * 60
  else if (max === g) hue = ((b - r) / delta + 2) * 60
  else hue = ((r - g) / delta + 4) * 60
  return hue < 0 ? hue + 360 : hue
}

describe('USER_AVATAR_PALETTE', () => {
  test('maintains WCAG AA (>= 4.5:1) contrast with white text on every swatch', () => {
    for (const hex of USER_AVATAR_PALETTE) {
      expect(contrastRatio(hex, '#FFFFFF')).toBeGreaterThanOrEqual(4.5)
    }
  })

  test('excludes violet hues reserved for AI energy flow in DESIGN.md', () => {
    for (const hex of USER_AVATAR_PALETTE) {
      const hue = hueOf(hex)
      expect(hue < 235 || hue > 300).toBe(true)
    }
  })
})

describe('getUserAvatarStyle', () => {
  test('returns a deterministic curated swatch with white text', () => {
    const first = getUserAvatarStyle('admin')
    const second = getUserAvatarStyle('admin')
    expect(first).toEqual(second)
    expect(first.color).toBe('#FFFFFF')
    expect(USER_AVATAR_PALETTE).toContain(first.backgroundColor)
  })
})

describe('getUserAvatarFallback', () => {
  test.each([
    { input: 'admin', expected: 'A' },
    { input: '  seller-li  ', expected: 'S' },
    { input: '李掌柜', expected: '李' },
    { input: '   ', expected: '?' },
    { input: '', expected: '?' },
    { input: '🚀rocket', expected: '🚀' },
  ])('extracts fallback "$expected" from "$input"', ({ input, expected }) => {
    expect(getUserAvatarFallback(input)).toBe(expected)
  })
})
