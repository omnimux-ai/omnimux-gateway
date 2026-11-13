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

import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'

import { getAdminAccount, HOSTED_ADMIN_KEYS } from '../api'
import { ACCOUNT_STATUS_LABEL, TASK_STATUS_LABEL, TASK_TYPE_LABEL } from '../constants'
import { HostedEmptyState } from './empty-state'
import { HostedStatusBadge } from './status-badge'

type HostedAccountDetailSheetProps = {
  accountId: number | null
  onClose: () => void
}

export function HostedAccountDetailSheet(props: HostedAccountDetailSheetProps) {
  const { t } = useTranslation()
  const open = props.accountId != null
  const detailQuery = useQuery({
    queryKey: HOSTED_ADMIN_KEYS.account(props.accountId ?? 0),
    queryFn: () => getAdminAccount(props.accountId as number),
    enabled: open,
  })
  const row = detailQuery.data

  return (
    <Sheet
      open={open}
      onOpenChange={(next) => {
        if (!next) {
          props.onClose()
        }
      }}
    >
      <SheetContent className='sm:max-w-md'>
        <SheetHeader>
          <SheetTitle>{t('Channel detail')}</SheetTitle>
          <SheetDescription>
            {t('Live hosted channel. Credentials are never shown.')}
          </SheetDescription>
        </SheetHeader>
        <div className='space-y-4 overflow-y-auto px-4 pb-4'>
          {detailQuery.isLoading ? (
            <HostedEmptyState labelKey='Loading hosted data' />
          ) : null}
          {row ? (
            <>
              <dl className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-sm'>
                <dt className='text-muted-foreground'>{t('Phone ID')}</dt>
                <dd className='font-mono text-xs'>{row.phoneId}</dd>
                <dt className='text-muted-foreground'>{t('Remark')}</dt>
                <dd>{row.remark}</dd>
                <dt className='text-muted-foreground'>{t('Status')}</dt>
                <dd>
                  <HostedStatusBadge labelKey={ACCOUNT_STATUS_LABEL[row.status]} />
                </dd>
                <dt className='text-muted-foreground'>{t('Buyer')}</dt>
                <dd>{row.buyer ?? t('Unassigned')}</dd>
                <dt className='text-muted-foreground'>{t('Brand')}</dt>
                <dd>{row.brand ?? t('Unassigned')}</dd>
                <dt className='text-muted-foreground'>{t('Readiness')}</dt>
                <dd>{row.readiness}%</dd>
                <dt className='text-muted-foreground'>{t('Allocatable')}</dt>
                <dd>{row.allocatable ? t('Yes') : t('No')}</dd>
              </dl>
              <div>
                <h3 className='mb-2 text-sm font-medium'>{t('Recent tasks')}</h3>
                {row.tasks.length === 0 ? (
                  <HostedEmptyState labelKey='No hosted tasks yet' />
                ) : (
                  <ul className='space-y-2 text-sm'>
                    {row.tasks.map((task) => (
                      <li key={task.id} className='rounded-md border px-3 py-2'>
                        <p className='font-mono text-xs'>#{task.id}</p>
                        <p>
                          {t(TASK_TYPE_LABEL[task.type])} ·{' '}
                          {t(TASK_STATUS_LABEL[task.status])}
                        </p>
                        {task.note ? (
                          <p className='text-muted-foreground'>{task.note}</p>
                        ) : null}
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            </>
          ) : null}
        </div>
      </SheetContent>
    </Sheet>
  )
}
