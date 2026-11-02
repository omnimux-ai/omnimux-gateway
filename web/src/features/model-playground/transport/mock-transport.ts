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
import type {
  GenerationTask,
  GenerationTransport,
  SubmitRequest,
  TaskMedia,
  TaskStatus,
  UploadResult,
} from '../types'

/**
 * Debug switch read from the submitted parameters.
 *
 * The mock only — the gateway transport ignores it. Lets the page (and tests)
 * reach the failed / timeout terminal states without waiting on a real upstream.
 */
export const MOCK_OUTCOME_PARAM = 'debug_outcome'

export type MockTransportOptions = {
  /** Injected clock so tests stay deterministic. */
  now?: () => number
  /** Wall time a simulated task takes to succeed. */
  runDurationMs?: number
  /** Wall time after which a `timeout` task reports timeout. */
  timeoutAfterMs?: number
}

const DEFAULT_RUN_MS = 6000
const DEFAULT_TIMEOUT_MS = 12000

const MOCK_VIDEO_URL =
  'https://commondatastorage.googleapis.com/gtv-videos-bucket/sample/ForBiggerBlazes.mp4'

function imageUrlFor(taskId: string): string {
  return `https://picsum.photos/seed/${encodeURIComponent(taskId)}/1024/1024`
}

function mediaFor(taskId: string, kind: 'image' | 'video'): TaskMedia[] {
  return [
    {
      url: kind === 'video' ? MOCK_VIDEO_URL : imageUrlFor(taskId),
      kind,
    },
  ]
}

/**
 * Simulated execution gateway.
 *
 * Task state is a pure function of elapsed time, so it advances without timers:
 * queued → running (progress grows) → succeeded, or the failed / timeout
 * terminal state when MOCK_OUTCOME_PARAM asks for it.
 */
export class MockTransport implements GenerationTransport {
  readonly mocked = true

  private readonly now: () => number
  private readonly runDurationMs: number
  private readonly timeoutAfterMs: number
  private readonly tasks = new Map<string, GenerationTask>()
  private sequence = 0

  constructor(options: MockTransportOptions = {}) {
    this.now = options.now ?? (() => Date.now())
    this.runDurationMs = options.runDurationMs ?? DEFAULT_RUN_MS
    this.timeoutAfterMs = options.timeoutAfterMs ?? DEFAULT_TIMEOUT_MS
  }

  async uploadReference(file: File): Promise<UploadResult> {
    // No server in mock mode; a blob URL is enough to preview the reference.
    const creatable =
      typeof URL !== 'undefined' &&
      typeof URL.createObjectURL === 'function'
    return { url: creatable ? URL.createObjectURL(file) : '' }
  }

  async submit(request: SubmitRequest): Promise<{ taskId: string }> {
    this.sequence += 1
    const taskId = `mock-task-${this.sequence}`
    this.tasks.set(taskId, {
      taskId,
      modelId: request.modelId,
      status: 'queued',
      progress: 0,
      params: { ...request.params },
      estimatedUsd: null,
      settledUsd: null,
      createdAt: this.now(),
      media: [],
    })
    return { taskId }
  }

  async getTask(taskId: string): Promise<GenerationTask> {
    const stored = this.tasks.get(taskId)
    if (!stored) {
      throw new Error(`Unknown mock task: ${taskId}`)
    }
    const advanced = this.advance(stored)
    this.tasks.set(taskId, advanced)
    return { ...advanced }
  }

  async listHistory(modelId: string): Promise<GenerationTask[]> {
    return [...this.tasks.values()]
      .map((task) => {
        const advanced = this.advance(task)
        this.tasks.set(task.taskId, advanced)
        return { ...advanced }
      })
      .filter((task) => task.modelId === modelId)
      .sort((a, b) => b.createdAt - a.createdAt)
  }

  private advance(task: GenerationTask): GenerationTask {
    if (isTerminal(task.status)) return task

    const elapsed = Math.max(0, this.now() - task.createdAt)
    const outcome = task.params[MOCK_OUTCOME_PARAM]
    const ratio = Math.min(1, elapsed / this.runDurationMs)
    const progress = Math.min(99, Math.floor(ratio * 100))

    if (outcome === 'failed' && ratio >= 0.5) {
      return {
        ...task,
        status: 'failed',
        progress,
        finishedAt: this.now(),
        failReason: 'Mock task failed. No charge was applied.',
      }
    }

    if (outcome === 'timeout' && elapsed >= this.timeoutAfterMs) {
      return {
        ...task,
        status: 'timeout',
        progress,
        finishedAt: this.now(),
        failReason: 'Mock task timed out. No charge was applied.',
      }
    }

    if (ratio >= 1 && outcome !== 'failed' && outcome !== 'timeout') {
      return {
        ...task,
        status: 'succeeded',
        progress: 100,
        finishedAt: this.now(),
        settledUsd: task.estimatedUsd,
        media: mediaFor(task.taskId, this.mediaKind(task)),
      }
    }

    // A task stays queued until the simulated worker actually picks it up.
    if (progress === 0) {
      return { ...task, status: 'queued', progress: 0 }
    }

    return { ...task, status: 'running', progress }
  }

  private mediaKind(task: GenerationTask): 'image' | 'video' {
    return task.params.output_format === 'mp4' ||
      task.params.output_format === 'mov' ||
      task.params.duration !== undefined
      ? 'video'
      : 'image'
  }
}

export function isTerminal(status: TaskStatus): boolean {
  return status === 'succeeded' || status === 'failed' || status === 'timeout'
}
