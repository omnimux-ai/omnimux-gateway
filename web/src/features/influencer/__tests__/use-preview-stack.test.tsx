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

import {
  fetchTaskRecord,
  INFLUENCER_ACTION,
  INFLUENCER_MOTION_ACTION,
} from '../api'
import {
  usePreviewStack,
  type UsePreviewStackOptions,
} from '../hooks/use-preview-stack'
import type { PreviewEntry } from '../lib/preview-reference'
import type { UploadRecord } from '../lib/upload-library'
import type { TaskRecord } from '../types'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    fetchTaskRecord: vi.fn(),
  }
})

const fetchTaskRecordMock = vi.mocked(fetchTaskRecord)

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

const SHEET_TASK: TaskRecord = {
  id: 1,
  task_id: 'sheet-1',
  action: INFLUENCER_ACTION,
  status: 'SUCCESS',
  artifacts: [
    {
      key: 'image',
      content_url:
        'https://gw.example/v1/tasks/sheet-1/artifacts/image/content',
    },
  ],
}

const OFF_PAGE_SLOW: TaskRecord = {
  id: 2,
  task_id: 'task-slow',
  action: INFLUENCER_ACTION,
  status: 'SUCCESS',
  artifacts: [
    {
      key: 'image',
      content_url:
        'https://gw.example/v1/tasks/task-slow/artifacts/image/content',
    },
  ],
}

const OFF_PAGE_FAST: TaskRecord = {
  id: 3,
  task_id: 'task-fast',
  action: INFLUENCER_MOTION_ACTION,
  status: 'SUCCESS',
  artifacts: [
    {
      key: 'video',
      content_url:
        'https://gw.example/v1/tasks/task-fast/artifacts/video/content',
    },
  ],
}

const MULTIVIEW_TASK: TaskRecord = {
  id: 4,
  task_id: 'mv-board',
  action: INFLUENCER_ACTION,
  status: 'SUCCESS',
  properties: {
    input: JSON.stringify({ kind: 'multiview', parent_task_id: 'sheet-1' }),
  },
}

const UPLOADED_IMAGE: UploadRecord = {
  kind: 'image',
  url: 'https://files.example/uploads/face.png',
  fileName: 'face.png',
  uploadedAt: 1,
  expiresAt: Date.UTC(2026, 11, 31),
}

const UPLOADED_VIDEO: UploadRecord = {
  kind: 'video',
  url: 'https://files.example/uploads/clip.mp4',
  fileName: 'clip.mp4',
  uploadedAt: 2,
  expiresAt: Date.UTC(2026, 11, 31),
}

function createOptions(
  overrides: Partial<UsePreviewStackOptions> = {}
): UsePreviewStackOptions {
  return {
    accountId: 1,
    historyItems: [],
    uploadedImages: [],
    uploadedVideos: [],
    hiddenIds: new Set(),
    ...overrides,
  }
}

beforeEach(() => {
  fetchTaskRecordMock.mockReset()
})

afterEach(() => {
  vi.clearAllMocks()
})

