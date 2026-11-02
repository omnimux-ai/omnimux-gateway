import type { TaskRecord } from '../types'

const TERMINAL = new Set(['SUCCESS', 'FAILURE'])

/** Terminal task statuses accepted by the workbench and provider adapters. */
export function isTerminalStatus(status: string | undefined): boolean {
  return (
    TERMINAL.has((status ?? '').toUpperCase()) ||
    (status ?? '').toLowerCase() === 'succeeded' ||
    (status ?? '').toLowerCase() === 'failed'
  )
}

/** A confirmed terminal result wins over an older reconciliation snapshot. */
export function hasUnverifiedResult(
  task: Pick<TaskRecord, 'status' | 'result_unverified'> | null | undefined
): boolean {
  return task?.result_unverified === true && !isTerminalStatus(task.status)
}
