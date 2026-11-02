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
import { AlertTriangle, Loader2, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'

export type PricingOfflineBannerProps = {
  /** When true (or network failed with cache), show the stale-cache warning. */
  isUsingCache: boolean
  /** Epoch ms of the last successful pricing sync. */
  lastUpdated?: number | null
  isRefreshing?: boolean
  onRefresh?: () => void
  className?: string
}

export function PricingOfflineBanner(props: PricingOfflineBannerProps) {
  const { t } = useTranslation()

  if (!props.isUsingCache) {
    return null
  }

  const syncedLabel =
    typeof props.lastUpdated === 'number' && props.lastUpdated > 0
      ? dayjs(props.lastUpdated).format('YYYY-MM-DD HH:mm:ss')
      : t('unknown time')

  return (
    <div
      role='status'
      aria-live='polite'
      className={cn(
        'flex flex-col gap-3 rounded-lg border px-3 py-2.5 text-sm sm:flex-row sm:items-center sm:justify-between',
        'border-amber-500/30 bg-amber-500/10 text-amber-600 dark:text-amber-400',
        props.className
      )}
    >
      <div className='flex items-start gap-2 sm:items-center'>
        <AlertTriangle
          className='mt-0.5 size-4 shrink-0 sm:mt-0'
          aria-hidden='true'
        />
        <p>
          {t(
            'Showing cached prices that may be outdated. Last synced: {{time}}.',
            { time: syncedLabel }
          )}
        </p>
      </div>

      {props.onRefresh && (
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={props.onRefresh}
          disabled={props.isRefreshing}
          className='border-amber-500/40 bg-background/60 text-amber-700 hover:bg-amber-500/10 dark:text-amber-300'
        >
          {props.isRefreshing ? (
            <Loader2 className='size-3.5 animate-spin' aria-hidden='true' />
          ) : (
            <RefreshCw className='size-3.5' aria-hidden='true' />
          )}
          <span>{t('Refresh now')}</span>
        </Button>
      )}
    </div>
  )
}
