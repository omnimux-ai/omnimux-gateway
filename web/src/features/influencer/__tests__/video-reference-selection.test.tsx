import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { handleServerError } from '@/lib/handle-server-error'

import { fetchTaskRecord, inspectMotionVideo } from '../api'
import { useVideoReferenceSelection } from '../hooks/use-video-reference-selection'
import {
  readBrowserVideoMetadata,
  type BrowserVideoMetadata,
} from '../lib/browser-video-metadata'
import { validateBrowserMotionMetadata } from '../lib/motion'
import type { MotionVideoMetadata } from '../types'

vi.mock('../api', () => ({
  fetchTaskRecord: vi.fn(),
  inspectMotionVideo: vi.fn(),
}))
vi.mock('../lib/browser-video-metadata', () => ({
  readBrowserVideoMetadata: vi.fn(),
}))
vi.mock('@/lib/handle-server-error', () => ({ handleServerError: vi.fn() }))
const raw: BrowserVideoMetadata = {
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
const checked = validateBrowserMotionMetadata(raw)
if (!checked.ok) throw new Error('Bad fixture')
const metadata: MotionVideoMetadata = checked
const uploadReference =
  vi.fn<
    (
      file: File,
      metadata: MotionVideoMetadata,
      signal?: AbortSignal
    ) => Promise<{ url: string; metadata: MotionVideoMetadata }>
  >()
const task = {
  id: 1,
  task_id: 'clip-1',
  action: 'influencer_motion',
  status: 'SUCCESS',
  artifacts: [
    {
      key: 'video',
      content_url: 'https://media.example/clip.mp4?access=permission',
    },
  ],
}
const file = () => new File(['clip'], 'clip.mp4', { type: 'video/mp4' })
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason: unknown) => void
  const promise = new Promise<T>((done, fail) => {
    resolve = done
    reject = fail
  })
  return { promise, resolve, reject }
}
beforeEach(() => {
  vi.mocked(readBrowserVideoMetadata).mockResolvedValue(raw)
  vi.mocked(fetchTaskRecord).mockResolvedValue(task)
  uploadReference.mockImplementation(async (_file, metadata) => ({
    url: 'https://files.example/clip.mp4',
    metadata,
  }))
})
afterEach(() => {
  cleanup()
  vi.resetAllMocks()
})

