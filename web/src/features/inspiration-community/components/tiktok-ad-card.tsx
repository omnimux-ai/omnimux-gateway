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

import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'

import { TRAFFIC_LABEL_KEYS } from '../constants'
import { useCoverLoadState } from '../hooks/use-cover-load-state'
import {
  UNKNOWN_METRIC,
  formatDuration,
  formatViews,
} from '../lib/format'
import type { TikTokAdItem } from '../types'

export interface TikTokAdCardProps {
  item: TikTokAdItem
  onOpen: (item: TikTokAdItem) => void
}

function MetricChip(props: { label: string; value: string }) {
  return (
    <span className='metricChip flex min-w-0 flex-1 flex-col items-start gap-0.5 rounded-lg px-1.5 py-1 transition-colors'>
      <span className='text-muted-foreground text-[10px] font-medium tracking-[0.06em] uppercase'>
        {props.label}
      </span>
      <span className='w-full truncate font-mono text-xs font-semibold'>
        {props.value}
      </span>
    </span>
  )
}

/**
 * TikTok ad card — cover with title overlay, then the three metric chips the
 * service can actually answer: play count, clip length and traffic type
 * (PRD section 4.3).
 *
 * Everything on the card is a field of the payload, or hidden:
 *
 * - the play affordance appears only for a creative with playable media, so a
 *   photo post does not advertise a video it has none of;
 * - the region badge is dropped when `country` is empty instead of rendering an
 *   empty pill (1,323 of the 2,954 rows carry no region);
 * - a metric the payload does not report prints `—`, never `0`.
 *
 * The affordance is a still-only tile: it renders the cover and never a
 * `<video>`, so nothing in the grid can play on its own. TikTok's own player is
 * mounted in `ad-detail-modal` after the visitor asks for it.
 *
 * Until the cover lands the tile shows a skeleton over the same box, and the
 * image fades in over it; a cover that fails to load drops the image and leaves
 * the creative's gradient backdrop, so the tile is never stuck behind a pulse.
 */
export function TikTokAdCard(props: TikTokAdCardProps) {
  const { t } = useTranslation()
  const item = props.item
  const cover = item.coverUrl
  const coverLoad = useCoverLoadState(cover ?? '')

  // `title` is the service's headline and `caption` its post text; either can be
  // empty, and one of them standing in for the other is not a fabrication.
  const heading = item.title || item.caption
  const showCover = cover !== null && !coverLoad.failed
  // Only a creative with TikTok's own player gets the play affordance; a badge
  // on the other 76% would promise a video that cannot be opened.
  const playable = item.embedUrl !== null

  return (
    <button
      type='button'
      onClick={() => props.onOpen(item)}
      aria-label={heading || t('Ad details')}
      className={cn(
        'group bg-card text-card-foreground flex w-full flex-col overflow-hidden rounded-2xl border text-left shadow-xs',
        'transition-[transform,box-shadow,border-color] duration-200',
        'hover:-translate-y-1 hover:border-foreground/20 hover:shadow-lg',
        'focus-visible:ring-ring focus-visible:ring-2 focus-visible:outline-none'
      )}
    >
      <div
        className='relative aspect-[3/4] w-full overflow-hidden'
        style={{ background: item.coverGradient }}
      >
        {showCover ? (
          <img
            ref={coverLoad.imageRef}
            src={cover}
            alt=''
            loading='lazy'
            onLoad={coverLoad.onLoad}
            onError={coverLoad.onError}
            className={cn(
              'absolute inset-0 h-full w-full object-cover transition-opacity duration-300',
              coverLoad.loaded ? 'opacity-100' : 'opacity-0'
            )}
          />
        ) : null}

        {/* Skeleton of the cover box itself: same size and clip, so finishing
            the load changes nothing but the pixels. */}
        {showCover && coverLoad.pending ? (
          <Skeleton
            aria-hidden='true'
            data-testid='tiktok-ad-card-skeleton'
            className='absolute inset-0 size-full rounded-none'
          />
        ) : null}

        {playable ? (
          <span
            aria-hidden='true'
            className='absolute top-1/2 left-1/2 flex size-11 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full bg-black/40 text-white backdrop-blur-sm transition-transform duration-200 group-hover:scale-110'
          >
            <Play className='size-4.5 fill-current' />
          </span>
        ) : null}

        {heading ? (
          <div className='bg-gradient-to-t from-black/80 via-black/25 to-transparent absolute inset-x-0 bottom-0 p-3 pt-8'>
            <span className='line-clamp-2 text-[13px] leading-snug font-semibold text-white'>
              {heading}
            </span>
          </div>
        ) : null}

        {item.country ? (
          <span className='absolute top-2.5 left-2.5 rounded-full bg-black/45 px-2 py-0.5 font-mono text-[10px] font-semibold text-white'>
            {item.country}
          </span>
        ) : null}
      </div>

      <div className='metrics border-t flex items-stretch gap-1 p-1.5 group-hover:bg-muted/40'>
        <MetricChip label={t('Views')} value={formatViews(item.views)} />
        <MetricChip
          label={t('Clip length')}
          value={formatDuration(item.durationSeconds)}
        />
        <MetricChip
          label={t('Traffic')}
          value={
            item.trafficType
              ? t(TRAFFIC_LABEL_KEYS[item.trafficType])
              : UNKNOWN_METRIC
          }
        />
      </div>
    </button>
  )
}
