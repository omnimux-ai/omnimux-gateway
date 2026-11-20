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
import { getVideoEstimateRates } from '../../lib/video-cost-estimate'
import type { PricingData, PricingModel } from '../../types'
import { VideoCostEstimator } from '../video-cost-estimator'

const model: PricingModel = {
  id: 1,
  model_name: 'wan-3.0',
  quota_type: QUOTA_TYPE_VALUES.REQUEST,
  model_ratio: 1,
  completion_ratio: 1,
  model_price: 0.2,
  enable_groups: ['example'],
  contract: {
    model_id: 'wan-3.0',
    billing: {
      unit: 'per_second',
      pricing_basis: 'duration_seconds',
      unit_price_usd: 0.176471,
    },
  },
}

const livePricing: PricingData = {
  success: true,
  data: [model],
  vendors: [],
  group_ratio: { example: 0.5 },
  usable_group: { example: 'Example' },
  supported_endpoint: {},
  auto_groups: [],
}

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    '{{amount}} credits': '{{amount}} credits',
    '{{value}} seconds': '{{value}} seconds',
    '{{model}} · {{rate}} credits/s': '{{model}} · {{rate}} credits/s',
  })
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

describe('VideoCostEstimator', () => {
  test('estimates a 15-second clip and updates the amount at the duration limit', () => {
    render(
      <VideoCostEstimator rates={getVideoEstimateRates(undefined, undefined)} />
    )

    expect(screen.getByText('120 credits')).toBeInTheDocument()
    expect(screen.getByText('≈ $12.00')).toBeInTheDocument()

    // The layout-free DOM keeps Base UI's inset thumb visually hidden.
    const duration = screen.getByLabelText('Video duration', {
      selector: 'input[type="range"]',
    })
    expect(duration).toHaveAttribute('min', '5')
    expect(duration).toHaveAttribute('max', '30')
    expect(duration).toHaveAttribute('step', '1')
    fireEvent.keyDown(duration, { key: 'End' })

    expect(screen.getByText('240 credits')).toBeInTheDocument()
    expect(screen.getByText('≈ $24.00')).toBeInTheDocument()
  })

  test('keeps the published fallback rate precise until displaying the estimate', () => {
    render(
      <VideoCostEstimator rates={getVideoEstimateRates(undefined, 'example')} />
    )
    fireEvent.change(
      screen.getByRole('combobox', { name: 'Estimator model' }),
      {
        target: { value: 'wan-3.0' },
      }
    )

    expect(screen.getByText('Wan 3.0 · 1.76 credits/s')).toBeInTheDocument()
    expect(screen.getByText('26.5 credits')).toBeInTheDocument()
    expect(screen.getByText('≈ $2.65')).toBeInTheDocument()
  })

  test('uses the live per-second model price with one enabled-group multiplier', () => {
    render(
      <VideoCostEstimator
        rates={getVideoEstimateRates(livePricing, 'example')}
      />
    )
    fireEvent.change(
      screen.getByRole('combobox', { name: 'Estimator model' }),
      {
        target: { value: 'wan-3.0' },
      }
    )

    expect(screen.getByText('15 credits')).toBeInTheDocument()
    expect(screen.getByText('≈ $1.50')).toBeInTheDocument()
  })

  test.each([
    { ...livePricing, usable_group: {} },
    { ...livePricing, group_ratio: {} },
    { ...livePricing, data: [{ ...model, enable_groups: ['another'] }] },
    { ...livePricing, data: [{ ...model, model_name: 'wan-3.0-task' }] },
    { ...livePricing, data: [{ ...model, contract: { model_id: 'wan-3.0' } }] },
    {
      ...livePricing,
      data: [
        {
          ...model,
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

  test('reports only the latest duration after 500 ms', () => {
    vi.useFakeTimers()
    const tracker = vi.fn()
    vi.stubGlobal('__OMNIMUX_TRACK__', tracker)
    render(
      <VideoCostEstimator rates={getVideoEstimateRates(undefined, undefined)} />
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
