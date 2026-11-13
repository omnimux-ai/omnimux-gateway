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
import { renderHook } from '@testing-library/react'
import { toast } from 'sonner'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { useGenerationQueueNotice } from '../hooks/use-generation-queue-notice'
import type { TaskRecord } from '../types'

const record = (id: string, status = 'IN_PROGRESS'): TaskRecord => ({
  id: 1,
  task_id: id,
  action: 'influencer_sheet',
  status,
})
const active = Array.from({ length: 5 }, (_, i) => record(`active-${i}`))
afterEach(() => vi.restoreAllMocks())

describe('generation capacity notification', () => {
  it('notifies once when a newly submitted task is confirmed waiting behind five mixed tasks', () => {
    const info = vi.spyOn(toast, 'info').mockImplementation(() => 1)
    const waiting = record('new', 'WAITING')
    const mixed: TaskRecord[] = active.map((item, i) => ({
      ...item,
      action: i % 2 ? 'influencer_motion' : 'influencer_sheet',
    }))
    const { rerender } = renderHook(
      ({ submitted, history }) => useGenerationQueueNotice(submitted, history),
      {
        initialProps: { submitted: [waiting], history: mixed },
      }
    )
    expect(info).toHaveBeenCalledTimes(1)
    rerender({ submitted: [waiting], history: [...mixed, waiting] })
    expect(info).toHaveBeenCalledTimes(1)
  })

  it('does not notify for an unconfirmed placeholder, spare capacity or completed tasks', () => {
    const info = vi.spyOn(toast, 'info').mockImplementation(() => 1)
    const { rerender } = renderHook(
      ({ submitted, history }) => useGenerationQueueNotice(submitted, history),
      {
        initialProps: { submitted: [record('new', 'QUEUED')], history: active },
      }
    )
    expect(info).not.toHaveBeenCalled()
    rerender({
      submitted: [record('new', 'WAITING')],
      history: [...active.slice(0, 4), record('done', 'SUCCESS')],
    })
    expect(info).not.toHaveBeenCalled()
  })

  it('does not notify for restored history without a new submission', () => {
    const info = vi.spyOn(toast, 'info').mockImplementation(() => 1)
    renderHook(() =>
      useGenerationQueueNotice([], [...active, record('old', 'WAITING')])
    )
    expect(info).not.toHaveBeenCalled()
  })
})
