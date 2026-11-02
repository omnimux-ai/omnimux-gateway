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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Switch } from '@/components/ui/switch'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { handleServerError } from '@/lib/handle-server-error'

import { getAdminSkus, HOSTED_ADMIN_KEYS, setSkuEnabled } from '../api'
import { HostedDemoBanner } from '../components/demo-banner'
import { HostedEmptyState } from '../components/empty-state'
import { HostedStatGrid } from '../components/stat-grid'
import { skuStockTotal } from '../lib/summarize'

export function HostedSkusPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const skusQuery = useQuery({
    queryKey: HOSTED_ADMIN_KEYS.skus,
    queryFn: getAdminSkus,
  })
  const rows = skusQuery.data ?? []
  const toggleMutation = useMutation({
    mutationFn: (input: { id: number; enabled: boolean }) =>
      setSkuEnabled(input.id, input.enabled),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: HOSTED_ADMIN_KEYS.skus })
    },
    onError: (error) => {
      handleServerError(error)
    },
  })

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Hosted SKUs')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <HostedDemoBanner />
          <HostedStatGrid
            items={[
              { labelKey: 'Listed SKUs', value: rows.length },
              {
                labelKey: 'Live stock',
                value: skuStockTotal(rows),
              },
              { labelKey: 'Price', value: '$99' },
              { labelKey: 'Assign SLA', value: t('{{hours}}h', { hours: 72 }) },
            ]}
          />
          <div className='bg-card overflow-hidden rounded-lg border shadow-xs'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('SKU')}</TableHead>
                  <TableHead>{t('Platform')}</TableHead>
                  <TableHead>{t('Region')}</TableHead>
                  <TableHead>{t('Price')}</TableHead>
                  <TableHead>{t('Enabled')}</TableHead>
                  <TableHead>{t('Live stock')}</TableHead>
                  <TableHead>{t('Open orders')}</TableHead>
                  <TableHead>{t('Assign SLA')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell className='font-mono text-xs'>#{row.id}</TableCell>
                    <TableCell>{row.platform}</TableCell>
                    <TableCell>{row.region}</TableCell>
                    <TableCell>${row.priceUsd}</TableCell>
                    <TableCell>
                      <div className='flex items-center gap-2'>
                        <Switch
                          aria-label={t('Toggle SKU {{id}}', { id: row.id })}
                          checked={row.enabled}
                          disabled={toggleMutation.isPending}
                          onCheckedChange={(next) =>
                            toggleMutation.mutate({
                              id: row.id,
                              enabled: next,
                            })
                          }
                        />
                        <span className='text-muted-foreground text-xs'>
                          {row.enabled ? t('Enabled') : t('Disabled')}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>{row.stock}</TableCell>
                    <TableCell>{row.openOrders}</TableCell>
                    <TableCell>
                      {t('{{hours}}h', { hours: row.estDeliveryHours })}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {skusQuery.isLoading ? (
              <HostedEmptyState labelKey='Loading hosted data' />
            ) : null}
            {!skusQuery.isLoading && rows.length === 0 ? (
              <HostedEmptyState labelKey='No hosted SKUs yet' />
            ) : null}
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
