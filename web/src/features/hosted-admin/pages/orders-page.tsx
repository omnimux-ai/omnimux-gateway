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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { getAdminOrders, HOSTED_ADMIN_KEYS } from '../api'
import { HostedDemoBanner } from '../components/demo-banner'
import { HostedEmptyState } from '../components/empty-state'
import { HostedStatGrid } from '../components/stat-grid'
import { HostedStatusBadge } from '../components/status-badge'
import { ORDER_KIND_LABEL, ORDER_STATUS_LABEL } from '../constants'
import {
  assignCountdown,
  openOrderCount,
  undeliveredSeats,
  type AssignCountdown,
} from '../lib/summarize'
import type { HostedOrderStatus } from '../types'

function orderTone(status: HostedOrderStatus) {
  if (status === 'cancelled' || status === 'refund_partial') {
    return 'destructive'
  }
  if (status === 'pending_fulfillment' || status === 'paid') {
    return 'warning'
  }
  return 'outline'
}

function formatCountdown(
  countdown: AssignCountdown,
  t: (key: string, options?: Record<string, unknown>) => string
): string {
  if (countdown.kind === 'delivered') {
    return t('Assigned')
  }
  if (countdown.kind === 'invalid') {
    return t('Unknown deadline')
  }
  const clock = t('{{hours}}h {{minutes}}m', {
    hours: countdown.hours,
    minutes: countdown.minutes,
  })
  if (countdown.kind === 'overdue') {
    return t('{{clock}} overdue', { clock })
  }
  return t('{{clock}} left', { clock })
}

export function HostedOrdersPage() {
  const { t } = useTranslation()
  const nowMs = Date.now()
  const ordersQuery = useQuery({
    queryKey: HOSTED_ADMIN_KEYS.orders,
    queryFn: getAdminOrders,
  })
  const rows = ordersQuery.data ?? []

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Hosted order fulfillment')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <HostedDemoBanner />
          <HostedStatGrid
            items={[
              {
                labelKey: 'Open orders',
                value: openOrderCount(rows),
              },
              {
                labelKey: 'Undelivered seats',
                value: undeliveredSeats(rows),
              },
              {
                labelKey: 'SLA window',
                value: t('{{hours}}h assign', { hours: 72 }),
              },
              {
                labelKey: 'Wallet billed',
                value: `$${rows.reduce((sum, row) => sum + row.amountUsd, 0)}`,
              },
            ]}
          />
          <div className='bg-card overflow-hidden rounded-lg border shadow-xs'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Order')}</TableHead>
                  <TableHead>{t('User')}</TableHead>
                  <TableHead>{t('SKU')}</TableHead>
                  <TableHead>{t('Kind')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Progress')}</TableHead>
                  <TableHead>{t('Amount')}</TableHead>
                  <TableHead>{t('Assign deadline')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((row) => {
                  const countdown = assignCountdown(row, nowMs)
                  return (
                    <TableRow key={row.id}>
                      <TableCell className='font-mono text-xs'>
                        #{row.id}
                      </TableCell>
                      <TableCell>{row.user}</TableCell>
                      <TableCell>{row.sku}</TableCell>
                      <TableCell>{t(ORDER_KIND_LABEL[row.kind])}</TableCell>
                      <TableCell>
                        <HostedStatusBadge
                          labelKey={ORDER_STATUS_LABEL[row.status]}
                          tone={orderTone(row.status)}
                        />
                      </TableCell>
                      <TableCell>
                        {row.delivered}/{row.quantity}
                      </TableCell>
                      <TableCell>${row.amountUsd}</TableCell>
                      <TableCell>
                        <div className='space-y-0.5'>
                          <p>{formatCountdown(countdown, t)}</p>
                          <p className='text-muted-foreground text-xs'>
                            {row.deadline}
                          </p>
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
            {ordersQuery.isLoading ? (
              <HostedEmptyState labelKey='Loading hosted data' />
            ) : null}
            {!ordersQuery.isLoading && rows.length === 0 ? (
              <HostedEmptyState labelKey='No hosted orders yet' />
            ) : null}
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
