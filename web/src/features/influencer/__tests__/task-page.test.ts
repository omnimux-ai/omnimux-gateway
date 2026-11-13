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

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { fetchInfluencerTasks, fetchTaskRecord } from '../api'
import type { TaskRecord } from '../types'

const { get } = vi.hoisted(() => ({ get: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: { get } }))

function rec(over: Partial<TaskRecord> = {}): TaskRecord {
  return { id: 1, task_id: 'task_1', status: 'SUCCESS', ...over }
}

/** The envelope `common.ApiSuccess` wraps a PageInfo in. */
function page(items: TaskRecord[], total?: number) {
  return {
    data: {
      success: true,
      data: total === undefined ? { items } : { items, total },
    },
  }
}

type GetCall = [string, { params: Record<string, unknown> }]

function lastParams(): Record<string, unknown> {
  const calls = get.mock.calls as GetCall[]
  const call = calls.at(-1)
  if (!call) throw new Error('api.get was never called')
  return call[1].params
}

describe('fetchInfluencerTasks', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('sends the page as `p`, the key the gateway actually parses', async () => {
    get.mockResolvedValue(page([rec()], 1))

    await fetchInfluencerTasks({ page: 3, page_size: 12 })

    const params = lastParams()
    expect(params.p).toBe(3)
    // A `page` key is silently ignored server-side and would pin the response
    // to page 1 — the regression this test exists for.
    expect(params.page).toBeUndefined()
    expect(params.page_size).toBe(12)
  })

  it('defaults to the first page with the artifact projection on', async () => {
    get.mockResolvedValue(page([], 0))

    await fetchInfluencerTasks()

    const params = lastParams()
    expect(params.p).toBe(1)
    expect(params.with_artifacts).toBe(true)
    expect(params.action).toBe('influencer_sheet')
  })

  it('skips the artifact projection when the caller only needs a status', async () => {
    get.mockResolvedValue(page([rec()], 1))

    await fetchInfluencerTasks({ task_id: 'task_1', with_artifacts: false })

    expect(lastParams().with_artifacts).toBe(false)
  })

  it('returns the server total so the caller can tell when pages run out', async () => {
    get.mockResolvedValue(page([rec()], 57))

    const result = await fetchInfluencerTasks()

    expect(result.total).toBe(57)
    expect(result.items).toHaveLength(1)
  })

  it('falls back to zero total and no items on an empty envelope', async () => {
    get.mockResolvedValue({ data: { success: true, data: {} } })

    const result = await fetchInfluencerTasks()

    expect(result.total).toBe(0)
    expect(result.items).toEqual([])
  })
})

describe('fetchTaskRecord', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('reads one record without the artifact projection by default', async () => {
    get.mockResolvedValue(page([rec()], 1))

    const record = await fetchTaskRecord('task_1')

    expect(record?.task_id).toBe('task_1')
    expect(lastParams().page_size).toBe(1)
    expect(lastParams().with_artifacts).toBe(false)
  })

  it('asks for artifacts when the caller is settling a finished task', async () => {
    get.mockResolvedValue(page([rec()], 1))

    await fetchTaskRecord('task_1', { with_artifacts: true })

    expect(lastParams().with_artifacts).toBe(true)
  })

  it('prefers the record whose id matches the request', async () => {
    get.mockResolvedValue(page([rec({ id: 9, task_id: 'other' }), rec()], 2))

    const record = await fetchTaskRecord('task_1')

    expect(record?.id).toBe(1)
  })

  it('returns null when the task is not readable from this session', async () => {
    get.mockResolvedValue(page([], 0))

    expect(await fetchTaskRecord('task_missing')).toBeNull()
  })
})
