import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { InfluencerWorkbench } from '../index'
import { readBrowserVideoMetadata } from '../lib/browser-video-metadata'
import { resetUploadCache } from '../lib/upload-library'
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
  size_bytes: 4,
}

const VIDEO_URL = 'https://files.example/reference.mp4'
const IMAGE_URL = 'https://files.example/character.png'
const ENTRY = 'Add a reference video to extract motion'
const IMAGE_ENTRY = 'Add character, product or clothing images'
let clients: QueryClient[]

beforeEach(() => {
  vi.mocked(readBrowserVideoMetadata)
    .mockReset()
    .mockImplementation(async (source) => ({
      ...RAW_VIDEO_METADATA,
      size_bytes:
        typeof source === 'string'
          ? RAW_VIDEO_METADATA.size_bytes
          : source.size,
    }))
  mockReferenceLayout()
  localStorage.clear()
  sessionStorage.clear()
  resetUploadCache()
  useAuthStore.getState().auth.reset()
  useAuthStore.setState((state) => ({
    auth: {
      ...state.auth,
      user: { id: 7, username: 'test', role: 1 },
      accessToken: 'test-only',
    },
  }))
  clients = []
  vi.spyOn(toast, 'error').mockReturnValue('test-toast')
  const create = document.createElement.bind(document)
  vi.spyOn(document, 'createElement').mockImplementation((tag, options) => {
    const element = create(tag, options)
    if (tag === 'video') {
      const video = element as HTMLVideoElement
      video.load = vi.fn()
    }
    return element
  })
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  vi.stubGlobal(
    'URL',
    class extends URL {
      static createObjectURL = vi.fn(() => 'blob:upload-state')
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
  clients.forEach((client) => client.clear())
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
  localStorage.clear()
  sessionStorage.clear()
  resetUploadCache()
})

function mountWorkbench(): void {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  clients.push(client)
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
  render(<RouterProvider router={router} />)
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: Error) => void
  const promise = new Promise<T>((done, fail) => {
    resolve = done
    reject = fail
  })
  return { promise, resolve, reject }
}

const UPLOAD_RESPONSE = {
  data: { success: true, data: { file_url: VIDEO_URL } },
}

function pendingUpload(checkPending = false) {
  const upload = deferred<typeof UPLOAD_RESPONSE>()
  const images: Array<ReturnType<typeof deferred<typeof UPLOAD_RESPONSE>>> = []
  const inspection = deferred<typeof RAW_VIDEO_METADATA>()
  if (checkPending) {
    vi.mocked(readBrowserVideoMetadata).mockReturnValueOnce(inspection.promise)
  }
  let uploadResponse = upload.promise
  const post = vi.spyOn(api, 'post').mockImplementation((url, body) => {
    if (url === '/api/v1/files/upload/presign') {
      return Promise.reject({ response: { status: 501 } })
    }

    if (
      url === '/api/v1/files/upload/stream' &&
      (body as FormData).get('file') instanceof File &&
      ((body as FormData).get('file') as File).type.startsWith('image/')
    ) {
      const next = deferred<typeof UPLOAD_RESPONSE>()
      images.push(next)
      return next.promise
    }
    return uploadResponse
  })
  return {
    post,
    resolve: () => upload.resolve(UPLOAD_RESPONSE),
    reject: upload.reject,
    rejectInspection: inspection.reject,
    resolveInspection: () => inspection.resolve(RAW_VIDEO_METADATA),
    resolveImage: () => {
      for (const [index, pending] of images.splice(0).entries()) {
        pending.resolve({
          data: {
            success: true,
            data: {
              file_url: index === 0 ? IMAGE_URL : `${IMAGE_URL}?${index}`,
            },
          },
        })
      }
    },
    retry: () => {
      uploadResponse = Promise.resolve(UPLOAD_RESPONSE)
    },
  }
}

async function startUpload(
  user: ReturnType<typeof userEvent.setup>
): Promise<void> {
  mountWorkbench()
  await user.click(await screen.findByRole('radio', { name: 'Motion video' }))
  await user.click(screen.getByRole('button', { name: ENTRY }))
  const panel = screen.getByRole('complementary', {
    name: 'Select reference videos',
  })
  await user.upload(
    panel.querySelector('input[type=file]') as HTMLInputElement,
    new File(['clip'], 'reference.mp4', { type: 'video/mp4' })
  )
}

async function closePanel(
  user: ReturnType<typeof userEvent.setup>,
  outside = false
): Promise<void> {
  if (outside) fireEvent.pointerDown(document.body)
  else await user.click(screen.getByRole('button', { name: 'Close panel' }))
  expect(
    screen.queryByRole('complementary', { name: 'Select reference videos' })
  ).not.toBeInTheDocument()
}

describe('upload task survives reference panel visibility changes', () => {
  it.each([false, true])(
    'uploads and selects an image independently while browser video inspection is pending=%s',
    async (checkPending) => {
      const upload = pendingUpload(checkPending)
      const user = userEvent.setup()
      await startUpload(user)
      if (checkPending) await act(async () => upload.resolve())

      await user.click(screen.getByRole('button', { name: IMAGE_ENTRY }))
      const images = screen.getByRole('complementary', {
        name: 'Select reference images',
      })
      expect(
        within(images).getByRole('button', { name: 'Upload local photo' })
      ).toBeEnabled()
      const input = images.querySelector('input[type=file]') as HTMLInputElement
      await user.upload(
        input,
        new File(['image'], 'character.png', { type: 'image/png' })
      )
      // The upload entry is not occupied by the running transfer, so a second
      // pick starts another upload in the same queue.
      expect(
        within(images).getByRole('button', { name: 'Upload local photo' })
      ).toBeEnabled()
      fireEvent.change(input, {
        target: {
          files: [
            new File(['duplicate'], 'duplicate.png', { type: 'image/png' }),
          ],
        },
      })
      await waitFor(() =>
        expect(
          upload.post.mock.calls.filter(
            ([url]) => url === '/api/v1/files/upload/stream'
          )
        ).toHaveLength(checkPending ? 2 : 3)
      )

      await act(async () => upload.resolveImage())
      expect(
        await within(images).findByRole('button', { name: 'character.png' })
      ).toHaveAttribute('aria-pressed', 'true')
      await user.click(screen.getByRole('button', { name: ENTRY }))
      const videos = screen.getByRole('complementary', {
        name: 'Select reference videos',
      })
      expect(within(videos).getByRole('status')).toHaveTextContent(
        checkPending ? 'Reading reference video…' : 'Uploading reference video…'
      )
      expect(
        within(videos).getByRole('button', { name: 'Upload local video' })
      ).toBeDisabled()
      await act(async () => {
        if (checkPending) upload.resolveInspection()
        else upload.resolve()
      })
      expect(
        await within(videos).findByRole('button', { name: 'reference.mp4' })
      ).toHaveAttribute('aria-pressed', 'true')
      await user.click(
        screen.getByRole('button', { name: 'Add reference image' })
      )
      expect(
        within(
          screen.getByRole('complementary', { name: 'Select reference images' })
        ).getByRole('button', { name: 'character.png' })
      ).toHaveAttribute('aria-pressed', 'true')
    }
  )

  it('starts a video during an image upload and retains both selections when the video finishes first', async () => {
    const upload = pendingUpload()
    const user = userEvent.setup()
    mountWorkbench()
    await user.click(await screen.findByRole('radio', { name: 'Motion video' }))
    await user.click(screen.getByRole('button', { name: IMAGE_ENTRY }))
    const images = screen.getByRole('complementary', {
      name: 'Select reference images',
    })
    await user.upload(
      images.querySelector('input[type=file]') as HTMLInputElement,
      new File(['image'], 'character.png', { type: 'image/png' })
    )
    await user.click(screen.getByRole('button', { name: ENTRY }))
    const videos = screen.getByRole('complementary', {
      name: 'Select reference videos',
    })
    expect(
      within(videos).getByRole('button', { name: 'Upload local video' })
    ).toBeEnabled()
    await user.upload(
      videos.querySelector('input[type=file]') as HTMLInputElement,
      new File(['clip'], 'reference.mp4', { type: 'video/mp4' })
    )
    expect(
      upload.post.mock.calls.filter(
        ([url]) => url === '/api/v1/files/upload/stream'
      )
    ).toHaveLength(2)
    await act(async () => upload.resolve())
    expect(
      await within(videos).findByRole('button', { name: 'reference.mp4' })
    ).toHaveAttribute('aria-pressed', 'true')
    // Adding more references stays available while the image upload runs.
    expect(screen.getByRole('button', { name: IMAGE_ENTRY })).toBeEnabled()
    await act(async () => upload.resolveImage())
    await user.click(
      screen.getByRole('button', { name: 'Add reference image' })
    )
    expect(
      await screen.findByRole('button', { name: 'character.png' })
    ).toHaveAttribute('aria-pressed', 'true')
    await user.click(screen.getByRole('button', { name: `Replace ${ENTRY}` }))
    expect(
      screen.getByRole('button', { name: 'reference.mp4' })
    ).toHaveAttribute('aria-pressed', 'true')
  })

  it('keeps a directly dropped image locked until its own upload finishes after the video', async () => {
    const upload = pendingUpload()
    const user = userEvent.setup()
    mountWorkbench()
    await user.click(await screen.findByRole('radio', { name: 'Motion video' }))
    fireEvent.drop(screen.getByRole('button', { name: ENTRY }), {
      dataTransfer: {
        files: [new File(['clip'], 'reference.mp4', { type: 'video/mp4' })],
      },
    })
    fireEvent.drop(screen.getByRole('button', { name: IMAGE_ENTRY }), {
      dataTransfer: {
        files: [new File(['image'], 'character.png', { type: 'image/png' })],
      },
    })
    await waitFor(() =>
      expect(
        upload.post.mock.calls.filter(
          ([url]) => url === '/api/v1/files/upload/stream'
        )
      ).toHaveLength(2)
    )
    await act(async () => upload.resolve())
    expect(screen.getByRole('button', { name: IMAGE_ENTRY })).toBeEnabled()
    await act(async () => upload.resolveImage())
    expect(
      screen.getByRole('button', { name: `Replace ${ENTRY}` })
    ).toBeEnabled()
    await user.click(
      screen.getByRole('button', { name: 'Add reference image' })
    )
    expect(
      screen.getByRole('button', { name: 'character.png' })
    ).toHaveAttribute('aria-pressed', 'true')
  })

  it('uploads a style reference after switching to character design while its video upload continues', async () => {
    const get = vi.mocked(api.get).getMockImplementation()
    if (!get) throw new Error('Expected the shared request mock')
    vi.mocked(api.get).mockImplementation(async (url, config) => {
      if (String(url).includes('/pricing')) {
        return {
          data: {
            success: true,
            data: [
              {
                model_name: 'gpt-image-2.5-sunburst',
                model_price: 0.04,
                enable_groups: ['gpt-image-2.5-sunburst-image'],
                supported_endpoint_types: ['image-generation'],
                vendor_id: 1,
              },
            ],
            group_ratio: { 'gpt-image-2.5-sunburst-image': 1 },
          },
        }
      }
      if (String(url).includes('/groups')) {
        return {
          data: {
            success: true,
            data: { 'gpt-image-2.5-sunburst-image': { ratio: 1 } },
          },
        }
      }
      return get(url, config)
    })
    const upload = pendingUpload()
    const user = userEvent.setup()
    await startUpload(user)
    await user.click(screen.getByRole('radio', { name: 'Character design' }))
    const style = screen.getByRole('button', { name: 'Style reference' })
    expect(style).toBeEnabled()
    fireEvent.drop(style, {
      dataTransfer: {
        files: [new File(['image'], 'character.png', { type: 'image/png' })],
      },
    })
    await waitFor(() =>
      expect(
        upload.post.mock.calls.filter(
          ([url]) => url === '/api/v1/files/upload/stream'
        )
      ).toHaveLength(2)
    )
    await act(async () => upload.resolveImage())
    expect(
      await screen.findByRole('button', { name: 'Replace Style reference' })
    ).toBeEnabled()
    expect(screen.getByRole('button', { name: /^Generate/ })).toBeDisabled()
    await user.click(screen.getByRole('radio', { name: 'Motion video' }))
    await user.click(screen.getByRole('button', { name: ENTRY }))
    const videos = screen.getByRole('complementary', {
      name: 'Select reference videos',
    })
    expect(within(videos).getByRole('status')).toHaveTextContent(
      'Uploading reference video…'
    )
    await act(async () => upload.resolve())
    expect(
      await within(videos).findByRole('button', { name: 'reference.mp4' })
    ).toHaveAttribute('aria-pressed', 'true')
    await user.click(screen.getByRole('radio', { name: 'Character design' }))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: /^Generate/ })).toBeEnabled()
    )
  })

  it.each([false, true])(
    'closing with outside=%s then reopening retains upload progress and blocks duplicate files',
    async (outside) => {
      const upload = pendingUpload()
      const user = userEvent.setup()
      await startUpload(user)
      await closePanel(user, outside)
      await user.click(screen.getByRole('button', { name: ENTRY }))
      const panel = screen.getByRole('complementary', {
        name: 'Select reference videos',
      })
      expect(within(panel).getByRole('status')).toHaveTextContent(
        'Uploading reference video…'
      )
      expect(
        within(panel).getByRole('button', { name: 'Upload local video' })
      ).toBeDisabled()
      expect(panel).toHaveAttribute('aria-busy', 'true')
      fireEvent.drop(panel, {
        dataTransfer: {
          files: [
            new File(['duplicate'], 'duplicate.mp4', { type: 'video/mp4' }),
          ],
        },
      })
      expect(
        upload.post.mock.calls.filter(
          ([url]) => url === '/api/v1/files/upload/stream'
        )
      ).toHaveLength(1)
      await act(async () => upload.resolve())
      expect(
        await within(panel).findByRole('button', { name: 'reference.mp4' })
      ).toHaveAttribute('aria-pressed', 'true')
    }
  )

  it('closing during browser inspection preserves the preparation lock and notifies once when checking fails', async () => {
    const upload = pendingUpload(true)
    const user = userEvent.setup()
    await startUpload(user)
    await act(async () => upload.resolve())
    expect(readBrowserVideoMetadata).toHaveBeenCalledOnce()
    expect(upload.post).not.toHaveBeenCalled()
    await closePanel(user)
    await user.click(screen.getByRole('button', { name: ENTRY }))
    const panel = screen.getByRole('complementary', {
      name: 'Select reference videos',
    })
    expect(panel).toHaveAttribute('aria-busy', 'true')
    expect(within(panel).getByRole('status')).toHaveTextContent(
      'Reading reference video…'
    )
    expect(
      within(panel).getByRole('button', { name: 'Upload local video' })
    ).toBeDisabled()
    await act(async () =>
      upload.rejectInspection(
        new Error('Failed to parse video. Please check the file.')
      )
    )
    expect(toast.error).toHaveBeenCalledExactlyOnceWith(
      'Failed to parse video. Please check the file.'
    )
    expect(within(panel).queryByRole('alert')).not.toBeInTheDocument()
    expect(
      within(panel).queryByRole('button', { name: 'reference.mp4' })
    ).not.toBeInTheDocument()
  })

  it('a successful upload while closed is selected and listed once when reopened', async () => {
    const upload = pendingUpload()
    const user = userEvent.setup()
    await startUpload(user)
    await closePanel(user)
    await act(async () => upload.resolve())
    await waitFor(() =>
      expect(
        screen.getByRole('button', {
          name: 'Replace Add a reference video to extract motion',
        })
      ).toBeInTheDocument()
    )
    expect(screen.queryByText('reference.mp4')).toBeNull()
    await user.click(screen.getByRole('button', { name: `Replace ${ENTRY}` }))
    expect(
      screen.getByRole('button', { name: 'reference.mp4' })
    ).toHaveAttribute('aria-pressed', 'true')
    expect(
      upload.post.mock.calls.filter(
        ([url]) => url === '/api/v1/files/upload/stream'
      )
    ).toHaveLength(1)
  })

  it('failure while closed notifies once and reopening permits a successful retry', async () => {
    const upload = pendingUpload()
    const user = userEvent.setup()
    await startUpload(user)
    await closePanel(user)
    await act(async () =>
      upload.reject(new Error('Upload failed. Please try again.'))
    )
    await user.click(screen.getByRole('button', { name: ENTRY }))
    const panel = screen.getByRole('complementary', {
      name: 'Select reference videos',
    })
    expect(toast.error).toHaveBeenCalledExactlyOnceWith(
      'Upload failed. Please try again.'
    )
    expect(within(panel).queryByRole('alert')).not.toBeInTheDocument()
    expect(
      within(panel).queryByRole('button', { name: 'reference.mp4' })
    ).not.toBeInTheDocument()
    upload.retry()
    await user.upload(
      panel.querySelector('input[type=file]') as HTMLInputElement,
      new File(['clip'], 'reference.mp4', { type: 'video/mp4' })
    )
    expect(
      await within(panel).findByRole('button', { name: 'reference.mp4' })
    ).toHaveAttribute('aria-pressed', 'true')
    expect(within(panel).queryByRole('alert')).not.toBeInTheDocument()
  })

  it('opening the image picker hides the video panel without cancelling its pending upload', async () => {
    const upload = pendingUpload()
    const user = userEvent.setup()
    await startUpload(user)
    await user.click(
      screen.getByRole('button', {
        name: 'Add character, product or clothing images',
      })
    )
    expect(
      screen.getByRole('complementary', { name: 'Select reference images' })
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: ENTRY }))
    expect(
      within(
        screen.getByRole('complementary', { name: 'Select reference videos' })
      ).getByRole('status')
    ).toHaveTextContent('Uploading reference video…')
    await act(async () => upload.resolve())
  })

  it('closing and reopening after a reported failure neither duplicates its toast nor leaves an inline warning', async () => {
    const upload = pendingUpload()
    const user = userEvent.setup()
    await startUpload(user)
    await act(async () =>
      upload.reject(new Error('Upload failed. Please try again.'))
    )
    expect(toast.error).toHaveBeenCalledExactlyOnceWith(
      'Upload failed. Please try again.'
    )
    await closePanel(user)
    await user.click(screen.getByRole('button', { name: ENTRY }))
    expect(toast.error).toHaveBeenCalledOnce()
    expect(
      within(
        screen.getByRole('complementary', { name: 'Select reference videos' })
      ).queryByRole('alert')
    ).not.toBeInTheDocument()
  })

  it('switching workspace views retains the same pending video task', async () => {
    const upload = pendingUpload()
    const user = userEvent.setup()
    await startUpload(user)
    await user.click(screen.getByRole('radio', { name: 'Character design' }))
    await user.click(screen.getByRole('radio', { name: 'Motion video' }))
    await user.click(screen.getByRole('button', { name: ENTRY }))
    expect(
      within(
        screen.getByRole('complementary', { name: 'Select reference videos' })
      ).getByRole('status')
    ).toHaveTextContent('Uploading reference video…')
    await act(async () => upload.resolve())
    expect(
      upload.post.mock.calls.filter(
        ([url]) => url === '/api/v1/files/upload/stream'
      )
    ).toHaveLength(1)
  })

  it('on mobile Escape hides the dialog while reopening restores the pending upload', async () => {
    const width = window.innerWidth
    Object.defineProperty(window, 'innerWidth', {
      configurable: true,
      value: 390,
    })
    const upload = pendingUpload()
    const user = userEvent.setup()
    try {
      await startUpload(user)
      await user.keyboard('{Escape}')
      expect(
        screen.queryByRole('dialog', { name: 'Select reference videos' })
      ).not.toBeInTheDocument()
      await user.click(screen.getByRole('button', { name: ENTRY }))
      const dialog = screen.getByRole('dialog', {
        name: 'Select reference videos',
      })
      expect(within(dialog).getByRole('status')).toHaveTextContent(
        'Uploading reference video…'
      )
      await act(async () => upload.resolve())
      expect(
        await within(dialog).findByRole('button', { name: 'reference.mp4' })
      ).toHaveAttribute('aria-pressed', 'true')
      expect(
        upload.post.mock.calls.filter(
          ([url]) => url === '/api/v1/files/upload/stream'
        )
      ).toHaveLength(1)
    } finally {
      Object.defineProperty(window, 'innerWidth', {
        configurable: true,
        value: width,
      })
    }
  })

  it('changing accounts while closed prevents the old upload from selecting media for the new account', async () => {
    const upload = pendingUpload()
    const user = userEvent.setup()
    await startUpload(user)
    await closePanel(user)
    act(() =>
      useAuthStore
        .getState()
        .auth.setUser({ id: 8, username: 'other', role: 1 })
    )
    await act(async () => upload.resolve())
    await user.click(screen.getByRole('button', { name: ENTRY }))
    expect(
      screen.queryByRole('button', { name: 'reference.mp4' })
    ).not.toBeInTheDocument()
    expect(screen.queryByText('reference.mp4')).not.toBeInTheDocument()
  })
})
