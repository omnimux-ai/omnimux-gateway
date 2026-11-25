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
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { fetchTaskRecord, INFLUENCER_MOTION_ACTION } from '../api'
import { taskKind } from '../lib/history'
import {
  classifyReference,
  type PreviewEntry,
  type PreviewMediaKind,
} from '../lib/preview-reference'
import type { UploadRecord } from '../lib/upload-library'
import type { TaskRecord } from '../types'

interface AccountPreviewStack {
  accountId: number | undefined
  entries: PreviewEntry[]
}

export interface UsePreviewStackOptions {
  accountId: number | undefined
  historyItems: readonly TaskRecord[]
  uploadedImages: readonly UploadRecord[]
  uploadedVideos: readonly UploadRecord[]
  hiddenIds: ReadonlySet<string>
}

/**
 * Manages the shared history and reference preview stack with request-sequence
 * race protection and explicit account ownership isolation.
 */
export function usePreviewStack({
  accountId,
  historyItems,
  uploadedImages,
  uploadedVideos,
  hiddenIds,
}: UsePreviewStackOptions) {
  // The history preview is a stack: references open above the record that cites them.
  const [previewState, setPreviewState] = useState<AccountPreviewStack>(() => ({
    accountId,
    entries: [],
  }))
  const previewRequest = useRef(0)

  useEffect(() => {
    previewRequest.current += 1
    setPreviewState((prev) =>
      prev.accountId === accountId && prev.entries.length === 0
        ? prev
        : { accountId, entries: [] }
    )
    return () => {
      previewRequest.current += 1
    }
  }, [accountId])

  /** Opens one record on its own, or closes the preview when given null. */
  const setViewTask = useCallback(
    (task: TaskRecord | null) => {
      previewRequest.current += 1
      setPreviewState({
        accountId,
        entries: task ? [{ kind: 'task', task }] : [],
      })
    },
    [accountId]
  )

  /** Read-only when the record is a derived board or was removed from the gallery. */
  const previewTaskEntry = useCallback(
    (task: TaskRecord): PreviewEntry => ({
      kind: 'task',
      task,
      readOnly:
        taskKind(task) === 'multiview' ||
        hiddenIds.has(String(task.task_id || task.id)),
    }),
    [hiddenIds]
  )

  const onOpenReference = useCallback(
    (url: string, media: PreviewMediaKind) => {
      const request = ++previewRequest.current
      const push = (entry: PreviewEntry) => {
        if (request !== previewRequest.current) return
        // A reference can also open the shared playback window on its own:
        // the workbench slots and the video picker reuse this same entry point.
        setPreviewState((prev) => {
          const stack = prev.accountId === accountId ? prev.entries : []
          return {
            accountId,
            entries: stack.length ? [...stack, entry] : [entry],
          }
        })
      }
      const found = classifyReference(url, media, historyItems, [
        ...uploadedImages,
        ...uploadedVideos,
      ])
      if ('entry' in found) {
        push(
          found.entry.kind === 'task'
            ? previewTaskEntry(found.entry.task)
            : found.entry
        )
        return
      }
      // Not on a loaded history page: read that one owned record.
      void fetchTaskRecord(found.taskId, {
        action: media === 'video' ? INFLUENCER_MOTION_ACTION : undefined,
        with_artifacts: true,
      })
        .then((task) => {
          push(
            task && task.task_id === found.taskId
              ? previewTaskEntry(task)
              : { kind: 'unknown', url, media }
          )
        })
        .catch(() => push({ kind: 'unknown', url, media }))
    },
    [accountId, historyItems, uploadedImages, uploadedVideos, previewTaskEntry]
  )

  /** Reads a reference into the one shared playback window; no second preview layer. */
  const onPreviewVideo = useCallback(
    (url: string) => onOpenReference(url, 'video'),
    [onOpenReference]
  )

  /** Images share the same preview window; only the classified media differs. */
  const onPreviewImage = useCallback(
    (url: string) => onOpenReference(url, 'image'),
    [onOpenReference]
  )

  const onPreviewBack = useCallback(() => {
    previewRequest.current += 1
    setPreviewState((prev) =>
      prev.accountId === accountId
        ? { accountId, entries: prev.entries.slice(0, -1) }
        : { accountId, entries: [] }
    )
  }, [accountId])

  const closePreview = useCallback(() => {
    previewRequest.current += 1
    setPreviewState({ accountId, entries: [] })
  }, [accountId])

  // Detail follows the same current record as the gallery, not the opening snapshot.
  const viewStack = useMemo<PreviewEntry[]>(() => {
    if (previewState.accountId !== accountId) return []
    return previewState.entries.map((entry) => {
      if (entry.kind !== 'task') return entry
      const current = historyItems.find(
        (item) => item.task_id === entry.task.task_id
      )
      return current ? { ...entry, task: current } : entry
    })
  }, [previewState, historyItems, accountId])

  return {
    viewStack,
    setViewTask,
    onOpenReference,
    onPreviewVideo,
    onPreviewImage,
    onPreviewBack,
    closePreview,
  }
}
