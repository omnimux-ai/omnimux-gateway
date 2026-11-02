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

import type { MotionCharacterSource, PendingCharacterUpload } from '../types'

/** One pick may add at most thirty reference images. */
export const MAX_REFERENCE_UPLOAD_BATCH = 30

/** Three transfers at a time; later picks join the same queue. */
const UPLOAD_CONCURRENCY = 3

export interface CharacterUploadOutcome {
  /** Files accepted into the queue by this call (0 when the pick was refused). */
  accepted: number
  /** Ceiling that applied to this pick; a refusal message shows it. */
  limit: number
  added: number
  failed: number
  error?: Error
}

interface Options {
  /** Uploads one image and reports transfer progress; aborts on cancel. */
  uploadImage: (
    file: File,
    onProgress: (loaded: number, total: number) => void,
    signal: AbortSignal
  ) => Promise<string>
  /** Appends one finished upload to the ordered reference list. */
  addCharacter: (source: MotionCharacterSource) => void
  /** Current ceiling: 30 generally, 9 for the recreate form. */
  imageLimit: () => number
  /** How many references are selected right now. */
  selectedCount: () => number
  /** Reports a pick that selected more files than the ceiling allows. */
  onLimit: (allowed: number) => void
}

interface Queue {
  entries: PendingCharacterUpload[]
  /** Settled outcome per entry: a URL to append, or null for a failed file. */
  results: Map<string, string | null>
  /** Next entry to start. */
  cursor: number
  /** Next entry whose outcome may be appended; keeps picker order. */
  flushCursor: number
  running: number
  settled: number
  added: number
  failed: number
  error: Error | null
  controller: AbortController
  resolve: (outcome: CharacterUploadOutcome) => void
}

function createPreviewUrl(file: File): string {
  if (typeof URL.createObjectURL !== 'function') return ''
  try {
    return URL.createObjectURL(file)
  } catch {
    return ''
  }
}

function releasePreviewUrl(url: string): void {
  if (!url || typeof URL.revokeObjectURL !== 'function') return
  try {
    URL.revokeObjectURL(url)
  } catch {
    /* the browser already released it */
  }
}

/** Same file picked twice in one pick must upload once. */
function fileIdentity(file: File): string {
  return `${file.name}:${file.size}:${file.lastModified}`
}

/**
 * Reference-image uploads for the character workbench.
 *
 * A pick with more files than the ceiling allows (thirty per pick, and never
 * more than the mode still has room for) is refused as a whole so the user can
 * re-select. Otherwise every file shows up as an upload card immediately,
 * three transfers run at once, and further picks append to the same queue —
 * the upload button never locks. Files append to the reference list in picker
 * order, not completion order, because the first image defines the subject.
 */
