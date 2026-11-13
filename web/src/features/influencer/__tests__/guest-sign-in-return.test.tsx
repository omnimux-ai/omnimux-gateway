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
import userEvent from '@testing-library/user-event'
import {
  AxiosError,
  AxiosHeaders,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  resolveAuthRedirect,
  sanitizeAuthRedirect,
} from '@/features/auth/lib/auth-redirect'
import { api } from '@/lib/http-client'
import { createAppQueryClient } from '@/lib/query-client'

import { InfluencerWorkbench } from '../index'
import { writeGuestHandoff } from '../lib/guest'
import { TOOL_HANDOFF_REDEEM_PATH } from '../lib/tool-handoff'
import { clearSession } from '../lib/workbench-session'

/**
 * The tool-page → workbench journey for a visitor, driven through the real
 * HTTP client and its global 401 handling (only the transport is stubbed):
 * the guest must stay on /influencer, leave for sign-in with a same-origin
 * `redirect`, and find the handed-off video in the recreate slot afterwards.
 */
const { navigate, auth, replace, refresh, viewport } = vi.hoisted(() => ({
  viewport: { mobile: false },
  navigate: vi.fn(),
  replace: vi.fn(),
  refresh: vi.fn(),
  auth: {
    user: null as null | { id: number; username: string },
    accessToken: null as null | string,
    session: null as null | { sid: string },
  },
}))

vi.mock('@tanstack/react-router', () => ({
  useSearch: () => ({}),
  useNavigate: () => navigate,
}))
vi.mock('@/stores/auth-store', () => {
  const useAuthStore = (select: (state: unknown) => unknown) => select({ auth })
  useAuthStore.getState = () => ({ auth })
  return { useAuthStore }
})
vi.mock('@/lib/auth-session', () => ({
  applyAuthRotation: vi.fn(),
  clearAuthentication: vi.fn(),
  getFreshAuthHeaders: vi.fn().mockResolvedValue({}),
  refreshAuthentication: refresh,
}))
vi.mock('sonner', () => ({
  toast: { error: vi.fn(), info: vi.fn(), success: vi.fn(), warning: vi.fn() },
}))
vi.mock('@/lib/guest-nav', () => ({}))
// jsdom has no layout, so the anchored desktop picker closes itself; the
// picker test uses the compact dialog, which offers the same upload entry.
vi.mock('@/hooks/use-mobile', () => ({ useIsMobile: () => viewport.mobile }))
vi.mock('../lib/browser-video-metadata', () => ({
  readBrowserVideoMetadata: vi.fn().mockResolvedValue({
    duration: 8,
    video_duration: 8,
    width: 720,
    height: 1280,
    frame_rate: 30,
    video_codec: 'h264',
    audio_codec: 'aac',
    container: 'mp4',
    size_bytes: 2048,
  }),
}))

const ORIGIN = 'https://omnimux.ai'
const TOKEN = 'a'.repeat(43)
const STORED_URL =
  'https://files.omnimux.ai/influencer/reference/f_tiktok-1.mp4'
const MOTION_OPTIONS = '/pg/influencer/motion/options'

const TAXONOMY = {
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
      options: [{ id: 'curvy', label_en: 'Curvy', visibleIn: ['total'] }],
    },
  ],
  prompt_map: { tiers: {}, categories: {} },
}

const requested: string[] = []

/** What the gateway answers: signed-in routes are 401 until `auth` has a user. */
function serverBody(url: string): { status: number; data: unknown } {
  const signedIn = auth.user !== null
  if (url.includes('/taxonomy')) {
    return { status: 200, data: { success: true, data: TAXONOMY, etag: 't' } }
  }
  if (url.includes('/presets')) {
    return { status: 200, data: { success: true, data: { items: [] } } }
  }
  if (url.includes('/api/pricing')) {
    return {
      status: 200,
      data: { success: true, data: [], vendors: [], group_ratio: {} },
    }
  }
  if (!signedIn) return { status: 401, data: { success: false } }
  if (url.includes(TOOL_HANDOFF_REDEEM_PATH)) {
    return {
      status: 200,
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
  if (url.includes(MOTION_OPTIONS)) {
    return {
      status: 200,
      data: {
        success: true,
        data: {
          version: '1',
          resolutions: { '720p': { aspect_ratios: ['9:16'] } },
        },
      },
    }
  }
  return { status: 200, data: { success: true, data: {} } }
}

async function adapter(
  config: InternalAxiosRequestConfig
): Promise<AxiosResponse> {
  const url = String(config.url)
  requested.push(url)
  const { status, data } = serverBody(url)
  const response = {
    status,
    statusText: String(status),
    data,
    headers: new AxiosHeaders(),
    config,
  }
  if (status >= 400) {
    throw new AxiosError('failed', 'ERR_BAD_REQUEST', config, null, response)
  }
  return response
}

function renderWorkbench() {
  return render(
    <QueryClientProvider client={createAppQueryClient()}>
      <InfluencerWorkbench />
    </QueryClientProvider>
  )
}

const originalLocation = window.location
const originalAdapter = api.defaults.adapter

beforeEach(() => {
  auth.user = null
  auth.accessToken = null
  auth.session = null
  viewport.mobile = false
  requested.length = 0
  refresh.mockResolvedValue({ kind: 'anonymous' })
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  api.defaults.adapter = adapter
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: {
      ...originalLocation,
      origin: ORIGIN,
      href: `${ORIGIN}/influencer`,
      pathname: '/influencer',
      search: '',
      hash: '',
      replace,
    },
  })
  localStorage.clear()
  sessionStorage.clear()
  clearSession()
  // The tool page's "Start recreating" leaves the video surface on recreate.
  localStorage.setItem('influencer:workbench-mode', 'video')
  localStorage.setItem('influencer:motion-mode', 'recreate')
  writeGuestHandoff({
    token: TOKEN,
    expiresAt: Date.now() + 10 * 60_000,
    mode: 'video',
    motionMode: 'recreate',
  })
})

