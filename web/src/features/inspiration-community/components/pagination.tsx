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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Pagination,
  PaginationContent,
  PaginationEllipsis,
  PaginationItem,
} from '@/components/ui/pagination'
import { cn } from '@/lib/utils'

import { buildPageItems } from '../lib/filter'

/**
 * Stable React key for a pagination entry. Page numbers are unique; the two
 * possible ellipses are disambiguated by which end of the control they sit on
 * (index 1 is always the leading one), so no raw array index leaks into a key.
 */
function pageItemKey(entry: number | 'ellipsis', index: number): string {
  if (entry !== 'ellipsis') return `page-${entry}`
  return index === 1 ? 'ellipsis-leading' : 'ellipsis-trailing'
}

export interface GalleryPaginationProps {
  page: number
  totalPages: number
  onPageChange: (page: number) => void
  /** Enables the numeric jump field under the control. */
  showJumpToPage?: boolean
  className?: string
}

/**
 * Numeric pagination with ellipsis collapse and an optional jump-to-page
 * field, mirroring the reference ad library's `1 2 3 … 10924` control.
 */
export function GalleryPagination(props: GalleryPaginationProps) {
  const { t } = useTranslation()
  const [draft, setDraft] = useState('')
  const items = buildPageItems(props.page, props.totalPages)

  const goTo = (page: number) => {
    const next = Math.min(Math.max(1, page), Math.max(1, props.totalPages))
    if (next !== props.page) props.onPageChange(next)
  }

  const submitJump = () => {
    const parsed = Number.parseInt(draft, 10)
    if (Number.isNaN(parsed)) return
    goTo(parsed)
    setDraft('')
  }

  if (props.totalPages <= 1) return null

  return (
    <div
      className={cn(
        'flex flex-row flex-wrap items-center justify-center gap-x-4 gap-y-2',
        props.className,
      )}
    >
      <Pagination className='w-auto'>
        <PaginationContent>
          <PaginationItem>
            <Button
              variant='ghost'
              size='lg'
              className='px-2.5'
              aria-label={t('Previous page')}
              disabled={props.page <= 1}
              onClick={() => goTo(props.page - 1)}
            >
              ‹
            </Button>
          </PaginationItem>

          {items.map((entry, index) => (
            <PaginationItem key={pageItemKey(entry, index)}>
              {entry === 'ellipsis' ? (
                <PaginationEllipsis />
              ) : (
                <Button
                  variant={entry === props.page ? 'outline' : 'ghost'}
                  size='icon-lg'
                  aria-label={`${entry}`}
                  aria-current={entry === props.page ? 'page' : undefined}
                  className={cn(entry === props.page && 'font-semibold')}
                  onClick={() => goTo(entry)}
                >
                  {entry}
                </Button>
              )}
            </PaginationItem>
          ))}

          <PaginationItem>
            <Button
              variant='ghost'
              size='lg'
              className='px-2.5'
              aria-label={t('Next page')}
              disabled={props.page >= props.totalPages}
              onClick={() => goTo(props.page + 1)}
            >
              ›
            </Button>
          </PaginationItem>
        </PaginationContent>
      </Pagination>

      {props.showJumpToPage && (
        <form
          className='flex items-center gap-2'
          onSubmit={(event) => {
            event.preventDefault()
            submitJump()
          }}
        >
          <label
            htmlFor='inspiration-page-jump'
            className='text-muted-foreground text-xs'
          >
            {t('Go to page')}
          </label>
          <Input
            id='inspiration-page-jump'
            inputMode='numeric'
            className='h-8 w-20 text-center text-xs'
            aria-label={t('Page number')}
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
          />
          <Button type='submit' variant='outline' size='sm' className='h-8'>
            {t('Go')}
          </Button>
        </form>
      )}
    </div>
  )
}
