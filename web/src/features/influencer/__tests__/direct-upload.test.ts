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

import { api } from '@/lib/api'

import { uploadReferenceAsset } from '../api'

vi.mock('@/lib/api', () => ({ api: { post: vi.fn() } }))
afterEach(() => {
  vi.resetAllMocks()
  vi.unstubAllGlobals()
})
const ticket = {
  success: true,
  data: {
    file_id: 'file_x',
    upload_url:
      'https://account.r2.cloudflarestorage.com/files/a?signature=test',
    upload_method: 'PUT',
    upload_headers: {
      'Content-Type': 'image/png',
      'If-None-Match': '*',
      Host: 'account.r2.cloudflarestorage.com',
    },
  },
}
const videoMetadata = {
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
const confirmed = {
  success: true,
  data: {
    file_url: 'https://files.example/a',
    expires_at: '2026-10-15T00:00:00Z',
  },
}

describe('reference direct upload', () => {
  it('uploads file bytes to storage without application credentials before confirmation', async () => {
    const file = new File(['data'], 'a.png', { type: 'image/png' })
    const signal = new AbortController().signal
    vi.mocked(api.post)
      .mockResolvedValueOnce({ data: ticket })
      .mockResolvedValueOnce({ data: confirmed })
    const put = vi.fn().mockResolvedValue({ ok: true, status: 200 })
    vi.stubGlobal('fetch', put)
    await expect(uploadReferenceAsset(file, signal)).resolves.toEqual({
      url: 'https://files.example/a',
      expiresAt: Date.parse(confirmed.data.expires_at),
    })
    expect(api.post).toHaveBeenNthCalledWith(
      1,
      '/api/v1/files/upload/presign',
      { file_name: 'a.png', mime_type: 'image/png', file_size: 4 },
      { signal }
    )
    expect(put).toHaveBeenCalledWith(
      ticket.data.upload_url,
      expect.objectContaining({
        method: 'PUT',
        body: file,
        signal,
        credentials: 'omit',
        redirect: 'error',
      })
    )
    const headers = new Headers(put.mock.calls[0][1].headers)
    expect(headers.get('if-none-match')).toBe('*')
    expect(headers.has('authorization')).toBe(false)
    expect(headers.has('cookie')).toBe(false)
    expect(headers.has('host')).toBe(false)
    expect(api.post).toHaveBeenNthCalledWith(
      2,
      '/api/v1/files/upload/confirm',
      { file_id: 'file_x', file_size: 4 },
      { signal }
    )
  })
  it('falls back only when ticket capability is explicitly unsupported', async () => {
    vi.mocked(api.post)
      .mockRejectedValueOnce({ response: { status: 501 } })
      .mockResolvedValueOnce({ data: confirmed })
    const put = vi.fn()
    vi.stubGlobal('fetch', put)
    await uploadReferenceAsset(
      new File(['data'], 'a.png', { type: 'image/png' })
    )
    expect(api.post).toHaveBeenNthCalledWith(
      2,
      '/api/v1/files/upload/stream',
      expect.any(FormData),
      { signal: undefined }
    )
    expect(put).not.toHaveBeenCalled()
  })
  it.each([403, 429])(
    'does not retry through server after ticket refusal %s',
    async (status) => {
      vi.mocked(api.post).mockRejectedValue({ response: { status } })
      await expect(
        uploadReferenceAsset(new File(['data'], 'a.png', { type: 'image/png' }))
      ).rejects.toBeTruthy()
      expect(api.post).toHaveBeenCalledWith(
        '/api/v1/files/upload/presign',
        expect.any(Object),
        { signal: undefined }
      )
      expect(api.post).toHaveBeenCalledTimes(1)
    }
  )
  it('never confirms or relays a failed storage upload', async () => {
    vi.mocked(api.post).mockResolvedValue({ data: ticket })
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue({ ok: false, status: 403 })
    )
    await expect(
      uploadReferenceAsset(new File(['data'], 'a.png', { type: 'image/png' }))
    ).rejects.toBeTruthy()
    expect(api.post).toHaveBeenCalledTimes(1)
  })
  it('binds browser-checked video metadata to its upload ticket and keeps confirmation bound to that ticket', async () => {
    const file = new File(['clip'], 'reference.mp4', { type: 'video/mp4' })
    const signal = new AbortController().signal
    vi.mocked(api.post)
      .mockResolvedValueOnce({ data: ticket })
      .mockResolvedValueOnce({ data: confirmed })
    const put = vi.fn().mockResolvedValue({ ok: true, status: 200 })
    vi.stubGlobal('fetch', put)
    await uploadReferenceAsset(file, signal, {
      purpose: 'influencer_reference_video',
      videoMetadata,
    })
    expect(api.post).toHaveBeenNthCalledWith(
      1,
      '/api/v1/files/upload/presign',
      {
        file_name: file.name,
        mime_type: 'video/mp4',
        file_size: file.size,
        purpose: 'influencer_reference_video',
        video_metadata: videoMetadata,
      },
      { signal }
    )
    expect(put).toHaveBeenCalledWith(
      ticket.data.upload_url,
      expect.objectContaining({ body: file, signal, credentials: 'omit' })
    )
    expect(api.post).toHaveBeenNthCalledWith(
      2,
      '/api/v1/files/upload/confirm',
      { file_id: 'file_x', file_size: file.size },
      { signal }
    )
  })
  it('reports progress for large direct uploads without falling back to fetch', async () => {
    const file = new File([new Uint8Array(1024 * 1024)], 'big.png', {
      type: 'image/png',
    })
    const signal = new AbortController().signal
    vi.mocked(api.post)
      .mockResolvedValueOnce({ data: ticket })
      .mockResolvedValueOnce({ data: confirmed })
    const put = vi.fn()
    vi.stubGlobal('fetch', put)
    type Listener = (event: unknown) => void
    class ProgressXHR {
      status = 0
      withCredentials = false
      private listeners: Record<string, Listener[]> = {}
      upload: { addEventListener: (type: string, listener: Listener) => void } =
        {
          addEventListener: (type, listener) => {
            const list = this.listeners[`upload:${type}`] ?? []
            list.push(listener)
            this.listeners[`upload:${type}`] = list
          },
        }
      addEventListener(type: string, listener: Listener) {
        const list = this.listeners[type] ?? []
        list.push(listener)
        this.listeners[type] = list
      }
      private emit(type: string, event: unknown) {
        for (const listener of this.listeners[type] ?? []) listener(event)
      }
      private emitUpload(type: string, event: unknown) {
        for (const listener of this.listeners[`upload:${type}`] ?? []) {
          listener(event)
        }
      }
      open() {}
      setRequestHeader() {}
      abort() {
        this.emit('abort', {})
      }
      send() {
        this.emitUpload('progress', {
          lengthComputable: true,
          loaded: 512 * 1024,
          total: 1024 * 1024,
        })
        this.status = 200
        this.emit('load', {})
      }
    }
    vi.stubGlobal('XMLHttpRequest', ProgressXHR)
    const onProgress = vi.fn()
    await uploadReferenceAsset(file, signal, undefined, onProgress)
    expect(onProgress).toHaveBeenCalledWith(512 * 1024, 1024 * 1024)
    expect(put).not.toHaveBeenCalled()
    expect(api.post).toHaveBeenNthCalledWith(
      2,
      '/api/v1/files/upload/confirm',
      { file_id: 'file_x', file_size: file.size },
      { signal }
    )
  })
  it('preserves the video purpose and browser metadata through the explicitly unsupported-ticket fallback', async () => {
    const file = new File(['clip'], 'reference.mp4', { type: 'video/mp4' })
    const signal = new AbortController().signal
    vi.mocked(api.post)
      .mockRejectedValueOnce({ response: { status: 501 } })
      .mockResolvedValueOnce({ data: confirmed })
    const put = vi.fn()
    vi.stubGlobal('fetch', put)
    await uploadReferenceAsset(file, signal, {
      purpose: 'influencer_reference_video',
      videoMetadata,
    })
    expect(api.post).toHaveBeenNthCalledWith(
      1,
      '/api/v1/files/upload/presign',
      {
        file_name: file.name,
        mime_type: 'video/mp4',
        file_size: file.size,
        purpose: 'influencer_reference_video',
        video_metadata: videoMetadata,
      },
      { signal }
    )
    expect(api.post).toHaveBeenNthCalledWith(
      2,
      '/api/v1/files/upload/stream',
      expect.any(FormData),
      { signal }
    )
    const form = vi.mocked(api.post).mock.calls[1][1] as FormData
    expect(form.get('file')).toBe(file)
    expect(form.get('purpose')).toBe('influencer_reference_video')
    expect(form.get('video_metadata')).toBe(JSON.stringify(videoMetadata))
    expect(put).not.toHaveBeenCalled()
    expect(api.post).toHaveBeenCalledTimes(2)
  })
})
