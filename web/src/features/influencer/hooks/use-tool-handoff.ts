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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  peekGuestHandoff,
  takeGuestHandoff,
  type GuestHandoff,
} from '../lib/guest'
import {
  issueToolHandoff,
  redeemToolHandoff,
  ToolHandoffError,
  ToolHandoffTimeoutError,
} from '../lib/tool-handoff'
import type { MotionMode, WorkbenchMode } from '../types'

/** One message for a handoff that cannot reach the slot. */
export const TOOL_HANDOFF_FAILED_MESSAGE =
  'Could not download this video. Please try again.'
export const TOOL_HANDOFF_TIMEOUT_MESSAGE =
  'Video loading timed out. Please retry.'
export const TOOL_HANDOFF_DURATION_MESSAGE =
  'Reference videos must be 4–30 seconds'
export const TOOL_HANDOFF_LOADING_MESSAGE = 'Loading video…'
export const TOOL_HANDOFF_STILL_LOADING_MESSAGE = 'Still loading video…'

export type ToolHandoffStatus = 'idle' | 'loading' | 'failed'

export interface ToolHandoffState {
  status: ToolHandoffStatus
  /** The slot label; switches once the wait passes ten seconds. */
  loadingLabel: string
  /** Empty until the slot fails. */
  error: string
  retry: () => void
}

function handoffErrorMessage(error: unknown): string {
  if (error instanceof ToolHandoffTimeoutError) {
    return TOOL_HANDOFF_TIMEOUT_MESSAGE
  }
  if (error instanceof ToolHandoffError && error.code === 'handoff_duration') {
    return TOOL_HANDOFF_DURATION_MESSAGE
  }
  return TOOL_HANDOFF_FAILED_MESSAGE
}

/**
 * Brings a tool-page video into the viral-recreate reference slot once per
 * handoff. The tool page leaves only a work id and navigates immediately; this
 * hook issues the token and redeems it after landing, exposing a loading state
 * so the slot is never blank. Guests leave the record in place: the redeemed
 * copy belongs to an account, so the same tab consumes it after signing in.
 */
export function useToolHandoff(options: {
  isGuest: boolean
  workbenchMode: WorkbenchMode
  onWorkbenchModeChange: (next: WorkbenchMode) => void
  motionMode: MotionMode
  setMotionMode: (next: MotionMode) => void
  setRemoteVideo: (
    url: string,
    name: string,
    previewUrl?: string,
    modeOverride?: MotionMode
  ) => Promise<boolean>
}): ToolHandoffState {
  const { t } = useTranslation()
  const started = useRef(false)
  const latest = useRef(options)
  latest.current = options
  const handoffRef = useRef<GuestHandoff | null>(null)
  const tokenRef = useRef<string | undefined>(undefined)
  const attemptRef = useRef(0)
  const stallTimerRef = useRef<ReturnType<typeof setTimeout> | undefined>(
    undefined
  )
  const [status, setStatus] = useState<ToolHandoffStatus>(() =>
    !options.isGuest && peekGuestHandoff() ? 'loading' : 'idle'
  )
  const [stalled, setStalled] = useState(false)
  const [error, setError] = useState('')

  const clearStallTimer = useCallback(() => {
    if (stallTimerRef.current) {
      clearTimeout(stallTimerRef.current)
      stallTimerRef.current = undefined
    }
  }, [])

  const run = useCallback(
    async (handoff: GuestHandoff) => {
      const attempt = ++attemptRef.current
      setStatus('loading')
      setStalled(false)
      setError('')
      clearStallTimer()
      stallTimerRef.current = setTimeout(() => {
        if (attempt === attemptRef.current) setStalled(true)
      }, 10_000)
      try {
        let token = tokenRef.current ?? handoff.token
        if (!token && handoff.workId) {
          const issued = await issueToolHandoff({ id: handoff.workId })
          token = issued.token
          tokenRef.current = token
        }
        if (!token) throw new ToolHandoffError('handoff_unavailable')
        const video = await redeemToolHandoff({ token })
        const accepted = await latest.current.setRemoteVideo(
          video.url,
          video.name,
          video.preview_url,
          'recreate'
        )
        if (attempt !== attemptRef.current) return
        setStatus('idle')
        if (!accepted) setError('')
      } catch (failure) {
        if (attempt !== attemptRef.current) return
        const message = handoffErrorMessage(failure)
        setStatus('failed')
        setError(t(message))
        toast.error(t(message))
      } finally {
        if (attempt === attemptRef.current) clearStallTimer()
      }
    },
    [clearStallTimer, t]
  )

  const retry = useCallback(() => {
    const handoff = handoffRef.current
    if (handoff) void run(handoff)
  }, [run])

  useEffect(() => {
    if (options.isGuest || started.current) return
    started.current = true
    const handoff = takeGuestHandoff()
    if (!handoff) {
      setStatus((current) => (current === 'loading' ? 'idle' : current))
      return
    }
    handoffRef.current = handoff
    const current = latest.current
    if (current.workbenchMode !== handoff.mode) {
      current.onWorkbenchModeChange(handoff.mode)
    }
    if (current.motionMode !== handoff.motionMode) {
      current.setMotionMode(handoff.motionMode)
    }
    if (handoff.failed) {
      setStatus('failed')
      setError(t(TOOL_HANDOFF_FAILED_MESSAGE))
      toast.error(t(TOOL_HANDOFF_FAILED_MESSAGE))
      return
    }
    void run(handoff)
  }, [options.isGuest, run, t])

  useEffect(
    () => () => {
      attemptRef.current += 1
      clearStallTimer()
    },
    [clearStallTimer]
  )

  return {
    status,
    loadingLabel: stalled
      ? t(TOOL_HANDOFF_STILL_LOADING_MESSAGE)
      : t(TOOL_HANDOFF_LOADING_MESSAGE),
    error,
    retry,
  }
}
