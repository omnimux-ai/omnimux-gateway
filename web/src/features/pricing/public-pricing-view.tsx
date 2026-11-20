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

import { AlertCircle, ChevronDown, Loader2, RefreshCw } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import { resolvePricingGroup } from '@/lib/surface'
import { cn } from '@/lib/utils'

import {
  BillingHero,
  PRICING_MODELS_SECTION_ID,
} from './components/billing-hero'
import { EmptyState } from './components/empty-state'
import { LoadingSkeleton } from './components/loading-skeleton'
import { ModelPricingCard } from './components/model-pricing-card'
import { PricingDownloadCta } from './components/pricing-download-cta'
import { PricingFaq } from './components/pricing-faq'
import { PricingOfflineBanner } from './components/pricing-offline-banner'
import { PricingUseCases } from './components/pricing-use-cases'
import { PublicPricingToolbar } from './components/public-pricing-toolbar'
import { TopupLadder } from './components/topup-ladder'
import { VideoCostEstimator } from './components/video-cost-estimator'
import {
  MODALITY_FILTERS,
  TOPUP_LADDER,
  type ModalityFilter,
} from './constants'
import { usePricingData } from './hooks/use-pricing-data'
import { usePublicPricingFilters } from './hooks/use-public-pricing-filters'
import {
  PRICING_ANALYTICS_EVENTS,
  trackPricingEvent,
} from './lib/pricing-analytics'
import {
  getVideoEstimateRates,
  REFERENCE_CLIP_CREDITS,
} from './lib/video-cost-estimate'
import type { PublicModelCardItem } from './types'

export type PricingCatalogSearch = {
  search?: string
  vendor?: string
  modality?: ModalityFilter
  tag?: string
  group?: string
  quotaType?: string
  endpointType?: string
}

export type PublicPricingViewProps = {
  className?: string
  search?: PricingCatalogSearch
}

function buildModalityCounts(
  cards: PublicModelCardItem[]
): Partial<Record<ModalityFilter, number>> {
  const counts: Partial<Record<ModalityFilter, number>> = {
    [MODALITY_FILTERS.ALL]: cards.length,
    [MODALITY_FILTERS.TEXT]: 0,
    [MODALITY_FILTERS.IMAGE]: 0,
    [MODALITY_FILTERS.VIDEO]: 0,
    [MODALITY_FILTERS.AUDIO]: 0,
    [MODALITY_FILTERS.SOCIAL]: 0,
  }

  for (const card of cards) {
    const key = card.category as ModalityFilter
    if (key in counts && key !== MODALITY_FILTERS.ALL) {
      counts[key] = (counts[key] || 0) + 1
    }
  }

  return counts
}

function PricingErrorState(props: {
  message: string
  isRefreshing: boolean
  onRetry: () => void
}) {
  const { t } = useTranslation()

  return (
    <div
      role='alert'
      className='border-destructive/30 bg-destructive/5 flex min-h-[280px] flex-col items-center justify-center rounded-lg border border-dashed px-6 py-12 text-center'
    >
      <AlertCircle
        className='text-destructive mb-3 size-10'
        aria-hidden='true'
      />
      <h3 className='text-foreground mb-1 text-base font-semibold'>
        {t('Unable to load prices')}
      </h3>
      <p className='text-muted-foreground mb-5 max-w-sm text-sm'>
        {props.message}
      </p>
      <Button
        type='button'
        variant='outline'
        size='sm'
        onClick={props.onRetry}
        disabled={props.isRefreshing}
        className='gap-1.5'
      >
        {props.isRefreshing ? (
          <Loader2 className='size-3.5 animate-spin' aria-hidden='true' />
        ) : (
          <RefreshCw className='size-3.5' aria-hidden='true' />
        )}
        <span>{t('Retry')}</span>
      </Button>
    </div>
  )
}

