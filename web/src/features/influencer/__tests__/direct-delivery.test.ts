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
import { expect, it } from 'vitest'

import {
  taskAbsoluteImageUrl,
  taskBrowserMediaUrl,
  taskImageUrl,
} from '../lib/history'

it('opts browser content into direct delivery but leaves the model reference canonical', () => {
  const canonical =
    'https://omnimux.ai/v1/tasks/t/artifacts/image/content?access=abc'
  const task = {
    id: 1,
    status: 'SUCCESS',
    task_id: 't',
    artifacts: [{ key: 'image', content_url: canonical }],
  }
  expect(taskImageUrl(task)).toBe(
    '/v1/tasks/t/artifacts/image/content?access=abc&delivery=direct'
  )
  expect(taskAbsoluteImageUrl(task)).toBe(canonical)
})
it('keeps capability and existing arguments when requesting attachment delivery', () => {
  expect(
    taskBrowserMediaUrl(
      '/v1/tasks/t/artifacts/video/content?access=abc&delivery=direct',
      true
    )
  ).toBe(
    '/v1/tasks/t/artifacts/video/content?access=abc&delivery=direct&download=1'
  )
})
it('does not rewrite external task-shaped or protocol-relative URLs into gateway requests', () => {
  const external =
    'https://files.example/v1/tasks/t/artifacts/image/content?access=external'
  expect(taskBrowserMediaUrl(external, true)).toBe(external)
  expect(
    taskImageUrl({
      id: 1,
      task_id: 't',
      status: 'SUCCESS',
      artifacts: [{ key: 'image', content_url: external }],
    })
  ).toBe(external)
  const relativeExternal =
    '//files.example/v1/tasks/t/artifacts/image/content?access=external'
  expect(taskBrowserMediaUrl(relativeExternal)).toBe(relativeExternal)
})
it('does not modify non-task media URLs', () => {
  expect(taskBrowserMediaUrl('https://files.example/a.png', true)).toBe(
    'https://files.example/a.png'
  )
})
