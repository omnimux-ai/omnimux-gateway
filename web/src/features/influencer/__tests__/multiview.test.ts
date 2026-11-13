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

import { taskAbsoluteImageUrl, parentTaskId, taskKind } from '../lib/history'
import {
  canDeriveMultiView,
  multiViewProgress,
  multiViewState,
  partitionTasks,
  taskKey,
} from '../lib/multiview'
import type { TaskRecord } from '../types'

function sheet(taskId: string, over: Partial<TaskRecord> = {}): TaskRecord {
  return {
    id: 1,
    task_id: taskId,
    status: 'SUCCESS',
    properties: { input: JSON.stringify({ tier: 'normal', selection: {} }) },
    ...over,
  }
}

function board(
  parentTaskId: string,
  over: Partial<TaskRecord> = {}
): TaskRecord {
  return {
    id: 2,
    task_id: `board-${parentTaskId}`,
    status: 'QUEUED',
    properties: {
      input: JSON.stringify({
        kind: 'multiview',
        parent_task_id: parentTaskId,
      }),
    },
    ...over,
  }
}

describe('taskKind', () => {
  it('reads a row without a kind as a character sheet', () => {
    expect(taskKind(sheet('a'))).toBe('sheet')
    expect(taskKind(board('a'))).toBe('multiview')
  })

  it('reads an unparseable payload as a character sheet', () => {
    expect(taskKind(sheet('a', { properties: { input: 'not json' } }))).toBe(
      'sheet'
    )
  })
})

describe('parentTaskId', () => {
  it('returns the parent only for a derived row', () => {
    expect(parentTaskId(board('parent-1'))).toBe('parent-1')
    expect(parentTaskId(sheet('a'))).toBeNull()
  })

  it('ignores an empty or missing parent', () => {
    const empty = board('', {
      properties: { input: JSON.stringify({ kind: 'multiview' }) },
    })
    expect(parentTaskId(empty)).toBeNull()
  })
})

describe('taskAbsoluteImageUrl', () => {
  it('keeps the absolute artifact URL an upstream provider must fetch', () => {
    const url = 'https://omnimux.ai/v1/tasks/t/artifacts/image/content?access=a'
    expect(
      taskAbsoluteImageUrl(sheet('t', { artifacts: [{ content_url: url }] }))
    ).toBe(url)
  })

  it('refuses a relative URL, which no provider could fetch', () => {
    expect(
      taskAbsoluteImageUrl(
        sheet('t', { artifacts: [{ content_url: '/v1/tasks/t/content' }] })
      )
    ).toBeNull()
  })

  it('returns null when the row carries no image', () => {
    expect(taskAbsoluteImageUrl(sheet('t'))).toBeNull()
  })
})

describe('multiViewState', () => {
  it('maps task status to the badge state', () => {
    expect(multiViewState(null)).toBe('absent')
    expect(multiViewState(board('a', { status: 'QUEUED' }))).toBe('queued')
    expect(multiViewState(board('a', { status: 'IN_PROGRESS' }))).toBe(
      'generating'
    )
    expect(multiViewState(board('a', { status: 'SUCCESS' }))).toBe('ready')
    expect(multiViewState(board('a', { status: 'FAILURE' }))).toBe('failed')
  })

  it('accepts the lowercase status spelling some adaptors return', () => {
    expect(multiViewState(board('a', { status: 'succeeded' }))).toBe('ready')
  })
})

describe('multiViewProgress', () => {
  it('parses a percentage string', () => {
    expect(multiViewProgress(board('a', { progress: '42%' }))).toBe(42)
  })

  it('clamps missing, malformed and out-of-range values', () => {
    expect(multiViewProgress(null)).toBe(0)
    expect(multiViewProgress(board('a', { progress: '' }))).toBe(0)
    expect(multiViewProgress(board('a', { progress: 'abc' }))).toBe(0)
    expect(multiViewProgress(board('a', { progress: '140' }))).toBe(100)
    expect(multiViewProgress(board('a', { progress: '-5' }))).toBe(0)
  })
})

describe('partitionTasks', () => {
  it('keeps sheets as roots and attaches a board to its parent', () => {
    const root = sheet('parent-1', { id: 10 })
    const child = board('parent-1', { id: 11 })

    const { roots, childByParent } = partitionTasks([child, root])

    expect(roots.map(taskKey)).toEqual(['parent-1'])
    expect(childByParent.get('parent-1')).toBe(child)
  })

  it('never promotes a derived row to a root card', () => {
    const { roots } = partitionTasks([board('parent-1')])
    expect(roots).toEqual([])
  })

  it('keeps the newest board when a card has several', () => {
    const older = board('parent-1', { id: 20 })
    const newer = board('parent-1', { id: 21 })

    const { childByParent } = partitionTasks([
      sheet('parent-1', { id: 10 }),
      older,
      newer,
    ])

    expect(childByParent.get('parent-1')).toBe(newer)
  })

  it('drops a board whose parent is no longer listed', () => {
    const { roots, childByParent } = partitionTasks([board('parent-gone')])
    expect(roots).toEqual([])
    expect(childByParent.size).toBe(0)
  })

  it('treats a malformed payload as a root rather than losing the row', () => {
    const broken = sheet('broken', {
      properties: { input: '{not json' },
    })
    const { roots } = partitionTasks([broken])
    expect(roots.map(taskKey)).toEqual(['broken'])
  })
})

describe('multi-view character derivation', () => {
  it.each(['edit', 'masked_edit'])(
    'rejects an ordinary %s result as a character parent',
    (image_mode) => {
      expect(
        canDeriveMultiView(
          sheet('edited', {
            properties: {
              input: JSON.stringify({
                tier: 'normal',
                selection: {},
                image_mode,
              }),
            },
          })
        )
      ).toBe(false)
    }
  )
  it.each(['text', 'reference', undefined])(
    'retains a %s character sheet as a valid parent',
    (image_mode) => {
      expect(
        canDeriveMultiView(
          sheet('character', {
            properties: {
              input: JSON.stringify({
                tier: 'normal',
                selection: {},
                image_mode,
              }),
            },
          })
        )
      ).toBe(true)
    }
  )
})
