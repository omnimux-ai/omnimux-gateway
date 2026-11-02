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
import { describe, expect, it, vi } from 'vitest'

import type { RecreateAnalysisTask } from '../api'
import { pollRecreateAnalysis } from '../lib/recreate-poll'

const RUNNING: RecreateAnalysisTask = {
  task_id: 'task-1',
  status: 'running',
  progress: '40%',
}

const SUCCESS: RecreateAnalysisTask = {
  task_id: 'task-1',
  status: 'success',
  prompt: 'Reference image 1 is the hero product.',
  analysis: { video_overview: { one_line: 'A single continuous take.' } },
  image_analysis: [],
  meta: { duration_sec: 8, pricing: { '480p': 6.47, '720p': 13.89 } },
}

/** A deterministic clock: each poll advances the clock by its interval. */
function clock() {
  let value = 0
  return {
    now: () => value,
    sleep: async (ms: number) => {
      value += ms
    },
  }
}

describe('recreate analysis polling', () => {
  it('enqueues once, then reads running → success', async () => {
    const time = clock()
    const start = vi.fn().mockResolvedValue({ task_id: 'task-1' })
    const fetchTask = vi
      .fn()
      .mockResolvedValueOnce(RUNNING)
      .mockResolvedValueOnce(SUCCESS)

    const result = await pollRecreateAnalysis({
      start,
      fetchTask,
      signal: new AbortController().signal,
      pollMs: 2000,
      maxMs: 480000,
      now: time.now,
      sleep: time.sleep,
    })

    expect(result).toEqual({ status: 'success', task: SUCCESS })
    expect(start).toHaveBeenCalledTimes(1)
    expect(fetchTask).toHaveBeenCalledTimes(2)
    expect(fetchTask).toHaveBeenCalledWith('task-1', expect.any(AbortSignal))
  })

  it('returns the server message when the job fails', async () => {
    const time = clock()
    const result = await pollRecreateAnalysis({
      start: vi.fn().mockResolvedValue({ task_id: 'task-1' }),
      fetchTask: vi.fn().mockResolvedValue({
        task_id: 'task-1',
        status: 'failed',
        message: 'Reference video analysis timed out. Try again later.',
      }),
      signal: new AbortController().signal,
      pollMs: 2000,
      maxMs: 480000,
      now: time.now,
      sleep: time.sleep,
    })

    expect(result).toEqual({
      status: 'failed',
      message: 'Reference video analysis timed out. Try again later.',
    })
  })

  it('times out after the eight-minute ceiling', async () => {
    const time = clock()
    const fetchTask = vi.fn().mockResolvedValue(RUNNING)
    const result = await pollRecreateAnalysis({
      start: vi.fn().mockResolvedValue({ task_id: 'task-1' }),
      fetchTask,
      signal: new AbortController().signal,
      pollMs: 2000,
      maxMs: 8000,
      now: time.now,
      sleep: time.sleep,
    })

    expect(result).toEqual({ status: 'timeout' })
    expect(fetchTask).toHaveBeenCalledTimes(4)
  })

  it('stops between polls when the caller cancels', async () => {
    const controller = new AbortController()
    const fetchTask = vi.fn().mockResolvedValue(RUNNING)
    const result = await pollRecreateAnalysis({
      start: vi.fn().mockResolvedValue({ task_id: 'task-1' }),
      fetchTask,
      signal: controller.signal,
      pollMs: 2000,
      maxMs: 480000,
      now: () => 0,
      sleep: async () => {
        controller.abort()
      },
    })

    expect(result).toEqual({ status: 'cancelled' })
    expect(fetchTask).not.toHaveBeenCalled()
  })

  it('does not start a job once the signal is already aborted', async () => {
    const controller = new AbortController()
    controller.abort()
    const start = vi.fn().mockResolvedValue({ task_id: 'task-1' })
    const result = await pollRecreateAnalysis({
      start,
      fetchTask: vi.fn(),
      signal: controller.signal,
    })
    expect(result).toEqual({ status: 'cancelled' })
    expect(start).not.toHaveBeenCalled()
  })
})
