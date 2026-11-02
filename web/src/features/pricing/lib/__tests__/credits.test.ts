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
  CREDITS_PER_USD,
  DEFAULT_QUOTA_PER_UNIT,
  USD_PER_CREDIT,
} from '../../constants'
import {
  calculateSaving,
  creditsToUsd,
  formatCredits,
  formatUsdValue,
  rawQuotaToUsd,
  quoteFixedImageExpression,
  usdToCredits,
} from '../credits'

describe('credits currency conversion', () => {
  test('usdToCredits multiplies by 10 (1 USD = 10 Credits)', () => {
    expect(usdToCredits(1)).toBe(10)
    expect(usdToCredits(0.05)).toBeCloseTo(0.5)
    expect(usdToCredits(2.5)).toBe(25)
    expect(CREDITS_PER_USD).toBe(10)
  })

  test('creditsToUsd multiplies by 0.1 (1 Credit = $0.1)', () => {
    expect(creditsToUsd(10)).toBe(1)
    expect(creditsToUsd(0.5)).toBeCloseTo(0.05)
    expect(creditsToUsd(25)).toBe(2.5)
    expect(USD_PER_CREDIT).toBe(0.1)
  })

  test('usd ↔ credits round-trip preserves finite values', () => {
    expect(creditsToUsd(usdToCredits(1.23))).toBeCloseTo(1.23)
    expect(usdToCredits(creditsToUsd(42))).toBeCloseTo(42)
  })

  test('rawQuotaToUsd divides by quota_per_unit (default 500000)', () => {
    expect(rawQuotaToUsd(500000)).toBe(1)
    expect(rawQuotaToUsd(250000)).toBe(0.5)
    expect(rawQuotaToUsd(1_000_000, 500000)).toBe(2)
    expect(DEFAULT_QUOTA_PER_UNIT).toBe(500000)
  })

  test('rawQuotaToUsd falls back to default when quotaPerUnit is invalid', () => {
    expect(rawQuotaToUsd(500000, 0)).toBe(1)
    expect(rawQuotaToUsd(500000, -1)).toBe(1)
    expect(rawQuotaToUsd(500000, Number.NaN)).toBe(1)
  })

  test('non-finite inputs yield NaN for conversion helpers', () => {
    expect(Number.isNaN(creditsToUsd(Number.NaN))).toBe(true)
    expect(Number.isNaN(usdToCredits(Number.POSITIVE_INFINITY))).toBe(true)
    expect(Number.isNaN(rawQuotaToUsd(Number.NaN))).toBe(true)
  })
})

describe('formatCredits / formatUsdValue', () => {
  test('formatCredits strips trailing zeros and groups thousands', () => {
    expect(formatCredits(10)).toBe('10')
    expect(formatCredits(1.5)).toBe('1.5')
    expect(formatCredits(0.055)).toBe('0.055')
    expect(formatCredits(1000)).toBe('1,000')
  })

  test('formatUsdValue strips redundant trailing zeros', () => {
    expect(formatUsdValue(1)).toBe('1')
    expect(formatUsdValue(0.1)).toBe('0.1')
    expect(formatUsdValue(0.05)).toBe('0.05')
    // ≥ 1 uses up to 2 fraction digits
    expect(formatUsdValue(1.234567)).toBe('1.23')
    // tiny values keep finer precision
    expect(formatUsdValue(0.001234)).toBe('0.001234')
  })

  test('format helpers render em dash for non-finite values', () => {
    expect(formatCredits(Number.NaN)).toBe('—')
    expect(formatUsdValue(Number.POSITIVE_INFINITY)).toBe('—')
  })
})

describe('calculateSaving', () => {
  test('prefers explicitSaving when finite (including 0 for Official 1:1)', () => {
    expect(calculateSaving(10, 2, 25)).toBe(25)
    expect(calculateSaving(10, 2, 0)).toBe(0)
    expect(calculateSaving(10, 2, -5)).toBeNull()
  })

  test('derives saving percent from credits vs officialUsd when explicit is absent', () => {
    // 10 credits = $1; official $2 → 50% saving
    expect(calculateSaving(10, 2)).toBe(50)
    // 5 credits = $0.5; official $1 → 50%
    expect(calculateSaving(5, 1)).toBe(50)
  })

  test('returns 0 for official 1:1 and null when official missing or our price is higher', () => {
    expect(calculateSaving(10)).toBeNull()
    expect(calculateSaving(10, 0)).toBeNull()
    expect(calculateSaving(10, 1)).toBe(0) // same price → Official 1:1
    expect(calculateSaving(20, 1)).toBeNull() // more expensive
    expect(calculateSaving(Number.NaN, 2)).toBeNull()
  })
})

describe('fixed image expression quota projection', () => {
  test('shares native positive half rounding and the single-request quota bound', () => {
    expect(
      quoteFixedImageExpression(
        'tier("image", fixed(0.000003)) * image_count',
        1
      )?.usd
    ).toBe(0.000004)
    expect(
      quoteFixedImageExpression(
        'tier("image", fixed(4294.967294)) * image_count',
        1
      )?.usd
    ).toBe(4294.967294)
    expect(
      quoteFixedImageExpression(
        'tier("image", fixed(4294.967296)) * image_count',
        1
      )
    ).toBeNull()
  })
  test.each([Number.NaN, Number.POSITIVE_INFINITY, -1])(
    'rejects an invalid ratio without producing a quote: %s',
    (ratio) => {
      expect(
        quoteFixedImageExpression(
          'tier("image", fixed(0.01)) * image_count',
          ratio
        )
      ).toBeNull()
    }
  )
  test('keeps explicit free prices and free groups representable', () => {
    expect(
      quoteFixedImageExpression('tier("image", fixed(0)) * image_count', 1)
    ).toEqual({ usd: 0, credits: 0 })
    expect(
      quoteFixedImageExpression('tier("image", fixed(0.01)) * image_count', 0)
    ).toEqual({ usd: 0, credits: 0 })
  })
})
