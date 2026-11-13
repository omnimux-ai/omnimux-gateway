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
import { Maximize2, Package, Plus, Shirt, UserRound, X } from 'lucide-react'
import {
  useId,
  useRef,
  useState,
  type CSSProperties,
  type RefObject,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import './character-assets.css'

import { cn } from '@/lib/utils'

import type { MotionCharacterSource } from '../types'
import { FadeInImage } from './fade-in-image'

interface Props {
  slotRef?: RefObject<HTMLDivElement | null>
  label: string
  hint: string
  /** One-line tooltip for guidance that must not take panel space. */
  hintTitle?: string
  characters: MotionCharacterSource[]
  busy?: boolean
  /** Reference ceiling for this surface; the workbench itself allows 30. */
  max?: number
  onOpenPicker: () => void
  onRemoveCharacter: (index: number) => void
  /** Opens this image in the shared preview window. */
  onPreviewImage?: (url: string) => void
  /** Every file from one drop selection, in picker order. */
  onPickDirectFiles: (files: File[]) => void
}

/** Stable upload frame with an independently scrolling character gallery. */
export function CharacterSlot(props: Props) {
  const hintId = useId()
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement>(null)
  const [dragging, setDragging] = useState(false)
  const [activeUrl, setActiveUrl] = useState<string | null>(null)
  const touchPointer = useRef(false)
  const characters = props.characters ?? []
  const hasCharacters = characters.length > 0
  const max = props.max ?? 30
  const isFull = characters.length >= max
  const columns = Math.min(
    characters.length + (isFull ? 0 : 1),
    characters.length <= 3 ? 3 : 4
  )

  return (
    <div
      ref={props.slotRef}
      className={cn('inf-slot inf-character-slot', dragging && 'drag')}
      data-populated={hasCharacters || undefined}
      onPointerDown={(event) => {
        touchPointer.current =
          event.pointerType === 'touch' || event.pointerType === 'pen'
        if (!(event.target as HTMLElement).closest('.inf-character-tile')) {
          setActiveUrl(null)
        }
      }}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
          setActiveUrl(null)
        }
      }}
      onKeyDown={(event) => {
        if (event.key === 'Escape') setActiveUrl(null)
      }}
      onDragOver={(event) => {
        event.preventDefault()
        if (!props.busy && !isFull) setDragging(true)
      }}
      onDragLeave={() => setDragging(false)}
      onDrop={(event) => {
        event.preventDefault()
        setDragging(false)
        if (props.busy || isFull) return
        const files = [...(event.dataTransfer.files ?? [])].filter((file) =>
          file.type.startsWith('image/')
        )
        if (files.length > 0) props.onPickDirectFiles(files)
      }}
    >
      <input
        ref={inputRef}
        type='file'
        accept='image/png,image/jpeg,image/webp'
        multiple
        className='hidden'
        aria-hidden='true'
        tabIndex={-1}
        disabled={props.busy || isFull}
        onChange={(event) => {
          const files = [...(event.target.files ?? [])]
          if (files.length > 0 && !props.busy && !isFull) {
            props.onPickDirectFiles(files)
          }
          event.target.value = ''
        }}
      />
      {props.hintTitle ? (
        <span id={hintId} className='sr-only'>
          {props.hintTitle}
        </span>
      ) : null}
      {!hasCharacters ? (
        <Button
          variant='ghost'
          className='inf-slot-empty'
          aria-label={props.label}
          aria-describedby={props.hintTitle ? hintId : undefined}
          title={props.hintTitle}
          disabled={props.busy}
          onClick={props.onOpenPicker}
        >
          <span className='inf-slot-image-icons' aria-hidden='true'>
            <span className='inf-slot-video-icon'>
              <UserRound className='size-5' />
            </span>
            <span className='inf-slot-video-icon'>
              <Package className='size-5' />
            </span>
            <span className='inf-slot-video-icon'>
              <Shirt className='size-5' />
            </span>
          </span>
          <span className='inf-slot-title'>{props.label}</span>
          {props.hint ? (
            <span className='inf-slot-hint'>{props.hint}</span>
          ) : null}
        </Button>
      ) : (
        <>
          <span className='sr-only'>
            {props.label}: {t('Selected')} <strong>{characters.length}</strong>{' '}
            / {max}
          </span>
          <div
            className='inf-character-scroll'
            tabIndex={0}
            aria-label={props.label}
          >
            <div
              className='inf-character-grid'
              style={{ '--character-columns': columns } as CSSProperties}
            >
              {!isFull && (
                <Button
                  variant='ghost'
                  className='inf-character-add'
                  title={
                    props.hintTitle ??
                    (max === 30
                      ? t('Add reference image (up to 30)')
                      : t('Add reference image'))
                  }
                  aria-label={t('Add reference image')}
                  aria-describedby={props.hintTitle ? hintId : undefined}
                  disabled={props.busy}
                  onClick={props.onOpenPicker}
                >
                  <span className='inf-character-plus'>
                    <Plus className='size-3.5' aria-hidden='true' />
                  </span>
                </Button>
              )}
              {characters.map((item, index) => (
                <div
                  key={item.url}
                  className='inf-character-tile'
                  data-active={activeUrl === item.url || undefined}
                >
                  <Button
                    variant='ghost'
                    className='inf-character-image'
                    aria-label={`${t('Preview')}: ${item.fileName || `${props.label} ${index + 1}`}`}
                    title={item.fileName || `${props.label} ${index + 1}`}
                    disabled={props.busy}
                    onClick={(event) => {
                      if (
                        touchPointer.current &&
                        event.detail !== 0 &&
                        !(event.target as Element).closest(
                          '.inf-character-enlarge'
                        )
                      ) {
                        setActiveUrl(item.url)
                      } else {
                        setActiveUrl(null)
                        props.onPreviewImage?.(item.url)
                      }
                    }}
                  >
                    <FadeInImage
                      src={item.previewUrl || item.url}
                      alt=''
                      className='h-full w-full object-cover'
                    />
                    <Maximize2
                      className='inf-character-enlarge size-4'
                      aria-hidden='true'
                    />
                  </Button>
                  <Button
                    variant='ghost'
                    className='inf-character-remove'
                    title={t('Remove this image')}
                    aria-label={t('Remove')}
                    disabled={props.busy}
                    onClick={() => {
                      setActiveUrl(null)
                      props.onRemoveCharacter(index)
                    }}
                  >
                    <X className='size-3' aria-hidden='true' />
                  </Button>
                </div>
              ))}
            </div>
          </div>
        </>
      )}
    </div>
  )
}
