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
  buildOfficialPriceMap,
  normalizeOfficialBrandModelId,
  resolveOfficialUsdForTier,
} from '../official-brand-prices'

describe('normalizeOfficialBrandModelId', () => {
  test('maps dated / preview catalog aliases onto ledger ids', () => {
    expect(normalizeOfficialBrandModelId('gpt-image-1')).toBe(
      'gpt-image-1-preview'
    )
    expect(normalizeOfficialBrandModelId('claude-3-5-sonnet-20241022')).toBe(
      'claude-3-5-sonnet'
    )
    expect(normalizeOfficialBrandModelId('kling-v1.6-pro')).toBe(
      'kling-v1.5-pro'
    )
    expect(normalizeOfficialBrandModelId('suno-v4')).toBe('suno-v3.5')
  })
})

describe('resolveOfficialUsdForTier', () => {
  test('returns per-spec token lane prices in USD / 1M', () => {
    expect(resolveOfficialUsdForTier('gpt-4o', 'Input')).toBe(2.5)
    expect(resolveOfficialUsdForTier('gpt-4o', 'Output')).toBe(10)
    expect(resolveOfficialUsdForTier('deepseek-r1', 'Cache')).toBe(0.14)
    expect(resolveOfficialUsdForTier('gemini-3.8-flash', 'Input')).toBe(0.75)
    expect(resolveOfficialUsdForTier('gemini-3.8-flash', 'Output')).toBe(3)
  })

  test('falls back to primary usd for request models', () => {
    expect(resolveOfficialUsdForTier('mj-v6.1', 'Per request')).toBe(0.05)
    expect(resolveOfficialUsdForTier('flux-1-pro')).toBe(0.055)
    expect(resolveOfficialUsdForTier('minimax-h3-turbo', 'Per request')).toBe(0.04)
    expect(resolveOfficialUsdForTier('minimax-h3', 'Per request')).toBe(0.08)
    expect(resolveOfficialUsdForTier('gpt-image-2.5', 'Per request')).toBe(0.04)
    expect(resolveOfficialUsdForTier('gpt-image-2.5-flare', 'Per request')).toBe(0.04)
    expect(resolveOfficialUsdForTier('gpt-image-2.5-sunburst', 'Per request')).toBe(0.04)
    expect(resolveOfficialUsdForTier('seedance-2-5-task', 'Per request')).toBe(3.088)
    expect(resolveOfficialUsdForTier('seedance-2-0-task', 'Per request')).toBe(0.993)
  })
})

describe('buildOfficialPriceMap', () => {
  test('includes aliases and merges caller overrides', () => {
    const map = buildOfficialPriceMap({ 'custom-model': 1.23 })
    expect(map['gpt-4o']).toBe(2.5)
    expect(map['gpt-image-1']).toBe(0.04)
    expect(map['claude-3-5-sonnet-20241022']).toBe(3)
    expect(map['custom-model']).toBe(1.23)
    expect(map['minimax-h3-turbo']).toBe(0.04)
    expect(map['minimax-h3']).toBe(0.08)
    expect(map['gpt-image-2.5']).toBe(0.04)
    expect(map['seedance-2-5-task']).toBe(3.088)
    expect(map['seedance-2-0-task']).toBe(0.993)
  })
})
