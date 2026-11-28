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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { takeGuestHandoff } from '@/features/influencer/lib/guest'
import { readMotionMode } from '@/features/influencer/lib/motion-mode'
import { TOOL_HANDOFF_ISSUE_PATH } from '@/features/influencer/lib/tool-handoff'
import { readWorkbenchMode } from '@/features/influencer/lib/workbench-mode'
import { createAppQueryClient } from '@/lib/query-client'

import { TikTokToolPage } from '../index'

const { api, navigate, toastError, track } = vi.hoisted(() => ({
  api: { get: vi.fn(), post: vi.fn() },
  navigate: vi.fn(),
  toastError: vi.fn(),
  track: vi.fn(),
}))

vi.mock('@/lib/api', () => ({ api }))
vi.mock('sonner', () => ({ toast: { error: toastError } }))
vi.mock('@/hooks/use-sidebar-config', () => ({
  useIsSidebarModuleVisible: () => true,
}))
vi.mock('@/features/influencer/lib/track', () => ({
  trackInfluencerEvent: track,
  INFLUENCER_ANALYTICS_EVENTS: {},
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (select: (state: unknown) => unknown) =>
    select({ auth: { user: null } }),
}))
vi.mock('@/components/turnstile', () => ({ Turnstile: () => null }))
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigate,
  useRouter: () => ({ subscribe: () => () => {} }),
  Link: (props: {
    to?: string
    href?: string
    children: React.ReactNode
    className?: string
    onClick?: (event: React.MouseEvent<HTMLAnchorElement>) => void
  }) => (
    <a
      href={props.to ?? props.href}
      className={props.className}
      onClick={(event) => {
        props.onClick?.(event)
        // Stand-in for the router: a link the handler did not take over
        // would navigate on its own.
        if (!event.defaultPrevented) event.preventDefault()
      }}
    >
      {props.children}
    </a>
  ),
}))

const VIDEO_URL =
  'https://www.tiktok.com/@mythicshadowws/video/7470610564927032609'
const WORK_ID = '7470610564927032609'
const MEDIA_PATH = '/api/tools/tiktok/file/video?u=v'

function config() {
  return {
    data: {
      success: true,
      data: {
        enabled: true,
        turnstile_site_key: 'site',
        pass_valid: true,
        quota_exhausted: false,
        logged_in: false,
      },
    },
  }
}

function videoResult(durationMs = 15000) {
  return {
    data: {
      success: true,
      data: {
        kind: 'video',
        id: WORK_ID,
        author: { unique_id: 'mythicshadowws', nickname: 'M' },
        description: '',
        created_at: 1739404800,
        cover_url: '',
        video: {
          duration_ms: durationMs,
          width: 720,
          height: 1280,
          preview_url: MEDIA_PATH,
          download_url: `${MEDIA_PATH}&dl=1`,
        },
        quota_exhausted: false,
      },
    },
  }
}

function serve(options: { durationMs?: number } = {}) {
  api.get.mockResolvedValue(config())
  api.post.mockResolvedValue(videoResult(options.durationMs))
}

function renderPage() {
  return render(
    <QueryClientProvider client={createAppQueryClient()}>
      <TikTokToolPage kind='video' locale='en' />
    </QueryClientProvider>
  )
}

async function renderParsed() {
  renderPage()
  await act(async () => {
    await Promise.resolve()
  })
  fireEvent.change(screen.getByRole('textbox'), {
    target: { value: VIDEO_URL },
  })
  fireEvent.click(screen.getByRole('button', { name: /parse/i }))
  await screen.findByRole('link', { name: /^recreate$/i })
}

function handoffCalls() {
  return api.post.mock.calls.filter(
    (call) => call[0] === TOOL_HANDOFF_ISSUE_PATH
  )
}

afterEach(() => {
  window.sessionStorage.clear()
  window.localStorage.clear()
  vi.resetAllMocks()
})

