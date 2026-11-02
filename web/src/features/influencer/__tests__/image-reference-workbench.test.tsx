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
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { InfluencerWorkbench } from '../index'
import { clearSession } from '../lib/workbench-session'
import { writeGuestDraft } from '../lib/guest'

const taxonomy = {
  version: 1,
  source: 'test',
  tier_group: { options: [{ id: 'total', label_en: 'Extreme' }] },
  categories: [
    {
      id: 'body',
      label_en: 'Body',
      kind: 'single',
      options: [{ id: 'curvy', label_en: 'Curvy', visibleIn: ['total'] }],
    },
  ],
  category_priority: ['body'],
  prompt_map: { tiers: {}, categories: {} },
}
const post = () => vi.spyOn(api, 'post')
// Narrow UI fixtures use the documented unsupported-capability fallback.
const unsupportedPresign = { response: { status: 501 } }
function mount(guest = false) {
  if (!guest) {
    useAuthStore.setState((state) => ({
      auth: {
        ...state.auth,
        user: { id: 1, username: 'test', role: 1 },
        accessToken: 'test-only',
      },
    }))
  }
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const root = createRootRoute({
    component: () => (
      <QueryClientProvider client={client}>
        <InfluencerWorkbench />
      </QueryClientProvider>
    ),
  })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  const view = render(<RouterProvider router={router} />)
  return { ...view, client }
}
beforeEach(() => {
  localStorage.clear()
  sessionStorage.clear()
  clearSession()
  useAuthStore.getState().auth.reset()
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (String(url).includes('taxonomy')) {
      return { data: { success: true, data: taxonomy } }
    }
    if (String(url).includes('pricing')) {
      return {
        data: {
          success: true,
          data: [
            {
              model_name: 'gpt-image-2.5-sunburst',
              model_price: 0.04,
              enable_groups: [
                'gpt-image-2.5-goeasy',
                'gpt-image-2.5-sunburst-image',
              ],
              supported_endpoint_types: ['image-generation'],
              vendor_id: 1,
            },
          ],
          group_ratio: {
            'gpt-image-2.5-goeasy': 0.5,
            'gpt-image-2.5-sunburst-image': 1,
          },
        },
      }
    }
    if (String(url).includes('/groups')) {
      return {
        data: {
          success: true,
          data: {
            'gpt-image-2.5-goeasy': { ratio: 0.5 },
            'gpt-image-2.5-sunburst-image': { ratio: 1 },
          },
        },
      }
    }
    return { data: { success: true, data: { items: [], total: 0 } } }
  })
})
afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})
const image = new File(['png'], 'style.png', { type: 'image/png' })
function input(container: HTMLElement) {
  return container.querySelector('input[type=file]') as HTMLInputElement
}

