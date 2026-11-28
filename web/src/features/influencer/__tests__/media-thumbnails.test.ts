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
  captureVideoPoster,
  generateImageThumbnail,
  getUploadThumbnails,
  putUploadThumbnail,
} from '../lib/media-thumbnails'

const thumbnail = {
  blob: new Blob(['thumb'], { type: 'image/webp' }),
  width: 96,
  height: 96,
}

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('local media thumbnails', () => {
  it('resolves without thumbnails when IndexedDB is unavailable', async () => {
    await expect(
      getUploadThumbnails(7, ['https://files.example/a.png'])
    ).resolves.toEqual(new Map())
  })

  it('treats a missing IndexedDB as a non-blocking write failure', async () => {
    await expect(
      putUploadThumbnail(7, 'https://files.example/a.png', thumbnail)
    ).resolves.toBeUndefined()
  })

  it('does not invent a derivative when the browser cannot decode the image', async () => {
    await expect(
      generateImageThumbnail(
        new File(['not an image'], 'a.png', { type: 'image/png' })
      )
    ).resolves.toBeNull()
  })

  it('resolves null cleanly when a video poster timeout fires after window teardown', async () => {
    vi.useFakeTimers()
    const createObjectURL = vi.fn(() => 'blob:poster-teardown')
    const revokeObjectURL = vi.fn()
    vi.stubGlobal(
      'URL',
      class extends URL {
        static createObjectURL = createObjectURL
        static revokeObjectURL = revokeObjectURL
      }
    )
    const create = document.createElement.bind(document)
    vi.spyOn(document, 'createElement').mockImplementation((tag, options) => {
      const element = create(tag, options)
      if (tag === 'video') {
        ;(element as HTMLVideoElement).load = vi.fn()
      }
      return element
    })

    const posterPromise = captureVideoPoster(
      new File(['clip'], 'clip.mp4', { type: 'video/mp4' })
    )
    vi.stubGlobal('window', undefined)
    await vi.advanceTimersByTimeAsync(3_000)

    await expect(posterPromise).resolves.toBeNull()
    expect(revokeObjectURL).toHaveBeenCalledOnce()
  })

  it('aborts video poster capture immediately when signal aborts', async () => {
    vi.useFakeTimers()
    const createObjectURL = vi.fn(() => 'blob:poster-abort')
    const revokeObjectURL = vi.fn()
    vi.stubGlobal(
      'URL',
      class extends URL {
        static createObjectURL = createObjectURL
        static revokeObjectURL = revokeObjectURL
      }
    )
    const create = document.createElement.bind(document)
    vi.spyOn(document, 'createElement').mockImplementation((tag, options) => {
      const element = create(tag, options)
      if (tag === 'video') {
        ;(element as HTMLVideoElement).load = vi.fn()
      }
      return element
    })

    const controller = new AbortController()
    const posterPromise = captureVideoPoster(
      new File(['clip'], 'clip.mp4', { type: 'video/mp4' }),
      controller.signal
    )
    controller.abort()

    await expect(posterPromise).resolves.toBeNull()
    expect(revokeObjectURL).toHaveBeenCalledOnce()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('skips video creation when signal is already aborted', async () => {
    const createObjectURL = vi.fn(() => 'blob:poster-preaborted')
    vi.stubGlobal(
      'URL',
      class extends URL {
        static createObjectURL = createObjectURL
        static revokeObjectURL = vi.fn()
      }
    )

    const controller = new AbortController()
    controller.abort()

    await expect(
      captureVideoPoster(
        new File(['clip'], 'clip.mp4', { type: 'video/mp4' }),
        controller.signal
      )
    ).resolves.toBeNull()
    expect(createObjectURL).not.toHaveBeenCalled()
  })
})
