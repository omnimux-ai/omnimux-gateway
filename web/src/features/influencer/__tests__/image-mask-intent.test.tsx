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
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { InfluencerWorkbench } from '../index'

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

type UploadReply = { data: { success: boolean; data: { file_url: string } } }
const reply = (url: string): UploadReply => ({
  data: { success: true, data: { file_url: url } },
})
const sourceURL = 'https://files.example/owned-source.png'
const historicalMask = 'https://files.example/historical-mask.png'

function mockHistory(mode = 'masked_edit', brief = 'A new character in blue') {
  const original = {
    id: 7,
    task_id: 'historical-edit',
    action: 'influencer_sheet',
    status: 'SUCCESS',
    artifacts: [
      {
        key: 'image',
        mime_type: 'image/png',
        content_url: 'https://files.example/completed.png',
      },
    ],
    properties: {
      input: JSON.stringify({
        tier: 'total',
        selection: { body: ['curvy'] },
        brief,
        image_mode: mode,
        image_url: sourceURL,
        mask_url: historicalMask,
        edit_instruction: 'Replace the cup',
      }),
    },
  }
  const get = vi.mocked(api.get)
  const fallback = get.getMockImplementation()
  if (!fallback) throw new Error('Missing network fixture')
  get.mockImplementation(async (...args) => {
    if (String(args[0]).includes('/task/self')) {
      const items =
        args[1]?.params?.action === 'influencer_sheet' ? [original] : []
      return { data: { success: true, data: { items, total: items.length } } }
    }
    return fallback(...args)
  })
}

function expectNoEditing() {
  expect(screen.queryByText('Describe your image')).not.toBeInTheDocument()
  expect(
    screen.queryByRole('textbox', { name: 'Describe your image' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Edit whole image' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Edit selected area' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Save selection' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('img', { name: 'Edit selection canvas' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('textbox', { name: 'Describe your changes' })
  ).not.toBeInTheDocument()
}

async function submitReference(
  network: ReturnType<typeof post>,
  source = sourceURL
) {
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
    image_url: source,
    brief: '',
  })
  expect(payload).not.toHaveProperty('mask_url')
  expect(payload).not.toHaveProperty('edit_instruction')
}

async function openHistory() {
  fireEvent.click(
    (await screen.findAllByRole('button', { name: 'History' }))[0]
  )
  fireEvent.click(
    (await screen.findByRole('link', { name: 'Download' })).closest(
      '.inf-card'
    ) as HTMLElement
  )
  return screen.findByRole('dialog', { name: 'Preview' })
}

