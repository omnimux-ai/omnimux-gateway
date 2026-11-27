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
import { act, fireEvent, render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, beforeAll, describe, expect, test, vi } from 'vitest'

import { QUOTA_TYPE_VALUES } from '../../constants'
import {
  getVideoEstimateRates,
  getVideoEstimatorOptions,
} from '../../lib/video-cost-estimate'
import type { PricingData, PricingModel } from '../../types'
import { VideoCostEstimator } from '../video-cost-estimator'

const wanModel: PricingModel = {
  id: 1,
  model_name: 'wan-3.0',
  quota_type: QUOTA_TYPE_VALUES.REQUEST,
  model_ratio: 1,
  completion_ratio: 1,
  model_price: 0.2,
  enable_groups: ['example'],
  contract: {
    model_id: 'wan-3.0',
    family: 'video',
    billing: {
      unit: 'per_second',
      pricing_basis: 'duration_seconds',
      unit_price_usd: 0.176471,
    },
  },
}

const seedance25Model: PricingModel = {
  id: 2,
  model_name: 'seedance-2-5',
  quota_type: QUOTA_TYPE_VALUES.REQUEST,
  model_ratio: 1,
  completion_ratio: 1,
  model_price: 0.8,
  enable_groups: ['example', 'desktop-720p'],
  contract: {
    model_id: 'seedance-2-5',
    family: 'video',
    billing: {
      unit: 'per_second',
      pricing_basis: 'duration_seconds',
      unit_price_usd: 0.8,
    },
  },
}

const magi2Model: PricingModel = {
  id: 3,
  model_name: 'magi-2',
  quota_type: QUOTA_TYPE_VALUES.REQUEST,
  model_ratio: 1,
  completion_ratio: 1,
  model_price: 0.08,
  enable_groups: ['default'],
  contract: {
    model_id: 'magi-2',
    family: 'video',
    billing: {
      unit: 'per_second',
      pricing_basis: 'duration_seconds',
      unit_price_usd: 0.08,
    },
  },
}

const tieredVideoModel: PricingModel = {
  id: 4,
  model_name: 'minimax-h3-video',
  quota_type: QUOTA_TYPE_VALUES.TOKEN,
  model_ratio: 1,
  completion_ratio: 1,
  billing_mode: 'tiered_expr',
  billing_expr:
    'u("resolution") == "1440p" ? tier("1440p", u("seconds") * 0.035) : tier("base", u("seconds") * 0.025)',
  billing_usage_schema: {
    seconds: { type: 'number', unit: 'second' },
    resolution: { enum: ['480p', '1440p'] },
  },
  enable_groups: ['default'],
  contract: {
    model_id: 'minimax-h3-video',
    family: 'video',
    billing: {
      unit: 'per_second',
      pricing_basis: 'duration_seconds',
      unit_price_usd: 0.025,
    },
  },
}

const perUsageModel: PricingModel = {
  id: 5,
  model_name: 'minimax-h3-turbo',
  quota_type: QUOTA_TYPE_VALUES.REQUEST,
  model_ratio: 1,
  completion_ratio: 1,
  model_price: 0.035,
  enable_groups: ['default'],
  contract: {
    model_id: 'minimax-h3-turbo',
    family: 'video',
    billing: {
      unit: 'per_usage',
      pricing_basis: 'reported_usage',
      unit_price_usd: 0.035,
    },
  },
}

const perVideoModel: PricingModel = {
  id: 6,
  model_name: 'muse-video',
  quota_type: QUOTA_TYPE_VALUES.REQUEST,
  model_ratio: 1,
  completion_ratio: 1,
  model_price: 0.030882,
  enable_groups: ['default'],
  contract: {
    model_id: 'muse-video',
    family: 'video',
    billing: {
      unit: 'per_video',
      pricing_basis: 'request',
      unit_price_usd: 0.030882,
    },
  },
}

const unpricedVideoModel: PricingModel = {
  id: 7,
  model_name: 'grok-imagine-video-1-5',
  quota_type: QUOTA_TYPE_VALUES.TOKEN,
  model_ratio: 1,
  completion_ratio: 1,
  enable_groups: ['default'],
  contract: {
    model_id: 'grok-imagine-video-1-5',
    family: 'video',
  },
}

const visionChatLlmModel: PricingModel = {
  id: 8,
  model_name: 'gemini-3.8-flash',
  quota_type: QUOTA_TYPE_VALUES.TOKEN,
  model_ratio: 1,
  completion_ratio: 1,
  tags: 'Text Generation',
  input_modalities: ['text', 'image', 'video'],
  output_modalities: ['text'],
  enable_groups: ['default'],
  contract: {
    model_id: 'gemini-3.8-flash',
    family: 'llm',
  },
}

