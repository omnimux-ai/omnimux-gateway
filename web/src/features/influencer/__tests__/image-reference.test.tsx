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
import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
} from '@testing-library/react'
import { toast } from 'sonner'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { MediaSlot } from '../components/media-slot'
import { useBuilder } from '../hooks/use-builder'

const file = (type = 'image/png', size = 1) =>
  new File([new Uint8Array(size)], 'reference.png', { type })
interface UploadResponse {
  data: { success: boolean; data: { file_url: string } }
}
function mockDirectUpload(
  confirm: () => UploadResponse | Promise<UploadResponse>
) {
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: true, status: 200 }))
  return vi.spyOn(api, 'post').mockImplementation(async (url) => {
    if (url === '/api/v1/files/upload/presign') {
      return {
        data: {
          success: true,
          data: {
            file_id: 'reference-file',
            upload_url:
              'https://account.r2.cloudflarestorage.com/files/reference?signature=test',
            upload_method: 'PUT',
            upload_headers: { 'If-None-Match': '*' },
          },
        },
      }
    }
    if (url === '/api/v1/files/upload/confirm') return await confirm()
    throw new Error(`Unexpected POST: ${url}`)
  })
}
afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

describe('single style reference upload', () => {
  it('writes the uploaded URL only after success without changing exaggeration', async () => {
    let resolve!: (value: UploadResponse) => void
    const confirmation = new Promise<UploadResponse>((done) => {
      resolve = done
    })
    mockDirectUpload(() => confirmation)
    const { result } = renderHook(() => useBuilder(null))
    let upload!: Promise<void>
    act(() => {
      upload = result.current.pickImage(file())
    })
    expect(result.current.imageUploading).toBe(true)
    expect(result.current.state.imageUrl).toBe('')
    await act(async () => {
      resolve({
        data: {
          success: true,
          data: { file_url: 'https://files.example/new.png' },
        },
      })
      await upload
    })
    expect(result.current.state.imageUrl).toBe('https://files.example/new.png')
    expect(result.current.state.tier).toBe('total')
  })

  it('preserves an upload business failure msg instead of replacing it with a generic error', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({
      data: { success: false, msg: 'Storage temporarily unavailable' },
    })
    const errorToast = vi.spyOn(toast, 'error')
    const { result } = renderHook(() => useBuilder(null))
    await act(async () => {
      await result.current.pickImage(file())
    })
    expect(errorToast).toHaveBeenCalledWith('Storage temporarily unavailable')
  })

  it('keeps the old URL when replacement fails and exposes the server reason', async () => {
    vi.spyOn(api, 'post').mockRejectedValue(new Error('Storage unavailable'))
    const errorToast = vi.spyOn(toast, 'error')
    const { result } = renderHook(() => useBuilder(null))
    act(() => result.current.setImageUrl('https://files.example/old.png'))
    await act(async () => {
      await result.current.pickImage(file())
    })
    expect(result.current.state.imageUrl).toBe('https://files.example/old.png')
    expect(errorToast).toHaveBeenCalledWith('Storage unavailable')
  })

  it.each(['reset', 'applyLook'] as const)(
    'ignores a late upload after %s replaces the look',
    async (action) => {
      let resolve!: (value: UploadResponse) => void
      const confirmation = new Promise<UploadResponse>((done) => {
        resolve = done
      })
      mockDirectUpload(() => confirmation)
      const { result } = renderHook(() => useBuilder(null))
      let upload!: Promise<void>
      act(() => {
        upload = result.current.pickImage(file())
      })
      act(() => {
        if (action === 'reset') result.current.reset()
        else {
          result.current.applyLook({
            values: {},
            image_url: 'https://files.example/history.png',
          })
        }
      })
      await act(async () => {
        resolve({
          data: {
            success: true,
            data: { file_url: 'https://files.example/late.png' },
          },
        })
        await upload
      })
      expect(result.current.state.imageUrl).toBe(
        action === 'reset' ? '' : 'https://files.example/history.png'
      )
      expect(result.current.imageUploading).toBe(false)
    }
  )

  it.each([
    ['image/gif', 1],
    ['image/png', 5 * 1024 * 1024 + 1],
  ])('rejects invalid %s / %s bytes without network', async (type, size) => {
    const post = vi.spyOn(api, 'post')
    const errorToast = vi.spyOn(toast, 'error')
    const { result } = renderHook(() => useBuilder(null))
    await act(async () => {
      await result.current.pickImage(file(String(type), Number(size)))
    })
    expect(post).not.toHaveBeenCalled()
    expect(errorToast).toHaveBeenCalled()
  })

  it.each(['image/jpeg', 'image/png', 'image/webp'])(
    'accepts %s at the 5MiB boundary',
    async (type) => {
      mockDirectUpload(() => ({
        data: {
          success: true,
          data: { file_url: 'https://files.example/valid.png' },
        },
      }))
      const { result } = renderHook(() => useBuilder(null))
      await act(async () => {
        await result.current.pickImage(file(type, 5 * 1024 * 1024))
      })
      expect(result.current.state.imageUrl).toBe(
        'https://files.example/valid.png'
      )
    }
  )

  it('rejects a concurrent upload without replacing the pending request', async () => {
    let resolve!: (value: UploadResponse) => void
    const confirmation = new Promise<UploadResponse>((done) => {
      resolve = done
    })
    const post = mockDirectUpload(() => confirmation)
    const { result } = renderHook(() => useBuilder(null))
    let upload!: Promise<void>
    act(() => {
      upload = result.current.pickImage(file())
    })
    await act(async () => {
      await result.current.pickImage(file())
    })
    expect(post.mock.calls.map(([url]) => url)).toEqual([
      '/api/v1/files/upload/presign',
      '/api/v1/files/upload/confirm',
    ])
    expect(fetch).toHaveBeenCalledTimes(1)
    await act(async () => {
      resolve({
        data: {
          success: true,
          data: { file_url: 'https://files.example/first.png' },
        },
      })
      await upload
    })
  })
})

