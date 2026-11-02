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

import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, cleanup, renderHook } from '@testing-library/react'
import { StrictMode, type ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { INFLUENCER_ACTION, INFLUENCER_MOTION_ACTION } from '../api'
import { useInfluencerHistory } from '../hooks/use-tasks'
import type { TaskRecord } from '../types'

const missing: TaskRecord = {
  id: 1,
  task_id: 'sheet-ready',
  action: INFLUENCER_ACTION,
  status: 'SUCCESS',
}
const ready: TaskRecord = {
  ...missing,
  artifacts: [{ content_url: '/result.png' }],
}
function response(items: TaskRecord[]) {
  return { data: { success: true, data: { items, total: items.length } } }
}
let client: QueryClient
function wrapper(props: { children: ReactNode }) {
  return (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
}
async function advance(ms = 1) {
  await act(async () => {
    await vi.advanceTimersByTimeAsync(ms)
  })
}
beforeEach(() => {
  vi.useFakeTimers()
  localStorage.clear()
  useAuthStore.getState().auth.setUser(null)
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
  vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true)
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})
afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.getState().auth.setUser(null)
  localStorage.clear()
  vi.restoreAllMocks()
  vi.useRealTimers()
})

describe('history read-only artifact enrichment', () => {
  it('delayed history updates recover successful missing main media without submitting generation', async () => {
    const post = vi.spyOn(api, 'post')
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValueOnce(response([missing]))
      .mockResolvedValue(response([ready]))
    const history = renderHook(
      () => useInfluencerHistory(true, INFLUENCER_ACTION),
      { wrapper }
    )
    await advance()
    expect(history.result.current.items[0]).toMatchObject(missing)
    await advance(4_000)
    expect(get).toHaveBeenCalledTimes(2)
    await advance()
    expect(history.result.current.items[0]?.artifacts).toEqual(ready.artifacts)
    expect(post).not.toHaveBeenCalled()
  })

  it('video cover still recovers after list changes cancel the original delay in StrictMode', async () => {
    const video: TaskRecord = {
      ...missing,
      action: INFLUENCER_MOTION_ACTION,
      artifacts: [{ key: 'video', content_url: '/video.mp4' }],
    }
    const get = vi
      .spyOn(api, 'get')
      .mockImplementation(async (_url, config) =>
        response(
          config?.params.task_id
            ? [{ ...video, poster_url: '/cover.png' }]
            : [video]
        )
      )
    const history = renderHook(
      () => useInfluencerHistory(true, INFLUENCER_MOTION_ACTION),
      {
        wrapper: (props) => <StrictMode>{wrapper(props)}</StrictMode>,
      }
    )
    await advance()
    await advance(2_000)
    act(() =>
      client.setQueryData(
        ['influencer', 'history', 'pages', INFLUENCER_MOTION_ACTION, undefined],
        {
          pages: [{ items: [{ ...video, progress: '100%' }], total: 1 }],
          pageParams: [1],
        }
      )
    )
    await advance(4_000)
    await advance()
    expect(history.result.current.items[0]?.poster_url).toBe('/cover.png')
    expect(
      get.mock.calls.filter(([, config]) => config?.params.task_id)
    ).toHaveLength(1)
  })

  it('staggered observers share three executed reads and a newly added task receives its own budget', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue(response([missing]))
    const first = renderHook(
      () => useInfluencerHistory(true, INFLUENCER_ACTION),
      { wrapper }
    )
    await advance()
    await advance(2_000)
    const second = renderHook(
      () => useInfluencerHistory(true, INFLUENCER_ACTION),
      { wrapper }
    )
    await advance()
    for (let round = 0; round < 3; round += 1) await advance(4_000)
    await advance(20_000)
    expect(
      get.mock.calls.filter(
        ([, config]) => config?.params.task_id === missing.task_id
      )
    ).toHaveLength(3)
    expect(first.result.current.items[0]?.status).toBe('SUCCESS')
    expect(second.result.current.items[0]?.status).toBe('SUCCESS')
    const newTask = { ...missing, id: 2, task_id: 'new-task' }
    get.mockImplementation(async (_url, config) =>
      response(
        config?.params.task_id === 'new-task'
          ? [{ ...newTask, artifacts: ready.artifacts }]
          : [missing, newTask]
      )
    )
    await act(async () => {
      await first.result.current.retry?.()
    })
    await advance()
    await advance(4_000)
    await advance()
    expect(
      first.result.current.items.find((item) => item.task_id === 'new-task')
        ?.artifacts
    ).toEqual(ready.artifacts)
    expect(
      get.mock.calls.filter(
        ([, config]) => config?.params.task_id === missing.task_id
      )
    ).toHaveLength(3)
  })

  it('hidden and offline pauses neither execute reads nor consume the later recovery budget', async () => {
    const visible = vi.spyOn(document, 'visibilityState', 'get')
    const online = vi.spyOn(navigator, 'onLine', 'get')
    const get = vi.spyOn(api, 'get').mockResolvedValue(response([missing]))
    renderHook(() => useInfluencerHistory(true, INFLUENCER_ACTION), { wrapper })
    await advance()
    await advance(2_000)
    visible.mockReturnValue('hidden')
    act(() => document.dispatchEvent(new Event('visibilitychange')))
    await advance(20_000)
    expect(
      get.mock.calls.filter(([, config]) => config?.params.task_id)
    ).toHaveLength(0)
    visible.mockReturnValue('visible')
    online.mockReturnValue(false)
    act(() => document.dispatchEvent(new Event('visibilitychange')))
    await advance(20_000)
    expect(
      get.mock.calls.filter(([, config]) => config?.params.task_id)
    ).toHaveLength(0)
    online.mockReturnValue(true)
    act(() => window.dispatchEvent(new Event('online')))
    for (let round = 0; round < 3; round += 1) await advance(4_000)
    await advance(20_000)
    expect(
      get.mock.calls.filter(([, config]) => config?.params.task_id)
    ).toHaveLength(3)
  })

  it('all observers leaving cancels only unexecuted work and remount retains three usable reads', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue(response([missing]))
    const first = renderHook(
      () => useInfluencerHistory(true, INFLUENCER_ACTION),
      { wrapper }
    )
    await advance()
    await advance(2_000)
    first.unmount()
    await advance(20_000)
    expect(
      get.mock.calls.filter(([, config]) => config?.params.task_id)
    ).toHaveLength(0)
    renderHook(() => useInfluencerHistory(true, INFLUENCER_ACTION), { wrapper })
    await advance()
    for (let round = 0; round < 3; round += 1) await advance(4_000)
    await advance(20_000)
    expect(
      get.mock.calls.filter(([, config]) => config?.params.task_id)
    ).toHaveLength(3)
  })

  it('disabling history cancels a scheduled read without spending budget', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue(response([missing]))
    const history = renderHook(
      (enabled) => useInfluencerHistory(enabled, INFLUENCER_ACTION),
      { wrapper, initialProps: true }
    )
    await advance()
    await advance(2_000)
    history.rerender(false)
    await advance(20_000)
    expect(
      get.mock.calls.filter(([, config]) => config?.params.task_id)
    ).toHaveLength(0)
    history.rerender(true)
    await advance()
    for (let round = 0; round < 3; round += 1) await advance(4_000)
    await advance(20_000)
    expect(
      get.mock.calls.filter(([, config]) => config?.params.task_id)
    ).toHaveLength(3)
  })

  it('remounting an exhausted task does not grant another automatic budget', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue(response([missing]))
    const first = renderHook(
      () => useInfluencerHistory(true, INFLUENCER_ACTION),
      { wrapper }
    )
    await advance()
    for (let round = 0; round < 3; round += 1) await advance(4_000)
    first.unmount()
    await advance()
    renderHook(() => useInfluencerHistory(true, INFLUENCER_ACTION), { wrapper })
    await advance()
    await advance(20_000)
    expect(
      get.mock.calls.filter(([, config]) => config?.params.task_id)
    ).toHaveLength(3)
  })

  it('preserves the confirmed legacy media when cover enrichment supplies a higher-priority fallback', async () => {
    const video: TaskRecord = {
      ...missing,
      action: INFLUENCER_MOTION_ACTION,
      result_url: '/confirmed.mp4',
    }
    vi.spyOn(api, 'get')
      .mockResolvedValueOnce(response([video]))
      .mockResolvedValue(
        response([
          {
            ...video,
            legacy_content_url: '/stale.mp4',
            poster_url: '/cover.jpg',
          },
        ])
      )
    const history = renderHook(
      () => useInfluencerHistory(true, INFLUENCER_MOTION_ACTION),
      { wrapper }
    )
    await advance()
    await advance(4000)
    await advance()
    expect(history.result.current.items[0]?.legacy_content_url).toBeUndefined()
    expect(history.result.current.items[0]?.result_url).toBe('/confirmed.mp4')
    expect(history.result.current.items[0]?.poster_url).toBe('/cover.jpg')
  })

  it('complete media and confirmed failure never schedule enrichment', async () => {
    const failed = { ...missing, id: 2, task_id: 'failed', status: 'FAILURE' }
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValue(response([ready, failed]))
    renderHook(() => useInfluencerHistory(true, INFLUENCER_ACTION), { wrapper })
    await advance()
    await advance(20_000)
    expect(get).toHaveBeenCalledTimes(1)
  })

  it('read rejection, business failure and wrong task exhaust exactly three reads without changing success', async () => {
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValueOnce(response([missing]))
      .mockRejectedValueOnce(new Error('Read unavailable'))
      .mockResolvedValueOnce({
        data: { success: false, message: 'Artifact unavailable' },
      })
      .mockResolvedValue(response([{ ...ready, task_id: 'wrong-task' }]))
    const history = renderHook(
      () => useInfluencerHistory(true, INFLUENCER_ACTION),
      { wrapper }
    )
    await advance()
    for (let round = 0; round < 3; round += 1) await advance(4_000)
    await advance(20_000)
    expect(
      get.mock.calls.filter(([, config]) => config?.params.task_id)
    ).toHaveLength(3)
    expect(history.result.current.items[0]).toEqual(missing)
    expect(history.result.current.error).toBeNull()
  })

  it('partial video enrichment preserves confirmed main media and metadata while adding cover', async () => {
    const video = {
      ...missing,
      action: INFLUENCER_MOTION_ACTION,
      artifacts: [{ key: 'video', content_url: '/confirmed.mp4' }],
      properties: { input: '{"brief":"current"}' },
    }
    vi.spyOn(api, 'get').mockImplementation(async (_url, config) =>
      response(
        config?.params.task_id
          ? [
              {
                ...video,
                artifacts: [],
                properties: {},
                poster_url: '/cover.png',
              },
            ]
          : [video]
      )
    )
    const history = renderHook(
      () => useInfluencerHistory(true, INFLUENCER_MOTION_ACTION),
      { wrapper }
    )
    await advance()
    await advance(4_000)
    await advance()
    expect(history.result.current.items[0]).toEqual({
      ...video,
      poster_url: '/cover.png',
    })
  })

  it('account change isolates delayed reads and lets the new account recover the same task ID', async () => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'old', role: 1 })
    let finish!: (value: ReturnType<typeof response>) => void
    const late = new Promise<ReturnType<typeof response>>((resolve) => {
      finish = resolve
    })
    vi.spyOn(api, 'get').mockImplementation(async (_url, config) => {
      if (useAuthStore.getState().auth.user?.id === 1) {
        return config?.params.task_id ? late : response([missing])
      }
      return response(
        config?.params.task_id
          ? [{ ...ready, result_url: '/new-account.png' }]
          : [missing]
      )
    })
    const history = renderHook(
      () => useInfluencerHistory(true, INFLUENCER_ACTION),
      { wrapper }
    )
    await advance()
    await advance(4_000)
    act(() =>
      useAuthStore.getState().auth.setUser({ id: 2, username: 'new', role: 1 })
    )
    await advance()
    await act(async () =>
      finish(response([{ ...ready, result_url: '/old-account.png' }]))
    )
    await advance()
    expect(history.result.current.items[0]?.artifacts).toBeUndefined()
    await advance(4_000)
    await advance()
    expect(history.result.current.items[0]?.result_url).toBe('/new-account.png')
  })

  it('real Axios GET boundary shares observers without attaching a cancelling transport signal', async () => {
    const adapter = api.defaults.adapter
    const requests: { taskId: string | undefined; signal: unknown }[] = []
    api.defaults.adapter = async (config) => {
      requests.push({ taskId: config.params.task_id, signal: config.signal })
      return {
        config,
        status: 200,
        statusText: 'OK',
        headers: {},
        ...response(config.params.task_id ? [ready] : [missing]),
      }
    }
    try {
      const first = renderHook(
        () => useInfluencerHistory(true, INFLUENCER_ACTION),
        { wrapper }
      )
      await advance()
      await advance(2_000)
      const second = renderHook(
        () => useInfluencerHistory(true, INFLUENCER_ACTION),
        { wrapper }
      )
      await advance()
      first.unmount()
      await advance(4_000)
      await advance()
      expect(second.result.current.items[0]?.artifacts).toEqual(ready.artifacts)
      expect(requests.filter((request) => request.taskId)).toEqual([
        { taskId: missing.task_id, signal: undefined },
      ])
    } finally {
      api.defaults.adapter = adapter
    }
  })

  it('late success cannot replace a newer confirmed failure', async () => {
    let finish!: (value: ReturnType<typeof response>) => void
    const late = new Promise<ReturnType<typeof response>>((resolve) => {
      finish = resolve
    })
    const get = vi
      .spyOn(api, 'get')
      .mockResolvedValueOnce(response([missing]))
      .mockReturnValue(late)
    const history = renderHook(
      () => useInfluencerHistory(true, INFLUENCER_ACTION),
      { wrapper }
    )
    await advance()
    await advance(4_000)
    act(() =>
      client.setQueryData(
        ['influencer', 'history', 'pages', INFLUENCER_ACTION, undefined],
        {
          pages: [{ items: [{ ...missing, status: 'FAILURE' }], total: 1 }],
          pageParams: [1],
        }
      )
    )
    await advance()
    await act(async () => finish(response([ready])))
    await advance()
    expect(history.result.current.items[0]?.status).toBe('FAILURE')
    await advance(20_000)
    expect(
      get.mock.calls.filter(([, config]) => config?.params.task_id)
    ).toHaveLength(1)
  })
})
