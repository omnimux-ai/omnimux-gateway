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

import { useCallback, useEffect, useRef, useState } from 'react'

import { useAuthStore } from '@/stores/auth-store'

import {
  loadTaskRecord,
  submitMultiView,
  type MultiViewSubmitBody,
} from '../api'
import { isTerminalStatus } from '../lib/history'
import { pendingMultiViewRecord } from '../lib/multiview'
import type { TaskRecord } from '../types'

const POLL_MS = 2000

/**
 * Submits and tracks multi-view boards without touching the gallery's own
 * submit state: a board is attached to its parent card, so it must never appear
 * as a pending card in the list. Polling is keyed by parent task id and keeps
 * running while the viewer is closed, which is what lets a reopened page show
 * the finished board.
 */
export function useMultiViewSubmit() {
  const userId = useAuthStore((state) => state.auth.user?.id)
  const childrenRef = useRef<Record<string, TaskRecord>>({})
  const postingRef = useRef(new Set<string>())
  const [childByParent, setChildByParent] = useState<
    Record<string, TaskRecord>
  >({})
  const [submittingParent, setSubmittingParent] = useState<string | null>(null)
  const timersRef = useRef<Map<string, ReturnType<typeof setInterval>>>(
    new Map()
  )
  const mountedRef = useRef(true)
  const identitiesRef = useRef(new Map<string, { taskId: string | null }>())

  const clearTimer = useCallback((parentId: string) => {
    const timer = timersRef.current.get(parentId)
    if (timer !== undefined) {
      clearInterval(timer)
      timersRef.current.delete(parentId)
    }
  }, [])

  const clearAll = useCallback(() => {
    for (const timer of timersRef.current.values()) {
      clearInterval(timer)
    }
    timersRef.current.clear()
  }, [])

  useEffect(() => {
    mountedRef.current = true
    clearAll()
    identitiesRef.current.clear()
    postingRef.current.clear()
    childrenRef.current = {}
    setChildByParent({})
    setSubmittingParent(null)
    const identities = identitiesRef.current
    return () => {
      mountedRef.current = false
      identities.clear()
      clearAll()
    }
  }, [clearAll, userId])

  const track = useCallback(
    (
      parentId: string,
      record: TaskRecord,
      identity: { taskId: string | null }
    ) => {
      if (identitiesRef.current.get(parentId) === identity) {
        childrenRef.current[parentId] = record
      }
      setChildByParent((prev) => {
        if (identitiesRef.current.get(parentId) !== identity) return prev
        return { ...prev, [parentId]: record }
      })
    },
    []
  )

  const startPolling = useCallback(
    (parentId: string, taskId: string, identity: { taskId: string | null }) => {
      clearTimer(parentId)
      let inFlight = false
      const poll = () => {
        if (inFlight) return
        inFlight = true
        void (async () => {
          try {
            const next = await loadTaskRecord(taskId)
            if (
              !mountedRef.current ||
              identitiesRef.current.get(parentId) !== identity ||
              (next && next.task_id !== identity.taskId)
            ) {
              return
            }
            if (next) track(parentId, next, identity)
            if (next && isTerminalStatus(next.status)) clearTimer(parentId)
          } catch {
            // Transient read errors do not cancel an accepted generation.
          } finally {
            inFlight = false
          }
        })()
      }
      const timer = setInterval(poll, POLL_MS)
      timersRef.current.set(parentId, timer)
      poll()
    },
    [clearTimer, track]
  )

  const submit = useCallback(
    async (body: MultiViewSubmitBody): Promise<TaskRecord | null> => {
      const parentId = body.parentTaskId
      const child = childrenRef.current[parentId]
      if (
        postingRef.current.has(parentId) ||
        (child && !isTerminalStatus(child.status))
      ) {
        return null
      }
      postingRef.current.add(parentId)
      const identity: { taskId: string | null } = { taskId: null }
      identitiesRef.current.set(parentId, identity)
      clearTimer(parentId)
      setSubmittingParent(parentId)
      try {
        const taskId = await submitMultiView(body)
        identity.taskId = taskId
        const record: TaskRecord = {
          ...pendingMultiViewRecord(taskId),
          submit_time: Math.floor(Date.now() / 1000),
          properties: {
            input: JSON.stringify({
              kind: 'multiview',
              parent_task_id: parentId,
            }),
          },
        }
        if (
          !mountedRef.current ||
          identitiesRef.current.get(parentId) !== identity
        ) {
          return null
        }
        track(parentId, record, identity)
        startPolling(parentId, taskId, identity)
        return record
      } finally {
        if (identitiesRef.current.get(parentId) === identity) {
          postingRef.current.delete(parentId)
        }
        if (
          mountedRef.current &&
          identitiesRef.current.get(parentId) === identity
        ) {
          setSubmittingParent((current) =>
            current === parentId ? null : current
          )
        }
      }
    },
    [clearTimer, startPolling, track]
  )

  /** Drops the attached board locally; the task row itself is left alone. */
  const forget = useCallback(
    (parentId: string) => {
      identitiesRef.current.delete(parentId)
      delete childrenRef.current[parentId]
      postingRef.current.delete(parentId)
      clearTimer(parentId)
      setSubmittingParent((current) => (current === parentId ? null : current))
      setChildByParent((prev) => {
        const next = { ...prev }
        delete next[parentId]
        return next
      })
    },
    [clearTimer]
  )

  return { childByParent, submittingParent, submit, forget }
}
