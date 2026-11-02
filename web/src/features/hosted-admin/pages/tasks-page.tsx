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
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
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
  cancelAdminTask,
  getAdminTasks,
  HOSTED_ADMIN_KEYS,
  retryAdminTask,
} from '../api'
import { HostedDemoBanner } from '../components/demo-banner'
import { HostedEmptyState } from '../components/empty-state'
import { HostedStatGrid } from '../components/stat-grid'
import { HostedStatusBadge } from '../components/status-badge'
import { TASK_STATUS_LABEL, TASK_TYPE_LABEL } from '../constants'
import { countByStatus } from '../lib/summarize'
import type { HostedTaskStatus } from '../types'

function taskTone(status: keyof typeof TASK_STATUS_LABEL) {
  if (status === 'failed' || status === 'cancelled') {
    return 'destructive'
  }
  if (status === 'running' || status === 'queued' || status === 'pending') {
    return 'warning'
  }
  return 'outline'
}

function canCancel(status: HostedTaskStatus) {
  return (
    status === 'pending' || status === 'queued' || status === 'running'
  )
}

export function HostedTasksPage() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const tasksQuery = useQuery({
    queryKey: HOSTED_ADMIN_KEYS.tasks,
    queryFn: getAdminTasks,
  })
  const rows = tasksQuery.data ?? []
  const counts = countByStatus(rows)
  const cancelMutation = useMutation({
    mutationFn: cancelAdminTask,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: HOSTED_ADMIN_KEYS.tasks })
      toast.success(t('Task cancelled'))
    },
    onError: (error) => {
      handleServerError(error)
    },
  })
  const retryMutation = useMutation({
    mutationFn: retryAdminTask,
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: HOSTED_ADMIN_KEYS.tasks })
      toast.success(t('Retry queued'))
    },
    onError: (error) => {
      handleServerError(error)
    },
  })
  const busy = cancelMutation.isPending || retryMutation.isPending

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Hosted task log')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='space-y-4'>
          <HostedDemoBanner />
          <HostedStatGrid
            items={[
              { labelKey: 'Running', value: counts.running ?? 0 },
              { labelKey: 'Queued', value: counts.queued ?? 0 },
              { labelKey: 'Succeeded', value: counts.succeeded ?? 0 },
              { labelKey: 'Failed', value: counts.failed ?? 0 },
            ]}
          />
          <div className='bg-card overflow-hidden rounded-lg border shadow-xs'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Task')}</TableHead>
                  <TableHead>{t('Channel')}</TableHead>
                  <TableHead>{t('Type')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Receipt')}</TableHead>
                  <TableHead>{t('Note')}</TableHead>
                  <TableHead>{t('Actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {rows.map((row) => {
                  const receipt = row.receipt
                  return (
                    <TableRow key={row.id}>
                      <TableCell className='font-mono text-xs'>
                        #{row.id}
                      </TableCell>
                      <TableCell>#{row.channelId}</TableCell>
                      <TableCell>{t(TASK_TYPE_LABEL[row.type])}</TableCell>
                      <TableCell>
                        <HostedStatusBadge
                          labelKey={TASK_STATUS_LABEL[row.status]}
                          tone={taskTone(row.status)}
                        />
                      </TableCell>
                      <TableCell>
                        {receipt
                          ? t(
                              '{{views}} views · {{likes}} likes · {{comments}} comments',
                              {
                                views: receipt.views,
                                likes: receipt.likes,
                                comments: receipt.comments,
                              }
                            )
                          : t('No receipt yet')}
                      </TableCell>
                      <TableCell>{row.note}</TableCell>
                      <TableCell>
                        <div className='flex items-center gap-2'>
                          {canCancel(row.status) ? (
                            <Button
                              type='button'
                              variant='ghost'
                              size='sm'
                              disabled={busy}
                              onClick={() => cancelMutation.mutate(row.id)}
                            >
                              {t('Cancel')}
                            </Button>
                          ) : null}
                          {row.status === 'failed' ? (
                            <Button
                              type='button'
                              variant='ghost'
                              size='sm'
                              disabled={busy}
                              onClick={() => retryMutation.mutate(row.id)}
                            >
                              {t('Retry')}
                            </Button>
                          ) : null}
                        </div>
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
            {tasksQuery.isLoading ? (
              <HostedEmptyState labelKey='Loading hosted data' />
            ) : null}
            {!tasksQuery.isLoading && rows.length === 0 ? (
              <HostedEmptyState labelKey='No hosted tasks yet' />
            ) : null}
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
