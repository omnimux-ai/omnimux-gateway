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

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'
import {
  Dialog as DialogRoot,
  DialogContent,
  DialogDescription,
  DialogTitle,
} from '@/components/ui/dialog'

import { itemDescription, itemTitle } from '../lib/item-text'
import type { PromptItem } from '../types'

/**
 * Media element of the detail layer. `h-full` is the whole point: the stage is
 * full-height, so the media has to grow with it instead of sitting at the top
 * of a half-empty pane.
 */
const MEDIA_CLASS = 'h-full w-full object-contain'

/** Wheel travel that counts as one deliberate flip, and the flip cooldown. */
const WHEEL_FLIP_THRESHOLD = 48
const WHEEL_FLIP_COOLDOWN_MS = 420
/** Horizontal touch travel that counts as a swipe. */
const SWIPE_THRESHOLD = 56

/**
 * Left stage of the detail layer. The poster stays until playback is asked
 * for, so opening a tile never starts a video the visitor did not request.
 */
function renderMedia(
  item: PromptItem,
  /** Localized caption; the media's accessible name follows the UI language. */
  title: string,
  playing: boolean,
  playLabel: string,
  onPlay: () => void
) {
  const cover = item.posterUrl || item.previewImageUrl

  if (item.previewVideoUrl && playing) {
    return (
      <video
        className={MEDIA_CLASS}
        src={item.previewVideoUrl}
        poster={cover}
        controls
        autoPlay
        loop
        playsInline
        aria-label={title}
      />
    )
  }

  if (item.previewVideoUrl) {
    return (
      <button
        type='button'
        aria-label={playLabel}
        onClick={onPlay}
        className='relative block h-full w-full cursor-pointer'
      >
        <img src={cover} alt={title} className={MEDIA_CLASS} />
        <span className='absolute inset-0 grid place-items-center bg-black/20'>
          <Play
            aria-hidden='true'
            className='size-14 fill-white text-white drop-shadow-[0_2px_12px_rgba(0,0,0,0.65)]'
          />
        </span>
      </button>
    )
  }

  return <img src={cover} alt={title} className={MEDIA_CLASS} />
}

export interface PromptPreviewModalProps {
  /** The ordered list the open tile belongs to; flipping walks this list. */
  items: PromptItem[]
  /** Index into `items`; out of range means the layer is closed. */
  index: number
  onIndexChange: (index: number) => void
  onClose: () => void
  onUsePrompt: (item: PromptItem) => void
}

/**
 * Detail layer of the prompt pages, ported from the loomi template dialog:
 * media on the left, everything readable and actionable in a right pane that
 * scrolls on its own.
 *
 * It fills the viewport so the media can own the stage instead of floating in
 * the middle of it, and it is a browser, not a single-tile viewer: neighbours
 * are reachable with the arrows, the left/right keys, a wheel or trackpad
 * flick, or a horizontal swipe on touch.
 *
 * This is the layer that owns the prompt. The tile deliberately carries only a
 * poster and a two-line caption, so all the words and both conversion actions
 * have to land here — that split is what keeps the grid scannable.
 *
 * It composes the `@/components/ui/dialog` primitive directly instead of the
 * `@/components/dialog` wrapper on purpose: the wrapper hard-codes a visible
 * title header, a body clamped to `calc(100vh-14rem)` and a footer action bar,
 * none of which this layout has. Overriding all three from the outside would be
 * a longer and more fragile diff than the composition below. Every control
 * inside is still the project's own (`Button`, `CopyButton`).
 */
