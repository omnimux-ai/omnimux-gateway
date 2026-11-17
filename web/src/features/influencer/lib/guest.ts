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

import type { ImageMode, Selection, WorkbenchMode } from '../types'
import { selectionSchema } from './builder-draft'

/**
 * Guest-browse draft: the look a visitor assembles before signing in is kept
 * in sessionStorage so the post-login redirect can restore it verbatim.
 * sessionStorage, not localStorage — a draft that outlives the tab would
 * surprise the next session on a shared browser.
 */
const KEY = 'influencer:guest-draft:v1'

export interface GuestDraft {
  tier: string
  selection: Selection
  /** Hand picks the visitor owned before signing in; the only locks worth carrying. */
  locks?: Selection
  brief: string
  seed: string
  imageUrl: string
  imageMode?: ImageMode
  imageMimeType?: string
  maskUrl?: string
  editInstruction?: string
  model: string
  group: string
  vendorId: number | null
  /**
   * Which workbench face the visitor meant to start on. Absent means "let the
   * studio decide"; only the landing's photo entry sets it.
   */
  mode?: WorkbenchMode
}

export function writeGuestDraft(draft: GuestDraft): void {
  try {
    const imageUrl = draft.imageUrl.trim()
    const stored: GuestDraft = {
      ...draft,
      brief: '',
      imageUrl,
      imageMode: imageUrl ? 'reference' : 'text',
    }
    delete stored.maskUrl
    delete stored.editInstruction
    sessionStorage.setItem(KEY, JSON.stringify(stored))
  } catch {
    // A full or unavailable store must never block the sign-in journey.
  }
}

/**
 * Reads and removes the draft — it is consumed once, so a stale look cannot
 * leak into a later, unrelated visit.
 */
export function takeGuestDraft(): GuestDraft | null {
  try {
    const raw = sessionStorage.getItem(KEY)
    if (!raw) return null
    sessionStorage.removeItem(KEY)
    const parsed = JSON.parse(raw) as Partial<GuestDraft> & {
      manualSelection?: unknown
    }
    if (typeof parsed !== 'object' || parsed === null) return null
    const selection = selectionSchema.safeParse(parsed.selection ?? {})
    if (!selection.success) return null
    // `manualSelection` is the previous release's name for the same layer, kept
    // readable so a draft written moments before a deploy still lands.
    const rawLocks = parsed.locks ?? parsed.manualSelection
    const locks =
      rawLocks === undefined ? null : selectionSchema.safeParse(rawLocks)
    if (locks && !locks.success) return null
    return {
      // An empty tier means the landing asked for no particular look, so the
      // studio's own default stands in rather than an empty (unselectable) one.
      tier:
        typeof parsed.tier === 'string' && parsed.tier ? parsed.tier : 'total',
      selection: selection.data,
      locks: locks?.data,
      brief: '',
      seed: typeof parsed.seed === 'string' ? parsed.seed : '',
      imageUrl: typeof parsed.imageUrl === 'string' ? parsed.imageUrl : '',
      imageMimeType:
        typeof parsed.imageMimeType === 'string'
          ? parsed.imageMimeType
          : undefined,
      imageMode:
        typeof parsed.imageUrl === 'string' && parsed.imageUrl.trim()
          ? 'reference'
          : 'text',
      model: typeof parsed.model === 'string' ? parsed.model : '',
      group: typeof parsed.group === 'string' ? parsed.group : '',
      vendorId: typeof parsed.vendorId === 'number' ? parsed.vendorId : null,
      mode:
        parsed.mode === 'video' || parsed.mode === 'image'
          ? parsed.mode
          : undefined,
    }
  } catch {
    return null
  }
}

/**
 * Tool-page handoff: the work id of one parsed TikTok video, plus the face and
 * sub-mode it promised. It lives beside the look draft (same tab,
 * sessionStorage) and survives the sign-in round trip because only a signed-in
 * workbench consumes it. It never holds a link or a media address.
 *
 * The tool page writes only `workId` and navigates immediately; the workbench
 * issues the short-lived opaque token after landing. `token` stays readable so
 * a tab that clicked just before a deploy still finishes its handoff.
 * `failed` records a legacy handoff that could not be issued.
 */
const HANDOFF_KEY = 'influencer:guest-handoff:v1'

export interface GuestHandoff {
  /** Parsed TikTok work id; the workbench issues its own token after landing. */
  workId?: string
  /** Legacy token issued on the tool page before the click-no-wait change. */
  token?: string
  failed?: boolean
  /** Epoch milliseconds after which the server has already dropped the token. */
  expiresAt?: number
  mode: 'video'
  motionMode: 'recreate'
}

export function writeGuestHandoff(handoff: GuestHandoff): void {
  try {
    sessionStorage.setItem(HANDOFF_KEY, JSON.stringify(handoff))
  } catch {
    // An unavailable store only means the workbench opens with an empty slot.
  }
}

function parseGuestHandoff(
  parsed: Partial<GuestHandoff> | null,
  now: number
): GuestHandoff | null {
  if (typeof parsed !== 'object' || parsed === null) return null
  if (parsed.failed === true) {
    return { failed: true, mode: 'video', motionMode: 'recreate' }
  }
  const workId =
    typeof parsed.workId === 'string' && /^\d{5,32}$/.test(parsed.workId)
      ? parsed.workId
      : undefined
  const token =
    typeof parsed.token === 'string' && /^[\w-]{43}$/.test(parsed.token)
      ? parsed.token
      : undefined
  if (!workId && !token) return null
  if (token) {
    if (typeof parsed.expiresAt !== 'number' || parsed.expiresAt <= now) {
      return null
    }
    return {
      token,
      expiresAt: parsed.expiresAt,
      mode: 'video',
      motionMode: 'recreate',
    }
  }
  return {
    workId,
    mode: 'video',
    motionMode: 'recreate',
  }
}

/** Reads a handoff without consuming it, so the first paint can show loading. */
export function peekGuestHandoff(now = Date.now()): GuestHandoff | null {
  try {
    const raw = sessionStorage.getItem(HANDOFF_KEY)
    if (!raw) return null
    return parseGuestHandoff(
      JSON.parse(raw) as Partial<GuestHandoff> | null,
      now
    )
  } catch {
    return null
  }
}

/** Reads and removes the handoff; malformed or expired records read as null. */
export function takeGuestHandoff(now = Date.now()): GuestHandoff | null {
  try {
    const raw = sessionStorage.getItem(HANDOFF_KEY)
    if (!raw) return null
    sessionStorage.removeItem(HANDOFF_KEY)
    return parseGuestHandoff(
      JSON.parse(raw) as Partial<GuestHandoff> | null,
      now
    )
  } catch {
    return null
  }
}
