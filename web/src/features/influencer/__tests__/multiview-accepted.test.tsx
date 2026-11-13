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
import { useAuthStore } from '@/stores/auth-store'

import type { MultiViewSubmitBody } from '../api'
import { useMultiViewSubmit } from '../hooks/use-multiview'

const body: MultiViewSubmitBody = {
  model: 'gpt-image-2.5-sunburst',
  group: 'gpt-image-2.5-sunburst-image',
  parentTaskId: 'parent',
  imageUrl: 'https://files.example/character.png',
}

function page(taskId: string, status: string) {
  return {
    data: {
      success: true,
      data: { items: [{ id: 1, task_id: taskId, status }], total: 1 },
    },
  }
}

beforeEach(() => vi.useFakeTimers())
afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

describe('accepted multi-view task tracking through the real API', () => {
  it('loads artifacts once after confirmed success and stops reading the completed board', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({ data: { task_id: 'accepted' } })
    const get = vi
      .spyOn(api, 'get')
      .mockImplementation(async (_url, config) => {
        const response = page('accepted', 'SUCCESS')
        if (config?.params.with_artifacts) {
          return {
            data: {
              success: true,
              data: {
                items: [
                  {
                    ...response.data.data.items[0],
                    artifacts: [
                      { content_url: 'https://files.example/board.png' },
                    ],
                  },
                ],
                total: 1,
              },
            },
          }
        }
        return response
      })
    const { result } = renderHook(() => useMultiViewSubmit())
    await act(async () => {
      await result.current.submit(body)
    })
    expect(result.current.childByParent.parent).toMatchObject({
      status: 'SUCCESS',
      artifacts: [{ content_url: 'https://files.example/board.png' }],
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4000)
    })
    expect(
      get.mock.calls.map((call) => call[1]?.params.with_artifacts)
    ).toEqual([false, true])
  })

  it.each(['error', 'empty', 'wrong-id', 'nonterminal'])(
    'preserves confirmed success when artifact read returns %s',
    async (failure) => {
      vi.spyOn(api, 'post').mockResolvedValue({ data: { task_id: 'accepted' } })
      const get = vi
        .spyOn(api, 'get')
        .mockImplementation(async (_url, config) => {
          if (!config?.params.with_artifacts) return page('accepted', 'SUCCESS')
          if (failure === 'error') throw new Error('Artifacts unavailable')
          if (failure === 'empty') {
            return { data: { success: true, data: { items: [], total: 0 } } }
          }
          return page(
            failure === 'wrong-id' ? 'other' : 'accepted',
            failure === 'nonterminal' ? 'IN_PROGRESS' : 'SUCCESS'
          )
        })
      const { result } = renderHook(() => useMultiViewSubmit())
      await act(async () => {
        await result.current.submit(body)
      })
      expect(result.current.childByParent.parent).toMatchObject({
        task_id: 'accepted',
        status: 'SUCCESS',
      })
      expect(
        get.mock.calls.map((call) => call[1]?.params.with_artifacts)
      ).toEqual([false, true])
      await act(async () => {
        await vi.advanceTimersByTimeAsync(4000)
      })
      expect(get).toHaveBeenCalledTimes(2)
    }
  )

  it.each(['same-parent', 'account'])(
    'ignores late terminal artifacts after a %s identity change without stopping the newer task',
    async (change) => {
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
            return page('new', 'IN_PROGRESS')
          }
          return config?.params.with_artifacts ? late : page('old', 'SUCCESS')
        })
      const { result } = renderHook(() => useMultiViewSubmit())
      await act(async () => {
        await result.current.submit(body)
      })
      expect(get.mock.calls.at(-1)?.[1]?.params.with_artifacts).toBe(true)
      act(() => {
        if (change === 'same-parent') result.current.forget('parent')
        else {
          useAuthStore.setState((state) => ({
            auth: {
              ...state.auth,
              user: { id: 2, username: 'second', role: 1 },
            },
          }))
        }
      })
      await act(async () => {
        await result.current.submit(body)
      })
      await act(async () => {
        finish(page('old', 'SUCCESS'))
      })
      expect(result.current.childByParent.parent).toMatchObject({
        task_id: 'new',
        status: 'IN_PROGRESS',
      })
      const reads = get.mock.calls.length
      await act(async () => {
        await vi.advanceTimersByTimeAsync(2000)
      })
      expect(
        get.mock.calls.slice(reads).map((call) => call[1]?.params.task_id)
      ).toEqual(['new'])
      expect(post).toHaveBeenCalledTimes(2)
    }
  )

  it('keeps an unknown child locked but lets another parent submit and continue polling', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValueOnce({ data: { task_id: 'unknown' } })
      .mockResolvedValueOnce({ data: { task_id: 'other' } })
    const get = vi
      .spyOn(api, 'get')
      .mockImplementation(async (_url, config) => {
        const response = page(config?.params.task_id, 'DISPATCHING')
        return {
          data: {
            success: true,
            data: {
              items: [
                {
                  ...response.data.data.items[0],
                  fail_reason:
                    config?.params.task_id === 'unknown'
                      ? 'Submission needs review'
                      : '',
                },
              ],
              total: 1,
            },
          },
        }
      })
    const { result } = renderHook(() => useMultiViewSubmit())
    await act(async () => {
      await result.current.submit(body)
      expect(await result.current.submit(body)).toBeNull()
      await result.current.submit({ ...body, parentTaskId: 'other' })
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(Object.keys(result.current.childByParent)).toEqual([
      'parent',
      'other',
    ])
    expect(
      get.mock.calls
        .slice(-2)
        .map((call) => call[1]?.params.task_id)
        .sort()
    ).toEqual(['other', 'unknown'])
    expect(post).toHaveBeenCalledTimes(2)
  })

  it('ignores a different task returned by history and keeps polling the accepted task', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({ data: { task_id: 'accepted' } })
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValueOnce(page('other-task', 'SUCCESS'))
      .mockResolvedValueOnce(page('other-task', 'SUCCESS'))
      .mockResolvedValue(page('accepted', 'IN_PROGRESS'))
    const { result } = renderHook(() => useMultiViewSubmit())
    await act(async () => {
      await result.current.submit(body)
    })
    expect(result.current.childByParent.parent).toMatchObject({
      task_id: 'accepted',
      status: 'QUEUED',
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(result.current.childByParent.parent.task_id).toBe('accepted')
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(result.current.childByParent.parent).toMatchObject({
      task_id: 'accepted',
      status: 'IN_PROGRESS',
    })
    expect(get).toHaveBeenCalledTimes(3)
  })
  it('attaches the accepted ID before the initial read finishes, preserving the nonterminal busy record', async () => {
    let finish!: (value: ReturnType<typeof page>) => void
    vi.spyOn(api, 'post').mockResolvedValue({ data: { task_id: 'accepted' } })
    vi.spyOn(api, 'get').mockReturnValue(
      new Promise((resolve) => {
        finish = resolve
      })
    )
    const { result } = renderHook(() => useMultiViewSubmit())
    let submission!: Promise<unknown>
    await act(async () => {
      submission = result.current.submit(body)
    })
    expect(result.current.childByParent.parent).toMatchObject({
      task_id: 'accepted',
      status: 'QUEUED',
    })
    expect(result.current.submittingParent).toBeNull()
    await act(async () => {
      finish(page('accepted', 'IN_PROGRESS'))
      await submission
    })
    expect(result.current.submittingParent).toBeNull()
    expect(result.current.childByParent.parent.status).toBe('IN_PROGRESS')
  })

  it('keeps an empty first read pending and only polls that accepted ID', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { task_id: 'accepted' } })
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValueOnce({
        data: { success: true, data: { items: [], total: 0 } },
      })
      .mockResolvedValue(page('accepted', 'SUCCESS'))
    const { result } = renderHook(() => useMultiViewSubmit())
    await act(async () => {
      await result.current.submit(body)
    })
    expect(result.current.childByParent.parent).toMatchObject({
      task_id: 'accepted',
      status: 'QUEUED',
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(result.current.childByParent.parent.status).toBe('SUCCESS')
    expect(get.mock.calls.at(-1)?.[1]?.params.task_id).toBe('accepted')
    expect(post).toHaveBeenCalledTimes(1)
  })

  it('forgets only the requested parent while the other parent keeps its task and polling', async () => {
    vi.spyOn(api, 'post')
      .mockResolvedValueOnce({ data: { task_id: 'first' } })
      .mockResolvedValueOnce({ data: { task_id: 'second' } })
    const get = vi
      .spyOn(api, 'get')
      .mockImplementation(async (_url, config) =>
        page(config?.params.task_id, 'IN_PROGRESS')
      )
    const { result } = renderHook(() => useMultiViewSubmit())
    await act(async () => {
      await result.current.submit(body)
      await result.current.submit({ ...body, parentTaskId: 'other' })
    })
    act(() => result.current.forget('parent'))
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(result.current.childByParent.parent).toBeUndefined()
    expect(result.current.childByParent.other).toMatchObject({
      task_id: 'second',
      status: 'IN_PROGRESS',
    })
    expect(get.mock.calls.at(-1)?.[1]?.params.task_id).toBe('second')
    expect(get).toHaveBeenCalledTimes(3)
  })

  it('leaves no accepted child after a rejected POST and permits the next explicit submission', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockRejectedValueOnce(new Error('Rejected'))
      .mockResolvedValueOnce({ data: { task_id: 'accepted' } })
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue(page('accepted', 'SUCCESS'))
    const { result } = renderHook(() => useMultiViewSubmit())
    await act(async () => {
      await expect(result.current.submit(body)).rejects.toThrow('Rejected')
    })
    expect(result.current.childByParent.parent).toBeUndefined()
    expect(result.current.submittingParent).toBeNull()
    expect(get).not.toHaveBeenCalled()
    await act(async () => {
      await result.current.submit(body)
    })
    expect(result.current.childByParent.parent).toMatchObject({
      task_id: 'accepted',
      status: 'SUCCESS',
    })
    expect(post).toHaveBeenCalledTimes(2)
  })
  it.each(['initial', 'poll'])(
    'ignores a late %s read after forgetting and cannot replace the newer task or stop its polling',
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
            return page('new', 'IN_PROGRESS')
          }
          if (phase === 'poll' && get.mock.calls.length === 1) {
            return page('old', 'IN_PROGRESS')
          }
          return late
        })
      const { result } = renderHook(() => useMultiViewSubmit())
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
      act(() => result.current.forget('parent'))
      await act(async () => {
        await result.current.submit(body)
      })
      await act(async () => {
        finish(page('old', 'SUCCESS'))
        await oldSubmit
      })
      expect(result.current.childByParent.parent).toMatchObject({
        task_id: 'new',
        status: 'IN_PROGRESS',
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

  it.each(['initial', 'poll'])(
    'does not resurrect a forgotten task when its late %s read finishes',
    async (phase) => {
      let finish!: (value: ReturnType<typeof page>) => void
      const late = new Promise<ReturnType<typeof page>>((resolve) => {
        finish = resolve
      })
      vi.spyOn(api, 'post').mockResolvedValue({ data: { task_id: 'old' } })
      const get = vi.spyOn(api, 'get').mockImplementation(async () => {
        if (phase === 'poll' && get.mock.calls.length === 1) {
          return page('old', 'IN_PROGRESS')
        }
        return late
      })
      const { result } = renderHook(() => useMultiViewSubmit())
      let submission!: Promise<unknown>
      await act(async () => {
        submission = result.current.submit(body)
      })
      if (phase === 'poll') {
        await act(async () => {
          await submission
          await vi.advanceTimersByTimeAsync(2000)
        })
      }
      act(() => result.current.forget('parent'))
      await act(async () => {
        finish(page('old', 'IN_PROGRESS'))
        await submission
      })
      expect(result.current.childByParent.parent).toBeUndefined()
      const reads = get.mock.calls.length
      await act(async () => {
        await vi.advanceTimersByTimeAsync(4000)
      })
      expect(get).toHaveBeenCalledTimes(reads)
    }
  )
  it('retains the attached task after a polling error and keeps reading until terminal', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { task_id: 'accepted' } })
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValueOnce(page('accepted', 'IN_PROGRESS'))
      .mockRejectedValueOnce(new Error('Polling unavailable'))
      .mockResolvedValue(page('accepted', 'SUCCESS'))
    const { result } = renderHook(() => useMultiViewSubmit())
    await act(async () => {
      await result.current.submit(body)
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(result.current.childByParent.parent).toMatchObject({
      task_id: 'accepted',
      status: 'IN_PROGRESS',
    })
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(result.current.childByParent.parent.status).toBe('SUCCESS')
    await act(async () => {
      await vi.advanceTimersByTimeAsync(4000)
    })
    expect(get).toHaveBeenCalledTimes(4)
    expect(get.mock.calls.at(-1)?.[1]?.params.with_artifacts).toBe(true)
    expect(post).toHaveBeenCalledTimes(1)
  })
  it('keeps the accepted ID when the first read fails and recovers with reads only', async () => {
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { task_id: 'accepted' } })
    const get = vi
      .spyOn(api, 'get')
      .mockRejectedValueOnce(new Error('Read unavailable'))
      .mockResolvedValue(page('accepted', 'IN_PROGRESS'))
    const { result } = renderHook(() => useMultiViewSubmit())
    await act(async () => {
      await expect(result.current.submit(body)).resolves.toMatchObject({
        task_id: 'accepted',
        status: 'QUEUED',
      })
    })
    expect(result.current.childByParent.parent).toMatchObject({
      task_id: 'accepted',
      status: 'QUEUED',
    })
    expect(result.current.submittingParent).toBeNull()
    await act(async () => {
      await vi.advanceTimersByTimeAsync(2000)
    })
    expect(result.current.childByParent.parent).toMatchObject({
      task_id: 'accepted',
      status: 'IN_PROGRESS',
    })
    expect(get).toHaveBeenCalledTimes(2)
    expect(get.mock.calls.at(-1)?.[1]?.params.task_id).toBe('accepted')
    expect(post).toHaveBeenCalledTimes(1)
  })
})
