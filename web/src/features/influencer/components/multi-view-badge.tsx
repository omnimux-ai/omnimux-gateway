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

import { useState, type CSSProperties, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'

import type { MultiViewState } from '../lib/multiview'

interface Props {
  state: MultiViewState
  progress: number
  thumbnail: string | null
  original?: string | null
  onOpen: () => void
}

/**
 * The 1:1 tile anchored inside a card's main image. It appears once a board has
 * been requested for that card and carries the board's own state, so the list
 * never grows a second card for a derived render.
 */
export function MultiViewBadge(props: Props) {
  const { t } = useTranslation()
  const [failedThumbnail, setFailedThumbnail] = useState<string | null>(null)
  const identity = JSON.stringify([props.thumbnail, props.original])
  const source = failedThumbnail === identity ? props.original : props.thumbnail

  if (props.state === 'absent') return null

  const LABELS = {
    ready: t('View multi-view'),
    failed: t('Multi-view generation failed'),
    generating: t('Generating multi-view storyboard…'),
    queued: t('Queued multi-view storyboard…'),
    submitting: t('Submitting multi-view storyboard…'),
    review: t('Multi-view needs review'),
  } as const
  const label = LABELS[props.state]
  let activityLabel = t('Submitting…')
  if (props.state === 'queued') activityLabel = t('Queued')
  if (props.state === 'review') activityLabel = t('Needs review')

  const onOpen = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation()
    props.onOpen()
  }

  return (
    <button
      type='button'
      aria-label={label}
      title={label}
      onClick={onOpen}
      className={`inf-mv-tile inf-mv-tile--${props.state === 'ready' || props.state === 'failed' ? props.state : 'generating'}`}
    >
      {props.state === 'ready' && props.thumbnail ? (
        <img
          key={source}
          src={source ?? undefined}
          onError={() => setFailedThumbnail(identity)}
          alt=''
          className='h-full w-full object-cover'
          loading='lazy'
        />
      ) : null}

      {props.state === 'generating' ? (
        <>
          <span
            className='inf-mv-ring'
            style={
              { '--inf-mv-progress': `${props.progress}%` } as CSSProperties
            }
            aria-hidden='true'
          />
          <span className='inf-mv-pct'>{props.progress}%</span>
        </>
      ) : null}

      {props.state === 'queued' ||
      props.state === 'submitting' ||
      props.state === 'review' ? (
        <span className='inf-mv-pct'>{activityLabel}</span>
      ) : null}

      {props.state === 'failed' ? (
        <span className='inf-mv-fail' aria-hidden='true'>
          !
        </span>
      ) : null}
    </button>
  )
}
