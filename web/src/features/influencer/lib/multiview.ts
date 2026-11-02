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

import type { TaskRecord } from '../types'
import { parentTaskId, parseTaskMeta, taskKind } from './history'
import { hasUnverifiedResult } from './task-status'

/** The panel aspect the turnaround layout is generated at. */
export const MULTIVIEW_SIZE = '16:9'

export type MultiViewState =
  | 'absent'
  | 'queued'
  | 'submitting'
  | 'review'
  | 'generating'
  | 'failed'
  | 'ready'

/** Ordinary image edits do not establish a new character identity. */
export function canDeriveMultiView(task: TaskRecord): boolean {
  const mode = parseTaskMeta(task)?.image_mode
  return taskKind(task) === 'sheet' && mode !== 'edit' && mode !== 'masked_edit'
}

/** The key a task is identified by in the gallery. */
export function taskKey(task: TaskRecord): string {
  return String(task.task_id || task.id)
}

/** Derives the 1:1 badge state from the derived row attached to a card. */
export function multiViewState(
  child: TaskRecord | null | undefined
): MultiViewState {
  if (!child) return 'absent'
  // Adaptors spell the terminal states either way, so both are accepted.
  const status = (child.status ?? '').toUpperCase()
  if (status === 'SUCCESS' || status === 'SUCCEEDED') return 'ready'
  if (status === 'FAILURE' || status === 'FAILED') return 'failed'
  if (hasUnverifiedResult(child)) return 'review'
  if (['WAITING', 'QUEUED', 'SUBMITTED', 'NOT_START'].includes(status)) {
    return 'queued'
  }
  if (status === 'CLAIMED' || status === 'DISPATCHING') {
    return child.fail_reason ? 'review' : 'submitting'
  }
  return 'generating'
}

/** Progress as an integer percentage, clamped to 0..100. */
export function multiViewProgress(
  child: TaskRecord | null | undefined
): number {
  const parsed = Number.parseInt(child?.progress ?? '', 10)
  if (!Number.isFinite(parsed)) return 0
  return Math.min(Math.max(parsed, 0), 100)
}

/** A local placeholder for a board that was just submitted but is not yet
 * readable from history, so its badge appears without waiting a poll. */
export function pendingMultiViewRecord(taskId: string): TaskRecord {
  return { id: 0, task_id: taskId, status: 'QUEUED', progress: '0%' }
}

export interface PartitionedTasks {
  /** Character sheets, which are the only rows the gallery renders. */
  roots: TaskRecord[]
  /** The newest derived row per parent task id. */
  childByParent: Map<string, TaskRecord>
}

/** Splits history into gallery roots and the derived rows attached to them.
 * A derived row never renders as a top-level card, and one whose parent is no
 * longer listed is dropped rather than promoted. */
export function partitionTasks(tasks: TaskRecord[]): PartitionedTasks {
  const roots: TaskRecord[] = []
  const childByParent = new Map<string, TaskRecord>()

  for (const task of tasks) {
    const parent = parentTaskId(task)
    if (!parent) {
      roots.push(task)
      continue
    }
    const current = childByParent.get(parent)
    if (!current || (task.id ?? 0) > (current.id ?? 0)) {
      childByParent.set(parent, task)
    }
  }

  const rootKeys = new Set(roots.map(taskKey))
  const orphans: string[] = []
  for (const parent of childByParent.keys()) {
    if (!rootKeys.has(parent)) orphans.push(parent)
  }
  for (const parent of orphans) childByParent.delete(parent)

  return { roots, childByParent }
}
