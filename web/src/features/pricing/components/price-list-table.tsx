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
import type { PaginationState, Row } from '@tanstack/react-table'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTablePagination,
  DataTableRow,
  DataTableView,
  useDataTable,
} from '@/components/data-table'

import { DEFAULT_PRICING_PAGE_SIZE } from '../constants'
import type { PriceListRow } from '../lib/price-list-rows'
import { usePriceListColumns } from './price-list-columns'

export interface PriceListTableProps {
  rows: PriceListRow[]
  isLoading?: boolean
  onModelClick?: (modelName: string) => void
}

export function PriceListTable(props: PriceListTableProps) {
  const { t } = useTranslation()
  const { rows, isLoading = false, onModelClick } = props

  const [pagination, setPagination] = useState<PaginationState>({
    pageIndex: 0,
    pageSize: DEFAULT_PRICING_PAGE_SIZE,
  })

  const columns = usePriceListColumns()

  const { table } = useDataTable({
    data: rows,
    columns,
    pageCount: Math.max(1, Math.ceil(rows.length / pagination.pageSize)),
    pagination,
    onPaginationChange: setPagination,
    manualPagination: false,
    withFilteredRowModel: false,
    withSortedRowModel: false,
    withFacetedRowModel: false,
    getRowId: (row) => row.rowKey,
  })

  const handleRowClick = useCallback(
    (row: PriceListRow) => {
      onModelClick?.(row.modelName)
    },
    [onModelClick]
  )

  return (
    <div className='space-y-4'>
      <DataTableView
        table={table}
        isLoading={isLoading}
        emptyTitle={t('No Models Found')}
        emptyDescription={t('No models match your current filters.')}
        skeletonKeyPrefix='price-list-skeleton'
        applyHeaderSize
        getColumnClassName={(_columnId, kind) =>
          kind === 'header' ? 'text-muted-foreground font-medium' : undefined
        }
        renderRow={(row: Row<PriceListRow>) => (
          <DataTableRow
            key={row.id}
            row={row}
            className='hover:bg-muted/30 cursor-pointer transition-colors'
            onClick={() => handleRowClick(row.original)}
          />
        )}
      />

      {!isLoading && rows.length > 0 && <DataTablePagination table={table} />}
    </div>
  )
}
