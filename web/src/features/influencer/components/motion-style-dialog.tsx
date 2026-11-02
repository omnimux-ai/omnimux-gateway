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

import { Clapperboard, Palette } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { useMediaQuery } from '@/hooks/use-media-query'
import { cn } from '@/lib/utils'

import type { MotionStyleChoice } from '../types'

interface Props {
  open: boolean
  presets: MotionStyleChoice[]
  selectedId: string | null
  onOpenChange: (open: boolean) => void
  onApply: (preset: MotionStyleChoice | null) => void
}

interface ClipProps {
  src: string
}

/**
 * Mounted only for the style being viewed. The cover stays visible until an
 * actual playing event, including when loading or autoplay fails.
 */
function StyleClip(props: ClipProps) {
  const ref = useRef<HTMLVideoElement>(null)
  const [playing, setPlaying] = useState(false)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    const el = ref.current
    if (!el) return
    let disposed = false
    el.setAttribute('src', props.src)
    void el.play()?.catch(() => {
      if (!disposed) setFailed(true)
    })
    return () => {
      disposed = true
      el.pause()
      el.removeAttribute('src')
      el.load()
    }
  }, [props.src])

  return (
    <video
      ref={ref}
      className={cn(
        'inf-style-card-media inf-style-card-clip',
        playing && !failed && 'on'
      )}
      src={failed ? undefined : props.src}
      muted
      loop
      playsInline
      preload='none'
      aria-hidden='true'
      tabIndex={-1}
      onPlaying={() => setPlaying(true)}
      onWaiting={() => setPlaying(false)}
      onPause={() => setPlaying(false)}
      onError={() => setFailed(true)}
    />
  )
}

function StyleCover(props: { src?: string }) {
  const [failed, setFailed] = useState(false)
  return props.src && !failed ? (
    <img
      className='inf-style-card-media'
      src={props.src}
      alt=''
      loading='lazy'
      onError={() => setFailed(true)}
    />
  ) : (
    <span className='inf-style-card-media inf-style-card-ph'>
      <Palette className='size-5' aria-hidden='true' />
    </span>
  )
}

/**
 * Selection is staged until Apply. Preview media is display-only, with one
 * hovered/focused card taking precedence over the staged selection.
 */
export function MotionStyleDialog(props: Props) {
  const { t } = useTranslation()
  const [staged, setStaged] = useState<string | null>(props.selectedId)
  const [hovered, setHovered] = useState<string | null>(null)
  const [focused, setFocused] = useState<string | null>(null)
  const [tabStop, setTabStop] = useState<string | null>(props.selectedId)
  const [wasOpen, setWasOpen] = useState(props.open)
  const grid = useRef<HTMLDivElement>(null)
  const columns = useMediaQuery('(max-width: 767px)') ? 2 : 3

  // Re-seed the staged choice each time the dialog opens; a cancelled visit
  // must not leave a half-made selection behind.
  if (props.open !== wasOpen) {
    setWasOpen(props.open)
    if (props.open) {
      setStaged(props.selectedId)
      setHovered(null)
      setFocused(null)
      setTabStop(props.selectedId)
    }
  }

  const activeId = props.open ? (hovered ?? focused ?? staged) : null
  const validTabStop = props.presets.some((preset) => preset.id === tabStop)
    ? tabStop
    : null

  const apply = () => {
    const picked = props.presets.find((preset) => preset.id === staged) ?? null
    props.onApply(picked)
    props.onOpenChange(false)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Choose a style')}
      /* 弹窗经 Portal 渲染在 `.influencer-page` 之外，作用域类必须重新挂上，
         否则 `--inf-*` 令牌取不到值（该作用域只声明令牌与后代规则）。 */
      contentClassName='influencer-page inf-style-dialog sm:max-w-3xl'
      footerClassName='inf-style-footer'
      footer={
        <Button
          variant='ghost'
          type='button'
          className='inf-style-apply h-auto'
          onClick={apply}
        >
          {t('Apply style')}
        </Button>
      }
    >
      <div
        ref={grid}
        className='inf-style-grid'
        role='listbox'
        aria-label={t('Choose a style')}
        onKeyDown={(event) => {
          const options = [
            ...(grid.current?.querySelectorAll<HTMLButtonElement>(
              '[role="option"]'
            ) ?? []),
          ]
          const index = options.indexOf(event.target as HTMLButtonElement)
          if (index < 0) return
          const offsets: Record<string, number> = {
            ArrowRight: 1,
            ArrowLeft: -1,
            ArrowDown: columns,
            ArrowUp: -columns,
          }
          let next: number
          if (event.key === 'Home') next = 0
          else if (event.key === 'End') next = options.length - 1
          else if (event.key in offsets) {
            next = Math.max(
              0,
              Math.min(options.length - 1, index + offsets[event.key])
            )
          } else return
          event.preventDefault()
          options[next]?.focus()
        }}
      >
        <Button
          variant='ghost'
          type='button'
          role='option'
          aria-selected={staged === null}
          tabIndex={validTabStop === null ? 0 : -1}
          className={cn(
            'inf-style-card h-auto whitespace-normal p-0',
            staged === null && 'sel'
          )}
          onClick={() => {
            setStaged(null)
            setHovered(null)
            setFocused('')
          }}
          onPointerEnter={(event) => {
            if (event.pointerType !== 'touch') setHovered('')
          }}
          onPointerLeave={() => setHovered(null)}
          onFocus={() => {
            setTabStop(null)
            setFocused('')
            setHovered(null)
          }}
          onBlur={() => setFocused(null)}
        >
          <span className='inf-style-card-media inf-style-card-ph'>
            <Clapperboard className='size-5' aria-hidden='true' />
          </span>
          <span className='inf-style-card-name'>{t('No style')}</span>
        </Button>

        {props.presets.map((preset) => {
          const visualStyle = 'summaryKey' in preset
          const thumbUrl = visualStyle
            ? preset.previewImageUrl
            : preset.thumbUrl
          const videoUrl = visualStyle
            ? preset.previewVideoUrl
            : preset.videoUrl

          return (
            <Button
              variant='ghost'
              key={preset.id}
              type='button'
              role='option'
              aria-label={t(preset.nameKey)}
              aria-selected={staged === preset.id}
              tabIndex={validTabStop === preset.id ? 0 : -1}
              className={cn(
                'inf-style-card h-auto whitespace-normal p-0',
                staged === preset.id && 'sel'
              )}
              onClick={() => {
                setStaged(preset.id)
                setHovered(null)
                setFocused(null)
              }}
              onPointerEnter={(event) => {
                if (event.pointerType !== 'touch') setHovered(preset.id)
              }}
              onPointerLeave={() =>
                setHovered((id) => (id === preset.id ? null : id))
              }
              onFocus={() => {
                setTabStop(preset.id)
                setFocused(preset.id)
                setHovered(null)
              }}
              onBlur={() => setFocused((id) => (id === preset.id ? null : id))}
            >
              <span className='inf-style-card-art'>
                <StyleCover src={thumbUrl} />
                {videoUrl && activeId === preset.id ? (
                  <StyleClip key={videoUrl} src={videoUrl} />
                ) : null}
              </span>
              <span className='inf-style-card-name'>
                {t(preset.nameKey)}
                {visualStyle ? (
                  <span className='block font-normal'>
                    {t(preset.summaryKey)}
                  </span>
                ) : null}
              </span>
            </Button>
          )
        })}
      </div>
    </Dialog>
  )
}
