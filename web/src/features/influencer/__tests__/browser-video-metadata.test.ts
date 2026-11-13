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

import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  createVideoDataSource,
  normalizeVideoMetadata,
  VIDEO_READ_BUDGET,
  VIDEO_UNSUPPORTED_MESSAGES,
} from '../lib/browser-video-metadata-reader'

function result(video: Record<string, unknown> = {}) {
  return {
    media: {
      track: [
        {
          '@type': 'General',
          Format: 'MPEG-4',
          CodecID: 'isom',
          Duration: 7.04,
        },
        {
          '@type': 'Video',
          Format: 'AVC',
          CodecID: 'avc1',
          Duration: 7,
          Width: 1080,
          Height: 1920,
          FrameRate: 30,
          PixelAspectRatio: 1,
          ...video,
        },
        { '@type': 'Audio', Format: 'AAC' },
      ],
    },
  }
}

afterEach(() => vi.unstubAllGlobals())

describe('browser video metadata normalization', () => {
  it('preserves fractional container and video durations separately', () => {
    expect(normalizeVideoMetadata(result(), 1234)).toEqual({
      duration: 7.04,
      video_duration: 7,
      width: 1080,
      height: 1920,
      frame_rate: 30,
      video_codec: 'h264',
      audio_codec: 'aac',
      container: 'mp4',
      size_bytes: 1234,
    })
  })

  it('preserves the reported AAC-tail container and picture durations without rounding', () => {
    const media = result({
      Duration: 7.041667,
      Width: 480,
      Height: 854,
      FrameRate: 24,
    })
    media.media.track[0].Duration = 7.072
    expect(normalizeVideoMetadata(media, 6650581)).toEqual({
      duration: 7.072,
      video_duration: 7.041667,
      width: 480,
      height: 854,
      frame_rate: 24,
      video_codec: 'h264',
      audio_codec: 'aac',
      container: 'mp4',
      size_bytes: 6650581,
    })
  })

  it('uses actual QuickTime signature and codec instead of the filename', () => {
    const media = result({ Format: 'HEVC', CodecID: 'hvc1' })
    media.media.track[0] = {
      '@type': 'General',
      Format: 'MPEG-4',
      CodecID: 'qt  ',
      Duration: 7,
    }
    expect(normalizeVideoMetadata(media, 100)).toMatchObject({
      container: 'mov',
      video_codec: 'hevc',
    })
  })

  it('follows the displayed direction of a quarter-turn video', () => {
    expect(
      normalizeVideoMetadata(
        result({ Width: 1920, Height: 1080, Rotation: 90 }),
        100
      )
    ).toMatchObject({ width: 1080, height: 1920 })
  })

  it('rejects non-square pixels rather than misreporting the displayed aspect', () => {
    expect(() =>
      normalizeVideoMetadata(result({ PixelAspectRatio: 1.5 }), 100)
    ).toThrow(/pixel/i)
  })

  it('rejects missing frame rate rather than inventing a default', () => {
    expect(() =>
      normalizeVideoMetadata(result({ FrameRate: undefined }), 100)
    ).toThrow()
  })

  it('rejects multiple video tracks rather than silently choosing the first', () => {
    const media = result()
    media.media.track.push({ ...media.media.track[1] })
    expect(() => normalizeVideoMetadata(media, 100)).toThrow(/track/i)
  })

  it('rejects inconsistent track counts instead of accepting partial parser output', () => {
    const media = result()
    Object.assign(media.media.track[0], { VideoCount: 2 })
    expect(() => normalizeVideoMetadata(media, 100)).toThrow(/track/i)
  })
})