describe('usePreviewStack', () => {
  it('keeps only the latest fetchTaskRecord response when async requests resolve out of order', async () => {
    const slow = createDeferred<TaskRecord | null>()
    const fast = createDeferred<TaskRecord | null>()
    fetchTaskRecordMock
      .mockImplementationOnce(() => slow.promise)
      .mockImplementationOnce(() => fast.promise)

    const { result } = renderHook((props) => usePreviewStack(props), {
      initialProps: createOptions(),
    })

    act(() => {
      result.current.onPreviewImage(
        'https://gw.example/v1/tasks/task-slow/artifacts/image/content'
      )
      result.current.onPreviewVideo(
        'https://gw.example/v1/tasks/task-fast/artifacts/video/content'
      )
    })

    expect(fetchTaskRecordMock).toHaveBeenNthCalledWith(1, 'task-slow', {
      action: undefined,
      with_artifacts: true,
    })
    expect(fetchTaskRecordMock).toHaveBeenNthCalledWith(2, 'task-fast', {
      action: INFLUENCER_MOTION_ACTION,
      with_artifacts: true,
    })

    await act(async () => {
      fast.resolve(OFF_PAGE_FAST)
      await fast.promise
    })
    expect(result.current.viewStack).toEqual([
      { kind: 'task', task: OFF_PAGE_FAST, readOnly: false },
    ])

    // Late resolution of the superseded first request must be discarded.
    await act(async () => {
      slow.resolve(OFF_PAGE_SLOW)
      await slow.promise
    })
    expect(result.current.viewStack).toEqual([
      { kind: 'task', task: OFF_PAGE_FAST, readOnly: false },
    ])
  })

  it('hides the previous account preview on the very first render after accountId changes and drops in-flight reads', async () => {
    const pending = createDeferred<TaskRecord | null>()
    fetchTaskRecordMock.mockImplementationOnce(() => pending.promise)

    const observedByAccount = new Map<number | undefined, PreviewEntry[][]>()
    const { result, rerender } = renderHook(
      (props) => {
        const value = usePreviewStack(props)
        const list = observedByAccount.get(props.accountId) ?? []
        list.push(value.viewStack)
        observedByAccount.set(props.accountId, list)
        return value
      },
      {
        initialProps: createOptions({
          accountId: 1,
          historyItems: [SHEET_TASK],
        }),
      }
    )

    act(() => {
      result.current.setViewTask(SHEET_TASK)
      result.current.onPreviewImage(
        'https://gw.example/v1/tasks/task-slow/artifacts/image/content'
      )
    })
    expect(result.current.viewStack).toEqual([
      { kind: 'task', task: SHEET_TASK },
    ])

    // Switch to account 2: even the very first render before effects must see an empty stack.
    rerender(createOptions({ accountId: 2, historyItems: [] }))
    expect(observedByAccount.get(2)?.[0]).toEqual([])
    expect(result.current.viewStack).toEqual([])

    // Account 1's in-flight fetch resolves after the switch: must be dropped.
    await act(async () => {
      pending.resolve(OFF_PAGE_SLOW)
      await pending.promise
    })
    expect(result.current.viewStack).toEqual([])
  })

  it('opens direct workbench reference previews immediately after switching accounts without clicking FeedGrid first', () => {
    const { result, rerender } = renderHook((props) => usePreviewStack(props), {
      initialProps: createOptions({
        accountId: 1,
        historyItems: [SHEET_TASK],
      }),
    })

    // Switch account from 1 -> 2 without ever calling setViewTask for account 2.
    rerender(
      createOptions({
        accountId: 2,
        uploadedImages: [UPLOADED_IMAGE],
        uploadedVideos: [UPLOADED_VIDEO],
      })
    )
    expect(result.current.viewStack).toEqual([])

    act(() => {
      result.current.onPreviewImage(UPLOADED_IMAGE.url)
    })
    expect(result.current.viewStack).toEqual([
      { kind: 'upload', record: UPLOADED_IMAGE },
    ])

    act(() => {
      result.current.onPreviewVideo(UPLOADED_VIDEO.url)
    })
    expect(result.current.viewStack).toEqual([
      { kind: 'upload', record: UPLOADED_IMAGE },
      { kind: 'upload', record: UPLOADED_VIDEO },
    ])

    act(() => {
      result.current.onPreviewBack()
    })
    expect(result.current.viewStack).toEqual([
      { kind: 'upload', record: UPLOADED_IMAGE },
    ])

    act(() => {
      result.current.closePreview()
    })
    expect(result.current.viewStack).toEqual([])
  })

  it('projects live history updates, marks derived/hidden references read-only, and falls back to unknown on fetch failure', async () => {
    fetchTaskRecordMock
      .mockRejectedValueOnce(new Error('not found'))
      .mockResolvedValueOnce(MULTIVIEW_TASK)

    const initialTask: TaskRecord = { ...SHEET_TASK, artifacts: [] }
    const { result, rerender } = renderHook((props) => usePreviewStack(props), {
      initialProps: createOptions({
        accountId: 1,
        historyItems: [initialTask],
        hiddenIds: new Set(['sheet-1']),
      }),
    })

    act(() => {
      result.current.setViewTask(initialTask)
    })
    expect(result.current.viewStack).toEqual([
      { kind: 'task', task: initialTask },
    ])

    // Enriching history updates the open task in place.
    rerender(
      createOptions({
        accountId: 1,
        historyItems: [SHEET_TASK],
        hiddenIds: new Set(['sheet-1']),
      })
    )
    expect(result.current.viewStack).toEqual([
      { kind: 'task', task: SHEET_TASK },
    ])

    // Opening a hidden task as a reference marks it read-only.
    act(() => {
      result.current.onPreviewImage(
        'https://gw.example/v1/tasks/sheet-1/artifacts/image/content'
      )
    })
    expect(result.current.viewStack[1]).toEqual({
      kind: 'task',
      task: SHEET_TASK,
      readOnly: true,
    })

    // Failed fetch falls back to unknown without losing existing layers.
    await act(async () => {
      result.current.onPreviewImage(
        'https://gw.example/v1/tasks/missing-id/artifacts/image/content'
      )
      await Promise.resolve()
    })
    expect(result.current.viewStack.at(-1)).toEqual({
      kind: 'unknown',
      url: 'https://gw.example/v1/tasks/missing-id/artifacts/image/content',
      media: 'image',
    })

    // MultiView board fetched off-page is marked read-only.
    await act(async () => {
      result.current.onPreviewImage(
        'https://gw.example/v1/tasks/mv-board/artifacts/image/content'
      )
      await Promise.resolve()
    })
    expect(result.current.viewStack.at(-1)).toEqual({
      kind: 'task',
      task: MULTIVIEW_TASK,
      readOnly: true,
    })
  })

  it('cancels pending fetchTaskRecord results on closePreview, onPreviewBack, and mismatched task_id', async () => {
    const cancelledByClose = createDeferred<TaskRecord | null>()
    const mismatched = createDeferred<TaskRecord | null>()
    fetchTaskRecordMock
      .mockImplementationOnce(() => cancelledByClose.promise)
      .mockImplementationOnce(() => mismatched.promise)

    const { result } = renderHook((props) => usePreviewStack(props), {
      initialProps: createOptions({ accountId: 1 }),
    })

    act(() => {
      result.current.onPreviewImage(
        'https://gw.example/v1/tasks/task-slow/artifacts/image/content'
      )
      result.current.closePreview()
    })

    await act(async () => {
      cancelledByClose.resolve(OFF_PAGE_SLOW)
      await cancelledByClose.promise
    })
    expect(result.current.viewStack).toEqual([])

    const url =
      'https://gw.example/v1/tasks/expected-id/artifacts/image/content'
    act(() => {
      result.current.onPreviewImage(url)
    })
    await act(async () => {
      mismatched.resolve({ ...OFF_PAGE_SLOW, task_id: 'different-id' })
      await mismatched.promise
    })
    expect(result.current.viewStack).toEqual([
      { kind: 'unknown', url, media: 'image' },
    ])
  })
})
