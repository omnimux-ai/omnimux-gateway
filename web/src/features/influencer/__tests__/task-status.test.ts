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
import { describe, expect, it } from 'vitest'

import { recreateTaskPhase } from '../lib/task-status'
import type { TaskRecord } from '../types'

const webRecreate: TaskRecord = {
  id: 1,
  task_id: 'recreate-1',
  action: 'influencer_motion',
  status: 'WAITING',
  properties: {
    input: JSON.stringify({
      mode: 'recreate',
      prompt: 'keep the turn',
      video_url: 'https://files.example/reference.mp4',
    }),
  },
}

describe('recreate task phase', () => {
  it('reports the analyzing phase while the worker plans', () => {
    expect(recreateTaskPhase({ ...webRecreate, stage: 'analyzing' })).toBe(
      'analyzing'
    )
  })

  it('reports the generating phase after the planner hands off', () => {
    expect(recreateTaskPhase({ ...webRecreate, stage: 'generating' })).toBe(
      'generating'
    )
  })

  it('falls back to status while a freshly accepted task has no stage yet', () => {
    expect(recreateTaskPhase({ ...webRecreate, status: 'QUEUED' })).toBe(
      'analyzing'
    )
    expect(recreateTaskPhase({ ...webRecreate, status: 'CLAIMED' })).toBe(
      'generating'
    )
  })

  it('reports completed and failed only at terminal states', () => {
    expect(recreateTaskPhase({ ...webRecreate, status: 'SUCCESS' })).toBe(
      'completed'
    )
    expect(
      recreateTaskPhase({
        ...webRecreate,
        status: 'FAILURE',
        fail_reason: 'provider rejected the request',
      })
    ).toBe('failed')
  })

  it('distinguishes a timeout failure from an ordinary failure', () => {
    expect(
      recreateTaskPhase({
        ...webRecreate,
        status: 'FAILURE',
        fail_reason: '参考视频拆解超时，视频生成未开始',
      })
    ).toBe('timeout')
    expect(
      recreateTaskPhase({
        ...webRecreate,
        status: 'FAILURE',
        fail_reason: 'Task timed out',
      })
    ).toBe('timeout')
  })

  it('does not claim non-recreate motion tasks', () => {
    expect(
      recreateTaskPhase({
        ...webRecreate,
        properties: {
          input: JSON.stringify({
            mode: 'motion',
            video_url: 'https://files.example/reference.mp4',
          }),
        },
      })
    ).toBeNull()
  })

  it('does not claim the legacy CLI recreate path without a saved clip', () => {
    expect(
      recreateTaskPhase({
        ...webRecreate,
        properties: {
          input: JSON.stringify({
            mode: 'recreate',
            prompt: 'compiled prompt',
          }),
        },
      })
    ).toBeNull()
  })
})
