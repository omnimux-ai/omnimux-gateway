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
  classifySavingBadge,
  resolveCardSavingBadge,
  resolveTierSavingBadge,
} from '../saving-display'

describe('classifySavingBadge', () => {
  test('returns none when official price is absent', () => {
    expect(classifySavingBadge(20, false)).toEqual({
      kind: 'none',
      percent: null,
    })
  })

  test('returns off for positive saving and same for zero', () => {
    expect(classifySavingBadge(30, true)).toEqual({
      kind: 'off',
      percent: 30,
    })
    expect(classifySavingBadge(0, true)).toEqual({
      kind: 'same',
      percent: 0,
    })
  })
})

describe('resolveTierSavingBadge', () => {
  test('derives OFF badge from credits vs officialUsd', () => {
    // 0.15 credits = $0.015; official $0.01875 → 20%
    expect(
      resolveTierSavingBadge({
        credits: 0.15,
        officialUsd: 0.01875,
        savingPercent: 20,
      })
    ).toEqual({ kind: 'off', percent: 20 })
  })

  test('returns Official 1:1 when savingPercent is 0', () => {
    expect(
      resolveTierSavingBadge({
        credits: 0.4,
        officialUsd: 0.04,
        savingPercent: 0,
      })
    ).toEqual({ kind: 'same', percent: 0 })
  })
})

describe('resolveCardSavingBadge', () => {
  test('picks the highest positive saving across tiers', () => {
    expect(
      resolveCardSavingBadge([
        { credits: 0.4, officialUsd: 0.04, savingPercent: 0 },
        { credits: 1.2, officialUsd: 0.16, savingPercent: 25 },
        { credits: 0.8, officialUsd: 0.1, savingPercent: 20 },
      ])
    ).toEqual({ kind: 'off', percent: 25 })
  })

  test('falls back to Official 1:1 when all comparable tiers are same price', () => {
    expect(
      resolveCardSavingBadge([
        { credits: 0.4, officialUsd: 0.04, savingPercent: 0 },
        { credits: 0.8, officialUsd: 0.08, savingPercent: 0 },
      ])
    ).toEqual({ kind: 'same', percent: 0 })
  })
})
