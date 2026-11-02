import type { TaskRecord } from '../types'
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
import type { UploadRecord } from './upload-library'

export type PreviewMediaKind = 'image' | 'video'

/** One layer of the history preview: a generated record, an upload or an untraceable reference. */
export type PreviewEntry =
  | { kind: 'task'; task: TaskRecord; readOnly?: boolean }
  | { kind: 'upload'; record: UploadRecord }
  | { kind: 'unknown'; url: string; media: PreviewMediaKind }

const ARTIFACT_PATH = /^\/v1\/tasks\/([^/]+)\/artifacts\/([^/]+)\/content$/

/** The task id inside a gateway artifact address, or null for any other URL. */
export function artifactTaskId(url: string): string | null {
  try {
    const parsed = new URL(url, 'https://reference.invalid')
    const match = ARTIFACT_PATH.exec(parsed.pathname)
    return match ? decodeURIComponent(match[1]) : null
  } catch {
    return null
  }
}

/**
 * Classifies a stored reference without network access. A gateway artifact
 * address that is not among the loaded records returns `taskId` so the caller
 * can read that one record; the read is scoped to the signed-in user.
 */
export function classifyReference(
  url: string,
  media: PreviewMediaKind,
  tasks: readonly TaskRecord[],
  uploads: readonly UploadRecord[]
): { entry: PreviewEntry } | { taskId: string } {
  const taskId = artifactTaskId(url)
  if (taskId) {
    const task = tasks.find((item) => item.task_id === taskId)
    return task ? { entry: { kind: 'task', task } } : { taskId }
  }
  const record = uploads.find((item) => item.url === url && item.kind === media)
  if (record) return { entry: { kind: 'upload', record } }
  return { entry: { kind: 'unknown', url, media } }
}

/** Upper-case file extension from a file name or URL path, when it is a plain one. */
export function mediaFormat(nameOrUrl: string): string | undefined {
  let path = nameOrUrl
  try {
    path = new URL(nameOrUrl).pathname
  } catch {
    /* a plain file name */
  }
  const match = /\.([a-z0-9]{2,5})$/i.exec(path)
  if (!match) return undefined
  const ext = match[1].toUpperCase()
  return ext === 'JPEG' ? 'JPG' : ext
}
