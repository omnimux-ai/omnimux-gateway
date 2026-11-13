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

import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  parseTaskMeta,
  taskCharacterSource,
  resolveTaskCharacterSource,
} from '../lib/history'
import { publishedPresets, resolvePresetSelection } from '../lib/presets'
import type { ExplorePreset, InfluencerTaxonomy, TaskRecord } from '../types'

const tax: InfluencerTaxonomy = {
  version: 1,
  source: 'fixture',
  categories: [],
  category_priority: [],
  tier_group: {
    options: ['normal', 'cats', 'dogs'].map((id) => ({
      id,
      label_en: id,
      visibleIn: [],
    })),
    comingSoonImageUrls: [],
  },
  prompt_map: { tiers: {}, categories: {} },
}
const preset: ExplorePreset = {
  id: 'cat',
  name: 'Cat',
  tier: 'cats',
  selection: {},
  preview: { path: '/cat.png', width: 100, height: 100 },
  sheet: { path: '/cat.png', width: 100, height: 100 },
}
const record = (input?: string): TaskRecord => ({
  id: 1,
  task_id: 'history-sheet',
  status: 'SUCCESS',
  properties: input === undefined ? undefined : { input },
})
it.each([
  undefined,
  'not json',
  'null',
  '[]',
  '{}',
  '{"tier":null,"selection":{}}',
  '{"selection":null}',
  '{"tier":"cats","selection":{"animal_breed":null}}',
  '{"selection":{"animal_breed":["cat_siamese"]}}',
])(
  'damaged sheet metadata %s cannot restore or invent a human source',
  (input) => {
    const task = record(input)
    expect(parseTaskMeta(task)).toBeNull()
    expect(
      taskCharacterSource(task, 'https://files.example/sheet.png', 'Sheet')
    ).toEqual({
      url: 'https://files.example/sheet.png',
      fileName: 'Sheet',
    })
  }
)
it.each([
  { selection: {} },
  { brief: 'Legacy character' },
  { image_url: 'https://files.example/reference.png' },
])('a valid legacy sheet without tier restores normal from %j', (meta) => {
  const task = record(JSON.stringify(meta))
  expect(parseTaskMeta(task)).toMatchObject({ tier: 'normal', selection: {} })
  expect(
    taskCharacterSource(task, 'https://files.example/sheet.png', 'Sheet')
  ).toMatchObject({
    characterType: 'human',
    sourceTaskId: 'history-sheet',
  })
})
it('a multiview with an unreadable loaded parent cannot invent human metadata', () => {
  const child = record(
    JSON.stringify({ kind: 'multiview', parent_task_id: 'history-sheet' })
  )
  child.task_id = 'views'
  expect(
    taskCharacterSource(child, 'https://files.example/views.png', 'Views', [
      record('not json'),
    ])
  ).toEqual({
    url: 'https://files.example/views.png',
    fileName: 'Views',
  })
})
afterEach(() => vi.restoreAllMocks())
it.each(['missing', 'unreadable', 'network'])(
  'an old multiview with %s parent lookup falls back to an unknown image source',
  async (failure) => {
    const child = record(
      JSON.stringify({ kind: 'multiview', parent_task_id: 'history-sheet' })
    )
    child.task_id = 'views'
    const get = vi.spyOn(api, 'get')
    if (failure === 'network') get.mockRejectedValue(new Error('offline'))
    else
      {get.mockResolvedValue({
        data: {
          success: true,
          data: {
            items: failure === 'missing' ? [] : [record('not json')],
            total: 1,
          },
        },
      })}
    await expect(
      resolveTaskCharacterSource(
        child,
        'https://files.example/views.png',
        'Views'
      )
    ).resolves.toEqual({
      url: 'https://files.example/views.png',
      fileName: 'Views',
    })
  }
)
it('a tierless child with a damaged selection never borrows its cat parent identity', async () => {
  const child = record(
    JSON.stringify({
      kind: 'multiview',
      selection: null,
      parent_task_id: 'history-sheet',
    })
  )
  await expect(
    resolveTaskCharacterSource(
      child,
      'https://files.example/views.png',
      'Views',
      [record('{"tier":"cats"}')]
    )
  ).resolves.toEqual({
    url: 'https://files.example/views.png',
    fileName: 'Views',
  })
})
it('an explicit null child tier never borrows its cat parent identity', async () => {
  const child = record(
    JSON.stringify({
      kind: 'multiview',
      tier: null,
      parent_task_id: 'history-sheet',
    })
  )
  const get = vi.spyOn(api, 'get')
  await expect(
    resolveTaskCharacterSource(
      child,
      'https://files.example/views.png',
      'Views',
      [record('{"tier":"cats"}')]
    )
  ).resolves.toEqual({
    url: 'https://files.example/views.png',
    fileName: 'Views',
  })
  expect(get).not.toHaveBeenCalled()
})
it.each([null, [], { animal_breed: null }, { animal_breed: 'cat_siamese' }])(
  'damaged animal preset selection %j is not reusable and does not crash the library',
  (selection) => {
    const damaged = { ...preset, selection } as unknown as ExplorePreset
    expect(resolvePresetSelection(damaged, tax)).toBeNull()
    expect(
      publishedPresets([{ ...damaged, published: true, created_at: 1 }], tax)
    ).toEqual([])
  }
)
it('an animal preset with no optional traits is still reusable', () => {
  expect(resolvePresetSelection(preset, tax)).toEqual({})
  expect(
    resolvePresetSelection(
      { ...preset, selection: undefined } as unknown as ExplorePreset,
      tax
    )
  ).toEqual({})
})
it('a damaged published type stays viewable in the library but cannot be reapplied', () => {
  const rows = publishedPresets(
    [{ ...preset, tier: 'damaged', published: true, created_at: 1 }],
    tax
  )
  expect(rows).toHaveLength(1)
  expect(rows[0].preview.path).toBe('/cat.png')
  expect(resolvePresetSelection(rows[0], tax)).toBeNull()
})
it('an animal history record with only tier restores its natural default and unknown tier is refused', () => {
  const task = (input: object): TaskRecord => ({
    id: 1,
    task_id: 'cat',
    status: 'SUCCESS',
    properties: { input: JSON.stringify(input) },
  })
  expect(parseTaskMeta(task({ tier: 'cats' }))).toMatchObject({
    tier: 'cats',
    selection: {},
  })
  expect(parseTaskMeta(task({ tier: 'corrupt', selection: {} }))).toBeNull()
})
