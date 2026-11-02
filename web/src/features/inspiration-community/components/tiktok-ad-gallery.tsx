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
import { useNavigate } from '@tanstack/react-router'
import { type ReactNode, useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { toIntlLocale } from '@/i18n/languages'
import { getServerErrorMessage } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

import {
  CONTENT_TYPE_LABEL_KEYS,
  DURATION_LABEL_KEYS,
  PUBLISHED_LABEL_KEYS,
  TRAFFIC_LABEL_KEYS,
  VIEWS_LABEL_KEYS,
  regionLabelKey,
} from '../constants'
import { useTikTokAds } from '../hooks/use-tiktok-ads'
import { formatCount } from '../lib/format'
import type { TikTokAdItem, TikTokFilters } from '../types'
import { AdDetailModal } from './ad-detail-modal'
import { FilterChips, type ActiveFilterChip } from './filter-bar'
import { InspirationShell } from './inspiration-shell'
import { GalleryPagination } from './pagination'
import { TikTokAdCard } from './tiktok-ad-card'
import { TikTokAdFilterBar } from './tiktok-ad-filter-bar'

const GRID_CLASS =
  'grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4 2xl:grid-cols-5'

/**
 * Page B — TikTok ad library (PRD section 4).
 *
 * Every figure on this page comes from the inspiration service: the grid is one
 * page of `GET /api/inspiration/v1/public/inspirations`, and the result count
 * and the page count are that response's `total` and `page_count`. Nothing
 * here is computed from a bundled array, and nothing is printed that the
 * service did not send.
 */
export function TikTokAdGallery() {
  const { t, i18n } = useTranslation()
  // Number grouping is locale copy: `10,924` in en, `10 924` in fr/ru.
  const numberLocale = toIntlLocale(i18n.language)
  const ads = useTikTokAds()
  const { filters, sort, page } = ads.state
  // The detail layer walks the list it was opened from — the page the service
  // just returned — so browsing stops at the end of the loaded rows instead of
  // inventing neighbours nobody has fetched.
  const [detail, setDetail] = useState<{
    items: TikTokAdItem[]
    index: number
  } | null>(null)

  const openDetail = useCallback(
    (item: TikTokAdItem) => {
      const index = ads.items.findIndex((row) => row.id === item.id)
      setDetail({ items: ads.items, index: index < 0 ? 0 : index })
    },
    [ads.items]
  )

  const hasActiveFilters = useMemo(
    () => Object.values(filters).some((value) => value !== null),
    [filters]
  )

  let navigate: ReturnType<typeof useNavigate> | null = null
  try {
    navigate = useNavigate()
  } catch {
    navigate = null
  }

  const handleRecreate = useCallback(
    (item: TikTokAdItem) => {
      const promptText = item.title || item.caption || ''
      if (navigate) {
        try {
          navigate({
            to: '/pricing/$modelId',
            params: { modelId: 'seedance-2-5' },
            search: { prompt: promptText },
          })
          return
        } catch {
          // Fall back
        }
      }
      if (typeof window !== 'undefined') {
        window.location.assign(
          `/pricing/seedance-2-5?prompt=${encodeURIComponent(promptText)}`
        )
      }
    },
    [navigate]
  )

  const chips = useMemo<ActiveFilterChip[]>(() => {
    const entries: ActiveFilterChip[] = []
    const push = (
      dimension: keyof TikTokFilters,
      value: string,
      label: string
    ) => {
      entries.push({
        key: `${dimension}:${value}`,
        label,
        onRemove: () => ads.setFilters({ ...filters, [dimension]: null }),
      })
    }

    if (filters.country) {
      push(
        'country',
        filters.country,
        `${t('Region')}: ${t(regionLabelKey(filters.country))}`
      )
    }
    if (filters.contentType) {
      push(
        'contentType',
        filters.contentType,
        `${t('Content type')}: ${t(CONTENT_TYPE_LABEL_KEYS[filters.contentType])}`
      )
    }
    if (filters.duration) {
      push(
        'duration',
        filters.duration,
        `${t('Clip length')}: ${t(DURATION_LABEL_KEYS[filters.duration])}`
      )
    }
    if (filters.views) {
      push(
        'views',
        filters.views,
        `${t('Views')}: ${t(VIEWS_LABEL_KEYS[filters.views])}`
      )
    }
    if (filters.trafficType) {
      push(
        'trafficType',
        filters.trafficType,
        `${t('Traffic')}: ${t(TRAFFIC_LABEL_KEYS[filters.trafficType])}`
      )
    }
    if (filters.published) {
      push(
        'published',
        filters.published,
        `${t('Published')}: ${t(PUBLISHED_LABEL_KEYS[filters.published])}`
      )
    }
    return entries
  }, [filters, t, ads])

  const clearFilters = useCallback(() => {
    ads.reset()
  }, [ads])

  let body: ReactNode
  if (ads.isLoading) {
    body = <TikTokAdGallerySkeleton />
  } else if (ads.error) {
    body = (
      <ErrorState
        className='mt-10'
        description={getServerErrorMessage(ads.error)}
        onRetry={ads.refetch}
      />
    )
  } else if (ads.items.length === 0 && hasActiveFilters) {
    body = (
      <EmptyState
        className='mt-10'
        bordered
        title={t('No ads match these filters')}
        description={t('Loosen a filter to see more creatives.')}
        action={
          <Button variant='ghost' size='sm' onClick={clearFilters}>
            {t('Clear all')}
          </Button>
        }
      />
    )
  } else if (ads.items.length === 0) {
    body = (
      <EmptyState
        className='mt-10'
        bordered
        title={t('No creatives in the ad library yet')}
        description={t(
          'New creatives appear here as soon as they are collected.'
        )}
      />
    )
  } else {
    body = (
      <>
        <div
          className={cn(
            `${GRID_CLASS} pt-4`,
            ads.isFetching && 'opacity-60 transition-opacity'
          )}
          aria-busy={ads.isFetching}
        >
          {ads.items.map((item) => (
            <TikTokAdCard key={item.id} item={item} onOpen={openDetail} />
          ))}
        </div>

        {ads.pageCount === null ? null : (
          <GalleryPagination
            className='mt-8'
            page={page}
            totalPages={ads.pageCount}
            showJumpToPage
            onPageChange={ads.setPage}
          />
        )}
      </>
    )
  }

  return (
    <InspirationShell active='tiktok' title={t('TikTok Ad Library')}>
      <div className='mt-8 flex flex-col gap-3'>
        <TikTokAdFilterBar
          filters={filters}
          sort={sort}
          onFiltersChange={ads.setFilters}
          onSortChange={ads.setSort}
        />

        <FilterChips chips={chips} onClearAll={clearFilters} />

        <div className='text-muted-foreground flex flex-wrap items-center gap-2 text-sm'>
          <span aria-live='polite'>
            {ads.total === null
              ? null
              : t('{{count}} creatives', {
                  count: formatCount(ads.total, numberLocale),
                })}
          </span>
        </div>
      </div>

      {body}

      <AdDetailModal
        items={detail?.items ?? []}
        index={detail?.index ?? -1}
        onIndexChange={(index) =>
          setDetail((previous) =>
            previous ? { ...previous, index } : previous
          )
        }
        onClose={() => setDetail(null)}
        onRecreate={handleRecreate}
      />
    </InspirationShell>
  )
}

/** Skeleton shown while the ad library's data is not ready yet. */
export function TikTokAdGallerySkeleton() {
  return (
    <div
      className={`${GRID_CLASS} pt-4`}
      aria-hidden='true'
      data-testid='ad-skeleton'
    >
      {Array.from({ length: 10 }, (_, index) => (
        <div key={index} className='overflow-hidden rounded-2xl border'>
          <Skeleton className='aspect-[3/4] w-full rounded-none' />
          <div className='flex gap-1 p-1.5'>
            <Skeleton className='h-8 flex-1' />
            <Skeleton className='h-8 flex-1' />
            <Skeleton className='h-8 flex-1' />
          </div>
        </div>
      ))}
    </div>
  )
}
