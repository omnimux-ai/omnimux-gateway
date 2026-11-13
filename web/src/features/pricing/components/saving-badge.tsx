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
import { ArrowDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import type { SavingBadgeModel } from '../lib/saving-display'

export type SavingBadgeProps = {
  badge: SavingBadgeModel
  className?: string
  /** Compact inline variant for table cells; default is capsule Tag. */
  variant?: 'capsule' | 'inline'
  /** Show down-arrow on positive OFF badges (default true for inline). */
  showArrow?: boolean
}

/**
 * Highlighted saving / official-same badge for pricing cards & tier rows.
 * Supports light + dark contrast via emerald / muted tokens.
 */
export function SavingBadge(props: SavingBadgeProps) {
  const { t } = useTranslation()
  const variant = props.variant ?? 'capsule'
  const showArrow =
    props.showArrow ?? (variant === 'inline' && props.badge.kind === 'off')

  if (props.badge.kind === 'none') {
    return (
      <span
        className={cn(
          'text-muted-foreground/60 font-mono text-xs',
          props.className
        )}
      >
        —
      </span>
    )
  }

  if (props.badge.kind === 'same') {
    if (variant === 'inline') {
      return (
        <span
          className={cn(
            'text-muted-foreground inline-flex items-center text-[11px] font-medium whitespace-nowrap sm:text-xs',
            props.className
          )}
          title={t('Official 1:1')}
        >
          {t('Official 1:1')}
        </span>
      )
    }

    return (
      <span
        className={cn(
          'border-border bg-muted/60 text-muted-foreground inline-flex items-center rounded-full border px-2 py-0.5 font-mono text-[10px] font-medium tracking-wide uppercase select-none sm:text-[11px]',
          'dark:border-border/60 dark:bg-muted/40 dark:text-muted-foreground',
          props.className
        )}
        title={t('Official 1:1')}
      >
        {t('Official 1:1')}
      </span>
    )
  }

  const percent = props.badge.percent ?? 0

  if (variant === 'inline') {
    return (
      <span
        className={cn(
          'inline-flex items-center gap-0.5 text-xs font-medium text-emerald-600 sm:text-sm dark:text-emerald-400',
          props.className
        )}
        title={t('Save {{percent}}%', { percent })}
      >
        <span>{percent}%</span>
        {showArrow ? (
          <ArrowDown className='size-3' aria-hidden='true' />
        ) : null}
      </span>
    )
  }

  return (
    <span
      className={cn(
        'inline-flex items-center gap-0.5 rounded-full border border-emerald-500/30 bg-emerald-500/10 px-2 py-0.5 font-mono text-[10px] font-semibold tracking-wide text-emerald-700 uppercase select-none sm:text-[11px]',
        'dark:border-emerald-400/30 dark:bg-emerald-400/10 dark:text-emerald-300',
        props.className
      )}
      title={t('Save {{percent}}%', { percent })}
    >
      <span>{t('Save {{percent}}%', { percent })}</span>
      {showArrow ? (
        <ArrowDown className='size-3' aria-hidden='true' />
      ) : null}
    </span>
  )
}
