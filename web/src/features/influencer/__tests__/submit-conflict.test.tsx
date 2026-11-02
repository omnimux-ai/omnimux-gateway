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

import { existsSync, readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'

import { QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppQueryClient } from '@/lib/query-client'

import presetSnapshot from '../data/presets.json'
import { InfluencerWorkbench } from '../index'
import { conflictMessage } from '../lib/taxonomy'
import type { InfluencerTaxonomy, Selection, TaskRecord } from '../types'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
const auth = vi.hoisted(() => ({
  quota: undefined as number | undefined,
  setUser: vi.fn(),
}))

vi.mock('@/lib/api', () => ({ api: { get, post } }))
vi.mock('@tanstack/react-router', () => ({
  useSearch: () => ({}),
  useNavigate: () => vi.fn(),
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (select: (state: unknown) => unknown) =>
    select({
      auth: {
        user: {
          id: 1,
          username: 'tester',
          quota: auth.quota,
        },
        accessToken: 'test-token',
        setUser: auth.setUser,
      },
    }),
}))
vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
    info: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
  },
}))

/** The taxonomy the gateway serves, so the Explore grid renders the real set. */
function servedTaxonomy(): InfluencerTaxonomy {
  let dir = process.cwd()
  for (let depth = 0; depth < 4; depth += 1) {
    const candidate = join(dir, 'service/influencer/data/taxonomy.json')
    if (existsSync(candidate)) {
      return JSON.parse(readFileSync(candidate, 'utf8')) as InfluencerTaxonomy
    }
    dir = dirname(dir)
  }
  throw new Error('service/influencer/data/taxonomy.json not found')
}

const PRICING = {
  data: {
    success: true,
    data: [
      {
        model_name: 'gpt-image-2.5-sunburst',
        model_price: 0.04,
        enable_groups: ['default', 'economy', 'gpt-image-2.5-sunburst-image'],
        supported_endpoint_types: ['image-generation'],
        vendor_id: 1,
      },
    ],
    vendors: [{ id: 1, name: 'OpenAI' }],
    group_ratio: {
      default: 1,
      economy: 0.6,
      'gpt-image-2.5-sunburst-image': 0.6,
    },
  },
}

const GROUPS = {
  data: {
    success: true,
    data: {
      default: { ratio: 1 },
      'gpt-image-2.5-sunburst-image': { ratio: 0.6 },
    },
  },
}
const TASKS = { data: { success: true, data: { items: [], total: 0 } } }

function routeByUrl(url: string) {
  if (url.includes('/pricing')) return PRICING
  if (url.includes('/taxonomy')) {
    return { data: { success: true, data: servedTaxonomy(), etag: 'test' } }
  }
  if (url.includes('/groups')) return GROUPS
  if (url.includes('/task/self')) return TASKS
  return { data: { success: true, data: {} } }
}

/** The mirrored preset that reproduces the report: Muscular + Short legs, Extreme tier. */
const CONFLICTING_PRESET = 'AI Influencer 13'

interface SheetBody {
  tier: string
  selection: Selection
  size: string
  brief: string
}

