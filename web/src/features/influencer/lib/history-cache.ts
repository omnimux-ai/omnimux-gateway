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
import { isTerminalStatus } from './history'

const KEY = 'influencer:history-cache:v1'
const VERSION = 1
/** The gallery pages in from the network past this; the cache only buys a
 * instant first paint, so it stays small enough for a synchronous read. */
const MAX_ITEMS = 60

interface HistoryCacheEnvelope {
  v: number
  userId: number
  savedAt: number
  total: number
  items: TaskRecord[]
}

export interface HistoryCache {
  items: TaskRecord[]
  total: number
  savedAt: number
}

/** Storage access itself throws when cookies are blocked, so every entry point
 * goes through this guard instead of touching `localStorage` directly. */
function storage(): Storage | null {
  try {
    return typeof localStorage === 'undefined' ? null : localStorage
  } catch {
    return null
  }
}

/**
 * Reads the records cached for this account. A payload written by another
 * account, an older version or a corrupted value is dropped rather than
 * surfaced — showing one user's tasks to another is not an acceptable fallback.
 */
export function readHistoryCache(userId: number | undefined): HistoryCache | null {
  if (typeof userId !== 'number') return null
  const store = storage()
  if (!store) return null
  try {
    const raw = store.getItem(KEY)
    if (!raw) return null
    const parsed = JSON.parse(raw) as Partial<HistoryCacheEnvelope>
    if (
      parsed?.v !== VERSION ||
      parsed.userId !== userId ||
      !Array.isArray(parsed.items)
    ) {
      store.removeItem(KEY)
      return null
    }
    const items = parsed.items.filter(
      (item) => item && isTerminalStatus(item.status)
    )
    if (!items.length) return null
    return {
      items,
      total: typeof parsed.total === 'number' ? parsed.total : items.length,
      savedAt: typeof parsed.savedAt === 'number' ? parsed.savedAt : 0,
    }
  } catch {
    try {
      store.removeItem(KEY)
    } catch {
      /* nothing left to recover */
    }
    return null
  }
}

/**
 * Caches the newest terminal records. Running tasks are skipped: their status
 * and artifacts still change, so replaying them would show a stale gallery.
 */
export function writeHistoryCache(
  userId: number | undefined,
  items: TaskRecord[],
  total: number
): void {
  if (typeof userId !== 'number') return
  const store = storage()
  if (!store) return
  const terminal = items
    .filter((item) => item && isTerminalStatus(item.status))
    .slice(0, MAX_ITEMS)
  if (!terminal.length) return
  const envelope: HistoryCacheEnvelope = {
    v: VERSION,
    userId,
    savedAt: Date.now(),
    total,
    items: terminal,
  }
  try {
    store.setItem(KEY, JSON.stringify(envelope))
  } catch {
    /* Quota exceeded or private mode: the gallery still loads from the network. */
  }
}

export function clearHistoryCache(): void {
  const store = storage()
  if (!store) return
  try {
    store.removeItem(KEY)
  } catch {
    /* nothing to clear */
  }
}
