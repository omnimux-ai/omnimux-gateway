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

import { AlignLeft, ChevronDown, ChevronUp, Settings2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Button } from '@/components/ui/button'

export interface PreviewDetailsProps {
  brief?: string
  tags?: string[]
  briefLabel?: string
}

export function PreviewDetails(props: PreviewDetailsProps) {
  const { t } = useTranslation()
  const [directionExpanded, setDirectionExpanded] = useState(false)
  const [paramsExpanded, setParamsExpanded] = useState(false)
  const tagsKey = JSON.stringify(props.tags ?? [])
  useEffect(() => {
    setDirectionExpanded(false)
    setParamsExpanded(false)
  }, [props.brief, tagsKey])
  const tags = [...new Set(props.tags ?? [])]
  const visibleTags = paramsExpanded ? tags : tags.slice(0, 9)
  const toggleClass =
    'w-full justify-center gap-1 text-sm font-medium text-white/60 hover:bg-white/5 hover:text-white'
  return (
    <>
      {props.brief ? (
        <section className='flex shrink-0 flex-col rounded-2xl border border-white/5 px-3 pb-1'>
          <header className='flex items-center gap-2 py-3'>
            <AlignLeft
              className='size-4 shrink-0 text-white/60'
              aria-hidden='true'
            />
            <span className='min-w-0 flex-1 text-xs font-medium tracking-wide text-white/60 uppercase'>
              {props.briefLabel ?? t('Direction')}
            </span>
            <CopyButton
              key={props.brief}
              value={props.brief}
              size='sm'
              variant='outline'
              aria-label={t('Copy')}
              className='gap-1 border-white/10 bg-transparent text-xs text-white hover:bg-white/5 hover:text-white'
              iconClassName='size-3'
            >
              {t('Copy')}
            </CopyButton>
          </header>
          <p
            className='no-scrollbar overflow-y-auto text-xs leading-relaxed whitespace-pre-wrap text-white/80'
            style={directionExpanded ? undefined : { maxHeight: 160 }}
          >
            {props.brief}
          </p>
          <Button
            type='button'
            variant='ghost'
            aria-expanded={directionExpanded}
            onClick={() => setDirectionExpanded((prev) => !prev)}
            className={toggleClass}
          >
            <span>{directionExpanded ? t('Show less') : t('See all')}</span>
            {directionExpanded ? (
              <ChevronUp className='size-4' aria-hidden='true' />
            ) : (
              <ChevronDown className='size-4' aria-hidden='true' />
            )}
          </Button>
        </section>
      ) : null}
      {tags.length ? (
        <section className='flex shrink-0 flex-col rounded-2xl border border-white/5 px-3 pb-1'>
          <header className='flex items-center gap-2 py-3'>
            <Settings2
              className='size-4 shrink-0 text-white/60'
              aria-hidden='true'
            />
            <span className='min-w-0 flex-1 text-xs font-medium tracking-wide text-white/60 uppercase'>
              {t('Character parameters')}
            </span>
          </header>
          <ul className='flex flex-wrap gap-1'>
            {visibleTags.map((tag) => (
              <li
                key={tag}
                className='rounded-lg bg-white/10 px-2 py-1 text-xs text-white'
              >
                {tag}
              </li>
            ))}
          </ul>
          {tags.length > 9 ? (
            <Button
              type='button'
              variant='ghost'
              aria-expanded={paramsExpanded}
              onClick={() => setParamsExpanded((prev) => !prev)}
              className={toggleClass}
            >
              <span>{paramsExpanded ? t('Show less') : t('See all')}</span>
              {paramsExpanded ? (
                <ChevronUp className='size-4' aria-hidden='true' />
              ) : (
                <ChevronDown className='size-4' aria-hidden='true' />
              )}
            </Button>
          ) : null}
        </section>
      ) : null}
    </>
  )
}
