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

import { useCoverLoadState } from '../hooks/use-cover-load-state'
import { itemTitle } from '../lib/item-text'
import { isVideoCoverUrl } from '../lib/media'
import type { PromptItem } from '../types'

export interface PromptDetailCardProps {
  item: PromptItem
  onOpenPreview: (item: PromptItem) => void
}

/**
 * Listing tile of the L2 model page, ported 1:1 from the loomi template tile.
 *
 * The whole tile is a single button that opens the detail layer: the browsing
 * surface carries nothing but the media and a two-line caption over a bottom
 * scrim, and every word of the prompt plus both actions live in the dialog
 * (`prompt-preview-modal`). Keeping the split is the point — the grid stays
 * scannable because it never tries to be the detail page.
 *
 * The media box is sized by the poster's own aspect ratio, baked at build time
 * from the image header (`PromptItem.aspectRatio`). That is what makes the
 * waterfall ragged instead of a uniform grid, and it reserves the space before
 * the image loads so the columns never jump.
 *
 * The tile carries a still and never a player: no `<video>` is mounted here at
 * any point, so a grid of tiles cannot start playing on its own. A cover URL
 * that is really a video file is dropped rather than rendered — an `<img>`
 * pointed at an `.mp4` would draw a broken frame — and the box keeps its flat
 * backdrop instead. Playback belongs to `prompt-preview-modal` alone.
 *
 * Until the cover lands the tile shows a skeleton of its own size, and the
 * image fades in over it, so a column of tiles fills in instead of popping.
 */
export function PromptDetailCard(props: PromptDetailCardProps) {
  const { i18n } = useTranslation()
  const item = props.item
  const title = itemTitle(item, i18n.language)
  const isVideo = item.mediaType === 'video'
  const cover = item.posterUrl || item.previewImageUrl
  const stillCover = isVideoCoverUrl(cover) ? '' : cover
  const coverLoad = useCoverLoadState(stillCover)

  return (
    <article id={`prompt-${item.slug}`} className='min-w-0 scroll-mt-24'>
      <button
        type='button'
        aria-label={title}
        onClick={() => props.onOpenPreview(item)}
        className='group bg-muted focus-visible:ring-ring relative block w-full min-w-0 overflow-hidden rounded-2xl text-left transition duration-200 hover:-translate-y-0.5 focus-visible:ring-2 focus-visible:outline-none'
      >
        <span
          className='bg-muted relative block overflow-hidden'
          style={{ aspectRatio: item.aspectRatio }}
        >
          {stillCover ? (
            <img
              ref={coverLoad.imageRef}
              src={stillCover}
              alt=''
              loading='lazy'
              decoding='async'
              onLoad={coverLoad.onLoad}
              onError={coverLoad.onError}
              className={cn(
                'block size-full object-contain transition-[opacity,transform] duration-300 group-hover:scale-[1.015]',
                coverLoad.loaded ? 'opacity-100' : 'opacity-0'
              )}
            />
          ) : null}

          {/* Skeleton of the cover box itself: same size and clip, so finishing
              the load changes nothing but the pixels. */}
          {coverLoad.pending ? (
            <Skeleton
              aria-hidden='true'
              data-testid='prompt-card-skeleton'
              className='absolute inset-0 size-full rounded-none'
            />
          ) : null}

          <span className='pointer-events-none absolute inset-0 grid place-items-center opacity-0 transition group-hover:bg-black/15 group-hover:opacity-100'>
            {isVideo ? (
              <Play
                aria-hidden='true'
                className='size-8 fill-white/90 text-white/90 drop-shadow'
              />
            ) : null}
          </span>
        </span>

        {/* Caption scrim: the only text the browsing layer carries. */}
        <span className='pointer-events-none absolute inset-x-0 bottom-0 bg-gradient-to-t from-black/80 via-black/35 to-transparent px-3 pt-12 pb-3 text-white'>
          <span className='mb-1 inline-flex items-center gap-1 text-[11px] font-medium tracking-[0.14em] text-white/80 uppercase'>
            {isVideo ? (
              <Play aria-hidden='true' className='size-3 fill-current' />
            ) : null}
            {item.modelLabel}
          </span>
          <span className='line-clamp-2 text-sm leading-5 font-semibold break-words drop-shadow-sm sm:text-base'>
            {title}
          </span>
        </span>
      </button>
    </article>
  )
}
