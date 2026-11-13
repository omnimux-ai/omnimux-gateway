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
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { takeGuestHandoff } from '../lib/guest'
import { redeemToolHandoff } from '../lib/tool-handoff'
import type { MotionMode, WorkbenchMode } from '../types'

/** One message for every handoff that cannot reach the slot. */
export const TOOL_HANDOFF_FAILED_MESSAGE =
  'This video is unavailable. Choose another video or upload a file.'

/**
 * Brings a tool-page video into the viral-recreate reference slot once per
 * handoff. Guests leave the record in place: the redeemed copy belongs to an
 * account, so the same tab consumes it after signing in. Any failure leaves
 * the empty recreate slot and shows one toast; a video the slot itself
 * rejects is already reported by the slot.
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
}) {
  const { t } = useTranslation()
  const started = useRef(false)
  const latest = useRef(options)
  latest.current = options

  useEffect(() => {
    if (options.isGuest || started.current) return
    started.current = true
    const handoff = takeGuestHandoff()
    if (!handoff) return
    const current = latest.current
    if (current.workbenchMode !== handoff.mode) {
      current.onWorkbenchModeChange(handoff.mode)
    }
    if (current.motionMode !== handoff.motionMode) {
      current.setMotionMode(handoff.motionMode)
    }
    const failed = () => toast.error(t(TOOL_HANDOFF_FAILED_MESSAGE))
    if (!handoff.token) {
      failed()
      return
    }
    void redeemToolHandoff({ token: handoff.token }).then(
      (video) =>
        latest.current.setRemoteVideo(
          video.url,
          video.name,
          video.preview_url,
          'recreate'
        ),
      failed
    )
  }, [options.isGuest, t])
}
