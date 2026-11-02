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

import { act, cleanup, renderHook } from '@testing-library/react'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { inspectMotionVideo } from '../api'
import { useMotion } from '../hooks/use-motion'
import {
  readBrowserVideoMetadata,
  type BrowserVideoMetadata,
} from '../lib/browser-video-metadata'
import {
  checkVideoFileBasic,
  inspectVideoSource,
  motionAspectRatio,
  validateMotionVideo,
  validateMotionVideoSource,
  validateMotionMetadata,
} from '../lib/motion'
import { buildMotionSubmitPayload } from '../lib/submit-payload'

vi.mock('../api', () => ({
  inspectMotionVideo: vi.fn(),
  fetchTaskRecord: vi.fn(),
  uploadReferenceAsset: vi.fn(async () => ({
    url: 'https://files.example/clip.mp4',
    expiresAt: Date.now() + 72 * 60 * 60 * 1000,
  })),
}))
vi.mock('../lib/browser-video-metadata', () => ({
  readBrowserVideoMetadata: vi.fn(),
}))
vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))

let videos: HTMLVideoElement[]
let revoke: ReturnType<typeof vi.fn<(url: string) => void>>

beforeEach(() => {
  vi.mocked(toast.error).mockClear()
  localStorage.clear()
  vi.mocked(inspectMotionVideo).mockReset()
  vi.mocked(readBrowserVideoMetadata)
    .mockReset()
    .mockResolvedValue(browserMetadata)
  videos = []
  const create = document.createElement.bind(document)
  vi.spyOn(document, 'createElement').mockImplementation((tag, options) => {
    const element = create(tag, options)
    if (tag === 'video') {
      const video = element as HTMLVideoElement
      video.load = vi.fn()
      videos.push(video)
    }
    return element
  })
  revoke = vi.fn()
  vi.stubGlobal(
    'URL',
    class extends URL {
      static createObjectURL = vi.fn(() => 'blob:reference')
      static revokeObjectURL = revoke
    }
  )
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

function loaded(
  video: HTMLVideoElement,
  duration: number,
  width = 720,
  height = 1280
) {
  Object.defineProperties(video, {
    duration: { configurable: true, value: duration },
    videoWidth: { configurable: true, value: width },
    videoHeight: { configurable: true, value: height },
  })
  video.dispatchEvent(new Event('loadedmetadata'))
}

const file = () => new File(['clip'], 'clip.mp4', { type: 'video/mp4' })

describe('motion video metadata validation', () => {
  it.each([1.99, 30.01])(
    'rejects raw %s seconds before integer rounding',
    async (duration) => {
      const pending = validateMotionVideo(file())
      loaded(videos[0], duration)
      expect(await pending).toEqual({ ok: false, reason: 'duration' })
    }
  )

  it.each([2, 30])(
    'accepts exact %s seconds and retains dimensions and ratio',
    async (duration) => {
      const pending = validateMotionVideo(file())
      loaded(videos[0], duration)
      expect(await pending).toEqual({
        ok: true,
        rawDurationSec: duration,
        durationSec: duration,
        width: 720,
        height: 1280,
        aspectRatio: '9:16',
      })
      expect(revoke).toHaveBeenCalledWith('blob:reference')
      expect(videos[0].hasAttribute('src')).toBe(false)
      expect(videos[0].load).toHaveBeenCalledOnce()
      expect(videos[0].onloadedmetadata).toBeNull()
    }
  )

  it("reports the clip's own standard ratio and leaves non-standard clips usable", () => {
    expect(motionAspectRatio(853, 480)).toBe('16:9')
    expect(motionAspectRatio(480, 853)).toBe('9:16')
    expect(motionAspectRatio(768, 1024)).toBe('3:4')
    expect(motionAspectRatio(720, 720)).toBeNull()
    expect(motionAspectRatio(700, 1280)).toBeNull()
    expect(motionAspectRatio(600, 480)).toBeNull()
  })

  it('keeps non-standard clip dimensions as metadata instead of a rejection', async () => {
    const pending = validateMotionVideo(file())
    loaded(videos[0], 10, 720, 720)
    expect(await pending).toEqual({
      ok: true,
      rawDurationSec: 10,
      durationSec: 10,
      width: 720,
      height: 720,
      aspectRatio: null,
    })
  })

  it('rejects decode failure without a fake duration and releases the file URL', async () => {
    const pending = validateMotionVideo(file())
    videos[0].dispatchEvent(new Event('error'))
    expect(await pending).toEqual({ ok: false, reason: 'corrupt' })
    expect(revoke).toHaveBeenCalledOnce()
    expect(videos[0].hasAttribute('src')).toBe(false)
  })

  it('rejects timeout and releases all metadata resources', async () => {
    vi.useFakeTimers()
    const pending = validateMotionVideo(file())
    await vi.advanceTimersByTimeAsync(5000)
    expect(await pending).toEqual({ ok: false, reason: 'corrupt' })
    expect(revoke).toHaveBeenCalledOnce()
    expect(videos[0].onerror).toBeNull()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('accepts remote preset metadata arriving after five seconds', async () => {
    vi.useFakeTimers()
    const pending = validateMotionVideoSource('https://cdn.example/preset.mp4')
    await vi.advanceTimersByTimeAsync(6000)
    loaded(videos[0], 9.042, 600, 338)
    expect(await pending).toMatchObject({
      ok: true,
      rawDurationSec: 9.042,
      durationSec: 9,
      width: 600,
      height: 338,
    })
    expect(videos[0].hasAttribute('src')).toBe(false)
    expect(vi.getTimerCount()).toBe(0)
  })

  it('bounds remote metadata waiting and ignores success after timeout', async () => {
    vi.useFakeTimers()
    const pending = validateMotionVideoSource('https://cdn.example/stalled.mp4')
    await vi.advanceTimersByTimeAsync(30000)
    expect(await pending).toEqual({ ok: false, reason: 'corrupt' })
    loaded(videos[0], 9)
    expect(await pending).toEqual({ ok: false, reason: 'corrupt' })
    expect(videos[0].hasAttribute('src')).toBe(false)
    expect(revoke).not.toHaveBeenCalled()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('aborts remote reading and releases the media element without revoking a public URL', async () => {
    const controller = new AbortController()
    const pending = inspectVideoSource(
      'https://cdn.example/clip.mp4',
      controller.signal
    )
    const rejected = expect(pending).rejects.toThrow('cancelled')
    controller.abort()
    await rejected
    expect(videos[0].load).toHaveBeenCalledOnce()
    expect(videos[0].hasAttribute('src')).toBe(false)
    expect(revoke).not.toHaveBeenCalled()
  })
})

describe('motion reference limits', () => {
  it.each([2, 3.99, 4, 30])(
    'accepts a %s-second ordinary reference',
    (duration) => {
      expect(
        validateMotionMetadata({ duration, width: 1200, height: 960 }, 'motion')
          .ok
      ).toBe(true)
    }
  )

  it.each([1.99, 3.99, 30.01])(
    'rejects a %s-second edited original before rounding',
    (duration) => {
      expect(
        validateMotionMetadata(
          { duration, width: 1200, height: 960 },
          'replace'
        )
      ).toEqual({ ok: false, reason: 'duration' })
    }
  )

  it.each([4, 30])(
    'accepts a %s-second 5:4 edited original without a standard-ratio label',
    (duration) => {
      expect(
        validateMotionMetadata(
          { duration, width: 1200, height: 960 },
          'replace'
        )
      ).toMatchObject({ ok: true, rawDurationSec: duration, aspectRatio: null })
    }
  )

  it('accepts the exact 200 MiB ceiling and rejects one byte above it', () => {
    expect(
      checkVideoFileBasic({
        name: 'large.mov',
        type: 'video/quicktime',
        size: 200 * 1024 * 1024,
      })
    ).toEqual({ ok: true })
    expect(
      checkVideoFileBasic({
        name: 'large.mov',
        type: 'video/quicktime',
        size: 200 * 1024 * 1024 + 1,
      })
    ).toEqual({ ok: false, reason: 'size' })
  })
})

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>((done) => {
    resolve = done
  })
  return { promise, resolve }
}

const browserMetadata: BrowserVideoMetadata = {
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
const checkedMetadata = {
  rawDurationSec: 8,
  durationSec: 8,
  width: 720,
  height: 1280,
  aspectRatio: '9:16',
  version: 1,
  browserMetadata,
}

describe('motion form metadata gates', () => {
  it('choosing and clearing a visual style preserves the pending browser read', async () => {
    const read = deferred<BrowserVideoMetadata>()
    vi.mocked(readBrowserVideoMetadata).mockReturnValueOnce(read.promise)
    const { result } = renderHook(() => useMotion())
    let pending!: Promise<boolean>
    act(() => {
      pending = result.current.setRemoteVideo(
        'https://cdn.example/pending.mp4',
        'Pending'
      )
    })
    await act(async () => {
      await result.current.applyStyle({
        id: 'visual',
        nameKey: 'Visual',
        summaryKey: 'Only text',
        prompt: 'Use a consistent treatment.',
        mode: 'motion',
      })
      await result.current.applyStyle(null)
    })
    expect(result.current.videoMetadataLoading).toBe(true)
    await act(async () => {
      read.resolve(browserMetadata)
      await pending
    })
    expect(result.current.video).toMatchObject({
      url: 'https://cdn.example/pending.mp4',
      metadata: checkedMetadata,
    })
  })
  it('derives both generation parameters from the reference with no setters', async () => {
    const { result } = renderHook(() => useMotion())
    await act(async () => {
      await result.current.pickCharacter(
        new File(['image'], 'face.png', { type: 'image/png' })
      )
      await result.current.setRemoteVideo(
        'https://cdn.example/clip.mp4',
        'Clip'
      )
    })
    expect(result.current.canSubmit).toBe(true)
    expect(result.current.durationSec).toBe(8)
    expect(result.current.outputAspect).toBe('9:16')
    expect(result.current).not.toHaveProperty('setDurationSec')
    expect(result.current).not.toHaveProperty('setOutputAspect')
    expect(
      buildMotionSubmitPayload({ ...result.current, prompt: '' })
    ).toMatchObject({
      duration_sec: 8,
      aspect_ratio: '9:16',
      video_metadata: browserMetadata,
    })
    expect(inspectMotionVideo).not.toHaveBeenCalled()
  })
  it('preserves a valid original when a subsequent reference cannot follow its duration or framing', async () => {
    const { result } = renderHook(() => useMotion())
    await act(async () => {
      await result.current.setRemoteVideo(
        'https://cdn.example/valid.mp4',
        'Valid'
      )
    })
    const selected = result.current.video
    for (const raw of [
      { ...browserMetadata, duration: 2.5, video_duration: 2.5 },
      { ...browserMetadata, width: 1200, height: 960 },
    ]) {
      vi.mocked(readBrowserVideoMetadata).mockResolvedValueOnce(raw)
      await act(async () => {
        await result.current.setRemoteVideo(
          'https://cdn.example/other.mp4',
          'Other'
        )
      })
      expect(result.current.video).toBe(selected)
    }
    expect(toast.error).toHaveBeenCalledTimes(2)
  })
  it('retains the reference and warns when a new resolution cannot preserve 3:4', async () => {
    vi.mocked(readBrowserVideoMetadata).mockResolvedValueOnce({
      ...browserMetadata,
      width: 960,
      height: 1280,
    })
    const { result } = renderHook(() => useMotion())
    act(() => result.current.setResolution('480p'))
    await act(async () => {
      await result.current.setRemoteVideo(
        'https://cdn.example/portrait.mp4',
        'Portrait'
      )
    })
    const selected = result.current.video
    act(() => result.current.setResolution('720p'))
    expect(result.current.resolution).toBe('480p')
    expect(result.current.video).toBe(selected)
    expect(result.current.outputAspect).toBe('3:4')
    expect(toast.error).toHaveBeenCalledOnce()
  })
  it('restores current stored metadata locally without reading or replaying independent legacy parameters', async () => {
    const { result } = renderHook(() => useMotion())
    await act(async () => {
      await result.current.restoreTask({
        mode: 'motion',
        prompt: 'again',
        subject_image_url: '/face.png',
        video_url: 'https://files.example/original.mp4',
        duration_sec: 12,
        aspect_ratio: '16:9',
        resolution: '480p',
        video_metadata: browserMetadata,
      })
    })
    expect(result.current.durationSec).toBe(8)
    expect(result.current.outputAspect).toBe('9:16')
    expect(readBrowserVideoMetadata).not.toHaveBeenCalled()
  })
  it('retains a replacement preset until its checked source is removed', async () => {
    const { result } = renderHook(() => useMotion())
    act(() => result.current.setMode('replace'))
    await act(async () => {
      await result.current.applyStyle({
        id: 'replacement',
        nameKey: 'Replacement',
        prompt: 'Keep the scene.',
        mode: 'replace',
        videoUrl: 'https://cdn.example/replacement.mp4',
        durationSec: 8,
      })
    })
    expect(result.current.video).toMatchObject({
      url: 'https://cdn.example/replacement.mp4',
    })
    act(() => result.current.clearVideo())
    expect(result.current.video).toBeNull()
    expect(result.current.style).toBeNull()
  })
  it('aborts a pending read on a mode switch without leaking it into either mode', async () => {
    const { result } = renderHook(() => useMotion())
    await act(async () => {
      await result.current.setRemoteVideo(
        'https://cdn.example/valid.mp4',
        'Valid'
      )
    })
    const selected = result.current.video
    const read = deferred<BrowserVideoMetadata>()
    vi.mocked(readBrowserVideoMetadata).mockReturnValueOnce(read.promise)
    let pending!: Promise<boolean>
    act(() => {
      pending = result.current.setRemoteVideo(
        'https://cdn.example/pending.mp4',
        'Pending'
      )
    })
    const signal = vi.mocked(readBrowserVideoMetadata).mock.calls.at(-1)?.[1]
    act(() => result.current.setMode('replace'))
    expect(signal?.aborted).toBe(true)
    await act(async () => {
      read.resolve(browserMetadata)
      await pending
    })
    // Each mode owns its own selection: replace started empty and stays empty,
    // while the motion selection is restored when switching back.
    expect(result.current.video).toBeNull()
    act(() => result.current.setMode('motion'))
    expect(result.current.video).toBe(selected)
  })

  it('keeps the selected video, images and prompt per mode', async () => {
    const { result } = renderHook(() => useMotion())
    await act(async () => {
      await result.current.setRemoteVideo(
        'https://cdn.example/motion-only.mp4',
        'Motion only'
      )
    })
    act(() => {
      result.current.addCharacter({ url: '/motion-face.png', fileName: 'face.png' })
      result.current.setPrompt('motion draft')
    })
    expect(result.current.characters).toHaveLength(1)

    act(() => result.current.setMode('replace'))
    expect(result.current.video).toBeNull()
    expect(result.current.characters).toHaveLength(0)
    expect(result.current.prompt).toBe('')

    act(() => result.current.setMode('motion'))
    expect(result.current.video).toMatchObject({
      url: 'https://cdn.example/motion-only.mp4',
    })
    expect(result.current.characters).toHaveLength(1)
    expect(result.current.prompt).toBe('motion draft')
  })
})

describe('browser-only reference eligibility', () => {
  const raw = {
    duration: 7.041,
    video_duration: 7,
    width: 720,
    height: 1280,
    frame_rate: 24,
    video_codec: 'h264',
    audio_codec: 'aac',
    container: 'mp4',
    size_bytes: 1024,
  }
  it('retains the browser result and rejects declarations more than two frames apart', async () => {
    const { validateBrowserMotionMetadata, validateMotionOutput } =
      await import('../lib/motion')
    const checked = validateBrowserMotionMetadata(raw, 'motion')
    expect(checked).toMatchObject({
      ok: true,
      rawDurationSec: 7.041,
      durationSec: 7,
      browserMetadata: raw,
      version: 1,
    })
    if (!checked.ok) throw new Error('Expected a valid reference')
    expect(validateMotionOutput(checked, 'motion', '480p')).toMatchObject({
      ok: true,
    })
    const decimal = validateBrowserMotionMetadata(
      { ...raw, duration: 7.2 },
      'motion'
    )
    if (!decimal.ok) throw new Error('Decimal duration remains a valid input')
    expect(validateMotionOutput(decimal, 'motion', '480p')).toEqual({
      ok: false,
      reason: 'output-duration',
    })
  })
  it.each([
    { mode: 'motion' as const, videoDuration: 7.2 },
    { mode: 'motion' as const, videoDuration: 8 },
    { mode: 'replace' as const, videoDuration: 7.2 },
    { mode: 'replace' as const, videoDuration: 8 },
  ])(
    'rejects $mode when a 7-second container hides a $videoDuration-second video track',
    async ({ mode, videoDuration }) => {
      const { validateBrowserMotionMetadata, validateMotionOutput } =
        await import('../lib/motion')
      const checked = validateBrowserMotionMetadata(
        { ...raw, duration: 7, video_duration: videoDuration },
        mode
      )
      if (!checked.ok) {
        throw new Error('The original remains a valid reference input')
      }
      expect(validateMotionOutput(checked, mode, '480p')).toEqual({
        ok: false,
        reason: 'output-duration',
      })
    }
  )
  it('rejects a video track below four seconds instead of extending it to the container duration', async () => {
    const { validateBrowserMotionMetadata, validateMotionOutput } =
      await import('../lib/motion')
    const checked = validateBrowserMotionMetadata(
      { ...raw, duration: 4, video_duration: 3.999 },
      'motion'
    )
    if (!checked.ok) {
      throw new Error('The original remains within motion input bounds')
    }
    expect(validateMotionOutput(checked, 'motion', '480p')).toEqual({
      ok: false,
      reason: 'output-duration',
    })
  })
  it.each(['motion', 'replace'] as const)(
    'accepts $mode when both container and video track are within one frame of the same output second',
    async (mode) => {
      const { validateBrowserMotionMetadata, validateMotionOutput } =
        await import('../lib/motion')
      const checked = validateBrowserMotionMetadata(
        { ...raw, duration: 7, video_duration: 7.041 },
        mode
      )
      if (!checked.ok) {
        throw new Error('The original is a valid reference input')
      }
      expect(validateMotionOutput(checked, mode, '480p')).toMatchObject({
        ok: true,
        durationSec: 7,
      })
    }
  )
  it.each([
    {
      mode: 'motion' as const,
      duration: 7.072,
      videoDuration: 7.041667,
      seconds: 7,
    },
    {
      mode: 'replace' as const,
      duration: 7.072,
      videoDuration: 7.041667,
      seconds: 7,
    },
    {
      mode: 'motion' as const,
      duration: 4.064,
      videoDuration: 4.041667,
      seconds: 4,
    },
    {
      mode: 'replace' as const,
      duration: 4.064,
      videoDuration: 4.041667,
      seconds: 4,
    },
  ])(
    'accepts $mode with $duration seconds of container and $videoDuration seconds of picture without rewriting either',
    async ({ mode, duration, videoDuration, seconds }) => {
      const { validateBrowserMotionMetadata, validateMotionOutput } =
        await import('../lib/motion')
      const candidate = Object.freeze({
        ...raw,
        duration,
        video_duration: videoDuration,
        width: 480,
        height: 854,
        size_bytes: 6650581,
      })
      const checked = validateBrowserMotionMetadata(candidate, mode)
      if (!checked.ok) throw new Error('Expected a valid reference input')
      const output = validateMotionOutput(checked, mode, '480p')
      expect(output).toMatchObject({
        ok: true,
        rawDurationSec: duration,
        durationSec: seconds,
        aspectRatio: '9:16',
      })
      if (!output.ok) throw new Error('Expected a supported audio tail')
      expect(output.browserMetadata).toBe(candidate)
      expect(candidate).toMatchObject({
        duration,
        video_duration: videoDuration,
      })
    }
  )
  it.each([
    { frameRate: 24, duration: 7.084333, videoDuration: 7.041667 },
    { frameRate: 60, duration: 7.034333, videoDuration: 7.016667 },
  ])(
    'accepts a positive audio tail within the two-frame guard at $frameRate fps',
    async ({ frameRate, duration, videoDuration }) => {
      const { validateBrowserMotionMetadata, validateMotionOutput } =
        await import('../lib/motion')
      const checked = validateBrowserMotionMetadata({
        ...raw,
        duration,
        video_duration: videoDuration,
        frame_rate: frameRate,
      })
      if (!checked.ok) throw new Error('Expected a valid reference input')
      expect(validateMotionOutput(checked, 'motion', '480p')).toMatchObject({
        ok: true,
        durationSec: 7,
      })
    }
  )
  it.each([
    {
      name: 'reported AAC tail without audio',
      duration: 7.072,
      videoDuration: 7.041667,
      audioCodec: '',
      frameRate: 24,
      seconds: 7,
    },
    {
      name: 'consistent fractional pair',
      duration: 7.2,
      videoDuration: 7.2,
      audioCodec: 'aac',
      frameRate: 24,
      seconds: 7,
    },
    {
      name: 'tail just above one frame',
      duration: 7.084335,
      videoDuration: 7.041667,
      audioCodec: 'aac',
      frameRate: 24,
      seconds: 7,
    },
    {
      name: 'tail just above one frame at 60 fps',
      duration: 7.034335,
      videoDuration: 7.016667,
      audioCodec: 'aac',
      frameRate: 60,
      seconds: 7,
    },
    {
      name: 'reverse record discrepancy',
      duration: 6.927,
      videoDuration: 6.958333,
      audioCodec: 'aac',
      frameRate: 24,
      seconds: 7,
    },
  ])(
    'accepts $name duration=$duration picture=$videoDuration audio=$audioCodec fps=$frameRate',
    async ({ duration, videoDuration, audioCodec, frameRate, seconds }) => {
      const { validateBrowserMotionMetadata, validateMotionOutput } =
        await import('../lib/motion')
      const checked = validateBrowserMotionMetadata({
        ...raw,
        duration,
        video_duration: videoDuration,
        audio_codec: audioCodec,
        frame_rate: frameRate,
      })
      if (!checked.ok) throw new Error('Expected a valid reference input')
      expect(validateMotionOutput(checked, 'motion', '480p')).toMatchObject({
        ok: true,
        rawDurationSec: duration,
        durationSec: seconds,
      })
    }
  )
  it.each([
    { duration: 7.2, videoDuration: 7, audioCodec: 'aac', frameRate: 24 },
    { duration: 7.5, videoDuration: 7.2, audioCodec: 'aac', frameRate: 24 },
  ])(
    'rejects a container that disagrees with the picture beyond two frames duration=$duration picture=$videoDuration',
    async ({ duration, videoDuration, audioCodec, frameRate }) => {
      const { validateBrowserMotionMetadata, validateMotionOutput } =
        await import('../lib/motion')
      const checked = validateBrowserMotionMetadata({
        ...raw,
        duration,
        video_duration: videoDuration,
        audio_codec: audioCodec,
        frame_rate: frameRate,
      })
      if (!checked.ok) throw new Error('Expected a valid reference input')
      expect(validateMotionOutput(checked, 'motion', '480p')).toEqual({
        ok: false,
        reason: 'output-duration',
      })
    }
  )
  it.each(['motion', 'replace'] as const)(
    'accepts the reported fractional $mode reference and requests six seconds',
    async (mode) => {
      const { validateBrowserMotionMetadata, validateMotionOutput } =
        await import('../lib/motion')
      const candidate = Object.freeze({
        ...raw,
        duration: 5.717007,
        video_duration: 5.708333,
        width: 720,
        height: 1280,
        frame_rate: 24,
        audio_codec: 'aac',
        size_bytes: 2863940,
      })
      const checked = validateBrowserMotionMetadata(candidate, mode)
      if (!checked.ok) throw new Error('Expected a valid reference input')
      const output = validateMotionOutput(checked, mode, '480p')
      expect(output).toMatchObject({
        ok: true,
        rawDurationSec: 5.717007,
        durationSec: 6,
        aspectRatio: '9:16',
      })
      if (!output.ok) throw new Error('Expected the fractional reference')
      expect(output.browserMetadata).toBe(candidate)
    }
  )
  it.each([
    { frameRate: 24, duration: 4.5, videoDuration: 4.5, seconds: 5 },
    { frameRate: 24, duration: 29.6, videoDuration: 29.6, seconds: 30 },
  ])(
    'rounds a consistent $duration-second reference to $seconds seconds',
    async ({ frameRate, duration, videoDuration, seconds }) => {
      const { validateBrowserMotionMetadata, validateMotionOutput } =
        await import('../lib/motion')
      const checked = validateBrowserMotionMetadata({
        ...raw,
        duration,
        video_duration: videoDuration,
        frame_rate: frameRate,
      })
      if (!checked.ok) throw new Error('Expected a valid reference input')
      expect(validateMotionOutput(checked, 'motion', '480p')).toMatchObject({
        ok: true,
        durationSec: seconds,
      })
    }
  )
  it('rejects a 2.4-second motion picture even though it is a valid input', async () => {
    const { validateBrowserMotionMetadata, validateMotionOutput } =
      await import('../lib/motion')
    const checked = validateBrowserMotionMetadata(
      { ...raw, duration: 2.4, video_duration: 2.4 },
      'motion'
    )
    if (!checked.ok) throw new Error('Expected a valid motion input')
    expect(validateMotionOutput(checked, 'motion', '480p')).toEqual({
      ok: false,
      reason: 'output-duration',
    })
  })
  it.each([
    { duration: 30.4, videoDuration: 30.4 },
    { duration: 1.5, videoDuration: 1.5 },
  ])(
    'rejects an out-of-range raw duration before output checks container=$duration',
    async ({ duration, videoDuration }) => {
      const { validateBrowserMotionMetadata } = await import('../lib/motion')
      expect(
        validateBrowserMotionMetadata(
          { ...raw, duration, video_duration: videoDuration },
          'motion'
        )
      ).toEqual({ ok: false, reason: 'duration' })
    }
  )
  it.each(['', 'aac'])(
    'keeps the existing allowance when container and picture straddle the same second with audio=%s',
    async (audioCodec) => {
      const { validateBrowserMotionMetadata, validateMotionOutput } =
        await import('../lib/motion')
      const checked = validateBrowserMotionMetadata({
        ...raw,
        duration: 7.04,
        video_duration: 6.96,
        audio_codec: audioCodec,
      })
      if (!checked.ok) throw new Error('Expected a valid reference input')
      expect(validateMotionOutput(checked, 'motion', '480p')).toMatchObject({
        ok: true,
        durationSec: 7,
      })
    }
  )
  it.each([
    { duration: 3.999, videoDuration: 4 },
    { duration: 4, videoDuration: 3.999 },
    { duration: 30.001, videoDuration: 30 },
    { duration: 30, videoDuration: 30.001 },
  ])(
    'does not expand the output bounds for container=$duration picture=$videoDuration',
    async ({ duration, videoDuration }) => {
      const { validateBrowserMotionMetadata, validateMotionOutput } =
        await import('../lib/motion')
      for (const mode of ['motion', 'replace'] as const) {
        const checked = validateBrowserMotionMetadata(
          { ...raw, duration, video_duration: videoDuration },
          mode
        )
        const output = checked.ok
          ? validateMotionOutput(checked, mode, '480p')
          : checked
        expect(output.ok).toBe(false)
      }
    }
  )
  it('keeps valid continuous input framing separate from unavailable output framing', async () => {
    const { validateBrowserMotionMetadata, validateMotionOutput } =
      await import('../lib/motion')
    const checked = validateBrowserMotionMetadata(
      { ...raw, width: 1200, height: 960 },
      'motion'
    )
    expect(checked.ok).toBe(true)
    if (!checked.ok) throw new Error('Expected a valid input')
    expect(validateMotionOutput(checked, 'motion', '480p')).toEqual({
      ok: false,
      reason: 'output-aspect',
    })
  })
  it('accepts locally parsed PCM in MOV while retaining the MP4 audio restriction', async () => {
    const { validateBrowserMotionMetadata } = await import('../lib/motion')
    expect(
      validateBrowserMotionMetadata(
        { ...raw, container: 'mov', audio_codec: 'pcm' },
        'motion'
      ).ok
    ).toBe(true)
    expect(
      validateBrowserMotionMetadata(
        { ...raw, container: 'mp4', audio_codec: 'pcm' },
        'motion'
      )
    ).toEqual({ ok: false, reason: 'audio' })
  })
  it.each([
    [{ ...raw, frame_rate: 23 }, 'frame-rate'],
    [{ ...raw, width: 200 }, 'dimensions'],
    [{ ...raw, width: 300, height: 300 }, 'pixels'],
    [{ ...raw, video_codec: 'av1' }, 'codec'],
    [{ ...raw, audio_codec: 'opus' }, 'audio'],
  ])(
    'rejects an unsupported local media property before upload',
    async (candidate, reason) => {
      const { validateBrowserMotionMetadata } = await import('../lib/motion')
      expect(validateBrowserMotionMetadata(candidate, 'motion')).toEqual({
        ok: false,
        reason,
      })
    }
  )
})
