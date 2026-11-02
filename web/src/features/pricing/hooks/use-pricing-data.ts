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
import { useQuery } from '@tanstack/react-query'
import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { useStatus } from '@/hooks/use-status'
import {
  DESKTOP_PRICING_GROUPS,
  resolveSurface,
  type OmniMuxSurface,
} from '@/lib/surface'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getPricing } from '../api'
import { PRICING_STALE_TIME } from '../constants'
import { loadPricingCache, savePricingCache } from '../lib/cache'
import { transformPricingDataToCards } from '../lib/transform'
import type { PricingCacheEnvelope, PricingData } from '../types'

/**
 * The public catalogue renders the cheapest enabled group. Desktop tiers must
 * never reach the Web page (and vice versa), so each surface sees only its own
 * groups before the cheapest-group reduction runs.
 */
function filterPricingGroups(
  data: PricingData | undefined,
  surface: OmniMuxSurface
): PricingData | undefined {
  if (!data?.data) return data
  const desktop = new Set<string>(DESKTOP_PRICING_GROUPS)
  return {
    ...data,
    data: data.data.map((model) => ({
      ...model,
      enable_groups: (model.enable_groups ?? []).filter((group) =>
        surface === 'desktop' ? desktop.has(group) : !desktop.has(group)
      ),
    })),
  }
}

export function usePricingData(
  enabled = true,
  options: { selectedGroup?: string } = {}
) {
  const { status } = useStatus()
  const [bootCache] = useState<PricingCacheEnvelope | null>(() =>
    loadPricingCache()
  )
  const wroteNetworkRef = useRef(false)
  const [lastUpdated, setLastUpdated] = useState<number | null>(
    bootCache?.timestamp ?? null
  )

  const {
    data,
    isLoading,
    isFetching,
    isError,
    error,
    isSuccess,
    isPlaceholderData,
    dataUpdatedAt,
    refetch: queryRefetch,
  } = useQuery({
    queryKey: ['pricing'],
    queryFn: async () => requireServerSuccess(await getPricing()),
    staleTime: PRICING_STALE_TIME,
    refetchInterval: PRICING_STALE_TIME,
    refetchIntervalInBackground: false,
    enabled,
    // L2 hydrate for first paint without treating cache as authoritative.
    placeholderData: bootCache?.pricingData,
  })

  // Persist successful network payloads into L2 (never write placeholder/cache-only).
  useEffect(() => {
    if (!isSuccess || isPlaceholderData || !data || data.success === false) {
      return
    }

    const version =
      (typeof data.pricing_version === 'string' && data.pricing_version) ||
      data.data?.[0]?.pricing_version ||
      'unknown'
    const timestamp = dataUpdatedAt || Date.now()
    const envelope: PricingCacheEnvelope = {
      version,
      timestamp,
      pricingData: data,
      statusData: status
        ? {
            price: status.price as number | undefined,
            usd_exchange_rate: status.usd_exchange_rate as number | undefined,
            quota_per_unit: status.quota_per_unit as number | undefined,
          }
        : bootCache?.statusData,
    }
    savePricingCache(envelope)
    wroteNetworkRef.current = true
    setLastUpdated(timestamp)
  }, [
    bootCache?.statusData,
    data,
    dataUpdatedAt,
    isPlaceholderData,
    isSuccess,
    status,
  ])

  const priceRate = useMemo(
    () => Math.max((status?.price as number) ?? 1, 0.001),
    [status?.price]
  )
  const usdExchangeRate = useMemo(
    () => Math.max((status?.usd_exchange_rate as number) ?? priceRate, 0.001),
    [status?.usd_exchange_rate, priceRate]
  )

  const statusSnapshot = useMemo(
    () => ({
      price: priceRate,
      usd_exchange_rate: usdExchangeRate,
      quota_per_unit: (status?.quota_per_unit as number | undefined) ?? undefined,
    }),
    [priceRate, status?.quota_per_unit, usdExchangeRate]
  )

  const rawPricingData: PricingData | undefined = data

  const models = useMemo(() => {
    if (!data?.data || !data?.vendors) return []

    const vendorMap = new Map(data.vendors.map((v) => [v.id, v]))

    return data.data.map((model) => {
      const vendor = model.vendor_id
        ? vendorMap.get(model.vendor_id)
        : undefined
      return {
        ...model,
        key: model.model_name,
        vendor_name: vendor?.name,
        vendor_icon: vendor?.icon,
        vendor_description: vendor?.description,
        group_ratio: data.group_ratio,
      }
    })
  }, [data])

  const cards = useMemo(
    () =>
      transformPricingDataToCards(
        filterPricingGroups(rawPricingData, resolveSurface()),
        statusSnapshot,
        { selectedGroup: options.selectedGroup }
      ),
    [rawPricingData, statusSnapshot, options.selectedGroup]
  )

  const pricingVersion =
    (typeof data?.pricing_version === 'string' && data.pricing_version) ||
    data?.data?.[0]?.pricing_version ||
    bootCache?.version ||
    null

  const isUsingCache = Boolean(
    bootCache && (isPlaceholderData || (isError && !wroteNetworkRef.current))
  )

  const refetch = useCallback(
    async (options?: { force?: boolean }) => {
      if (options?.force) {
        return queryRefetch({ cancelRefetch: false })
      }
      return queryRefetch()
    },
    [queryRefetch]
  )

  let dataSource: 'network' | 'cache' | 'none' = 'none'
  if (isUsingCache) {
    dataSource = 'cache'
  } else if (data && !isPlaceholderData) {
    dataSource = 'network'
  } else if (data) {
    dataSource = 'cache'
  }

  return {
    rawPricingData,
    models,
    cards,
    vendors: data?.vendors ?? bootCache?.pricingData.vendors ?? [],
    groupRatio: data?.group_ratio ?? {},
    usableGroup: data?.usable_group ?? {},
    endpointMap: data?.supported_endpoint ?? {},
    autoGroups: data?.auto_groups ?? [],
    isLoading: isLoading && !bootCache,
    isFetching,
    isUsingCache,
    lastUpdated,
    pricingVersion,
    error,
    refetch,
    forceRefetch: () => refetch({ force: true }),
    priceRate,
    usdExchangeRate,
    dataSource,
  }
}