describe('style reference slot', () => {
  function slot(busy = false) {
    const onPick = vi.fn()
    const onClear = vi.fn()
    const view = render(
      <MediaSlot
        kind='image'
        variant='reference'
        label='Style reference'
        hint='Overall look and style'
        accept='image/png,image/jpeg,image/webp'
        previewUrl='https://files.example/old.png'
        busy={busy}
        onPick={onPick}
        onClear={onClear}
      />
    )
    return { ...view, onPick, onClear }
  }
  it('does not describe a generation lock as an image upload', () => {
    slot(true)
    expect(screen.queryByText('Uploading image…')).not.toBeInTheDocument()
  })
  it('blocks drop, replace, remove and file-input changes while busy', () => {
    const { container, onPick, onClear } = slot(true)
    expect(screen.getByRole('button', { name: /Replace/ })).toBeDisabled()
    expect(screen.getByRole('button', { name: /Remove/ })).toBeDisabled()
    fireEvent.drop(container.firstElementChild as HTMLElement, {
      dataTransfer: { files: [file()] },
    })
    fireEvent.change(container.querySelector('input') as HTMLInputElement, {
      target: { files: [file()] },
    })
    expect(onPick).not.toHaveBeenCalled()
    expect(onClear).not.toHaveBeenCalled()
  })
  it('rejects multiple dropped files through the top toast', () => {
    const errorToast = vi.spyOn(toast, 'error')
    const { container, onPick } = slot()
    fireEvent.drop(container.firstElementChild as HTMLElement, {
      dataTransfer: { files: [file(), file()] },
    })
    expect(onPick).not.toHaveBeenCalled()
    expect(errorToast).toHaveBeenCalledWith('Choose one image only')
    expect(screen.queryByRole('alert')).toBeNull()
  })
  it('shows recoverable preview failure while keeping remove and replace available', () => {
    const { container } = slot()
    fireEvent.error(container.querySelector('img') as HTMLImageElement)
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Reference image is unavailable'
    )
    expect(screen.getByRole('button', { name: /Replace/ })).toBeEnabled()
    expect(screen.getByRole('button', { name: /Remove/ })).toBeEnabled()
  })
  it('restores focus to the empty upload button after removing the image', async () => {
    const { container, rerender } = slot()
    fireEvent.click(screen.getByRole('button', { name: /Remove/ }))
    rerender(
      <MediaSlot
        kind='image'
        variant='reference'
        label='Style reference'
        hint='Overall look and style'
        accept='image/png'
        onPick={() => {}}
        onClear={() => {}}
      />
    )
    await waitFor(() =>
      expect(
        screen.getByRole('button', { name: 'Style reference' })
      ).toHaveFocus()
    )
    expect(container.querySelector('img')).toBeNull()
  })
})
