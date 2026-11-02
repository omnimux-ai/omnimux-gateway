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
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { InfluencerWorkbench } from '../index'
import { clearSession } from '../lib/workbench-session'

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

function sourceUploads() {
  let source = 0
  return post().mockImplementation(async (url) => {
    if (url === '/api/v1/files/upload/presign') throw unsupportedPresign
    if (url === '/api/v1/files/upload/stream') {
      source += 1
      return {
        data: {
          success: true,
          data: { file_url: `https://files.example/source-${source}.png` },
        },
      }
    }
    return { data: { task_id: 'local-test-only' } }
  })
}

function expectNoEditControls() {
  expect(screen.queryByText('Describe your image')).not.toBeInTheDocument()
  expect(
    screen.queryByRole('textbox', { name: 'Describe your image' })
  ).not.toBeInTheDocument()
  for (const name of [
    'Text to image',
    'Reference image',
    'Edit whole image',
    'Edit selected area',
    'Save selection',
  ]) {
    expect(screen.queryByRole('button', { name })).not.toBeInTheDocument()
  }
  expect(
    screen.queryByRole('textbox', { name: 'Describe your changes' })
  ).not.toBeInTheDocument()
  expect(
    screen.queryByRole('img', { name: 'Edit selection canvas' })
  ).not.toBeInTheDocument()
}

async function generate(network: ReturnType<typeof post>) {
  const button = screen.getByRole('button', { name: /^Generate/ })
  await waitFor(() => expect(button).toBeEnabled())
  fireEvent.click(button)
  await waitFor(() =>
    expect(
      network.mock.calls.filter(([url]) => url === '/pg/influencer/sheet')
    ).toHaveLength(1)
  )
  return network.mock.calls.find(([url]) => url === '/pg/influencer/sheet')?.[1]
}

