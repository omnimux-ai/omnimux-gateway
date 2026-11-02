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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import type { FailReasonView } from '../lib/fail-reason'

interface Props {
  view: FailReasonView
  /** Layout hook for the surface that renders the disclosure. */
  className?: string
  /** Extra classes for the original-text block. */
  rawClassName?: string
}

/**
 * Secondary disclosure for the original server text.
 *
 * The primary failure sentence is always localized; the raw text stays one
 * click away for support without becoming the message a user has to read in a
 * language they do not understand.
 */
export function FailReasonDetail(props: Props) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  if (!props.view.showDetail) return null
  return (
    <div
      className={cn(
        'flex w-full flex-col items-center gap-1.5',
        props.className
      )}
    >
      <button
        type='button'
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
        className='rounded-sm text-[11px] leading-4 text-[var(--inf-txt-3)] underline underline-offset-2 transition-colors hover:text-[var(--inf-txt)] focus-visible:text-[var(--inf-txt)] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-[var(--inf-brand)]'
      >
        {t('Details')}
      </button>
      {open ? (
        <div
          className={cn(
            'w-full max-w-[280px] overflow-y-auto rounded-lg border border-[var(--inf-line)] bg-[var(--inf-surface-soft)] px-3 py-2 text-left text-[11px] leading-4 break-words whitespace-pre-wrap text-[var(--inf-txt-3)] select-text',
            'max-h-24 overscroll-contain',
            props.rawClassName
          )}
        >
          {props.view.raw}
        </div>
      ) : null}
    </div>
  )
}
