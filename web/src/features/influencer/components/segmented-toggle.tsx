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

import type { ReactNode } from 'react'

import { cn } from '@/lib/utils'

export interface SegmentedOption<T extends string> {
  value: T
  /** Visible text; may be a compact form when the track is narrow. */
  label: string
  /** Full name for the accessible label and hover title; defaults to `label`. */
  ariaLabel?: string
  icon?: ReactNode
}

interface Props<T extends string> {
  value: T
  onChange: (value: T) => void
  options: SegmentedOption<T>[]
  /** Accessible name for the group; each radio carries its own label. */
  ariaLabel: string
  className?: string
  /** Stretch to the container with equal-width options. The workbench's
   * surface tabs use it; the small inline switchers keep content-sized pills. */
  block?: boolean
  /** Tab hierarchy variant: 'primary' (large rounded rect) or 'pill' (capsule). */
  variant?: 'primary' | 'pill'
  /** Icon-only mode: omits textual labels from visible rendering while keeping accessibility labels. */
  iconOnly?: boolean
}

/**
 * The workbench's segmented switcher: supports primary large rounded rects
 * and secondary pills, with pure icon-only compact display options. All
 * variants use a borderless surface container per design specifications.
 */
export function SegmentedToggle<T extends string>(props: Props<T>) {
  const isPill = props.variant === 'pill'

  return (
    <div
      role='radiogroup'
      aria-label={props.ariaLabel}
      className={cn(
        'items-center bg-[var(--inf-surface-2)] p-0.5',
        isPill ? 'rounded-full' : 'rounded-[10px]',
        props.block ? 'flex w-full' : 'inline-flex',
        props.className
      )}
    >
      {props.options.map((option) => {
        const active = props.value === option.value
        return (
          <button
            key={option.value}
            type='button'
            role='radio'
            aria-checked={active}
            aria-label={option.ariaLabel ?? option.label}
            title={option.ariaLabel ?? option.label}
            onClick={() => props.onChange(option.value)}
            className={cn(
              'inline-flex items-center transition-all duration-150',
              isPill
                ? cn(
                    'rounded-full',
                    props.iconOnly
                      ? 'h-7 w-7 justify-center p-0 text-xs'
                      : 'gap-1.5 px-3 py-1 text-xs font-medium'
                  )
                : cn(
                    'rounded-[8px]',
                    props.iconOnly
                      ? 'h-8 w-8 justify-center p-0 text-xs'
                      : 'gap-1.5 px-3 py-1.5 text-xs font-medium'
                  ),
              props.block && 'flex-1 justify-center',
              active
                ? 'bg-[var(--inf-surface)] font-semibold text-[var(--inf-txt)] shadow-sm'
                : 'text-[var(--inf-txt-3)] hover:text-[var(--inf-txt-2)]'
            )}
          >
            {option.icon}
            {!props.iconOnly && <span>{option.label}</span>}
          </button>
        )
      })}
    </div>
  )
}
