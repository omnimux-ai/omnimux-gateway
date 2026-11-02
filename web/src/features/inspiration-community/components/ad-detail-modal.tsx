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
import {
  ChevronLeft,
  ChevronRight,
  ExternalLink,
  Play,
  Sparkles,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog as DialogRoot,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog'
import { toIntlLocale } from '@/i18n/languages'

import { TRAFFIC_LABEL_KEYS } from '../constants'
import {
  UNKNOWN_METRIC,
  formatCount,
  formatDuration,
  formatViews,
} from '../lib/format'
import type { TikTokAdItem } from '../types'

/**
 * Media element of the detail layer. `h-full` is the whole point: the stage is
 * full-height, so the creative has to grow with it instead of sitting at the
 * top of a half-empty pane.
 */
const MEDIA_CLASS = 'h-full w-full object-contain'

/** Wheel travel that counts as one deliberate flip, and the flip cooldown. */
const WHEEL_FLIP_THRESHOLD = 48
const WHEEL_FLIP_COOLDOWN_MS = 420
/** Horizontal touch travel that counts as a swipe. */
const SWIPE_THRESHOLD = 56

/**
 * Creative media of the stage.
 *
 * Playback is TikTok's own player and it is mounted only once asked for; the
 * cover stays until then. A creative with no `embedUrl` gets its cover and
 * nothing else — `mediaUrls` is not a fallback, because every object behind it
 * answers 404 today, so a player built from it would open on an error page.
 *
 * The player is not sandboxed, and that is deliberate: it is TikTok's document
 * on TikTok's origin and it needs its own scripts and its own origin to fetch
 * its media. A sandbox would either break playback (`allow-scripts` alone gives
 * it an opaque origin) or keep `allow-scripts allow-same-origin`, which the
 * linter refuses and which only matters for a frame that can already reach this
 * document — this one cannot.
 */
function AdMedia(props: {
  item: TikTokAdItem
  playing: boolean
  playLabel: string
  onPlay: () => void
}) {
  const [coverFailed, setCoverFailed] = useState(false)
  const cover = props.item.coverUrl
  const showCover = cover !== null && !coverFailed
  const embedUrl = props.item.embedUrl

  if (embedUrl !== null && props.playing) {
    return (
      // eslint-disable-next-line react/iframe-missing-sandbox -- cross-origin TikTok document
      <iframe
        className='h-full w-full border-0'
        src={embedUrl}
        title={props.item.title || props.item.caption || props.playLabel}
        allow='autoplay; fullscreen'
        allowFullScreen
        loading='lazy'
      />
    )
  }

  const still = showCover ? (
    <img
      src={cover}
      alt={props.item.title || props.item.caption}
      onError={() => setCoverFailed(true)}
      className={MEDIA_CLASS}
    />
  ) : (
    <span
      aria-hidden='true'
      className='block h-full w-full'
      style={{ background: props.item.coverGradient }}
    />
  )

  if (embedUrl === null) {
    return still
  }

  return (
    <button
      type='button'
      aria-label={props.playLabel}
      onClick={props.onPlay}
      className='relative block h-full w-full cursor-pointer'
    >
      {still}
      <span className='absolute inset-0 grid place-items-center bg-black/20'>
        <Play
          aria-hidden='true'
          className='size-14 fill-white text-white drop-shadow-[0_2px_12px_rgba(0,0,0,0.65)]'
        />
      </span>
    </button>
  )
}

export interface AdDetailModalProps {
  /** The ordered list the open card belongs to; flipping walks this list. */
  items: TikTokAdItem[]
  /** Index into `items`; out of range means the layer is closed. */
  index: number
  onIndexChange: (index: number) => void
  onClose: () => void
  onRecreate: (item: TikTokAdItem) => void
}

/**
 * Detail layer of the ad library: the creative on the left, everything readable
 * and actionable in a right pane that scrolls on its own.
 *
 * It fills the viewport so the media can own the stage instead of floating in
 * the middle of it, and it is a browser, not a single-card viewer: neighbours
 * are reachable with the arrows, the left/right keys, a wheel or trackpad
 * flick, or a horizontal swipe on touch. The list it walks is the one the
 * caller is showing — the filtered page the service returned — so at its end
 * the layer stops instead of inventing rows nobody has loaded.
 *
 * It composes the `@/components/ui/dialog` primitive directly instead of the
 * `@/components/dialog` wrapper on purpose: the wrapper hard-codes a visible
 * title header, a body clamped to `calc(100vh-14rem)` and a footer action bar,
 * none of which this layout has. Overriding all three from the outside would be
 * a longer and more fragile diff than the composition below. Every control
 * inside is still the project's own (`Button`, `Badge`).
 */
export function AdDetailModal(props: AdDetailModalProps) {
  const { t, i18n } = useTranslation()
  const { items, index } = props
  const item = index >= 0 && index < items.length ? items[index] : null
  const dateLocale = toIntlLocale(i18n.language)
  const [playing, setPlaying] = useState(false)
  const wheelTravel = useRef(0)
  const lastFlipAt = useRef(0)
  const touchStart = useRef<{ x: number; y: number } | null>(null)

  const open = item !== null
  const canGoPrevious = index > 0
  const canGoNext = index >= 0 && index < items.length - 1

  const go = (delta: number) => {
    const next = index + delta
    if (next < 0 || next >= items.length || next === index) {
      return
    }
    props.onIndexChange(next)
  }

  // The cover has to come back when the visitor flips to another creative: a
  // player that is still mounted would otherwise follow them down the list.
  useEffect(() => {
    setPlaying(false)
  }, [index])

  /**
   * The arrows are read on the capture phase, on purpose.
   *
   * The layer sits on the project's `@/components/ui/dialog`, which wraps Base
   * UI: its popup calls `stopPropagation()` on every key in Base UI's
   * `COMPOSITE_KEYS` — the four arrows, Home and End — so a listener on the
   * bubble phase is cut off before the event ever reaches `window`. A test that
   * dispatches straight at `window` never crosses the popup and passes anyway,
   * which is why this only ever broke in a real browser. Capturing from the
   * window runs before the popup's own handler and before any descendant's
   * `stopPropagation`, so the arrows work wherever focus sits in the layer.
   *
   * It cannot help while focus is inside TikTok's player: that document lives
   * in a cross-origin iframe, so its keystrokes are not this page's events at
   * all. Clicking the stage hands focus back and the arrows work again.
   *
   * `Escape` is not handled here: closing stays with the dialog primitive's own
   * default.
   */
  useEffect(() => {
    if (!open) {
      return
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'ArrowLeft') {
        event.preventDefault()
        go(-1)
        return
      }
      if (event.key === 'ArrowRight') {
        event.preventDefault()
        go(1)
      }
    }
    window.addEventListener('keydown', onKeyDown, true)
    return () => window.removeEventListener('keydown', onKeyDown, true)
  })

  /**
   * A wheel flick flips the card, but only once the reading pane has nothing
   * left to scroll in that direction — otherwise the caption could never be
   * read to its end.
   */
  const handleWheel = (event: React.WheelEvent<HTMLDivElement>) => {
    const horizontal = Math.abs(event.deltaX) > Math.abs(event.deltaY)
    const scroller = (event.target as HTMLElement).closest<HTMLElement>(
      '[data-modal-scroll]'
    )
    if (scroller && !horizontal) {
      const atTop = scroller.scrollTop <= 0
      const atBottom =
        scroller.scrollTop + scroller.clientHeight >= scroller.scrollHeight - 1
      if ((event.deltaY > 0 && !atBottom) || (event.deltaY < 0 && !atTop)) {
        return
      }
    }

    wheelTravel.current += horizontal ? event.deltaX : event.deltaY
    if (Math.abs(wheelTravel.current) < WHEEL_FLIP_THRESHOLD) {
      return
    }
    const now = Date.now()
    if (now - lastFlipAt.current < WHEEL_FLIP_COOLDOWN_MS) {
      return
    }
    const delta = wheelTravel.current > 0 ? 1 : -1
    wheelTravel.current = 0
    lastFlipAt.current = now
    go(delta)
  }

  const handleTouchStart = (event: React.TouchEvent<HTMLDivElement>) => {
    const touch = event.touches[0]
    touchStart.current = touch ? { x: touch.clientX, y: touch.clientY } : null
  }

  const handleTouchEnd = (event: React.TouchEvent<HTMLDivElement>) => {
    const start = touchStart.current
    touchStart.current = null
    const touch = event.changedTouches[0]
    if (!start || !touch) {
      return
    }
    const dx = touch.clientX - start.x
    const dy = touch.clientY - start.y
    if (Math.abs(dx) < SWIPE_THRESHOLD || Math.abs(dx) <= Math.abs(dy)) {
      return
    }
    go(dx < 0 ? 1 : -1)
  }

  return (
    <DialogRoot
      open={open}
      onOpenChange={(nextOpen) => {
        if (!nextOpen) props.onClose()
      }}
    >
      <DialogContent
        className='inset-0 h-[100dvh] w-screen max-w-none translate-x-0 translate-y-0 gap-0 rounded-none p-0 ring-0 sm:max-w-none'
        aria-label={t('Ad details')}
        onWheel={handleWheel}
        onTouchStart={handleTouchStart}
        onTouchEnd={handleTouchEnd}
      >
        {item ? (
          <div className='flex h-full min-h-0 flex-col md:flex-row'>
            {/* Media stage: the cover stays until the visitor asks to play. */}
            <div className='relative flex min-h-0 flex-1 items-center justify-center overflow-hidden bg-black'>
              <MediaBackdrop item={item} />

              <div className='relative z-10 flex h-full w-full items-center justify-center'>
                <AdMedia
                  key={item.id}
                  item={item}
                  playing={playing}
                  playLabel={t('Play video')}
                  onPlay={() => setPlaying(true)}
                />
              </div>

              <NavigationButton
                side='start'
                label={t('Previous')}
                disabled={!canGoPrevious}
                onClick={() => go(-1)}
              />
              <NavigationButton
                side='end'
                label={t('Next')}
                disabled={!canGoNext}
                onClick={() => go(1)}
              />

              <span
                aria-hidden='true'
                className='absolute bottom-4 start-1/2 z-20 -translate-x-1/2 rounded-full bg-black/60 px-3 py-1 font-mono text-xs font-medium text-white tabular-nums'
              >
                {index + 1} / {items.length}
              </span>
            </div>

            {/* Reading pane — scrolls independently of the media. */}
            <div
              data-modal-scroll
              className='bg-card flex min-h-0 w-full shrink-0 flex-col gap-4 overflow-y-auto border-t p-5 md:h-full md:w-[420px] md:border-s md:border-t-0 md:p-7 lg:w-[480px] xl:w-[540px]'
            >
              <div>
                <DialogTitle className='text-2xl font-semibold tracking-[-0.03em] break-words'>
                  {item.title || item.caption || t('Ad details')}
                </DialogTitle>
                {item.caption && item.caption !== item.title ? (
                  <DialogDescription className='text-muted-foreground mt-2 text-sm leading-6 break-words whitespace-pre-wrap'>
                    {item.caption}
                  </DialogDescription>
                ) : null}
              </div>

              <div className='flex flex-wrap items-center gap-2'>
                {item.country ? (
                  <Badge variant='outline'>{item.country}</Badge>
                ) : null}
                {item.category ? (
                  <Badge variant='outline'>{item.category}</Badge>
                ) : null}
                {item.trafficType ? (
                  <Badge variant='ghost'>
                    {t(TRAFFIC_LABEL_KEYS[item.trafficType])}
                  </Badge>
                ) : null}
                {item.postedAt ? (
                  <span className='text-muted-foreground text-xs'>
                    {new Date(item.postedAt).toLocaleDateString(dateLocale)}
                  </span>
                ) : null}
              </div>

              <div className='grid grid-cols-2 gap-2'>
                <MetricBlock
                  label={t('Views')}
                  value={formatViews(item.views)}
                  hint={
                    item.views === null ? undefined : formatCount(item.views)
                  }
                />
                <MetricBlock
                  label={t('Clip length')}
                  value={formatDuration(item.durationSeconds)}
                />
              </div>

              {item.creator ? (
                <dl className='flex flex-col gap-0.5 text-sm'>
                  <dt className='text-muted-foreground text-xs'>
                    {t('Creator')}
                  </dt>
                  <dd className='truncate font-medium'>
                    {item.creator.name || item.creator.handle || UNKNOWN_METRIC}
                    {item.creator.name && item.creator.handle ? (
                      <span className='text-muted-foreground'>
                        {' '}
                        @{item.creator.handle}
                      </span>
                    ) : null}
                  </dd>
                </dl>
              ) : null}

              <div className='mt-auto flex flex-wrap items-center gap-2 pt-1'>
                <Button
                  size='lg'
                  className='bg-foreground text-background hover:bg-foreground/90 gap-1.5 rounded-full px-4'
                  onClick={() => props.onRecreate(item)}
                >
                  <Sparkles className='size-3.5' aria-hidden='true' />
                  {t('Recreate this ad')}
                </Button>
                {item.sourceUrl ? (
                  <a
                    href={item.sourceUrl}
                    target='_blank'
                    rel='noopener noreferrer'
                    className='text-muted-foreground hover:text-foreground ms-1 inline-flex items-center gap-1.5 text-xs transition-colors'
                  >
                    <ExternalLink className='size-3.5' aria-hidden='true' />
                    {t('View original post')}
                  </a>
                ) : null}
              </div>
            </div>
          </div>
        ) : null}
      </DialogContent>
    </DialogRoot>
  )
}

