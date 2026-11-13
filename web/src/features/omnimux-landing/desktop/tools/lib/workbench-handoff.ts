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
import { issueToolHandoff } from '@/features/influencer/lib/tool-handoff'
import { writeWorkbenchMode } from '@/features/influencer/lib/workbench-mode'

import type { ToolResult } from './api'

/** The viral-recreate slot accepts 4–30 s reference videos. */
const RECREATE_MIN_MS = 4000
const RECREATE_MAX_MS = 30000

/**
 * Points the workbench at video · viral recreate and, when a video was parsed,
 * leaves a handoff for it. Overwrites the remembered surface and sub-mode.
 * Only the work id leaves the page; the token it gets back carries no link.
 */
export async function prepareViralRecreate(
  result: ToolResult | null | undefined
): Promise<void> {
  writeWorkbenchMode('video')
  writeMotionMode('recreate')
  // A leftover handoff from an earlier click must not fill this visit's slot.
  takeGuestHandoff()
  if (!result || result.kind !== 'video' || !result.video) return
  const target = { mode: 'video', motionMode: 'recreate' } as const
  const durationMs = result.video.duration_ms
  if (
    durationMs > 0 &&
    (durationMs < RECREATE_MIN_MS || durationMs > RECREATE_MAX_MS)
  ) {
    writeGuestHandoff({ ...target, failed: true })
    return
  }
  try {
    const issued = await issueToolHandoff({ id: result.id })
    writeGuestHandoff({
      ...target,
      token: issued.token,
      expiresAt: Date.now() + issued.expires_in * 1000,
    })
  } catch {
    writeGuestHandoff({ ...target, failed: true })
  }
}

/**
 * Click handler for a recreate entry that links to `/influencer`. A plain
 * click waits for the handoff and then navigates; a modified click (new tab or
 * window) keeps the browser default and lands on the empty recreate slot,
 * because the new tab cannot read this tab's session storage.
 */
export function useViralRecreateEntry(result: ToolResult | null | undefined) {
  const navigate = useNavigate()
  const pending = useRef(false)
  return useCallback(
    (event: MouseEvent<HTMLAnchorElement>) => {
      const modified =
        event.metaKey ||
        event.ctrlKey ||
        event.shiftKey ||
        event.altKey ||
        event.button !== 0
      if (modified) {
        writeWorkbenchMode('video')
        writeMotionMode('recreate')
        return
      }
      event.preventDefault()
      if (pending.current) return
      pending.current = true
      void prepareViralRecreate(result).finally(() => {
        pending.current = false
        void navigate({ to: '/influencer' })
      })
    },
    [navigate, result]
  )
}
