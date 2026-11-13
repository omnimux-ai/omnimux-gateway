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
import { renderHook, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { describe, expect, test, vi } from 'vitest'

import type { GenerationTask, GenerationTransport } from '../../types'
import { usePlaygroundHistory } from '../use-playground-history'

const MODEL_ID = 'gpt-image-2.5-flare'

function historyTask(): GenerationTask {
  return {
    taskId: 'task_1',
    modelId: MODEL_ID,
    status: 'succeeded',
    progress: 100,
    params: { prompt: 'a red fox' },
    estimatedUsd: null,
    settledUsd: 0.014706,
    createdAt: 1,
    media: [],
  }
}

function fakeTransport(
  listHistory: GenerationTransport['listHistory']
): GenerationTransport {
  return {
    mocked: false,
    uploadReference: async () => ({ url: 'https://cdn.example/a.png' }),
    submit: async () => ({ taskId: 'task_1' }),
    getTask: async () => historyTask(),
    listHistory,
  }
}

function renderHistory(authenticated: boolean, transport: GenerationTransport) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return renderHook(
    () => usePlaygroundHistory(transport, MODEL_ID, null, authenticated),
    {
      wrapper: ({ children }: { children: ReactNode }) =>
        createElement(QueryClientProvider, { client }, children),
    }
  )
}

describe('usePlaygroundHistory viewer scope', () => {
  test('does not request the per-user list for a signed-out viewer', async () => {
    const listHistory = vi.fn(async () => [historyTask()])
    const { result } = renderHistory(false, fakeTransport(listHistory))

    await waitFor(() => {
      expect(result.current.loading).toBe(false)
    })

    // The model page is public; an unauthorized list here is what answers 401
    // and turns into a sign-in redirect for a visitor who never signed in.
    expect(listHistory).not.toHaveBeenCalled()
    expect(result.current.tasks).toEqual([])
  })

  test('loads the list for a signed-in viewer', async () => {
    const listHistory = vi.fn(async () => [historyTask()])
    const { result } = renderHistory(true, fakeTransport(listHistory))

    await waitFor(() => {
      expect(result.current.tasks).toHaveLength(1)
    })

    expect(listHistory).toHaveBeenCalledWith(MODEL_ID)
    expect(result.current.tasks[0].taskId).toBe('task_1')
  })
})