describe('applying an inspiration preset and generating', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockImplementation((url: string) => Promise.resolve(routeByUrl(url)))
    post.mockResolvedValue({ data: { task_id: 'task-1' } })
    auth.quota = undefined
    localStorage.clear()
    sessionStorage.clear()
  })

  it.each([true, false])(
    'keeps the correct mobile workspace after submission acceptance=%s',
    async (accepted) => {
      const user = userEvent.setup()
      get.mockImplementation(
        (url: string, config?: { params?: Record<string, unknown> }) => {
          if (config?.params?.task_id) {
            return Promise.resolve({
              data: {
                success: true,
                data: {
                  items: [
                    {
                      id: 1,
                      task_id: 'task-1',
                      status: 'SUCCESS',
                      action: 'influencer_sheet',
                    },
                  ],
                  total: 1,
                },
              },
            })
          }
          return Promise.resolve(routeByUrl(url))
        }
      )
      if (!accepted) post.mockRejectedValue(new Error('Fixture rejection'))
      render(
        <QueryClientProvider client={createAppQueryClient()}>
          <InfluencerWorkbench />
        </QueryClientProvider>
      )
      await user.click(await screen.findByRole('button', { name: /Bold/ }))
      await user.click(screen.getByRole('button', { name: /Randomize/ }))
      const generate = await screen.findByRole('button', { name: /Generate/ })
      await waitFor(() => expect(generate).toBeEnabled())
      await user.click(generate)
      await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
      const nav = screen.getByRole('navigation', { name: 'Workspace' })
      await waitFor(() =>
        expect(
          within(nav).getByRole('button', {
            name: accepted ? 'History' : 'Create',
          })
        ).toHaveAttribute('aria-pressed', 'true')
      )
    }
  )

  it.each(['9:16', '1024x1536', '2048x1152'])(
    'refills a legacy %s sheet without submitting until Generate is explicitly clicked',
    async (size) => {
      const user = userEvent.setup()
      const selection: Selection = { gender: ['female'] }
      const task: TaskRecord = {
        id: 2,
        task_id: 'legacy-sheet',
        status: 'SUCCESS',
        action: 'influencer_sheet',
        properties: {
          input: JSON.stringify({
            tier: 'normal',
            selection,
            brief: 'red coat',
            seed: 42,
            image_url: 'https://example.com/reference.png',
            size,
          }),
        },
        artifacts: [
          {
            key: 'image',
            content_url: '/api/task/legacy-sheet/artifacts/image',
          },
        ],
      }
      const originalInput = task.properties?.input
      get.mockImplementation(
        (url: string, config?: { params?: Record<string, unknown> }) => {
          if (url.includes('/task/self')) {
            const items =
              config?.params?.action === 'influencer_motion' ? [] : [task]
            return Promise.resolve({
              data: { success: true, data: { items, total: items.length } },
            })
          }
          return Promise.resolve(routeByUrl(url))
        }
      )
      render(
        <QueryClientProvider client={createAppQueryClient()}>
          <InfluencerWorkbench />
        </QueryClientProvider>
      )
      await user.click(await screen.findByRole('radio', { name: 'History' }))
      const card = await waitFor(() => {
        const found = screen
          .getAllByRole('button')
          .find(
            (el) =>
              el.querySelector('img')?.getAttribute('src') ===
              '/api/task/legacy-sheet/artifacts/image'
          )
        if (!found) throw new Error('legacy sheet not loaded')
        return found
      })
      await user.click(card)
      await user.click(
        await screen.findByRole('button', { name: 'Regenerate' })
      )
      await waitFor(() =>
        expect(
          screen.queryByRole('dialog', { name: 'Preview' })
        ).not.toBeInTheDocument()
      )
      expect(post).not.toHaveBeenCalled()
      expect(task.properties?.input).toBe(originalInput)
      expect(
        screen.getByRole('radio', { name: 'Character design' })
      ).toHaveAttribute('aria-checked', 'true')
      expect(
        within(screen.getByRole('navigation', { name: 'Workspace' })).getByRole(
          'button',
          { name: 'Create' }
        )
      ).toHaveAttribute('aria-pressed', 'true')
      expect(screen.getByRole('button', { name: /Female/ })).toHaveAttribute(
        'aria-pressed',
        'true'
      )
      expect(
        screen.getByRole('button', { name: 'Preview: Style reference' })
      ).toBeInTheDocument()
      const generate = screen.getByRole('button', { name: /Generate/ })
      await waitFor(() => expect(generate).toBeEnabled())
      await user.click(generate)
      await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
      expect(post.mock.calls[0][0]).toBe('/pg/influencer/sheet')
      expect(post.mock.calls[0][1]).toMatchObject({
        tier: 'normal',
        selection,
        brief: '',
        image_url: 'https://example.com/reference.png',
        size: '1024x576',
      })
      expect(task.properties?.input).toBe(originalInput)
    }
  )

  it('opens the subscription takeover without submitting when the balance cannot cover the quote', async () => {
    auth.quota = 10 // 0.0002 credits; the image quote is 0.0147
    const user = userEvent.setup()
    render(
      <QueryClientProvider client={createAppQueryClient()}>
        <InfluencerWorkbench />
      </QueryClientProvider>
    )
    await user.click(await screen.findByRole('button', { name: /Bold/ }))
    await user.click(screen.getByRole('button', { name: 'Randomize' }))
    const generate = screen.getByRole('button', { name: /^Generate/ })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.click(generate)
    expect(
      await screen.findByRole('dialog', {
        name: 'Subscribe to continue this generation',
      })
    ).toBeVisible()
    expect(post).not.toHaveBeenCalled()
    auth.quota = undefined
  })

  it('opens the full-screen subscription takeover when the gateway rejects for quota', async () => {
    const user = userEvent.setup()
    post.mockImplementation((url: string) => {
      if (url === '/pg/influencer/sheet') {
        return Promise.reject({
          isAxiosError: true,
          response: {
            status: 403,
            data: { code: 'insufficient_user_quota', message: 'quota' },
          },
        })
      }
      return Promise.resolve({ data: { success: true, data: {} } })
    })
    render(
      <QueryClientProvider client={createAppQueryClient()}>
        <InfluencerWorkbench />
      </QueryClientProvider>
    )
    await user.click(await screen.findByRole('button', { name: /Bold/ }))
    await user.click(screen.getByRole('button', { name: 'Randomize' }))
    const generate = screen.getByRole('button', { name: /^Generate/ })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.click(generate)
    expect(
      await screen.findByRole('dialog', {
        name: 'Subscribe to continue this generation',
      })
    ).toBeVisible()
    // The site sells no plans in this fixture, so the takeover degrades to top-up.
    expect(
      await screen.findByRole('heading', {
        name: 'No subscription plans available',
      })
    ).toBeVisible()
  })

  it('keeps an invalid legacy sheet preview open without refilling the creation form', async () => {
    const user = userEvent.setup()
    const task: TaskRecord = {
      id: 3,
      task_id: 'invalid-legacy-sheet',
      status: 'SUCCESS',
      action: 'influencer_sheet',
      properties: {
        input: JSON.stringify({
          tier: 'normal',
          selection: { gender: 'female' },
          brief: 'Invalid draft',
        }),
      },
      artifacts: [
        {
          key: 'image',
          content_url: '/api/task/invalid-legacy-sheet/artifacts/image',
        },
      ],
    }
    get.mockImplementation(
      (url: string, config?: { params?: Record<string, unknown> }) => {
        if (url.includes('/task/self')) {
          const items =
            config?.params?.action === 'influencer_motion' ? [] : [task]
          return Promise.resolve({
            data: { success: true, data: { items, total: items.length } },
          })
        }
        return Promise.resolve(routeByUrl(url))
      }
    )
    render(
      <QueryClientProvider client={createAppQueryClient()}>
        <InfluencerWorkbench />
      </QueryClientProvider>
    )
    const selectedTier = await screen.findByRole('button', { name: /Bold/ })
    await user.click(selectedTier)
    const navigation = screen.getByRole('navigation', { name: 'Workspace' })
    const historyButton = within(navigation).getByRole('button', {
      name: 'History',
    })
    await user.click(historyButton)
    const download = await screen.findByRole('link', { name: 'Download' })
    await user.click(download.closest('.inf-card') as HTMLElement)
    const dialog = await screen.findByRole('dialog', { name: 'Preview' })
    const regenerate = within(dialog).getByRole('button', {
      name: 'Regenerate',
    })
    expect(regenerate).toBeDisabled()
    await user.click(regenerate)
    expect(post).not.toHaveBeenCalled()
    expect(dialog).toBeInTheDocument()
    expect(selectedTier).toHaveAttribute('aria-pressed', 'true')
    expect(historyButton).toHaveAttribute('aria-pressed', 'true')
  })

  it('does not offer a legacy brief-only inspiration or enable an empty character', async () => {
    const user = userEvent.setup()
    const legacyPreset = {
      id: 'pub-legacy-brief',
      name: 'Legacy brief-only inspiration',
      tier: 'normal',
      selection: {},
      brief: 'A red coat',
      preview: { path: '/legacy-preview.png', width: 1024, height: 576 },
      sheet: { path: '/legacy-sheet.png', width: 1024, height: 576 },
      created_at: 1,
      published: true,
    }
    get.mockImplementation((url: string) =>
      Promise.resolve(
        url.includes('/presets')
          ? { data: { success: true, data: { items: [legacyPreset] } } }
          : routeByUrl(url)
      )
    )
    render(
      <QueryClientProvider client={createAppQueryClient()}>
        <InfluencerWorkbench />
      </QueryClientProvider>
    )
    await user.click(
      await screen.findByRole('button', { name: 'Reset features' })
    )
    expect(
      screen.queryByRole('button', { name: legacyPreset.name })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Generate/ })).toBeDisabled()
    expect(post).not.toHaveBeenCalled()
    expect(legacyPreset.brief).toBe('A red coat')
  })

  it('submits the look with the pick the hard conflicts refuse taken out', async () => {
    const user = userEvent.setup()
    const preset = presetSnapshot.items.find(
      (item) => item.name === CONFLICTING_PRESET
    )
    expect(preset?.brief).toBeTruthy()
    const originalBrief = preset?.brief
    render(
      <QueryClientProvider client={createAppQueryClient()}>
        <InfluencerWorkbench />
      </QueryClientProvider>
    )

    // The card applies the preset exactly the way the sidebar's own button does.
    const card = (
      await screen.findByRole('button', { name: CONFLICTING_PRESET })
    ).closest('article')
    expect(card).not.toBeNull()
    await user.click(
      within(card as HTMLElement).getByRole('button', { name: 'Recreate' })
    )

    const generate = await screen.findByRole('button', { name: /Generate/ })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.click(generate)

    await waitFor(() => expect(post).toHaveBeenCalledTimes(1))
    const [url, body] = post.mock.calls[0] as [string, SheetBody]
    expect(url).toBe('/pg/influencer/sheet')
    expect(body.size).toBe('1024x576')
    expect(body.brief).toBe('')
    // The reported dead end: Muscular + Short legs in the Extreme tier.
    expect(body.tier).toBe('total')
    expect(body.selection.proportions).toEqual(['pr_shoulders'])
    expect(conflictMessage(body.tier, body.selection)).toBeNull()
    expect(preset?.brief).toBe(originalBrief)
  })
})
