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
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { useMediaRecovery } from '../hooks/use-media-recovery'
import { FadeInImage } from './fade-in-image'

interface Props {
  identity: string
  src: string | null
  fallbackSrc?: string | null
  poster?: string | null
  video: boolean
  playing?: boolean
  detail?: boolean
  label?: string
  muted?: boolean
  loop?: boolean
  onUnavailableChange?: (unavailable: boolean) => void
  onReloadRecord?: () => void
}

/** Cards and detail share recovery; changing result identity resets the budget. */
export function TaskMediaPreview(props: Props) {
  const { t } = useTranslation()
  if (!props.src) {
    return (
      <div className='relative h-full w-full'>
        {props.video && props.poster ? (
          <Cover key={props.poster} source={props.poster} hidden={false} />
        ) : null}
        <div className='absolute inset-0 flex flex-col items-center justify-center gap-2 p-3 text-center text-xs text-[var(--inf-txt-3)]'>
          <span
            role='status'
            className='rounded-lg bg-black/60 px-3 py-2 text-white'
          >
            {t('Preview unavailable')}
          </span>
          {props.onReloadRecord ? (
            <Button
              variant='secondary'
              size='sm'
              onClick={(event) => {
                event.stopPropagation()
                props.onReloadRecord?.()
              }}
            >
              {t('Reload preview')}
            </Button>
          ) : null}
        </div>
      </div>
    )
  }
  return (
    <MediaSource
      key={JSON.stringify([
        props.identity,
        props.src,
        props.fallbackSrc,
        props.video,
      ])}
      {...props}
      src={props.src}
    />
  )
}

/** A derivative failure switches once to the original, then uses normal recovery. */
function MediaSource(props: Props & { src: string }) {
  const [useOriginal, setUseOriginal] = useState(false)
  const canFallback =
    !props.video &&
    !useOriginal &&
    Boolean(props.fallbackSrc && props.fallbackSrc !== props.src)
  const source = useOriginal ? (props.fallbackSrc ?? props.src) : props.src
  return (
    <MediaLifecycle
      {...props}
      key={source}
      src={source}
      onSourceError={canFallback ? () => setUseOriginal(true) : undefined}
    />
  )
}

function MediaLifecycle(
  props: Props & { src: string; onSourceError?: () => void }
) {
  const { t } = useTranslation()
  const media = useMediaRecovery(props.src)
  const videoRef = useRef<HTMLVideoElement>(null)
  const showMedia = media.phase === 'loading' || media.phase === 'ready'
  const unavailable = media.phase === 'waiting' || media.phase === 'unavailable'
  const [playingAttempt, setPlayingAttempt] = useState<string | null>(null)
  const [failedCover, setFailedCover] = useState<string | null>(null)
  const coverAvailable = Boolean(props.poster && props.poster !== failedCover)
  const showVideo =
    media.phase === 'ready' &&
    (props.detail || (props.playing && playingAttempt === media.key))

  const onUnavailableChange = props.onUnavailableChange
  useEffect(() => {
    onUnavailableChange?.(unavailable)
  }, [unavailable, onUnavailableChange])

  useEffect(() => {
    const el = videoRef.current
    // StrictMode replays effects without replacing the native node.
    if (el && !el.getAttribute('src')) el.setAttribute('src', media.src)
    return () => {
      if (!el) return
      el.pause()
      el.removeAttribute('src')
      el.load()
    }
  }, [media.key, media.src, showMedia])

  useEffect(() => {
    const el = videoRef.current
    if (!el || props.detail) return
    if (props.playing) {
      void el.play().catch(() => undefined)
    } else {
      el.pause()
      el.currentTime = 0
    }
  }, [props.playing, props.detail, media.key, showMedia])

  return (
    <div
      className={cn(
        'relative h-full w-full',
        props.video && 'inf-history-video'
      )}
    >
      {props.video && props.poster ? (
        <Cover
          key={props.poster}
          source={props.poster}
          hidden={Boolean(showVideo && !unavailable)}
          onAvailabilityChange={(available) =>
            setFailedCover(available ? null : (props.poster ?? null))
          }
        />
      ) : null}
      {showMedia && props.video ? (
        <video
          key={media.key}
          ref={videoRef}
          src={media.src}
          aria-label={props.label}
          muted={props.muted ?? !props.detail}
          loop={props.loop ?? !props.detail}
          controls={props.detail}
          autoPlay={props.detail}
          playsInline
          preload={props.playing || props.detail ? 'auto' : 'metadata'}
          onLoadedData={media.onLoad}
          onPlaying={() => setPlayingAttempt(media.key)}
          onPause={() => setPlayingAttempt(null)}
          onError={media.onError}
          className={cn(
            'absolute inset-0 h-full w-full object-contain transition-opacity duration-150',
            coverAvailable && !showVideo && 'opacity-0'
          )}
        />
      ) : null}
      {showMedia && !props.video ? (
        <FadeInImage
          key={media.key}
          src={media.src}
          alt={props.detail ? t('Preview') : ''}
          loading={props.detail ? 'eager' : 'lazy'}
          decoding='async'
          onLoad={media.onLoad}
          onError={props.onSourceError ?? media.onError}
          className='h-full w-full object-contain'
        />
      ) : null}
      {props.video && !props.detail && !props.playing && !unavailable ? (
        <span
          aria-hidden='true'
          className='pointer-events-none absolute inset-0 grid place-items-center bg-black/10'
        >
          <span className='grid h-10 w-10 place-items-center rounded-full bg-black/45 backdrop-blur-sm'>
            <Play className='h-4 w-4 fill-white text-white' />
          </span>
        </span>
      ) : null}
      {unavailable ? (
        <div className='absolute inset-0 flex flex-col items-center justify-center gap-2 p-3 text-center'>
          <span
            role='status'
            className='rounded-lg bg-black/60 px-3 py-2 text-xs text-white'
          >
            {media.phase === 'waiting'
              ? t('Loading preview…')
              : t('Preview unavailable')}
          </span>
          {media.phase === 'unavailable' ? (
            <Button
              variant='secondary'
              size='sm'
              onClick={(event) => {
                event.stopPropagation()
                media.reload()
              }}
            >
              {t('Reload preview')}
            </Button>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

/** Cover failure has its own budget and never changes the video lifecycle. */
function Cover(props: {
  source: string
  hidden: boolean
  onAvailabilityChange?: (available: boolean) => void
}) {
  const media = useMediaRecovery(props.source)
  const unavailable = media.phase === 'waiting' || media.phase === 'unavailable'
  const onAvailabilityChange = props.onAvailabilityChange
  useEffect(() => {
    onAvailabilityChange?.(!unavailable)
  }, [unavailable, onAvailabilityChange])
  if (media.phase === 'waiting' || media.phase === 'unavailable') return null
  return (
    <img
      key={media.key}
      src={media.src}
      alt=''
      loading='lazy'
      decoding='async'
      onLoad={media.onLoad}
      onError={media.onError}
      className={cn(
        'h-full w-full object-contain transition-opacity duration-150',
        props.hidden && 'opacity-0'
      )}
    />
  )
}
