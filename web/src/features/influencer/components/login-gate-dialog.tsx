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
import { Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'

import { trackInfluencerEvent, INFLUENCER_ANALYTICS_EVENTS } from '../lib/track'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** Whether the guest already assembled parameters worth restoring. */
  hasDraft?: boolean
}

/**
 * The conversion gate shown to guests on any write action. The dialog keeps
 * the visitor on the page (no hard redirect) and offers sign-in / sign-up as
 * the only exits — the look they assembled is already snapshotted before this
 * opens.
 */
export function LoginGateDialog(props: Props) {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const go = (to: '/sign-in' | '/sign-up') => {
    trackInfluencerEvent(INFLUENCER_ANALYTICS_EVENTS.LOGIN_CTA, {
      action: to === '/sign-in' ? 'sign_in' : 'sign_up',
    })
    props.onOpenChange(false)
    void navigate({ to, search: { redirect: '/influencer' } })
  }

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      {/* The portal escapes `.influencer-page`; re-hang the scope class or the
          --inf-* tokens and the .inf-cta button lose their theme. */}
      <DialogContent className='influencer-page sm:max-w-sm'>
        <DialogHeader>
          <div className='mx-auto mb-1 flex h-11 w-11 items-center justify-center rounded-full bg-[var(--inf-surface-2)]'>
            <Sparkles className='h-5 w-5 text-[var(--inf-txt)]' aria-hidden='true' />
          </div>
          <DialogTitle className='text-center text-[var(--inf-txt)]'>
            {t('Log in to generate')}
          </DialogTitle>
          <DialogDescription className='text-center text-[var(--inf-txt-3)]'>
            {props.hasDraft
              ? t(
                  'Your selections are saved — sign in to create this character and keep it in your history.'
                )
              : t(
                  'Sign in to start generating characters and keep them in your history.'
                )}
          </DialogDescription>
        </DialogHeader>
        <DialogFooter className='flex-col gap-2 sm:flex-col'>
          <button type='button' className='inf-cta w-full' onClick={() => go('/sign-in')}>
            <span>{t('Sign in')}</span>
          </button>
          <button
            type='button'
            className='inf-abtn w-full justify-center'
            onClick={() => go('/sign-up')}
          >
            {t('Create a free account')}
          </button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
