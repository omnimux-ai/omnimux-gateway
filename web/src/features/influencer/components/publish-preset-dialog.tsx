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

import { useEffect, useId, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

import { influencerErrorText } from '../lib/error-copy'
import { parseTaskMeta, taskImageUrl } from '../lib/history'
import type { TaskRecord } from '../types'

/** The suggested title is a teaser, not the field's limit; the gateway caps the
 * stored name at 120 characters and the input allows exactly that. */
const SUGGESTED_NAME_LENGTH = 40
const NAME_MAX_LENGTH = 120

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The finished character being published. */
  task: TaskRecord | null
  /** The multi-view board derived from it, when one exists. */
  child: TaskRecord | null
  /** Publishes under this name. The parent owns reading the artwork bytes, so a
   * failed read is reported once, from the code that actually did it. */
  onSubmit: (input: { name: string }) => Promise<void>
  submitting: boolean
}

/** The task's own direction makes the best title, shortened to a card-sized
 * teaser; a character generated from options alone falls back to a dated one. */
function suggestedName(brief: string | undefined, fallback: string): string {
  const trimmed = (brief ?? '').trim()
  if (trimmed) return trimmed.slice(0, SUGGESTED_NAME_LENGTH)
  const now = new Date()
  const month = String(now.getMonth() + 1).padStart(2, '0')
  const day = String(now.getDate()).padStart(2, '0')
  return `${fallback} ${now.getFullYear()}-${month}-${day}`
}

interface ThumbProps {
  url: string | null
  label: string
  missing: string
}

/** One artwork of the entry, captioned so the two are never confused. */
function Thumb(props: ThumbProps) {
  return (
    <figure className='flex min-w-0 flex-1 flex-col gap-1.5'>
      <div className='flex h-32 items-center justify-center overflow-hidden rounded-[var(--radius-media)] bg-[var(--inf-surface-2)]'>
        {props.url ? (
          <img
            src={props.url}
            alt=''
            loading='lazy'
            decoding='async'
            className='max-h-full max-w-full object-contain'
          />
        ) : (
          <span className='text-xs text-[var(--inf-txt-3)]'>
            {props.missing}
          </span>
        )}
      </div>
      <figcaption className='text-[11px] text-[var(--inf-txt-3)]'>
        {props.label}
      </figcaption>
    </figure>
  )
}

/**
 * The administrator's publish form: it shows the two artworks the entry will
 * carry, lets the name be edited, and hands the name back to the parent, which
 * reads the bytes and performs the upload.
 */
export function PublishPresetDialog(props: Props) {
  const { t } = useTranslation()
  const inputId = useId()
  const [name, setName] = useState('')

  const task = props.task
  const brief = useMemo(
    () => (task ? parseTaskMeta(task)?.brief : undefined),
    [task]
  )
  const defaultName = useMemo(
    () => suggestedName(brief, t('AI Creator')),
    [brief, t]
  )

  // Each opening starts from a fresh suggestion: a leftover name would publish
  // the next character under the previous one's title.
  const wasOpen = useRef(false)
  useEffect(() => {
    if (props.open && !wasOpen.current) {
      setName(defaultName)
    }
    wasOpen.current = props.open
  }, [props.open, defaultName])

  const trimmed = name.trim()
  const canSubmit = trimmed.length > 0 && !props.submitting

  // 就地定义而非 useCallback：这个回调只挂在本弹窗自己的按钮上，
  // 而 useCallback 会要求把整个 props 对象写进依赖数组。
  const handleSubmit = async (): Promise<void> => {
    if (!trimmed || props.submitting) return
    try {
      await props.onSubmit({ name: trimmed })
    } catch (e) {
      toast.error(influencerErrorText(t, e, 'Request failed'))
    }
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Publish to the inspiration library')}
      /* 弹窗经 Portal 渲染在 `.influencer-page` 之外，作用域类必须重新挂上，
         否则 `--inf-*` 令牌取不到值（该作用域只声明令牌与后代规则）。 */
      contentClassName='influencer-page sm:max-w-lg'
      footer={
        <Button
          type='button'
          disabled={!canSubmit}
          onClick={() => {
            void handleSubmit()
          }}
        >
          {t('Publish')}
        </Button>
      }
    >
      <div className='flex flex-col gap-4'>
        <div className='flex gap-3'>
          <Thumb
            url={task ? taskImageUrl(task) : null}
            label={t('Character render')}
            missing={t('Preview unavailable')}
          />
          {props.child ? (
            <Thumb
              url={taskImageUrl(props.child)}
              label={t('Multi-view board')}
              missing={t('Preview unavailable')}
            />
          ) : null}
        </div>

        <div className='flex flex-col gap-1.5'>
          <label
            htmlFor={inputId}
            className='text-[11px] font-semibold tracking-wide text-[var(--inf-txt-3)] uppercase'
          >
            {t('Name')}
          </label>
          <Input
            id={inputId}
            value={name}
            maxLength={NAME_MAX_LENGTH}
            onChange={(e) => setName(e.target.value)}
          />
        </div>
      </div>
    </Dialog>
  )
}
