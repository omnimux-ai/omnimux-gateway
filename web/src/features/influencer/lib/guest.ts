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
