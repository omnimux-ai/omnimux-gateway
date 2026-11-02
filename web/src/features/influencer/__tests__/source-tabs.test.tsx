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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppQueryClient } from '@/lib/query-client'

import { InfluencerWorkbench } from '../index'
import { usablePresets } from '../lib/presets'
import type { InfluencerTaxonomy } from '../types'

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

const TAXONOMY = servedTaxonomy()

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

const GROUPS = { data: { success: true, data: { default: { ratio: 1 } } } }
const TASKS = { data: { success: true, data: { items: [], total: 0 } } }

function routeByUrl(url: string) {
  if (url.includes('/pricing')) return PRICING
  if (url.includes('/taxonomy')) {
    return { data: { success: true, data: TAXONOMY, etag: 'test' } }
  }
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

/**
 * How many category option cards the sidebar shows as picked.
 *
 * The exaggeration tier is excluded: it is its own control (one pick, not part
 * of a preset's per-category `selection`), and it reports `aria-pressed` too.
 */
function selectedOptionCount(): number {
  return document.querySelectorAll(
    '.inf-opt[aria-pressed="true"]:not(.inf-acc-tier .inf-opt)'
  ).length
}

describe('InfluencerWorkbench Explore / History', () => {
  beforeEach(() => {
    get.mockReset()
    get.mockImplementation((url: string) => Promise.resolve(routeByUrl(url)))
    localStorage.clear()
    vi.mocked(toast.success).mockClear()
  })

  it('opens on Explore and leaves out the history-only view switch', async () => {
    renderWorkbench()

    const explore = await screen.findByRole('radio', { name: 'Explore' })
    expect(explore).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByRole('radio', { name: 'History' })).toHaveAttribute(
      'aria-checked',
      'false'
    )
    expect(
      screen.queryByRole('radio', { name: 'Timeline' })
    ).not.toBeInTheDocument()
  })

  it('renders the mirrored presets while Explore is active', async () => {
    renderWorkbench()

    const first = usablePresets(TAXONOMY)[0]
    expect(
      await screen.findByRole('button', { name: first.name })
    ).toBeInTheDocument()
  })

  it('keeps display switching absent with a legacy preference and restores Explore', async () => {
    localStorage.setItem('influencer:view-mode', 'timeline')
    const user = userEvent.setup()
    renderWorkbench()
    const first = usablePresets(TAXONOMY)[0]

    await user.click(await screen.findByRole('radio', { name: 'History' }))
    expect(screen.getByRole('radio', { name: 'History' })).toHaveAttribute(
      'aria-checked',
      'true'
    )
    expect(
      screen.queryByRole('radio', { name: 'Timeline' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('radio', { name: 'Grid' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: first.name })
    ).not.toBeInTheDocument()

    await user.click(screen.getByRole('radio', { name: 'Explore' }))
    expect(screen.getByRole('radio', { name: 'Explore' })).toHaveAttribute(
      'aria-checked',
      'true'
    )
    expect(
      screen.queryByRole('radio', { name: 'Timeline' })
    ).not.toBeInTheDocument()
    expect(
      await screen.findByRole('button', { name: first.name })
    ).toBeInTheDocument()
  })

  it('fills the sidebar from the preview dialog and closes it', async () => {
    const user = userEvent.setup()
    renderWorkbench()
    const first = usablePresets(TAXONOMY)[0]
    const picks = Object.values(first.selection).flat()

    expect(selectedOptionCount()).toBe(0)

    await user.click(await screen.findByRole('button', { name: first.name }))
    const dialog = await screen.findByRole('dialog')
    await user.click(within(dialog).getByRole('button', { name: 'Recreate' }))

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(selectedOptionCount()).toBe(picks.length)
    expect(toast.success).toHaveBeenCalledTimes(1)
  })

  it('keeps the same video prompt mounted while switching workbenches', async () => {
    const user = userEvent.setup()
    renderWorkbench()
    await user.click(await screen.findByRole('radio', { name: 'Motion video' }))
    await user.click(screen.getByRole('switch', { name: 'Prompt' }))
    const prompt = screen.getByRole('textbox', { name: 'Prompt' })
    await user.click(screen.getByRole('radio', { name: 'Character design' }))
    expect(prompt).toBeInTheDocument()
    expect(prompt).not.toBeVisible()
    await user.click(screen.getByRole('radio', { name: 'Motion video' }))
    expect(screen.getByRole('textbox', { name: 'Prompt' })).toBe(prompt)
    expect(prompt).toBeVisible()
  })

  it('returns to character creation when applying a character preset from video mode', async () => {
    const user = userEvent.setup()
    renderWorkbench()
    await user.click(await screen.findByRole('radio', { name: 'Motion video' }))
    const first = usablePresets(TAXONOMY)[0]
    const picks = Object.values(first.selection).flat()

    const card = (
      await screen.findByRole('button', { name: first.name })
    ).closest('article')
    expect(card).not.toBeNull()
    await user.click(
      within(card as HTMLElement).getByRole('button', { name: 'Recreate' })
    )

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(
      screen.getByRole('radio', { name: 'Character design' })
    ).toHaveAttribute('aria-checked', 'true')
    expect(selectedOptionCount()).toBe(picks.length)
  })
})
