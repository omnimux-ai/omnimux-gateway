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

const DB_NAME = 'influencer-media-v1'
const STORE = 'thumbnails'
const VERSION = 1
const MAX_EDGE = 384
const MAX_ENTRIES = 200
const MAX_IMAGE_BYTES = 40 * 1024 * 1024
const VIDEO_TIMEOUT_MS = 3_000

export interface LocalThumbnail {
  blob: Blob
  width: number
  height: number
}

interface StoredThumbnail extends LocalThumbnail {
  key: string
  ownerId: number
  url: string
  createdAt: number
}

let databasePromise: Promise<IDBDatabase | null> | null = null
const listeners = new Set<() => void>()

/** Lets a late poster/thumbnail write refresh the cards that already rendered. */
export function subscribeUploadThumbnails(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

function notifyUploadThumbnails(): void {
  for (const listener of listeners) listener()
}

function openDatabase(): Promise<IDBDatabase | null> {
  if (databasePromise) return databasePromise
  if (typeof indexedDB === 'undefined') {
    databasePromise = Promise.resolve(null)
    return databasePromise
  }
  databasePromise = new Promise((resolve) => {
    let request: IDBOpenDBRequest
    try {
      request = indexedDB.open(DB_NAME, VERSION)
    } catch {
      resolve(null)
      return
    }
    request.addEventListener('upgradeneeded', () => {
      const database = request.result
      if (!database.objectStoreNames.contains(STORE)) {
        const store = database.createObjectStore(STORE, { keyPath: 'key' })
        store.createIndex('createdAt', 'createdAt')
      }
    })
    request.addEventListener('success', () => resolve(request.result), {
      once: true,
    })
    request.addEventListener('error', () => resolve(null), { once: true })
    request.addEventListener('blocked', () => resolve(null), { once: true })
  })
  return databasePromise
}

function keyFor(ownerId: number, url: string): string {
  return `${ownerId}:${url}`
}

function transactionDone(transaction: IDBTransaction): Promise<void> {
  return new Promise((resolve) => {
    transaction.addEventListener('complete', () => resolve(), { once: true })
    transaction.addEventListener('error', () => resolve(), { once: true })
    transaction.addEventListener('abort', () => resolve(), { once: true })
  })
}

/** Best-effort persistence: private mode or quota failures must never block upload. */
export async function putUploadThumbnail(
  ownerId: number | undefined,
  url: string,
  thumbnail: LocalThumbnail
): Promise<void> {
  if (typeof ownerId !== 'number' || !url) return
  const database = await openDatabase()
  if (!database) return
  try {
    const record: StoredThumbnail = {
      key: keyFor(ownerId, url),
      ownerId,
      url,
      createdAt: Date.now(),
      ...thumbnail,
    }
    const transaction = database.transaction(STORE, 'readwrite')
    transaction.objectStore(STORE).put(record)
    await transactionDone(transaction)
    notifyUploadThumbnails()
    void pruneThumbnails(ownerId)
  } catch {
    /* The card falls back to its original URL. */
  }
}

/** Reads the caller's local derivatives in one transaction; unknown URLs are skipped. */
export async function getUploadThumbnails(
  ownerId: number | undefined,
  urls: readonly string[]
): Promise<Map<string, Blob>> {
  const result = new Map<string, Blob>()
  if (typeof ownerId !== 'number' || urls.length === 0) return result
  const database = await openDatabase()
  if (!database) return result
  try {
    const transaction = database.transaction(STORE, 'readonly')
    const store = transaction.objectStore(STORE)
    await Promise.all(
      urls.map(
        (url) =>
          new Promise<void>((resolve) => {
            const request = store.get(keyFor(ownerId, url))
            request.addEventListener(
              'success',
              () => {
                const record = request.result as StoredThumbnail | undefined
                if (record?.blob) result.set(url, record.blob)
                resolve()
              },
              { once: true }
            )
            request.addEventListener('error', () => resolve(), { once: true })
          })
      )
    )
    await transactionDone(transaction)
  } catch {
    /* Fall back to the original URL for every card. */
  }
  return result
}

export async function pruneThumbnails(ownerId: number): Promise<void> {
  const database = await openDatabase()
  if (!database) return
  try {
    const transaction = database.transaction(STORE, 'readwrite')
    const store = transaction.objectStore(STORE)
    const request = store.getAll()
    request.addEventListener('success', () => {
      const records = (request.result as StoredThumbnail[])
        .filter((record) => record.ownerId === ownerId)
        .sort((a, b) => b.createdAt - a.createdAt)
      records.forEach((record, index) => {
        // Upload records outlive this derivative cache; only the bounded entry
        // count may evict a thumbnail, never its age.
        if (index >= MAX_ENTRIES) store.delete(record.key)
      })
    })
    await transactionDone(transaction)
  } catch {
    /* Pruning is opportunistic. */
  }
}

function createCanvas(width: number, height: number) {
  if (typeof OffscreenCanvas !== 'undefined') {
    return new OffscreenCanvas(width, height)
  }
  const canvas = document.createElement('canvas')
  canvas.width = width
  canvas.height = height
  return canvas
}

async function canvasToBlob(
  canvas: HTMLCanvasElement | OffscreenCanvas,
  type: string,
  quality: number
): Promise<Blob | null> {
  if ('convertToBlob' in canvas) {
    return canvas.convertToBlob({ type, quality })
  }
  return new Promise((resolve) => canvas.toBlob(resolve, type, quality))
}

function scaledSize(width: number, height: number) {
  const scale = Math.min(1, MAX_EDGE / Math.max(width, height))
  return {
    width: Math.max(1, Math.round(width * scale)),
    height: Math.max(1, Math.round(height * scale)),
  }
}

/** Produces a bounded WebP derivative, falling back to JPEG, never the original. */
export async function generateImageThumbnail(
  file: File
): Promise<LocalThumbnail | null> {
  if (file.size > MAX_IMAGE_BYTES) return null
  if (typeof createImageBitmap === 'function') {
    let bitmap: ImageBitmap | null = null
    try {
      bitmap = await createImageBitmap(file, { imageOrientation: 'from-image' })
      const size = scaledSize(bitmap.width, bitmap.height)
      const canvas = createCanvas(size.width, size.height)
      const context = canvas.getContext('2d') as
        | CanvasRenderingContext2D
        | OffscreenCanvasRenderingContext2D
        | null
      if (!context) return null
      context.drawImage(bitmap, 0, 0, size.width, size.height)
      const blob =
        (await canvasToBlob(canvas, 'image/webp', 0.82)) ??
        (await canvasToBlob(canvas, 'image/jpeg', 0.82))
      return blob ? { blob, ...size } : null
    } catch {
      return null
    } finally {
      bitmap?.close()
    }
  }
  return null
}

function waitForMedia(
  element: HTMLVideoElement,
  event: string,
  timeoutMs: number
): Promise<void> {
  return new Promise((resolve, reject) => {
    const timer = window.setTimeout(() => {
      cleanup()
      reject(new Error('media timeout'))
    }, timeoutMs)
    const onEvent = () => {
      cleanup()
      resolve()
    }
    const onError = () => {
      cleanup()
      reject(new Error('media failed'))
    }
    const cleanup = () => {
      window.clearTimeout(timer)
      element.removeEventListener(event, onEvent)
      element.removeEventListener('error', onError)
    }
    element.addEventListener(event, onEvent, { once: true })
    element.addEventListener('error', onError, { once: true })
  })
}

/** Best-effort first-frame poster for a local video; upload never waits for it. */
export async function captureVideoPoster(
  file: File
): Promise<LocalThumbnail | null> {
  if (typeof document === 'undefined') return null
  const url = URL.createObjectURL(file)
  const video = document.createElement('video')
  video.muted = true
  video.playsInline = true
  video.preload = 'metadata'
  video.src = url
  try {
    await waitForMedia(video, 'loadedmetadata', VIDEO_TIMEOUT_MS)
    const duration = Number.isFinite(video.duration) ? video.duration : 0
    video.currentTime = duration > 0 ? Math.min(0.2, duration / 2) : 0
    await waitForMedia(video, 'seeked', VIDEO_TIMEOUT_MS)
    if (!video.videoWidth || !video.videoHeight) return null
    const size = scaledSize(video.videoWidth, video.videoHeight)
    const canvas = createCanvas(size.width, size.height)
    const context = canvas.getContext('2d') as
      | CanvasRenderingContext2D
      | OffscreenCanvasRenderingContext2D
      | null
    if (!context) return null
    context.drawImage(video, 0, 0, size.width, size.height)
    const blob =
      (await canvasToBlob(canvas, 'image/jpeg', 0.82)) ??
      (await canvasToBlob(canvas, 'image/webp', 0.82))
    return blob ? { blob, ...size } : null
  } catch {
    return null
  } finally {
    video.removeAttribute('src')
    video.load()
    URL.revokeObjectURL(url)
  }
}
