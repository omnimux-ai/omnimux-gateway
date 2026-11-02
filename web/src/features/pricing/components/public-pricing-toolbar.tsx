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
import { Loader2, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  NativeSelect,
  NativeSelectOption,
} from '@/components/ui/native-select'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'

import { FILTER_ALL, type ModalityFilter } from '../constants'
import type { PricingVendor } from '../types'
import { ModalityTabs } from './modality-tabs'
import { SearchBar } from './search-bar'

export type PublicPricingToolbarProps = {
  modality: ModalityFilter
  onModalityChange: (value: ModalityFilter) => void
  modalityCounts?: Partial<Record<ModalityFilter, number>>
  search: string
  onSearchChange: (value: string) => void
  onSearchClear: () => void
  vendorId: string
  onVendorChange: (value: string) => void
  vendors: PricingVendor[]
  filteredCount: number
  lastUpdated?: number | null
  isRefreshing?: boolean
  onRefresh?: () => void
  className?: string
}

export function PublicPricingToolbar(props: PublicPricingToolbarProps) {
  const { t } = useTranslation()

  const syncedLabel =
    typeof props.lastUpdated === 'number' && props.lastUpdated > 0
      ? dayjs(props.lastUpdated).format('YYYY-MM-DD HH:mm:ss')
      : t('Never')

  return (
    <div
      className={cn(
        'bg-card border-border flex flex-col gap-3 rounded-xl border p-3',
        props.className
      )}
    >
      <ModalityTabs
        variant='public'
        value={props.modality}
        onChange={props.onModalityChange}
        counts={props.modalityCounts}
      />

      <div className='flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between'>
        <div className='flex min-w-0 flex-1 flex-col gap-2 sm:flex-row sm:items-center'>
          <SearchBar
            value={props.search}
            onChange={props.onSearchChange}
            onClear={props.onSearchClear}
            placeholder={t('Search models, IDs, or providers…')}
            className='w-full sm:max-w-md'
          />

          <NativeSelect
            aria-label={t('All providers')}
            value={props.vendorId}
            onChange={(event) => props.onVendorChange(event.target.value)}
            className='w-full sm:w-44'
          >
            <NativeSelectOption value={FILTER_ALL}>
              {t('All providers')}
            </NativeSelectOption>
            {props.vendors.map((vendor) => (
              <NativeSelectOption key={vendor.id} value={String(vendor.id)}>
                {vendor.name}
              </NativeSelectOption>
            ))}
          </NativeSelect>
        </div>

        <div className='text-muted-foreground flex flex-wrap items-center gap-3 text-sm'>
          <div className='flex items-baseline gap-1'>
            <span className='text-foreground font-semibold tabular-nums'>
              {props.filteredCount.toLocaleString()}
            </span>
            <span>
              {props.filteredCount === 1 ? t('model') : t('models')}
            </span>
          </div>

          <span className='text-muted-foreground/70 hidden text-xs sm:inline'>
            {t('Synced {{time}}', { time: syncedLabel })}
          </span>

          {props.onRefresh && (
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={props.onRefresh}
              disabled={props.isRefreshing}
              aria-label={t('Refresh prices')}
              className='gap-1.5'
            >
              {props.isRefreshing ? (
                <Loader2 className='size-3.5 animate-spin' aria-hidden='true' />
              ) : (
                <RefreshCw className='size-3.5' aria-hidden='true' />
              )}
              <span className='hidden sm:inline'>{t('Refresh')}</span>
            </Button>
          )}
        </div>
      </div>
    </div>
  )
}
