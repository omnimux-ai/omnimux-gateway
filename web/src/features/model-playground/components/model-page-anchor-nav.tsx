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

/**
 * In-page sections the model page navigates to.
 *
 * The benchmark page is a single scrolling document, not a tab panel: the four
 * entries are anchors. Section containers are supplied by the page shell.
 */
export const MODEL_PAGE_SECTIONS = [
  { id: 'playground', labelKey: 'Playground' },
  { id: 'pricing', labelKey: 'Pricing' },
  { id: 'about', labelKey: 'Readme' },
  { id: 'api', labelKey: 'API' },
] as const satisfies readonly {
  id: 'playground' | 'pricing' | 'about' | 'api'
  labelKey: string
}[]

export type ModelPageSectionId = (typeof MODEL_PAGE_SECTIONS)[number]['id']

/**
 * Section tabs.
 *
 * A recessed track carries the four entries and the current one sits on a raised
 * pill, so the bar reads as a control rather than as loose text. Both layers come
 * from theme tokens, which is what keeps the resting surface quietly darker in
 * Light and quietly lighter in Dark.
 */
export function ModelPageAnchorNav(props: { activeId: string }) {
  const { t } = useTranslation()

  return (
    <nav
      data-parity='shell.anchor-nav'
      aria-label={t('Model page sections')}
      className='bg-muted border-border inline-flex w-fit min-w-0 flex-wrap items-center gap-1 rounded-full border p-[3px]'
    >
      {MODEL_PAGE_SECTIONS.map((section) => {
        const active = props.activeId === section.id
        return (
          <a
            key={section.id}
            href={`#${section.id}`}
            aria-current={active ? 'true' : undefined}
            data-parity={active ? 'shell.anchor-active' : undefined}
            className={cn(
              'inline-flex h-7 items-center gap-1.5 rounded-full border px-3 text-[13px] font-medium transition-colors',
              active
                ? 'border-border bg-card text-foreground shadow-xs'
                : 'text-muted-foreground hover:text-foreground border-transparent'
            )}
          >
            {active && (
              <span
                aria-hidden='true'
                className='bg-primary size-1.5 rounded-full'
              />
            )}
            {t(section.labelKey)}
          </a>
        )
      })}
    </nav>
  )
}
