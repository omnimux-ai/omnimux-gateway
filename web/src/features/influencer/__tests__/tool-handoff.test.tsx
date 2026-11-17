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
import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { LoginGateDialog } from '../components/login-gate-dialog'
import { useLoginGate } from '../hooks/use-login-gate'
import {
  TOOL_HANDOFF_DURATION_MESSAGE,
  TOOL_HANDOFF_FAILED_MESSAGE,
  TOOL_HANDOFF_TIMEOUT_MESSAGE,
  useToolHandoff,
} from '../hooks/use-tool-handoff'
import {
  peekGuestHandoff,
  takeGuestHandoff,
  writeGuestHandoff,
} from '../lib/guest'
import {
  TOOL_HANDOFF_ISSUE_PATH,
  TOOL_HANDOFF_REDEEM_PATH,
} from '../lib/tool-handoff'
import type { MotionMode, WorkbenchMode } from '../types'

const { post, toastError, navigate, track, auth } = vi.hoisted(() => ({
  post: vi.fn(),
  toastError: vi.fn(),
  navigate: vi.fn(),
  track: vi.fn(),
  auth: {
    user: null as null | { id: number },
    accessToken: null as null | string,
  },
}))

vi.mock('@/lib/api', () => ({ api: { post } }))
vi.mock('sonner', () => ({ toast: { error: toastError } }))
vi.mock('react-i18next', () => ({
  useTranslation: () => ({ t: (key: string) => key }),
}))
vi.mock('@tanstack/react-router', () => ({ useNavigate: () => navigate }))
vi.mock('../lib/track', () => ({
  trackInfluencerEvent: track,
  INFLUENCER_ANALYTICS_EVENTS: { LOGIN_CTA: 'influencer_login_cta' },
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (select: (state: unknown) => unknown) => select({ auth }),
}))

const TOKEN = 'a'.repeat(43)
const WORK_ID = '7470610564927032609'
const STORED_URL =
  'https://files.omnimux.ai/influencer/reference/f_tiktok-1.mp4'

function pendingHandoff() {
  writeGuestHandoff({
    workId: WORK_ID,
    mode: 'video',
    motionMode: 'recreate',
  })
}

function legacyTokenHandoff(expiresAt = Date.now() + 60_000) {
  writeGuestHandoff({
    token: TOKEN,
    expiresAt,
    mode: 'video',
    motionMode: 'recreate',
  })
}

function renderHandoff(options: {
  isGuest: boolean
  workbenchMode?: WorkbenchMode
  motionMode?: MotionMode
  accepted?: boolean
}) {
  const props = {
    isGuest: options.isGuest,
    workbenchMode: options.workbenchMode ?? 'image',
    onWorkbenchModeChange: vi.fn(),
    motionMode: options.motionMode ?? 'motion',
    setMotionMode: vi.fn(),
    setRemoteVideo: vi.fn().mockResolvedValue(options.accepted ?? true),
  }
  const hook = renderHook((current: typeof props) => useToolHandoff(current), {
    initialProps: props,
  })
  return { props, hook }
}

function issued() {
  return {
    data: { success: true, data: { token: TOKEN, expires_in: 1800 } },
  }
}

function redeemed() {
  return {
    data: {
      success: true,
      data: {
        url: STORED_URL,
        preview_url: STORED_URL,
        name: 'tiktok-1.mp4',
      },
    },
  }
}

function rejectWith(status: number, code?: string) {
  return Promise.reject({
    response: { status, data: code ? { success: false, code } : undefined },
  })
}

beforeEach(() => {
  auth.user = null
  auth.accessToken = null
})

afterEach(() => {
  window.sessionStorage.clear()
  vi.useRealTimers()
  vi.resetAllMocks()
})

describe('guest handoff record', () => {
  it('returns the work id once and then nothing when read twice', () => {
    pendingHandoff()
    expect(takeGuestHandoff()?.workId).toBe(WORK_ID)
    expect(takeGuestHandoff()).toBeNull()
  })

  it('peeks at the record without consuming it', () => {
    pendingHandoff()
    expect(peekGuestHandoff()?.workId).toBe(WORK_ID)
    expect(takeGuestHandoff()?.workId).toBe(WORK_ID)
  })

  it('drops a legacy token whose expiry has passed', () => {
    legacyTokenHandoff(Date.now() - 1)
    expect(takeGuestHandoff()).toBeNull()
  })

  it('drops a record whose token is malformed', () => {
    writeGuestHandoff({
      token: 'https://www.tiktok.com/@a/video/1',
      expiresAt: Date.now() + 60_000,
      mode: 'video',
      motionMode: 'recreate',
    })
    expect(takeGuestHandoff()).toBeNull()
  })
})

