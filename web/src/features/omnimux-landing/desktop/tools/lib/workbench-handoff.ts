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
import { useNavigate } from '@tanstack/react-router'
import { useCallback, useRef, type MouseEvent } from 'react'

import {
  takeGuestHandoff,
  writeGuestHandoff,
} from '@/features/influencer/lib/guest'
import { writeMotionMode } from '@/features/influencer/lib/motion-mode'
import { writeWorkbenchMode } from '@/features/influencer/lib/workbench-mode'

import type { ToolResult } from './api'

/** The viral-recreate slot accepts 4–30 s reference videos. */
const RECREATE_MIN_MS = 4000
const RECREATE_MAX_MS = 30000

export type ViralRecreateClick = { ok: true } | { ok: false; duration: string }

function formatDuration(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000))
  const minutes = String(Math.floor(total / 60)).padStart(2, '0')
  const seconds = String(total % 60).padStart(2, '0')
  return `${minutes}:${seconds}`
}

/**
 * Points the workbench at video · viral recreate and leaves the parsed work id
 * for it. This runs synchronously from the click: the workbench issues the
 * short-lived token after landing, so a slow `/handoff` can never delay the
 * route change. Only the work id leaves the page; no link or media address.
 */
export function prepareViralRecreate(
  result: ToolResult | null | undefined
): ViralRecreateClick {
  writeWorkbenchMode('video')
  writeMotionMode('recreate')
  // A leftover handoff from an earlier click must not fill this visit's slot.
  takeGuestHandoff()
  if (
    !result ||
    (result.kind !== 'video' && result.kind !== 'transcript') ||
    !result.video
  ) {
    return { ok: true }
  }
  const durationMs = result.video.duration_ms
  if (
    durationMs > 0 &&
    (durationMs < RECREATE_MIN_MS || durationMs > RECREATE_MAX_MS)
  ) {
    return { ok: false, duration: formatDuration(durationMs) }
  }
  writeGuestHandoff({
    workId: result.id,
    mode: 'video',
    motionMode: 'recreate',
  })
  return { ok: true }
}

/**
 * Click handler for a recreate entry that links to `/influencer`. A plain
 * click writes the handoff intent and navigates before any network request;
 * a modified click (new tab or window) keeps the browser default and lands on
 * the empty recreate slot, because the new tab cannot read this tab's session
 * storage.
 */
export function useViralRecreateEntry(result: ToolResult | null | undefined) {
  const navigate = useNavigate()
  const pending = useRef(false)
  return useCallback(
    (event: MouseEvent<HTMLAnchorElement>): ViralRecreateClick => {
      const modified =
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey ||
        event.button !== 0
      if (modified) {
        writeWorkbenchMode('video')
        writeMotionMode('recreate')
        return { ok: true }
      }
      event.preventDefault()
      if (pending.current) return { ok: true }
      const outcome = prepareViralRecreate(result)
      if (!outcome.ok) return outcome
      pending.current = true
      void Promise.resolve(navigate({ to: '/influencer' })).finally(() => {
        pending.current = false
      })
      return outcome
    },
    [navigate, result]
  )
}
