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

import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import type { SheetSubmitBody } from '../api'
import { useSheetSubmit } from '../hooks/use-tasks'

const body: SheetSubmitBody = {
  model: 'gpt-image-2.5-sunburst',
  group: 'gpt-image-2.5-sunburst-image',
  tier: 'total',
  selection: {},
  brief: '',
  size: '9:16',
  image_url: 'https://files.example/style.png',
}
function page(taskId: string, status: string) {
  return {
    data: {
      success: true,
      data: {
        items: [{ id: 1, task_id: taskId, status }],
        total: 1,
      },
    },
  }
}
beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
})

describe('accepted sheet task tracking through the real API', () => {
  it('continues tracking the same public task as its dispatch state changes without another submission', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { task_id: 'accepted' } })
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValueOnce(page('accepted', 'DISPATCHING'))
      .mockResolvedValueOnce(page('accepted', 'IN_PROGRESS'))
      .mockResolvedValue(page('accepted', 'SUCCESS'))
    const { result } = renderHook(() => useSheetSubmit())

    await act(async () => {
      await result.current.submit(body)
    })
    expect(result.current.pending).toMatchObject({
      task_id: 'accepted',
      status: 'DISPATCHING',
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(result.current.pending).toMatchObject({
      task_id: 'accepted',
      status: 'IN_PROGRESS',
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(result.current.pending).toMatchObject({
      task_id: 'accepted',
      status: 'SUCCESS',
    })
    expect(post).toHaveBeenCalledTimes(1)
    expect(
      get.mock.calls.every(
        ([, config]) => config?.params.task_id === 'accepted'
      )
    ).toBe(true)
  })

  it('ignores a different task returned by history and retains the accepted task while allowing another submission', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { task_id: 'accepted' } })
    vi.spyOn(api, 'get').mockResolvedValue(page('other', 'SUCCESS'))
    const { result } = renderHook(() => useSheetSubmit())
    await act(async () => {
      await result.current.submit(body)
    })
    expect(result.current.pending).toMatchObject({
      task_id: 'accepted',
      status: 'QUEUED',
    })
    await act(async () => {
      await result.current.submit(body)
    })
    expect(post).toHaveBeenCalledTimes(2)
  })
  it('unlocks only after the server confirms a terminal task and replaces that record when the next ID is accepted', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValueOnce({ data: { task_id: 'first' } })
      .mockResolvedValueOnce({ data: { task_id: 'second' } })
    vi.spyOn(api, 'get').mockImplementation(async (_url, config) =>
      config?.params.task_id === 'first'
        ? page('first', 'SUCCESS')
        : { data: { success: true, data: { items: [], total: 0 } } }
    )
    const { result } = renderHook(() => useSheetSubmit())
    await act(async () => {
      await result.current.submit(body)
    })
    expect(result.current.pending?.status).toBe('SUCCESS')
    await act(async () => {
      await result.current.submit(body)
    })
    expect(result.current.pending).toMatchObject({
      task_id: 'second',
      status: 'QUEUED',
    })
    expect(post).toHaveBeenCalledTimes(2)
  })

  it('allows an explicit retry after a rejected submission that never returned a task ID', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockRejectedValueOnce(new Error('Rejected'))
      .mockResolvedValueOnce({ data: { task_id: 'accepted' } })
    vi.spyOn(api, 'get').mockResolvedValue(page('accepted', 'IN_PROGRESS'))
    const { result } = renderHook(() => useSheetSubmit())
    await act(async () => {
      await result.current.submit(body)
    })
    expect(result.current.pending).toBeNull()
    expect(result.current.error).toEqual(new Error('Rejected'))
    await act(async () => {
      await result.current.submit(body)
    })
    expect(result.current.pending?.task_id).toBe('accepted')
    expect(post).toHaveBeenCalledTimes(2)
  })
  it('keeps the accepted task after polling fails and continues read-only recovery', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { task_id: 'accepted' } })
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValueOnce(page('accepted', 'IN_PROGRESS'))
      .mockRejectedValueOnce(new Error('Polling unavailable'))
      .mockResolvedValue(page('accepted', 'IN_PROGRESS'))
    const { result } = renderHook(() => useSheetSubmit())
    await act(async () => {
      await result.current.submit(body)
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(result.current.pending?.status).toBe('IN_PROGRESS')
    // A read outage is not a rejected submission and must not invite a replay.
    expect(result.current.error).toBeNull()
    await act(async () => {
      await result.current.submit(body)
    })
    expect(post).toHaveBeenCalledTimes(2)
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(get).toHaveBeenCalledTimes(4)
    expect(result.current.error).toBeNull()
  })

  it.each(['initial', 'poll'])(
    'ignores a late %s read after reset and keeps the new accepted task tracking',
    async (phase) => {
      let finish!: (value: ReturnType<typeof page>) => void
      const late = new Promise<ReturnType<typeof page>>((resolve) => {
        finish = resolve
      })
      const post = vi
        .spyOn(api, 'post')
        .mockResolvedValueOnce({ data: { task_id: 'old' } })
        .mockResolvedValueOnce({ data: { task_id: 'new' } })
      const get = vi
        .spyOn(api, 'get')
        .mockImplementation(async (_url, config) => {
          if (config?.params.task_id === 'new') {
            return { data: { success: true, data: { items: [], total: 0 } } }
          }
          if (phase === 'poll' && get.mock.calls.length === 1) {
            return page('old', 'IN_PROGRESS')
          }
          return late
        })
      const { result } = renderHook(() => useSheetSubmit())
      let oldSubmit!: Promise<unknown>
      await act(async () => {
        oldSubmit = result.current.submit(body)
      })
      if (phase === 'poll') {
        await act(async () => {
          await oldSubmit
          await vi.advanceTimersByTimeAsync(2000)
        })
      }
      act(() => result.current.reset())
      await act(async () => {
        await result.current.submit(body)
      })
      await act(async () => {
        finish(page('old', 'SUCCESS'))
        await oldSubmit
      })
      expect(result.current.pending).toMatchObject({
        task_id: 'new',
        status: 'QUEUED',
      })
      const reads = get.mock.calls.length
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000)
      })
      expect(get.mock.calls.length).toBeGreaterThan(reads)
      expect(get.mock.calls.at(-1)?.[1]?.params.task_id).toBe('new')
      expect(post).toHaveBeenCalledTimes(2)
    }
  )

  it('ignores a late initial read after reset without resurrecting the forgotten task', async () => {
    let finish!: (value: ReturnType<typeof page>) => void
    vi.spyOn(api, 'post').mockResolvedValue({ data: { task_id: 'old' } })
    const get = vi.spyOn(api, 'get').mockReturnValue(
      new Promise((resolve) => {
        finish = resolve
      })
    )
    const { result } = renderHook(() => useSheetSubmit())
    let submission!: Promise<unknown>
    await act(async () => {
      submission = result.current.submit(body)
    })
    act(() => result.current.reset())
    await act(async () => {
      finish(page('old', 'IN_PROGRESS'))
      await submission
    })
    expect(result.current.pending).toBeNull()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(get).toHaveBeenCalledTimes(1)
  })
  it.each(['empty', 'error'])(
    'keeps an accepted ID after the first read is %s and retries reads without resubmitting',
    async (failure) => {
      const post = vi
        .spyOn(api, 'post')
        .mockResolvedValue({ data: { task_id: 'accepted' } })
      const get = vi.spyOn(api, 'get')
      if (failure === 'error') {
        get.mockRejectedValueOnce(new Error('Read unavailable'))
      } else {
        get.mockResolvedValueOnce({
          data: { success: true, data: { items: [], total: 0 } },
        })
      }
      get.mockResolvedValue(page('accepted', 'IN_PROGRESS'))
      const { result } = renderHook(() => useSheetSubmit())
      await act(async () => {
        await result.current.submit(body)
      })
      expect(result.current.pending).toMatchObject({
        task_id: 'accepted',
        status: 'QUEUED',
      })
      expect(result.current.pending?.progress).toBeUndefined()
      expect(result.current.submitting).toBe(false)
      expect(post).toHaveBeenCalledTimes(1)
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000)
      })
      expect(result.current.pending).toMatchObject({
        task_id: 'accepted',
        status: 'IN_PROGRESS',
      })
      expect(get).toHaveBeenCalledTimes(2)
      expect(result.current.error).toBeNull()
    }
  )
})
