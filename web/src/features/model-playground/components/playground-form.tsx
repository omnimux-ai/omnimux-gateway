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
import { Cancel01Icon, MoreHorizontalIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useLayoutEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

import type {
  ParamControl,
  PlaygroundParamSchema,
  PlaygroundParams,
} from '../types'

export type PlaygroundFormProps = {
  schema: PlaygroundParamSchema
  params: PlaygroundParams
  uploading: boolean
  disabled: boolean
  replaceSingleFirstFrame: boolean
  onParamChange: (key: string, value: string | number | boolean) => void
  onMediaFiles: (
    key: string,
    files: FileList | null,
    replaceIndex?: number
  ) => void
  onMediaRemove: (key: string, index: number) => void
  onMediaSwap: (key: string, index?: number) => void
}

/** localStorage-free description of a filled slot, for the accessible name. */
function slotName(url: string): string {
  const trimmed = url.split('?').at(0) ?? url
  return trimmed.split('/').at(-1) || url
}

function values(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string')
}

/** data-parity id for one control, matching the parity checklist ids. */
function controlParityId(key: string): string | undefined {
  const map: Record<string, string> = {
    prompt: 'playground.form.prompt',
    // The flat form's single media slot and the reference mode's first slot are
    // the same control, so they keep the benchmark's id.
    reference_media: 'playground.form.media',
    reference_images: 'playground.form.media',
    reference_videos: 'playground.form.reference-videos',
    reference_audios: 'playground.form.reference-audio',
    reference_image: 'playground.form.media',
    reference_audio: 'playground.form.reference-audio',
    first_frame: 'playground.form.first-frame',
    last_frame: 'playground.form.last-frame',
    source_videos: 'playground.form.source-video',
    file: 'playground.form.file',
    url: 'playground.form.url',
    size_mode: 'playground.form.size-mode',
    aspect_ratio: 'playground.form.ratio',
    resolution: 'playground.form.resolution',
    quality: 'playground.form.quality',
    background: 'playground.form.background',
    output_format: 'playground.form.output-format',
    duration: 'playground.form.duration',
    generate_audio: 'playground.form.audio',
  }
  return map[key]
}

function optionLabel(option: { value: string; labelKey?: string }): string {
  return option.labelKey ?? option.value
}

function ChipsControl(props: {
  control: ParamControl
  value: string
  disabled: boolean
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()
  return (
    <ToggleGroup
      value={props.value ? [props.value] : []}
      onValueChange={(next) => {
        const picked = next.at(-1)
        if (picked !== undefined) props.onChange(picked)
      }}
      variant='outline'
      size='sm'
      disabled={props.disabled}
      aria-label={t(props.control.labelKey)}
      className='flex-wrap'
    >
      {(props.control.options ?? []).map((option) => (
        <ToggleGroupItem key={option.value} value={option.value}>
          {t(optionLabel(option))}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}

/**
 * A control the mode pins to one value.
 *
 * Rendered as a statement rather than a disabled input: the mode follows the
 * input media's geometry, so there is nothing here to choose, and a greyed-out
 * picker would suggest otherwise.
 */
function LockedControl(props: { control: ParamControl; parityId?: string }) {
  const { t } = useTranslation()
  return (
    <div data-parity={props.parityId} className='space-y-1.5'>
      <div className='flex items-center gap-2'>
        <Label>{t(props.control.labelKey)}</Label>
        <span
          data-parity='playground.form.locked'
          className='bg-muted text-muted-foreground rounded-full px-2 py-0.5 text-[12px]'
        >
          {t(optionLabel(props.control.options?.[0] ?? { value: '' }))}
        </span>
      </div>
      {props.control.lockedReasonKey && (
        <p className='text-muted-foreground text-[12px]'>
          {t(props.control.lockedReasonKey)}
        </p>
      )}
    </div>
  )
}

function MediaControl(props: {
  control: ParamControl
  urls: string[]
  disabled: boolean
  uploading: boolean
  replaceSingleFirstFrame: boolean
  parityId?: string
  onMediaFiles: PlaygroundFormProps['onMediaFiles']
  onMediaRemove: PlaygroundFormProps['onMediaRemove']
  onMediaSwap: PlaygroundFormProps['onMediaSwap']
}) {
  const { t } = useTranslation()
  const { control } = props
  const maxCount = control.maxCount
  const isFull = maxCount !== undefined && props.urls.length >= maxCount
  const orderedImages = control.firstItemRole === 'first_frame'
  const expandedOrder = orderedImages && (maxCount ?? 0) > 2
  const imageActionsAvailable = orderedImages || props.replaceSingleFirstFrame
  const disabled = props.disabled || props.uploading
  const replaceInput = useRef<HTMLInputElement>(null)
  const replaceIndex = useRef<number | undefined>(undefined)
  const replacement = useRef<{ index: number; previousUrl: string } | null>(
    null
  )
  const imageActions = useRef(new Map<number, HTMLButtonElement>())
  const urlOccurrences = new Map<string, number>()
  const counter =
    maxCount === undefined
      ? `${props.urls.length}`
      : `${props.urls.length} / ${maxCount}`

  useLayoutEffect(() => {
    if (disabled || !replacement.current) return
    const { index, previousUrl } = replacement.current
    replacement.current = null
    const trigger = imageActions.current.get(index)
    if (!trigger || props.urls[index] === previousUrl) return
    const document = trigger.ownerDocument
    if (document.activeElement === document.body) trigger.focus()
  }, [disabled, props.urls])

  return (
    <div
      data-parity={props.parityId}
      className='space-y-2'
      onDragOver={orderedImages ? (event) => event.preventDefault() : undefined}
      onDrop={
        orderedImages
          ? (event) => {
              event.preventDefault()
              if (!disabled) {
                props.onMediaFiles(control.key, event.dataTransfer.files)
              }
            }
          : undefined
      }
    >
      <div className='flex items-center gap-2'>
        <Label
          htmlFor={
            imageActionsAvailable && !isFull
              ? `media-${control.key}`
              : undefined
          }
        >
          {orderedImages ? t('Reference images') : t(control.labelKey)}
        </Label>
        {imageActionsAvailable && (
          <div className='ml-auto flex items-center gap-2'>
            <span className='text-muted-foreground text-xs tabular-nums'>
              {counter}
            </span>
            {orderedImages && props.urls.length === 2 && (
              <Button
                type='button'
                variant='outline'
                size='sm'
                className='h-7 rounded-full text-xs'
                disabled={disabled}
                onClick={() => props.onMediaSwap(control.key)}
              >
                {t('Swap order')}
              </Button>
            )}
          </div>
        )}
        {control.hintKey && (
          <span className='text-muted-foreground text-[12px]'>
            {t(control.hintKey, { count: control.maxCount ?? 1 })}
          </span>
        )}
      </div>

      {!isFull && (
        <label className='border-muted-foreground/30 hover:bg-muted/40 flex cursor-pointer flex-col items-center gap-1 rounded-xl border border-dashed px-4 py-6 text-center transition-colors'>
          <span className='text-sm'>
            {t('Click to upload or drag files here')}
          </span>
          {control.accept?.includes('image/') && (
            <span className='text-muted-foreground text-[12px]'>
              {t('JPG / PNG / WebP')}
            </span>
          )}
          {(props.uploading || !imageActionsAvailable) && (
            <span className='text-muted-foreground text-[12px] tabular-nums'>
              {props.uploading ? t('Uploading…') : counter}
            </span>
          )}
          <input
            type='file'
            id={imageActionsAvailable ? `media-${control.key}` : undefined}
            multiple={maxCount !== 1}
            accept={control.accept}
            className='sr-only'
            disabled={disabled}
            onChange={(event) => {
              props.onMediaFiles(control.key, event.currentTarget.files)
              event.currentTarget.value = ''
            }}
          />
        </label>
      )}

      {imageActionsAvailable && (
        <input
          type='file'
          hidden
          ref={replaceInput}
          accept={control.accept}
          aria-label={t('Replace reference')}
          disabled={disabled}
          onChange={(event) => {
            const index = replaceIndex.current
            const previousUrl =
              index === undefined ? undefined : props.urls[index]
            if (
              index !== undefined &&
              previousUrl &&
              event.currentTarget.files?.length
            ) {
              replacement.current = { index, previousUrl }
              props.onMediaFiles(control.key, event.currentTarget.files, index)
            }
            event.currentTarget.value = ''
            replaceIndex.current = undefined
          }}
        />
      )}

      {props.urls.length > 0 && (
        <ol
          className={cn(
            'gap-2',
            imageActionsAvailable ? 'grid grid-cols-2' : 'flex flex-wrap'
          )}
        >
          {props.urls.map((url, index) => {
            const occurrence = (urlOccurrences.get(url) ?? 0) + 1
            urlOccurrences.set(url, occurrence)
            return (
              <li
                key={`${url}:${occurrence}`}
                className={cn(
                  'group relative',
                  imageActionsAvailable
                    ? 'min-w-0 overflow-hidden rounded-lg border'
                    : 'size-12'
                )}
              >
                {control.accept?.includes('image/') ? (
                  <img
                    src={url}
                    alt={slotName(url)}
                    className={cn(
                      'bg-muted object-cover',
                      imageActionsAvailable
                        ? 'h-16 w-full'
                        : 'size-12 rounded-lg'
                    )}
                  />
                ) : (
                  <span className='bg-muted flex size-12 items-center justify-center rounded-lg px-1 text-center text-[10px] break-all'>
                    {slotName(url)}
                  </span>
                )}
                {imageActionsAvailable && (
                  <div className='bg-muted/40 flex items-center gap-1.5 px-2 py-1 text-xs'>
                    <strong className='shrink-0'>
                      {(expandedOrder || props.replaceSingleFirstFrame) &&
                      index === 0
                        ? t('Starting image')
                        : t('Image {{index}}', { index: index + 1 })}
                    </strong>
                    <span
                      className='text-muted-foreground min-w-0 truncate'
                      title={slotName(url)}
                    >
                      {slotName(url)}
                    </span>
                  </div>
                )}
                {imageActionsAvailable && (
                  <DropdownMenu>
                    <DropdownMenuTrigger
                      render={
                        <Button
                          type='button'
                          variant='outline'
                          size='icon'
                          className='bg-background absolute top-1 right-8 size-6 rounded-full'
                          aria-label={`${t('Image {{index}}', { index: index + 1 })} · ${t('Actions')}`}
                          disabled={disabled}
                          ref={(button) => {
                            if (button) imageActions.current.set(index, button)
                            else imageActions.current.delete(index)
                          }}
                        />
                      }
                    >
                      <HugeiconsIcon
                        icon={MoreHorizontalIcon}
                        aria-hidden='true'
                      />
                    </DropdownMenuTrigger>
                    <DropdownMenuContent align='end'>
                      <DropdownMenuGroup>
                        {expandedOrder && index > 0 && (
                          <DropdownMenuItem
                            onSelect={() =>
                              props.onMediaSwap(control.key, index)
                            }
                          >
                            {t('Use as starting image')}
                          </DropdownMenuItem>
                        )}
                        <DropdownMenuItem
                          onSelect={() => {
                            replaceIndex.current = index
                            replaceInput.current?.click()
                          }}
                        >
                          {t('Replace reference')}
                        </DropdownMenuItem>
                      </DropdownMenuGroup>
                    </DropdownMenuContent>
                  </DropdownMenu>
                )}
                <Button
                  type='button'
                  variant='outline'
                  size='icon'
                  aria-label={
                    orderedImages
                      ? t('Remove image {{index}}', { index: index + 1 })
                      : t('Remove {{name}}', { name: slotName(url) })
                  }
                  disabled={disabled}
                  className={cn(
                    'bg-background absolute size-6 rounded-full',
                    imageActionsAvailable
                      ? 'top-1 right-1'
                      : '-top-2 -right-2 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100'
                  )}
                  onClick={() => props.onMediaRemove(control.key, index)}
                >
                  <HugeiconsIcon
                    icon={Cancel01Icon}
                    className='size-3.5'
                    aria-hidden='true'
                  />
                </Button>
              </li>
            )
          })}
        </ol>
      )}
      {orderedImages && !expandedOrder && (
        <p className='text-muted-foreground text-xs'>
          {t(
            'Image 1 is the default start. Use [图1] and [图2] in the prompt.'
          )}
        </p>
      )}
    </div>
  )
}

function Control(props: {
  control: ParamControl
  params: PlaygroundParams
  disabled: boolean
  uploading: boolean
  replaceSingleFirstFrame: boolean
  onParamChange: PlaygroundFormProps['onParamChange']
  onMediaFiles: PlaygroundFormProps['onMediaFiles']
  onMediaRemove: PlaygroundFormProps['onMediaRemove']
  onMediaSwap: PlaygroundFormProps['onMediaSwap']
}) {
  const { t } = useTranslation()
  const { control } = props
  const raw = props.params[control.key]
  const parityId = controlParityId(control.key)

  const label = (
    <div className='flex items-center gap-2'>
      <Label>{t(control.labelKey)}</Label>
      {control.hintKey && (
        <span className='text-muted-foreground text-[12px]'>
          {t(control.hintKey)}
        </span>
      )}
    </div>
  )

  if (control.kind === 'text') {
    const text = typeof raw === 'string' ? raw : ''
    // The model's declared ceiling, so an over-long prompt is impossible to
    // compose rather than refused after submission.
    const maxChars = control.maxChars
    return (
      <div data-parity={parityId} className='space-y-2'>
        {label}
        <Textarea
          value={text}
          disabled={props.disabled}
          maxLength={maxChars}
          rows={8}
          className='min-h-[200px] rounded-[12px]'
          placeholder={t(
            control.placeholderKey ?? 'Describe the result you want'
          )}
          onChange={(event) =>
            props.onParamChange(control.key, event.target.value)
          }
        />
        <p className='text-muted-foreground text-right text-[12px] tabular-nums'>
          {maxChars === undefined
            ? text.length
            : `${text.length} / ${maxChars}`}
        </p>
      </div>
    )
  }

  if (control.kind === 'media') {
    return (
      <MediaControl
        control={control}
        urls={values(raw)}
        disabled={props.disabled}
        uploading={props.uploading}
        replaceSingleFirstFrame={props.replaceSingleFirstFrame}
        parityId={parityId}
        onMediaFiles={props.onMediaFiles}
        onMediaRemove={props.onMediaRemove}
        onMediaSwap={props.onMediaSwap}
      />
    )
  }

  if (control.lockedReasonKey) {
    return <LockedControl control={control} parityId={parityId} />
  }

  if (control.kind === 'switch') {
    return (
      <div data-parity={parityId} className='flex items-center gap-3'>
        <Switch
          checked={raw === true}
          disabled={props.disabled}
          aria-label={t(control.labelKey)}
          onCheckedChange={(checked) =>
            props.onParamChange(control.key, checked)
          }
        />
        {label}
      </div>
    )
  }

  if (control.kind === 'slider') {
    const value = typeof raw === 'number' ? raw : Number(raw) || 0
    return (
      <div data-parity={parityId} className='space-y-2'>
        {label}
        <div className='flex items-center gap-3'>
          <Slider
            value={[value]}
            min={control.min}
            max={control.max}
            step={control.step}
            disabled={props.disabled}
            aria-label={t(control.labelKey)}
            onValueChange={(next) => {
              const picked = Array.isArray(next) ? next.at(0) : next
              if (typeof picked === 'number') {
                props.onParamChange(control.key, picked)
              }
            }}
          />
          <span className='w-16 text-right text-[13px] tabular-nums'>
            {value}
          </span>
        </div>
      </div>
    )
  }

  return (
    <div data-parity={parityId} className='space-y-2'>
      {label}
      <ChipsControl
        control={control}
        value={typeof raw === 'string' ? raw : ''}
        disabled={props.disabled}
        onChange={(value) => props.onParamChange(control.key, value)}
      />
    </div>
  )
}

export function PlaygroundForm(props: PlaygroundFormProps) {
  const { t } = useTranslation()
  const primary = props.schema.controls.filter(
    (control) => control.advanced !== true
  )
  const advanced = props.schema.controls.filter(
    (control) => control.advanced === true
  )

  const renderControl = (control: ParamControl) => (
    <Control
      key={control.key}
      control={control}
      params={props.params}
      disabled={props.disabled}
      uploading={props.uploading}
      replaceSingleFirstFrame={
        props.replaceSingleFirstFrame && control.mediaRole === 'first_frame'
      }
      onParamChange={props.onParamChange}
      onMediaFiles={props.onMediaFiles}
      onMediaRemove={props.onMediaRemove}
      onMediaSwap={props.onMediaSwap}
    />
  )

  return (
    <div className='space-y-5'>
      {primary.map(renderControl)}

      {advanced.length > 0 && (
        <Collapsible data-parity='playground.form.advanced'>
          <CollapsibleTrigger
            render={
              <Button
                variant='outline'
                className='h-9 w-full rounded-xl text-[13px]'
              />
            }
          >
            {t('Advanced settings')} ({advanced.length})
          </CollapsibleTrigger>
          <CollapsibleContent className='space-y-5 pt-5'>
            {advanced.map(renderControl)}
          </CollapsibleContent>
        </Collapsible>
      )}
    </div>
  )
}
