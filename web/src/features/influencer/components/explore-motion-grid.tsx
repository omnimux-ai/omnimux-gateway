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

import { Clapperboard, Play, Wand2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { cn } from '@/lib/utils'

import type { MotionExplorePreset } from '../types'

interface Props {
  presets: MotionExplorePreset[]
  /** Opens the enlarged preview dialog for this preset. */
  onPreview: (preset: MotionExplorePreset) => void
  /** Fills the video form straight from the card, skipping the dialog. */
  onApply: (preset: MotionExplorePreset) => void
}

/**
 * The motion shelves: official presets and community trends, as a masonry of
 * video cards. Hovering a card swaps the still for the clip; on touch, where
 * there is no hover, the clip autoplays muted and the Use button stays put.
 */
export function ExploreMotionGrid(props: Props) {
  const { t } = useTranslation()

  if (!props.presets.length) {
    return (
      <EmptyState
        icon={Clapperboard}
        title={t('No motion presets found')}
        description={t('Motion presets are unavailable right now')}
      />
    )
  }

  return (
    <div className='columns-2 gap-4 py-4 sm:columns-3 xl:columns-4'>
      {props.presets.map((preset) => (
        <ExploreMotionCard
          key={preset.id}
          preset={preset}
          onPreview={props.onPreview}
          onApply={props.onApply}
        />
      ))}
    </div>
  )
}

interface CardProps {
  preset: MotionExplorePreset
  onPreview: (preset: MotionExplorePreset) => void
  onApply: (preset: MotionExplorePreset) => void
}

function ExploreMotionCard(props: CardProps) {
  const { t } = useTranslation()
  const videoRef = useRef<HTMLVideoElement>(null)
  const [playing, setPlaying] = useState(false)
  const [failed, setFailed] = useState(false)
  const preset = props.preset

  const isTouch =
    typeof window !== 'undefined' && window.matchMedia('(hover: none)').matches

  // Touch devices have no hover to trigger playback; let the first card's
  // clip run so the shelf does not look broken.
  useEffect(() => {
    if (isTouch) setPlaying(true)
  }, [isTouch])

  useEffect(() => {
    const el = videoRef.current
    if (!el) return
    if (playing) {
      void el.play().catch(() => undefined)
    } else {
      el.pause()
      el.currentTime = 0
    }
  }, [playing])

  if (failed) return null

  return (
    <article
      className={cn('group/motion relative isolate mb-4 break-inside-avoid')}
      onMouseEnter={() => setPlaying(true)}
      onMouseLeave={() => setPlaying(false)}
      onFocusCapture={() => setPlaying(true)}
      onBlurCapture={() => setPlaying(false)}
    >
      <button
        type='button'
        aria-label={t(preset.title)}
        onClick={() => props.onPreview(preset)}
        className='block w-full cursor-pointer overflow-hidden rounded-[var(--radius-card)] bg-[var(--inf-surface-2)] ring-1 ring-[var(--inf-line)] ring-inset'
      >
        <div className='relative aspect-[2/3] w-full'>
          <img
            src={preset.thumbPath}
            alt=''
            loading='lazy'
            decoding='async'
            onError={() => setFailed(true)}
            className={cn(
              'h-full w-full object-cover transition-opacity duration-150',
              playing && 'opacity-0'
            )}
          />
          <video
            ref={videoRef}
            src={preset.videoUrl}
            poster={preset.thumbPath}
            muted
            loop
            playsInline
            preload={playing ? 'auto' : 'metadata'}
            onError={() => setFailed(true)}
            className={cn(
              'absolute inset-0 h-full w-full object-cover transition-opacity duration-150',
              !playing && 'opacity-0'
            )}
          />
          {!playing && !isTouch ? (
            <span
              aria-hidden='true'
              className='absolute inset-0 grid place-items-center bg-black/10'
            >
              <span className='grid h-10 w-10 place-items-center rounded-full bg-black/45 backdrop-blur-sm'>
                <Play className='h-4 w-4 fill-white text-white' />
              </span>
            </span>
          ) : null}
          <span className='inf-motion-dur'>{preset.durationSec}s</span>
          <span className='inf-motion-title'>{t(preset.title)}</span>
        </div>
      </button>

      <button
        type='button'
        onClick={() => props.onApply(preset)}
        className='inf-abtn absolute right-2 bottom-2 inline-flex items-center gap-1.5 opacity-0 transition-opacity duration-150 group-focus-within/motion:opacity-100 group-hover/motion:opacity-100 max-md:opacity-100 [@media(hover:none)]:opacity-100'
      >
        <Wand2 className='h-3.5 w-3.5' aria-hidden='true' />
        <span>{t('Use')}</span>
      </button>
    </article>
  )
}
