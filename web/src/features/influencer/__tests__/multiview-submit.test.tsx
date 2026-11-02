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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppQueryClient } from '@/lib/query-client'

import { InfluencerWorkbench } from '../index'
import type { TaskRecord } from '../types'

const { get, post, toast } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  toast: { error: vi.fn(), info: vi.fn(), success: vi.fn(), warning: vi.fn() },
}))

vi.mock('@/lib/api', () => ({ api: { get, post } }))
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
vi.mock('sonner', () => ({ toast }))

/** A finished character sheet: the card the user opens before deriving a board. */
const PARENT: TaskRecord = {
  id: 1,
  task_id: 'sheet_1',
  status: 'SUCCESS',
  // The submit needs a public absolute URL: a relative artifact path cannot be
  // handed to the provider as a reference image.
  artifacts: [
    {
      key: 'image',
      content_url: 'https://omnimux.ai/api/task/sheet_1/artifacts/image',
    },
  ],
}

/** The derived board, still rendering when the dialog is already gone. */
const CHILD: TaskRecord = {
  id: 2,
  task_id: 'board_1',
  status: 'IN_PROGRESS',
  progress: '20',
}

const PRICING = {
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
    vendors: [{ id: 1, name: 'OpenAI' }],
    group_ratio: { 'gpt-image-2.5-sunburst-image': 1 },
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

const GROUPS = {
  data: {
    success: true,
    data: { 'gpt-image-2.5-sunburst-image': { ratio: 1 } },
  },
}

/** The board read back by the tracker, and the sheet list behind the grid. */
function tasksResponse(params: Record<string, unknown> | undefined) {
  const items = params?.task_id === CHILD.task_id ? [CHILD] : [PARENT]
  return { data: { success: true, data: { items, total: items.length } } }
}

function renderWorkbench() {
  return render(
    <QueryClientProvider client={createAppQueryClient()}>
      <InfluencerWorkbench />
    </QueryClientProvider>
  )
}

/**
 * Opens the preview dialog the way the user does: switch to the history
 * gallery (the workbench opens on the inspiration library) and click the sheet
 * card.
 */
async function openPreview() {
  fireEvent.click(await screen.findByRole('radio', { name: 'History' }))
  const card = await waitFor(() => {
    const found = screen
      .getAllByRole('button')
      .find((el) => el.querySelector('img') !== null)
    if (!found) throw new Error('sheet card not rendered yet')
    return found
  })
  fireEvent.click(card)
  return screen.findByRole('button', { name: 'Generate multi-view' })
}

describe('multi-view submit from the preview dialog', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    toast.error.mockReset()
    localStorage.clear()
    get.mockImplementation(
      (url: string, config?: { params?: Record<string, unknown> }) => {
        if (url.includes('/pricing')) return Promise.resolve(PRICING)
        if (url.includes('/taxonomy')) return Promise.resolve(TAXONOMY)
        if (url.includes('/groups')) return Promise.resolve(GROUPS)
        if (url.includes('/task/self')) {
          return Promise.resolve(tasksResponse(config?.params))
        }
        return Promise.resolve({ data: { success: true, data: {} } })
      }
    )
    post.mockResolvedValue({ data: { task_id: CHILD.task_id } })
  })

  it('closes the preview dialog once the board is accepted, so the progress tile is visible', async () => {
    renderWorkbench()
    await openPreview()

    fireEvent.click(screen.getByRole('button', { name: 'Generate multi-view' }))

    expect(
      await screen.findByRole('button', {
        name: 'Generating multi-view storyboard…',
      })
    ).toBeInTheDocument()
    await waitFor(() => {
      expect(
        screen.queryByRole('button', { name: 'Generate multi-view' })
      ).not.toBeInTheDocument()
    })
  })

  it('keeps the preview dialog open when the board is rejected', async () => {
    post.mockRejectedValue(new Error('gateway refused'))
    renderWorkbench()
    await openPreview()

    fireEvent.click(screen.getByRole('button', { name: 'Generate multi-view' }))

    await waitFor(() => {
      expect(toast.error).toHaveBeenCalled()
    })
    expect(
      screen.getByRole('button', { name: 'Generate multi-view' })
    ).toBeInTheDocument()
  })
})