function MetricBlock(props: { label: string; value: string; hint?: string }) {
  return (
    <div className='bg-muted/40 flex flex-col gap-1 rounded-xl border p-3'>
      <span className='text-muted-foreground text-[10px] font-medium tracking-[0.08em] uppercase'>
        {props.label}
      </span>
      <span className='font-mono text-base font-semibold'>{props.value}</span>
      {props.hint && (
        <span className='text-muted-foreground text-[11px]'>{props.hint}</span>
      )}
    </div>
  )
}

/**
 * Ambient copy of the cover behind the media: it fills the letterboxed area so
 * the stage reads as one picture instead of a frame floating on a black wall.
 */
function MediaBackdrop(props: { item: TikTokAdItem }) {
  const cover = props.item.coverUrl
  if (!cover) {
    return null
  }
  return (
    <img
      src={cover}
      alt=''
      aria-hidden='true'
      className='absolute inset-0 h-full w-full scale-110 object-cover opacity-25 blur-2xl'
    />
  )
}

function NavigationButton(props: {
  side: 'start' | 'end'
  label: string
  disabled: boolean
  onClick: () => void
}) {
  return (
    <button
      type='button'
      aria-label={props.label}
      disabled={props.disabled}
      onClick={props.onClick}
      className={`absolute top-1/2 z-20 grid size-11 -translate-y-1/2 place-items-center rounded-full bg-black/55 text-white backdrop-blur-sm transition-opacity hover:bg-black/75 disabled:pointer-events-none disabled:opacity-0 ${
        props.side === 'start' ? 'start-4' : 'end-4'
      }`}
    >
      {props.side === 'start' ? (
        <ChevronLeft className='size-5' aria-hidden='true' />
      ) : (
        <ChevronRight className='size-5' aria-hidden='true' />
      )}
    </button>
  )
}
