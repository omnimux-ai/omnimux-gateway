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
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { createAppQueryClient } from '@/lib/query-client'

import { InfluencerWorkbench } from '../index'
import { readBrowserVideoMetadata } from '../lib/browser-video-metadata'
import type { InfluencerTaxonomy, TaskRecord } from '../types'
import { mockReferenceLayout } from './reference-layout-fixture'

beforeEach(mockReferenceLayout)

const RAW_VIDEO_METADATA = {
  duration: 8,
  video_duration: 8,
  width: 1920,
  height: 1080,
  frame_rate: 30,
  video_codec: 'h264',
  audio_codec: 'aac',
  container: 'mp4',
  size_bytes: 4,
}

vi.mock('../lib/browser-video-metadata', () => ({
  readBrowserVideoMetadata: vi.fn(),
}))

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: { get, post } }))
vi.mock('@tanstack/react-router', () => ({
  useSearch: () => ({}),
  useNavigate: () => vi.fn(),
}))
vi.mock('@/stores/auth-store', () => {
  const state = {
    auth: { user: { id: 1, username: 'tester' }, accessToken: 'test-token' },
  }
  return {
    useAuthStore: Object.assign(
      (select: (value: unknown) => unknown) => select(state),
      { getState: () => state }
    ),
  }
})
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
  data: { success: true, data: [], vendors: [], group_ratio: {} },
}
const GROUPS = { data: { success: true, data: { default: { ratio: 1 } } } }

const SHEET: TaskRecord = {
  id: 1,
  task_id: 'sheet-older',
  action: 'influencer_sheet',
  status: 'SUCCESS',
  submit_time: 1000,
  properties: {
    input: JSON.stringify({
      tier: 't1',
      selection: { gender: ['male'] },
      size: '9:16',
    }),
  },
  artifacts: [
    {
      key: 'image',
      content_url:
        'https://gw.example/v1/tasks/sheet-older/artifacts/image/content',
    },
  ],
}

const MOTION: TaskRecord = {
  id: 2,
  task_id: 'motion-newer',
  action: 'influencer_motion',
  status: 'SUCCESS',
  submit_time: 2000,
  properties: { input: JSON.stringify({ kind: 'motion', mode: 'motion' }) },
  artifacts: [
    {
      key: 'video',
      content_url:
        'https://gw.example/v1/tasks/motion-newer/artifacts/video/content',
    },
  ],
}

function routeByUrl(url: string, params: Record<string, unknown>) {
  if (url.includes('/pricing')) return PRICING
  if (url.includes('/taxonomy')) {
    return { data: { success: true, data: TAXONOMY, etag: 'test' } }
  }
  if (url.includes('/groups')) return GROUPS
  if (url.includes('/task/self')) {
    if (params.action === 'influencer_sheet') {
      return { data: { success: true, data: { items: [SHEET], total: 1 } } }
    }
    if (params.action === 'influencer_motion') {
      return { data: { success: true, data: { items: [MOTION], total: 1 } } }
    }
    return { data: { success: true, data: { items: [], total: 0 } } }
  }
  return { data: { success: true, data: {} } }
}

function renderWorkbench() {
  return render(
    <QueryClientProvider client={createAppQueryClient()}>
      <InfluencerWorkbench />
    </QueryClientProvider>
  )
}

