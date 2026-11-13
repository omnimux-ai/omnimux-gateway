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
import { ArrowDown01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { OPERATION_LABEL_KEYS } from '../lib/operation-matrix'
import type { PlaygroundOperation } from '../types'

/** Two-line rate block, pre-formatted by the caller. */
export type ModePriceLine = {
  /** e.g. "From ~$0.3143 / request" */
  amount: string
  /** e.g. "3.143 credits"; omitted when the catalog cannot convert. */
  credits?: string | null
}

export type PlaygroundModeSelectorProps = {
  operations: PlaygroundOperation[]
  value: PlaygroundOperation
  /** Starting rate for a mode; a mode without one drops the rate block. */
  priceFor?: (operation: PlaygroundOperation) => ModePriceLine | null
  disabled?: boolean
  onChange: (operation: PlaygroundOperation) => void
}

/**
 * Mode picker for models that declare more than one input mode.
 *
 * Reproduces the benchmark page's control: a bordered trigger carrying the
 * current mode over its starting rate, opening onto a list where every mode
 * carries its own rate. Compact proportions — a 14px semibold title over a 12px
 * rate, 12px padding, and the 16px card radius from DESIGN.md §5.3 — keep the
 * picker from dominating the form column. The rate here is a starting price, not
 * the estimate for the configured request; the footer owns that number.
 */
export function PlaygroundModeSelector(props: PlaygroundModeSelectorProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const listRef = useRef<HTMLDivElement | null>(null)
  const interactive = props.operations.length > 1
  const currentPrice = props.priceFor?.(props.value) ?? null

  // Move focus onto the current option so keyboard users land where the mouse
  // would.
  useEffect(() => {
    if (!open) return
    const selected = listRef.current?.querySelector<HTMLButtonElement>(
      '[role="option"][aria-selected="true"]'
    )
    const first = listRef.current?.querySelector<HTMLButtonElement>('[role="option"]')
    ;(selected ?? first)?.focus()
  }, [open])

  const onListKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    const keys = ['ArrowDown', 'ArrowUp', 'Home', 'End']
    if (!keys.includes(event.key)) return
    const options = [
      ...(listRef.current?.querySelectorAll<HTMLButtonElement>('[role="option"]') ?? []),
    ]
    if (options.length === 0) return
    event.preventDefault()
    const current = options.indexOf(document.activeElement as HTMLButtonElement)
    let next = current
    if (event.key === 'ArrowDown') next = (current + 1) % options.length
    if (event.key === 'ArrowUp') next = (current - 1 + options.length) % options.length
    if (event.key === 'Home') next = 0
    if (event.key === 'End') next = options.length - 1
    options[next < 0 ? 0 : next]?.focus()
  }

  const title = (operation: PlaygroundOperation, price: ModePriceLine | null) => (
    <>
      <span
        aria-hidden='true'
        className='bg-foreground size-[7px] shrink-0 rounded-full'
      />
      <span className='min-w-0 flex-1 pl-2'>
        <span className='block text-sm font-semibold'>
          {t(OPERATION_LABEL_KEYS[operation])}
        </span>
        {price ? (
          <span className='text-muted-foreground block text-xs tabular-nums'>
            {price.credits ? `${price.amount} · ${price.credits}` : price.amount}
          </span>
        ) : null}
      </span>
    </>
  )

  return (
    <div
      data-parity='playground.route-bar'
      className='border-border bg-background overflow-hidden rounded-(--radius-card) border'
    >
      {interactive ? (
        <button
          type='button'
          data-parity='playground.mode.select'
          aria-expanded={open}
          aria-haspopup='listbox'
          aria-label={t('Generation mode')}
          disabled={props.disabled}
          className='bg-muted/40 hover:bg-muted/70 flex w-full items-center gap-3 px-4 py-3 text-left transition-colors disabled:opacity-60'
          onClick={() => setOpen((previous) => !previous)}
        >
          {title(props.value, currentPrice)}
          <HugeiconsIcon
            icon={ArrowDown01Icon}
            className={cn(
              'text-muted-foreground size-4 shrink-0 transition-transform',
              open && 'rotate-180'
            )}
            aria-hidden='true'
          />
        </button>
      ) : (
        <div className='bg-muted/40 flex items-center gap-3 px-4 py-3'>
          {title(props.value, currentPrice)}
        </div>
      )}

      {open && (
        <div
          ref={listRef}
          role='listbox'
          aria-label={t('Generation mode')}
          className='border-t'
          onKeyDown={onListKeyDown}
        >
          {props.operations.map((operation) => {
            const selected = operation === props.value
            const price = props.priceFor?.(operation) ?? null
            return (
              <button
                key={operation}
                type='button'
                role='option'
                aria-selected={selected}
                className={cn(
                  'flex w-full items-center justify-between gap-4 border-l-2 px-4 py-3 text-left transition-colors',
                  selected
                    ? 'border-l-foreground bg-muted/55'
                    : 'hover:bg-muted/45 border-l-transparent'
                )}
                onClick={() => {
                  setOpen(false)
                  if (!selected) props.onChange(operation)
                }}
              >
                <span className='min-w-0 flex-1 text-sm font-semibold'>
                  {t(OPERATION_LABEL_KEYS[operation])}
                </span>
                {price ? (
                  <span className='shrink-0 text-right'>
                    <span className='block text-xs font-medium tabular-nums'>
                      {price.amount}
                    </span>
                    {price.credits ? (
                      <span className='text-muted-foreground block text-xs tabular-nums'>
                        {price.credits}
                      </span>
                    ) : null}
                  </span>
                ) : null}
              </button>
            )
          })}
        </div>
      )}
    </div>
  )
}
