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

import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppQueryClient } from '@/lib/query-client'

import { InfluencerWorkbench } from '../index'

/**
 * Renders the workbench route exactly as it ships. The page is assembled from
 * several merges, so a symbol that survives a call site without its import
 * only fails when this component actually runs — typecheck catches it first,
 * and this test catches it in the built route as well.
 */
const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: { get, post: vi.fn() } }))
vi.mock('@tanstack/react-router', () => ({
  useSearch: () => ({}),
  useNavigate: () => vi.fn(),
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (select: (state: unknown) => unknown) =>
    select({
      auth: { user: { id: 1, username: 'tester' }, accessToken: 'test-token' },
    }),
}))
vi.mock('sonner', () => ({
  toast: { error: vi.fn(), info: vi.fn(), success: vi.fn() },
}))

/** One advertised image model, so the workbench's auto-pick effect runs. */
const PRICING = {
  data: {
    success: true,
    data: [
      {
        model_name: 'gpt-image-2.5-sunburst',
        model_price: 0.04,
        enable_groups: ['default', 'economy'],
        supported_endpoint_types: ['image-generation'],
        vendor_id: 1,
      },
    ],
    vendors: [{ id: 1, name: 'OpenAI' }],
    group_ratio: { default: 1, economy: 0.6 },
  },
}

const TAXONOMY = {
  data: {
    success: true,
    data: {
      version: 1,
      source: 'test',
      tier_group: { options: [], comingSoonImageUrls: [] },
      category_priority: [],
      categories: [],
      prompt_map: { tiers: {}, categories: {} },
    },
    etag: 'test',
  },
}

const GROUPS = { data: { success: true, data: { default: { ratio: 1 } } } }
const TASKS = { data: { success: true, data: { items: [], total: 0 } } }

function routeByUrl(url: string) {
  if (url.includes('/pricing')) return PRICING
  if (url.includes('/taxonomy')) return TAXONOMY
  if (url.includes('/groups')) return GROUPS
  if (url.includes('/task/self')) return TASKS
  return { data: { success: true, data: {} } }
}

function renderWorkbench() {
  return render(
    <QueryClientProvider client={createAppQueryClient()}>
      <InfluencerWorkbench />
    </QueryClientProvider>
  )
}

describe('InfluencerWorkbench', () => {
  beforeEach(() => {
    get.mockReset()
    get.mockImplementation((url: string) => Promise.resolve(routeByUrl(url)))
    localStorage.clear()
  })

  it('renders skeleton while taxonomy is in flight, avoiding false empty state', () => {
    // Hang taxonomy request so it stays pending
    get.mockImplementation((url: string) => {
      if (url.includes('/taxonomy')) {
        return new Promise(() => {})
      }
      return Promise.resolve(routeByUrl(url))
    })

    const { container } = renderWorkbench()

    // Must show skeleton status role and shimmers, NOT "No presets found"
    expect(screen.getByRole('status', { name: /loading/i })).toBeInTheDocument()
    expect(container.querySelectorAll('.inf-shimmer')).not.toHaveLength(0)
    expect(screen.queryByText('No presets found')).not.toBeInTheDocument()
  })

  it('renders empty state when taxonomy request fails', async () => {
    get.mockImplementation((url: string) => {
      if (url.includes('/taxonomy')) {
        return Promise.reject(new Error('Network error'))
      }
      return Promise.resolve(routeByUrl(url))
    })

    renderWorkbench()

    expect(await screen.findByText('No presets found')).toBeInTheDocument()
  })
})