describe('InfluencerWorkbench unified history', () => {
  it('opens video and image pickers at the same assets-group top rather than their individual slot tops', async () => {
    const user = userEvent.setup()
    renderWorkbench()
    await user.click(screen.getByRole('radio', { name: 'Motion video' }))
    for (const name of [
      'Add a reference video to extract motion',
      'Add character, product or clothing images',
    ]) {
      await user.click(screen.getByRole('button', { name }))
      expect(
        screen.getByRole('complementary', { name: /Select reference/ })
      ).toHaveStyle({ top: '0px' })
      await user.click(screen.getByRole('button', { name: 'Close panel' }))
    }
  })
  it.each(['card', 'preview'] as const)(
    'adds a history image from %s to video references without generating or duplicating',
    async (entry) => {
      const user = userEvent.setup()
      renderWorkbench()
      const openHistory = async () => {
        await user.click(
          within(
            screen.getByRole('navigation', { name: 'Workspace' })
          ).getByRole('button', { name: 'History' })
        )
        const downloads = await screen.findAllByRole('link', {
          name: 'Download',
        })
        const card = downloads
          .find((link) => link.getAttribute('href')?.includes('sheet-older'))
          ?.closest('.inf-card') as HTMLElement
        if (entry === 'preview') {
          await user.click(card)
          await user.click(
            within(
              await screen.findByRole('dialog', { name: 'Preview' })
            ).getByRole('button', { name: 'Turn to motion' })
          )
        } else {
          await user.click(
            within(card).getByRole('button', { name: 'Turn to motion' })
          )
        }
      }
      await openHistory()
      expect(
        screen.queryByRole('dialog', { name: 'Preview' })
      ).not.toBeInTheDocument()
      expect(
        screen.getByRole('radio', { name: 'Motion video' })
      ).toHaveAttribute('aria-checked', 'true')
      expect(screen.getAllByRole('button', { name: 'Remove' })).toHaveLength(1)
      await openHistory()
      expect(screen.getAllByRole('button', { name: 'Remove' })).toHaveLength(1)
      expect(post).not.toHaveBeenCalled()
      await user.click(
        screen.getByRole('button', { name: 'Add reference image' })
      )
      const picker = await screen.findByRole('complementary', {
        name: 'Select reference images',
      })
      expect(
        within(picker).getByRole('button', { name: 'Character' })
      ).toHaveAttribute('aria-pressed', 'true')
      await user.click(
        within(picker).getByRole('button', { name: 'Character' })
      )
      expect(
        screen.queryByRole('button', { name: 'Remove' })
      ).not.toBeInTheDocument()
    }
  )
  it('refills image and video creation forms from history without submitting or adding queued cards', async () => {
    const user = userEvent.setup()
    const storedVideo = {
      ...MOTION,
      properties: {
        input: JSON.stringify({
          kind: 'motion',
          mode: 'motion',
          prompt: 'Move',
          subject_image_url: 'https://example.org/image.png',
          video_url: 'https://example.org/video.mp4',
          duration_sec: 8,
          size: '16:9',
          resolution: '720p',
        }),
      },
    }
    get.mockImplementation(
      (url: string, config?: { params?: Record<string, unknown> }) => {
        if (url.includes('/task/self') && config?.params?.task_id) {
          return Promise.reject(new Error('Read unavailable'))
        }
        if (
          url.includes('/task/self') &&
          config?.params?.action === 'influencer_sheet'
        ) {
          const sheet = {
            ...SHEET,
            properties: {
              input: JSON.stringify({
                tier: 'normal',
                selection: { gender: ['male'] },
                size: '16:9',
              }),
            },
          }
          return Promise.resolve({
            data: { success: true, data: { items: [sheet], total: 1 } },
          })
        }
        if (
          url.includes('/task/self') &&
          config?.params?.action === 'influencer_motion'
        ) {
          return Promise.resolve({
            data: { success: true, data: { items: [storedVideo], total: 1 } },
          })
        }
        return Promise.resolve(routeByUrl(url, config?.params ?? {}))
      }
    )
    renderWorkbench()
    await user.click(await screen.findByRole('radio', { name: 'History' }))
    for (const id of ['sheet-older', 'motion-newer']) {
      const links = await screen.findAllByRole('link', { name: 'Download' })
      await user.click(
        links
          .find((link) => link.getAttribute('href')?.includes(id))
          ?.closest('.inf-card') as HTMLElement
      )
      await user.click(
        within(
          await screen.findByRole('dialog', { name: 'Preview' })
        ).getByRole('button', { name: 'Regenerate' })
      )
      await waitFor(() =>
        expect(
          screen.queryByRole('dialog', { name: 'Preview' })
        ).not.toBeInTheDocument()
      )
      expect(post).not.toHaveBeenCalled()
      await waitFor(() =>
        expect(
          screen.getByRole('radio', {
            name: id === 'sheet-older' ? 'Character design' : 'Motion video',
          })
        ).toHaveAttribute('aria-checked', 'true')
      )
      const navigation = screen.getByRole('navigation', { name: 'Workspace' })
      expect(
        within(navigation).getByRole('button', { name: 'Create' })
      ).toHaveAttribute('aria-pressed', 'true')
      if (id === 'sheet-older') {
        expect(screen.getByRole('button', { name: /Male/ })).toHaveAttribute(
          'aria-pressed',
          'true'
        )
      } else {
        expect(screen.getByRole('switch', { name: 'Prompt' })).toBeChecked()
        expect(screen.getByRole('textbox')).toHaveTextContent('Move')
        expect(screen.getByRole('radio', { name: '720P' })).toHaveAttribute(
          'aria-checked',
          'true'
        )
        expect(screen.getAllByRole('button', { name: /^Remove/ })).toHaveLength(
          2
        )
      }
      await user.click(
        within(navigation).getByRole('button', { name: 'History' })
      )
      expect(
        await screen.findAllByRole('link', { name: 'Download' })
      ).toHaveLength(2)
      expect(screen.queryByText('Queued')).not.toBeInTheDocument()
    }
    expect(post).not.toHaveBeenCalled()
  })

  it('keeps selected character settings when switching mobile workspaces', async () => {
    const user = userEvent.setup()
    renderWorkbench()
    const option = await screen.findByRole('button', { name: /Bold/ })
    await user.click(option)
    const navigation = screen.getByRole('navigation', { name: 'Workspace' })
    await user.click(
      within(navigation).getByRole('button', { name: 'History' })
    )
    expect(
      within(navigation).getByRole('button', { name: 'History' })
    ).toHaveAttribute('aria-pressed', 'true')
    await user.click(within(navigation).getByRole('button', { name: 'Create' }))
    expect(option).toHaveAttribute('aria-pressed', 'true')
  })

  it('opens a saved image in a closable dialog with an image download', async () => {
    const user = userEvent.setup()
    renderWorkbench()
    await user.click(
      within(screen.getByRole('navigation', { name: 'Workspace' })).getByRole(
        'button',
        { name: 'History' }
      )
    )
    const downloads = await screen.findAllByRole('link', { name: 'Download' })
    const imageCard = downloads
      .find((link) => link.getAttribute('href')?.includes('sheet-older'))
      ?.closest('.inf-card')
    expect(imageCard).not.toBeNull()
    await user.click(imageCard as HTMLElement)
    const dialog = await screen.findByRole('dialog', { name: 'Preview' })
    expect(
      within(dialog).getByRole('link', { name: 'Download' })
    ).toHaveAttribute('href', expect.stringContaining('sheet-older'))
    await user.keyboard('{Escape}')
    expect(
      screen.queryByRole('dialog', { name: 'Preview' })
    ).not.toBeInTheDocument()
  })

  it('retains video character, prompt and resolution after visiting history', async () => {
    const user = userEvent.setup()
    renderWorkbench()
    await user.click(await screen.findByRole('radio', { name: 'Motion video' }))
    await user.click(
      screen.getByRole('button', {
        name: 'Add character, product or clothing images',
      })
    )
    await user.click(
      within(
        await screen.findByRole('complementary', {
          name: 'Select reference images',
        })
      ).getAllByRole('button', { name: 'Character' })[0]
    )
    await user.click(screen.getByRole('button', { name: 'Close panel' }))
    await user.click(screen.getByRole('switch', { name: 'Prompt' }))
    fireEvent.paste(screen.getByRole('textbox'), {
      clipboardData: {
        getData: (type: string) =>
          type === 'text/plain' ? 'Keep this phone draft' : '',
        files: [],
      },
    })
    await user.click(screen.getByRole('radio', { name: '720P' }))
    const navigation = screen.getByRole('navigation', { name: 'Workspace' })
    await user.click(
      within(navigation).getByRole('button', { name: 'History' })
    )
    await user.click(within(navigation).getByRole('button', { name: 'Create' }))
    expect(screen.getByRole('textbox')).toHaveTextContent(
      'Keep this phone draft'
    )
    expect(screen.getByRole('radio', { name: '720P' })).toHaveAttribute(
      'aria-checked',
      'true'
    )
    expect(screen.getByRole('button', { name: 'Remove' })).toBeInTheDocument()
  })

  it('refills stored replacement video settings without submitting and preserves identity on explicit generation', async () => {
    const user = userEvent.setup()
    const input = {
      kind: 'motion',
      mode: 'replace',
      prompt: 'Stored direction',
      subject_character_type: 'cat',
      source_task_id: 'cat-source',
      subject_image_url: 'https://example.org/one.png',
      subject_image_urls: [
        'https://example.org/one.png',
        'https://example.org/two.png',
      ],
      video_url: 'https://example.org/original.mp4',
      duration_sec: 8,
      size: '16:9',
      resolution: '720p',
    }
    get.mockImplementation(
      (url: string, config?: { params?: Record<string, unknown> }) => {
        if (
          url.includes('/task/self') &&
          config?.params?.action === 'influencer_motion'
        ) {
          return Promise.resolve({
            data: {
              success: true,
              data: {
                items: [
                  { ...MOTION, properties: { input: JSON.stringify(input) } },
                ],
                total: 1,
              },
            },
          })
        }
        return Promise.resolve(routeByUrl(url, config?.params ?? {}))
      }
    )
    post.mockResolvedValue({ data: { task_id: 'replacement-generated' } })
    renderWorkbench()
    await user.click(await screen.findByRole('radio', { name: 'History' }))
    const links = await screen.findAllByRole('link', { name: 'Download' })
    await user.click(
      links
        .find((x) => x.getAttribute('href')?.includes('motion-newer'))
        ?.closest('.inf-card') as HTMLElement
    )
    const dialog = await screen.findByRole('dialog', { name: 'Preview' })
    await user.click(within(dialog).getByRole('button', { name: 'Regenerate' }))
    await waitFor(() =>
      expect(
        screen.queryByRole('dialog', { name: 'Preview' })
      ).not.toBeInTheDocument()
    )
    expect(post).not.toHaveBeenCalled()
    await waitFor(() =>
      expect(
        screen.getByRole('radio', { name: 'Motion video' })
      ).toHaveAttribute('aria-checked', 'true')
    )
    expect(screen.getByRole('radio', { name: 'Object swap' })).toHaveAttribute(
      'aria-checked',
      'true'
    )
    expect(screen.getByRole('switch', { name: 'Prompt' })).toBeChecked()
    expect(screen.getByRole('textbox')).toHaveTextContent(input.prompt)
    expect(screen.getByRole('radio', { name: '720P' })).toHaveAttribute(
      'aria-checked',
      'true'
    )
    expect(readBrowserVideoMetadata).toHaveBeenCalledExactlyOnceWith(
      input.video_url,
      expect.any(AbortSignal)
    )
    expect(screen.getAllByRole('button', { name: /^Remove/ })).toHaveLength(3)
    expect(
      screen.getByRole('button', { name: 'Preview: one.png' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Preview: two.png' })
    ).toBeInTheDocument()
    const generate = screen.getByRole('button', { name: 'Generate video' })
    await waitFor(() => expect(generate).toBeEnabled())
    await user.click(generate)
    await waitFor(() =>
      expect(post).toHaveBeenCalledWith(
        '/pg/influencer/motion',
        expect.objectContaining({
          mode: 'replace',
          prompt: input.prompt,
          subject_image_urls: input.subject_image_urls,
          subject_character_type: 'cat',
          source_task_id: 'cat-source',
          video_url: input.video_url,
          aspect_ratio: '16:9',
          duration_sec: 8,
          video_metadata: RAW_VIDEO_METADATA,
          resolution: '720p',
        }),
        expect.anything()
      )
    )
    expect(
      post.mock.calls.filter(([url]) => url === '/pg/influencer/motion')
    ).toHaveLength(1)
  })

  it.each([
    { subject_character_type: 'unknown' },
    { subject_character_type: null },
    { source_task_id: 42 },
  ])(
    'damaged stored video identity %j prevents regeneration instead of silently changing the subject',
    async (identity) => {
      const user = userEvent.setup()
      const input = {
        kind: 'motion',
        mode: 'replace',
        prompt: '',
        subject_image_url: 'https://files.example/cat.png',
        video_url: 'https://files.example/clip.mp4',
        duration_sec: 5,
        size: '16:9',
        resolution: '720p',
        ...identity,
      }
      get.mockImplementation(
        (url: string, config?: { params?: Record<string, unknown> }) => {
          if (
            url.includes('/task/self') &&
            config?.params?.action === 'influencer_motion'
          ) {
            return Promise.resolve({
              data: {
                success: true,
                data: {
                  items: [
                    { ...MOTION, properties: { input: JSON.stringify(input) } },
                  ],
                  total: 1,
                },
              },
            })
          }
          return Promise.resolve(routeByUrl(url, config?.params ?? {}))
        }
      )
      renderWorkbench()
      await user.click(await screen.findByRole('radio', { name: 'History' }))
      const links = await screen.findAllByRole('link', { name: 'Download' })
      await user.click(
        links
          .find((link) => link.getAttribute('href')?.includes('motion-newer'))
          ?.closest('.inf-card') as HTMLElement
      )
      await user.click(
        within(
          await screen.findByRole('dialog', { name: 'Preview' })
        ).getByRole('button', { name: 'Regenerate' })
      )
      expect(post).not.toHaveBeenCalled()
      expect(
        screen.getByRole('dialog', { name: 'Preview' })
      ).toBeInTheDocument()
    }
  )

  beforeEach(() => {
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
    post.mockReset()
    post.mockResolvedValue({ data: {} })
    vi.mocked(readBrowserVideoMetadata)
      .mockReset()
      .mockResolvedValue(RAW_VIDEO_METADATA)
    get.mockReset()
    get.mockImplementation(
      (url: string, config?: { params?: Record<string, unknown> }) =>
        Promise.resolve(routeByUrl(url, config?.params ?? {}))
    )
    localStorage.clear()
  })

  it('fetches both the image and video task lists for the history feed', async () => {
    const user = userEvent.setup()
    renderWorkbench()

    await user.click(await screen.findByRole('radio', { name: 'History' }))
    await screen.findAllByRole('link', { name: 'Download' })

    const taskCalls = get.mock.calls.filter(
      ([url]) => typeof url === 'string' && url.includes('/task/self')
    )
    const actions = taskCalls.map(
      ([, config]) => (config as { params: { action: string } }).params.action
    )
    expect(actions).toContain('influencer_sheet')
    expect(actions).toContain('influencer_motion')
  })

  it('updates an already open detail when successful missing media is enriched, without submitting generation', async () => {
    let supplyMedia!: () => void
    const enrichment = new Promise((resolve) => {
      supplyMedia = () =>
        resolve({ data: { success: true, data: { items: [SHEET], total: 1 } } })
    })
    get.mockImplementation(
      (url: string, config?: { params?: Record<string, unknown> }) => {
        const params = config?.params ?? {}
        if (
          url.includes('/task/self') &&
          params.action === 'influencer_sheet'
        ) {
          if (params.task_id) return enrichment
          return Promise.resolve({
            data: {
              success: true,
              data: { items: [{ ...SHEET, artifacts: [] }], total: 1 },
            },
          })
        }
        return Promise.resolve(routeByUrl(url, params))
      }
    )
    const user = userEvent.setup()
    const view = renderWorkbench()
    await user.click(await screen.findByRole('radio', { name: 'History' }))
    await waitFor(() =>
      expect(view.container.querySelectorAll('.inf-card')).toHaveLength(2)
    )
    const cards = view.container.querySelectorAll('.inf-card')
    await user.click(cards[1] as HTMLElement)
    const dialog = await screen.findByRole('dialog', { name: 'Preview' })
    expect(
      within(dialog).queryByRole('img', { name: 'Preview' })
    ).not.toBeInTheDocument()
    supplyMedia()
    await waitFor(() =>
      expect(
        within(dialog).getByRole('img', { name: 'Preview' })
      ).toBeInTheDocument()
    )
    expect(view.container.querySelector('.inf-card img')).toBeInTheDocument()
    expect(post).not.toHaveBeenCalled()
  })

  it('renders image and video records in one feed ordered newest-first', async () => {
    const user = userEvent.setup()
    renderWorkbench()

    await user.click(await screen.findByRole('radio', { name: 'History' }))
    const downloads = await screen.findAllByRole('link', { name: 'Download' })

    expect(downloads).toHaveLength(2)
    // The newer motion record must paint before the older sheet record.
    expect(downloads[0].getAttribute('href')).toContain('motion-newer')
    expect(downloads[1].getAttribute('href')).toContain('sheet-older')
  })
})