describe('automatic image-reference input boundary', () => {
  it('shows character choices and optional reference without a free image description field', async () => {
    const network = post()
    mount()
    await screen.findByRole('button', { name: 'Style reference' })
    expect(screen.queryByText('Describe your image')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('textbox', { name: 'Describe your image' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Curvy' })).toBeInTheDocument()
    expectNoEditControls()
    expect(network).not.toHaveBeenCalled()
  })

  it('uses the uploaded image automatically without adding image-mode or edit controls', async () => {
    const network = sourceUploads()
    const { container } = mount()
    await screen.findByRole('button', { name: 'Style reference' })
    expectNoEditControls()
    fireEvent.change(input(container), { target: { files: [image] } })
    await screen.findByRole('button', { name: 'Replace Style reference' })
    fireEvent.click(screen.getByRole('button', { name: 'Reset features' }))
    expectNoEditControls()
    const payload = await generate(network)
    expect(payload).toMatchObject({
      image_mode: 'reference',
      image_url: 'https://files.example/source-1.png',
      selection: {},
      brief: '',
    })
    expect(payload).not.toHaveProperty('mask_url')
    expect(payload).not.toHaveProperty('edit_instruction')
  })

  it('returns to text generation from the selected character features after removing the image', async () => {
    const network = sourceUploads()
    const { container } = mount()
    await screen.findByRole('button', { name: 'Style reference' })
    fireEvent.change(input(container), { target: { files: [image] } })
    await screen.findByRole('button', { name: 'Replace Style reference' })
    fireEvent.click(
      screen.getByRole('button', { name: 'Remove Style reference' })
    )
    expectNoEditControls()
    const payload = await generate(network)
    expect(payload).toMatchObject({
      image_mode: 'text',
      brief: '',
      selection: { body: ['curvy'] },
    })
    expect(payload).toMatchObject({ image_url: undefined })
    expect(payload).not.toHaveProperty('mask_url')
    expect(payload).not.toHaveProperty('edit_instruction')
  })

  it.each([true, false])(
    'ignores old history text and derives creation validity from its effective reference %s',
    async (hasReference) => {
      const get = vi.mocked(api.get)
      const fallback = get.getMockImplementation()
      if (!fallback) throw new Error('Missing network fixture')
      get.mockImplementation(async (...args) => {
        if (String(args[0]).includes('/task/self')) {
          const items =
            args[1]?.params?.action === 'influencer_sheet'
              ? [
                  {
                    id: 7,
                    task_id: 'old-text-with-source',
                    action: 'influencer_sheet',
                    status: 'SUCCESS',
                    result_url: 'https://files.example/finished.png',
                    properties: {
                      input: JSON.stringify({
                        tier: 'total',
                        selection: {},
                        brief: 'A hidden historical red coat',
                        image_mode: 'text',
                        image_url: hasReference
                          ? 'https://files.example/retained.png'
                          : undefined,
                      }),
                    },
                  },
                ]
              : []
          return {
            data: { success: true, data: { items, total: items.length } },
          }
        }
        return fallback(...args)
      })
      const network = post().mockResolvedValue({
        data: { task_id: 'accepted' },
      })
      mount()
      fireEvent.click(
        (await screen.findAllByRole('button', { name: 'History' }))[0]
      )
      fireEvent.click(
        (await screen.findByRole('link', { name: 'Download' })).closest(
          '.inf-card'
        ) as HTMLElement
      )
      const preview = await screen.findByRole('dialog', { name: 'Preview' })
      expect(
        within(preview).queryByRole('button', { name: 'Edit this image' })
      ).not.toBeInTheDocument()
      fireEvent.click(
        within(preview).getByRole('button', { name: 'Regenerate' })
      )
      await screen.findByRole('button', {
        name: hasReference ? 'Replace Style reference' : 'Style reference',
      })
      expect(network).not.toHaveBeenCalled()
      expectNoEditControls()
      if (!hasReference) {
        expect(screen.getByRole('button', { name: 'Curvy' })).toHaveAttribute(
          'aria-pressed',
          'false'
        )
        expect(screen.getByRole('button', { name: /^Generate/ })).toBeDisabled()
        return
      }
      expect(await generate(network)).toMatchObject({
        image_mode: 'reference',
        image_url: 'https://files.example/retained.png',
        brief: '',
        selection: {},
      })
    }
  )

  it.each([
    ['edit', 'https://files.example/legacy-source.png', 'reference'],
    ['masked_edit', 'https://files.example/legacy-source.png', 'reference'],
    ['edit', '', 'text'],
    ['masked_edit', '', 'text'],
  ])(
    'restores a legacy %s draft with image %s as %s without promoting its edit instruction',
    async (mode, imageUrl, expectedMode) => {
      // Write the old representation directly: the current writer must not hide a
      // stale-draft restore bug by normalizing this fixture before it is read.
      sessionStorage.setItem(
        'influencer:guest-draft:v1',
        JSON.stringify({
          tier: 'total',
          selection: imageUrl ? {} : { body: ['curvy'] },
          brief: 'A character wearing blue',
          seed: '',
          imageUrl,
          imageMimeType: 'image/png',
          imageMode: mode,
          maskUrl: 'https://files.example/owned-mask.png',
          editInstruction: 'Replace the cup',
          model: 'gpt-image-2.5-sunburst',
          group: 'gpt-image-2.5-sunburst-image',
          vendorId: null,
        })
      )
      const network = post().mockResolvedValue({
        data: { task_id: 'accepted' },
      })
      mount()
      await screen.findByRole('button', {
        name: imageUrl ? 'Replace Style reference' : 'Style reference',
      })
      expectNoEditControls()
      expect(
        screen.queryByDisplayValue('Replace the cup')
      ).not.toBeInTheDocument()
      expect(network).not.toHaveBeenCalled()
      const payload = await generate(network)
      expect(payload).toMatchObject({
        image_mode: expectedMode,
        brief: '',
        selection: imageUrl ? {} : { body: ['curvy'] },
      })
      if (imageUrl) expect(payload).toMatchObject({ image_url: imageUrl })
      else expect(payload).toMatchObject({ image_url: undefined })
      expect(payload).not.toHaveProperty('mask_url')
      expect(payload).not.toHaveProperty('edit_instruction')
    }
  )

  it('does not let an old brief or edit instruction unlock an empty character draft', async () => {
    sessionStorage.setItem(
      'influencer:guest-draft:v1',
      JSON.stringify({
        tier: 'total',
        selection: {},
        brief: 'Old text cannot provide hidden character content',
        seed: '',
        imageUrl: '',
        imageMode: 'edit',
        editInstruction: 'Replace the cup',
      })
    )
    const network = post()
    mount()
    await screen.findByRole('button', { name: 'Style reference' })
    expectNoEditControls()
    expect(screen.getByRole('button', { name: /^Generate/ })).toBeDisabled()
    expect(network).not.toHaveBeenCalled()
  })
})
