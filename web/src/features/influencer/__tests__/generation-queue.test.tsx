import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import type { SheetSubmitBody } from '../api'
import { FeedGrid } from '../components/feed-grid'
import { useMotion } from '../hooks/use-motion'
import { useMultiViewSubmit } from '../hooks/use-multiview'
import {
  useInfluencerHistory,
  useMotionSubmit,
  useSheetSubmit,
} from '../hooks/use-tasks'
import type { TaskRecord } from '../types'

const body: SheetSubmitBody = {
  model: 'gpt-image-2.5-sunburst',
  group: 'auto',
  tier: 'total',
  selection: {},
  brief: 'A character',
  size: '16:9',
}
const page = (items: TaskRecord[]) => ({
  data: { success: true, data: { items, total: items.length } },
})
const task = (
  id: string,
  action = 'influencer_sheet',
  status = 'QUEUED'
): TaskRecord => ({ id: 1, task_id: id, action, status })
function account(id: number) {
  useAuthStore.setState((state) => ({
    auth: { ...state.auth, user: { id, username: `user-${id}`, role: 1 } },
  }))
}
function wrapper() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return (props: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
}
beforeEach(() => {
  vi.useFakeTimers()
  localStorage.clear()
  account(1)
})
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  useAuthStore.getState().auth.reset()
})