describe('bounded browser video data source', () => {
  it('reads only the requested local slice and performs no network request', async () => {
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    const slice = vi.fn(() => ({
      arrayBuffer: async () => new Uint8Array([1, 2]).buffer,
    }))
    const file = { size: 100, slice } as unknown as File
    const source = await createVideoDataSource(file)
    expect(await source.readChunk(2, 8)).toEqual(new Uint8Array([1, 2]))
    expect(slice).toHaveBeenCalledWith(8, 10)
    expect(fetchMock).not.toHaveBeenCalled()
  })

  it('rejects excessive parser reads before allocating a large buffer', async () => {
    const file = { size: 200 * 1024 * 1024, slice: vi.fn() } as unknown as File
    const source = await createVideoDataSource(file)
    await expect(source.readChunk(VIDEO_READ_BUDGET + 1, 0)).rejects.toThrow()
    expect(file.slice).not.toHaveBeenCalled()
  })

  it('rejects a remote server ignoring Range without downloading its body', async () => {
    const cancel = vi.fn()
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        status: 200,
        body: { cancel },
        headers: new Headers(),
      }))
    )
    await expect(
      createVideoDataSource('https://assets.example/video.mp4')
    ).rejects.toThrow(/range/i)
    expect(cancel).toHaveBeenCalled()
  })

  it('reads a remote video in one bounded request when it fits the read budget', async () => {
    const fetchMock = vi.fn(
      async (_input: string, _init?: RequestInit) =>
        new Response(new Uint8Array([1, 2, 3, 4]), {
          status: 200,
          headers: { 'Content-Length': '4' },
        })
    )
    vi.stubGlobal('fetch', fetchMock)
    const source = await createVideoDataSource(
      'https://assets.example/video.mp4'
    )
    expect(fetchMock).toHaveBeenCalledOnce()
    expect(fetchMock.mock.calls[0]?.[1]).not.toHaveProperty('headers')
    expect(source.size).toBe(4)
    expect(await source.readChunk(2, 1)).toEqual(new Uint8Array([2, 3]))
  })

  it('falls back to bounded ranges when the remote object exceeds the read budget', async () => {
    const size = VIDEO_READ_BUDGET + 1
    let rangeCalls = 0
    const fetchMock = vi.fn(async (_input: string, init?: RequestInit) => {
      if (!init?.headers) {
        return new Response(null, {
          status: 200,
          headers: { 'Content-Length': String(size) },
        })
      }
      rangeCalls += 1
      return rangeCalls === 1
        ? new Response(new Uint8Array([0]), {
            status: 206,
            headers: { 'Content-Range': `bytes 0-0/${size}`, ETag: 'same' },
          })
        : new Response(new Uint8Array([7, 8]), {
            status: 206,
            headers: { 'Content-Range': `bytes 1-2/${size}`, ETag: 'same' },
          })
    })
    vi.stubGlobal('fetch', fetchMock)
    const source = await createVideoDataSource(
      'https://assets.example/video.mp4'
    )
    expect(source.size).toBe(size)
    expect(await source.readChunk(2, 1)).toEqual(new Uint8Array([7, 8]))
    expect(fetchMock).toHaveBeenCalledTimes(3)
  })

  it('rejects a remote object above the upload ceiling before buffering it', async () => {
    const cancel = vi.fn()
    vi.stubGlobal(
      'fetch',
      vi.fn(async () => ({
        status: 200,
        body: { cancel },
        headers: new Headers({
          'Content-Length': String(200 * 1024 * 1024 + 1),
        }),
      }))
    )
    await expect(
      createVideoDataSource('https://assets.example/video.mp4')
    ).rejects.toThrow(/exceeds/i)
    expect(cancel).toHaveBeenCalled()
  })

  it('rejects changed remote content between ranges', async () => {
    const size = VIDEO_READ_BUDGET + 1
    let rangeCalls = 0
    vi.stubGlobal(
      'fetch',
      vi.fn(async (_input: string, init?: RequestInit) => {
        if (!init?.headers) {
          return new Response(null, {
            status: 200,
            headers: { 'Content-Length': String(size) },
          })
        }
        rangeCalls += 1
        const range = rangeCalls === 1 ? 'bytes 0-0/100' : 'bytes 5-6/100'
        return new Response(new Uint8Array(rangeCalls === 1 ? [0] : [1, 2]), {
          status: 206,
          headers: {
            'Content-Range': range,
            ETag: rangeCalls === 1 ? 'first' : 'changed',
          },
        })
      })
    )
    const source = await createVideoDataSource(
      'https://assets.example/video.mp4'
    )
    await expect(source.readChunk(2, 5)).rejects.toThrow(/changed/i)
  })

  it('stops an oversized remote range stream instead of buffering it all', async () => {
    const cancel = vi.fn()
    const size = VIDEO_READ_BUDGET + 1
    vi.stubGlobal(
      'fetch',
      vi.fn(async (_input: string, init?: RequestInit) => {
        if (!init?.headers) {
          return new Response(null, {
            status: 200,
            headers: { 'Content-Length': String(size) },
          })
        }
        return new Response(
          new ReadableStream({
            start(controller) {
              controller.enqueue(new Uint8Array([1, 2]))
            },
            cancel,
          }),
          { status: 206, headers: { 'Content-Range': 'bytes 0-0/100' } }
        )
      })
    )
    await expect(
      createVideoDataSource('https://assets.example/video.mp4')
    ).rejects.toThrow()
    expect(cancel).toHaveBeenCalled()
  })
})

