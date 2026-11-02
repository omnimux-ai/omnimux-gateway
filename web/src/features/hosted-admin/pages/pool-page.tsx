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
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
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

import {
  getAdminPool,
  HOSTED_ADMIN_KEYS,
  setAllocatable,
  syncAdminPool,
} from '../api'
import { HostedAccountDetailSheet } from '../components/account-detail-sheet'
import { HostedDemoBanner } from '../components/demo-banner'
import { HostedEmptyState } from '../components/empty-state'
import { HostedStatGrid } from '../components/stat-grid'
import { HostedStatusBadge } from '../components/status-badge'
import { ACCOUNT_STATUS_LABEL } from '../constants'
import {
  availableStock,
  countByStatus,
  filterPoolRows,
  type PoolStatusFilter,
} from '../lib/summarize'
import type { HostedAccountStatus } from '../types'

const STATUS_FILTERS: PoolStatusFilter[] = [
  'all',
  'available',
  'reserved',
  'assigned',
  'warming_up',
  'ready',
  'active',
  'quarantined',
]

function poolTone(status: HostedAccountStatus) {
  if (status === 'quarantined' || status === 'suspended') {
    return 'destructive'
  }
  if (
    status === 'warming_up' ||
    status === 'assigned' ||
    status === 'reserved'
  ) {
    return 'warning'
  }
  return 'outline'
}

export function HostedPoolPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [query, setQuery] = useState('')
  const [status, setStatus] = useState<PoolStatusFilter>('all')
  const [allocatableOnly, setAllocatableOnly] = useState(false)
  const [detailId, setDetailId] = useState<number | null>(null)
  const poolQuery = useQuery({
    queryKey: HOSTED_ADMIN_KEYS.pool,
    queryFn: getAdminPool,
  })
  const rows = poolQuery.data
  const counts = countByStatus(rows ?? [])
  const visibleRows = useMemo(
    () =>
      filterPoolRows(rows ?? [], { query, status, allocatableOnly }),
    [allocatableOnly, query, rows, status]
  )
  const toggleMutation = useMutation({
    mutationFn: (input: { id: number; allocatable: boolean }) =>
      setAllocatable(input.id, input.allocatable),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: HOSTED_ADMIN_KEYS.pool })
    },
    onError: (error) => {
      handleServerError(error)
    },
  })
  const syncMutation = useMutation({
    mutationFn: syncAdminPool,
    onSuccess: async (result) => {
      await queryClient.invalidateQueries({ queryKey: HOSTED_ADMIN_KEYS.pool })
      toast.success(t('Synced {{count}} phones', { count: result.upserted }))
    },
    onError: (error) => {
      handleServerError(error)
    },
  })

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {t('Hosted channel pool')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <HostedDemoBanner />
          <HostedStatGrid
            items={[
              {
                labelKey: 'Pool size',
                value: rows?.length ?? 0,
              },
              {
                labelKey: 'Allocatable stock',
                value: availableStock(rows ?? []),
              },
              {
                labelKey: 'Warming up',
                value: counts.warming_up ?? 0,
              },
              {
                labelKey: 'Do not allocate',
                value: (rows ?? []).filter((row) => !row.allocatable).length,
              },
            ]}
          />
          <div className='flex flex-col gap-3 sm:flex-row sm:items-end'>
            <div className='min-w-0 flex-1 space-y-1'>
              <Label htmlFor='hosted-pool-query'>{t('Search pool')}</Label>
              <Input
                id='hosted-pool-query'
                value={query}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={t('Remark, phone ID, or buyer')}
              />
            </div>
            <div className='space-y-1'>
              <Label htmlFor='hosted-pool-status'>{t('Status')}</Label>
              <select
                id='hosted-pool-status'
                className='border-input bg-background h-8 rounded-lg border px-2.5 text-sm'
                value={status}
                onChange={(event) =>
                  setStatus(event.target.value as PoolStatusFilter)
                }
              >
                {STATUS_FILTERS.map((value) => (
                  <option key={value} value={value}>
                    {value === 'all'
                      ? t('All statuses')
                      : t(ACCOUNT_STATUS_LABEL[value])}
                  </option>
                ))}
              </select>
            </div>
            <div className='flex h-8 items-center gap-2'>
              <Switch
                id='hosted-pool-allocatable-only'
                checked={allocatableOnly}
                onCheckedChange={setAllocatableOnly}
              />
              <Label htmlFor='hosted-pool-allocatable-only'>
                {t('Allocatable only')}
              </Label>
            </div>
            <Button
              type='button'
              variant='outline'
              disabled={syncMutation.isPending}
              onClick={() => syncMutation.mutate()}
            >
              {syncMutation.isPending ? t('Syncing') : t('Sync now')}
            </Button>
          </div>
          <div className='bg-card overflow-hidden rounded-lg border shadow-xs'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Remark')}</TableHead>
                  <TableHead>{t('Phone ID')}</TableHead>
                  <TableHead>{t('Group')}</TableHead>
                  <TableHead>{t('Country')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Readiness')}</TableHead>
                  <TableHead>{t('Buyer')}</TableHead>
                  <TableHead>{t('Allocatable')}</TableHead>
                  <TableHead>{t('Detail')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {visibleRows.map((row) => (
                  <TableRow key={row.id}>
                    <TableCell>{row.remark}</TableCell>
                    <TableCell className='font-mono text-xs'>
                      {row.phoneId}
                    </TableCell>
                    <TableCell>{row.group}</TableCell>
                    <TableCell>{row.countryName}</TableCell>
                    <TableCell>
                      <HostedStatusBadge
                        labelKey={ACCOUNT_STATUS_LABEL[row.status]}
                        tone={poolTone(row.status)}
                      />
                    </TableCell>
                    <TableCell>{row.readiness}%</TableCell>
                    <TableCell>{row.buyer ?? t('Unassigned')}</TableCell>
                    <TableCell>
                      <div className='flex items-center gap-2'>
                        <Switch
                          aria-label={t('Toggle allocatable for {{phone}}', {
                            phone: row.phoneId,
                          })}
                          checked={row.allocatable}
                          disabled={toggleMutation.isPending}
                          onCheckedChange={(next) =>
                            toggleMutation.mutate({
                              id: row.id,
                              allocatable: next,
                            })
                          }
                        />
                        <span className='text-muted-foreground text-xs'>
                          {row.allocatable ? t('Yes') : t('No')}
                        </span>
                      </div>
                    </TableCell>
                    <TableCell>
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        onClick={() => setDetailId(row.id)}
                      >
                        {t('Open')}
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
            {poolQuery.isLoading ? (
              <HostedEmptyState labelKey='Loading hosted data' />
            ) : null}
            {!poolQuery.isLoading && visibleRows.length === 0 ? (
              <HostedEmptyState labelKey='No hosted channels yet' />
            ) : null}
          </div>
          <HostedAccountDetailSheet
            accountId={detailId}
            onClose={() => setDetailId(null)}
          />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
