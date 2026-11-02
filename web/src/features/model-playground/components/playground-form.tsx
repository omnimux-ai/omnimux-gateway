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
import { Cancel01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { Label } from '@/components/ui/label'
import { Slider } from '@/components/ui/slider'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

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
  onParamChange: (key: string, value: string | number | boolean) => void
  onMediaFiles: (key: string, files: FileList | null) => void
  onMediaRemove: (key: string, url: string) => void
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
    first_frame: 'playground.form.first-frame',
    last_frame: 'playground.form.last-frame',
    source_videos: 'playground.form.source-video',
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
  parityId?: string
  onMediaFiles: PlaygroundFormProps['onMediaFiles']
  onMediaRemove: PlaygroundFormProps['onMediaRemove']
}) {
  const { t } = useTranslation()
  const { control } = props
  const maxCount = control.maxCount
  const isFull = maxCount !== undefined && props.urls.length >= maxCount
  const counter =
    maxCount === undefined
      ? `${props.urls.length}`
      : `${props.urls.length} / ${maxCount}`

  return (
    <div data-parity={props.parityId} className='space-y-2'>
      <div className='flex items-center gap-2'>
        <Label>{t(control.labelKey)}</Label>
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
          <span className='text-muted-foreground text-[12px]'>
            {t('JPG / PNG / WebP')}
          </span>
          <span className='text-muted-foreground text-[12px] tabular-nums'>
            {props.uploading ? t('Uploading…') : counter}
          </span>
          <input
            type='file'
            multiple={maxCount !== 1}
            accept={control.accept}
            className='sr-only'
            disabled={props.disabled || props.uploading}
            onChange={(event) =>
              props.onMediaFiles(control.key, event.target.files)
            }
          />
        </label>
      )}

      {props.urls.length > 0 && (
        <ul className='flex flex-wrap gap-2'>
          {props.urls.map((url) => (
            <li key={url} className='group relative size-12'>
              <img
                src={url}
                alt={slotName(url)}
                className='bg-muted size-12 rounded-lg object-cover'
              />
              <Button
                type='button'
                variant='outline'
                size='icon'
                aria-label={t('Remove {{name}}', { name: slotName(url) })}
                disabled={props.disabled}
                className='bg-background absolute -top-2 -right-2 size-6 rounded-full opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100'
                onClick={() => props.onMediaRemove(control.key, url)}
              >
                <HugeiconsIcon
                  icon={Cancel01Icon}
                  className='size-3.5'
                  aria-hidden='true'
                />
              </Button>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function Control(props: {
  control: ParamControl
  params: PlaygroundParams
  disabled: boolean
  uploading: boolean
  onParamChange: PlaygroundFormProps['onParamChange']
  onMediaFiles: PlaygroundFormProps['onMediaFiles']
  onMediaRemove: PlaygroundFormProps['onMediaRemove']
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
          placeholder={t('Describe the result you want')}
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
        parityId={parityId}
        onMediaFiles={props.onMediaFiles}
        onMediaRemove={props.onMediaRemove}
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
      onParamChange={props.onParamChange}
      onMediaFiles={props.onMediaFiles}
      onMediaRemove={props.onMediaRemove}
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