afterEach(() => {
  api.defaults.adapter = originalAdapter
  Object.defineProperty(window, 'location', {
    configurable: true,
    value: originalLocation,
  })
})

describe('guest on the video recreate workbench', () => {
  it('stays on /influencer for more than five seconds', async () => {
    renderWorkbench()
    expect(
      await screen.findByRole('button', { name: /log in/i })
    ).toBeInTheDocument()

    await act(() => new Promise((resolve) => setTimeout(resolve, 5_500)))

    expect(replace).not.toHaveBeenCalled()
    expect(requested.some((url) => url.includes(MOTION_OPTIONS))).toBe(false)
    expect(
      requested.some((url) => url.includes(TOOL_HANDOFF_REDEEM_PATH))
    ).toBe(false)
    expect(screen.getByRole('button', { name: /log in/i })).toBeInTheDocument()
  }, 15_000)

  it('still sends any other signed-in 401 to sign-in', async () => {
    await expect(api.get('/api/user/self')).rejects.toBeInstanceOf(AxiosError)
    expect(replace).toHaveBeenCalledWith('/sign-in')
  })

  it('returns to /influencer after sign-in and fills the reference slot', async () => {
    const user = userEvent.setup()
    const guest = renderWorkbench()

    await user.click(await screen.findByRole('button', { name: /log in/i }))
    await user.click(await screen.findByRole('button', { name: 'Sign in' }))

    expect(navigate).toHaveBeenCalledWith({
      to: '/sign-in',
      search: { redirect: '/influencer' },
    })
    const { redirect } = navigate.mock.calls[0][0].search as {
      redirect: string
    }
    // The sign-in page resolves the destination with its same-origin check.
    expect(resolveAuthRedirect(redirect, ORIGIN)).toBe('/influencer')
    guest.unmount()

    // Signed in, the same tab lands back on the workbench.
    auth.user = { id: 7, username: 'creator' }
    auth.accessToken = 'access-token'
    auth.session = { sid: 'sid-7' }
    const { container } = renderWorkbench()

    await waitFor(() =>
      expect(
        container.querySelector(`video[src="${STORED_URL}"]`)
      ).not.toBeNull()
    )
    expect(
      requested.filter((url) => url.includes(TOOL_HANDOFF_REDEEM_PATH))
    ).toHaveLength(1)
    expect(requested.some((url) => url.includes(MOTION_OPTIONS))).toBe(true)
    expect(replace).not.toHaveBeenCalled()
  })
})

describe('guest image uploads on the video surface', () => {
  const PRESIGN = '/api/v1/files/upload/presign'
  const image = () => new File(['png'], 'look.png', { type: 'image/png' })

  async function slotFor(motionMode: string, label: string) {
    localStorage.setItem('influencer:motion-mode', motionMode)
    renderWorkbench()
    const trigger = await screen.findByRole('button', { name: label })
    const slot = trigger.closest('.inf-character-slot')
    if (!slot) throw new Error('character slot not found')
    const input = slot.querySelector('input[type="file"]')
    if (!(input instanceof HTMLInputElement)) {
      throw new Error('character input not found')
    }
    return { trigger, slot, input }
  }

  async function expectGateWithoutUpload() {
    expect(await screen.findByText('Log in to generate')).toBeInTheDocument()
    await act(() => new Promise((resolve) => setTimeout(resolve, 300)))
    expect(requested.some((url) => url.includes(PRESIGN))).toBe(false)
    expect(replace).not.toHaveBeenCalled()
  }

  it.each([
    ['recreate', 'Add reference images'],
    ['motion', 'Add character, product or clothing images'],
  ])('opens the login gate when a %s image is chosen', async (mode, label) => {
    const { input } = await slotFor(mode, label)
    fireEvent.change(input, { target: { files: [image()] } })
    await expectGateWithoutUpload()
  })

  it.each([
    ['recreate', 'Add reference images'],
    ['motion', 'Add character, product or clothing images'],
  ])('opens the login gate when a %s image is dropped', async (mode, label) => {
    const { slot } = await slotFor(mode, label)
    fireEvent.drop(slot, { dataTransfer: { files: [image()] } })
    await expectGateWithoutUpload()
  })

  it('opens the login gate when the reference picker uploads an image', async () => {
    viewport.mobile = true
    const { trigger } = await slotFor('recreate', 'Add reference images')
    fireEvent.click(trigger)
    const upload = await screen.findByRole('button', {
      name: 'Upload local photo',
    })
    const pickerInput = upload
      .closest('.inf-character-grid')
      ?.parentElement?.querySelector<HTMLInputElement>('input[type="file"]')
    if (!pickerInput) throw new Error('picker input not found')
    fireEvent.change(pickerInput, { target: { files: [image()] } })
    await expectGateWithoutUpload()
  })
})

describe('sign-in redirect stays on this site', () => {
  it.each([
    '//evil.example/influencer',
    '///evil.example',
    '/\\evil.example',
    '\\\\evil.example',
    'https://evil.example/influencer',
    'http:evil.example',
    'javascript:alert(1)',
    'data:text/html,hi',
    `${ORIGIN}.evil.example/influencer`,
  ])('rejects %s', (target) => {
    expect(sanitizeAuthRedirect(target, ORIGIN)).toBeNull()
    expect(resolveAuthRedirect(target, ORIGIN)).toBe('/')
  })

  it('keeps an on-site path', () => {
    expect(sanitizeAuthRedirect('/influencer', ORIGIN)).toBe('/influencer')
  })
})
