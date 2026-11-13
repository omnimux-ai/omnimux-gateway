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
import { useTranslation } from 'react-i18next'

import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import type { TopupRung } from '../constants'
import { SavingBadge } from './saving-badge'

export type TopupLadderProps = {
  /** Preset top-up rungs, cheapest first. */
  ladder: readonly TopupRung[]
  className?: string
}

/**
 * Read-only volume ladder for the public pricing page.
 *
 * Reads no network state: the rungs come from `TOPUP_LADDER`, which a
 * regression test keeps in sync with `scripts/ops/payment/amount-ladder.json`.
 * Shows credits and the USD usage value only — the payable amount depends on
 * the visitor's currency and payment provider, so it belongs to the wallet page.
 */
export function TopupLadder(props: TopupLadderProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  if (props.ladder.length === 0) {
    return null
  }

  return (
    <section
      className={cn('mb-12 sm:mb-16', props.className)}
      aria-labelledby='topup-ladder-heading'
    >
      <div className='border-border/60 mb-6 flex flex-col gap-2 border-b pb-5'>
        <h2
          id='topup-ladder-heading'
          className='text-foreground text-2xl font-bold tracking-tight sm:text-3xl'
        >
          {t('Top up more, pay less')}
        </h2>
        <p className='text-muted-foreground text-xs sm:text-sm'>
          {t('Discounts apply to preset amounts only')}
        </p>
      </div>

      <ul className='grid grid-cols-2 gap-2 sm:grid-cols-3 sm:gap-3 lg:grid-cols-6'>
        {props.ladder.map((rung) => {
          const offPercent = Math.round((1 - rung.discount) * 100)
          return (
            <li
              key={rung.amountUsd}
              className='bg-card border-border flex flex-col gap-1 rounded-sm border p-4'
            >
              <span className='text-foreground font-mono text-lg font-semibold tracking-tight tabular-nums'>
                {formatNumber(rung.credits, locale)}
              </span>
              <span className='text-muted-foreground text-xs'>
                {t('Credits')}
              </span>
              {offPercent > 0 ? (
                <SavingBadge
                  badge={{ kind: 'off', percent: offPercent }}
                  className='mt-0.5 self-start'
                />
              ) : null}
              <span className='text-muted-foreground mt-1 font-mono text-xs tabular-nums'>
                {`$${formatNumber(rung.amountUsd, locale)}`}
              </span>
            </li>
          )
        })}
      </ul>
    </section>
  )
}
