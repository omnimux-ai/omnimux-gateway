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
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { DataTableColumnHeader } from '@/components/data-table'
import { getLobeIcon } from '@/lib/lobe-icon'

import type { PriceListRow } from '../lib/price-list-rows'

export function usePriceListColumns(): ColumnDef<PriceListRow>[] {
  const { t } = useTranslation()

  return [
    {
      accessorKey: 'modelName',
      meta: { label: t('Model') },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Model')} />
      ),
      cell: ({ row }) => {
        const item = row.original
        if (!item.isFirstForModel) {
          return <span className='sr-only'>{item.modelName}</span>
        }
        const iconKey = item.vendorIcon
        const icon = iconKey ? getLobeIcon(iconKey, 14) : null
        return (
          <div className='flex max-w-full min-w-0 items-center gap-2'>
            {icon}
            <div className='min-w-0'>
              <div className='truncate font-mono text-sm font-medium'>
                {item.modelName}
              </div>
              {item.vendorName ? (
                <div className='text-muted-foreground truncate text-[11px]'>
                  {item.vendorName}
                </div>
              ) : null}
            </div>
          </div>
        )
      },
      minSize: 200,
    },
    {
      accessorKey: 'spec',
      meta: { label: t('Spec') },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Spec')} />
      ),
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm'>
          {t(row.original.specLabelKey)}
        </span>
      ),
      size: 120,
      enableSorting: false,
    },
    {
      accessorKey: 'ourPriceLabel',
      meta: { label: t('Our Price') },
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('Our Price')} />
      ),
      cell: ({ row }) => {
        const item = row.original
        return (
          <div className='min-w-0 text-right'>
            <div className='font-mono text-sm font-medium tabular-nums'>
              {item.ourPriceLabel}
            </div>
            <div className='text-muted-foreground/60 text-[10px]'>
              {item.unitHintKey.startsWith('/')
                ? item.unitHintKey
                : t(item.unitHintKey)}
            </div>
          </div>
        )
      },
      size: 160,
      enableSorting: false,
    },
  ]
}