const livePricing: PricingData = {
  success: true,
  data: [
    wanModel,
    seedance25Model,
    magi2Model,
    tieredVideoModel,
    perUsageModel,
    perVideoModel,
    unpricedVideoModel,
    visionChatLlmModel,
  ],
  vendors: [],
  group_ratio: {
    example: 0.5,
    default: 1,
    'desktop-720p': 1,
    'desktop-480p': 0.5,
  },
  usable_group: {
    example: 'Example',
    default: 'Standard',
    'desktop-720p': 'Desktop 720p',
    'desktop-480p': 'Desktop 480p',
  },
  supported_endpoint: {},
  auto_groups: [],
  defaults: {
    text_to_video: 'seedance-2-5',
  },
}

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    '{{amount}} credits': '{{amount}} credits',
    '{{value}} seconds': '{{value}} seconds',
    '{{model}} ({{rate}} credits/s)': '{{model}} ({{rate}} credits/s)',
    '{{model}} (from {{rate}} credits/s)':
      '{{model}} (from {{rate}} credits/s)',
    '{{model}} ({{rate}} credits/video)': '{{model}} ({{rate}} credits/video)',
    '{{model}} (from {{rate}} credits/video)':
      '{{model}} (from {{rate}} credits/video)',
    '{{model}} (quoted at run time)': '{{model}} (quoted at run time)',
    'Per video': 'Per video',
    'From {{amount}} credits': 'From {{amount}} credits',
    'From ≈ {{amount}}': 'From ≈ {{amount}}',
    'Quoted at run time': 'Quoted at run time',
  })
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('getVideoEstimatorOptions', () => {
  test('discovers all video model IDs dynamically, excludes non-video LLMs, and orders default first and unpriced last', () => {
    const options = getVideoEstimatorOptions(
      livePricing,
      'desktop-720p',
      'desktop'
    )

    expect(options.map((option) => option.id)).toEqual([
      'seedance-2-5',
      'magi-2',
      'minimax-h3-turbo',
      'minimax-h3-video',
      'muse-video',
      'wan-3.0',
      'grok-imagine-video-1-5',
    ])
    expect(
      options.find((option) => option.id === 'gemini-3.8-flash')
    ).toBeUndefined()
  })

  test('resolves selected group first, falls back to lowest usable model group, and keeps desktop tiers off web', () => {
    const fallbackPricing: PricingData = {
      ...livePricing,
      data: [
        seedance25Model,
        {
          ...wanModel,
          enable_groups: ['desktop-480p', 'default'],
        },
      ],
    }

    const desktopOptions = getVideoEstimatorOptions(
      fallbackPricing,
      'desktop-720p',
      'desktop'
    )
    expect(desktopOptions.find((o) => o.id === 'seedance-2-5')).toEqual({
      id: 'seedance-2-5',
      kind: 'per_second',
      creditsPerSecond: 8,
      isRange: false,
    })
    expect(desktopOptions.find((o) => o.id === 'wan-3.0')).toEqual({
      id: 'wan-3.0',
      kind: 'per_second',
      creditsPerSecond: 1,
      isRange: false,
    })

    const webOptions = getVideoEstimatorOptions(
      fallbackPricing,
      'default',
      'web'
    )
    expect(webOptions.find((o) => o.id === 'wan-3.0')).toEqual({
      id: 'wan-3.0',
      kind: 'per_second',
      creditsPerSecond: 2,
      isRange: false,
    })
  })

  test('derives ranged per-second, per-usage, per-video, and unpriced video options accurately', () => {
    const options = getVideoEstimatorOptions(livePricing, 'default', 'web')

    expect(options.find((o) => o.id === 'minimax-h3-video')).toEqual({
      id: 'minimax-h3-video',
      kind: 'per_second',
      creditsPerSecond: 0.25,
      isRange: true,
    })
    const turbo = options.find((o) => o.id === 'minimax-h3-turbo')
    expect(turbo).toMatchObject({
      id: 'minimax-h3-turbo',
      kind: 'per_second',
      isRange: true,
    })
    expect(
      turbo?.kind === 'per_second' ? turbo.creditsPerSecond : undefined
    ).toBeCloseTo(0.35, 6)
    expect(options.find((o) => o.id === 'muse-video')).toEqual({
      id: 'muse-video',
      kind: 'per_video',
      creditsPerVideo: 0.30882,
      isRange: false,
    })
    expect(options.find((o) => o.id === 'grok-imagine-video-1-5')).toEqual({
      id: 'grok-imagine-video-1-5',
      kind: 'unavailable',
    })
  })
})

