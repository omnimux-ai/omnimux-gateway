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

import {
  clearHistoryCache,
  readHistoryCache,
  writeHistoryCache,
} from '../lib/history-cache'
import type { TaskRecord } from '../types'

const KEY = 'influencer:history-cache:v1'
const USER = 7

function rec(over: Partial<TaskRecord> = {}): TaskRecord {
  return { id: 1, task_id: 'task_1', status: 'SUCCESS', ...over }
}

describe('history cache', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('round-trips terminal records with the server total for the same account', () => {
    writeHistoryCache(USER, [rec()], 42)

    const cached = readHistoryCache(USER)

    expect(cached?.items.map((item) => item.task_id)).toEqual(['task_1'])
    expect(cached?.total).toBe(42)
  })

  it('drops a payload written for a different account instead of showing it', () => {
    writeHistoryCache(USER, [rec()], 1)

    expect(readHistoryCache(USER + 1)).toBeNull()
    expect(localStorage.getItem(KEY)).toBeNull()
  })

  it('drops a payload from an older cache version', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({ v: 0, userId: USER, total: 1, items: [rec()] })
    )

    expect(readHistoryCache(USER)).toBeNull()
  })

  it('returns null and clears a corrupted payload', () => {
    localStorage.setItem(KEY, '{not json')

    expect(readHistoryCache(USER)).toBeNull()
    expect(localStorage.getItem(KEY)).toBeNull()
  })

  it('caches only terminal records', () => {
    writeHistoryCache(
      USER,
      [rec(), rec({ id: 2, task_id: 'task_2', status: 'IN_PROGRESS' })],
      2
    )

    expect(readHistoryCache(USER)?.items.map((item) => item.task_id)).toEqual([
      'task_1',
    ])
  })

  it('keeps at most the newest 60 records', () => {
    const items = Array.from({ length: 80 }, (_, index) =>
      rec({ id: index + 1, task_id: `task_${index + 1}` })
    )

    writeHistoryCache(USER, items, items.length)

    expect(readHistoryCache(USER)?.items).toHaveLength(60)
  })

  it('writes nothing when no record has finished', () => {
    writeHistoryCache(USER, [rec({ status: 'QUEUED' })], 1)

    expect(localStorage.getItem(KEY)).toBeNull()
  })

  it('degrades silently when storage refuses the write', () => {
    const spy = vi
      .spyOn(localStorage, 'setItem')
      .mockImplementation(() => {
        throw new Error('quota exceeded')
      })

    expect(() => writeHistoryCache(USER, [rec()], 1)).not.toThrow()
    spy.mockRestore()
  })

  it('ignores a missing account id on both read and write', () => {
    expect(readHistoryCache(undefined)).toBeNull()

    writeHistoryCache(undefined, [rec()], 1)

    expect(localStorage.getItem(KEY)).toBeNull()
  })

  it('clears the cache on demand', () => {
    writeHistoryCache(USER, [rec()], 1)

    clearHistoryCache()

    expect(readHistoryCache(USER)).toBeNull()
  })
})
