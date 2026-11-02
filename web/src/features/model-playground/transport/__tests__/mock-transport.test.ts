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
import { beforeEach, describe, expect, test } from 'vitest'

import { MOCK_OUTCOME_PARAM, MockTransport } from '../mock-transport'

const RUN_MS = 1000
const TIMEOUT_MS = 4000

let clock = 0
let transport: MockTransport

beforeEach(() => {
  clock = 0
  transport = new MockTransport({
    now: () => clock,
    runDurationMs: RUN_MS,
    timeoutAfterMs: TIMEOUT_MS,
  })
})

describe('MockTransport lifecycle', () => {
  test('advances queued → running → succeeded with playable media', async () => {
    const { taskId } = await transport.submit({
      modelId: 'gpt-image-flat',
      params: { output_format: 'png' },
    })

    const queued = await transport.getTask(taskId)
    expect(queued.status).toBe('queued')

    clock = RUN_MS / 2
    const running = await transport.getTask(taskId)
    expect(running.status).toBe('running')
    expect(running.progress).toBeGreaterThan(0)
    expect(running.progress).toBeLessThan(100)

    clock = RUN_MS
    const succeeded = await transport.getTask(taskId)
    expect(succeeded.status).toBe('succeeded')
    expect(succeeded.progress).toBe(100)
    expect(succeeded.media).toHaveLength(1)
    expect(succeeded.media[0].kind).toBe('image')
    expect(succeeded.finishedAt).toBe(RUN_MS)
  })

  test('reports a video asset when the submission is duration based', async () => {
    const { taskId } = await transport.submit({
      modelId: 'seedance-flat',
      params: { duration: '5', output_format: 'mp4' },
    })
    clock = RUN_MS
    const task = await transport.getTask(taskId)
    expect(task.media[0].kind).toBe('video')
  })

  test('reaches the failed terminal state when the debug switch asks for it', async () => {
    const { taskId } = await transport.submit({
      modelId: 'gpt-image-flat',
      params: { [MOCK_OUTCOME_PARAM]: 'failed' },
    })

    clock = RUN_MS
    const failed = await transport.getTask(taskId)
    expect(failed.status).toBe('failed')
    expect(failed.failReason).toBeTruthy()
    expect(failed.media).toHaveLength(0)

    clock = RUN_MS * 10
    const stillFailed = await transport.getTask(taskId)
    expect(stillFailed.status).toBe('failed')
    expect(stillFailed.finishedAt).toBe(RUN_MS)
  })

  test('reaches the timeout terminal state only after the timeout budget', async () => {
    const { taskId } = await transport.submit({
      modelId: 'seedance-flat',
      params: { [MOCK_OUTCOME_PARAM]: 'timeout' },
    })

    clock = TIMEOUT_MS - 1
    expect((await transport.getTask(taskId)).status).toBe('running')

    clock = TIMEOUT_MS
    expect((await transport.getTask(taskId)).status).toBe('timeout')
  })

  test('rejects reads for an unknown task instead of inventing one', async () => {
    await expect(transport.getTask('missing')).rejects.toThrow()
  })
})

describe('MockTransport history', () => {
  test('lists the model history newest first and excludes other models', async () => {
    const first = await transport.submit({ modelId: 'a', params: {} })
    clock = 10
    const second = await transport.submit({ modelId: 'a', params: {} })
    clock = 20
    await transport.submit({ modelId: 'b', params: {} })

    clock = RUN_MS * 10
    const history = await transport.listHistory('a')
    expect(history.map((task) => task.taskId)).toEqual([
      second.taskId,
      first.taskId,
    ])
    expect(history.every((task) => task.status === 'succeeded')).toBe(true)
  })
})