describe('browser inspection lifecycle', () => {
  class FakeWorker {
    static instances: FakeWorker[] = []
    listeners: Record<string, (event: { data: unknown }) => void> = {}
    addEventListener(
      type: string,
      listener: (event: { data: unknown }) => void
    ) {
      this.listeners[type] = listener
    }
    postMessage = vi.fn()
    terminate = vi.fn()
    constructor() {
      FakeWorker.instances.push(this)
    }
    finish(metadata = normalizeVideoMetadata(result(), 100)) {
      const request = this.postMessage.mock.lastCall?.[0] as { id: number }
      this.listeners.message?.({ data: { id: request.id, metadata } })
    }
    fail(message: string) {
      const request = this.postMessage.mock.lastCall?.[0] as { id: number }
      this.listeners.message?.({
        data: { id: request.id, code: 'unsupported', message },
      })
    }
  }

  async function reader() {
    vi.resetModules()
    vi.useFakeTimers()
    FakeWorker.instances = []
    vi.stubGlobal('Worker', FakeWorker)
    return (await import('../lib/browser-video-metadata'))
      .readBrowserVideoMetadata
  }

  afterEach(() => {
    vi.clearAllTimers()
    vi.useRealTimers()
  })

  it('serializes requests through one worker and releases it when idle', async () => {
    const read = await reader()
    const first = read(new File(['one'], 'one.mp4'))
    const second = read(new File(['two'], 'two.mp4'))
    const worker = FakeWorker.instances[0]
    expect(worker.postMessage).toHaveBeenCalledTimes(1)
    worker.finish()
    await first
    expect(worker.postMessage).toHaveBeenCalledTimes(2)
    worker.finish()
    await second
    await vi.advanceTimersByTimeAsync(30_000)
    expect(worker.terminate).toHaveBeenCalledOnce()
  })

  it('resolves a same-origin asset path before handing it to the worker', async () => {
    const read = await reader()
    const promise = read('/assets/video.mp4')
    const worker = FakeWorker.instances[0]
    expect(worker.postMessage.mock.lastCall?.[0]).toMatchObject({
      source: new URL('/assets/video.mp4', document.baseURI).href,
    })
    worker.finish()
    await promise
  })

  it('terminates an active parse on cancellation and starts queued work in a fresh worker', async () => {
    const read = await reader()
    const abort = new AbortController()
    const first = read(new File(['one'], 'one.mp4'), abort.signal)
    const rejected = expect(first).rejects.toMatchObject({ name: 'AbortError' })
    const second = read(new File(['two'], 'two.mp4'))
    abort.abort()
    await rejected
    expect(FakeWorker.instances[0].terminate).toHaveBeenCalledOnce()
    expect(FakeWorker.instances).toHaveLength(2)
    FakeWorker.instances[1].finish()
    await expect(second).resolves.toMatchObject({ duration: 7.04 })
  })

  it('terminates a stalled worker on timeout and exposes only a stable product message', async () => {
    const read = await reader()
    const promise = read(new File(['one'], 'one.mp4'))
    const rejected = expect(promise).rejects.toMatchObject({
      code: 'timeout',
      message:
        'Could not read this video in your browser. Choose another MP4 or MOV file.',
    })
    await vi.advanceTimersByTimeAsync(20_000)
    await rejected
    expect(FakeWorker.instances[0].terminate).toHaveBeenCalledOnce()
  })

  it('gives a remote URL a longer transfer window than a local file', async () => {
    const read = await reader()
    const promise = read('https://assets.example/video.mp4')
    const rejected = expect(promise).rejects.toMatchObject({ code: 'timeout' })
    await vi.advanceTimersByTimeAsync(20_000)
    expect(FakeWorker.instances[0].terminate).not.toHaveBeenCalled()
    await vi.advanceTimersByTimeAsync(25_000)
    await rejected
    expect(FakeWorker.instances[0].terminate).toHaveBeenCalledOnce()
  })

  it('reuses successful metadata only for the identical immutable File object', async () => {
    const read = await reader()
    const file = new File(['one'], 'one.mp4')
    const first = read(file)
    FakeWorker.instances[0].finish()
    const metadata = await first
    metadata.width = 1
    await expect(read(file)).resolves.toMatchObject({ width: 1080 })
    expect(FakeWorker.instances[0].postMessage).toHaveBeenCalledOnce()
  })

  it('preserves an actionable unsupported-specification message for the page toast', async () => {
    const read = await reader()
    const promise = read(new File(['one'], 'one.mp4'))
    FakeWorker.instances[0].fail(VIDEO_UNSUPPORTED_MESSAGES.pixelAspect)
    await expect(promise).rejects.toMatchObject({
      message: VIDEO_UNSUPPORTED_MESSAGES.pixelAspect,
    })
  })

  it('does not expose an unexpected parser error or private URL to the page toast', async () => {
    const read = await reader()
    const promise = read(new File(['one'], 'one.mp4'))
    FakeWorker.instances[0].fail(
      'Parser failed at https://private.example/secret'
    )
    await expect(promise).rejects.toMatchObject({
      message:
        'Could not read this video in your browser. Choose another MP4 or MOV file.',
    })
  })
})
