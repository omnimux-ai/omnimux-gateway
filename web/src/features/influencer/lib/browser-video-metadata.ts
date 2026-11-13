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

import {
  BrowserVideoMetadataError,
  VIDEO_UNSUPPORTED_MESSAGES,
  type BrowserVideoMetadata,
  type BrowserVideoMetadataErrorCode,
} from './browser-video-metadata-reader'

export { BrowserVideoMetadataError }
export type { BrowserVideoMetadata, BrowserVideoMetadataErrorCode }

const FAILURE_MESSAGE =
  'Could not read this video in your browser. Choose another MP4 or MOV file.'
const INSPECTION_TIMEOUT_MS = 20_000
/**
 * A remote clip is read with a bounded whole-object request, so its timeout has
 * to cover one network transfer instead of a local file read.
 */
const REMOTE_INSPECTION_TIMEOUT_MS = 45_000
const WORKER_IDLE_MS = 30_000
const MAX_PENDING = 2
const localMetadata = new WeakMap<File, BrowserVideoMetadata>()

interface Inspection {
  id: number
  source: File | string
  signal?: AbortSignal
  resolve(metadata: BrowserVideoMetadata): void
  reject(error: Error): void
  cancel(): void
  timer?: ReturnType<typeof setTimeout>
}

interface InspectionResult {
  id: number
  metadata?: BrowserVideoMetadata
  code?: BrowserVideoMetadataErrorCode
  message?: string
}

let worker: Worker | undefined
let active: Inspection | undefined
let idleTimer: ReturnType<typeof setTimeout> | undefined
let nextId = 0
const pending: Inspection[] = []

function stopWorker(): void {
  clearTimeout(idleTimer)
  idleTimer = undefined
  worker?.terminate()
  worker = undefined
}

function complete(error?: Error, metadata?: BrowserVideoMetadata): void {
  const inspection = active
  if (!inspection) return
  active = undefined
  clearTimeout(inspection.timer)
  inspection.signal?.removeEventListener('abort', inspection.cancel)
  if (metadata) {
    if (typeof inspection.source !== 'string') {
      localMetadata.set(inspection.source, { ...metadata })
    }
    inspection.resolve({ ...metadata })
  } else {
    inspection.reject(
      error ?? new BrowserVideoMetadataError('unreadable', FAILURE_MESSAGE)
    )
  }
  idleTimer = setTimeout(stopWorker, WORKER_IDLE_MS)
  processNext()
}

function processNext(): void {
  if (active || pending.length === 0) return
  clearTimeout(idleTimer)
  idleTimer = undefined
  const inspection = pending.shift()
  if (!inspection) return
  active = inspection
  const timeoutMs =
    typeof inspection.source === 'string'
      ? REMOTE_INSPECTION_TIMEOUT_MS
      : INSPECTION_TIMEOUT_MS
  inspection.timer = setTimeout(() => {
    stopWorker()
    complete(new BrowserVideoMetadataError('timeout', FAILURE_MESSAGE))
  }, timeoutMs)
  try {
    if (!worker) {
      worker = new Worker(
        new URL('./browser-video-metadata.worker.ts', import.meta.url),
        { type: 'module' }
      )
      worker.addEventListener(
        'message',
        (event: MessageEvent<InspectionResult>) => {
          if (event.data.id !== active?.id) return
          if (event.data.metadata) complete(undefined, event.data.metadata)
          else {
            const message =
              Object.values(VIDEO_UNSUPPORTED_MESSAGES).find(
                (known) => known === event.data.message
              ) ?? FAILURE_MESSAGE
            complete(
              new BrowserVideoMetadataError(
                event.data.code ?? 'unreadable',
                message
              )
            )
          }
        }
      )
      const failed = () => {
        stopWorker()
        complete(new BrowserVideoMetadataError('unreadable', FAILURE_MESSAGE))
      }
      worker.addEventListener('error', failed)
      worker.addEventListener('messageerror', failed)
    }
    // Structured cloning a File shares its immutable backing bytes; it does not
    // build a whole-file JS buffer or send media to a server.
    worker.postMessage({ id: inspection.id, source: inspection.source })
  } catch {
    stopWorker()
    complete(new BrowserVideoMetadataError('unreadable', FAILURE_MESSAGE))
  }
}

/** The sole browser-side reader, serialized with bounded memory and cancellation. */
export function readBrowserVideoMetadata(
  source: File | string,
  signal?: AbortSignal
): Promise<BrowserVideoMetadata> {
  if (signal?.aborted) {
    return Promise.reject(new DOMException('Aborted', 'AbortError'))
  }
  if (typeof source !== 'string') {
    const cached = localMetadata.get(source)
    if (cached) return Promise.resolve({ ...cached })
  }
  let resolvedSource = source
  if (typeof source === 'string') {
    try {
      // Resolve relative application assets against the page, not the worker's
      // emitted script directory. The worker still enforces HTTP(S) and CORS.
      resolvedSource = new URL(source, document.baseURI).href
    } catch {
      return Promise.reject(
        new BrowserVideoMetadataError('unreadable', FAILURE_MESSAGE)
      )
    }
  }
  if (pending.length >= MAX_PENDING) {
    return Promise.reject(
      new BrowserVideoMetadataError('budget-exceeded', FAILURE_MESSAGE)
    )
  }
  return new Promise((resolve, reject) => {
    const inspection: Inspection = {
      id: ++nextId,
      source: resolvedSource,
      signal,
      resolve,
      reject,
      cancel() {
        if (active === inspection) {
          stopWorker()
          complete(new DOMException('Aborted', 'AbortError'))
          return
        }
        const index = pending.indexOf(inspection)
        if (index !== -1) pending.splice(index, 1)
        signal?.removeEventListener('abort', inspection.cancel)
        reject(new DOMException('Aborted', 'AbortError'))
      },
    }
    signal?.addEventListener('abort', inspection.cancel, { once: true })
    pending.push(inspection)
    processNext()
  })
}
