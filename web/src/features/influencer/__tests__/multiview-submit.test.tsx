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
const PARENT_IMAGE =
  'https://files.example/v1/tasks/sheet_1/artifacts/image/content'
const PARENT: TaskRecord = {
  id: 1,
  task_id: 'sheet_1',
  status: 'SUCCESS',
  properties: {
    input: JSON.stringify({ tier: 'normal', brief: 'A portrait' }),
  },
  // The submit needs a public absolute URL: a relative artifact path cannot be
  // handed to the provider as a reference image.
  artifacts: [
    {
      key: 'image',
      type: 'image',
      content_url: PARENT_IMAGE,
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
  group: 'gpt-image-2.5-sunburst-image',
  properties: {
    origin_model_name: 'gpt-image-2.5-sunburst',
    input: JSON.stringify({
      kind: 'multiview',
      parent_task_id: PARENT.task_id,
      image_url: PARENT_IMAGE,
      size: '16:9',
    }),
  },
}
const account = { id: 1, username: 'tester', role: 1, quota: 0 }
const IMAGE_FAILURE: TaskRecord = {
  id: 3,
  task_id: 'sheet-2',
  status: 'FAILURE',
  fail_reason: 'Insufficient user quota',
  group: 'gpt-image-2.5-sunburst-image',
  properties: {
    origin_model_name: 'gpt-image-2.5-sunburst',
    input: JSON.stringify({
      tier: 'total',
      selection: { body: ['curvy'] },
      size: '16:9',
    }),
  },
}

async function confirmTopUp(
  user: ReturnType<typeof userEvent.setup>,
  dialog: HTMLElement
) {
  const wallet = await within(dialog).findByRole('button', {
    name: 'Add credits in Wallet',
  })
  await waitFor(() => expect(wallet).toBeEnabled())
  await user.click(wallet)
  getSelf.mockResolvedValue({
    success: true,
    data: { ...account, quota: 500_000 },
  })
  await user.click(
    within(dialog).getByRole('button', { name: 'I have completed the payment' })
  )
  return screen.findByRole('dialog', { name: 'Top up successful' })
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
      {
        model_name: 'gpt-image-2.5',
        model_price: 0.02,
        enable_groups: ['gpt-image-2.5-economy'],
        supported_endpoint_types: ['image-generation'],
        vendor_id: 1,
      },
    ],
    vendors: [{ id: 1, name: 'OpenAI' }],
    group_ratio: {
      'gpt-image-2.5-sunburst-image': 1,
      'gpt-image-2.5-economy': 1,
    },
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

const GROUPS: {
  data: { success: boolean; data: Record<string, { ratio: number }> }
} = {
  data: {
    success: true,
    data: {
      'gpt-image-2.5-sunburst-image': { ratio: 1 },
      'gpt-image-2.5-economy': { ratio: 1 },
    },
  },
}

/** The board read back by the tracker, and the sheet list behind the grid. */
function tasksResponse(
  params: Record<string, unknown> | undefined,
  child: TaskRecord,
  historyChild?: TaskRecord,
  extra: TaskRecord[] = []
) {
  const history = [PARENT, ...(historyChild ? [historyChild] : []), ...extra]
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
  let extraHistory: TaskRecord[]
  let pricing: typeof PRICING
  let groups: typeof GROUPS
  let track: ReturnType<typeof vi.fn>

  beforeEach(() => {
    polledChild = CHILD
    historyChild = undefined
    extraHistory = []
    pricing = PRICING
    groups = GROUPS
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
        if (url.includes('/pricing')) return Promise.resolve(pricing)
        if (url.includes('/taxonomy')) return Promise.resolve(TAXONOMY)
        if (url.includes('/groups')) return Promise.resolve(groups)
        if (url.includes('/task/self')) {
          return Promise.resolve(
            tasksResponse(
              config?.params,
              polledChild,
              historyChild,
              extraHistory
            )
          )
        }
        return Promise.resolve({ data: { success: true, data: {} } })
      }
    )
    post.mockResolvedValue({ data: { task_id: CHILD.task_id } })
  })

  afterEach(() => {
    vi.useRealTimers()
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

  it.each(['QUEUED', 'IN_PROGRESS'])(
    'does not submit a history retry after a newer board becomes %s during payment',
    async (status) => {
      historyChild = QUOTA_FAILURE
      vi.spyOn(window, 'open').mockReturnValue(null)
      const user = userEvent.setup()
      const { client } = renderWorkbench()
      await user.click(await screen.findByRole('radio', { name: 'History' }))
      await user.click(
        await screen.findByRole('button', {
          name: 'Multi-view generation failed',
        })
      )
      const details = await screen.findByRole('dialog', { name: 'Multi-view' })
      await user.click(within(details).getByRole('button', { name: 'Top up' }))
      const paywall = await screen.findByRole('dialog', {
        name: 'Top up to continue this generation',
      })
      const wallet = await within(paywall).findByRole('button', {
        name: 'Add credits in Wallet',
      })
      await waitFor(() => expect(wallet).toBeEnabled())
      await user.click(wallet)

      extraHistory = [
        {
          ...QUOTA_FAILURE,
          id: 9,
          task_id: 'latest-board',
          status,
          fail_reason: undefined,
        },
      ]
      await act(async () => {
        await client.invalidateQueries({ queryKey: ['influencer', 'history'] })
      })
      getSelf.mockResolvedValue({
        success: true,
        data: { ...account, quota: 500_000 },
      })
      await user.click(
        within(paywall).getByRole('button', {
          name: 'I have completed the payment',
        })
      )
      const receipt = await screen.findByRole('dialog', {
        name: 'Top up successful',
      })
      const action = within(receipt).getByRole('button', {
        name: /^(Done|Continue generating)$/,
      })
      const label = action.textContent
      await user.click(action)

      expect(post).not.toHaveBeenCalled()
      expect(label).toBe('Done')
      expect(
        await screen.findByRole('dialog', { name: 'Multi-view' })
      ).toBeVisible()
    }
  )

  it.each(['regeneration', 'top-up'])(
    'blocks %s while a different board for the same character is still rendering',
    async (entry) => {
      vi.spyOn(window, 'open').mockReturnValue(null)
      const user = userEvent.setup()
      const { client } = renderWorkbench()
      await user.click(await openPreview())
      await screen.findByRole('button', {
        name: 'Generating multi-view storyboard…',
      })

      historyChild = { ...QUOTA_FAILURE, id: 9, task_id: 'newer-failed-board' }
      await act(async () => {
        await client.invalidateQueries({ queryKey: ['influencer', 'history'] })
      })
      await user.click(
        await screen.findByRole('button', {
          name: 'Multi-view generation failed',
        })
      )
      const details = await screen.findByRole('dialog', { name: 'Multi-view' })
      if (entry === 'regeneration') {
        const regenerate = within(details).getByRole('button', {
          name: 'Regenerate',
        })
        await user.click(regenerate)
        expect(post).toHaveBeenCalledTimes(1)
        expect(regenerate).toBeDisabled()
        return
      }

      await user.click(within(details).getByRole('button', { name: 'Top up' }))
      const paywall = await screen.findByRole('dialog', {
        name: /^Top up(?: to continue this generation)?$/,
      })
      const receipt = await confirmTopUp(user, paywall)
      const action = within(receipt).getByRole('button', {
        name: /^(Done|Continue generating)$/,
      })
      await user.click(action)
      expect(post).toHaveBeenCalledTimes(1)
      expect(action).toHaveTextContent('Done')
    }
  )

  it.each(['dismissal', 'payment'])(
    'presents overlapping quota failures in order after %s and tracks each only once',
    async (end) => {
      historyChild = {
        ...QUOTA_FAILURE,
        status: 'IN_PROGRESS',
        fail_reason: undefined,
      }
      extraHistory = [
        { ...IMAGE_FAILURE, status: 'QUEUED', fail_reason: undefined },
      ]
      vi.spyOn(window, 'open').mockReturnValue(null)
      const user = userEvent.setup()
      const { client } = renderWorkbench()
      await user.click(await screen.findByRole('radio', { name: 'History' }))
      await screen.findByRole('button', {
        name: 'Generating multi-view storyboard…',
      })
      const refreshHistory = () =>
        act(async () => {
          await client.invalidateQueries({
            queryKey: ['influencer', 'history'],
          })
        })
      const paywallViews = () =>
        track.mock.calls
          .filter(([event]) => event === 'influencer_paywall_view')
          .map(([, payload]) => payload.origin)
      historyChild = QUOTA_FAILURE
      await refreshHistory()
      const first = await screen.findByRole('dialog', {
        name: 'Top up to continue this generation',
      })
      extraHistory = [IMAGE_FAILURE]
      await refreshHistory()
      expect(paywallViews()).toEqual(['multiview'])

      if (end === 'payment') {
        const receipt = await confirmTopUp(user, first)
        await user.click(
          within(receipt).getByRole('button', { name: 'Continue generating' })
        )
      } else {
        await user.click(
          within(first).getByRole('button', { name: 'Back to editor' })
        )
      }
      await waitFor(() =>
        expect(paywallViews()).toEqual(['multiview', 'image'])
      )
      const second = await screen.findByRole('dialog', {
        name: 'Top up to continue this generation',
      })
      expect(second).not.toBe(first)
      await user.click(
        within(second).getByRole('button', { name: 'Back to editor' })
      )
      await refreshHistory()
      expect(paywallViews()).toEqual(['multiview', 'image'])
      expect(
        screen.queryByRole('dialog', {
          name: 'Top up to continue this generation',
        })
      ).not.toBeInTheDocument()
    }
  )

  it.each(['multiview', 'image'] as const)(
    'replays the saved %s model and group even when the default route differs',
    async (kind) => {
      const original = {
        group: 'gpt-image-2.5-economy',
        origin_model_name: 'gpt-image-2.5',
      }
      const task = kind === 'multiview' ? QUOTA_FAILURE : IMAGE_FAILURE
      const record = {
        ...task,
        group: original.group,
        properties: {
          ...task.properties,
          origin_model_name: original.origin_model_name,
        },
      }
      if (kind === 'multiview') historyChild = record
      else extraHistory = [record]
      vi.spyOn(window, 'open').mockReturnValue(null)
      const user = userEvent.setup()
      renderWorkbench()
      await user.click(await screen.findByRole('radio', { name: 'History' }))
      if (kind === 'multiview') {
        await user.click(
          await screen.findByRole('button', {
            name: 'Multi-view generation failed',
          })
        )
      }
      await user.click(await screen.findByRole('button', { name: 'Top up' }))
      const paywall = await screen.findByRole('dialog', {
        name: 'Top up to continue this generation',
      })
      expect(within(paywall).getByText('Credits needed')).toHaveTextContent(
        'Credits needed0.2'
      )
      const receipt = await confirmTopUp(user, paywall)
      await user.click(
        within(receipt).getByRole('button', { name: 'Continue generating' })
      )
      const expectedBody =
        kind === 'multiview'
          ? {
              parent_task_id: PARENT.task_id,
              image_url: PARENT_IMAGE,
              size: '16:9',
            }
          : {
              tier: 'total',
              selection: { body: ['curvy'] },
              brief: '',
              size: '16:9',
            }
      expect(post).toHaveBeenCalledExactlyOnceWith(
        `/pg/influencer/${kind === 'multiview' ? 'multiview' : 'sheet'}`,
        expect.objectContaining({
          ...expectedBody,
          model: original.origin_model_name,
          group: original.group,
        }),
        expect.objectContaining({
          headers: { 'X-Omnimux-Group': original.group },
        })
      )
    }
  )

  it.each([
    'missing model',
    'unavailable model',
    'image group revoked',
    'image carries a saved free description',
    'group delisted during payment',
    'image group revoked during payment',
    'pricing read failed during payment',
    'groups read failed during payment',
    'non-matching parent image',
    'non-image parent artifact',
  ])(
    'offers top-up only when the original historical input cannot be restored: %s',
    async (condition) => {
      const imageFailure = condition.startsWith('image ')
      const failure = imageFailure ? IMAGE_FAILURE : QUOTA_FAILURE
      const stored = {
        ...failure.properties,
        origin_model_name: 'gpt-image-2.5',
      }
      const task = {
        ...failure,
        group: 'gpt-image-2.5-economy',
        properties: stored,
      }
      const takeAwayModel = () => {
        pricing = {
          data: { ...PRICING.data, data: PRICING.data.data.slice(0, 1) },
        }
      }
      if (condition === 'missing model') stored.origin_model_name = ''
      if (condition === 'unavailable model') takeAwayModel()
      if (condition === 'image group revoked') {
        groups = {
          data: {
            ...GROUPS.data,
            data: { 'gpt-image-2.5-sunburst-image': { ratio: 1 } },
          },
        }
      }
      if (condition === 'image carries a saved free description') {
        stored.input = JSON.stringify({
          tier: 'total',
          selection: { body: ['curvy'] },
          brief: 'The original description',
          size: '16:9',
        })
      }
      if (
        condition === 'non-matching parent image' ||
        condition === 'non-image parent artifact'
      ) {
        stored.input = JSON.stringify({
          kind: 'multiview',
          parent_task_id: PARENT.task_id,
          size: '16:9',
          image_url:
            condition === 'non-matching parent image'
              ? 'https://files.example/v1/tasks/other-sheet/artifacts/image/content'
              : 'https://files.example/v1/tasks/sheet_1/artifacts/video/content',
        })
      }
      if (imageFailure) extraHistory = [task]
      else historyChild = task
      vi.spyOn(window, 'open').mockReturnValue(null)
      const user = userEvent.setup()
      renderWorkbench()
      await user.click(await screen.findByRole('radio', { name: 'History' }))
      if (!imageFailure) {
        await user.click(
          await screen.findByRole('button', {
            name: 'Multi-view generation failed',
          })
        )
      }
      await user.click(await screen.findByRole('button', { name: 'Top up' }))
      const changing = condition.endsWith('during payment')
      const automatic = condition === 'groups read failed during payment'
      const paywall = await screen.findByRole('dialog', {
        name: changing ? 'Top up to continue this generation' : 'Top up',
      })
      const wallet = await within(paywall).findByRole('button', {
        name: 'Add credits in Wallet',
      })
      await waitFor(() => expect(wallet).toBeEnabled())
      if (automatic) {
        vi.useFakeTimers()
        await act(async () => fireEvent.click(wallet))
      } else {
        await user.click(wallet)
      }
      const readsBeforePayment = get.mock.calls.length
      if (condition === 'group delisted during payment') {
        pricing = {
          data: {
            ...PRICING.data,
            data: [
              PRICING.data.data[0],
              { ...PRICING.data.data[1], enable_groups: [] },
            ],
          },
        }
      }
      if (condition === 'image group revoked during payment') {
        groups = {
          data: {
            success: true,
            data: { 'gpt-image-2.5-sunburst-image': { ratio: 1 } },
          },
        }
      }
      if (condition === 'pricing read failed during payment') {
        pricing = { data: { ...PRICING.data, success: false } }
      }
      if (automatic) groups = { data: { ...GROUPS.data, success: false } }
      getSelf.mockResolvedValue({
        success: true,
        data: { ...account, quota: 500_000 },
      })
      if (automatic) {
        await act(async () => vi.advanceTimersByTimeAsync(3_000))
        const receipt = screen.getByRole('dialog', {
          name: 'Top up successful',
        })
        expect(
          within(receipt).getByRole('button', { name: 'Done' })
        ).toBeVisible()
        await act(async () => vi.advanceTimersByTimeAsync(1_400))
        expect(receipt).not.toBeInTheDocument()
      } else {
        await user.click(
          within(paywall).getByRole('button', {
            name: 'I have completed the payment',
          })
        )
        const receipt = await screen.findByRole('dialog', {
          name: 'Top up successful',
        })
        await user.click(within(receipt).getByRole('button', { name: 'Done' }))
      }
      if (changing) {
        const reads = get.mock.calls
          .slice(readsBeforePayment)
          .map(([url]) => url)
        expect(reads).toEqual(
          expect.arrayContaining(['/api/pricing', '/api/user/self/groups'])
        )
      }
      expect(post).not.toHaveBeenCalled()
    }
  )

  it.each(['image', 'multiview'] as const)(
    'checks the original %s route again when the first paid retry is rejected for quota',
    async (kind) => {
      if (kind === 'multiview') historyChild = QUOTA_FAILURE
      else extraHistory = [IMAGE_FAILURE]
      post.mockRejectedValueOnce({
        response: {
          status: 403,
          data: {
            error: {
              code: 'insufficient_user_quota',
              message: 'Insufficient user quota',
            },
          },
        },
      })
      vi.spyOn(window, 'open').mockReturnValue(null)
      const user = userEvent.setup()
      renderWorkbench()
      await user.click(await screen.findByRole('radio', { name: 'History' }))
      if (kind === 'multiview') {
        await user.click(
          await screen.findByRole('button', {
            name: 'Multi-view generation failed',
          })
        )
      }
      await user.click(await screen.findByRole('button', { name: 'Top up' }))
      const topUp = { name: 'Top up to continue this generation' }
      const first = await screen.findByRole('dialog', topUp)
      const firstReceipt = await confirmTopUp(user, first)
      await user.click(
        within(firstReceipt).getByRole('button', {
          name: 'Continue generating',
        })
      )
      const reopened = await screen.findByRole('dialog', topUp)
      expect(reopened).not.toBe(first)
      expect(post).toHaveBeenCalledTimes(1)
      expect(post.mock.calls[0][0]).toBe(
        kind === 'multiview'
          ? '/pg/influencer/multiview'
          : '/pg/influencer/sheet'
      )

      const wallet = await within(reopened).findByRole('button', {
        name: 'Add credits in Wallet',
      })
      await waitFor(() => expect(wallet).toBeEnabled())
      if (kind === 'multiview') {
        vi.useFakeTimers()
        await act(async () => fireEvent.click(wallet))
        groups = { data: { ...GROUPS.data, success: false } }
      } else {
        await user.click(wallet)
        pricing = {
          data: { ...PRICING.data, data: PRICING.data.data.slice(1) },
        }
      }
      const beforeConfirmation = get.mock.calls.length
      getSelf.mockResolvedValue({
        success: true,
        data: { ...account, quota: 1_000_000 },
      })
      const success = { name: 'Top up successful' }
      const receiptAction = { name: /^(Continue generating|Done)$/ }
      let label: string | null
      if (kind === 'multiview') {
        await act(async () => vi.advanceTimersByTimeAsync(3_000))
        const receipt = screen.getByRole('dialog', success)
        label = within(receipt).getByRole('button', receiptAction).textContent
        await act(async () => vi.advanceTimersByTimeAsync(1_400))
        expect(receipt).not.toBeInTheDocument()
      } else {
        await user.click(
          within(reopened).getByRole('button', {
            name: 'I have completed the payment',
          })
        )
        const receipt = await screen.findByRole('dialog', success)
        const action = within(receipt).getByRole('button', receiptAction)
        label = action.textContent
        await user.click(action)
      }
      expect(post).toHaveBeenCalledTimes(1)
      expect(label).toBe('Done')
      const reads = get.mock.calls.slice(beforeConfirmation).map(([url]) => url)
      expect(reads).toEqual(
        expect.arrayContaining(['/api/pricing', '/api/user/self/groups'])
      )
    }
  )

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
