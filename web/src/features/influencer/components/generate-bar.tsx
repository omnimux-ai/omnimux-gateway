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
import { Shuffle } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'
import { formatCredits } from '@/features/pricing/lib/credits'

interface Props {
  summary?: ReactNode
  quote: { usd: number; credits: number } | null
  canGenerate: boolean
  submitting: boolean
  /** Visitors see a sign-in CTA instead of the credit quote. */
  isGuest?: boolean
  /** Omitted by the video form, which has nothing to randomize; the CTA then
   * takes the full width on its own. */
  onRandomize?: () => void
  /** The visitor owns every visible category, so a roll would change nothing. */
  randomizeDisabled?: boolean
  /** Why the dice is disabled, kept in its own tooltip (never inline copy). */
  randomizeHint?: string
  /** Overrides the CTA wording; the video form asks for a video, not a sheet. */
  ctaLabel?: string
  /** Disabled-state reason shown as a native tooltip instead of inline copy. */
  ctaTitle?: string
  /** Optional wording next to the spinner while a multi-step action runs. */
  busyLabel?: string
  onGenerate: () => void
}

export function GenerateBar(props: Props) {
  const { t } = useTranslation()
  return (
    <footer className='inf-generate-bar shrink-0 border-t border-[var(--inf-line)] bg-[var(--inf-surface)] px-3 py-3'>
      {props.summary}
      <div className='flex items-center gap-2'>
        {props.onRandomize ? (
          <Button
            variant='ghost'
            type='button'
            className='inf-dice'
            title={
              props.randomizeDisabled ? props.randomizeHint : t('Randomize')
            }
            aria-label={t('Randomize')}
            aria-disabled={props.randomizeDisabled ? true : undefined}
            disabled={props.randomizeDisabled}
            onClick={props.onRandomize}
          >
            <Shuffle className='h-5 w-5' aria-hidden='true' />
          </Button>
        ) : null}

        <Button
          type='button'
          className='inf-cta'
          disabled={!props.canGenerate || props.submitting}
          title={props.ctaTitle}
          aria-busy={props.submitting}
          aria-label={
            props.submitting
              ? t(props.busyLabel ?? props.ctaLabel ?? 'Generate')
              : undefined
          }
          onClick={props.onGenerate}
        >
          {props.submitting ? (
            <>
              <Spinner aria-label={t('Loading')} />
              {props.busyLabel ? <span>{t(props.busyLabel)}</span> : null}
            </>
          ) : (
            <span>
              {props.isGuest ? t('Log in') : t(props.ctaLabel ?? 'Generate')}
            </span>
          )}
          {props.quote && !props.submitting && !props.isGuest && (
            <span className='text-sm font-bold'>
              ✦ {formatCredits(props.quote.credits)}
            </span>
          )}
        </Button>
      </div>
    </footer>
  )
}