describe('browser-only video selection', () => {
  it('reads once before upload and persists that same complete result without a server probe', async () => {
    const read = deferred<BrowserVideoMetadata>()
    vi.mocked(readBrowserVideoMetadata).mockReturnValueOnce(read.promise)
    const onCommit = vi.fn()
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit, upload: uploadReference })
    )
    const source = file()
    let pending!: Promise<unknown>
    act(() => {
      pending = result.current.pickVideo(source)
    })
    expect(result.current.phase).toBe('reading')
    expect(uploadReference).not.toHaveBeenCalled()
    await act(async () => {
      read.resolve(raw)
      await pending
    })
    expect(readBrowserVideoMetadata).toHaveBeenCalledExactlyOnceWith(
      source,
      expect.any(AbortSignal)
    )
    expect(uploadReference).toHaveBeenCalledExactlyOnceWith(
      source,
      expect.objectContaining({ browserMetadata: raw, version: 1 }),
      expect.any(AbortSignal)
    )
    expect(onCommit).toHaveBeenCalledWith(
      expect.objectContaining({
        kind: 'upload',
        metadata: expect.objectContaining({ browserMetadata: raw }),
      })
    )
    expect(inspectMotionVideo).not.toHaveBeenCalled()
  })
  it.each(['motion', 'replace'] as const)(
    'uploads the reported AAC-tail reference once in %s mode and preserves its original metadata',
    async (mode) => {
      const candidate: BrowserVideoMetadata = Object.freeze({
        ...raw,
        duration: 7.072,
        video_duration: 7.041667,
        width: 480,
        height: 854,
        frame_rate: 24,
        size_bytes: 6650581,
      })
      vi.mocked(readBrowserVideoMetadata).mockResolvedValueOnce(candidate)
      const onCommit = vi.fn()
      const { result } = renderHook(() =>
        useVideoReferenceSelection({ mode, onCommit, upload: uploadReference })
      )
      const source = new File(
        [new Uint8Array(candidate.size_bytes)],
        'video (1).mp4',
        {
          type: 'video/mp4',
        }
      )
      let verdict: unknown
      await act(async () => {
        verdict = await result.current.pickVideo(source)
      })
      expect(verdict).toMatchObject({
        ok: true,
        rawDurationSec: 7.072,
        durationSec: 7,
        aspectRatio: '9:16',
      })
      expect(uploadReference).toHaveBeenCalledExactlyOnceWith(
        source,
        expect.objectContaining({ browserMetadata: candidate, durationSec: 7 }),
        expect.any(AbortSignal)
      )
      expect(onCommit).toHaveBeenCalledExactlyOnceWith(
        expect.objectContaining({
          metadata: expect.objectContaining({ browserMetadata: candidate }),
        })
      )
      expect(uploadReference.mock.calls[0][1].browserMetadata).toBe(candidate)
      expect(inspectMotionVideo).not.toHaveBeenCalled()
      expect(handleServerError).not.toHaveBeenCalled()
    }
  )
  it('uploads a reported audio-padding tail without audio and keeps its declarations', async () => {
    const candidate: BrowserVideoMetadata = Object.freeze({
      ...raw,
      duration: 7.072,
      video_duration: 7.041667,
      frame_rate: 24,
      audio_codec: '',
    })
    vi.mocked(readBrowserVideoMetadata).mockResolvedValueOnce(candidate)
    const onCommit = vi.fn()
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit, upload: uploadReference })
    )
    let verdict: unknown
    await act(async () => {
      verdict = await result.current.pickVideo(file())
    })
    expect(verdict).toMatchObject({ ok: true, durationSec: 7 })
    expect(uploadReference).toHaveBeenCalledOnce()
    expect(onCommit).toHaveBeenCalledOnce()
    expect(handleServerError).not.toHaveBeenCalled()
  })
  it.each(['motion', 'replace'] as const)(
    'uploads the reported fractional %s reference and commits six seconds',
    async (mode) => {
      const candidate: BrowserVideoMetadata = Object.freeze({
        ...raw,
        duration: 5.717007,
        video_duration: 5.708333,
        width: 720,
        height: 1280,
        frame_rate: 24,
        size_bytes: 2863940,
      })
      vi.mocked(readBrowserVideoMetadata).mockResolvedValueOnce(candidate)
      const onCommit = vi.fn()
      const { result } = renderHook(() =>
        useVideoReferenceSelection({ mode, onCommit, upload: uploadReference })
      )
      let verdict: unknown
      await act(async () => {
        verdict = await result.current.pickVideo(file())
      })
      expect(verdict).toMatchObject({
        ok: true,
        rawDurationSec: 5.717007,
        durationSec: 6,
        aspectRatio: '9:16',
      })
      expect(uploadReference).toHaveBeenCalledExactlyOnceWith(
        expect.any(File),
        expect.objectContaining({ browserMetadata: candidate, durationSec: 6 }),
        expect.any(AbortSignal)
      )
      expect(onCommit).toHaveBeenCalledOnce()
      expect(handleServerError).not.toHaveBeenCalled()
    }
  )
  it('reports an inconsistent declaration and preserves the prior source without uploading', async () => {
    const tail = {
      duration: 7.2,
      video_duration: 7.041667,
      audio_codec: 'aac',
    }
    const onCommit = vi.fn()
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit, upload: uploadReference })
    )
    await act(async () => {
      await result.current.pickVideo(file())
    })
    const selected = onCommit.mock.calls[0][0]
    uploadReference.mockClear()
    vi.mocked(readBrowserVideoMetadata).mockResolvedValueOnce({
      ...raw,
      ...tail,
      frame_rate: 24,
    })
    let verdict: unknown
    await act(async () => {
      verdict = await result.current.pickVideo(file())
    })
    expect(verdict).toEqual({ ok: false, reason: 'output-duration' })
    expect(uploadReference).not.toHaveBeenCalled()
    expect(onCommit).toHaveBeenCalledOnce()
    expect(onCommit.mock.calls[0][0]).toBe(selected)
    expect(handleServerError).toHaveBeenCalledOnce()
    expect(inspectMotionVideo).not.toHaveBeenCalled()
  })
  it.each([
    [{ ...raw, duration: 1, video_duration: 1 }, 'duration'],
    [{ ...raw, duration: 2, video_duration: 2 }, 'output-duration'],
    [{ ...raw, duration: 3.999, video_duration: 3.999 }, 'output-duration'],
    [{ ...raw, duration: 7.2, video_duration: 7.041667 }, 'output-duration'],
    [{ ...raw, frame_rate: 23 }, 'frame-rate'],
    [{ ...raw, width: 300, height: 300 }, 'pixels'],
    [{ ...raw, width: 1200, height: 960 }, 'output-aspect'],
    [{ ...raw, video_codec: 'av1' }, 'codec'],
  ])(
    'rejects unsupported input or unpreservable output with zero upload',
    async (candidate, reason) => {
      vi.mocked(readBrowserVideoMetadata).mockResolvedValueOnce(candidate)
      const onCommit = vi.fn()
      const { result } = renderHook(() =>
        useVideoReferenceSelection({ onCommit, upload: uploadReference })
      )
      let verdict: unknown
      await act(async () => {
        verdict = await result.current.pickVideo(file())
      })
      expect(verdict).toEqual({ ok: false, reason })
      expect(uploadReference).not.toHaveBeenCalled()
      expect(inspectMotionVideo).not.toHaveBeenCalled()
      expect(onCommit).not.toHaveBeenCalled()
      expect(handleServerError).toHaveBeenCalledOnce()
    }
  )
  it('rejects 3:4 at 720p without replacing the prior source', async () => {
    const onCommit = vi.fn()
    const { result } = renderHook(() =>
      useVideoReferenceSelection({
        onCommit,
        upload: uploadReference,
        resolution: '720p',
      })
    )
    await act(async () => {
      await result.current.pickVideo(file())
    })
    vi.mocked(readBrowserVideoMetadata).mockResolvedValueOnce({
      ...raw,
      width: 960,
      height: 1280,
    })
    await act(async () => {
      await result.current.pickVideo(file())
    })
    expect(onCommit).toHaveBeenCalledOnce()
    expect(uploadReference).toHaveBeenCalledOnce()
    expect(handleServerError).toHaveBeenCalledOnce()
  })
  it.each(['timeout', 'unreadable', 'unsupported'])(
    'never uploads after a local %s failure',
    async (reason) => {
      vi.mocked(readBrowserVideoMetadata).mockRejectedValueOnce(
        new Error(reason)
      )
      const { result } = renderHook(() =>
        useVideoReferenceSelection({
          onCommit: vi.fn(),
          upload: uploadReference,
        })
      )
      await act(async () => {
        await result.current.pickVideo(file())
      })
      expect(uploadReference).not.toHaveBeenCalled()
      expect(handleServerError).toHaveBeenCalledOnce()
    }
  )
  it('rejects file basics without starting the reader or upload', async () => {
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit: vi.fn(), upload: uploadReference })
    )
    await act(async () => {
      await result.current.pickVideo(
        new File(['x'], 'x.webm', { type: 'video/webm' })
      )
    })
    expect(readBrowserVideoMetadata).not.toHaveBeenCalled()
    expect(uploadReference).not.toHaveBeenCalled()
  })
  it('cancels local reading, suppresses late results and unlocks immediately', async () => {
    const read = deferred<BrowserVideoMetadata>()
    vi.mocked(readBrowserVideoMetadata).mockReturnValueOnce(read.promise)
    const onCommit = vi.fn()
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit, upload: uploadReference })
    )
    let pending!: Promise<unknown>
    act(() => {
      pending = result.current.pickVideo(file())
      expect(result.current.isPreparing()).toBe(true)
    })
    const signal = vi.mocked(readBrowserVideoMetadata).mock.calls[0][1]
    act(() => result.current.cancel())
    expect(signal?.aborted).toBe(true)
    await act(async () => {
      read.resolve(raw)
      await pending
    })
    expect(uploadReference).not.toHaveBeenCalled()
    expect(onCommit).not.toHaveBeenCalled()
    expect(handleServerError).not.toHaveBeenCalled()
    expect(result.current.isPreparing()).toBe(false)
  })
  it('passes the live abort signal into upload and suppresses its stale completion', async () => {
    const upload = deferred<{ url: string; metadata: MotionVideoMetadata }>()
    uploadReference.mockReturnValueOnce(upload.promise)
    const onCommit = vi.fn()
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit, upload: uploadReference })
    )
    let pending!: Promise<unknown>
    await act(async () => {
      pending = result.current.pickVideo(file())
    })
    const signal = uploadReference.mock.calls[0][2]
    act(() => result.current.cancel())
    expect(signal?.aborted).toBe(true)
    await act(async () => {
      upload.resolve({ url: 'https://files.example/late.mp4', metadata })
      await pending
    })
    expect(onCommit).not.toHaveBeenCalled()
    expect(handleServerError).not.toHaveBeenCalled()
  })
  it('reuses current complete uploaded metadata without a read, and rechecks legacy records locally', async () => {
    const onCommit = vi.fn()
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit, upload: uploadReference })
    )
    await act(async () => {
      await result.current.pickUploaded({
        kind: 'upload',
        url: 'https://files.example/current.mp4',
        fileName: 'current.mp4',
        metadata,
      })
    })
    expect(readBrowserVideoMetadata).not.toHaveBeenCalled()
    await act(async () => {
      await result.current.pickUploaded({
        kind: 'upload',
        url: 'https://files.example/legacy.mp4',
        fileName: 'legacy.mp4',
        metadata: {
          rawDurationSec: 8,
          durationSec: 8,
          width: 720,
          height: 1280,
          aspectRatio: '9:16',
        },
      })
    })
    expect(readBrowserVideoMetadata).toHaveBeenCalledExactlyOnceWith(
      'https://files.example/legacy.mp4',
      expect.any(AbortSignal)
    )
    expect(uploadReference).not.toHaveBeenCalled()
  })
  it('reads the full-quality original rather than its preview and refreshes mutable remote URLs', async () => {
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit: vi.fn(), upload: uploadReference })
    )
    const source = {
      kind: 'remote' as const,
      url: 'https://media.example/original.mp4',
      previewUrl: 'https://media.example/preview.mp4',
      name: 'Original',
    }
    await act(async () => {
      await result.current.pickRemote(source)
      await result.current.pickRemote(source)
    })
    expect(readBrowserVideoMetadata).toHaveBeenCalledTimes(2)
    expect(readBrowserVideoMetadata).toHaveBeenLastCalledWith(
      source.url,
      expect.any(AbortSignal)
    )
  })
  it('refreshes history permissions then reads the exact same artifact', async () => {
    const onCommit = vi.fn()
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit, upload: uploadReference })
    )
    await act(async () => {
      await result.current.pickHistory(task)
    })
    expect(readBrowserVideoMetadata).toHaveBeenCalledWith(
      task.artifacts[0].content_url,
      expect.any(AbortSignal)
    )
    expect(onCommit).toHaveBeenCalledWith(
      expect.objectContaining({ taskId: 'clip-1', artifactKey: 'video' })
    )
    expect(uploadReference).not.toHaveBeenCalled()
  })
  it('reuses persisted history metadata instead of reading the video again', async () => {
    const onCommit = vi.fn()
    vi.mocked(fetchTaskRecord).mockResolvedValue({
      ...task,
      artifacts: [
        {
          key: 'video',
          content_url: 'https://media.example/clip.mp4?access=permission',
          video_metadata: raw,
        },
      ],
    })
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit, upload: uploadReference })
    )
    await act(async () => {
      await result.current.pickHistory(task)
    })
    expect(readBrowserVideoMetadata).not.toHaveBeenCalled()
    expect(onCommit).toHaveBeenCalledWith(
      expect.objectContaining({
        taskId: 'clip-1',
        artifactKey: 'video',
        metadata: expect.objectContaining({
          version: 1,
          durationSec: 8,
          width: 720,
          height: 1280,
          aspectRatio: '9:16',
        }),
      })
    )
  })
  it('falls back to the browser read when persisted history metadata is incomplete', async () => {
    const onCommit = vi.fn()
    vi.mocked(fetchTaskRecord).mockResolvedValue({
      ...task,
      artifacts: [
        {
          key: 'video',
          content_url: 'https://media.example/clip.mp4?access=permission',
          video_metadata: { ...raw, frame_rate: 0 },
        },
      ],
    })
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit, upload: uploadReference })
    )
    await act(async () => {
      await result.current.pickHistory(task)
    })
    expect(readBrowserVideoMetadata).toHaveBeenCalledExactlyOnceWith(
      'https://media.example/clip.mp4?access=permission',
      expect.any(AbortSignal)
    )
    expect(onCommit).toHaveBeenCalledWith(
      expect.objectContaining({ taskId: 'clip-1', artifactKey: 'video' })
    )
  })
  it('does not substitute another artifact after history refresh', async () => {
    vi.mocked(fetchTaskRecord).mockResolvedValue({
      ...task,
      artifacts: [
        {
          key: 'other',
          type: 'video',
          content_url: 'https://media.example/other.mp4',
        },
      ],
    })
    const onCommit = vi.fn()
    const { result } = renderHook(() =>
      useVideoReferenceSelection({ onCommit, upload: uploadReference })
    )
    await act(async () => {
      await result.current.pickHistory(task)
    })
    expect(onCommit).not.toHaveBeenCalled()
    expect(readBrowserVideoMetadata).not.toHaveBeenCalled()
    expect(handleServerError).toHaveBeenCalledOnce()
  })
})
