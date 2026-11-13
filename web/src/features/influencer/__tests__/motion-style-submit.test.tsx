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
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast, Toaster } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { InfluencerWorkbench } from '../index'
import { readBrowserVideoMetadata } from '../lib/browser-video-metadata'
import { MOTION_VISUAL_STYLES } from '../lib/motion-visual-styles'
import { mockReferenceLayout } from './reference-layout-fixture'

vi.mock('../lib/browser-video-metadata', () => ({
  readBrowserVideoMetadata: vi.fn(),
}))

const RAW_VIDEO_METADATA = {
  duration: 8,
  video_duration: 8,
  width: 720,
  height: 1280,
  frame_rate: 30,
  video_codec: 'h264',
  audio_codec: 'aac',
  container: 'mp4',
  size_bytes: 10,
}

const CHARACTER_URL = 'https://files.example/character.png'
const VIDEO_URL = 'https://files.example/reference.mp4'
const CEL_PROMPT = MOTION_VISUAL_STYLES[0].prompt
const USER_PROMPT =
  'Use photorealistic live-action skin, not animation.\n保留写实电影质感，不要手绘。'

let clients: QueryClient[]

beforeEach(() => {
  vi.mocked(readBrowserVideoMetadata).mockImplementation(async (source) => ({
    ...RAW_VIDEO_METADATA,
    size_bytes:
      typeof source === 'string' ? RAW_VIDEO_METADATA.size_bytes : source.size,
  }))
  mockReferenceLayout()
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  localStorage.clear()
  sessionStorage.clear()
  useAuthStore.getState().auth.reset()
  useAuthStore.setState((state) => ({
    auth: {
      ...state.auth,
      user: { id: 1, username: 'test', role: 1 },
      accessToken: 'test-only',
    },
  }))
  clients = []
  const createElement = document.createElement.bind(document)
  vi.spyOn(document, 'createElement').mockImplementation((tag, options) => {
    const element = createElement(tag, options)
    if (tag === 'video') {
      const video = element as HTMLVideoElement
      video.load = vi.fn()
    }
    return element
  })
  vi.stubGlobal(
    'URL',
    class extends URL {
      static createObjectURL = vi.fn(() => 'blob:reference-metadata')
      static revokeObjectURL = vi.fn()
    }
  )
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (String(url).includes('/taxonomy')) {
      return {
        data: {
          success: true,
          data: {
            version: 1,
            source: 'test',
            tier_group: { options: [{ id: 'total', label_en: 'Extreme' }] },
            categories: [],
            category_priority: [],
            prompt_map: { tiers: {}, categories: {} },
          },
        },
      }
    }
    if (String(url).includes('/pricing')) {
      return { data: { success: true, data: [], group_ratio: {} } }
    }
    if (String(url).includes('/groups')) {
      return { data: { success: true, data: {} } }
    }
    return { data: { success: true, data: { items: [], total: 0 } } }
  })
})

afterEach(() => {
  cleanup()
  for (const client of clients) client.clear()
  toast.dismiss()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
  localStorage.clear()
  sessionStorage.clear()
})

function mountWorkbench(): HTMLElement {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  clients.push(client)
  const root = createRootRoute({
    component: () => (
      <QueryClientProvider client={client}>
        <InfluencerWorkbench />
        <Toaster />
      </QueryClientProvider>
    ),
  })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  return render(<RouterProvider router={router} />).container
}

async function prepareMotionWithStyle(
  user: ReturnType<typeof userEvent.setup>
): Promise<void> {
  const container = mountWorkbench()
  await user.click(await screen.findByRole('radio', { name: 'Motion video' }))
  await user.click(
    screen.getByRole('button', {
      name: 'Add character, product or clothing images',
    })
  )
  const picker = await screen.findByRole('complementary', {
    name: 'Select reference images',
  })
  await user.click(
    within(picker).getByRole('button', { name: 'Upload local photo' })
  )
  await user.upload(
    picker.querySelector('input[type=file]') as HTMLInputElement,
    new File(['character'], 'character.png', { type: 'image/png' })
  )
  await within(picker).findByRole('button', { name: 'Preview: character.png' })
  await user.click(screen.getByRole('button', { name: 'Close panel' }))
  await user.click(
    screen.getByRole('button', {
      name: 'Add a reference video to extract motion',
    })
  )
  await user.upload(
    container.querySelector(
      'input[accept="video/mp4,video/quicktime,.mp4,.mov"]'
    ) as HTMLInputElement,
    new File(['valid clip'], 'reference.mp4', { type: 'video/mp4' })
  )
  await screen.findByRole('button', {
    name: 'Replace Add a reference video to extract motion',
  })
  expect(screen.queryByText('reference.mp4')).toBeNull()
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Generate video' })).toBeEnabled()
  )
  await user.click(screen.getByRole('button', { name: /Style preset/ }))
  const dialog = await screen.findByRole('dialog', { name: 'Choose a style' })
  await user.click(
    within(dialog).getByRole('option', { name: 'Hand-Painted Cel Animation' })
  )
  await user.click(within(dialog).getByRole('button', { name: 'Apply style' }))
  await waitFor(() =>
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  )
  expect(screen.getByText('8s')).toBeInTheDocument()
}

