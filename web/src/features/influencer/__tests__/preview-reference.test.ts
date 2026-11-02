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
  artifactTaskId,
  classifyReference,
  mediaFormat,
} from '../lib/preview-reference'
import type { UploadRecord } from '../lib/upload-library'
import type { TaskRecord } from '../types'

const artifact =
  'https://omnimux.ai/v1/tasks/task_abc/artifacts/image/content?access=x'
const owned: TaskRecord = { id: 1, task_id: 'task_abc', status: 'SUCCESS' }
const upload: UploadRecord = {
  kind: 'image',
  url: 'https://files.omnimux.ai/photos/portrait.jpg',
  fileName: 'portrait.jpg',
  uploadedAt: 1,
  expiresAt: 2,
}

describe('history preview reference classification', () => {
  it('reads the task id from a gateway artifact address only', () => {
    expect(artifactTaskId(artifact)).toBe('task_abc')
    expect(artifactTaskId('/v1/tasks/t%201/artifacts/video/content')).toBe(
      't 1'
    )
    expect(artifactTaskId('https://files.omnimux.ai/photos/a.jpg')).toBeNull()
    expect(artifactTaskId('/v1/tasks/x/artifacts/image')).toBeNull()
  })
  it('resolves a loaded generated record by its artifact address', () => {
    expect(classifyReference(artifact, 'image', [owned], [])).toEqual({
      entry: { kind: 'task', task: owned },
    })
  })
  it('asks for one record when the generated reference is not loaded', () => {
    expect(classifyReference(artifact, 'video', [], [upload])).toEqual({
      taskId: 'task_abc',
    })
  })
  it('matches an upload only by exact URL and media kind', () => {
    expect(classifyReference(upload.url, 'image', [], [upload])).toEqual({
      entry: { kind: 'upload', record: upload },
    })
    expect(classifyReference(upload.url, 'video', [], [upload])).toEqual({
      entry: { kind: 'unknown', url: upload.url, media: 'video' },
    })
  })
  it('falls back to unknown for an address with no known source', () => {
    expect(
      classifyReference('https://example.com/x.png', 'image', [owned], [])
    ).toEqual({
      entry: {
        kind: 'unknown',
        url: 'https://example.com/x.png',
        media: 'image',
      },
    })
  })
  it('derives a display format from a plain extension only', () => {
    expect(mediaFormat('take.mov')).toBe('MOV')
    expect(mediaFormat('photo.jpeg')).toBe('JPG')
    expect(mediaFormat('https://cdn.example.com/a/b.webp?sig=1')).toBe('WEBP')
    expect(mediaFormat(artifact)).toBeUndefined()
    expect(mediaFormat('no-extension')).toBeUndefined()
  })
})
