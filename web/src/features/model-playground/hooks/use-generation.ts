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
import { useCallback, useEffect, useRef, useState } from 'react'

import { isTerminal } from '../transport/mock-transport'
import type {
  GenerationTask,
  GenerationTransport,
  PlaygroundModality,
  PlaygroundParams,
} from '../types'

const DEFAULT_POLL_INTERVAL_MS = 2000

export type UseGenerationOptions = {
  transport: GenerationTransport
  /** Resolves the pre-submit estimate so the task card can show the quote. */
  resolveEstimate?: (params: PlaygroundParams) => number | null
  pollIntervalMs?: number
}

export type UseGenerationResult = {
  task: GenerationTask | null
  submitting: boolean
  /**
   * Raw failure from the last submit or poll, or null. Kept untranslated so the
   * rendering component can resolve it at paint time — a message resolved here
   * would not follow a language switch.
   */
  error: unknown
  submit: (
    modelId: string,
    params: PlaygroundParams,
    modality?: PlaygroundModality,
    /** Token group to pin, when the caller resolved one. */
    group?: string
  ) => Promise<void>
  reset: () => void
}

/**
 * Submit + poll a generation task until it reaches a terminal state.
 *
 * Polling is the documented contract for the gateway task API, so the interval
 * is the single source of progress updates; it is always cleared on unmount and
 * on reset.
 */
export function useGeneration(
  options: UseGenerationOptions
): UseGenerationResult {
  const { transport, resolveEstimate, pollIntervalMs } = options
  const [task, setTask] = useState<GenerationTask | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const mountedRef = useRef(true)

  const clearTimer = useCallback(() => {
    if (timerRef.current !== null) {
      clearInterval(timerRef.current)
      timerRef.current = null
    }
  }, [])

  useEffect(() => {
    mountedRef.current = true
    return () => {
      mountedRef.current = false
      clearTimer()
    }
  }, [clearTimer])

  const reset = useCallback(() => {
    clearTimer()
    setTask(null)
    setSubmitting(false)
    setError(null)
  }, [clearTimer])

  const submit = useCallback(
    async (
      modelId: string,
      params: PlaygroundParams,
      modality?: PlaygroundModality,
      group?: string
    ) => {
      clearTimer()
      setSubmitting(true)
      setError(null)
      const estimatedUsd = resolveEstimate ? resolveEstimate(params) : null
      try {
        const { taskId } = await transport.submit({
          modelId,
          params,
          modality,
          group,
        })
        const created = await transport.getTask(taskId)
        if (!mountedRef.current) return
        setTask({ ...created, estimatedUsd })
        setSubmitting(false)

        timerRef.current = setInterval(() => {
          void (async () => {
            try {
              const next = await transport.getTask(taskId)
              if (!mountedRef.current) return
              setTask({ ...next, estimatedUsd })
              if (isTerminal(next.status)) clearTimer()
            } catch (pollError) {
              // Polling is the only progress source, so a failure ends the task
              // view and must be reported rather than silently frozen.
              clearTimer()
              if (mountedRef.current) setError(pollError)
            }
          })()
        }, pollIntervalMs ?? DEFAULT_POLL_INTERVAL_MS)
      } catch (submitError) {
        setSubmitting(false)
        clearTimer()
        if (mountedRef.current) setError(submitError)
      }
    },
    [clearTimer, pollIntervalMs, resolveEstimate, transport]
  )

  return { task, submitting, error, submit, reset }
}