function pastePrompt(text: string): void {
  const editor = screen.getByRole('textbox', { name: 'Prompt' })
  fireEvent.paste(editor, {
    clipboardData: {
      getData: (type: string) => (type === 'text/plain' ? text : ''),
      files: [],
    },
  })
}

function mockSubmissionNetwork() {
  return vi.spyOn(api, 'post').mockImplementation(async (url, body) => {
    if (url === '/api/v1/files/upload/presign') {
      // Keep style assertions on the real uploader's documented fallback.
      throw { response: { status: 501 } }
    }
    if (url === '/api/v1/files/upload/stream') {
      const file = (body as FormData).get('file') as File
      return {
        data: {
          success: true,
          data: {
            file_url: file.type.startsWith('video/')
              ? VIDEO_URL
              : CHARACTER_URL,
          },
        },
      }
    }
    return { data: { task_id: 'accepted-test-only' } }
  })
}

describe('real motion workbench style submission', () => {
  it('posts the selected style plus two newlines and the conflicting user original without replacing uploaded assets or asking for conflict confirmation', async () => {
    const network = mockSubmissionNetwork()
    const user = userEvent.setup()
    await prepareMotionWithStyle(user)
    await user.click(screen.getByRole('switch', { name: 'Prompt' }))
    pastePrompt(`  ${USER_PROMPT}  `)
    await user.click(screen.getByRole('button', { name: 'Generate video' }))
    await waitFor(() =>
      expect(
        network.mock.calls.some(([url]) => url === '/pg/influencer/motion')
      ).toBe(true)
    )
    expect(
      network.mock.calls.find(([url]) => url === '/pg/influencer/motion')?.[1]
    ).toEqual({
      mode: 'motion',
      prompt: `${CEL_PROMPT}\n\n${USER_PROMPT}`,
      subject_image_url: CHARACTER_URL,
      subject_image_urls: [CHARACTER_URL],
      video_url: VIDEO_URL,
      duration_sec: 8,
      video_metadata: RAW_VIDEO_METADATA,
      aspect_ratio: '9:16',
      resolution: '480p',
    })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(
      document.querySelector('[data-sonner-toast]')
    ).not.toBeInTheDocument()
  })

  it('posts only the selected style after the user turns prompt input off and preserves the excluded draft', async () => {
    const network = mockSubmissionNetwork()
    const user = userEvent.setup()
    await prepareMotionWithStyle(user)
    await user.click(screen.getByRole('switch', { name: 'Prompt' }))
    const draft = '字'.repeat(3900)
    pastePrompt(draft)
    await user.click(screen.getByRole('switch', { name: 'Prompt' }))
    expect(screen.getByRole('switch', { name: 'Prompt' })).toHaveAttribute(
      'aria-checked',
      'false'
    )
    expect(
      screen.queryByRole('textbox', { name: 'Prompt' })
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Generate video' }))
    await waitFor(() =>
      expect(
        network.mock.calls.some(([url]) => url === '/pg/influencer/motion')
      ).toBe(true)
    )
    expect(
      network.mock.calls.find(([url]) => url === '/pg/influencer/motion')?.[1]
    ).toEqual({
      mode: 'motion',
      prompt: CEL_PROMPT,
      subject_image_url: CHARACTER_URL,
      subject_image_urls: [CHARACTER_URL],
      video_url: VIDEO_URL,
      duration_sec: 8,
      video_metadata: RAW_VIDEO_METADATA,
      aspect_ratio: '9:16',
      resolution: '480p',
    })
    await user.click(
      within(screen.getByRole('navigation', { name: 'Workspace' })).getByRole(
        'button',
        { name: 'Create' }
      )
    )
    await user.click(screen.getByRole('switch', { name: 'Prompt' }))
    expect(screen.getByRole('textbox', { name: 'Prompt' }).textContent).toBe(
      draft
    )
    expect(
      screen.queryByText('Prompt must be 4000 characters or fewer.')
    ).not.toBeInTheDocument()
  })

  it('shows only the length error and retains inputs without posting generation when style plus user text exceeds 4000 characters', async () => {
    const network = mockSubmissionNetwork()
    const user = userEvent.setup()
    await prepareMotionWithStyle(user)
    await user.click(screen.getByRole('switch', { name: 'Prompt' }))
    const longPrompt = '字'.repeat(3900)
    pastePrompt(longPrompt)
    await user.click(screen.getByRole('button', { name: 'Generate video' }))
    expect(
      await screen.findByText('Prompt must be 4000 characters or fewer.')
    ).toBeInTheDocument()
    expect(
      network.mock.calls.filter(([url]) => url === '/pg/influencer/motion')
    ).toHaveLength(0)
    expect(screen.getByRole('textbox', { name: 'Prompt' }).textContent).toBe(
      longPrompt
    )
    expect(
      screen.getByRole('button', { name: 'Preview: character.png' })
    ).toBeInTheDocument()
    expect(screen.queryByText('reference.mp4')).toBeNull()
    expect(screen.getByText('8s')).toBeInTheDocument()
    expect(
      screen.getByRole('button', {
        name: /Style preset.*Hand-Painted Cel Animation/,
      })
    ).toBeInTheDocument()
    expect(document.querySelectorAll('[data-sonner-toast]')).toHaveLength(1)
    expect(screen.getByRole('button', { name: 'Generate video' })).toBeEnabled()
  })
})
