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
import { useCallback, useMemo, useState } from 'react'

import { useDebounce } from '@/hooks/use-debounce'

import { FILTER_ALL, MODALITY_FILTERS, type ModalityFilter } from '../constants'
import type { PublicModelCardItem, PublicModelCategory } from '../types'

export type PublicPricingFiltersState = {
  searchInput: string
  debouncedSearch: string
  modality: ModalityFilter
  vendorId: string
  setSearchInput: (value: string) => void
  setModality: (value: ModalityFilter) => void
  setVendorId: (value: string) => void
  clearSearch: () => void
  clearFilters: () => void
  filteredCards: PublicModelCardItem[]
  filteredCount: number
  hasActiveFilters: boolean
}

function matchesModality(
  category: PublicModelCategory,
  modality: ModalityFilter
): boolean {
  if (modality === MODALITY_FILTERS.ALL) {
    return true
  }
  // Public tabs map TEXT ↔ LLM category "text".
  return category === modality
}

function matchesSearch(card: PublicModelCardItem, query: string): boolean {
  if (!query) {
    return true
  }
  const q = query.trim().toLowerCase()
  if (!q) {
    return true
  }
  return (
    card.name.toLowerCase().includes(q) ||
    card.modelId.toLowerCase().includes(q) ||
    card.provider.toLowerCase().includes(q) ||
    card.id.toLowerCase().includes(q)
  )
}

function matchesVendor(card: PublicModelCardItem, vendorId: string): boolean {
  if (!vendorId || vendorId === FILTER_ALL) {
    return true
  }
  const vendorName = card.provider.toLowerCase()
  const rawVendorId = card.rawModel.vendor_id
  if (String(rawVendorId ?? '') === vendorId) {
    return true
  }
  // Also allow filtering by vendor display name for chip UIs.
  return vendorName === vendorId.toLowerCase()
}

/**
 * Slim public-pricing filter state: modality + vendor + debounced search.
 * Operates on ACL card view-models from `transformPricingDataToCards`.
 */
export function usePublicPricingFilters(
  cards: PublicModelCardItem[]
): PublicPricingFiltersState {
  const [searchInput, setSearchInputState] = useState('')
  const [modality, setModalityState] = useState<ModalityFilter>(
    MODALITY_FILTERS.VIDEO
  )
  const [vendorId, setVendorIdState] = useState<string>(FILTER_ALL)

  const debouncedSearch = useDebounce(searchInput, 200)

  const setSearchInput = useCallback((value: string) => {
    setSearchInputState(value)
  }, [])

  const setModality = useCallback((value: ModalityFilter) => {
    setModalityState(value)
  }, [])

  const setVendorId = useCallback((value: string) => {
    setVendorIdState(value || FILTER_ALL)
  }, [])

  const clearSearch = useCallback(() => {
    setSearchInputState('')
  }, [])

  const clearFilters = useCallback(() => {
    setModalityState(MODALITY_FILTERS.VIDEO)
    setVendorIdState(FILTER_ALL)
    setSearchInputState('')
  }, [])

  const filteredCards = useMemo(() => {
    if (!cards || cards.length === 0) {
      return []
    }
    return cards.filter(
      (card) =>
        matchesModality(card.category, modality) &&
        matchesVendor(card, vendorId) &&
        matchesSearch(card, debouncedSearch)
    )
  }, [cards, modality, vendorId, debouncedSearch])

  const hasActiveFilters =
    modality !== MODALITY_FILTERS.VIDEO ||
    vendorId !== FILTER_ALL ||
    searchInput.trim().length > 0

  return {
    searchInput,
    debouncedSearch,
    modality,
    vendorId,
    setSearchInput,
    setModality,
    setVendorId,
    clearSearch,
    clearFilters,
    filteredCards,
    filteredCount: filteredCards.length,
    hasActiveFilters,
  }
}
