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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect } from 'react'

import { isTerminal } from '../transport/mock-transport'
import type { GenerationTask, GenerationTransport } from '../types'

export type UsePlaygroundHistoryResult = {
  tasks: GenerationTask[]
  loading: boolean
  refresh: () => void
}

/**
 * Cached history for one model backed by TanStack Query.
 *
 * Keeps a 30s stale time and 5m gc time so switching between model tabs or
 * scrolling back to playground delivers an instant response without redundant network requests.
 * Reloads automatically whenever a submitted generation task reaches a terminal state.
 *
 * The list belongs to the signed-in user, so it is only fetched for one. The
 * model page itself is public: an unauthorized history fetch answers 401, which
 * the shared HTTP client reads as an expired session and turns into a sign-in
 * redirect — an optional panel must not throw a visitor off a public page.
 */
export function usePlaygroundHistory(
  transport: GenerationTransport,
  modelId: string,
  finishedTask: GenerationTask | null,
  authenticated: boolean
): UsePlaygroundHistoryResult {
  const queryClient = useQueryClient()

  const finishedId =
    finishedTask && isTerminal(finishedTask.status)
      ? finishedTask.taskId
      : null

  const queryKey = ['playground-history', modelId]

  const { data: tasks = [], isFetching, refetch } = useQuery({
    queryKey,
    queryFn: async () => {
      if (!modelId) return []
      return await transport.listHistory(modelId)
    },
    enabled: authenticated && Boolean(modelId),
    // An optional panel: when the list cannot be loaded the reader keeps the
    // model page and sees an empty history, rather than losing the page.
    meta: { fullPageError: false },
    staleTime: 30 * 1000,
    gcTime: 5 * 60 * 1000,
  })

  useEffect(() => {
    if (finishedId) {
      void queryClient.invalidateQueries({ queryKey })
    }
  }, [finishedId, modelId, queryClient])

  const refresh = useCallback(() => {
    void refetch()
  }, [refetch])

  return { tasks, loading: isFetching, refresh }
}