describe('VideoCostEstimator', () => {
  test('renders a loading skeleton when catalog data is still loading and nothing when empty', () => {
    const { container, rerender } = render(
      <VideoCostEstimator options={[]} isLoading />
    )
    expect(
      container.querySelectorAll('[data-slot="skeleton"]').length
    ).toBeGreaterThan(0)

    rerender(<VideoCostEstimator options={[]} isLoading={false} />)
    expect(container).toBeEmptyDOMElement()
  })

  test('estimates a 15-second clip and updates the amount at the duration limit', () => {
    render(
      <VideoCostEstimator
        options={getVideoEstimatorOptions(
          livePricing,
          'desktop-720p',
          'desktop'
        )}
      />
    )

    expect(
      screen.getByRole('option', { name: 'seedance-2-5 (8 credits/s)' })
    ).toBeInTheDocument()
    expect(screen.getByText('120 credits')).toBeInTheDocument()
    expect(screen.getByText('≈ $12.00')).toBeInTheDocument()

    const duration = screen.getByLabelText('Video duration', {
      selector: 'input[type="range"]',
    })
    expect(duration).toHaveAttribute('min', '5')
    expect(duration).toHaveAttribute('max', '30')
    expect(duration).toHaveAttribute('step', '1')
    expect(duration).not.toBeDisabled()
    fireEvent.keyDown(duration, { key: 'End' })

    expect(screen.getByText('240 credits')).toBeInTheDocument()
    expect(screen.getByText('≈ $24.00')).toBeInTheDocument()
  })

  test('displays minimum-tier "from" estimates for multi-tier video models', () => {
    render(
      <VideoCostEstimator
        options={getVideoEstimatorOptions(livePricing, 'default', 'web')}
      />
    )

    fireEvent.change(
      screen.getByRole('combobox', { name: 'Estimator model' }),
      { target: { value: 'minimax-h3-video' } }
    )

    expect(
      screen.getByRole('option', {
        name: 'minimax-h3-video (from 0.25 credits/s)',
      })
    ).toBeInTheDocument()
    expect(screen.getByText('From 3.8 credits')).toBeInTheDocument()
    expect(screen.getByText('From ≈ $0.38')).toBeInTheDocument()
  })

  test('disables duration slider and keeps fixed quote for per-video models', () => {
    render(
      <VideoCostEstimator
        options={getVideoEstimatorOptions(livePricing, 'default', 'web')}
      />
    )

    fireEvent.change(
      screen.getByRole('combobox', { name: 'Estimator model' }),
      { target: { value: 'muse-video' } }
    )

    const duration = screen.getByLabelText('Video duration', {
      selector: 'input[type="range"]',
    })
    expect(duration).toBeDisabled()
    expect(screen.getByText('Per video')).toBeInTheDocument()
    expect(screen.getByText('0.3 credits')).toBeInTheDocument()
    expect(screen.getByText('≈ $0.03')).toBeInTheDocument()
  })

  test('disables duration slider and shows runtime quote fallback for unpriced video models', () => {
    render(
      <VideoCostEstimator
        options={getVideoEstimatorOptions(livePricing, 'default', 'web')}
      />
    )

    fireEvent.change(
      screen.getByRole('combobox', { name: 'Estimator model' }),
      { target: { value: 'grok-imagine-video-1-5' } }
    )

    const duration = screen.getByLabelText('Video duration', {
      selector: 'input[type="range"]',
    })
    expect(duration).toBeDisabled()
    expect(screen.getByText('Quoted at run time')).toBeInTheDocument()
    expect(screen.queryByText('$0.00')).toBeNull()
  })

  test('reports only the latest duration after 500 ms', () => {
    vi.useFakeTimers()
    const tracker = vi.fn()
    vi.stubGlobal('__OMNIMUX_TRACK__', tracker)
    render(
      <VideoCostEstimator
        options={getVideoEstimatorOptions(
          livePricing,
          'desktop-720p',
          'desktop'
        )}
      />
    )

    const duration = screen.getByLabelText('Video duration', {
      selector: 'input[type="range"]',
    })
    fireEvent.keyDown(duration, { key: 'ArrowRight' })
    act(() => vi.advanceTimersByTime(300))
    fireEvent.keyDown(duration, { key: 'ArrowRight' })
    act(() => vi.advanceTimersByTime(499))
    expect(tracker).not.toHaveBeenCalled()

    act(() => vi.advanceTimersByTime(1))
    expect(tracker).toHaveBeenCalledExactlyOnceWith(
      'pricing_estimator_interact',
      {
        model_id: 'seedance-2-5',
        seconds: 17,
      }
    )
  })
})

describe('getVideoEstimateRates (recommended rates table)', () => {
  test('uses the live per-second model price with one enabled-group multiplier', () => {
    const rate = getVideoEstimateRates(livePricing, 'example').find(
      (item) => item.id === 'wan-3.0'
    )
    expect(rate?.creditsPerSecond).toBeCloseTo(1, 6)
  })

  test.each([
    { ...livePricing, usable_group: {} },
    { ...livePricing, group_ratio: {} },
    { ...livePricing, data: [{ ...wanModel, enable_groups: ['another'] }] },
    { ...livePricing, data: [{ ...wanModel, model_name: 'wan-3.0-task' }] },
    {
      ...livePricing,
      data: [{ ...wanModel, contract: { model_id: 'wan-3.0' } }],
    },
    {
      ...livePricing,
      data: [
        {
          ...wanModel,
          contract: {
            model_id: 'wan-3.0',
            billing: { unit: 'per_request', pricing_basis: 'request' },
          },
        },
      ],
    },
  ])(
    'falls back when the exact live match has no usable per-second rate',
    (data) => {
      expect(
        getVideoEstimateRates(data, 'example').find(
          (rate) => rate.id === 'wan-3.0'
        )?.creditsPerSecond
      ).toBeCloseTo(1.76471, 6)
    }
  )
})
