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

import { z } from 'zod'

/**
 * The account's reference uploads shown under the pickers' "Uploaded" tab.
 *
 * The gateway keeps no per-user listing of these files, so the browser keeps
 * the record. Each account has its own key; a record leaves the list shortly
 * before the gateway deletes its file, so the list never offers a dead URL.
 */
const KEY_PREFIX = 'influencer:upload-library:v1:'
const VERSION = 1
/** Per kind; the oldest records fall off first. */
export const UPLOAD_LIBRARY_LIMIT = 50
/** Leaves time for a generation to fetch the file after it is picked. */
export const UPLOAD_EXPIRY_MARGIN_MS = 5 * 60 * 1000

const metadataSchema = z.object({
  version: z.literal(1).optional(),
  browserMetadata: z
    .object({
      duration: z.number().finite().positive(),
      video_duration: z.number().finite().positive(),
      width: z.number().int().positive(),
      height: z.number().int().positive(),
      frame_rate: z.number().finite().positive(),
      video_codec: z.string(),
      audio_codec: z.string(),
      container: z.string(),
      size_bytes: z.number().int().positive(),
    })
    .optional(),
  rawDurationSec: z.number().finite().positive(),
  durationSec: z.number().finite().positive(),
  width: z.number().finite().positive(),
  height: z.number().finite().positive(),
  aspectRatio: z.enum(['16:9', '9:16', '3:4']).nullable(),
})

const recordSchema = z.object({
  kind: z.enum(['image', 'video']),
  url: z.string().min(1).max(4096),
  fileName: z.string().max(512),
  uploadedAt: z.number().finite().nonnegative(),
  expiresAt: z.number().finite().positive(),
  metadata: metadataSchema.optional(),
})

const envelopeSchema = z.object({
  v: z.literal(VERSION),
  items: z.array(z.unknown()),
})

export type UploadKind = z.infer<typeof recordSchema>['kind']
export type UploadRecord = z.infer<typeof recordSchema>

function storage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage
  } catch {
    return null
  }
}

function keyFor(ownerId: number): string {
  return `${KEY_PREFIX}${ownerId}`
}

function usable(record: UploadRecord, now: number): boolean {
  return record.expiresAt - UPLOAD_EXPIRY_MARGIN_MS > now
}

/** Newest first, unique by URL, unexpired, and capped per kind. */
export function normalizeUploads(
  records: readonly UploadRecord[],
  now = Date.now()
): UploadRecord[] {
  const seen = new Set<string>()
  const counts: Record<UploadKind, number> = { image: 0, video: 0 }
  return [...records]
    .sort((a, b) => b.uploadedAt - a.uploadedAt)
    .filter((record) => {
      if (!usable(record, now) || seen.has(record.url)) return false
      if (counts[record.kind] >= UPLOAD_LIBRARY_LIMIT) return false
      seen.add(record.url)
      counts[record.kind] += 1
      return true
    })
}

/** Guests and unreadable storage get an empty list; one bad record never hides the rest. */
export function readUploadLibrary(
  ownerId: number | undefined,
  now = Date.now()
): UploadRecord[] {
  if (typeof ownerId !== 'number') return []
  const store = storage()
  if (!store) return []
  try {
    const raw = store.getItem(keyFor(ownerId))
    if (!raw) return []
    const envelope = envelopeSchema.safeParse(JSON.parse(raw))
    if (!envelope.success) {
      store.removeItem(keyFor(ownerId))
      return []
    }
    const records = envelope.data.items.flatMap((item) => {
      const parsed = recordSchema.safeParse(item)
      return parsed.success ? [parsed.data] : []
    })
    return normalizeUploads(records, now)
  } catch {
    try {
      store.removeItem(keyFor(ownerId))
    } catch {
      /* nothing left to recover */
    }
    return []
  }
}

export function writeUploadLibrary(
  ownerId: number | undefined,
  records: readonly UploadRecord[]
): void {
  if (typeof ownerId !== 'number') return
  const store = storage()
  if (!store) return
  try {
    if (records.length === 0) {
      store.removeItem(keyFor(ownerId))
      return
    }
    store.setItem(
      keyFor(ownerId),
      JSON.stringify({ v: VERSION, items: records })
    )
  } catch {
    /* Quota exceeded or private mode: the list still works for this visit. */
  }
}

/*
 * One list per account shared by every upload entry on the page. Guests have
 * none. The cache also keeps the list for this visit when storage is
 * unavailable.
 */
const cache = new Map<number, UploadRecord[]>()
const listeners = new Set<() => void>()
const EMPTY: UploadRecord[] = []

/** Returns a stable array until the list changes or one of its records expires. */
export function getUploads(
  ownerId: number | undefined,
  now = Date.now()
): UploadRecord[] {
  if (typeof ownerId !== 'number') return EMPTY
  const key = ownerId
  const cached = cache.get(key)
  if (cached && cached.every((record) => usable(record, now))) return cached
  const records = cached
    ? normalizeUploads(cached, now)
    : readUploadLibrary(ownerId, now)
  const next = records.length ? records : EMPTY
  cache.set(key, next)
  return next
}

/**
 * Records a finished upload. Storage is re-read first so an upload made in
 * another tab of the same account is kept.
 */
export function rememberUpload(
  ownerId: number | undefined,
  record: UploadRecord,
  now = Date.now()
): void {
  if (typeof ownerId !== 'number') return
  const next = normalizeUploads(
    [record, ...readUploadLibrary(ownerId, now), ...getUploads(ownerId, now)],
    now
  )
  cache.set(ownerId, next)
  writeUploadLibrary(ownerId, next)
  for (const listener of listeners) listener()
}

export function subscribeUploads(listener: () => void): () => void {
  listeners.add(listener)
  const onStorage = (event: StorageEvent) => {
    if (event.key !== null && !event.key.startsWith(KEY_PREFIX)) return
    cache.clear()
    listener()
  }
  window.addEventListener('storage', onStorage)
  return () => {
    listeners.delete(listener)
    window.removeEventListener('storage', onStorage)
  }
}

/** Drops the in-memory copy so the next read comes from storage. */
export function resetUploadCache(): void {
  cache.clear()
}
