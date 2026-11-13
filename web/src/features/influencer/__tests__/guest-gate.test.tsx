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
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppQueryClient } from '@/lib/query-client'

import { InfluencerWorkbench } from '../index'
import { clearSession } from '../lib/workbench-session'

/**
 * Guest window-mode contract: visitors browse the workbench, every write
 * action opens the login gate instead of calling the generation API, and the
 * history tab is an empty conversion touchpoint — no task fetch, no error.
 */
const { get, post } = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
}))

vi.mock('@/lib/api', () => ({ api: { get, post } }))
vi.mock('@tanstack/react-router', () => ({
  useSearch: () => ({}),
  useNavigate: () => vi.fn(),
}))
// No user, no token — this is the guest path.
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (select: (state: unknown) => unknown) =>
    select({ auth: { user: null, accessToken: null } }),
}))
vi.mock('sonner', () => ({
  toast: { error: vi.fn(), info: vi.fn(), success: vi.fn(), warning: vi.fn() },
}))
vi.mock('@/lib/guest-nav', () => ({}))

const PRICING = {
  data: {
    success: true,
    data: [
      {
        model_name: 'gpt-image-2.5-sunburst',
        model_price: 0.04,
        enable_groups: ['default'],
        supported_endpoint_types: ['image-generation'],
        vendor_id: 1,
      },
    ],
    vendors: [{ id: 1, name: 'OpenAI' }],
    group_ratio: { default: 1 },
  },
}

const TAXONOMY = {
  data: {
    success: true,
    data: {
      version: 1,
      source: 'test',
      tier_group: {
        options: [{ id: 'total', label_en: 'Extreme' }],
        comingSoonImageUrls: [],
      },
      category_priority: ['body'],
      categories: [
        {
          id: 'body',
          kind: 'single',
          label_en: 'Body',
          options: [
            {
              id: 'curvy',
              label_en: 'Curvy',
              visibleIn: ['total'],
            },
          ],
        },
      ],
      prompt_map: { tiers: {}, categories: {} },
    },
    etag: 'test',
  },
}

const PRESETS = { data: { success: true, data: { items: [] } } }

function routeByUrl(url: string) {
  if (url.includes('/pricing')) return PRICING
  if (url.includes('/taxonomy')) return TAXONOMY
  if (url.includes('/presets')) return PRESETS
  return { data: { success: true, data: {} } }
}

function renderWorkbench() {
  return render(
    <QueryClientProvider client={createAppQueryClient()}>
      <InfluencerWorkbench />
    </QueryClientProvider>
  )
}

describe('InfluencerWorkbench guest mode', () => {
  beforeEach(() => {
    get.mockReset()
    post.mockReset()
    get.mockImplementation((url: string) => Promise.resolve(routeByUrl(url)))
    localStorage.clear()
    sessionStorage.clear()
    clearSession()
  })

  it('restores a login draft without its legacy brief or losing a numeric seed reference', async () => {
    sessionStorage.setItem(
      'influencer:guest-draft:v1',
      JSON.stringify({
        tier: 'total',
        selection: { body: ['curvy'] },
        locks: {},
        brief: 'A red coat',
        seed: '42',
        imageUrl: 'https://files.example/ref.png',
        model: '',
        group: '',
        vendorId: null,
      })
    )
    renderWorkbench()
    await screen.findByRole('button', { name: 'Replace Style reference' })
    expect(screen.getByRole('button', { name: 'Curvy' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    // The workbench session lives in memory now: a carried draft is applied to
    // the panel, never persisted back as a pinned record.
    expect(sessionStorage.getItem('influencer:character-draft:v1')).toBeNull()
    expect(sessionStorage.getItem('influencer:guest-draft:v1')).toBeNull()
    await userEvent.click(screen.getByRole('button', { name: /log in/i }))
    expect(
      JSON.parse(String(sessionStorage.getItem('influencer:guest-draft:v1')))
    ).toMatchObject({
      brief: '',
      seed: '42',
      imageUrl: 'https://files.example/ref.png',
      locks: {},
    })
  })

  it('drops stale login draft options instead of restoring them', async () => {
    sessionStorage.setItem(
      'influencer:guest-draft:v1',
      JSON.stringify({
        tier: 'retired',
        selection: { body: ['curvy', 'missing'], unknown: ['bad'] },
        locks: { body: ['curvy', 'missing'] },
        brief: '',
        seed: '',
        imageUrl: '',
        model: '',
        group: '',
        vendorId: null,
      })
    )
    renderWorkbench()
    await screen.findByRole('button', { name: /Character type/ })
    // A retired tier shows none of the saved options, so the panel keeps none.
    expect(screen.queryByRole('button', { name: 'Curvy' })).toBeNull()
    expect(sessionStorage.getItem('influencer:guest-draft:v1')).toBeNull()
  })

  it('does not fetch user groups or history for guests', async () => {
    renderWorkbench()
    // Let the public queries settle.
    expect(
      await screen.findByRole('button', { name: 'Curvy' })
    ).toBeInTheDocument()
    const urls = get.mock.calls.map((c) => String(c[0]))
    expect(urls.some((u) => u.includes('/user/self/groups'))).toBe(false)
    expect(urls.some((u) => u.includes('/task/self'))).toBe(false)
    expect(urls.some((u) => u.includes('/taxonomy'))).toBe(true)
    expect(urls.some((u) => u.includes('/presets'))).toBe(true)
  })

  it('keeps the sign-in CTA enabled even with zero selections', async () => {
    renderWorkbench()
    // Let the taxonomy settle so canGenerate can be evaluated for real.
    expect(
      await screen.findByRole('button', { name: 'Curvy' })
    ).toBeInTheDocument()
    await userEvent.click(
      screen.getByRole('button', { name: 'Reset features' })
    )
    // The sign-in entry remains enabled after clearing the initial random picks.
    const cta = await screen.findByRole('button', { name: /log in/i })
    expect(cta).toBeEnabled()
  })

  it('opens the login gate on generate and never posts the sheet', async () => {
    const user = userEvent.setup()
    renderWorkbench()

    await screen.findByRole('button', { name: 'Curvy' })
    // Clear the automatically selected trait, then explicitly choose it.
    await user.click(screen.getByRole('button', { name: 'Reset features' }))
    await user.click(screen.getByRole('button', { name: 'Curvy' }))

    const cta = screen.getByRole('button', { name: /log in/i })
    await user.click(cta)

    // The gate dialog appears; the generation endpoint is never called.
    expect(await screen.findByText('Log in to generate')).toBeInTheDocument()
    expect(post).not.toHaveBeenCalled()

    // The guest draft is snapshotted for the post-login restore.
    const raw = sessionStorage.getItem('influencer:guest-draft:v1')
    expect(raw).not.toBeNull()
    expect(JSON.parse(String(raw)).selection.body).toContain('curvy')
  })
})
