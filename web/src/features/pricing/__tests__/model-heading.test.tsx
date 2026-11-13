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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { ModelHeader } from '../components/model-details'
import type { PricingModel } from '../types'

// The chart library cannot load under jsdom; the heading does not use it.
vi.mock('@visactor/react-vchart', () => ({ VChart: () => null }))

function model(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'wan-3.0',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    model_price: 0.01,
    enable_groups: ['default'],
    ...overrides,
  }
}

function renderHeader(value: PricingModel) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <ModelHeader
        model={value}
        priceRate={1}
        usdExchangeRate={1}
        showRechargePrice={false}
      />
    </QueryClientProvider>
  )
}

describe('model page heading', () => {
  it('uses the model name when the API sends no display name', () => {
    renderHeader(model())
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      /^wan-3\.0$/
    )
  })

  it('uses the display name and keeps the model name visible below it', () => {
    renderHeader(model({ display_name: 'Wan 3.0' }))
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      /^Wan 3\.0$/
    )
    expect(screen.getByText('wan-3.0', { selector: 'p' })).toBeInTheDocument()
  })

  it('falls back to the model name when the display name is blank', () => {
    renderHeader(model({ display_name: '  ' }))
    expect(screen.getByRole('heading', { level: 1 })).toHaveTextContent(
      /^wan-3\.0$/
    )
  })
})
