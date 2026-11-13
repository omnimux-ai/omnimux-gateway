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
import { ChevronDown, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

import { visibleFacetValues } from '../lib/catalog'
import { formatCount } from '../lib/format'

export interface FilterOption {
  value: string
  label: string
}

/**
 * The options a capped dropdown lists: the first `limit` in the array's own
 * order, plus the selected value when the cap would otherwise hide it. The
 * rule itself lives in `visibleFacetValues`, so the menu and the L2 chip row
 * cannot drift apart.
 */
function cappedOptions(
  options: FilterOption[],
  limit: number,
  selected: string | null
): FilterOption[] {
  const kept = new Set(
    visibleFacetValues(
      options.map((option) => option.value),
      limit,
      selected
    )
  )
  return options.filter((option) => kept.has(option.value))
}

export interface FilterDropdownProps {
  /** Accessible + visible name of the dimension, e.g. `Region`. */
  label: string
  options: FilterOption[]
  selected: string[]
  onChange: (values: string[]) => void
  /** Set false for single-choice dropdowns (the Sort control). */
  multiple?: boolean
  /**
   * Option count to show before the list offers the rest, for a dimension
   * whose vocabulary is crawled free text rather than a closed enum. Omit for
   * a short list that needs no cap.
   */
  limit?: number
  /**
   * Leading entries of `options` that are not facet values and stay visible
   * regardless of `limit` — the `All` sentinel of a facet dimension. They are
   * outside both the cap and the count the expand toggle prints, so
   * `Show all 61 categories` means 61 categories and not 61 options.
   */
  fixedLeadingOptions?: number
  className?: string
}

/**
 * Multi-select dropdown with a selected-count badge and an inline clear
 * action. Built on the shared `DropdownMenu` primitives so keyboard
 * navigation and focus management come from the design system rather than a
 * bespoke popover.
 *
 * With `limit`, only the first facet options in the array's own order are
 * listed (the manifest emits them by descending coverage) plus an in-place
 * toggle for the remainder, so a 100-value vocabulary stays a readable menu.
 * The currently selected value is always listed, even when the cap would hide
 * it, so the visitor can always see and clear the filter in effect.
 */
export function FilterDropdown(props: FilterDropdownProps) {
  const { t } = useTranslation()
  const multiple = props.multiple !== false
  const selectedCount = props.selected.length
  const [expanded, setExpanded] = useState(false)

  const fixed = props.fixedLeadingOptions ?? 0
  const leadingOptions = props.options.slice(0, fixed)
  const facetOptions = props.options.slice(fixed)
  const limit = props.limit
  const capShows = limit !== undefined && facetOptions.length > limit
  const capped = capShows && !expanded

  // These capped dimensions are single-choice, so the one selected value is
  // the one that must survive the cap.
  const visibleOptions = capped
    ? [
        ...leadingOptions,
        ...cappedOptions(facetOptions, limit, props.selected[0] ?? null),
      ]
    : props.options

  const handleToggle = (value: string, checked: boolean) => {
    if (!multiple) {
      props.onChange(checked ? [value] : [])
      return
    }
    if (checked) {
      props.onChange([...props.selected, value])
      return
    }
    props.onChange(props.selected.filter((entry) => entry !== value))
  }

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            variant='outline'
            size='lg'
            className={cn('gap-1.5 rounded-full px-3', props.className)}
            aria-label={props.label}
          />
        }
      >
        <span className='text-sm font-medium'>{props.label}</span>
        {selectedCount > 0 && (
          <Badge
            variant='secondary'
            className='h-4 min-w-4 rounded-full px-1 text-[10px] font-semibold'
          >
            {selectedCount}
          </Badge>
        )}
        <ChevronDown className='size-3.5 opacity-60' aria-hidden='true' />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align='start'
        className='max-h-80 w-56 overflow-y-auto'
      >
        <DropdownMenuGroup>
          <DropdownMenuLabel>{props.label}</DropdownMenuLabel>
          {visibleOptions.map((option) => (
            <DropdownMenuCheckboxItem
              key={option.value}
              checked={props.selected.includes(option.value)}
              onCheckedChange={(checked) =>
                handleToggle(option.value, checked === true)
              }
            >
              {option.label}
            </DropdownMenuCheckboxItem>
          ))}
        </DropdownMenuGroup>
        {capShows && (
          // `closeOnClick={false}` keeps the menu open so the rest of the list
          // appears in place instead of the visitor having to reopen it.
          <DropdownMenuItem
            closeOnClick={false}
            onClick={() => setExpanded((prev) => !prev)}
          >
            {expanded
              ? t('Show fewer')
              : t('Show all {{count}} categories', {
                  count: formatCount(facetOptions.length),
                })}
          </DropdownMenuItem>
        )}
        {selectedCount > 0 && (
          <>
            <DropdownMenuSeparator />
            <DropdownMenuItem onClick={() => props.onChange([])}>
              {t('Clear')}
            </DropdownMenuItem>
          </>
        )}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

export interface FilterToggleGroupProps<T extends string> {
  label: string
  value: T
  options: Array<{ value: T; label: string }>
  onChange: (value: T) => void
  className?: string
}

/** Segmented single-choice control, e.g. the media-type switch. */
export function FilterToggleGroup<T extends string>(
  props: FilterToggleGroupProps<T>
) {
  return (
    <ToggleGroup
      value={[props.value]}
      onValueChange={(values) => {
        const next = values.find((entry) => entry !== props.value)
        if (next) props.onChange(next as T)
      }}
      variant='outline'
      size='sm'
      aria-label={props.label}
      className={props.className}
    >
      {props.options.map((option) => (
        <ToggleGroupItem
          key={option.value}
          value={option.value}
          className='h-9 px-3 text-sm'
        >
          {option.label}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}

export interface ActiveFilterChip {
  /** Stable key for React lists; also used as the removal identity. */
  key: string
  label: string
  onRemove: () => void
}

export interface FilterChipsProps {
  chips: ActiveFilterChip[]
  onClearAll: () => void
  className?: string
}

/** Removable summary of the currently applied filters. */
export function FilterChips(props: FilterChipsProps) {
  const { t } = useTranslation()
  if (props.chips.length === 0) return null

  return (
    <div
      className={cn('flex flex-wrap items-center gap-2', props.className)}
      aria-live='polite'
    >
      {props.chips.map((chip) => (
        <span
          key={chip.key}
          className='bg-muted/60 text-foreground inline-flex items-center gap-1 rounded-full border py-1 pr-1 pl-2.5 text-xs'
        >
          {chip.label}
          <Button
            variant='ghost'
            size='icon-sm'
            className='size-5 rounded-full'
            aria-label={t('Remove')}
            onClick={chip.onRemove}
          >
            <X className='size-3' aria-hidden='true' />
          </Button>
        </span>
      ))}
      <Button
        variant='ghost'
        size='sm'
        className='h-7 rounded-full px-2 text-xs'
        onClick={props.onClearAll}
      >
        {t('Clear all')}
      </Button>
    </div>
  )
}
