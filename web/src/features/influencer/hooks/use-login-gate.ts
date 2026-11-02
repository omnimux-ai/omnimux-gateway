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

import { useCallback, useState } from 'react'

import { useAuthStore } from '@/stores/auth-store'

import { writeGuestDraft, type GuestDraft } from '../lib/guest'
import {
  trackInfluencerEvent,
  INFLUENCER_ANALYTICS_EVENTS,
  type LoginGateSource,
} from '../lib/track'

/**
 * Single funnel point for every write action on the workbench. Authed callers
 * run the action straight away; guests snapshot their draft, open the login
 * gate dialog and the action is dropped — after sign-in they land back on the
 * page and the draft restores.
 */
export function useLoginGate() {
  const isGuest = useAuthStore((s) => !s.auth.user || !s.auth.accessToken)
  const [open, setOpen] = useState(false)

  const requireLogin = useCallback(
    (source: LoginGateSource, draft: GuestDraft, action: () => void) => {
      if (!isGuest) {
        action()
        return
      }
      writeGuestDraft(draft)
      setOpen(true)
      trackInfluencerEvent(INFLUENCER_ANALYTICS_EVENTS.LOGIN_CTA, {
        source,
      })
    },
    [isGuest]
  )

  return {
    isGuest,
    gateOpen: open,
    requireLogin,
    onGateOpenChange: setOpen,
  }
}
