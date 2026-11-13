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

import { cn } from '@/lib/utils'

import {
  creditsToUsd,
  formatCredits,
  formatUsdValue,
  usdToCredits,
} from '../lib/credits'
import { resolveTierSavingBadge } from '../lib/saving-display'
import type { PublicPriceTier } from '../types'
import { SavingBadge } from './saving-badge'

export type TierPriceTableProps = {
  tiers: PublicPriceTier[]
  className?: string
  compact?: boolean
}

function normalizeUnitSuffix(unitSuffix?: string): string {
  if (!unitSuffix) return ''
  const trimmed = unitSuffix.trim()
  if (!trimmed) return ''
  if (trimmed.startsWith('/')) return trimmed
  return `/${trimmed}`
}

function resolveOfficialCredits(tier: PublicPriceTier): number | undefined {
  if (
    typeof tier.officialCredits === 'number' &&
    Number.isFinite(tier.officialCredits) &&
    tier.officialCredits > 0
  ) {
    return tier.officialCredits
  }
  if (
    typeof tier.officialUsd === 'number' &&
    Number.isFinite(tier.officialUsd) &&
    tier.officialUsd > 0
  ) {
    return usdToCredits(tier.officialUsd)
  }
  return undefined
}

function hasOfficialPrice(tier: PublicPriceTier): boolean {
  const officialCredits = resolveOfficialCredits(tier)
  return (
    officialCredits !== undefined &&
    typeof tier.officialUsd === 'number' &&
    Number.isFinite(tier.officialUsd) &&
    tier.officialUsd > 0
  )
}

export function TierPriceTable(props: TierPriceTableProps) {
  const { t } = useTranslation()
  const pyClass = props.compact ? 'py-2' : 'py-3.5'
  const pxClass = props.compact ? 'px-3 sm:px-4' : 'px-4 sm:px-5'

  return (
    <div className={cn('w-full overflow-x-auto', props.className)}>
      <table
        className='bg-card w-full min-w-[500px] table-fixed border-collapse sm:min-w-full'
        aria-label={t('Price tiers')}
      >
        <thead>
          <tr className='bg-muted/30 text-muted-foreground border-border/40 border-b text-[11px] font-mono uppercase'>
            <th
              scope='col'
              className={cn(
                'w-[36%] text-left font-normal sm:w-[35%]',
                pxClass,
                'py-3'
              )}
            >
              {t('SPEC')}
            </th>
            <th
              scope='col'
              className={cn(
                'w-[26%] text-right font-normal sm:w-[25%]',
                pxClass,
                'py-3'
              )}
            >
              {t('OUR PRICE')}
            </th>
            <th
              scope='col'
              className={cn(
                'hidden w-[24%] text-right font-normal sm:table-cell',
                pxClass,
                'py-3'
              )}
            >
              {t('OFFICIAL PRICE')}
            </th>
            <th
              scope='col'
              className={cn(
                'w-[14%] text-right font-normal sm:w-[16%]',
                pxClass,
                'py-3'
              )}
            >
              {t('SAVING')}
            </th>
          </tr>
        </thead>
        <tbody className='divide-border/40 divide-y text-xs'>
          {props.tiers.map((tier) => {
            const unitSuffix = normalizeUnitSuffix(tier.unitSuffix)
            const ourUsd = creditsToUsd(tier.credits)
            const officialCredits = resolveOfficialCredits(tier)
            const showOfficial = hasOfficialPrice(tier)
            const savingBadge = resolveTierSavingBadge(tier)
            const rowKey = [
              tier.spec,
              tier.credits,
              tier.unitSuffix ?? '',
              tier.note ?? '',
              tier.officialUsd ?? '',
              tier.officialCredits ?? '',
              tier.savingPercent ?? '',
            ].join('|')

            return (
              <tr
                key={rowKey}
                className='hover:bg-muted/20 transition-colors'
              >
                <td
                  className={cn(
                    'text-foreground align-middle font-normal',
                    pyClass,
                    pxClass
                  )}
                >
                  <div className='flex flex-col gap-0.5'>
                    <span className='text-foreground text-sm font-normal'>
                      {tier.spec}
                    </span>
                    {tier.note ? (
                      <span className='text-muted-foreground font-mono text-[10px]'>
                        {tier.note}
                      </span>
                    ) : null}
                  </div>
                </td>

                <td className={cn('align-middle text-right', pyClass, pxClass)}>
                  <div className='inline-flex flex-col items-end'>
                    <span className='text-foreground text-sm font-bold tabular-nums whitespace-nowrap'>
                      {formatCredits(tier.credits)}
                      <span className='text-muted-foreground ml-1 text-xs font-normal'>
                        {t('Credits')}
                        {unitSuffix}
                      </span>
                    </span>
                    <span className='text-muted-foreground font-mono text-xs whitespace-nowrap'>
                      ~${formatUsdValue(ourUsd)}
                      {unitSuffix}
                    </span>
                  </div>
                </td>

                <td
                  className={cn(
                    'hidden align-middle text-right sm:table-cell',
                    pyClass,
                    pxClass
                  )}
                >
                  {showOfficial &&
                  officialCredits !== undefined &&
                  typeof tier.officialUsd === 'number' ? (
                    <div
                      className={cn(
                        'text-muted-foreground inline-flex flex-col items-end',
                        // Only strike through when there is a real discount.
                        // Official-same (`same`) and missing (`none`) must not.
                        savingBadge.kind === 'off' &&
                          'line-through decoration-muted-foreground/70'
                      )}
                    >
                      <span className='text-sm font-bold tabular-nums whitespace-nowrap text-inherit'>
                        {formatCredits(officialCredits)}
                        <span className='text-muted-foreground ml-1 text-xs font-normal'>
                          {t('Credits')}
                          {unitSuffix}
                        </span>
                      </span>
                      <span className='text-muted-foreground font-mono text-xs whitespace-nowrap'>
                        ~${formatUsdValue(tier.officialUsd)}
                        {unitSuffix}
                      </span>
                    </div>
                  ) : (
                    <span className='text-muted-foreground/60 font-mono text-xs'>
                      —
                    </span>
                  )}
                </td>

                <td className={cn('align-middle text-right', pyClass, pxClass)}>
                  <div className='flex justify-end'>
                    <SavingBadge
                      badge={savingBadge}
                      variant={
                        savingBadge.kind === 'off' ? 'inline' : 'capsule'
                      }
                      showArrow={savingBadge.kind === 'off'}
                    />
                  </div>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