describe('tool page hands the parsed video to the workbench', () => {
  it('lands Start recreating on the empty recreate slot without a handoff when nothing was parsed', async () => {
    serve()
    renderPage()
    await act(async () => {
      await Promise.resolve()
    })

    fireEvent.click(screen.getByRole('link', { name: /start recreating/i }))

    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith({ to: '/influencer' })
    )
    expect(handoffCalls()).toHaveLength(0)
    expect(readWorkbenchMode()).toBe('video')
    expect(readMotionMode()).toBe('recreate')
    expect(takeGuestHandoff()).toBeNull()
  })

  it('navigates Start recreating without calling the handoff endpoint first', async () => {
    serve()
    await renderParsed()

    fireEvent.click(screen.getByRole('link', { name: /start recreating/i }))

    // The click handler must not wait for any network call.
    expect(navigate).toHaveBeenCalledWith({ to: '/influencer' })
    expect(handoffCalls()).toHaveLength(0)
    expect(takeGuestHandoff()).toMatchObject({ workId: WORK_ID })
  })

  it('stores the parsed work id for Recreate before navigating', async () => {
    serve()
    await renderParsed()

    fireEvent.click(screen.getByRole('link', { name: /^recreate$/i }))

    expect(navigate).toHaveBeenCalledWith({ to: '/influencer' })
    expect(handoffCalls()).toHaveLength(0)
    expect(readMotionMode()).toBe('recreate')
    expect(takeGuestHandoff()).toMatchObject({
      workId: WORK_ID,
      mode: 'video',
      motionMode: 'recreate',
    })
  })

  it('renders download and recreate in one action row', async () => {
    serve()
    await renderParsed()

    const rows = document.querySelectorAll('.tool-acts')
    expect(rows).toHaveLength(1)
    expect(rows[0].querySelectorAll('a')).toHaveLength(2)
    expect(rows[0].textContent).toContain('Download MP4')
    expect(rows[0].textContent).toContain('Recreate')
  })

  it('keeps the result card and reports the duration when the clip is outside 4–30 s', async () => {
    serve({ durationMs: 45000 })
    await renderParsed()

    fireEvent.click(screen.getByRole('link', { name: /^recreate$/i }))

    expect(navigate).not.toHaveBeenCalled()
    expect(toastError).toHaveBeenCalledWith(
      'Reference videos must be 4–30 seconds (current 00:45)'
    )
    expect(handoffCalls()).toHaveLength(0)
    expect(takeGuestHandoff()).toBeNull()
  })

  it('ignores a second click while the first navigation is pending', async () => {
    serve()
    await renderParsed()

    const link = screen.getByRole('link', { name: /^recreate$/i })
    fireEvent.click(link)
    fireEvent.click(link)

    expect(navigate).toHaveBeenCalledTimes(1)
    expect(handoffCalls()).toHaveLength(0)
  })

  it('leaves a new-tab click to the browser and carries no handoff', async () => {
    serve()
    await renderParsed()

    fireEvent.click(screen.getByRole('link', { name: /start recreating/i }), {
      metaKey: true,
    })

    expect(navigate).not.toHaveBeenCalled()
    expect(handoffCalls()).toHaveLength(0)
    expect(readMotionMode()).toBe('recreate')
  })

  it('keeps the original link and media paths out of navigation, analytics and the handoff', async () => {
    serve()
    await renderParsed()

    fireEvent.click(screen.getByRole('link', { name: /start recreating/i }))
    await waitFor(() => expect(navigate).toHaveBeenCalledTimes(1))

    const outbound = JSON.stringify({
      navigate: navigate.mock.calls,
      track: track.mock.calls,
      handoff: handoffCalls(),
      stored: window.sessionStorage.getItem('influencer:guest-handoff:v1'),
    })
    for (const secret of [VIDEO_URL, 'tiktok.com', MEDIA_PATH]) {
      expect(outbound).not.toContain(secret)
    }
    expect(window.location.search).toBe('')
  })
})
