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

import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'

const PRICING_KEYS = [
  'Pay for what you use',
  'How billing works',
  'Top up credits',
  'Call any model',
  'Every cent is traceable',
  'View model prices',
  'Enterprise pricing',
  'Search models, IDs, or providers…',
  'All providers',
  'VIDEO',
  'LLM',
  'IMAGE',
  'AUDIO',
  'ALL',
  'SPEC',
  'OUR PRICE',
  'OFFICIAL PRICE',
  'SAVING',
  'Credits',
] as const

describe('pricing locale namespace', () => {
  test('keeps pricing copy under translation and not at locale root', () => {
    for (const locale of [en, zh]) {
      const rootKeys = Object.keys(locale).filter((key) => key !== 'translation')
      expect(rootKeys).toEqual([])
      expect(locale.translation).toBeTruthy()
    }

    for (const key of PRICING_KEYS) {
      expect(en.translation[key]).toBeTruthy()
      expect(zh.translation[key]).toBeTruthy()
      expect(zh.translation[key]).not.toBe(key)
    }

    expect(zh.translation['Pay for what you use']).toBe('用多少，付多少')
    expect(zh.translation.Credits).toBe('积分')
  })
})
