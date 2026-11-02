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

import { AlertCircle, Loader2, RefreshCw } from 'lucide-react'
import { useCallback, useEffect, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { resolvePricingGroup } from '@/lib/surface'
import { cn } from '@/lib/utils'

import {
  BillingHero,
  PRICING_MODELS_SECTION_ID,
} from './components/billing-hero'
import { BillingMethodSteps } from './components/billing-method-steps'
import { EmptyState } from './components/empty-state'
import { LoadingSkeleton } from './components/loading-skeleton'
import { ModelPricingCard } from './components/model-pricing-card'
import { PricingOfflineBanner } from './components/pricing-offline-banner'
import { PublicPricingToolbar } from './components/public-pricing-toolbar'
import { TopupLadder } from './components/topup-ladder'
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
import type { PublicModelCardItem } from './types'

export type PublicPricingViewProps = {
  className?: string
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

/**
 * Public conversion-oriented pricing page (P0 default for /pricing).
 * Assembles Hero → billing steps → toolbar → model cards from live ACL data.
 */
export function PublicPricingView(props: PublicPricingViewProps) {
  const { t } = useTranslation()
  const pageViewTracked = useRef(false)
  const cacheServedTracked = useRef(false)
  const fetchErrorTracked = useRef(false)
  const searchTracked = useRef(false)

  const {
    cards,
    vendors,
    isLoading,
    isFetching,
    isUsingCache,
    lastUpdated,
    error,
    forceRefetch,
    dataSource,
  } = usePricingData(true, { selectedGroup: resolvePricingGroup() })

  const filters = usePublicPricingFilters(cards)
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

  const handleScrollToModels = useCallback(() => {
    trackPricingEvent(PRICING_ANALYTICS_EVENTS.CTA_VIEW_MODELS)
    const target = document.querySelector(
      `#${CSS.escape(PRICING_MODELS_SECTION_ID)}`
    )
    if (target) {
      target.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }, [])

  const handleConsultEnterprise = useCallback(() => {
    trackPricingEvent(PRICING_ANALYTICS_EVENTS.CTA_ENTERPRISE)
    toast.info(t('Opening enterprise pricing contact… sales@omnimux.ai'))
    window.location.assign('mailto:sales@omnimux.ai')
  }, [t])

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
    <div
      className={cn(
        'mx-auto max-w-7xl space-y-12 px-4 py-8 sm:px-6 lg:px-8',
        props.className
      )}
    >
      <BillingHero
        modelCount={cards.length}
        onScrollToModels={handleScrollToModels}
        onConsultEnterprise={handleConsultEnterprise}
      />

      <BillingMethodSteps modelCount={cards.length} />

      <TopupLadder ladder={TOPUP_LADDER} />

      <div id={PRICING_MODELS_SECTION_ID} className='scroll-mt-24 space-y-6'>
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
      </div>
    </div>
  )
}