export function useCharacterUploads(options: Options) {
  const [uploads, setUploads] = useState<PendingCharacterUpload[]>([])
  const [progress, setProgress] = useState<number | undefined>()
  const uploadsRef = useRef<PendingCharacterUpload[]>([])
  const filesRef = useRef(new Map<string, File>())
  const queueRef = useRef<Queue | null>(null)
  const sequenceRef = useRef(0)
  const optionsRef = useRef(options)
  optionsRef.current = options

  /** The state array and its ref must move together for queued callbacks. */
  const commit = useCallback((next: PendingCharacterUpload[]) => {
    uploadsRef.current = next
    setUploads(next)
  }, [])

  const publishProgress = useCallback(() => {
    const queue = queueRef.current
    if (!queue || queue.entries.length === 0) {
      setProgress(undefined)
      return
    }
    const inFlight = uploadsRef.current.reduce(
      (total, entry) =>
        total + (entry.status === 'uploading' ? entry.progress : 0),
      0
    )
    setProgress(
      Math.min(
        100,
        Math.round(
          ((queue.settled + inFlight / 100) / queue.entries.length) * 100
        )
      )
    )
  }, [])

  const patchEntry = useCallback(
    (id: string, patch: Partial<PendingCharacterUpload>) => {
      commit(
        uploadsRef.current.map((entry) =>
          entry.id === id ? { ...entry, ...patch } : entry
        )
      )
      publishProgress()
    },
    [commit, publishProgress]
  )

  /** Appends every settled run whose picker-order predecessors are settled. */
  const flushSettled = useCallback(
    (queue: Queue) => {
      while (queue.flushCursor < queue.entries.length) {
        const entry = queue.entries[queue.flushCursor]
        if (!queue.results.has(entry.id)) break
        const url = queue.results.get(entry.id) ?? null
        queue.results.delete(entry.id)
        if (url) {
          optionsRef.current.addCharacter({ url, fileName: entry.fileName })
          queue.added += 1
          filesRef.current.delete(entry.id)
          releasePreviewUrl(entry.previewUrl)
          commit(uploadsRef.current.filter((item) => item.id !== entry.id))
        }
        queue.flushCursor += 1
      }
      publishProgress()
    },
    [commit, publishProgress]
  )

  const finishQueue = useCallback(() => {
    const queue = queueRef.current
    if (!queue) return
    queueRef.current = null
    setProgress(undefined)
    queue.resolve({
      accepted: queue.entries.length,
      limit: optionsRef.current.imageLimit(),
      added: queue.added,
      failed: queue.failed,
      ...(queue.error ? { error: queue.error } : {}),
    })
  }, [])

  const pump = useCallback(() => {
    const queue = queueRef.current
    if (!queue) return
    while (
      queue.running < UPLOAD_CONCURRENCY &&
      queue.cursor < queue.entries.length
    ) {
      const entry = queue.entries[queue.cursor]
      queue.cursor += 1
      const file = filesRef.current.get(entry.id)
      if (!file) continue
      queue.running += 1
      patchEntry(entry.id, { status: 'uploading', progress: 0 })
      void optionsRef.current
        .uploadImage(
          file,
          (loaded, total) => {
            if (queueRef.current !== queue) return
            const next = total > 0 ? Math.round((loaded / total) * 100) : 0
            patchEntry(entry.id, {
              progress: Math.min(100, Math.max(0, next)),
            })
          },
          queue.controller.signal
        )
        .then((url) => {
          if (queueRef.current !== queue) return
          queue.running -= 1
          queue.settled += 1
          queue.results.set(entry.id, url)
          flushSettled(queue)
          pump()
        })
        .catch((error: unknown) => {
          if (queueRef.current !== queue) return
          queue.running -= 1
          queue.settled += 1
          queue.failed += 1
          if (!queue.error) {
            queue.error =
              error instanceof Error ? error : new Error('Upload failed')
          }
          queue.results.set(entry.id, null)
          patchEntry(entry.id, { status: 'error', progress: 100 })
          flushSettled(queue)
          pump()
        })
    }
    if (
      queue.cursor >= queue.entries.length &&
      queue.running === 0 &&
      queue.flushCursor >= queue.entries.length
    ) {
      finishQueue()
    }
  }, [finishQueue, flushSettled, patchEntry])

  const createQueue = useCallback((): Queue => {
    const queue: Queue = {
      entries: [],
      results: new Map(),
      cursor: 0,
      flushCursor: 0,
      running: 0,
      settled: 0,
      added: 0,
      failed: 0,
      error: null,
      controller: new AbortController(),
      resolve: () => undefined,
    }
    queueRef.current = queue
    return queue
  }, [])

  /** Waiting transfers still count against the ceiling until they settle. */
  const pendingCount = useCallback(
    () => uploadsRef.current.filter((entry) => entry.status !== 'error').length,
    []
  )

  const allowedForPick = useCallback(
    () =>
      Math.max(
        0,
        Math.min(
          MAX_REFERENCE_UPLOAD_BATCH,
          optionsRef.current.imageLimit() -
            optionsRef.current.selectedCount() -
            pendingCount()
        )
      ),
    [pendingCount]
  )

  /** Resolves when the shared queue drains. */
  const waitForQueue = useCallback((queue: Queue) => {
    return new Promise<CharacterUploadOutcome>((resolve) => {
      const previous = queue.resolve
      queue.resolve = (outcome) => {
        previous(outcome)
        resolve(outcome)
      }
    })
  }, [])

  /**
   * Queues every file from one pick, or refuses the whole pick when it is over
   * the ceiling. The queue keeps accepting later picks while it uploads.
   */
  const pickFiles = useCallback(
    (files: File[]): Promise<CharacterUploadOutcome> => {
      // The picker's accept list and the slot's drop filter already keep
      // non-images out; programmatic callers stay free to pass any file.
      const list = files
      const allowed = allowedForPick()
      if (list.length === 0) {
        return Promise.resolve({
          accepted: 0,
          limit: allowed,
          added: 0,
          failed: 0,
        })
      }
      if (list.length > allowed) {
        optionsRef.current.onLimit(allowed)
        return Promise.resolve({
          accepted: 0,
          limit: allowed,
          added: 0,
          failed: 0,
        })
      }
      const queue = queueRef.current ?? createQueue()
      const seen = new Set<string>()
      const entries: PendingCharacterUpload[] = []
      for (const file of list) {
        const identity = fileIdentity(file)
        if (seen.has(identity)) continue
        seen.add(identity)
        const id = `reference-${(sequenceRef.current += 1)}`
        filesRef.current.set(id, file)
        entries.push({
          id,
          fileName: file.name,
          previewUrl: createPreviewUrl(file),
          progress: 0,
          status: 'waiting',
        })
      }
      queue.entries.push(...entries)
      commit([...uploadsRef.current, ...entries])
      pump()
      return waitForQueue(queue)
    },
    [allowedForPick, commit, createQueue, pump, waitForQueue]
  )

  /**
   * Re-queues one failed entry behind the current transfers. Retrying is also
   * allowed while other files upload; the same ceiling applies.
   */
  const retry = useCallback(
    (id: string): Promise<CharacterUploadOutcome> => {
      const file = filesRef.current.get(id)
      const entry = uploadsRef.current.find((item) => item.id === id)
      const allowed = allowedForPick()
      if (!file || !entry || allowed === 0) {
        if (allowed === 0) optionsRef.current.onLimit(0)
        return Promise.resolve({
          accepted: 0,
          limit: allowed,
          added: 0,
          failed: 0,
        })
      }
      const queue = queueRef.current ?? createQueue()
      const retried: PendingCharacterUpload = {
        ...entry,
        status: 'waiting',
        progress: 0,
      }
      queue.entries.push(retried)
      commit(
        uploadsRef.current.map((item) => (item.id === id ? retried : item))
      )
      pump()
      return waitForQueue(queue)
    },
    [allowedForPick, commit, createQueue, pump, waitForQueue]
  )

  const dismiss = useCallback(
    (id: string) => {
      const entry = uploadsRef.current.find((item) => item.id === id)
      if (!entry) return
      filesRef.current.delete(id)
      releasePreviewUrl(entry.previewUrl)
      commit(uploadsRef.current.filter((item) => item.id !== id))
    },
    [commit]
  )

  /** Drops every queued transfer; used by reset, task restore and unmount. */
  const cancel = useCallback(() => {
    const queue = queueRef.current
    queueRef.current = null
    queue?.controller.abort()
    if (queue) {
      queue.resolve({
        accepted: queue.entries.length,
        limit: optionsRef.current.imageLimit(),
        added: queue.added,
        failed: queue.failed,
        ...(queue.error ? { error: queue.error } : {}),
      })
    }
    for (const entry of uploadsRef.current) {
      releasePreviewUrl(entry.previewUrl)
    }
    filesRef.current.clear()
    uploadsRef.current = []
    setUploads([])
    setProgress(undefined)
  }, [])

  useEffect(() => cancel, [cancel])

  return {
    uploads,
    progress,
    uploading: uploads.some((entry) => entry.status !== 'error'),
    pickFiles,
    retry,
    dismiss,
    cancel,
  }
}