describe('retired image-edit intent boundaries', () => {
  it.each(['edit', 'masked_edit'])(
    'restores %s history as visible character choices and style reference without its old text',
    async (mode) => {
      mockHistory(mode)
      const network = post().mockResolvedValue({
        data: { task_id: 'accepted' },
      })
      mount()
      const dialog = await openHistory()
      expect(
        within(dialog).queryByRole('button', { name: 'Edit this image' })
      ).not.toBeInTheDocument()
      // Existing edit results stay ineligible for identity-derived multi-view.
      expect(
        within(dialog).queryByRole('button', { name: 'Generate multi-view' })
      ).not.toBeInTheDocument()
      fireEvent.click(
        within(dialog).getByRole('button', { name: 'Regenerate' })
      )
      await screen.findByRole('button', { name: 'Replace Style reference' })
      expect(screen.getByRole('button', { name: 'Curvy' })).toHaveAttribute(
        'aria-pressed',
        'true'
      )
      expect(
        screen.getByRole('button', { name: 'Replace Style reference' })
      ).toBeInTheDocument()
      expect(
        screen.queryByDisplayValue('Replace the cup')
      ).not.toBeInTheDocument()
      expectNoEditing()
      expect(network).not.toHaveBeenCalled()
      await submitReference(network)
    }
  )

  it('does not promote a historical edit instruction to the ordinary description', async () => {
    mockHistory('masked_edit', '')
    const network = post().mockResolvedValue({ data: { task_id: 'accepted' } })
    mount()
    const dialog = await openHistory()
    fireEvent.click(within(dialog).getByRole('button', { name: 'Regenerate' }))
    await screen.findByRole('button', { name: 'Replace Style reference' })
    expect(
      screen.queryByDisplayValue('Replace the cup')
    ).not.toBeInTheDocument()
    expectNoEditing()
    expect(network).not.toHaveBeenCalled()
    await submitReference(network)
  })

  it('keeps the original style reference and selected features when a replacement upload fails', async () => {
    let uploads = 0
    const network = post().mockImplementation(async (url) => {
      if (url === '/api/v1/files/upload/presign') throw unsupportedPresign
      if (url === '/api/v1/files/upload/stream') {
        uploads += 1
        if (uploads === 1) return reply(sourceURL)
        throw new Error('Replacement storage failure')
      }
      return { data: { task_id: 'accepted' } }
    })
    const errorToast = vi.spyOn(toast, 'error')
    const { container } = mount()
    await screen.findByRole('button', { name: 'Style reference' })
    fireEvent.change(input(container), { target: { files: [image] } })
    await screen.findByRole('button', { name: 'Replace Style reference' })
    fireEvent.change(input(container), {
      target: {
        files: [new File(['replacement'], 'new.png', { type: 'image/png' })],
      },
    })
    await waitFor(() =>
      expect(errorToast).toHaveBeenCalledWith('Replacement storage failure')
    )
    expect(
      screen.getByRole('button', { name: 'Replace Style reference' })
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Curvy' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    expectNoEditing()
    await submitReference(network)
  })

  it.each(['success', 'failure'])(
    'ignores a previous account upload returning %s before the new account picks the same source',
    async (outcome) => {
      let resolveUpload!: (value: UploadReply) => void
      let rejectUpload!: (reason: Error) => void
      let uploads = 0
      const network = post().mockImplementation(async (url) => {
        if (url === '/api/v1/files/upload/presign') throw unsupportedPresign
        if (url === '/api/v1/files/upload/stream') {
          uploads += 1
          if (uploads === 1) {
            return new Promise<UploadReply>((resolve, reject) => {
              resolveUpload = resolve
              rejectUpload = reject
            })
          }
          return reply(sourceURL)
        }
        return { data: { task_id: 'accepted' } }
      })
      const { container } = mount()
      await screen.findByRole('button', { name: 'Style reference' })
      fireEvent.change(input(container), { target: { files: [image] } })
      await screen.findByText('Uploading image…')
      expect(screen.getByRole('button', { name: /^Generate/ })).toBeDisabled()
      act(() =>
        useAuthStore.setState((state) => ({
          auth: { ...state.auth, user: { id: 2, username: 'other', role: 1 } },
        }))
      )
      await screen.findByRole('button', { name: 'Style reference' })
      await act(async () => {
        if (outcome === 'success') resolveUpload(reply(sourceURL))
        else rejectUpload(new Error('Previous account upload failed'))
      })
      expect(
        screen.queryByRole('button', { name: 'Replace Style reference' })
      ).not.toBeInTheDocument()
      expect(
        screen.queryByText('Previous account upload failed')
      ).not.toBeInTheDocument()
      expectNoEditing()
      fireEvent.change(input(container), { target: { files: [image] } })
      await screen.findByRole('button', { name: 'Replace Style reference' })
      const feature = screen.getByRole('button', { name: 'Curvy' })
      expect(feature).toHaveAttribute('aria-pressed', 'false')
      fireEvent.click(feature)
      expect(feature).toHaveAttribute('aria-pressed', 'true')
      await submitReference(network)
      expect(
        network.mock.calls.find(([url]) => url === '/pg/influencer/sheet')?.[1]
      ).toMatchObject({ selection: { body: ['curvy'] } })
    }
  )

  it('keeps an upload completing in the video tab as a style reference when returning to character creation', async () => {
    let finishUpload!: (value: UploadReply) => void
    const network = post().mockImplementation(async (url) => {
      if (url === '/api/v1/files/upload/presign') throw unsupportedPresign
      if (url === '/api/v1/files/upload/stream') {
        return new Promise<UploadReply>((resolve) => {
          finishUpload = resolve
        })
      }
      return { data: { task_id: 'accepted' } }
    })
    const { container } = mount()
    await screen.findByRole('button', { name: 'Style reference' })
    fireEvent.change(input(container), { target: { files: [image] } })
    await screen.findByText('Uploading image…')
    fireEvent.click(screen.getByRole('radio', { name: 'Motion video' }))
    await act(async () => finishUpload(reply(sourceURL)))
    fireEvent.click(screen.getByRole('radio', { name: 'Character design' }))
    await screen.findByRole('button', { name: 'Replace Style reference' })
    expectNoEditing()
    await submitReference(network)
  })
})
