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
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppQueryClient } from '@/lib/query-client'
import { useAuthStore } from '@/stores/auth-store'

import { InfluencerWorkbench } from '../index'
import type { TaskRecord } from '../types'

const { get, getSelf, post, toast } = vi.hoisted(() => ({
  get: vi.fn(),
  getSelf: vi.fn(),
  post: vi.fn(),
  toast: { error: vi.fn(), info: vi.fn(), success: vi.fn(), warning: vi.fn() },
}))

vi.mock('@/lib/api', () => ({
  api: { get, post },
  getSelf,
  getStatus: async () => ({ success: true, data: {} }),
}))
vi.mock('@tanstack/react-router', () => ({
  useSearch: () => ({}),
  useNavigate: () => vi.fn(),
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
      content_url: 'https://files.example/characters/sheet.png',
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

const QUOTA_FAILURE: TaskRecord = {
  ...CHILD,
  status: 'FAILURE',
  progress: '100%',
  fail_reason: 'Insufficient user quota',
  properties: {
    input: JSON.stringify({
      kind: 'multiview',
      parent_task_id: PARENT.task_id,
    }),
  },
}
const account = { id: 1, username: 'tester', role: 1, quota: 0 }

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
function tasksResponse(
  params: Record<string, unknown> | undefined,
  child: TaskRecord,
  historyChild?: TaskRecord
) {
  const history = historyChild ? [PARENT, historyChild] : [PARENT]
  const items = params?.task_id === CHILD.task_id ? [child] : history
  return { data: { success: true, data: { items, total: items.length } } }
}

function renderWorkbench() {
  const client = createAppQueryClient()
  return {
    ...render(
      <QueryClientProvider client={client}>
        <InfluencerWorkbench />
      </QueryClientProvider>
    ),
    client,
  }
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
  let polledChild: TaskRecord
  let historyChild: TaskRecord | undefined
  let track: ReturnType<typeof vi.fn>

  beforeEach(() => {
    polledChild = CHILD
    historyChild = undefined
    track = vi.fn()
    vi.stubGlobal('__OMNIMUX_TRACK__', track)
    useAuthStore.setState((state) => ({
      auth: { ...state.auth, user: account, accessToken: 'fixture-session' },
    }))
    getSelf.mockResolvedValue({ success: true, data: account })
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
          return Promise.resolve(
            tasksResponse(config?.params, polledChild, historyChild)
          )
        }
        return Promise.resolve({ data: { success: true, data: {} } })
      }
    )
    post.mockResolvedValue({ data: { task_id: CHILD.task_id } })
  })

  afterEach(() => {
    vi.unstubAllGlobals()
    vi.restoreAllMocks()
    useAuthStore.getState().auth.reset()
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

  it('opens the quoted MultiView paywall on HTTP 403 and resumes the same request once after payment', async () => {
    post.mockRejectedValueOnce({
      response: {
        status: 403,
        data: {
          error: {
            message: 'Insufficient user quota',
            code: 'insufficient_user_quota',
          },
        },
      },
    })
    const open = vi.spyOn(window, 'open').mockReturnValue(null)
    const user = userEvent.setup()
    renderWorkbench()
    await user.click(await openPreview())

    const paywall = await screen.findByRole('dialog', {
      name: 'Top up to continue this generation',
    })
    expect(within(paywall).getByText('Balance')).toHaveTextContent('Balance0')
    expect(within(paywall).getByText('Credits needed')).toHaveTextContent(
      'Credits needed0.4'
    )
    expect(post).toHaveBeenCalledTimes(1)
    expect(post.mock.calls[0]?.[0]).toBe('/pg/influencer/multiview')
    expect(toast.error).not.toHaveBeenCalled()
    expect(
      track.mock.calls.filter(([event]) => event === 'influencer_paywall_view')
    ).toEqual([
      [
        'influencer_paywall_view',
        { origin: 'multiview', required_credits: 0.4 },
      ],
    ])

    await user.click(
      await within(paywall).findByRole('button', {
        name: 'Add credits in Wallet',
      })
    )
    expect(open).toHaveBeenCalledWith('/wallet', '_blank', 'noopener')
    getSelf.mockResolvedValue({
      success: true,
      data: { ...account, quota: 500_000 },
    })
    await user.click(
      within(paywall).getByRole('button', {
        name: 'I have completed the payment',
      })
    )
    await user.click(
      await within(paywall).findByRole('button', {
        name: 'Continue generating',
      })
    )
    expect(
      await screen.findByRole('button', {
        name: 'Generating multi-view storyboard…',
      })
    ).toBeVisible()
    expect(post.mock.calls).toEqual([post.mock.calls[0], post.mock.calls[0]])
    expect(
      track.mock.calls.filter(
        ([event]) => event === 'influencer_paywall_subscribed'
      )
    ).toEqual([['influencer_paywall_subscribed', { origin: 'multiview' }]])
  })

  it('opens the MultiView paywall once for the recorded dispatch reason even when the wallet covers the quote', async () => {
    const fundedAccount = { ...account, quota: 1_000_000 }
    useAuthStore.getState().auth.setUser(fundedAccount)
    getSelf.mockResolvedValue({ success: true, data: fundedAccount })
    polledChild = QUOTA_FAILURE
    historyChild = QUOTA_FAILURE
    const user = userEvent.setup()
    renderWorkbench()
    await user.click(await openPreview())

    const paywall = await screen.findByRole('dialog', {
      name: 'Top up to continue this generation',
    })
    expect(within(paywall).getByText('Balance')).toHaveTextContent('Balance20')
    await user.click(
      within(paywall).getByRole('button', { name: 'Back to editor' })
    )
    await screen.findByRole('button', { name: 'Multi-view generation failed' })
    expect(
      screen.queryByRole('dialog', {
        name: 'Top up to continue this generation',
      })
    ).not.toBeInTheDocument()
    expect(
      track.mock.calls.filter(([event]) => event === 'influencer_paywall_view')
    ).toEqual([
      [
        'influencer_paywall_view',
        { origin: 'multiview', required_credits: 0.4 },
      ],
    ])
    expect(post).toHaveBeenCalledTimes(1)
    expect(toast.error).not.toHaveBeenCalled()
  })

  it('resumes the board after history confirms a quota failure before the individual poll', async () => {
    vi.spyOn(window, 'open').mockReturnValue(null)
    post.mockResolvedValueOnce({ data: { task_id: CHILD.task_id } })
    post.mockResolvedValueOnce({ data: { task_id: 'board-retried' } })
    const user = userEvent.setup()
    const { client } = renderWorkbench()
    await user.click(await openPreview())
    await screen.findByRole('button', {
      name: 'Generating multi-view storyboard…',
    })

    historyChild = QUOTA_FAILURE
    await act(async () => {
      await client.invalidateQueries({ queryKey: ['influencer', 'history'] })
    })
    const paywall = await screen.findByRole('dialog', {
      name: 'Top up to continue this generation',
    })
    const wallet = await within(paywall).findByRole('button', {
      name: 'Add credits in Wallet',
    })
    await waitFor(() => expect(wallet).toBeEnabled())
    await user.click(wallet)
    getSelf.mockResolvedValue({
      success: true,
      data: { ...account, quota: 500_000 },
    })
    await user.click(
      within(paywall).getByRole('button', {
        name: 'I have completed the payment',
      })
    )
    await user.click(
      await within(paywall).findByRole('button', {
        name: 'Continue generating',
      })
    )

    await waitFor(() => expect(post).toHaveBeenCalledTimes(2))
    expect(post.mock.calls).toEqual([post.mock.calls[0], post.mock.calls[0]])
    expect(
      track.mock.calls.filter(([event]) => event === 'influencer_paywall_view')
    ).toHaveLength(1)
    expect(
      track.mock.calls.filter(
        ([event]) => event === 'influencer_paywall_subscribed'
      )
    ).toEqual([['influencer_paywall_subscribed', { origin: 'multiview' }]])
  })

  it('shows the quota recovery action inside the saved MultiView details', async () => {
    historyChild = QUOTA_FAILURE
    const user = userEvent.setup()
    renderWorkbench()
    await user.click(await screen.findByRole('radio', { name: 'History' }))
    await user.click(
      await screen.findByRole('button', {
        name: 'Multi-view generation failed',
      })
    )
    const details = await screen.findByRole('dialog', { name: 'Multi-view' })
    expect(within(details).getByText('Insufficient user quota')).toBeVisible()
    await user.click(within(details).getByRole('button', { name: 'Top up' }))
    expect(
      await screen.findByRole('dialog', {
        name: 'Top up to continue this generation',
      })
    ).toBeVisible()
    await user.keyboard('{Escape}')
    expect(
      await screen.findByRole('dialog', { name: 'Multi-view' })
    ).toBeVisible()
    expect(post).not.toHaveBeenCalled()
  })
})
