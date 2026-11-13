import { act, renderHook, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useAuthStore, type AuthUser } from '@/stores/auth-store'

import { uploadReferenceAsset } from '../api'
import { useBuilder } from '../hooks/use-builder'
import { useMotion } from '../hooks/use-motion'
import { readBrowserVideoMetadata } from '../lib/browser-video-metadata'
import { resetUploadCache } from '../lib/upload-library'

vi.mock('../api', () => ({
  uploadReferenceAsset: vi.fn(),
  fetchTaskRecord: vi.fn(),
}))
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
const VIDEO_METADATA = {
  version: 1,
  browserMetadata: RAW_VIDEO_METADATA,
  rawDurationSec: 8,
  durationSec: 8,
  width: 720,
  height: 1280,
  aspectRatio: '9:16',
}
const HOUR = 60 * 60 * 1000

function signIn(id: number | null) {
  useAuthStore
    .getState()
    .auth.setUser(
      id === null ? null : ({ id, username: `user-${id}`, role: 1 } as AuthUser)
    )
}

function stored(url: string) {
  return { url, expiresAt: Date.now() + 72 * HOUR }
}

function pendingAsset() {
  let resolve!: (value: ReturnType<typeof stored>) => void
  const promise = new Promise<ReturnType<typeof stored>>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

/** A page refresh drops all in-memory state; only browser storage survives. */
function refresh() {
  resetUploadCache()
}

beforeEach(() => {
  localStorage.clear()
  resetUploadCache()
  signIn(7)
  vi.mocked(readBrowserVideoMetadata)
    .mockReset()
    .mockImplementation(async (source) => ({
      ...RAW_VIDEO_METADATA,
      size_bytes:
        typeof source === 'string'
          ? RAW_VIDEO_METADATA.size_bytes
          : source.size,
    }))
})

afterEach(() => {
  vi.mocked(uploadReferenceAsset).mockReset()
  signIn(null)
})

describe('page upload library', () => {
  it.each(['image', 'video'] as const)(
    'keeps concurrent image and video selections and library entries when %s finishes first',
    async (firstCompleted) => {
      const image = pendingAsset()
      const video = pendingAsset()
      const imageFile = new File(['image'], 'character.png', {
        type: 'image/png',
      })
      const videoFile = new File(['clip'], 'dance.mp4', { type: 'video/mp4' })
      vi.mocked(uploadReferenceAsset).mockImplementation((file) =>
        file.type.startsWith('image/') ? image.promise : video.promise
      )
      const hook = renderHook(() => useMotion())
      let pendingImage!: Promise<void>
      let pendingVideo!: ReturnType<typeof hook.result.current.pickVideo>
      act(() => {
        pendingVideo = hook.result.current.pickVideo(videoFile)
        pendingImage = hook.result.current.pickCharacter(imageFile)
        void hook.result.current.pickVideo(videoFile)
        void hook.result.current.pickCharacter(imageFile)
      })
      await waitFor(() => expect(uploadReferenceAsset).toHaveBeenCalledTimes(2))
      expect(hook.result.current.characterUploading).toBe(true)
      expect(hook.result.current.videoPreparing).toBe(true)

      const finishImage = async () => {
        await act(async () => {
          image.resolve(stored('https://media.example.com/character.png'))
          await pendingImage
        })
        expect(hook.result.current.characters).toEqual([
          {
            url: 'https://media.example.com/character.png',
            fileName: 'character.png',
          },
        ])
        expect(hook.result.current.characterUploading).toBe(false)
      }
      const finishVideo = async () => {
        await act(async () => {
          video.resolve(stored('https://media.example.com/dance.mp4'))
          await pendingVideo
        })
        expect(hook.result.current.video).toMatchObject({
          kind: 'upload',
          url: 'https://media.example.com/dance.mp4',
          fileName: 'dance.mp4',
        })
        expect(hook.result.current.videoPreparing).toBe(false)
      }
      if (firstCompleted === 'image') {
        await finishImage()
        expect(hook.result.current.videoPreparing).toBe(true)
        await finishVideo()
      } else {
        await finishVideo()
        expect(hook.result.current.characterUploading).toBe(true)
        await finishImage()
      }
      expect(hook.result.current.uploadedCharacters).toHaveLength(1)
      expect(hook.result.current.uploadedVideos).toHaveLength(1)
      hook.unmount()
      refresh()
      const restored = renderHook(() => useMotion())
      expect(restored.result.current.uploadedCharacters).toEqual([
        {
          url: 'https://media.example.com/character.png',
          fileName: 'character.png',
        },
      ])
      expect(restored.result.current.uploadedVideos).toEqual([
        expect.objectContaining({
          url: 'https://media.example.com/dance.mp4',
          fileName: 'dance.mp4',
        }),
      ])
    }
  )

  it('keeps an uploaded video listed after the page refreshes', async () => {
    vi.mocked(uploadReferenceAsset).mockResolvedValue(
      stored('https://media.example.com/dance.mp4')
    )
    const first = renderHook(() => useMotion())
    await act(async () => {
      await first.result.current.pickVideo(
        new File(['clip'], 'dance.mp4', { type: 'video/mp4' })
      )
    })
    first.unmount()
    refresh()

    const { result } = renderHook(() => useMotion())

    expect(result.current.uploadedVideos).toEqual([
      {
        kind: 'upload',
        url: 'https://media.example.com/dance.mp4',
        fileName: 'dance.mp4',
        metadata: VIDEO_METADATA,
      },
    ])
  })

  it('lets a restored video be selected again as a submittable reference', async () => {
    vi.mocked(uploadReferenceAsset).mockResolvedValue(
      stored('https://media.example.com/dance.mp4')
    )
    const first = renderHook(() => useMotion())
    await act(async () => {
      await first.result.current.pickVideo(
        new File(['clip'], 'dance.mp4', { type: 'video/mp4' })
      )
    })
    first.unmount()
    refresh()
    const { result } = renderHook(() => useMotion())

    await act(async () => {
      await result.current.selectUploadedVideo(result.current.uploadedVideos[0])
    })

    expect(result.current.video).toMatchObject({
      kind: 'upload',
      url: 'https://media.example.com/dance.mp4',
    })
    expect(result.current.durationSec).toBe(8)
    expect(readBrowserVideoMetadata).toHaveBeenCalledTimes(1)
    expect(
      result.current.video?.kind !== 'preset' && result.current.video?.metadata
    ).toMatchObject(VIDEO_METADATA)
  })

  it('records no library entry and uploads no bytes when the file fails browser inspection', async () => {
    vi.mocked(uploadReferenceAsset).mockResolvedValue(
      stored('https://media.example.com/bad.mp4')
    )
    vi.mocked(readBrowserVideoMetadata).mockRejectedValueOnce(
      new Error('Video exceeds supported pixel count.')
    )
    const { result } = renderHook(() => useMotion())
    await act(async () => {
      await result.current.pickVideo(
        new File(['clip'], 'bad.mp4', { type: 'video/mp4' })
      )
    })
    expect(result.current.uploadedVideos).toEqual([])
    expect(result.current.video).toBeNull()
    expect(uploadReferenceAsset).not.toHaveBeenCalled()
  })

  it('keeps an uploaded character image after deselection and a refresh', async () => {
    vi.mocked(uploadReferenceAsset).mockResolvedValue(
      stored('https://media.example.com/photo.webp')
    )
    const first = renderHook(() => useMotion())
    await act(async () => {
      await first.result.current.pickCharacter(
        new File(['image'], 'photo.webp')
      )
    })
    act(() => first.result.current.clearCharacter())
    first.unmount()
    refresh()

    const { result } = renderHook(() => useMotion())

    expect(result.current.characters).toEqual([])
    expect(result.current.uploadedCharacters).toEqual([
      { url: 'https://media.example.com/photo.webp', fileName: 'photo.webp' },
    ])
  })

  it('adds the character-design style reference to the same image library', async () => {
    vi.mocked(uploadReferenceAsset).mockResolvedValue(
      stored('https://media.example.com/style.png')
    )
    const builder = renderHook(() => useBuilder(null))
    await act(async () => {
      await builder.result.current.pickImage(
        new File(['image'], 'style.png', { type: 'image/png' })
      )
    })

    const { result } = renderHook(() => useMotion())

    expect(result.current.uploadedCharacters).toEqual([
      { url: 'https://media.example.com/style.png', fileName: 'style.png' },
    ])
  })

  it('records nothing when the upload fails', async () => {
    vi.mocked(uploadReferenceAsset).mockRejectedValueOnce(new Error('rejected'))
    const { result } = renderHook(() => useMotion())

    await act(async () => {
      await expect(
        result.current.pickCharacter(new File(['image'], 'bad.png'))
      ).rejects.toThrow('rejected')
    })

    expect(result.current.uploadedCharacters).toEqual([])
  })

  it('shows each account only its own uploads when the signed-in account changes', async () => {
    vi.mocked(uploadReferenceAsset).mockResolvedValue(
      stored('https://media.example.com/mine.png')
    )
    const { result } = renderHook(() => useMotion())
    await act(async () => {
      await result.current.pickCharacter(new File(['image'], 'mine.png'))
    })

    act(() => signIn(8))
    expect(result.current.uploadedCharacters).toEqual([])
    act(() => signIn(null))
    expect(result.current.uploadedCharacters).toEqual([])
    act(() => signIn(7))
    expect(result.current.uploadedCharacters).toHaveLength(1)
  })

  it('keeps a late upload out of the form after an account reset', async () => {
    let resolve!: (value: ReturnType<typeof stored>) => void
    vi.mocked(uploadReferenceAsset).mockReturnValueOnce(
      new Promise((done) => {
        resolve = done
      })
    )
    const { result } = renderHook(() => useMotion())
    let pending!: Promise<void>
    act(() => {
      pending = result.current.pickCharacter(new File(['image'], 'late.png'))
    })
    expect(result.current.characterUploading).toBe(true)

    act(() => result.current.reset())
    await act(async () => {
      resolve(stored('https://media.example.com/late.png'))
      await pending
    })

    expect(result.current.characters).toEqual([])
    expect(result.current.characterUploading).toBe(false)
  })
})
