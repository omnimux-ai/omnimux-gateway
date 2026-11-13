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

import {
  getTaskTimestamp,
  parseTaskMeta,
  sortTasksByTime,
  taskImageUrl,
  taskCardImageUrl,
} from '../lib/history'
import type { TaskRecord } from '../types'

function rec(over: Partial<TaskRecord> = {}): TaskRecord {
  return { id: 1, task_id: 'task_x', status: 'SUCCESS', ...over }
}

describe('parseTaskMeta', () => {
  it('parses _influencer meta from properties.input JSON', () => {
    const meta = {
      _influencer: true,
      tier: 'normal',
      selection: { gender: ['male'], age: ['adult'] },
      brief: 'a reviewer',
      size: '1024x1536',
      seed: 42,
      image_url: null,
    }
    const parsed = parseTaskMeta(
      rec({ properties: { input: JSON.stringify(meta) } })
    )
    expect(parsed?.tier).toBe('normal')
    expect(parsed?.selection.gender).toEqual(['male'])
  })

  it('returns null for non-JSON or missing input', () => {
    expect(parseTaskMeta(rec({ properties: { input: 'not json' } }))).toBeNull()
    expect(parseTaskMeta(rec({}))).toBeNull()
    expect(parseTaskMeta(rec({ properties: { input: '{}' } }))).toBeNull()
  })
})

describe('taskImageUrl', () => {
  it('strips the gateway origin so dev-proxy hosts still resolve', () => {
    const url = taskImageUrl(
      rec({
        artifacts: [
          {
            key: 'image',
            content_url:
              'http://localhost:3000/v1/tasks/task_x/artifacts/image/content?access=abc',
          },
        ],
      })
    )
    expect(url).toBe(
      '/v1/tasks/task_x/artifacts/image/content?access=abc&delivery=direct'
    )
  })

  it('keeps thumbnail derivatives off the direct R2 redirect path', () => {
    const url = taskCardImageUrl(
      rec({
        artifacts: [
          {
            key: 'image',
            content_url: '/v1/tasks/task_x/artifacts/image/content?access=abc',
            thumbnail_url:
              'https://omnimux.ai/v1/tasks/task_x/artifacts/thumb-v1-abc.webp/content?access=abc',
          },
        ],
      })
    )
    expect(url).toBe(
      '/v1/tasks/task_x/artifacts/thumb-v1-abc.webp/content?access=abc'
    )
  })

  it('falls back to result_url then legacy_content_url', () => {
    const r2 = rec({ result_url: 'https://cdn.example.com/img.png' })
    expect(taskImageUrl(r2)).toBe('https://cdn.example.com/img.png')
    const r3 = rec({
      legacy_content_url: 'https://cdn.example.com/legacy.png?x=1',
    })
    expect(taskImageUrl(r3)).toBe('https://cdn.example.com/legacy.png?x=1')
  })

  it('returns null when nothing is available', () => {
    expect(taskImageUrl(rec())).toBeNull()
    expect(taskImageUrl(rec({ artifacts: [] }))).toBeNull()
  })
})

describe('getTaskTimestamp and sortTasksByTime', () => {
  it('prefers submit_time and converts seconds to milliseconds', () => {
    const t = rec({
      submit_time: 1700000000,
      created_at: '2020-01-01T00:00:00Z',
    })
    expect(getTaskTimestamp(t)).toBe(1700000000000)
  })

  it('keeps submit_time in milliseconds if already >= 1e11', () => {
    const t = rec({ submit_time: 1700000000123 })
    expect(getTaskTimestamp(t)).toBe(1700000000123)
  })

  it('falls back to created_at when submit_time is absent', () => {
    const t = rec({ created_at: '2026-10-08T10:00:00Z' })
    expect(getTaskTimestamp(t)).toBe(Date.parse('2026-10-08T10:00:00Z'))
  })

  it('falls back to 0 when both are absent or invalid', () => {
    expect(getTaskTimestamp(rec())).toBe(0)
    expect(getTaskTimestamp(rec({ created_at: 'invalid-date' }))).toBe(0)
  })

  it('sorts tasks reverse chronologically and breaks ties by id descending', () => {
    const t1 = rec({ id: 10, submit_time: 1000 })
    const t2 = rec({ id: 20, submit_time: 2000 }) // newest
    const t3 = rec({ id: 5, submit_time: 1000 }) // same time as t1, smaller id
    const t4 = rec({ id: 15, submit_time: 500 }) // oldest

    const sorted = sortTasksByTime([t1, t4, t2, t3])
    expect(sorted.map((t) => t.id)).toEqual([20, 10, 5, 15])
  })
})

describe('card-only derivative selection', () => {
  it('keeps generation references original and matches the first displayed output', () => {
    const task = rec({
      artifacts: [
        { content_url: '/first.png', thumbnail_url: '/first.webp' },
        { content_url: '/second.png', thumbnail_url: '/second.webp' },
      ],
      thumbnail_url: '/top.webp',
    })
    expect(taskCardImageUrl(task)).toBe('/first.webp')
    expect(taskImageUrl(task)).toBe('/first.png')
  })
  it('uses legacy top-level derivatives without changing legacy original selection', () => {
    const task = rec({
      result_url: '/legacy.png',
      thumbnail_url: '/legacy.webp',
    })
    expect(taskCardImageUrl(task)).toBe('/legacy.webp')
    expect(taskImageUrl(task)).toBe('/legacy.png')
    expect(taskCardImageUrl(rec({ result_url: '/original.png' }))).toBe(
      '/original.png'
    )
  })
})