describe('persistent generation queue client', () => {
  it('accepts six image submissions without client capacity rejection and retains every ID after read errors', async () => {
    const post = vi.spyOn(api, 'post').mockImplementation(async () => ({
      data: { task_id: `image-${post.mock.calls.length}` },
    }))
    vi.spyOn(api, 'get').mockRejectedValue(new Error('Read unavailable'))
    const { result } = renderHook(() => useSheetSubmit())
    for (let i = 0; i < 6; i++) {
      await act(async () => {
        await result.current.submit(body)
      })
    }
    expect(post).toHaveBeenCalledTimes(6)
    expect(result.current).toMatchObject({
      tasks: Array.from({ length: 6 }, (_, i) =>
        expect.objectContaining({ task_id: `image-${i + 1}`, status: 'QUEUED' })
      ),
    })
  })

  it('retains a confirmed terminal record when the artifact projection read fails', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({ data: { task_id: 'finished' } })
    vi.spyOn(api, 'get').mockImplementation(async (_url, config) => {
      if (config?.params.with_artifacts) {
        throw new Error('Artifact projection unavailable')
      }
      return page([task('finished', 'influencer_sheet', 'SUCCESS')])
    })
    const { result } = renderHook(() => useSheetSubmit())
    await act(async () => {
      await result.current.submit(body)
    })
    expect(result.current.pending?.status).toBe('SUCCESS')
  })

  it('blocks duplicate clicks only while POST is outstanding', async () => {
    let finish!: (value: unknown) => void
    const post = vi.spyOn(api, 'post').mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve
        })
    )
    vi.spyOn(api, 'get').mockResolvedValue(page([]))
    const { result } = renderHook(() => useSheetSubmit())
    let first!: Promise<unknown>
    await act(async () => {
      first = result.current.submit(body)
      await result.current.submit(body)
    })
    expect(post).toHaveBeenCalledTimes(1)
    await act(async () => {
      finish({ data: { task_id: 'one' } })
      await first
    })
    expect(result.current.submitting).toBe(false)
  })

  it('polls earlier image and video IDs independently and settles only the finished record', async () => {
    vi.spyOn(api, 'post')
      .mockResolvedValueOnce({ data: { task_id: 'one' } })
      .mockResolvedValueOnce({ data: { task_id: 'two' } })
      .mockResolvedValueOnce({ data: { task_id: 'video' } })
    let settled = false
    const get = vi
      .spyOn(api, 'get')
      .mockImplementation(async (_url, config) =>
        page([
          task(
            config?.params.task_id,
            config?.params.action,
            settled && config?.params.task_id === 'one' ? 'SUCCESS' : 'QUEUED'
          ),
        ])
      )
    const { result } = renderHook(() => ({
      sheet: useSheetSubmit(),
      motion: useMotionSubmit(),
    }))
    await act(async () => {
      await result.current.sheet.submit(body)
      await result.current.sheet.submit(body)
      await result.current.motion.submit({
        mode: 'motion',
        prompt: 'Move',
        subject_image_url: 'https://example.org/image.png',
        video_url: 'https://example.org/video.mp4',
        duration_sec: 8,
        aspect_ratio: '16:9',
        resolution: '720p',
      })
    })
    settled = true
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(result.current.sheet).toMatchObject({
      tasks: [
        expect.objectContaining({ task_id: 'one', status: 'SUCCESS' }),
        expect.objectContaining({ task_id: 'two', status: 'QUEUED' }),
      ],
    })
    const reads = get.mock.calls.length
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(
      get.mock.calls
        .slice(reads)
        .map((call) => call[1]?.params.task_id)
        .sort()
    ).toEqual(['two', 'video'])
  })

  it('clears accepted image and child records when switching accounts and ignores late reads', async () => {
    let finish!: (value: ReturnType<typeof page>) => void
    vi.spyOn(api, 'post').mockResolvedValue({ data: { task_id: 'old' } })
    const get = vi.spyOn(api, 'get').mockImplementation(
      () =>
        new Promise((resolve) => {
          finish = resolve
        })
    )
    const { result } = renderHook(() => ({
      sheet: useSheetSubmit(),
      board: useMultiViewSubmit(),
    }))
    let first!: Promise<unknown>
    await act(async () => {
      first = result.current.sheet.submit(body)
    })
    act(() => account(2))
    await act(async () => {
      finish(page([task('old')]))
      await first
    })
    expect(result.current.sheet.pending).toBeNull()
    const reads = get.mock.calls.length
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4000)
    })
    expect(get).toHaveBeenCalledTimes(reads)
  })

  it('rejects another in-flight board for the same parent but accepts a different parent', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { task_id: 'board' } })
    vi.spyOn(api, 'get').mockResolvedValue(page([]))
    const { result } = renderHook(() => useMultiViewSubmit())
    const board = {
      model: 'gpt-image-2.5-sunburst',
      group: 'auto',
      parentTaskId: 'parent',
      imageUrl: 'https://example.org/image.png',
    }
    await act(async () => {
      await result.current.submit(board)
      await result.current.submit(board)
    })
    expect(post).toHaveBeenCalledTimes(1)
    await act(async () => {
      await result.current.submit({ ...board, parentTaskId: 'other' })
    })
    expect(Object.keys(result.current.childByParent).sort()).toEqual([
      'other',
      'parent',
    ])
  })

  it('resumes refreshed queued history through the existing endpoint until it reaches terminal status', async () => {
    let status = 'QUEUED'
    const get = vi
      .spyOn(api, 'get')
      .mockImplementation(async (_url, config) =>
        page(
          config?.params.action === 'influencer_sheet'
            ? [task('restored', 'influencer_sheet', status)]
            : []
        )
      )
    const { result } = renderHook(() => useInfluencerHistory(), {
      wrapper: wrapper(),
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(10)
    })
    expect(result.current.items[0]?.status).toBe('QUEUED')
    status = 'SUCCESS'
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2010)
    })
    expect(result.current.items[0]?.status).toBe('SUCCESS')
    const statusReads = () =>
      get.mock.calls.filter(([, config]) => !config?.params.task_id).length
    const reads = statusReads()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(6000)
    })
    // Terminal generation polling stops; bounded artifact enrichment is a separate read.
    expect(statusReads()).toBe(reads)
  })

  it('keeps video generation unavailable while a character reference uploads', async () => {
    type ConfirmResponse = {
      data: { success: boolean; data: { file_url: string } }
    }
    let finish!: (value: ConfirmResponse) => void
    const confirmation = new Promise<ConfirmResponse>((resolve) => {
      finish = resolve
    })
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200 }))
    vi.spyOn(api, 'post').mockImplementation(async (url) => {
      if (url === '/api/v1/files/upload/presign') {
        return {
          data: {
            success: true,
            data: {
              file_id: 'character-file',
              upload_url:
                'https://account.r2.cloudflarestorage.com/files/character?signature=test',
              upload_method: 'PUT',
              upload_headers: {
                'Content-Type': 'image/png',
                'If-None-Match': '*',
              },
            },
          },
        }
      }
      if (url === '/api/v1/files/upload/confirm') return await confirmation
      throw new Error(`Unexpected POST: ${url}`)
    })
    const { result } = renderHook(() => useMotion())
    let upload!: Promise<void>
    await act(async () => {
      upload = result.current.pickCharacter(
        new File(['image'], 'character.png', { type: 'image/png' })
      )
    })
    expect(result.current).toMatchObject({ characterUploading: true })
    await act(async () => {
      finish({
        data: {
          success: true,
          data: { file_url: 'https://example.org/character.png' },
        },
      })
      await upload
    })
    expect(result.current).toMatchObject({
      characterUploading: false,
      character: { url: 'https://example.org/character.png' },
    })
  })

  it('clears accepted child state on account switch and rejects a late old-account POST', async () => {
    let finish!: (value: unknown) => void
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValueOnce({ data: { task_id: 'first' } })
      .mockImplementation(
        () =>
          new Promise((resolve) => {
            finish = resolve
          })
      )
    vi.spyOn(api, 'get').mockResolvedValue(page([]))
    const { result } = renderHook(() => useMultiViewSubmit())
    const board = {
      model: 'gpt-image-2.5-sunburst',
      group: 'auto',
      parentTaskId: 'parent',
      imageUrl: 'https://example.org/image.png',
    }
    await act(async () => {
      await result.current.submit(board)
    })
    let late!: Promise<unknown>
    await act(async () => {
      late = result.current.submit({ ...board, parentTaskId: 'late-parent' })
    })
    act(() => account(2))
    await act(async () => {
      finish({ data: { task_id: 'late-old' } })
      await late
    })
    expect(result.current.childByParent).toEqual({})
    expect(result.current.submittingParent).toBeNull()
    expect(post).toHaveBeenCalledTimes(2)
  })

  it('shows all newly accepted mixed tasks once, preferring the refreshed server record over a local placeholder', () => {
    render(
      <FeedGrid
        tasks={[
          task('image', 'influencer_sheet', 'IN_PROGRESS'),
          task('video', 'influencer_motion'),
        ]}
        pending={[
          task('image'),
          task('video', 'influencer_motion'),
          task('extra'),
        ]}
        isInitialLoading={false}
        hasMore={false}
        isFetchingMore={false}
        loadMoreFailed={false}
        onLoadMore={vi.fn()}
        onView={vi.fn()}
      />
    )
    expect(screen.getAllByText('Queued')).toHaveLength(2)
    expect(screen.getAllByText('Generating…')).toHaveLength(1)
  })
})
