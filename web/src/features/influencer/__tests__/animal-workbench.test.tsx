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
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, beforeEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { InfluencerWorkbench } from '../index'

beforeEach(() => {
  sessionStorage.clear()
  localStorage.clear()
  useAuthStore.setState((state) => ({
    auth: {
      ...state.auth,
      user: { id: 1, username: 'test', role: 1 },
      accessToken: 'test-only',
    },
  }))
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (String(url).includes('taxonomy')) {
      return {
        data: {
          success: true,
          data: {
            version: 1,
            source: 'approved fixture',
            categories: [],
            category_priority: [],
            tier_group: {
              options: ['normal', 'freak', 'total', 'cats', 'dogs'].map(
                (id) => ({ id, label_en: id, visibleIn: [] })
              ),
              comingSoonImageUrls: [],
            },
            prompt_map: { tiers: {}, categories: {} },
          },
        },
      }
    }
    if (String(url).includes('pricing')) {
      return {
        data: {
          success: true,
          data: [
            {
              model_name: 'gpt-image-2.5-sunburst',
              model_price: 0.04,
              enable_groups: ['gpt-image-2.5-sunburst-image'],
              supported_endpoint_types: ['image-generation'],
              vendor_id: 1,
            },
          ],
          group_ratio: { 'gpt-image-2.5-sunburst-image': 1 },
        },
      }
    }
    if (String(url).includes('/groups')) {
      return {
        data: {
          success: true,
          data: { 'gpt-image-2.5-sunburst-image': { ratio: 1 } },
        },
      }
    }
    return { data: { success: true, data: { items: [], total: 0 } } }
  })
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const root = createRootRoute({
    component: () => (
      <QueryClientProvider client={client}>
        <InfluencerWorkbench />
      </QueryClientProvider>
    ),
  })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  return render(<RouterProvider router={router} />)
}
it('a cat type without optional traits enables creation and submits cats with an empty selection', async () => {
  const network = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { task_id: 'accepted-cat' } })
  mount()
  fireEvent.click(await screen.findByRole('button', { name: 'cats' }))
  await waitFor(() =>
    expect(screen.getByRole('button', { name: /^Generate/ })).toBeEnabled()
  )
  fireEvent.click(screen.getByRole('button', { name: /^Generate/ }))
  await waitFor(() =>
    expect(network).toHaveBeenCalledWith(
      expect.stringContaining('/sheet'),
      expect.objectContaining({
        tier: 'cats',
        selection: {},
        brief: '',
        group: 'gpt-image-2.5-sunburst-image',
      }),
      expect.anything()
    )
  )
})
