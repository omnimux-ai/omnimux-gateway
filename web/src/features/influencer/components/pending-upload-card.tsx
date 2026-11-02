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
import { AlertCircle, Clock, Loader2, RotateCcw, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import type { PendingCharacterUpload } from '../types'

/**
 * One file inside a multi-select upload batch.
 *
 * The shared picker tile is a single selectable button, so it cannot host a
 * retry affordance; this card reuses the same frame classes and the project
 * Button instead. Only the batch's upload tile is a live region, so queued and
 * running cards never flood assistive technology with thirty announcements.
 */
export function PendingUploadCard(props: {
  upload: PendingCharacterUpload
  busy?: boolean
  onRetry: () => void
  onDismiss: () => void
}) {
  const { t } = useTranslation()
  const failed = props.upload.status === 'error'
  const waiting = props.upload.status === 'waiting'
  let state = 'uploading'
  if (failed) state = 'error'
  else if (waiting) state = 'waiting'
  return (
    <article className='inf-character-card-wrap'>
      <div
        className='inf-character-card inf-character-upload'
        data-state={state}
        role='group'
        aria-label={props.upload.fileName}
        aria-busy={!failed && !waiting}
      >
        {props.upload.previewUrl ? (
          <img
            className='inf-upload-preview'
            src={props.upload.previewUrl}
            alt=''
            aria-hidden='true'
          />
        ) : null}
        <span className='inf-upload-scrim' aria-hidden='true' />
        <span className='inf-upload-content'>
          {failed ? (
            <AlertCircle className='size-5' aria-hidden='true' />
          ) : null}
          {waiting ? <Clock className='size-5' aria-hidden='true' /> : null}
          {failed || waiting ? null : (
            <Loader2 className='size-5 animate-spin' aria-hidden='true' />
          )}
          {failed ? <span>{t('Upload failed')}</span> : null}
          {waiting ? <span>{t('Queued')}</span> : null}
          {failed || waiting ? (
            <span className='inf-upload-pct'>{props.upload.fileName}</span>
          ) : (
            <span className='sr-only'>{t('Uploading…')}</span>
          )}
        </span>
        {failed ? (
          <span className='inf-character-upload-actions'>
            <Button
              type='button'
              variant='secondary'
              size='sm'
              disabled={props.busy}
              aria-label={`${t('Retry upload')}: ${props.upload.fileName}`}
              onClick={props.onRetry}
            >
              <RotateCcw className='size-3.5' aria-hidden='true' />
              {t('Retry')}
            </Button>
            <Button
              type='button'
              variant='secondary'
              size='icon'
              className='size-7'
              disabled={props.busy}
              aria-label={`${t('Remove')}: ${props.upload.fileName}`}
              onClick={props.onDismiss}
            >
              <X className='size-3.5' aria-hidden='true' />
            </Button>
          </span>
        ) : null}
      </div>
    </article>
  )
}