describe('real creation page style reference', () => {
  it('keeps selected character features when an upload becomes a style reference without a free description', async () => {
    const network = post().mockImplementation(async (url) => {
      if (url === '/api/v1/files/upload/presign') throw unsupportedPresign
      if (url === '/api/v1/files/upload/stream') {
        return {
          data: {
            success: true,
            data: { file_url: 'https://files.example/owned-source.png' },
          },
        }
      }
      return { data: { task_id: 'reference-accepted' } }
    })
    const { container } = mount()
    await screen.findByRole('button', { name: 'Style reference' })
    fireEvent.change(input(container), { target: { files: [image] } })
    await screen.findByRole('button', { name: 'Replace Style reference' })
    expect(screen.queryByText('Describe your image')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('textbox', { name: 'Describe your image' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Curvy' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expect(
      screen.queryByRole('button', { name: 'Edit whole image' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Edit selected area' })
    ).not.toBeInTheDocument()
    const generate = screen.getByRole('button', { name: /^Generate/ })
    await waitFor(() => expect(generate).toBeEnabled())
    fireEvent.click(generate)
    await waitFor(() =>
      expect(
        network.mock.calls.filter(([url]) => url === '/pg/influencer/sheet')
      ).toHaveLength(1)
    )
    const payload = network.mock.calls.find(
      ([url]) => url === '/pg/influencer/sheet'
    )?.[1]
    expect(payload).toMatchObject({
      model: 'gpt-image-2.5-sunburst',
      group: 'gpt-image-2.5-sunburst-image',
      size: '1024x576',
      image_mode: 'reference',
      image_url: 'https://files.example/owned-source.png',
      brief: '',
      selection: { body: ['curvy'] },
    })
    expect(payload).not.toHaveProperty('mask_url')
    expect(payload).not.toHaveProperty('edit_instruction')
  })

  it.each([
    ['image/jpeg', 'source.jpg'],
    ['image/png', 'source.png'],
    ['image/webp', 'source.webp'],
  ])(
    'accepts %s as a style reference without exposing an image editor',
    async (mime, name) => {
      const network = post().mockImplementation(async (url) => {
        if (url === '/api/v1/files/upload/presign') throw unsupportedPresign
        return {
          data: {
            success: true,
            data: { file_url: 'https://files.example/owned-source.bin' },
          },
        }
      })
      const { container } = mount()
      await screen.findByRole('button', { name: 'Style reference' })
      fireEvent.change(input(container), {
        target: { files: [new File(['image'], name, { type: mime })] },
      })
      await screen.findByRole('button', { name: 'Replace Style reference' })
      expect(screen.queryByRole('alert')).not.toBeInTheDocument()
      expect(
        screen.queryByRole('img', { name: 'Edit selection canvas' })
      ).not.toBeInTheDocument()
      expect(
        screen.queryByRole('textbox', { name: 'Describe your changes' })
      ).not.toBeInTheDocument()
      expect(
        screen.queryByRole('textbox', { name: 'Describe your image' })
      ).not.toBeInTheDocument()
      await waitFor(() =>
        expect(screen.getByRole('button', { name: /^Generate/ })).toBeEnabled()
      )
      expect(
        network.mock.calls.filter(([url]) => url === '/pg/influencer/sheet')
      ).toHaveLength(0)
    }
  )

  it('keeps empty ordinary creation disabled when all features and references are absent', async () => {
    const network = post()
    mount()
    await screen.findByRole('button', { name: 'Style reference' })
    fireEvent.click(screen.getByRole('button', { name: 'Reset features' }))
    expect(
      screen.queryByRole('textbox', { name: 'Describe your image' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Generate/ })).toBeDisabled()
    expect(
      screen.queryByRole('button', { name: 'Edit selected area' })
    ).not.toBeInTheDocument()
    expect(network).not.toHaveBeenCalled()
  })

  it('does not borrow an old image price when the exact Sunburst catalog is absent', async () => {
    const get = vi.spyOn(api, 'get')
    const fallback = get.getMockImplementation()
    if (!fallback) throw new Error('missing network fixture')
    get.mockImplementation(async (...args) => {
      if (String(args[0]).includes('pricing')) {
        return {
          data: {
            success: true,
            data: [
              {
                model_name: 'gpt-image-2.5',
                model_price: 0.001,
                supported_endpoint_types: ['image-generation'],
                enable_groups: ['gpt-image-2.5-sunburst-image'],
                vendor_id: 1,
              },
            ],
            group_ratio: { 'gpt-image-2.5-sunburst-image': 1 },
          },
        }
      }
      return fallback(...args)
    })
    mount()
    const unavailable = await screen.findByRole('button', {
      name: 'Temporarily unavailable',
    })
    expect(unavailable).toBeDisabled()
    expect(unavailable).toHaveAttribute(
      'title',
      'Image generation is temporarily unavailable'
    )
  })

  it('falls back to the audited economy route instead of closing image generation', async () => {
    const get = vi.spyOn(api, 'get')
    const fallback = get.getMockImplementation()
    if (!fallback) throw new Error('missing network fixture')
    get.mockImplementation(async (...args) => {
      if (String(args[0]).includes('pricing')) {
        return {
          data: {
            success: true,
            data: [
              {
                model_name: 'gpt-image-2.5',
                model_price: 0.001,
                billing_mode: 'tiered_expr',
                billing_expr: 'tier("image", fixed(0.013072)) * image_count',
                supported_endpoint_types: ['image-generation'],
                enable_groups: ['gpt-image-2.5-economy'],
                vendor_id: 1,
              },
            ],
            group_ratio: { 'gpt-image-2.5-economy': 0.112455 },
          },
        }
      }
      if (String(args[0]).includes('/groups')) {
        return {
          data: {
            success: true,
            data: { 'gpt-image-2.5-economy': { ratio: 0.112455 } },
          },
        }
      }
      return fallback(...args)
    })
    const network = post().mockResolvedValue({
      data: { task_id: 'economy-accepted' },
    })
    mount()
    await screen.findByRole('button', { name: 'Style reference' })
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    const generate = await screen.findByRole('button', { name: /^Generate/ })
    expect(generate).toHaveTextContent('✦ 0.0147')
    await waitFor(() => expect(generate).toBeEnabled())
    fireEvent.click(generate)
    await waitFor(() => expect(network).toHaveBeenCalledTimes(1))
    const payload = network.mock.calls[0]?.[1] as Record<string, unknown>
    expect(payload).toMatchObject({
      model: 'gpt-image-2.5',
      group: 'gpt-image-2.5-economy',
      size: '16:9',
    })
    expect(payload).not.toHaveProperty('image_mode')
  })

  it.each(['new', 'restored'])(
    'uses the existing image quote anchor for a %s text generation even when another group is cheaper',
    async (draftKind) => {
      const network = post().mockResolvedValue({
        data: { task_id: 'accepted' },
      })
      if (draftKind === 'restored') {
        writeGuestDraft({
          tier: 'total',
          selection: { body: ['curvy'] },
          brief: 'Keep the hat',
          seed: '42',
          imageUrl: '',
          model: 'gpt-image-2.5-sunburst',
          group: 'gpt-image-2.5-goeasy',
          vendorId: 1,
        })
      }
      mount()
      const generate = await screen.findByRole('button', { name: /^Generate/ })
      await waitFor(() => expect(generate).toHaveTextContent('✦ 0.4'))
      await waitFor(() => expect(generate).toBeEnabled())
      fireEvent.click(generate)
      await waitFor(() => expect(network).toHaveBeenCalledTimes(1))
      expect(network.mock.calls[0]).toMatchObject([
        '/pg/influencer/sheet',
        {
          model: 'gpt-image-2.5-sunburst',
          group: 'gpt-image-2.5-sunburst-image',
          tier: 'total',
          size: '1024x576',
          ...(draftKind === 'restored'
            ? {
                brief: '',
                selection: { body: ['curvy'] },
              }
            : {}),
        },
        { headers: { 'X-Omnimux-Group': 'gpt-image-2.5-sunburst-image' } },
      ])
    }
  )

  it('keeps text generation unavailable rather than choosing an unrelated group when the quote anchor is missing', async () => {
    const get = vi.spyOn(api, 'get')
    const fallback = get.getMockImplementation()
    if (!fallback) throw new Error('missing network fixture')
    get.mockImplementation(async (...args) => {
      if (String(args[0]).includes('/groups')) {
        return {
          data: {
            success: true,
            data: { 'gpt-image-2.5-goeasy': { ratio: 0.5 } },
          },
        }
      }
      return fallback(...args)
    })
    const network = post()
    mount()
    await screen.findByRole('button', { name: 'Style reference' })
    const unavailable = await screen.findByRole('button', {
      name: 'Temporarily unavailable',
    })
    expect(unavailable).toBeDisabled()
    expect(unavailable).toHaveTextContent('Temporarily unavailable')
    expect(unavailable).not.toHaveTextContent('✦')
    fireEvent.click(unavailable)
    expect(network).not.toHaveBeenCalled()
  })

  it.each(['image', 'video'])(
    'removes a newly completed %s from the gallery without cancelling other accepted tasks',
    async (kind) => {
      if (kind === 'video') {
        vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue()
        vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(
          () => {}
        )
        vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(
          () => {}
        )
        const srcSetter = Object.getOwnPropertyDescriptor(
          HTMLMediaElement.prototype,
          'src'
        )?.set
        vi.spyOn(HTMLMediaElement.prototype, 'src', 'set').mockImplementation(
          function (this: HTMLMediaElement, value) {
            srcSetter?.call(this, value)
            Object.defineProperties(this, {
              duration: { configurable: true, value: 8 },
              videoWidth: { configurable: true, value: 1280 },
              videoHeight: { configurable: true, value: 720 },
            })
            queueMicrotask(() =>
              this.dispatchEvent(new Event('loadedmetadata'))
            )
          }
        )
      }
      const action = kind === 'video' ? 'influencer_motion' : 'influencer_sheet'
      const original = {
        id: 7,
        task_id: 'original',
        action,
        status: 'SUCCESS',
        artifacts: [
          {
            key: kind === 'video' ? 'video' : 'image',
            content_url: 'https://files.example/original',
          },
        ],
        properties: {
          input: JSON.stringify(
            kind === 'video'
              ? {
                  kind: 'motion',
                  mode: 'motion',
                  prompt: 'Move',
                  subject_image_url: 'https://files.example/image.png',
                  video_url: 'https://files.example/video.mp4',
                  duration_sec: 8,
                  resolution: '720p',
                  size: '16:9',
                }
              : {
                  tier: 'total',
                  selection: { body: ['curvy'] },
                  brief: 'Character',
                  size: '2048x1152',
                }
          ),
        },
      }
      const get = vi.spyOn(api, 'get')
      const fallback = get.getMockImplementation()
      if (!fallback) throw new Error('missing network fixture')
      get.mockImplementation(async (...args) => {
        if (String(args[0]).includes('/task/self')) {
          const id = args[1]?.params?.task_id
          let items = args[1]?.params?.action === action ? [original] : []
          if (id) {
            items = [
              {
                ...original,
                task_id: id,
                status: id === 'finished' ? 'SUCCESS' : 'WAITING',
                artifacts:
                  id === 'finished'
                    ? [
                        {
                          key: kind === 'video' ? 'video' : 'image',
                          content_url: 'https://files.example/finished',
                        },
                      ]
                    : undefined,
              } as typeof original,
            ]
          }
          return {
            data: { success: true, data: { items, total: items.length } },
          }
        }
        return fallback(...args)
      })
      const generated = ['finished', 'still-waiting']
      const network = post().mockImplementation(async (url) => {
        if (String(url).endsWith('/motion/inspect')) {
          return {
            data: {
              success: true,
              data: { duration: 8, width: 1280, height: 720 },
            },
          }
        }
        return { data: { task_id: generated.shift() } }
      })
      const generationCalls = () =>
        network.mock.calls.filter(
          ([url]) => !String(url).endsWith('/motion/inspect')
        )
      const cancel = vi.spyOn(api, 'delete')
      mount()
      fireEvent.click(
        (await screen.findAllByRole('button', { name: 'History' }))[0]
      )
      const regenerate = async () => {
        const links = await screen.findAllByRole('link', { name: 'Download' })
        fireEvent.click(
          links
            .find((link) => link.getAttribute('href')?.endsWith('/original'))
            ?.closest('.inf-card') as HTMLElement
        )
        const dialog = await screen.findByRole('dialog', { name: 'Preview' })
        const submitted = generationCalls().length
        fireEvent.click(
          within(dialog).getByRole('button', { name: 'Regenerate' })
        )
        await waitFor(() =>
          expect(
            screen.queryByRole('dialog', { name: 'Preview' })
          ).not.toBeInTheDocument()
        )
        // Reusing history fills the form; only the explicit Generate submits.
        expect(generationCalls()).toHaveLength(submitted)
        await waitFor(() =>
          expect(
            screen.getByRole('button', { name: /^Generate/ })
          ).toBeEnabled()
        )
        expect(generationCalls()).toHaveLength(submitted)
        fireEvent.click(screen.getByRole('button', { name: /^Generate/ }))
        await waitFor(() =>
          expect(generationCalls()).toHaveLength(submitted + 1)
        )
        fireEvent.click(screen.getAllByRole('button', { name: 'History' })[0])
      }
      await regenerate()
      await waitFor(() =>
        expect(
          screen
            .getAllByRole('link', { name: 'Download' })
            .some((link) => link.getAttribute('href')?.endsWith('/finished'))
        ).toBe(true)
      )
      await regenerate()
      await screen.findByText('Waiting for a free slot')
      fireEvent.click(
        screen
          .getAllByRole('link', { name: 'Download' })
          .find((link) => link.getAttribute('href')?.endsWith('/finished'))
          ?.closest('.inf-card') as HTMLElement
      )
      fireEvent.click(
        within(
          await screen.findByRole('dialog', { name: 'Preview' })
        ).getByRole('button', { name: 'Delete' })
      )
      let confirmation = await screen.findByRole('alertdialog', {
        name: 'Delete',
      })
      const deletionDescription = confirmation.textContent
      expect(confirmation).toHaveTextContent(
        'Source files will not be deleted.'
      )
      expect(cancel).not.toHaveBeenCalled()
      expect(screen.getByRole('dialog', { hidden: true })).toBeInTheDocument()
      expect(
        screen
          .getAllByRole('link', { name: 'Download', hidden: true })
          .some((link) => link.getAttribute('href')?.endsWith('/finished'))
      ).toBe(true)
      fireEvent.click(
        within(confirmation).getByRole('button', { name: 'Cancel' })
      )
      await waitFor(() =>
        expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
      )
      expect(
        screen
          .getAllByRole('link', { name: 'Download' })
          .some((link) => link.getAttribute('href')?.endsWith('/finished'))
      ).toBe(true)
      fireEvent.click(
        within(screen.getByRole('dialog', { name: 'Preview' })).getByRole(
          'button',
          { name: 'Delete' }
        )
      )
      confirmation = await screen.findByRole('alertdialog', { name: 'Delete' })
      fireEvent.click(
        within(confirmation).getByRole('button', { name: 'Delete' })
      )
      await waitFor(() =>
        expect(
          screen.queryByRole('dialog', { name: 'Preview' })
        ).not.toBeInTheDocument()
      )
      expect(
        screen
          .getAllByRole('link', { name: 'Download' })
          .some((link) => link.getAttribute('href')?.endsWith('/finished'))
      ).toBe(false)
      const reads = get.mock.calls.length
      await act(async () => {
        fireEvent(document, new Event('visibilitychange'))
      })
      expect(
        get.mock.calls
          .slice(reads)
          .some((call) => call[1]?.params?.task_id === 'still-waiting')
      ).toBe(true)
      expect(screen.getByText('Waiting for a free slot')).toBeInTheDocument()
      expect(generationCalls()).toHaveLength(2)
      expect(cancel).not.toHaveBeenCalled()
      expect(deletionDescription).toMatch(/history gallery/i)
      expect(deletionDescription).toMatch(/Source files will not be deleted/i)
    }
  )

  it.each(['image', 'video'])(
    'shows a failed %s history card reason without any retry submission',
    async (kind) => {
      const payload =
        kind === 'video'
          ? {
              kind: 'motion',
              mode: 'motion',
              prompt: 'Move',
              subject_image_url: 'https://files.example/image.png',
              video_url: 'https://files.example/video.mp4',
              duration_sec: 8,
              resolution: '720p',
              size: '16:9',
            }
          : {
              tier: 'total',
              selection: {},
              brief: 'Character',
              size: '2048x1152',
            }
      const action = kind === 'video' ? 'influencer_motion' : 'influencer_sheet'
      const get = vi.spyOn(api, 'get')
      const fallback = get.getMockImplementation()
      if (!fallback) throw new Error('missing network fixture')
      get.mockImplementation(async (...args) => {
        if (String(args[0]).includes('/task/self')) {
          const items =
            !args[1]?.params?.task_id && args[1]?.params?.action === action
              ? [
                  {
                    id: 7,
                    task_id: 'failed-original',
                    action,
                    status: 'FAILURE',
                    fail_reason: 'upstream rejected the task',
                    properties: { input: JSON.stringify(payload) },
                  },
                ]
              : []
          return {
            data: { success: true, data: { items, total: items.length } },
          }
        }
        return fallback(...args)
      })
      const network = post().mockResolvedValue({ data: { task_id: 'retried' } })
      mount()
      fireEvent.click(
        (await screen.findAllByRole('button', { name: 'History' }))[0]
      )
      // The provider text is unbounded, so the card shows the localized
      // fallback and keeps the original behind the Details disclosure.
      expect(
        await screen.findByText('Generation failed. Please try again.')
      ).toBeInTheDocument()
      expect(
        screen.getAllByRole('button', { name: 'Details' }).length
      ).toBeGreaterThan(0)
      expect(
        screen.queryByRole('button', { name: 'Retry' })
      ).not.toBeInTheDocument()
      expect(network).not.toHaveBeenCalled()
      expect(
        screen.queryByRole('dialog', { name: 'Preview' })
      ).not.toBeInTheDocument()
    }
  )

  it.each(['available', 'failed'])(
    'shows the accepted board artifacts when terminal projection is %s and lets history recover without hiding newer boards',
    async (projection) => {
      const parent = {
        id: 7,
        task_id: 'parent',
        status: 'SUCCESS',
        artifacts: [{ content_url: 'https://files.example/parent.png' }],
      }
      const board = (id: number, taskId: string, artifacts = false) => ({
        id,
        task_id: taskId,
        status: 'SUCCESS',
        properties: {
          input: JSON.stringify({
            kind: 'multiview',
            parent_task_id: 'parent',
          }),
        },
        artifacts: artifacts
          ? [{ content_url: `https://files.example/${taskId}.png` }]
          : undefined,
      })
      let history = [parent, board(6, 'old-board', true)]
      const get = vi.spyOn(api, 'get')
      const fallback = get.getMockImplementation()
      if (!fallback) throw new Error('missing network fixture')
      get.mockImplementation(async (...args) => {
        if (String(args[0]).includes('/task/self')) {
          const id = args[1]?.params?.task_id
          if (
            id &&
            args[1]?.params?.with_artifacts &&
            projection === 'failed'
          ) {
            throw new Error('Projection unavailable')
          }
          const items = id
            ? [
                board(
                  id === 'board' ? 8 : 9,
                  id,
                  Boolean(args[1]?.params?.with_artifacts)
                ),
              ]
            : history
          return {
            data: { success: true, data: { items, total: items.length } },
          }
        }
        return fallback(...args)
      })
      const network = post()
        .mockResolvedValueOnce({ data: { task_id: 'board' } })
        .mockResolvedValueOnce({ data: { task_id: 'new-board' } })
      const { client } = mount()
      fireEvent.click(
        (await screen.findAllByRole('button', { name: 'History' }))[0]
      )
      fireEvent.click(
        (await screen.findByRole('link', { name: 'Download' })).closest(
          '.inf-card'
        ) as HTMLElement
      )
      fireEvent.click(
        await screen.findByRole('button', { name: 'Generate multi-view' })
      )
      fireEvent.click(
        await screen.findByRole('button', { name: 'View multi-view' })
      )
      let dialog = await screen.findByRole('dialog', { name: 'Multi-view' })
      if (projection === 'failed') {
        expect(
          within(dialog).getByText('No preview available')
        ).toBeInTheDocument()
        history = [parent, board(8, 'board', true)]
        await act(async () => {
          await client.invalidateQueries({
            queryKey: ['influencer', 'history'],
          })
        })
      }
      await waitFor(() =>
        expect(
          within(dialog).getByRole('img', { name: 'Multi-view' })
        ).toHaveAttribute('src', 'https://files.example/board.png')
      )
      expect(
        within(dialog).getByRole('button', { name: 'Download' })
      ).toHaveAttribute('href', 'https://files.example/board.png')
      expect(network).toHaveBeenCalledTimes(1)
      if (projection === 'failed') {
        fireEvent.click(
          within(dialog).getByRole('button', { name: 'Regenerate' })
        )
        await waitFor(() =>
          expect(
            screen.queryByRole('dialog', { name: 'Multi-view' })
          ).not.toBeInTheDocument()
        )
        fireEvent.click(screen.getByRole('button', { name: 'View multi-view' }))
        dialog = await screen.findByRole('dialog', { name: 'Multi-view' })
        expect(
          within(dialog).getByText('No preview available')
        ).toBeInTheDocument()
        expect(
          within(dialog).queryByRole('img', { name: 'Multi-view' })
        ).not.toBeInTheDocument()
        history = [
          parent,
          board(10, 'server-newest', true),
          board(8, 'board', true),
        ]
        await act(async () => {
          await client.invalidateQueries({
            queryKey: ['influencer', 'history'],
          })
        })
        await waitFor(() =>
          expect(
            within(dialog).getByRole('img', { name: 'Multi-view' })
          ).toHaveAttribute('src', 'https://files.example/server-newest.png')
        )
        expect(network).toHaveBeenCalledTimes(2)
      }
    }
  )

  it.each([
    ['WAITING', '', 'Queued multi-view storyboard…', 'Waiting for a free slot'],
    ['CLAIMED', '', 'Submitting multi-view storyboard…', 'Submitting…'],
    ['DISPATCHING', '', 'Submitting multi-view storyboard…', 'Submitting…'],
    [
      'DISPATCHING',
      'Submission needs review; do not resubmit.',
      'Multi-view needs review',
      'Needs review',
    ],
    ['IN_PROGRESS', '', 'Generating multi-view storyboard…', 'Generating…'],
  ])(
    'shows %s accurately in the real multi-view badge and dialog without allowing retry',
    async (status, reason, badge, label) => {
      const get = vi.spyOn(api, 'get')
      const fallback = get.getMockImplementation()
      if (!fallback) throw new Error('missing network fixture')
      get.mockImplementation(async (...args) => {
        if (String(args[0]).includes('/task/self')) {
          const items = args[1]?.params?.task_id
            ? [
                {
                  id: 8,
                  task_id: 'board',
                  status,
                  fail_reason: reason,
                  progress: '25%',
                },
              ]
            : [
                {
                  id: 7,
                  task_id: 'parent',
                  status: 'SUCCESS',
                  artifacts: [
                    { content_url: 'https://files.example/parent.png' },
                  ],
                },
              ]
          return {
            data: { success: true, data: { items, total: items.length } },
          }
        }
        return fallback(...args)
      })
      const network = post().mockResolvedValue({ data: { task_id: 'board' } })
      mount()
      fireEvent.click(
        (await screen.findAllByRole('button', { name: 'History' }))[0]
      )
      fireEvent.click(
        (await screen.findByRole('link', { name: 'Download' })).closest(
          '.inf-card'
        ) as HTMLElement
      )
      fireEvent.click(
        await screen.findByRole('button', { name: 'Generate multi-view' })
      )
      fireEvent.click(await screen.findByRole('button', { name: badge }))
      const dialog = await screen.findByRole('dialog', { name: 'Multi-view' })
      expect(within(dialog).getByText(label)).toBeInTheDocument()
      // The internal reason is never shown; the user sees the bounded wait.
      if (reason) {
        expect(within(dialog).queryByText(reason)).not.toBeInTheDocument()
        expect(
          within(dialog).getByText(
            'Confirming the result. Unconfirmed tasks fail automatically within about 60 minutes and are never resent.'
          )
        ).toBeInTheDocument()
      }
      const regenerate = within(dialog).getByRole('button', {
        name: 'Regenerate',
      })
      const remove = within(dialog).getByRole('button', { name: 'Delete' })
      expect(regenerate).toBeDisabled()
      expect(remove).toBeDisabled()
      fireEvent.click(regenerate)
      fireEvent.click(remove)
      expect(network).toHaveBeenCalledTimes(1)
    }
  )

  it.each(['empty', 'error', 'poll-error'])(
    'allows another generation and reference change when an accepted read is %s',
    async (failure) => {
      const get = vi.spyOn(api, 'get')
      const fallback = get.getMockImplementation()
      if (!fallback) throw new Error('missing network fixture')
      let reads = 0
      let failPoll = false
      get.mockImplementation(async (...args) => {
        if (args[1]?.params?.task_id) {
          reads += 1
          if (failure === 'error' || failPoll) {
            throw new Error('Read unavailable')
          }
          const items =
            failure === 'empty'
              ? []
              : [{ id: 1, task_id: 'accepted', status: 'IN_PROGRESS' }]
          return {
            data: { success: true, data: { items, total: items.length } },
          }
        }
        return fallback(...args)
      })
      const network = post().mockImplementation(async (url) => {
        if (url === '/api/v1/files/upload/presign') throw unsupportedPresign
        if (url === '/api/v1/files/upload/stream') {
          return {
            data: {
              success: true,
              data: { file_url: 'https://files.example/style.png' },
            },
          }
        }
        return { data: { task_id: 'accepted' } }
      })
      const { container } = mount()
      await screen.findByRole('button', { name: 'Style reference' })
      fireEvent.change(input(container), { target: { files: [image] } })
      await waitFor(() =>
        expect(screen.getByRole('button', { name: /^Generate/ })).toBeEnabled()
      )
      fireEvent.click(screen.getByRole('button', { name: /^Generate/ }))
      await screen.findByText(
        failure === 'poll-error' ? 'Generating…' : 'Queued'
      )
      if (failure === 'poll-error') {
        failPoll = true
        vi.useFakeTimers()
        await act(async () => {
          fireEvent(document, new Event('visibilitychange'))
          await Promise.resolve()
        })
        vi.useRealTimers()
        expect(reads).toBeGreaterThan(1)
      }
      expect(screen.getByRole('button', { name: /^Generate/ })).toBeEnabled()
      expect(
        screen.getByRole('button', { name: 'Remove Style reference' })
      ).toBeEnabled()
      fireEvent.click(screen.getByRole('button', { name: /^Generate/ }))
      await waitFor(() =>
        expect(
          network.mock.calls.filter(([url]) => String(url).includes('/sheet'))
        ).toHaveLength(2)
      )
    }
  )

  it.each(['image-only', 'existing-description'])(
    'uses the reference-capable internal group for %s history multi-view regardless of the current builder group',
    async (parentKind) => {
      const parent = {
        id: 7,
        task_id: 'parent',
        action: 'influencer_sheet',
        status: 'SUCCESS',
        artifacts: [{ content_url: 'https://files.example/parent.png' }],
        properties: {
          input: JSON.stringify({
            tier: 'total',
            selection: {},
            brief: parentKind === 'image-only' ? '' : 'A character',
            image_url:
              parentKind === 'image-only'
                ? 'https://files.example/style.png'
                : undefined,
          }),
        },
      }
      const get = vi.spyOn(api, 'get')
      const fallback = get.getMockImplementation()
      if (!fallback) throw new Error('missing network fixture')
      get.mockImplementation(async (...args) => {
        if (String(args[0]).includes('/task/self')) {
          return {
            data: {
              success: true,
              data: {
                items: args[1]?.params?.task_id ? [] : [parent],
                total: 1,
              },
            },
          }
        }
        return fallback(...args)
      })
      const network = post().mockResolvedValue({ data: { task_id: 'board' } })
      localStorage.setItem('omnimux.influencer.source.v1', 'history')
      mount()
      fireEvent.click(
        (await screen.findAllByRole('button', { name: 'History' }))[0]
      )
      const card = (
        await screen.findByRole('link', { name: 'Download' })
      ).closest('.inf-card') as HTMLElement
      fireEvent.click(card)
      fireEvent.click(
        await screen.findByRole('button', { name: 'Generate multi-view' })
      )
      await waitFor(() => expect(network).toHaveBeenCalled())
      expect(network.mock.calls[0]?.[1]).toMatchObject({
        model: 'gpt-image-2.5-sunburst',
        group: 'gpt-image-2.5-sunburst-image',
        parent_task_id: 'parent',
        image_url: 'https://files.example/parent.png',
      })
      expect(network.mock.calls[0]?.[2]).toMatchObject({
        headers: { 'X-Omnimux-Group': 'gpt-image-2.5-sunburst-image' },
      })
      expect(screen.getByRole('button', { name: /^Generate/ })).toBeEnabled()
      expect(
        screen.getByRole('button', { name: 'Style reference' })
      ).toBeEnabled()
    }
  )

  it.each(['upload', 'generation'])(
    'keeps upload protection but allows history multi-view during reference %s',
    async (busyKind) => {
      const get = vi.spyOn(api, 'get')
      const fallback = get.getMockImplementation()
      if (!fallback) throw new Error('missing network fixture')
      get.mockImplementation(async (...args) => {
        if (String(args[0]).includes('/task/self')) {
          const items = args[1]?.params?.task_id
            ? []
            : [
                {
                  id: 7,
                  task_id: 'parent',
                  status: 'SUCCESS',
                  artifacts: [
                    { content_url: 'https://files.example/parent.png' },
                  ],
                },
              ]
          return {
            data: { success: true, data: { items, total: items.length } },
          }
        }
        return fallback(...args)
      })
      let finishUpload!: (value: unknown) => void
      const network = post().mockImplementation(async (url) => {
        if (url === '/api/v1/files/upload/presign') throw unsupportedPresign
        if (url === '/api/v1/files/upload/stream') {
          return (await new Promise<unknown>((resolve) => {
            finishUpload = resolve
          })) as never
        }
        return { data: { task_id: 'accepted' } } as never
      })
      const { container } = mount()
      fireEvent.click(
        (await screen.findAllByRole('button', { name: 'History' }))[0]
      )
      await screen.findByRole('link', { name: 'Download' })
      fireEvent.change(input(container), { target: { files: [image] } })
      await screen.findByText('Uploading image…')
      if (busyKind === 'generation') {
        await act(async () => {
          finishUpload({
            data: {
              success: true,
              data: { file_url: 'https://files.example/style.png' },
            },
          })
        })
        await waitFor(() =>
          expect(
            screen.getByRole('button', { name: /^Generate/ })
          ).toBeEnabled()
        )
        fireEvent.click(screen.getByRole('button', { name: /^Generate/ }))
        await screen.findByText('Queued')
      }
      fireEvent.click(
        screen
          .getByRole('link', { name: 'Download' })
          .closest('.inf-card') as HTMLElement
      )
      const dialog = await screen.findByRole('dialog', { name: 'Preview' })
      const generate = within(dialog).getByRole('button', {
        name: 'Generate multi-view',
      })
      if (busyKind === 'upload') {
        expect(generate).toBeDisabled()
        fireEvent.click(generate)
        expect(
          network.mock.calls.filter(([url]) =>
            String(url).includes('/multiview')
          )
        ).toHaveLength(0)
      } else {
        expect(generate).toBeEnabled()
      }
      expect(
        within(dialog).queryByText(
          'Generate multi-view: Wait for the current upload or generation to finish'
        )
      ).not.toBeInTheDocument()
      // Conversion remains a non-generating action in both cases.
      const turn = within(dialog).getByRole('button', {
        name: 'Turn to motion',
      })
      expect(turn).toBeEnabled()
      const sentBeforeConversion = network.mock.calls.length
      fireEvent.click(turn)
      await waitFor(() =>
        expect(
          screen.queryByRole('dialog', { name: 'Preview' })
        ).not.toBeInTheDocument()
      )
      expect(screen.getByRole('radio', { name: 'Motion video' })).toBeChecked()
      expect(network.mock.calls).toHaveLength(sentBeforeConversion)
    }
  )

  it('keeps an accepted multi-view locked when its generating board is opened and deletion is attempted', async () => {
    const get = vi.spyOn(api, 'get')
    const fallback = get.getMockImplementation()
    if (!fallback) throw new Error('missing network fixture')
    get.mockImplementation(async (...args) => {
      if (String(args[0]).includes('/task/self')) {
        const items = args[1]?.params?.task_id
          ? []
          : [
              {
                id: 7,
                task_id: 'parent',
                status: 'SUCCESS',
                artifacts: [
                  { content_url: 'https://files.example/parent.png' },
                ],
              },
            ]
        return { data: { success: true, data: { items, total: items.length } } }
      }
      return fallback(...args)
    })
    const network = post().mockResolvedValue({ data: { task_id: 'board' } })
    mount()
    fireEvent.click(
      (await screen.findAllByRole('button', { name: 'History' }))[0]
    )
    fireEvent.click(
      (await screen.findByRole('link', { name: 'Download' })).closest(
        '.inf-card'
      ) as HTMLElement
    )
    fireEvent.click(
      await screen.findByRole('button', { name: 'Generate multi-view' })
    )
    fireEvent.click(
      await screen.findByRole('button', {
        name: 'Queued multi-view storyboard…',
      })
    )
    const dialog = await screen.findByRole('dialog', { name: 'Multi-view' })
    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))
    expect(
      screen.getByRole('dialog', { name: 'Multi-view' })
    ).toBeInTheDocument()
    expect(
      within(dialog).getByRole('button', { name: 'Regenerate' })
    ).toBeDisabled()
    expect(network).toHaveBeenCalledTimes(1)
  })

  it('disables history multi-view with an explicit reason when the reference-capable group is unavailable', async () => {
    const get = vi.spyOn(api, 'get')
    const fallback = get.getMockImplementation()
    if (!fallback) throw new Error('missing network fixture')
    get.mockImplementation(async (...args) => {
      if (String(args[0]).includes('/groups')) {
        return {
          data: {
            success: true,
            data: { 'gpt-image-2.5-goeasy': { ratio: 0.5 } },
          },
        }
      }
      if (String(args[0]).includes('/task/self')) {
        return {
          data: {
            success: true,
            data: {
              items: [
                {
                  id: 7,
                  task_id: 'parent',
                  status: 'SUCCESS',
                  result_url: 'https://files.example/parent.png',
                },
              ],
              total: 1,
            },
          },
        }
      }
      return fallback(...args)
    })
    const network = post()
    mount()
    fireEvent.click(
      (await screen.findAllByRole('button', { name: 'History' }))[0]
    )
    fireEvent.click(
      (await screen.findByRole('link', { name: 'Download' })).closest(
        '.inf-card'
      ) as HTMLElement
    )
    const dialog = await screen.findByRole('dialog', { name: 'Preview' })
    expect(
      within(dialog).getByRole('button', { name: 'Generate multi-view' })
    ).toBeDisabled()
    expect(
      within(dialog).queryByText(
        'Generate multi-view: Style reference is not available for this model'
      )
    ).not.toBeInTheDocument()
    expect(network).not.toHaveBeenCalled()
  })
  it('places optional reference before exaggeration and gates guest picker before upload', async () => {
    const network = post()
    const user = userEvent.setup()
    mount(true)
    const upload = await screen.findByRole('button', {
      name: 'Style reference',
    })
    expect(
      upload.compareDocumentPosition(screen.getByText('Character type')) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
    await user.click(upload)
    expect(await screen.findByText('Log in to generate')).toBeInTheDocument()
    expect(network).not.toHaveBeenCalled()
  })
  it('blocks generation while upload is pending, then submits the single URL and unchanged tier', async () => {
    let finish!: (value: unknown) => void
    const network = post().mockImplementation(async (url) => {
      if (url === '/api/v1/files/upload/presign') throw unsupportedPresign
      if (url === '/api/v1/files/upload/stream') {
        return (await new Promise<unknown>((resolve) => {
          finish = resolve
        })) as never
      }
      return { data: { task_id: 'accepted-test-only' } } as never
    })
    const { container } = mount()
    await screen.findByRole('button', { name: 'Style reference' })
    fireEvent.change(input(container), { target: { files: [image] } })
    expect(await screen.findByText('Uploading image…')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Generate/ })).toBeDisabled()
    await act(async () => {
      finish({
        data: {
          success: true,
          data: { file_url: 'https://files.example/style.png' },
        },
      })
    })
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /^Generate/ })).toBeEnabled()
    )
    fireEvent.click(screen.getByRole('button', { name: /^Generate/ }))
    await waitFor(() =>
      expect(
        network.mock.calls.some(([url]) => String(url).includes('/sheet'))
      ).toBe(true)
    )
    expect(
      network.mock.calls.find(([url]) => String(url).includes('/sheet'))?.[1]
    ).toMatchObject({
      image_url: 'https://files.example/style.png',
      tier: 'total',
      group: 'gpt-image-2.5-sunburst-image',
      size: '1024x576',
    })
  })
  it('unlocks upload controls while an accepted generation is still running', async () => {
    const get = vi.spyOn(api, 'get')
    const fallback = get.getMockImplementation()
    if (!fallback) throw new Error('missing network fixture')
    get.mockImplementation(async (...args) => {
      if (String(args[0]).includes('/task/self') && args[1]?.params?.task_id) {
        return {
          data: {
            success: true,
            data: {
              items: [{ task_id: 'running-test', status: 'IN_PROGRESS' }],
              total: 1,
            },
          },
        }
      }
      return fallback(...args)
    })
    post().mockImplementation(async (url) => {
      if (url === '/api/v1/files/upload/presign') throw unsupportedPresign
      if (url === '/api/v1/files/upload/stream') {
        return {
          data: {
            success: true,
            data: { file_url: 'https://files.example/style.png' },
          },
        }
      }
      return { data: { task_id: 'running-test' } }
    })
    const { container } = mount()
    await screen.findByRole('button', { name: 'Style reference' })
    fireEvent.change(input(container), { target: { files: [image] } })
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /^Generate/ })).toBeEnabled()
    )
    fireEvent.click(screen.getByRole('button', { name: /^Generate/ }))
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Remove Style reference' })
      ).toBeEnabled()
    )
    expect(screen.getByRole('button', { name: /^Generate/ })).toBeEnabled()
  })

  it('explains unavailable reference routing and disables submission without dropping the URL', async () => {
    const get = vi.spyOn(api, 'get')
    const fallback = get.getMockImplementation()
    if (!fallback) throw new Error('missing network fixture')
    get.mockImplementation(async (...args) => {
      if (String(args[0]).includes('/groups')) {
        return {
          data: {
            success: true,
            data: { 'gpt-image-2.5-goeasy': { ratio: 0.5 } },
          },
        }
      }
      return fallback(...args)
    })
    post()
      .mockRejectedValueOnce(unsupportedPresign)
      .mockResolvedValue({
        data: {
          success: true,
          data: { file_url: 'https://files.example/style.png' },
        },
      })
    const { container } = mount()
    await screen.findByRole('button', { name: 'Style reference' })
    fireEvent.change(input(container), { target: { files: [image] } })
    await screen.findByRole('button', { name: 'Replace Style reference' })
    const unavailable = screen.getByRole('button', {
      name: 'Temporarily unavailable',
    })
    expect(unavailable).toBeDisabled()
    expect(unavailable).toHaveAttribute(
      'title',
      'Image generation is temporarily unavailable'
    )
  })

  it('clears only selected features through the real footer while keeping its uploaded reference', async () => {
    post()
      .mockRejectedValueOnce(unsupportedPresign)
      .mockResolvedValue({
        data: {
          success: true,
          data: { file_url: 'https://files.example/style.png' },
        },
      })
    const { container } = mount()
    await screen.findByRole('button', { name: 'Style reference' })
    fireEvent.change(input(container), { target: { files: [image] } })
    await screen.findByRole('button', { name: 'Replace Style reference' })
    // The sole fixture option is selected by the initial random combination.
    expect(screen.getByRole('button', { name: 'Curvy' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Reset features' }))
    expect(screen.getByRole('button', { name: 'Curvy' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
    expect(
      screen.queryByRole('button', { name: 'Reset features' })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Replace Style reference' })
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^Generate/ })).toBeEnabled()
  })

  it('keeps a successfully uploaded reference when switching workbench tabs', async () => {
    post()
      .mockRejectedValueOnce(unsupportedPresign)
      .mockResolvedValue({
        data: {
          success: true,
          data: { file_url: 'https://files.example/style.png' },
        },
      })
    const { container } = mount()
    await screen.findByRole('button', { name: 'Style reference' })
    fireEvent.change(input(container), { target: { files: [image] } })
    await screen.findByRole('button', { name: 'Replace Style reference' })
    fireEvent.click(screen.getByRole('radio', { name: 'Motion video' }))
    fireEvent.click(screen.getByRole('radio', { name: 'Character design' }))
    expect(
      screen.getByRole('button', { name: 'Replace Style reference' })
    ).toBeInTheDocument()
  })
})
