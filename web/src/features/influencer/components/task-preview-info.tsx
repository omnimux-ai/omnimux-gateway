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
import { Play } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatTimestampToDate } from '@/lib/format'

import { getTaskTimestamp, taskBrowserMediaUrl, taskKind } from '../lib/history'
import type { PreviewMediaKind } from '../lib/preview-reference'
import { taskPreviewInfo } from '../lib/task-preview-info'
import type { TaskRecord } from '../types'
import { FadeInImage } from './fade-in-image'

const referenceCardClassName =
  'relative aspect-square size-20 overflow-hidden rounded-lg bg-[var(--inf-surface-soft)] p-0 hover:bg-[var(--inf-surface-soft)]'

function statusLabel(status: string): string {
  switch (status.toLowerCase()) {
    case 'success':
    case 'succeeded':
      return 'Success'
    case 'failed':
    case 'failure':
      return 'Failed'
    case 'not_start':
    case 'pending':
    case 'submitted':
      return 'Pending'
    case 'queued':
      return 'Queued'
    case 'running':
    case 'processing':
    case 'in_progress':
      return 'Running'
    case 'cancelled':
    case 'canceled':
      return 'Cancelled'
    default:
      return 'Unknown'
  }
}

export function TaskPreviewInfo(props: {
  task: TaskRecord
  /** Shows the stored reference inside the same preview window. */
  onOpenReference?: (url: string, media: PreviewMediaKind) => void
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const info = taskPreviewInfo(props.task)
  const kind = taskKind(props.task)
  let type = t('Character')
  if (kind === 'motion') type = t('Motion video')
  if (kind === 'multiview') type = t('Multi-view')
  const timestamp = getTaskTimestamp(props.task)
  const rows = [
    { label: t('Type'), value: type },
    { label: t('Status'), value: t(statusLabel(props.task.status)) },
  ]
  if (info.resolution) {
    rows.push({ label: t('Resolution'), value: info.resolution })
  }
  if (info.size) rows.push({ label: t('Size'), value: info.size })
  if (info.durationSec) {
    rows.push({
      label: t('Duration (seconds)'),
      value: formatNumber(info.durationSec, locale),
    })
  }
  if (timestamp) {
    rows.push({
      label: t('Created At'),
      value: formatTimestampToDate(timestamp, 'milliseconds'),
    })
  }
  return (
    <section
      aria-label={t('Info')}
      className='flex shrink-0 flex-col gap-4 px-3 py-3 text-sm text-[var(--inf-txt)]'
    >
      <h2 className='font-semibold'>{t('Info')}</h2>
      <dl className='flex flex-col gap-3'>
        {rows.map((row) => (
          <div key={row.label} className='grid grid-cols-2 gap-3'>
            <dt className='text-[var(--inf-txt-2)]'>{row.label}</dt>
            <dd className='min-w-0 text-right break-words'>{row.value}</dd>
          </div>
        ))}
      </dl>
      {info.images.length ? (
        <div className='flex flex-col gap-2'>
          <h3 className='text-[var(--inf-txt-2)]'>{t('Reference images')}</h3>
          <ul className='flex flex-wrap gap-2'>
            {info.images.map((url, index) => (
              <li key={url}>
                <Button
                  variant='ghost'
                  className={referenceCardClassName}
                  aria-label={t('Reference image {{index}}', {
                    index: formatNumber(index + 1, locale),
                  })}
                  disabled={!props.onOpenReference}
                  onClick={() => props.onOpenReference?.(url, 'image')}
                >
                  <FadeInImage
                    src={taskBrowserMediaUrl(url)}
                    alt=''
                    loading='lazy'
                    className='size-full object-contain'
                  />
                </Button>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
      {info.video ? (
        <div className='flex flex-col items-start gap-2'>
          <h3 className='text-[var(--inf-txt-2)]'>{t('Reference video')}</h3>
          <Button
            variant='ghost'
            className={referenceCardClassName}
            aria-label={t('Reference video')}
            disabled={!props.onOpenReference}
            onClick={() => {
              if (info.video) props.onOpenReference?.(info.video, 'video')
            }}
          >
            <video
              src={taskBrowserMediaUrl(info.video)}
              muted
              playsInline
              preload='metadata'
              className='pointer-events-none size-full object-contain'
              aria-hidden='true'
            />
            <span className='pointer-events-none absolute inset-0 grid place-items-center'>
              <Play
                className='size-4 fill-white text-white'
                aria-hidden='true'
              />
            </span>
          </Button>
        </div>
      ) : null}
    </section>
  )
}
