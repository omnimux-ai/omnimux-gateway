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
  TOOL_HANDOFF_FAILED_MESSAGE,
  useToolHandoff,
} from '../hooks/use-tool-handoff'
import { takeGuestHandoff, writeGuestHandoff } from '../lib/guest'
import { TOOL_HANDOFF_REDEEM_PATH } from '../lib/tool-handoff'
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
const STORED_URL =
  'https://files.omnimux.ai/influencer/reference/f_tiktok-1.mp4'

function pendingHandoff(expiresAt = Date.now() + 60_000) {
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
  vi.resetAllMocks()
})

describe('guest handoff record', () => {
  it('returns the token once and then nothing when read twice', () => {
    pendingHandoff()
    expect(takeGuestHandoff()?.token).toBe(TOKEN)
    expect(takeGuestHandoff()).toBeNull()
  })

  it('drops a record whose token has already expired', () => {
    pendingHandoff(Date.now() - 1)
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
  it('fills the recreate reference slot with the redeemed upload when signed in', async () => {
    pendingHandoff()
    post.mockResolvedValue({
      data: {
        success: true,
        data: {
          url: STORED_URL,
          preview_url: STORED_URL,
          name: 'tiktok-1.mp4',
        },
      },
    })
    const { props } = renderHandoff({ isGuest: false })

    await waitFor(() =>
      expect(props.setRemoteVideo).toHaveBeenCalledWith(
        STORED_URL,
        'tiktok-1.mp4',
        STORED_URL,
        'recreate'
      )
    )
    expect(post).toHaveBeenCalledWith(
      TOOL_HANDOFF_REDEEM_PATH,
      { token: TOKEN },
      expect.anything()
    )
    expect(props.onWorkbenchModeChange).toHaveBeenCalledWith('video')
    expect(props.setMotionMode).toHaveBeenCalledWith('recreate')
    expect(toastError).not.toHaveBeenCalled()
  })

  it('leaves the record for after sign-in when the visitor is a guest', () => {
    pendingHandoff()
    renderHandoff({ isGuest: true })

    expect(post).not.toHaveBeenCalled()
    expect(takeGuestHandoff()?.token).toBe(TOKEN)
  })

  it.each([
    ['an expired token', () => rejectWith(410, 'handoff_expired')],
    ['an already redeemed token', () => rejectWith(410, 'handoff_expired')],
    ['another owner', () => rejectWith(403, 'handoff_forbidden')],
    ['a video outside 4–30 s', () => rejectWith(422, 'handoff_duration')],
    ['a missing endpoint', () => rejectWith(404)],
    ['a network failure', () => Promise.reject(new Error('Network Error'))],
  ])(
    'keeps the empty recreate slot and toasts once for %s',
    async (_, failure) => {
      pendingHandoff()
      post.mockImplementation(failure)
      const { props } = renderHandoff({ isGuest: false })

      await waitFor(() => expect(toastError).toHaveBeenCalledTimes(1))
      expect(toastError).toHaveBeenCalledWith(TOOL_HANDOFF_FAILED_MESSAGE)
      expect(props.setRemoteVideo).not.toHaveBeenCalled()
      expect(props.setMotionMode).toHaveBeenCalledWith('recreate')
    }
  )

  it('toasts once without calling the server when the tool page could not issue a handoff', () => {
    writeGuestHandoff({ failed: true, mode: 'video', motionMode: 'recreate' })
    renderHandoff({ isGuest: false })

    expect(post).not.toHaveBeenCalled()
    expect(toastError).toHaveBeenCalledTimes(1)
  })

  it('adds no toast of its own when the slot rejects the redeemed video', async () => {
    pendingHandoff()
    post.mockResolvedValue({
      data: {
        success: true,
        data: {
          url: STORED_URL,
          preview_url: STORED_URL,
          name: 'tiktok-1.mp4',
        },
      },
    })
    const { props } = renderHandoff({ isGuest: false, accepted: false })

    await waitFor(() => expect(props.setRemoteVideo).toHaveBeenCalledTimes(1))
    expect(toastError).not.toHaveBeenCalled()
  })

  it('redeems only once across re-renders', async () => {
    pendingHandoff()
    post.mockImplementation(() => rejectWith(410, 'handoff_expired'))
    const { props, hook } = renderHandoff({ isGuest: false })
    hook.rerender({ ...props, motionMode: 'recreate' })

    await waitFor(() => expect(toastError).toHaveBeenCalledTimes(1))
    expect(post).toHaveBeenCalledTimes(1)
  })

  it('does nothing without a handoff', () => {
    const { props } = renderHandoff({ isGuest: false })
    expect(post).not.toHaveBeenCalled()
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

    expect(takeGuestHandoff()?.token).toBe(TOKEN)
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
    expect(JSON.stringify(track.mock.calls)).not.toContain(TOKEN)
    expect(takeGuestHandoff()?.token).toBe(TOKEN)
  })
})
