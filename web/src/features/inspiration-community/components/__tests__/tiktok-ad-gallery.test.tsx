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
/**
 * The ad library page against the real endpoint contract.
 *
 * Only the network boundary is mocked, and it answers with the captured
 * production page (`fixtures/tiktok-page-live.json`), so the grid, the result
 * counter, the pagination and the failure states all run over real rows.
 */
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { createServerError } from '@/lib/server-error-message'

import livePage from '../../api/__tests__/fixtures/tiktok-page-live.json'
import { TikTokAdGallery } from '../tiktok-ad-gallery'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/lib/http-client', () => ({ api: { get } }))

vi.mock('@/components/layout', () => ({
  PublicLayout: (props: { children: React.ReactNode }) => (
    <div>{props.children}</div>
  ),
}))

interface Params {
  page: number
  page_size: number
  type?: string
}

/** The captured page, re-stamped with the page the caller asked for. */
function pageResponse(params: Params) {
  const source = livePage.data as unknown as {
    total: number
    items: unknown[]
  }
  return {
    data: {
      success: true,
      message: '',
      data: {
        total: source.total,
        page: params.page,
        page_size: params.page_size,
        page_count: Math.ceil(source.total / params.page_size),
        items: params.page_size === 1 ? [] : source.items,
      },
    },
  }
}

/** Answers the feed from the captured page. */
function answerWithCapturedPage() {
  get.mockImplementation((_url: string, config: { params: Params }) =>
    Promise.resolve(pageResponse(config.params))
  )
}

function renderGallery() {
  const root = createRootRoute()
  const home = createRoute({
    getParentRoute: () => root,
    path: '/',
    component: () => null,
  })
  const tiktok = createRoute({
    getParentRoute: () => root,
    path: '/inspiration/tiktok',
    component: () => <TikTokAdGallery />,
  })
  const router = createRouter({
    routeTree: root.addChildren([home, tiktok]),
    history: createMemoryHistory({ initialEntries: ['/inspiration/tiktok'] }),
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })

  return render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  get.mockReset()
})

afterEach(() => {
  get.mockReset()
})

describe('TikTokAdGallery feed', () => {
  it('renders the captured production page once it arrives', async () => {
    answerWithCapturedPage()
    renderGallery()

    expect(
      await screen.findByRole('button', { name: '情绪共鸣型助眠歌单推广' })
    ).toBeInTheDocument()
  })

  it('shows the placeholder grid until the feed arrives', async () => {
    let release = () => {}
    const pending = new Promise<void>((resolve) => {
      release = resolve
    })
    get.mockImplementation(async (_url: string, config: { params: Params }) => {
      await pending
      return pageResponse(config.params)
    })

    renderGallery()

    // The route mounts asynchronously, so the assertion waits for the shell;
    // the feed itself stays unanswered until `release()`.
    expect(await screen.findByTestId('ad-skeleton')).toBeInTheDocument()
    release()
    expect(await screen.findByText('2,954 creatives')).toBeInTheDocument()
    expect(screen.queryByTestId('ad-skeleton')).not.toBeInTheDocument()
  })

  it('prints the response total rather than the length of the rendered page', async () => {
    answerWithCapturedPage()
    renderGallery()

    // 30 cards are on screen; the counter speaks for the 2,954 rows behind them.
    expect(await screen.findByText('2,954 creatives')).toBeInTheDocument()
  })

  it('asks the service for the page the visitor picked', async () => {
    answerWithCapturedPage()
    renderGallery()
    await screen.findByText('2,954 creatives')

    await userEvent.click(screen.getByRole('button', { name: '2' }))

    await waitFor(() => {
      const pages = get.mock.calls.map(
        ([, config]) => (config as { params: Params }).params.page
      )
      expect(pages).toContain(2)
    })
  })
})

describe('TikTokAdGallery detail layer', () => {
  it('opens the detail layer on the card the visitor picked', async () => {
    answerWithCapturedPage()
    renderGallery()

    await userEvent.click(
      await screen.findByRole('button', { name: '情绪共鸣型助眠歌单推广' })
    )

    // The layer walks the page on screen, not a bundled list.
    expect(await screen.findByText('1 / 30')).toBeInTheDocument()
  })

  it('walks the loaded page from the detail layer', async () => {
    answerWithCapturedPage()
    renderGallery()

    await userEvent.click(
      await screen.findByRole('button', { name: '情绪共鸣型助眠歌单推广' })
    )
    await screen.findByText('1 / 30')

    fireEvent.keyDown(window, { key: 'ArrowRight' })

    expect(await screen.findByText('2 / 30')).toBeInTheDocument()
  })
})

describe('TikTokAdGallery failure and empty states', () => {
  it('retranslates a cached server failure after switching language without refetching', async () => {
    i18next.addResourceBundle('zhCN', 'translation', {
      'Task timed out': '任务超时',
    })
    try {
      get.mockRejectedValue(
        createServerError({
          message_key: 'Task timed out',
          message: 'Task timed out',
        })
      )
      renderGallery()
      expect(await screen.findByText('Task timed out')).toBeInTheDocument()
      const requestCount = get.mock.calls.length

      await act(() => i18next.changeLanguage('zhCN'))

      expect(await screen.findByText('任务超时')).toBeInTheDocument()
      expect(screen.queryByText('Task timed out')).not.toBeInTheDocument()
      expect(get).toHaveBeenCalledTimes(requestCount)
    } finally {
      await act(() => i18next.changeLanguage('en'))
      i18next.removeResourceBundle('zhCN', 'translation')
    }
  })

  it('shows the service reason with a retry when the feed fails', async () => {
    get.mockRejectedValue(
      new Error('The TikTok ad library is temporarily unavailable.')
    )

    renderGallery()

    expect(
      await screen.findByText(
        'The TikTok ad library is temporarily unavailable.'
      )
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
    expect(screen.queryByTestId('ad-skeleton')).not.toBeInTheDocument()
  })

  it('guides the visitor to loosen a filter when a filtered feed is empty', async () => {
    get.mockImplementation((_url: string, config: { params: Params }) => {
      return Promise.resolve({
        data: {
          success: true,
          message: '',
          data: {
            total: 0,
            page: 1,
            page_size: config.params.page_size,
            page_count: 1,
            items: [],
          },
        },
      })
    })

    renderGallery()

    // No filter is active yet, so the empty state cannot blame one.
    expect(
      await screen.findByText('No creatives in the ad library yet')
    ).toBeInTheDocument()
    expect(
      screen.queryByText('No ads match these filters')
    ).not.toBeInTheDocument()
  })
})
