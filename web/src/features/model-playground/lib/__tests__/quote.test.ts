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

import type { PricingModel } from '@/features/pricing/types'

import { buildParamSchema, defaultParams } from '../param-schema'
import { estimateQuote } from '../quote'

function model(overrides: Partial<PricingModel>): PricingModel {
  return {
    id: 1,
    model_name: 'test-model',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

describe('estimateQuote', () => {
  test('prices a fixed per-call image model from model_price', () => {
    const subject = model({
      model_name: 'gpt-image-flat',
      tags: 'Image Generation',
      model_price: 0.015,
    })
    const quote = estimateQuote(subject, {}, buildParamSchema(subject))
    expect(quote?.usd).toBeCloseTo(0.015, 6)
    expect(quote?.credits).toBeCloseTo(0.15, 6)
  })

  test('prices a per-call video model from its flat price instead of showing a placeholder', () => {
    const subject = model({
      model_name: 'seedance-2-5-task',
      tags: 'Video Generation,task',
      model_price: 0.558824,
    })
    const schema = buildParamSchema(subject)
    const quote = estimateQuote(subject, defaultParams(schema), schema)

    expect(quote?.usd).toBeCloseTo(0.558824, 6)
    expect(quote?.credits).toBeCloseTo(5.58824, 5)
  })

  test('keeps the flat price for a per-call video model whose length is unestablished', () => {
    const subject = model({
      model_name: 'grok-imagine-video-1-5-task',
      tags: 'Video Generation,task',
      model_price: 0.228571,
    })
    const schema = buildParamSchema(subject)
    const quote = estimateQuote(subject, defaultParams(schema), schema)

    expect(quote?.usd).toBeCloseTo(0.228571, 6)
  })

  test('shows no placeholder price for a per-call video model with nothing configured', () => {
    const subject = model({
      model_name: 'seedance-2-0-task',
      tags: 'Video Generation,task',
    })
    const schema = buildParamSchema(subject)
    expect(estimateQuote(subject, defaultParams(schema), schema)).toBeNull()
  })

  test('prices a per-second video model from the tiered expression and the chosen duration', () => {
    const subject = model({
      model_name: 'seedance-flat',
      tags: 'Video Generation,async,task',
      billing_mode: 'tiered_expr',
      billing_expr: 'tier("720p", u("duration") * 0.138)',
      billing_usage_schema: { duration: { type: 'number', unit: 'second' } },
    })
    const schema = buildParamSchema(subject)
    const params = { ...defaultParams(schema), duration: '5' }
    const quote = estimateQuote(subject, params, schema)
    expect(quote?.usd).toBeCloseTo(0.69, 4)
    expect(quote?.credits).toBeCloseTo(6.9, 4)
  })

  test('returns null for a duration-billed video model that only carries a flat price', () => {
    const subject = model({
      model_name: 'seedance-unknown',
      tags: 'Video Generation,async,task',
      model_price: 0.13,
    })
    const schema = buildParamSchema(subject)
    expect(estimateQuote(subject, defaultParams(schema), schema)).toBeNull()
  })

  test('returns null for a token-billed model with no usage schema', () => {
    const subject = model({ model_name: 'chat', quota_type: 0, model_ratio: 1 })
    const schema = buildParamSchema(subject)
    expect(estimateQuote(subject, defaultParams(schema), schema)).toBeNull()
  })

  test('prices wan-3.0 from the catalog base rate, the tier ratio and the duration', () => {
    const subject = model({
      model_name: 'wan-3.0',
      tags: 'Video Generation,async,task',
      model_price: 0.176471,
    })
    const schema = buildParamSchema(subject)

    // 720p tier: 0.176471 USD/s × 0.5 = 0.0882355 USD/s → 0.4411775 USD / 5s.
    const hd = estimateQuote(
      subject,
      { duration: '5', resolution: '720p' },
      schema,
      0.5
    )
    expect(hd?.usd).toBeCloseTo(0.4411775, 7)
    expect(hd?.credits).toBeCloseTo(4.411775, 6)

    // 1080p tier: 0.176471 USD/s × 1.5 = 0.2647065 USD/s → 1.3235325 USD / 5s.
    const fullHd = estimateQuote(
      subject,
      { duration: '5', resolution: '1080p' },
      schema,
      1.5
    )
    expect(fullHd?.usd).toBeCloseTo(1.3235325, 7)
    expect(fullHd?.credits).toBeCloseTo(13.235325, 6)
  })

  test('keeps the wan-3.0 base rate when the request selects no tier group', () => {
    const subject = model({
      model_name: 'wan-3.0',
      tags: 'Video Generation,async,task',
      model_price: 0.176471,
    })
    const schema = buildParamSchema(subject)

    // No tier resolved, so the quote is the model's own base rate: the tier
    // multiplier is never invented here.
    const quote = estimateQuote(
      subject,
      { duration: '5', resolution: '480p' },
      schema,
      1
    )
    expect(quote?.usd).toBeCloseTo(0.882355, 6)
    expect(quote?.credits).toBeCloseTo(8.82355, 5)
  })

  test('returns null for wan-3.0 when the catalog carries no base rate', () => {
    const subject = model({
      model_name: 'wan-3.0',
      tags: 'Video Generation,async,task',
    })
    const schema = buildParamSchema(subject)
    expect(
      estimateQuote(subject, { duration: '5', resolution: '720p' }, schema, 0.5)
    ).toBeNull()
  })

  test('returns null when the expression cannot be parsed instead of inventing a price', () => {
    const subject = model({
      model_name: 'broken',
      tags: 'Video Generation,async,task',
      billing_mode: 'tiered_expr',
      billing_expr: 'tier("x", ???)',
      billing_usage_schema: { duration: { type: 'number', unit: 'second' } },
    })
    const schema = buildParamSchema(subject)
    expect(estimateQuote(subject, { duration: '5' }, schema)).toBeNull()
  })

  test('scales the quote when groupRatio multiplier is supplied', () => {
    const subject = model({
      model_name: 'seedance-flat',
      tags: 'Video Generation,async,task',
      billing_mode: 'tiered_expr',
      billing_expr: 'tier("720p", u("duration") * 0.138)',
      billing_usage_schema: { duration: { type: 'number', unit: 'second' } },
    })
    const schema = buildParamSchema(subject)
    const params = { ...defaultParams(schema), duration: '5' }
    // Base is 0.69 USD, with groupRatio 0.5 should be 0.345 USD
    const quote = estimateQuote(subject, params, schema, 0.5)
    expect(quote?.usd).toBeCloseTo(0.345, 4)
    expect(quote?.credits).toBeCloseTo(3.45, 4)
  })
})
