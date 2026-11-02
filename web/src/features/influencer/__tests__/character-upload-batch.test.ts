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
import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import {
  useCharacterUploads,
  MAX_REFERENCE_UPLOAD_BATCH,
} from '../hooks/use-character-uploads'
import type { MotionCharacterSource } from '../types'

function imageFile(name: string, lastModified = 1000): File {
  return new File(['bytes'], name, { type: 'image/png', lastModified })
}

function deferredUpload() {
  const pending: Array<{
    file: File
    signal: AbortSignal
    onProgress: (loaded: number, total: number) => void
    resolve: (url: string) => void
    reject: (error: Error) => void
  }> = []
  const uploadImage = vi.fn(
    (
      file: File,
      onProgress: (loaded: number, total: number) => void,
      signal: AbortSignal
    ) =>
      new Promise<string>((resolve, reject) => {
        pending.push({ file, signal, onProgress, resolve, reject })
      })
  )
  return { pending, uploadImage }
}

beforeEach(() => {
  Object.defineProperty(URL, 'createObjectURL', {
    configurable: true,
    value: vi.fn(() => 'blob:preview'),
  })
  Object.defineProperty(URL, 'revokeObjectURL', {
    configurable: true,
    value: vi.fn(),
  })
})

describe('character upload queue', () => {
  it('refuses a pick over thirty files as a whole and reports the limit', async () => {
    const onLimit = vi.fn()
    const uploadImage = vi.fn()
    const { result } = renderHook(() =>
      useCharacterUploads({
        uploadImage,
        addCharacter: vi.fn(),
        imageLimit: () => 30,
        selectedCount: () => 0,
        onLimit,
      })
    )
    const files = Array.from(
      { length: MAX_REFERENCE_UPLOAD_BATCH + 1 },
      (_, i) => imageFile(`image-${i}.png`)
    )

    let outcome = { accepted: 0, limit: 0, added: 0, failed: 0 }
    await act(async () => {
      outcome = await result.current.pickFiles(files)
    })

    expect(outcome).toEqual({ accepted: 0, limit: 30, added: 0, failed: 0 })
    expect(onLimit).toHaveBeenCalledWith(30)
    expect(uploadImage).not.toHaveBeenCalled()
    expect(result.current.uploads).toHaveLength(0)
  })

  it('refuses a pick larger than the remaining capacity', async () => {
    const onLimit = vi.fn()
    const uploadImage = vi.fn()
    const { result } = renderHook(() =>
      useCharacterUploads({
        uploadImage,
        addCharacter: vi.fn(),
        imageLimit: () => 30,
        selectedCount: () => 28,
        onLimit,
      })
    )

    let outcome = { accepted: 0, limit: 0, added: 0, failed: 0 }
    await act(async () => {
      outcome = await result.current.pickFiles([
        imageFile('one.png'),
        imageFile('two.png'),
        imageFile('three.png'),
      ])
    })

    expect(outcome.accepted).toBe(0)
    expect(onLimit).toHaveBeenCalledWith(2)
    expect(uploadImage).not.toHaveBeenCalled()
  })

  it('shows every picked file immediately and runs three at a time', () => {
    const { uploadImage } = deferredUpload()
    const { result } = renderHook(() =>
      useCharacterUploads({
        uploadImage,
        addCharacter: vi.fn(),
        imageLimit: () => 30,
        selectedCount: () => 0,
        onLimit: vi.fn(),
      })
    )

    act(() => {
      void result.current.pickFiles([
        imageFile('a.png'),
        imageFile('b.png'),
        imageFile('c.png'),
        imageFile('d.png'),
      ])
    })

    expect(result.current.uploads).toHaveLength(4)
    expect(uploadImage).toHaveBeenCalledTimes(3)
    expect(
      result.current.uploads.filter((upload) => upload.status === 'waiting')
    ).toHaveLength(1)
  })

  it('appends files in picker order even when later transfers finish first', async () => {
    const { pending, uploadImage } = deferredUpload()
    const order: string[] = []
    const addCharacter = vi.fn((source: MotionCharacterSource) => {
      order.push(source.url)
    })
    const { result } = renderHook(() =>
      useCharacterUploads({
        uploadImage,
        addCharacter,
        imageLimit: () => 30,
        selectedCount: () => order.length,
        onLimit: vi.fn(),
      })
    )

    act(() => {
      void result.current.pickFiles([
        imageFile('a.png'),
        imageFile('b.png'),
        imageFile('c.png'),
        imageFile('d.png'),
      ])
    })

    await act(async () => {
      pending[2].resolve('https://files.example/c.png')
    })
    expect(addCharacter).not.toHaveBeenCalled()

    await act(async () => {
      pending[0].resolve('https://files.example/a.png')
    })
    await waitFor(() => expect(addCharacter).toHaveBeenCalledTimes(1))

    await act(async () => {
      pending[1].resolve('https://files.example/b.png')
    })
    await waitFor(() => expect(addCharacter).toHaveBeenCalledTimes(3))

    await act(async () => {
      pending[3].resolve('https://files.example/d.png')
    })
    await waitFor(() => expect(result.current.uploads).toHaveLength(0))

    expect(order).toEqual([
      'https://files.example/a.png',
      'https://files.example/b.png',
      'https://files.example/c.png',
      'https://files.example/d.png',
    ])
  })

  it('accepts another pick while the queue is uploading without exceeding three transfers', async () => {
    const { pending, uploadImage } = deferredUpload()
    const addCharacter = vi.fn()
    const { result } = renderHook(() =>
      useCharacterUploads({
        uploadImage,
        addCharacter,
        imageLimit: () => 30,
        selectedCount: () => addCharacter.mock.calls.length,
        onLimit: vi.fn(),
      })
    )

    act(() => {
      void result.current.pickFiles([imageFile('a.png'), imageFile('b.png')])
    })
    expect(uploadImage).toHaveBeenCalledTimes(2)

    act(() => {
      void result.current.pickFiles([imageFile('c.png'), imageFile('d.png')])
    })
    expect(uploadImage).toHaveBeenCalledTimes(3)
    expect(result.current.uploads).toHaveLength(4)

    await act(async () => {
      pending[0].resolve('https://files.example/a.png')
    })
    await waitFor(() => expect(uploadImage).toHaveBeenCalledTimes(4))

    for (const index of [1, 2, 3]) {
      await act(async () => {
        pending[index].resolve(`https://files.example/${index}.png`)
      })
    }
    await waitFor(() => expect(result.current.uploads).toHaveLength(0))
    expect(addCharacter).toHaveBeenCalledTimes(4)
  })

  it('reports aggregate progress from the in-flight transfers', async () => {
    const { pending, uploadImage } = deferredUpload()
    const { result } = renderHook(() =>
      useCharacterUploads({
        uploadImage,
        addCharacter: vi.fn(),
        imageLimit: () => 30,
        selectedCount: () => 0,
        onLimit: vi.fn(),
      })
    )

    act(() => {
      void result.current.pickFiles([imageFile('a.png'), imageFile('b.png')])
    })

    await act(async () => {
      pending[0].onProgress(50, 100)
    })

    expect(result.current.progress).toBe(25)
  })

  it('keeps a failed file for retry and still appends the files picked after it', async () => {
    const addCharacter = vi.fn()
    let failNext = true
    const uploadImage = vi.fn(async (file: File) => {
      if (file.name === 'bad.png' && failNext) {
        failNext = false
        throw new Error('upload failed')
      }
      return `https://files.example/${file.name}`
    })
    const { result } = renderHook(() =>
      useCharacterUploads({
        uploadImage,
        addCharacter,
        imageLimit: () => 30,
        selectedCount: () => 0,
        onLimit: vi.fn(),
      })
    )

    let outcome = { accepted: 0, limit: 0, added: 0, failed: 0 }
    await act(async () => {
      outcome = await result.current.pickFiles([
        imageFile('bad.png'),
        imageFile('ok.png'),
      ])
    })

    expect(outcome).toMatchObject({ accepted: 2, added: 1, failed: 1 })
    expect(addCharacter).toHaveBeenCalledWith({
      url: 'https://files.example/ok.png',
      fileName: 'ok.png',
    })
    const failed = result.current.uploads.find(
      (upload) => upload.status === 'error'
    )
    expect(failed?.fileName).toBe('bad.png')
    expect(result.current.uploading).toBe(false)

    await act(async () => {
      await result.current.retry(failed?.id ?? '')
    })
    expect(addCharacter).toHaveBeenCalledTimes(2)
    expect(result.current.uploads).toHaveLength(0)
  })

  it('refuses a retry that would exceed the reference ceiling', async () => {
    let selected = 0
    let failNext = true
    const onLimit = vi.fn()
    const uploadImage = vi.fn(async (file: File) => {
      if (failNext) {
        failNext = false
        throw new Error('upload failed')
      }
      return `https://files.example/${file.name}`
    })
    const { result } = renderHook(() =>
      useCharacterUploads({
        uploadImage,
        addCharacter: vi.fn(),
        imageLimit: () => 1,
        selectedCount: () => selected,
        onLimit,
      })
    )

    await act(async () => {
      await result.current.pickFiles([imageFile('bad.png')])
    })
    const failed = result.current.uploads.find(
      (upload) => upload.status === 'error'
    )
    expect(failed).toBeDefined()

    selected = 1
    let outcome = { accepted: 0, limit: 0, added: 0, failed: 0 }
    await act(async () => {
      outcome = await result.current.retry(failed?.id ?? '')
    })
    expect(outcome.accepted).toBe(0)
    expect(onLimit).toHaveBeenCalledWith(0)
    expect(uploadImage).toHaveBeenCalledTimes(1)
  })

  it('aborts in-flight transfers and releases every preview when cancelled', async () => {
    const { pending, uploadImage } = deferredUpload()
    const addCharacter = vi.fn()
    const { result } = renderHook(() =>
      useCharacterUploads({
        uploadImage,
        addCharacter,
        imageLimit: () => 30,
        selectedCount: () => 0,
        onLimit: vi.fn(),
      })
    )

    act(() => {
      void result.current.pickFiles([
        imageFile('one.png'),
        imageFile('two.png'),
      ])
    })
    expect(result.current.uploads).toHaveLength(2)

    act(() => {
      result.current.cancel()
    })
    expect(result.current.uploads).toHaveLength(0)
    expect(URL.revokeObjectURL).toHaveBeenCalledTimes(2)
    expect(pending[0].signal.aborted).toBe(true)
    expect(pending[1].signal.aborted).toBe(true)

    await act(async () => {
      pending[0].resolve('https://files.example/one.png')
      pending[1].resolve('https://files.example/two.png')
    })
    expect(addCharacter).not.toHaveBeenCalled()
  })

  it('releases the local preview once a file is appended', async () => {
    const uploadImage = vi.fn(
      async (file: File) => `https://files.example/${file.name}`
    )
    const addCharacter = vi.fn()
    const { result } = renderHook(() =>
      useCharacterUploads({
        uploadImage,
        addCharacter,
        imageLimit: () => 30,
        selectedCount: () => 0,
        onLimit: vi.fn(),
      })
    )

    await act(async () => {
      await result.current.pickFiles([imageFile('one.png')])
    })

    expect(addCharacter).toHaveBeenCalledTimes(1)
    expect(URL.revokeObjectURL).toHaveBeenCalledTimes(1)
    expect(result.current.uploads).toHaveLength(0)
  })

  it('deduplicates the same file picked twice in one pick', async () => {
    const addCharacter = vi.fn()
    const uploadImage = vi.fn(
      async (file: File) => `https://files.example/${file.name}`
    )
    const { result } = renderHook(() =>
      useCharacterUploads({
        uploadImage,
        addCharacter,
        imageLimit: () => 30,
        selectedCount: () => 0,
        onLimit: vi.fn(),
      })
    )
    const firstPick = imageFile('same.png', 1234)
    const secondPick = imageFile('same.png', 1234)
    expect(firstPick).not.toBe(secondPick)

    await act(async () => {
      await result.current.pickFiles([firstPick, secondPick])
    })

    expect(uploadImage).toHaveBeenCalledTimes(1)
    expect(addCharacter).toHaveBeenCalledTimes(1)
  })
})
