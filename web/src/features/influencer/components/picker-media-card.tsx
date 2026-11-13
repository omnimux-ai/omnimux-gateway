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
import { Film, ImageOff, Maximize2, Play } from 'lucide-react'
import { memo, useEffect, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { FadeInImage } from './fade-in-image'

export interface PickerMediaCardProps {
  title: string
  /** Card-only derivative. It never replaces the URL used for selection. */
  displaySrc: string | null
  /** Original image, tried once after a derivative fails. */
  fallbackSrc?: string | null
  /** Video poster lazy fallback: the browser reads metadata and paints frame 0
   * when no lightweight derivative exists (old uploads, missing server poster). */
  fallbackVideoSrc?: string | null
  kind: 'image' | 'video'
  selected: boolean
  disabled?: boolean
  /** First rows stay eager; everything below them decodes on demand. */
  priority?: boolean
  durationLabel?: string
  unavailableTitle?: string
  overlay?: ReactNode
  onSelect: () => void
  onPreview?: () => void
}

/**
 * One picker tile shared by the image and video panels: a square, selectable
 * button with a skeleton, one-shot fallback and an optional preview action.
 * Video tiles render their poster only; playback belongs to the preview dialog.
 */
export const PickerMediaCard = memo(function PickerMediaCard(
  props: PickerMediaCardProps
) {
  const { t } = useTranslation()
  const [src, setSrc] = useState<string | null>(props.displaySrc)
  const [usedFallback, setUsedFallback] = useState(false)
  const [loaded, setLoaded] = useState(false)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    setSrc(props.displaySrc)
    setUsedFallback(false)
    setLoaded(false)
    setFailed(false)
  }, [props.displaySrc, props.fallbackSrc])

  const onMediaError = () => {
    if (
      !usedFallback &&
      props.kind === 'image' &&
      props.fallbackSrc &&
      props.fallbackSrc !== src
    ) {
      setUsedFallback(true)
      setLoaded(false)
      setSrc(props.fallbackSrc)
      return
    }
    setFailed(true)
  }

  const videoFallback =
    !src && props.kind === 'video' ? props.fallbackVideoSrc : null
  const showPlaceholder = (!src && !videoFallback) || failed
  const placeholderIcon =
    props.kind === 'video' ? (
      <Film className='size-6' aria-hidden='true' />
    ) : (
      <ImageOff className='size-6' aria-hidden='true' />
    )

  return (
    <article className='inf-character-card-wrap'>
      <Button
        type='button'
        variant='ghost'
        aria-label={props.title}
        aria-pressed={props.selected}
        title={showPlaceholder ? props.unavailableTitle : undefined}
        disabled={props.disabled}
        onClick={props.onSelect}
        className={cn('inf-character-card', props.selected && 'is-selected')}
      >
        {videoFallback ? (
          <video
            src={videoFallback}
            preload='metadata'
            muted
            playsInline
            aria-hidden='true'
            className='inf-character-image'
            onLoadedMetadata={(event) => {
              const element = event.currentTarget
              try {
                element.currentTime = Math.min(0.1, element.duration || 0.1)
              } catch {
                /* A non-seekable source still paints its first frame. */
              }
            }}
          />
        ) : null}
        {src && !failed ? (
          <FadeInImage
            key={src}
            src={src}
            alt=''
            width={132}
            height={132}
            loading={props.priority ? 'eager' : 'lazy'}
            fetchPriority={props.priority ? 'high' : 'low'}
            decoding='async'
            className='inf-character-image'
            onLoadedChange={setLoaded}
            onError={onMediaError}
          />
        ) : null}
        {src && !failed && !loaded ? (
          <span className='inf-card-skeleton' aria-hidden='true'>
            <span className='inf-shimmer' />
          </span>
        ) : null}
        {showPlaceholder ? (
          <span className='inf-card-fallback' aria-hidden='true'>
            {placeholderIcon}
          </span>
        ) : null}
        {props.kind === 'video' && !showPlaceholder ? (
          <>
            <span className='inf-card-play' aria-hidden='true'>
              <Play className='size-3.5' />
            </span>
            {props.durationLabel ? (
              <span className='inf-card-duration'>{props.durationLabel}</span>
            ) : null}
          </>
        ) : null}
        {props.selected ? (
          <span className='inf-character-check'>
            <svg
              viewBox='0 0 24 24'
              fill='none'
              stroke='currentColor'
              strokeWidth='3'
              strokeLinecap='round'
              strokeLinejoin='round'
              className='size-3'
              aria-hidden='true'
            >
              <path d='M20 6 9 17l-5-5' />
            </svg>
          </span>
        ) : null}
        {props.overlay}
        {props.kind === 'image' ? (
          <span className='sr-only'>{props.title}</span>
        ) : null}
      </Button>
      {props.onPreview ? (
        <Button
          type='button'
          variant='ghost'
          size='icon'
          className='inf-character-preview-action inf-character-icon'
          aria-label={`${t('Preview')}: ${props.title}`}
          title={
            props.kind === 'image' ? t('Click to view image') : t('Preview')
          }
          onClick={props.onPreview}
        >
          <Maximize2 className='preview-icon size-3' aria-hidden='true' />
        </Button>
      ) : null}
    </article>
  )
})
