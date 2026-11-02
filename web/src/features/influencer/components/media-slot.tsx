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
  Clapperboard,
  ImageIcon,
  Maximize2,
  Play,
  Video,
  X,
} from 'lucide-react'
import {
  useEffect,
  useId,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface Props {
  slotRef?: RefObject<HTMLDivElement | null>
  kind: 'image' | 'video'
  variant?: 'reference'
  /** Return false to redirect a guest before opening a file picker. */
  onBeforePick?: () => boolean
  onOpenPicker?: () => void
  /** Opening the task panel may remain available while file edits are locked. */
  pickerBusy?: boolean
  /** Accessible name for the slot, and the empty state's title. */
  label: string
  /** One line of guidance under the empty state's title. */
  hint: string
  /** Filled state caption for catalogue clips; uploads never pass a file name. */
  caption?: string
  accept: string
  /** Filled state. Absent means the slot is empty. */
  previewUrl?: string
  /** Overrides the badge; a template knows its own length without a file. */
  badge?: string
  busy?: boolean
  uploading?: boolean
  /** When present, a selected video opens in the shared playback window. */
  onPreviewVideo?: (url: string) => void
  /** When present, a selected image opens in the same shared preview window. */
  onPreviewImage?: (url: string) => void
  onPick: (file: File) => void
  onClear: () => void
}

/**
 * One upload target of the motion form.
 *
 * Empty and filled are separate layouts rather than one control with two
 * states: the empty state is a button, and the filled state puts a full-bleed
 * replace button *next to* the remove button instead of inside it, so neither
 * ends up nested in the other. Clicking or dropping a file always replaces.
 */
export function MediaSlot(props: Props) {
  const { t } = useTranslation()
  const helpId = useId()
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const restoreFocus = useRef(false)
  const [failedUrl, setFailedUrl] = useState('')
  const [clipSeconds, setClipSeconds] = useState<number | null>(null)
  const videoRef = useRef<HTMLVideoElement>(null)
  const [blockedUrl, setBlockedUrl] = useState<string | null>(null)

  useEffect(() => {
    const video = videoRef.current
    if (!video || !props.previewUrl || props.kind !== 'video') return
    let cancelled = false
    const url = props.previewUrl
    void video.play().catch(() => {
      if (!cancelled) setBlockedUrl(url)
    })
    return () => {
      cancelled = true
      video.pause()
    }
  }, [props.previewUrl, props.kind])

  // A new source is a new clip; the old duration must not linger on the badge.
  useEffect(() => {
    setClipSeconds(null)
  }, [props.previewUrl])

  useEffect(() => {
    if (!props.previewUrl && restoreFocus.current) {
      triggerRef.current?.focus()
      restoreFocus.current = false
    }
  }, [props.previewUrl])

  const openBusy = props.onOpenPicker
    ? (props.pickerBusy ?? props.busy)
    : props.busy
  const open = () => {
    if (openBusy || props.onBeforePick?.() === false) return
    if (props.onOpenPicker) props.onOpenPicker()
    else inputRef.current?.click()
  }

  const takeFiles = (files: FileList | null) => {
    if (props.busy || !files?.length) return
    if (props.onBeforePick?.() === false) return
    if (
      (props.variant === 'reference' || props.kind === 'video') &&
      files.length !== 1
    ) {
      if (props.kind === 'video') {
        toast.error(t('Choose one video only'))
        return
      }
      toast.error(t('Choose one image only'))
      return
    }
    setFailedUrl('')
    props.onPick(files[0])
  }

  // A chosen template has no file to show yet, but the slot is no longer
  // empty — falling back to the empty layout would read as "nothing picked".
  const filled = Boolean(props.previewUrl || props.caption || props.badge)
  const badge =
    props.badge ??
    (clipSeconds !== null ? `${Math.round(clipSeconds)}s` : undefined)

  // Three chosen shapes: a template with no file yet, a clip, or a still.
  let media: ReactNode
  if (!props.previewUrl) {
    media = (
      <span className='inf-slot-media inf-slot-placeholder'>
        <Clapperboard className='h-6 w-6' aria-hidden='true' />
      </span>
    )
  } else if (props.kind === 'video') {
    media = (
      <video
        key={props.previewUrl}
        ref={videoRef}
        className='inf-slot-media'
        src={props.previewUrl}
        muted
        autoPlay
        loop
        playsInline
        onPlaying={() => setBlockedUrl(null)}
        preload='metadata'
        onLoadedMetadata={(event) =>
          setClipSeconds(event.currentTarget.duration)
        }
      />
    )
  } else {
    media = <img className='inf-slot-media' src={props.previewUrl} alt='' />
  }

  const clear = () => {
    if (props.busy) return
    restoreFocus.current = true
    setFailedUrl('')
    props.onClear()
  }
  const previewFailed = Boolean(
    props.previewUrl && failedUrl === props.previewUrl
  )
  const error = previewFailed
    ? t('Reference image is unavailable. Replace or remove it.')
    : ''

  return (
    <div
      ref={props.slotRef}
      className={cn(
        'inf-slot',
        props.variant === 'reference' && 'inf-reference-slot',
        filled && 'filled',
        dragging && !props.busy && 'drag'
      )}
      aria-busy={props.busy || undefined}
      onDragOver={(event) => {
        event.preventDefault()
        if (!props.busy) setDragging(true)
      }}
      onDragLeave={() => setDragging(false)}
      onDrop={(event) => {
        event.preventDefault()
        setDragging(false)
        takeFiles(event.dataTransfer.files)
      }}
    >
      <input
        ref={inputRef}
        type='file'
        disabled={props.busy}
        accept={props.accept}
        className='hidden'
        aria-hidden='true'
        tabIndex={-1}
        onChange={(event) => {
          takeFiles(event.target.files)
          // Reset so re-picking the same file still fires a change.
          event.target.value = ''
        }}
      />

      {props.variant === 'reference' ? (
        <div className='inf-reference-inner'>
          <div className='inf-reference-target'>
            {props.previewUrl && !previewFailed ? (
              <img
                className='inf-reference-thumbnail'
                src={props.previewUrl}
                alt=''
                onError={() => setFailedUrl(props.previewUrl ?? '')}
              />
            ) : (
              <span className='inf-reference-icon'>
                <ImageIcon className='size-4' aria-hidden='true' />
              </span>
            )}
            <Button
              ref={triggerRef}
              type='button'
              variant='ghost'
              className='inf-reference-pick'
              aria-describedby={error || props.hint ? helpId : undefined}
              disabled={props.busy}
              aria-label={
                filled
                  ? t('Replace {{label}}', { label: props.label })
                  : props.label
              }
              onClick={open}
            >
              <span className='inf-reference-copy'>
                {!filled ? (
                  <span className='inf-slot-title'>
                    {t('Upload a reference image')}
                  </span>
                ) : null}
                {props.uploading ? (
                  <span role='status'>{t('Uploading image…')}</span>
                ) : null}
              </span>
            </Button>
          </div>
          <span className='inf-reference-optional'>{t('Optional')}</span>
          {filled ? (
            <Button
              type='button'
              variant='ghost'
              className='inf-reference-remove'
              aria-label={t('Remove {{label}}', { label: props.label })}
              disabled={props.busy}
              onClick={clear}
            >
              <X className='size-4' aria-hidden='true' />
            </Button>
          ) : null}
        </div>
      ) : null}
      {props.variant !== 'reference' &&
        (filled ? (
          <>
            {media}

            {badge ? <span className='inf-slot-badge'>{badge}</span> : null}

            <button
              type='button'
              className='inf-slot-replace'
              aria-label={t('Replace {{label}}', { label: props.label })}
              onClick={open}
              disabled={openBusy}
            />

            <button
              type='button'
              className='inf-slot-remove'
              aria-label={t('Remove {{label}}', { label: props.label })}
              disabled={props.busy}
              onClick={clear}
            >
              <X className='h-3.5 w-3.5' aria-hidden='true' />
            </button>

            {props.caption ? (
              <span className='inf-slot-caption'>{props.caption}</span>
            ) : null}
          </>
        ) : (
          <button
            type='button'
            ref={triggerRef}
            className='inf-slot-empty'
            aria-label={props.label}
            disabled={openBusy}
            onClick={open}
          >
            {props.kind === 'video' ? (
              <span className='inf-slot-video-icon'>
                <Video className='h-5 w-5' aria-hidden='true' />
              </span>
            ) : (
              <ImageIcon className='h-5 w-5' aria-hidden='true' />
            )}
            <span className='inf-slot-title'>{props.label}</span>
            {props.hint ? (
              <span className='inf-slot-hint'>{props.hint}</span>
            ) : null}
          </button>
        ))}
      {props.previewUrl &&
      !previewFailed &&
      (props.kind === 'video' ? props.onPreviewVideo : props.onPreviewImage) ? (
        <Button
          type='button'
          variant='ghost'
          size='icon'
          className='inf-slot-preview'
          aria-label={`${t('Preview')}: ${props.label}`}
          onClick={() => {
            const url = props.previewUrl
            if (!url) return
            if (props.kind === 'video') {
              props.onPreviewVideo?.(url)
              return
            }
            props.onPreviewImage?.(url)
          }}
        >
          <Maximize2 className='size-6' aria-hidden='true' />
        </Button>
      ) : null}
      {props.kind === 'video' &&
      props.previewUrl &&
      blockedUrl === props.previewUrl ? (
        <Button
          type='button'
          variant='ghost'
          size='icon'
          className='inf-slot-play'
          aria-label={t('Play video')}
          onClick={() => {
            void videoRef.current
              ?.play()
              .catch(() => setBlockedUrl(props.previewUrl ?? null))
          }}
        >
          <Play className='size-6' aria-hidden='true' />
        </Button>
      ) : null}
      {props.variant !== 'reference' && error ? (
        <p role='alert' className='text-destructive text-xs'>
          {error}
        </p>
      ) : null}
      {props.variant === 'reference' && (error || props.hint) ? (
        <span
          id={helpId}
          className='inf-reference-help'
          role={error ? 'alert' : undefined}
          tabIndex={error ? 0 : undefined}
        >
          {error || props.hint}
        </span>
      ) : null}
    </div>
  )
}
