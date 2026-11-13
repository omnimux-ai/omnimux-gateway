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
import { PRICING_CACHE_KEY, PRICING_CACHE_TTL } from '../constants'
import type { PricingCacheEnvelope, PricingData } from '../types'

// ----------------------------------------------------------------------------
// L2 pricing cache (localStorage) — crash-safe read/write
// ----------------------------------------------------------------------------

/**
 * Load the L2 pricing envelope from localStorage.
 * Returns null when missing, expired (TTL), malformed, or storage unavailable.
 */
export function loadPricingCache(): PricingCacheEnvelope | null {
  if (typeof window === 'undefined') return null

  try {
    const raw = window.localStorage.getItem(PRICING_CACHE_KEY)
    if (!raw) return null

    const parsed: unknown = JSON.parse(raw)
    if (!isPricingCacheEnvelope(parsed)) {
      clearPricingCache()
      return null
    }

    const age = Date.now() - parsed.timestamp
    if (!Number.isFinite(age) || age < 0 || age > PRICING_CACHE_TTL) {
      clearPricingCache()
      return null
    }

    return parsed
  } catch {
    clearPricingCache()
    return null
  }
}

/**
 * Persist a pricing envelope to localStorage.
 * Returns false when storage is unavailable or write fails.
 */
export function savePricingCache(envelope: PricingCacheEnvelope): boolean {
  if (typeof window === 'undefined') return false
  if (!isPricingCacheEnvelope(envelope)) return false

  try {
    window.localStorage.setItem(PRICING_CACHE_KEY, JSON.stringify(envelope))
    return true
  } catch {
    return false
  }
}

/** Remove the L2 pricing envelope. No-ops when storage is unavailable. */
export function clearPricingCache(): void {
  if (typeof window === 'undefined') return

  try {
    window.localStorage.removeItem(PRICING_CACHE_KEY)
  } catch {
    // Private mode / quota — ignore
  }
}

function isPricingCacheEnvelope(value: unknown): value is PricingCacheEnvelope {
  if (value === null || typeof value !== 'object') return false

  const candidate = value as Record<string, unknown>
  if (typeof candidate.version !== 'string' || candidate.version.length === 0) {
    return false
  }
  if (
    typeof candidate.timestamp !== 'number' ||
    !Number.isFinite(candidate.timestamp)
  ) {
    return false
  }
  if (!isPricingData(candidate.pricingData)) return false

  if (candidate.statusData !== undefined) {
    if (
      candidate.statusData === null ||
      typeof candidate.statusData !== 'object'
    ) {
      return false
    }
    const status = candidate.statusData as Record<string, unknown>
    for (const key of ['price', 'usd_exchange_rate', 'quota_per_unit'] as const) {
      if (
        status[key] !== undefined &&
        (typeof status[key] !== 'number' || !Number.isFinite(status[key]))
      ) {
        return false
      }
    }
  }

  return true
}

function isPricingData(value: unknown): value is PricingData {
  if (value === null || typeof value !== 'object') return false
  const data = value as Record<string, unknown>
  return (
    typeof data.success === 'boolean' &&
    Array.isArray(data.data) &&
    Array.isArray(data.vendors)
  )
}
