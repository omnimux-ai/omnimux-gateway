import type { TaskRecord } from '../types'

const TERMINAL = new Set(['SUCCESS', 'FAILURE'])

/** Recreate rows share the motion action; their input marks the Web task. */
export function isRecreateTask(
  task: Pick<TaskRecord, 'action' | 'stage' | 'properties'>
): boolean {
  if (task.action !== 'influencer_motion') return false
  if (task.stage) return true
  try {
    const input = JSON.parse(task.properties?.input ?? 'null') as {
      mode?: unknown
      video_url?: unknown
    } | null
    return input?.mode === 'recreate' && typeof input.video_url === 'string'
  } catch {
    return false
  }
}

export type RecreateTaskPhase =
  | 'analyzing'
  | 'generating'
  | 'completed'
  | 'failed'
  | 'timeout'

/** The status name a recreate card shows; null for every other task. */
export function recreateTaskPhase(
  task: Pick<
    TaskRecord,
    'action' | 'stage' | 'status' | 'fail_reason' | 'properties'
  >
): RecreateTaskPhase | null {
  if (!isRecreateTask(task)) return null
  const status = (task.status ?? '').toUpperCase()
  if (
    status === 'SUCCESS' ||
    (task.status ?? '').toLowerCase() === 'succeeded'
  ) {
    return 'completed'
  }
  if (status === 'FAILURE' || (task.status ?? '').toLowerCase() === 'failed') {
    return /超时|timed?\s*out|timeout/i.test(task.fail_reason ?? '')
      ? 'timeout'
      : 'failed'
  }
  if (task.stage === 'generating') return 'generating'
  if (task.stage === 'analyzing') return 'analyzing'
  if (
    status === 'CLAIMED' ||
    status === 'DISPATCHING' ||
    status === 'IN_PROGRESS'
  ) {
    return 'generating'
  }
  return 'analyzing'
}

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
