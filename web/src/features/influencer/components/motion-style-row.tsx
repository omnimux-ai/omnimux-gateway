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

import { ChevronRight, Palette } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { styleThumbnails } from '../lib/motion-style'
import type { MotionStyleChoice } from '../types'

interface Props {
  presets: MotionStyleChoice[]
  style: MotionStyleChoice | null
  onOpen: () => void
}

function StyleThumbnail(props: { src: string }) {
  const [failed, setFailed] = useState(false)
  return failed ? (
    <span className='inf-style-thumb inf-style-thumb-ph'>
      <Palette className='size-3.5' />
    </span>
  ) : (
    <img
      className='inf-style-thumb'
      src={props.src}
      alt=''
      loading='lazy'
      onError={() => setFailed(true)}
    />
  )
}

/**
 * The style entry point: one row that opens the picker, instead of a strip the
 * caller has to swipe through. The stacked thumbnails show what the pack holds,
 * the chosen style is named outright. Visual styles show a localized summary;
 * legacy templates keep their instruction underneath.
 */
export function MotionStyleRow(props: Props) {
  const { t } = useTranslation()
  const selected = props.style
  const thumbs = styleThumbnails(
    selected
      ? [
          selected,
          ...props.presets.filter((preset) => preset.id !== selected.id),
        ]
      : props.presets
  )
  const description =
    selected &&
    ('summaryKey' in selected
      ? t(selected.summaryKey)
      : (selected.description ?? selected.prompt))

  return (
    <div className='inf-style'>
      <Button
        variant='ghost'
        type='button'
        className='inf-style-row h-auto whitespace-normal'
        onClick={props.onOpen}
        aria-haspopup='dialog'
      >
        <span className='inf-style-thumbs' aria-hidden='true'>
          {thumbs.length ? (
            thumbs.map((preset) => (
              <StyleThumbnail key={preset.id} src={preset.thumbUrl} />
            ))
          ) : (
            <span className='inf-style-thumb inf-style-thumb-ph'>
              <Palette className='h-3.5 w-3.5' />
            </span>
          )}
        </span>
        <span className='inf-style-label'>
          <span className='inf-style-title'>{t('Style preset')}</span>
          {selected ? (
            <span className='inf-style-picked'>{t(selected.nameKey)}</span>
          ) : null}
        </span>
        <span className='inf-style-action'>
          {selected ? t('Change') : t('Apply style')}
          <ChevronRight className='h-3.5 w-3.5' aria-hidden='true' />
        </span>
      </Button>

      {selected ? (
        <p className='inf-style-instruction' title={description ?? undefined}>
          {description}
        </p>
      ) : null}
    </div>
  )
}
