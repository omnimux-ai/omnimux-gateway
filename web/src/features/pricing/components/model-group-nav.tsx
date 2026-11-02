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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { getAvailableGroups } from '../lib/model-helpers'
import type { PricingModel } from '../types'

export interface ModelGroupNavProps {
  model: PricingModel
  usableGroup: Record<string, string>
  groupRatio?: Record<string, number>
  selectedGroup: string
  onSelectGroup: (group: string) => void
  className?: string
}

/**
 * Model version and tier selector.
 *
 * Renders as a clean, native underline tab bar adhering to the modern SaaS
 * minimal standard: full raw group identifier names, zero redundant badges/labels,
 * with active brand purple underline highlight and bottom border rule.
 *
 * Automatically hidden for single-group models to keep the layout minimal.
 */
export function ModelGroupNav(props: ModelGroupNavProps) {
  const { t } = useTranslation()

  const availableGroups = useMemo(
    () => getAvailableGroups(props.model, props.usableGroup || {}),
    [props.model, props.usableGroup]
  )

  // Single-group models hide this selector completely to preserve zero redundancy
  if (availableGroups.length <= 1) {
    return null
  }

  return (
    <div
      data-parity='shell.model-group-nav'
      className={cn('w-full border-b border-border/60', props.className)}
    >
      <nav
        aria-label={t('Model versions')}
        className='no-scrollbar flex items-center gap-6 overflow-x-auto sm:gap-8'
      >
        {availableGroups.map((group) => {
          const active = props.selectedGroup === group

          return (
            <button
              key={group}
              type='button'
              onClick={() => props.onSelectGroup(group)}
              aria-pressed={active}
              className={cn(
                'relative py-2.5 text-sm font-medium transition-colors whitespace-nowrap outline-none',
                active
                  ? 'text-primary'
                  : 'text-muted-foreground hover:text-foreground'
              )}
            >
              <span>{group}</span>
              {active && (
                <span
                  className='bg-primary absolute inset-x-0 -bottom-px h-0.5 rounded-full'
                  aria-hidden='true'
                />
              )}
            </button>
          )
        })}
      </nav>
    </div>
  )
}