describe('workbench redeems a tool-page handoff', () => {
  it('shows loading immediately, then fills the recreate slot after issue and redeem', async () => {
    pendingHandoff()
    post.mockImplementation((path: string) =>
      path === TOOL_HANDOFF_ISSUE_PATH
        ? Promise.resolve(issued())
        : Promise.resolve(redeemed())
    )
    const { props, hook } = renderHandoff({ isGuest: false })

    expect(hook.result.current.status).toBe('loading')
    expect(hook.result.current.loadingLabel).toBe('Loading video…')

    await waitFor(() => expect(hook.result.current.status).toBe('idle'))
    expect(post.mock.calls.map((call) => call[0])).toEqual([
      TOOL_HANDOFF_ISSUE_PATH,
      TOOL_HANDOFF_REDEEM_PATH,
    ])
    expect(props.setRemoteVideo).toHaveBeenCalledWith(
      STORED_URL,
      'tiktok-1.mp4',
      STORED_URL,
      'recreate'
    )
    expect(props.onWorkbenchModeChange).toHaveBeenCalledWith('video')
    expect(props.setMotionMode).toHaveBeenCalledWith('recreate')
    expect(toastError).not.toHaveBeenCalled()
  })

  it('keeps the legacy token path working for a tab that clicked before the deploy', async () => {
    legacyTokenHandoff()
    post.mockResolvedValue(redeemed())
    const { props, hook } = renderHandoff({ isGuest: false })

    await waitFor(() => expect(hook.result.current.status).toBe('idle'))
    expect(post).toHaveBeenCalledWith(
      TOOL_HANDOFF_REDEEM_PATH,
      { token: TOKEN },
      expect.anything()
    )
    expect(props.setRemoteVideo).toHaveBeenCalled()
  })

  it('leaves the record for after sign-in when the visitor is a guest', () => {
    pendingHandoff()
    const { hook } = renderHandoff({ isGuest: true })

    expect(post).not.toHaveBeenCalled()
    expect(hook.result.current.status).toBe('idle')
    expect(takeGuestHandoff()?.workId).toBe(WORK_ID)
  })

  it('shows a failed slot and toasts once when issuing the token fails', async () => {
    pendingHandoff()
    post.mockImplementation(() => rejectWith(503))
    const { props, hook } = renderHandoff({ isGuest: false })

    await waitFor(() => expect(hook.result.current.status).toBe('failed'))
    expect(hook.result.current.error).toBe(TOOL_HANDOFF_FAILED_MESSAGE)
    expect(toastError).toHaveBeenCalledTimes(1)
    expect(toastError).toHaveBeenCalledWith(TOOL_HANDOFF_FAILED_MESSAGE)
    expect(props.setRemoteVideo).not.toHaveBeenCalled()
  })

  it('maps handoff_duration to the readable duration message', async () => {
    pendingHandoff()
    post.mockImplementation(() => rejectWith(422, 'handoff_duration'))
    const { hook } = renderHandoff({ isGuest: false })

    await waitFor(() => expect(hook.result.current.status).toBe('failed'))
    expect(hook.result.current.error).toBe(TOOL_HANDOFF_DURATION_MESSAGE)
    expect(toastError).toHaveBeenCalledWith(TOOL_HANDOFF_DURATION_MESSAGE)
  })

  it('retries a failed issue without stacking a second request', async () => {
    pendingHandoff()
    post
      .mockImplementationOnce(() => rejectWith(503))
      .mockResolvedValueOnce(issued())
      .mockResolvedValueOnce(redeemed())
    const { props, hook } = renderHandoff({ isGuest: false })

    await waitFor(() => expect(hook.result.current.status).toBe('failed'))
    act(() => hook.result.current.retry())
    await waitFor(() => expect(hook.result.current.status).toBe('idle'))
    expect(props.setRemoteVideo).toHaveBeenCalledTimes(1)
    expect(toastError).toHaveBeenCalledTimes(1)
  })

  it('reuses the issued token when retrying a failed redeem', async () => {
    pendingHandoff()
    post
      .mockResolvedValueOnce(issued())
      .mockImplementationOnce(() => rejectWith(500))
      .mockResolvedValueOnce(redeemed())
    const { props, hook } = renderHandoff({ isGuest: false })

    await waitFor(() => expect(hook.result.current.status).toBe('failed'))
    act(() => hook.result.current.retry())
    await waitFor(() => expect(hook.result.current.status).toBe('idle'))
    expect(props.setRemoteVideo).toHaveBeenCalledTimes(1)
    expect(
      post.mock.calls.filter((call) => call[0] === TOOL_HANDOFF_ISSUE_PATH)
    ).toHaveLength(1)
    expect(
      post.mock.calls.filter((call) => call[0] === TOOL_HANDOFF_REDEEM_PATH)
    ).toHaveLength(2)
  })

  it('times out the issue call after five seconds', async () => {
    vi.useFakeTimers()
    pendingHandoff()
    post.mockImplementation(() => new Promise(() => {}))
    const { hook } = renderHandoff({ isGuest: false })

    await act(async () => {
      vi.advanceTimersByTime(5_000)
      await Promise.resolve()
    })
    expect(hook.result.current.status).toBe('failed')
    expect(hook.result.current.error).toBe(TOOL_HANDOFF_TIMEOUT_MESSAGE)
    expect(toastError).toHaveBeenCalledWith(TOOL_HANDOFF_TIMEOUT_MESSAGE)
  })

  it('times out the redeem call after sixty seconds', async () => {
    vi.useFakeTimers()
    pendingHandoff()
    post
      .mockResolvedValueOnce(issued())
      .mockImplementation(() => new Promise(() => {}))
    const { hook } = renderHandoff({ isGuest: false })

    await act(async () => {
      await Promise.resolve()
      await Promise.resolve()
    })
    expect(post).toHaveBeenCalledTimes(2)
    await act(async () => {
      vi.advanceTimersByTime(60_000)
      await Promise.resolve()
      await Promise.resolve()
    })
    expect(hook.result.current.status).toBe('failed')
    expect(hook.result.current.error).toBe(TOOL_HANDOFF_TIMEOUT_MESSAGE)
  })

  it('switches the loading label after ten seconds without a second request', async () => {
    vi.useFakeTimers()
    pendingHandoff()
    post
      .mockResolvedValueOnce(issued())
      .mockImplementation(() => new Promise(() => {}))
    const { hook } = renderHandoff({ isGuest: false })

    await act(async () => {
      await Promise.resolve()
      await Promise.resolve()
    })
    expect(post).toHaveBeenCalledTimes(2)
    await act(async () => {
      vi.advanceTimersByTime(10_000)
      await Promise.resolve()
    })
    expect(hook.result.current.status).toBe('loading')
    expect(hook.result.current.loadingLabel).toBe('Still loading video…')
    expect(post).toHaveBeenCalledTimes(2)
  })

  it('toasts once without calling the server when a legacy record already failed', () => {
    writeGuestHandoff({ failed: true, mode: 'video', motionMode: 'recreate' })
    const { hook } = renderHandoff({ isGuest: false })

    expect(post).not.toHaveBeenCalled()
    expect(hook.result.current.status).toBe('failed')
    expect(toastError).toHaveBeenCalledTimes(1)
  })

  it('adds no toast of its own when the slot rejects the redeemed video', async () => {
    pendingHandoff()
    post
      .mockResolvedValueOnce(issued())
      .mockResolvedValueOnce(redeemed())
    const { props, hook } = renderHandoff({ isGuest: false, accepted: false })

    await waitFor(() =>
      expect(props.setRemoteVideo).toHaveBeenCalledTimes(1)
    )
    await waitFor(() => expect(hook.result.current.status).toBe('idle'))
    expect(toastError).not.toHaveBeenCalled()
  })

  it('issues and redeems only once across re-renders', async () => {
    pendingHandoff()
    post
      .mockResolvedValueOnce(issued())
      .mockResolvedValueOnce(redeemed())
    const { props, hook } = renderHandoff({ isGuest: false })
    hook.rerender({ ...props, motionMode: 'recreate' })

    await waitFor(() => expect(hook.result.current.status).toBe('idle'))
    expect(post).toHaveBeenCalledTimes(2)
  })

  it('does nothing without a handoff', () => {
    const { props, hook } = renderHandoff({ isGuest: false })
    expect(post).not.toHaveBeenCalled()
    expect(hook.result.current.status).toBe('idle')
    expect(props.onWorkbenchModeChange).not.toHaveBeenCalled()
    expect(toastError).not.toHaveBeenCalled()
  })
})

describe('sign-in keeps the handoff', () => {
  it('keeps the record when the login gate snapshots the look', () => {
    pendingHandoff()
    const gate = renderHook(() => useLoginGate())
    gate.result.current.requireLogin(
      'generate',
      {
        tier: 'total',
        selection: {},
        brief: '',
        seed: '',
        imageUrl: '',
        model: '',
        group: '',
        vendorId: null,
      },
      vi.fn()
    )

    expect(takeGuestHandoff()?.workId).toBe(WORK_ID)
  })

  it('keeps the record and adds no query parameter when the gate sends the guest to sign in', () => {
    pendingHandoff()
    render(<LoginGateDialog open onOpenChange={vi.fn()} />)
    fireEvent.click(screen.getAllByRole('button')[0])

    expect(navigate).toHaveBeenCalledTimes(1)
    const target = navigate.mock.calls[0][0] as {
      search: Record<string, string>
    }
    expect(target.search).toEqual({ redirect: '/influencer' })
    expect(JSON.stringify(track.mock.calls)).not.toContain(WORK_ID)
    expect(takeGuestHandoff()?.workId).toBe(WORK_ID)
  })
})
