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
import { useEffect, useId, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { describeFailReason } from '../lib/fail-reason'
import {
  taskAbsoluteImageUrl,
  taskImageUrl,
  taskBrowserMediaUrl,
} from '../lib/history'
import { multiViewProgress, multiViewState } from '../lib/multiview'
import { hasUnverifiedResult } from '../lib/task-status'
import type { TaskRecord } from '../types'
import { FailReasonDetail } from './fail-reason-detail'
import { FullscreenPreviewDialog } from './fullscreen-preview-dialog'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  task: TaskRecord | null
  regenerating: boolean
  disabledReason?: string
  onRegenerate: () => void
  onDelete: () => void
}

export function MultiViewDialog(props: Props) {
  const { t } = useTranslation()
  const disabledReasonId = useId()
  const [imageFailed, setImageFailed] = useState(false)
  const state = multiViewState(props.task)
  const failure = describeFailReason(t, props.task?.fail_reason)
  const active = state !== 'ready' && state !== 'failed' && state !== 'absent'
  let activityLabel = t('Generating…')
  if (state === 'queued') {
    activityLabel =
      (props.task?.status ?? '').toUpperCase() === 'WAITING'
        ? t('Waiting for a free slot')
        : t('Queued')
  }
  if (state === 'submitting') activityLabel = t('Submitting…')
  if (state === 'review') activityLabel = t('Needs review')
  const preview = props.task ? taskImageUrl(props.task) : null
  const download = props.task ? taskAbsoluteImageUrl(props.task) : null
  useEffect(
    () => setImageFailed(false),
    [props.task?.task_id, props.task?.id, preview, props.open]
  )
  const panelClass =
    'flex size-full flex-col items-center justify-center gap-3 rounded-[var(--radius-media)] bg-black/40 p-6 text-center'
  return (
    <FullscreenPreviewDialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Multi-view')}
      backgroundUrl={
        state === 'ready' && !imageFailed ? (preview ?? undefined) : undefined
      }
      actions={
        <>
          <Button
            type='button'
            variant='outline'
            disabled={
              active || props.regenerating || Boolean(props.disabledReason)
            }
            aria-describedby={
              props.disabledReason ? disabledReasonId : undefined
            }
            onClick={props.onRegenerate}
          >
            {t('Regenerate')}
          </Button>
          <Button
            type='button'
            variant='destructive'
            disabled={props.regenerating || active}
            onClick={props.onDelete}
          >
            {t('Delete')}
          </Button>
          {download ? (
            <Button
              variant='outline'
              render={
                <a
                  href={taskBrowserMediaUrl(download, true)}
                  download
                  target='_blank'
                  rel='noreferrer'
                />
              }
            >
              {t('Download')}
            </Button>
          ) : null}
        </>
      }
      media={
        <>
          {state === 'ready' && preview && !imageFailed ? (
            <img
              key={`${props.task?.task_id}-${preview}`}
              src={preview}
              alt={t('Multi-view')}
              onError={() => setImageFailed(true)}
              className='size-full object-contain'
            />
          ) : null}
          {state === 'ready' && (!preview || imageFailed) ? (
            <div role='status' className={panelClass}>
              <p className='text-xs text-white/60'>
                {t('No preview available')}
              </p>
            </div>
          ) : null}
          {active ? (
            <div role='status' className={panelClass}>
              {state === 'review' ? null : <div className='inf-shimmer' />}
              <div className='inf-status q'>{activityLabel}</div>
              {state === 'review' ? (
                <p className='text-xs text-white/60'>
                  {hasUnverifiedResult(props.task)
                    ? t('Checking the original result. Do not submit again.')
                    : t(
                        'Confirming the result. Unconfirmed tasks fail automatically within about 60 minutes and are never resent.'
                      )}
                </p>
              ) : (
                <div className='inf-progress'>
                  <i style={{ width: `${multiViewProgress(props.task)}%` }} />
                </div>
              )}
            </div>
          ) : null}
          {state === 'failed' ? (
            <div
              role='status'
              className={cn(panelClass, 'flex flex-col items-center gap-2')}
            >
              <p className='text-xs text-[var(--inf-danger)]'>
                {failure.message}
              </p>
              <FailReasonDetail view={failure} />
            </div>
          ) : null}
          {state === 'absent' ? (
            <div role='status' className={panelClass}>
              <p className='text-xs text-white/60'>{t('Nothing here yet')}</p>
            </div>
          ) : null}
        </>
      }
    >
      {props.disabledReason ? (
        <span id={disabledReasonId} className='sr-only'>
          {props.disabledReason}
        </span>
      ) : null}
    </FullscreenPreviewDialog>
  )
}
