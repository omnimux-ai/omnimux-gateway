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
import type { RecreateAnalysisTask } from '../api'

/** The queued decomposition is polled at this cadence. */
export const RECREATE_ANALYSIS_POLL_MS = 2000

/** The longest a decomposition may run before the caller gives up. */
export const RECREATE_ANALYSIS_MAX_MS = 8 * 60 * 1000

export type RecreatePollResult =
  | { status: 'success'; task: RecreateAnalysisTask }
  | { status: 'failed'; message?: string }
  | { status: 'timeout' }
  | { status: 'cancelled' }

export interface RecreatePollOptions {
  /** Enqueues the job exactly once. */
  start: () => Promise<{ task_id: string }>
  fetchTask: (
    taskId: string,
    signal: AbortSignal
  ) => Promise<RecreateAnalysisTask>
  signal: AbortSignal
  pollMs?: number
  maxMs?: number
  now?: () => number
  sleep?: (ms: number, signal: AbortSignal) => Promise<void>
}

/** Sleep that resolves early when the caller aborts the poll. */
export function sleepAbortable(ms: number, signal: AbortSignal): Promise<void> {
  return new Promise((resolve) => {
    if (signal.aborted) {
      resolve()
      return
    }
    const done = () => {
      clearTimeout(timer)
      signal.removeEventListener('abort', done)
      resolve()
    }
    const timer = setTimeout(done, ms)
    signal.addEventListener('abort', done, { once: true })
  })
}

/**
 * Polls a queued decomposition until it succeeds, fails, exceeds its ceiling
 * or is cancelled. The POST happens once; every later read is a GET, so a slow
 * job can never be re-enqueued by the polling itself.
 */
export async function pollRecreateAnalysis(
  options: RecreatePollOptions
): Promise<RecreatePollResult> {
  const now = options.now ?? Date.now
  const pollMs = options.pollMs ?? RECREATE_ANALYSIS_POLL_MS
  const maxMs = options.maxMs ?? RECREATE_ANALYSIS_MAX_MS
  const sleep = options.sleep ?? sleepAbortable

  if (options.signal.aborted) return { status: 'cancelled' }
  const started = await options.start()
  if (options.signal.aborted) return { status: 'cancelled' }
  const deadline = now() + maxMs

  while (!options.signal.aborted) {
    if (now() >= deadline) return { status: 'timeout' }
    await sleep(pollMs, options.signal)
    if (options.signal.aborted) return { status: 'cancelled' }
    const task = await options.fetchTask(started.task_id, options.signal)
    if (options.signal.aborted) return { status: 'cancelled' }
    if (task.status === 'failed') {
      return { status: 'failed', message: task.message }
    }
    if (task.status === 'success') return { status: 'success', task }
  }
  return { status: 'cancelled' }
}