/** Budget estimates, preset top-ups, and the live catalog. */
export function PublicPricingView(props: PublicPricingViewProps) {
  const { t } = useTranslation()
  const pageViewTracked = useRef(false)
  const cacheServedTracked = useRef(false)
  const fetchErrorTracked = useRef(false)
  const searchTracked = useRef(false)
  const search = props.search
  const [catalogOpen, setCatalogOpen] = useState(() =>
    Object.values(search ?? {}).some((value) => value !== undefined)
  )
  const selectedGroup = resolvePricingGroup()

  const {
    rawPricingData,
    cards,
    vendors,
    isLoading,
    isFetching,
    isUsingCache,
    lastUpdated,
    error,
    forceRefetch,
    dataSource,
  } = usePricingData(true, { selectedGroup })

  const requestedGroup = search?.group
  const estimateGroup =
    requestedGroup &&
    Object.hasOwn(rawPricingData?.usable_group ?? {}, requestedGroup)
      ? requestedGroup
      : selectedGroup
  const rates = useMemo(
    () => getVideoEstimateRates(rawPricingData, estimateGroup),
    [rawPricingData, estimateGroup]
  )
  const filters = usePublicPricingFilters(cards, search)
  const modalityCounts = useMemo(() => buildModalityCounts(cards), [cards])

  const hasCards = cards.length > 0
  const showHardError = Boolean(error) && !hasCards && !isLoading
  const errorMessage =
    error instanceof Error && error.message
      ? error.message
      : t('The pricing service is temporarily unavailable. Please try again.')

  useEffect(() => {
    if (pageViewTracked.current) {
      return
    }
    pageViewTracked.current = true
    trackPricingEvent(PRICING_ANALYTICS_EVENTS.PAGE_VIEW, {
      dataSource,
      modelCount: cards.length,
    })
  }, [cards.length, dataSource])

  useEffect(() => {
    if (!isUsingCache || cacheServedTracked.current) {
      return
    }
    cacheServedTracked.current = true
    trackPricingEvent(PRICING_ANALYTICS_EVENTS.SERVED_FROM_CACHE, {
      lastUpdated: lastUpdated ?? null,
    })
  }, [isUsingCache, lastUpdated])

  useEffect(() => {
    if (!showHardError || fetchErrorTracked.current) {
      return
    }
    fetchErrorTracked.current = true
    trackPricingEvent(PRICING_ANALYTICS_EVENTS.FETCH_ERROR, {
      message: errorMessage,
    })
  }, [errorMessage, showHardError])

  useEffect(() => {
    if (!filters.debouncedSearch.trim() || searchTracked.current) {
      return
    }
    searchTracked.current = true
    trackPricingEvent(PRICING_ANALYTICS_EVENTS.SEARCH_USED, {
      queryLength: filters.debouncedSearch.trim().length,
    })
  }, [filters.debouncedSearch])

  const handleRefresh = useCallback(() => {
    trackPricingEvent(PRICING_ANALYTICS_EVENTS.FORCE_REFETCH)
    void forceRefetch()
  }, [forceRefetch])

  const handleModalityChange = useCallback(
    (value: ModalityFilter) => {
      filters.setModality(value)
      trackPricingEvent(PRICING_ANALYTICS_EVENTS.MODALITY_SWITCH, {
        modality: value,
      })
    },
    [filters]
  )

  const handleVendorChange = useCallback(
    (value: string) => {
      filters.setVendorId(value)
      trackPricingEvent(PRICING_ANALYTICS_EVENTS.VENDOR_FILTER, {
        vendorId: value,
      })
    },
    [filters]
  )

  const handleCopyModelId = useCallback(
    (modelId: string) => {
      toast.success(t('Copied to clipboard'))
      trackPricingEvent(PRICING_ANALYTICS_EVENTS.MODEL_ID_COPY, { modelId })
    },
    [t]
  )

  const renderModelArea = () => {
    if (isLoading) {
      return <LoadingSkeleton />
    }

    if (showHardError) {
      return (
        <PricingErrorState
          message={errorMessage}
          isRefreshing={isFetching}
          onRetry={handleRefresh}
        />
      )
    }

    if (filters.filteredCount === 0) {
      return (
        <EmptyState
          searchQuery={filters.searchInput}
          hasActiveFilters={filters.hasActiveFilters}
          onClearFilters={filters.clearFilters}
        />
      )
    }

    return (
      <div className='flex flex-col gap-6'>
        {filters.filteredCards.map((card) => (
          <ModelPricingCard
            key={card.id}
            card={card}
            onCopyModelId={handleCopyModelId}
          />
        ))}
      </div>
    )
  }

  return (
    <main
      className={cn(
        'mx-auto max-w-7xl space-y-12 px-4 pt-24 pb-12 sm:px-6 lg:px-8',
        props.className
      )}
    >
      <BillingHero />
      <VideoCostEstimator rates={rates} />
      <TopupLadder
        ladder={TOPUP_LADDER}
        referenceClipCredits={REFERENCE_CLIP_CREDITS}
      />

      <section
        id={PRICING_MODELS_SECTION_ID}
        className='scroll-mt-24 space-y-4'
      >
        <PricingUseCases rates={rates} />
        <Collapsible
          open={catalogOpen}
          onOpenChange={(expanded) => {
            setCatalogOpen(expanded)
            trackPricingEvent(PRICING_ANALYTICS_EVENTS.CATALOG_EXPAND, {
              expanded,
              model_count: cards.length,
            })
          }}
          className='bg-card border-border rounded-2xl border'
        >
          <CollapsibleTrigger className='group flex min-h-12 w-full items-center justify-between gap-3 px-4 py-3 text-start text-sm font-medium'>
            {cards.length > 0
              ? t('View all {{count}} models and technical specs', {
                  count: cards.length,
                })
              : t('View the full model catalog')}
            <ChevronDown
              className='size-4 shrink-0 transition-transform group-aria-expanded:rotate-180'
              aria-hidden='true'
            />
          </CollapsibleTrigger>
          <CollapsibleContent className='border-border space-y-6 border-t p-4'>
            <PricingOfflineBanner
              isUsingCache={isUsingCache}
              lastUpdated={lastUpdated}
              isRefreshing={isFetching}
              onRefresh={handleRefresh}
            />
            <PublicPricingToolbar
              modality={filters.modality}
              onModalityChange={handleModalityChange}
              modalityCounts={modalityCounts}
              search={filters.searchInput}
              onSearchChange={filters.setSearchInput}
              onSearchClear={filters.clearSearch}
              vendorId={filters.vendorId}
              onVendorChange={handleVendorChange}
              vendors={vendors}
              filteredCount={filters.filteredCount}
              lastUpdated={lastUpdated}
              isRefreshing={isFetching}
              onRefresh={handleRefresh}
            />
            {renderModelArea()}
          </CollapsibleContent>
        </Collapsible>
      </section>

      <PricingFaq />
      <PricingDownloadCta />
    </main>
  )
}
