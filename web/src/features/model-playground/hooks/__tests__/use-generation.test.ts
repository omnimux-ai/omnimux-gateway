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
import { afterEach, describe, expect, test, vi } from 'vitest'

import { useGeneration } from '../use-generation'
import type { GenerationTask, GenerationTransport } from '../../types'

const POLL_MS = 20

function task(overrides: Partial<GenerationTask> = {}): GenerationTask {
  return {
    taskId: 'task_1',
    modelId: 'seedance-2-5',
    status: 'running',
    progress: 10,
    params: {},
    estimatedUsd: null,
    settledUsd: null,
    createdAt: 1,
    media: [],
    ...overrides,
  }
}

/** Hand-written fake: only the seam the hook actually calls. */
function fakeTransport(overrides: Partial<GenerationTransport> = {}): GenerationTransport {
  return {
    mocked: false,
    uploadReference: async () => ({ url: 'https://cdn.example/a.png' }),
    submit: async () => ({ taskId: 'task_1' }),
    getTask: async () => task(),
    listHistory: async () => [],
    ...overrides,
  }
}

afterEach(() => {
  vi.useRealTimers()
})

describe('useGeneration failure reporting', () => {
  test('exposes a rejected submission and stops the submitting state', async () => {
    const failure = new Error('No available channel')
    const transport = fakeTransport({
      submit: async () => {
        throw failure
      },
    })
    const { result } = renderHook(() => useGeneration({ transport }))

    await act(async () => {
      await result.current.submit('seedance-2-5', { prompt: 'x' })
    })

    expect(result.current.error).toBe(failure)
    expect(result.current.submitting).toBe(false)
    expect(result.current.task).toBeNull()
  })

  test('clears the previous failure when the user submits again', async () => {
    const transport = fakeTransport({
      submit: async () => {
        throw new Error('No available channel')
      },
    })
    const { result, rerender } = renderHook(
      (props: { transport: GenerationTransport }) =>
        useGeneration({ transport: props.transport }),
      { initialProps: { transport } }
    )

    await act(async () => {
      await result.current.submit('seedance-2-5', {})
    })
    expect(result.current.error).toBeTruthy()

    rerender({ transport: fakeTransport() })
    await act(async () => {
      await result.current.submit('seedance-2-5', {})
    })

    expect(result.current.error).toBeNull()
    expect(result.current.task?.taskId).toBe('task_1')
  })

  test('clears the failure on reset', async () => {
    const transport = fakeTransport({
      submit: async () => {
        throw new Error('No available channel')
      },
    })
    const { result } = renderHook(() => useGeneration({ transport }))

    await act(async () => {
      await result.current.submit('seedance-2-5', {})
    })
    act(() => {
      result.current.reset()
    })

    expect(result.current.error).toBeNull()
  })

  test('surfaces a polling failure and stops the poll instead of freezing', async () => {
    vi.useFakeTimers()
    const failure = new Error('Task is not readable')
    let calls = 0
    const transport = fakeTransport({
      getTask: async () => {
        calls += 1
        if (calls === 1) return task()
        throw failure
      },
    })
    const { result } = renderHook(() =>
      useGeneration({ transport, pollIntervalMs: POLL_MS })
    )

    await act(async () => {
      await result.current.submit('seedance-2-5', {})
    })

    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_MS * 3)
    })

    expect(result.current.error).toBe(failure)

    const callsAfterFailure = calls
    await act(async () => {
      await vi.advanceTimersByTimeAsync(POLL_MS * 3)
    })
    expect(calls).toBe(callsAfterFailure)
  })

  test('reports nothing while a generation completes normally', async () => {
    const transport = fakeTransport()
    const { result } = renderHook(() => useGeneration({ transport }))

    await act(async () => {
      await result.current.submit('seedance-2-5', {})
    })

    await waitFor(() => {
      expect(result.current.task?.taskId).toBe('task_1')
    })
    expect(result.current.error).toBeNull()
  })
})