export function PromptPreviewModal(props: PromptPreviewModalProps) {
  const { t, i18n } = useTranslation()
  const { items, index } = props
  const item = index >= 0 && index < items.length ? items[index] : null
  // The reading pane is the row's whole text, so both caption and summary follow
  // the active language rather than the language the crawl happened to use.
  const title = item ? itemTitle(item, i18n.language) : ''
  const description = item ? itemDescription(item, i18n.language) : ''
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

  // The poster has to come back when the visitor flips to another tile: a video
  // that is still playing would otherwise follow them down the list.
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
   * A wheel flick flips the tile, but only once the reading pane has nothing
   * left to scroll in that direction — otherwise the prompt text could never be
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
        aria-label={t('Prompt preview')}
        onWheel={handleWheel}
        onTouchStart={handleTouchStart}
        onTouchEnd={handleTouchEnd}
      >
        {item ? (
          <div className='flex h-full min-h-0 flex-col md:flex-row'>
            {/* Media stage: the poster stays until the visitor asks to play. */}
            <div className='relative flex min-h-0 flex-1 items-center justify-center overflow-hidden bg-black'>
              <MediaBackdrop item={item} />

              <div className='relative z-10 flex h-full w-full items-center justify-center'>
                {renderMedia(item, title, playing, t('Play the rendered video'), () =>
                  setPlaying(true)
                )}
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
                className='absolute start-1/2 bottom-4 z-20 -translate-x-1/2 rounded-full bg-black/60 px-3 py-1 font-mono text-xs font-medium text-white tabular-nums'
              >
                {index + 1} / {items.length}
              </span>
            </div>

            {/* Reading pane — the text scrolls, the actions stay pinned below it. */}
            <div className='bg-card flex max-h-[60dvh] min-h-0 w-full shrink-0 flex-col border-t md:h-full md:max-h-none md:w-[420px] md:border-s md:border-t-0 lg:w-[480px] xl:w-[540px]'>
              <div
                data-modal-scroll
                className='flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto p-5 md:p-7'
              >
                <div>
                  <p className='text-muted-foreground text-[11px] font-semibold tracking-[0.18em] uppercase'>
                    {item.modelLabel}
                  </p>
                  <DialogTitle className='mt-1 text-2xl font-semibold tracking-[-0.03em] break-words'>
                    {title}
                  </DialogTitle>
                  {description ? (
                    <DialogDescription className='text-muted-foreground mt-2 text-sm leading-6'>
                      {description}
                    </DialogDescription>
                  ) : null}
                </div>

                <div className='min-h-0 flex-1'>
                  <p className='text-muted-foreground mb-2 text-xs font-medium tracking-[0.16em] uppercase'>
                    {t('Prompt')}
                  </p>
                  <pre className='bg-muted/50 overflow-auto rounded-2xl p-4 text-[13px] leading-6 break-words whitespace-pre-wrap'>
                    {item.prompt}
                  </pre>
                </div>
              </div>

              <div className='bg-card flex shrink-0 flex-wrap items-center gap-2 border-t px-5 py-4 md:px-7'>
                {/*「应用这条提示词」对有在线演示的行隐藏（Claude 专区固定隐藏）*/}
                {item.demoUrl || item.modelKey === 'claude-opus-5.5' ? null : (
                  <Button
                    size='lg'
                    className='bg-foreground text-background hover:bg-foreground/90 gap-1.5 rounded-full px-4'
                    onClick={() => props.onUsePrompt(item)}
                  >
                    <Sparkles className='size-3.5' aria-hidden='true' />
                    {t('Use this prompt')}
                  </Button>
                )}
                <CopyButton
                  value={item.prompt}
                  variant='outline'
                  size='lg'
                  className='rounded-full px-4'
                  tooltip={t('Copy to clipboard')}
                  successTooltip={t('Copied!')}
                >
                  {t('Copy')}
                </CopyButton>
                {item.sourceUrl ? (
                  <a
                    href={item.sourceUrl}
                    target='_blank'
                    rel='noopener noreferrer'
                    className='text-muted-foreground hover:text-foreground ms-1 inline-flex items-center gap-1.5 text-xs transition-colors'
                  >
                    <ExternalLink className='size-3.5' aria-hidden='true' />
                    {t('Open original post')}
                  </a>
                ) : null}
                {item.demoUrl ? (
                  <a
                    href={item.demoUrl}
                    target='_blank'
                    rel='noopener noreferrer'
                    className='text-muted-foreground hover:text-foreground ms-1 inline-flex items-center gap-1.5 text-xs transition-colors'
                  >
                    <ExternalLink className='size-3.5' aria-hidden='true' />
                    {t('Live demo')}
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

/**
 * Ambient copy of the poster behind the media: it fills the letterboxed area so
 * the stage reads as one picture instead of a frame floating on a black wall.
 */
function MediaBackdrop(props: { item: PromptItem }) {
  const cover = props.item.posterUrl || props.item.previewImageUrl
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
