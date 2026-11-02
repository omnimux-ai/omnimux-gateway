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
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { MODALITY_FILTERS, type ModalityFilter } from '../constants'

/** P0 public conversion tabs (PRD §3): VIDEO | TEXT (LLM) | IMAGE | AUDIO | ALL. */
const PUBLIC_TAB_ORDER: { id: ModalityFilter; labelKey: string }[] = [
  { id: MODALITY_FILTERS.VIDEO, labelKey: 'VIDEO' },
  { id: MODALITY_FILTERS.TEXT, labelKey: 'LLM' },
  { id: MODALITY_FILTERS.IMAGE, labelKey: 'IMAGE' },
  { id: MODALITY_FILTERS.AUDIO, labelKey: 'AUDIO' },
  { id: MODALITY_FILTERS.ALL, labelKey: 'ALL' },
]

/** Console / full catalog tabs (includes Social as P1). */
const CONSOLE_TAB_ORDER: { id: ModalityFilter; labelKey: string }[] = [
  { id: MODALITY_FILTERS.VIDEO, labelKey: 'Video' },
  { id: MODALITY_FILTERS.TEXT, labelKey: 'Text' },
  { id: MODALITY_FILTERS.IMAGE, labelKey: 'Image' },
  { id: MODALITY_FILTERS.AUDIO, labelKey: 'Audio' },
  { id: MODALITY_FILTERS.ALL, labelKey: 'All' },
  { id: MODALITY_FILTERS.SOCIAL, labelKey: 'Social data' },
]

export type ModalityTabsProps = {
  value: ModalityFilter
  onChange: (value: ModalityFilter) => void
  counts?: Partial<Record<ModalityFilter, number>>
  /** `public` = P0 ALL/IMAGE/VIDEO/LLM/AUDIO; `console` keeps Social. */
  variant?: 'public' | 'console'
  className?: string
}

export function ModalityTabs(props: ModalityTabsProps) {
  const { t } = useTranslation()
  const variant = props.variant || 'console'
  const tabOrder = variant === 'public' ? PUBLIC_TAB_ORDER : CONSOLE_TAB_ORDER

  return (
    <div
      role='tablist'
      aria-label={t('Model category')}
      className={cn(
        'flex flex-wrap items-center gap-1.5 sm:gap-2',
        variant === 'public' ? 'justify-start' : 'justify-center',
        props.className
      )}
    >
      {tabOrder.map((tab) => {
        const selected = props.value === tab.id
        const count = props.counts?.[tab.id]
        return (
          <button
            key={tab.id}
            type='button'
            role='tab'
            aria-selected={selected}
            onClick={() => props.onChange(tab.id)}
            className={cn(
              'inline-flex items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs font-medium transition-colors sm:text-sm',
              selected
                ? 'border-foreground/20 bg-foreground text-background'
                : 'border-border/80 bg-background/60 text-muted-foreground hover:border-foreground/15 hover:text-foreground'
            )}
          >
            <span>{t(tab.labelKey)}</span>
            {typeof count === 'number' && (
              <span
                className={cn(
                  'tabular-nums opacity-70',
                  selected ? 'opacity-80' : ''
                )}
              >
                {count}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}
