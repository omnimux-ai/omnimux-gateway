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

/**
 * Stable public-pricing analytics event names (PRD §9 / architecture §9).
 * Until a product-wide track() SDK is wired, events are no-op in production
 * and logged in DEV so QA can verify call sites.
 */
export const PRICING_ANALYTICS_EVENTS = {
  PAGE_VIEW: 'pricing_page_view',
  MODALITY_SWITCH: 'pricing_modality_switch',
  VENDOR_FILTER: 'pricing_vendor_filter',
  SEARCH_USED: 'pricing_search_used',
  MODEL_ID_COPY: 'pricing_model_id_copy',
  CTA_VIEW_MODELS: 'pricing_cta_view_models',
  CTA_ENTERPRISE: 'pricing_cta_enterprise',
  FORCE_REFETCH: 'pricing_force_refetch',
  SERVED_FROM_CACHE: 'pricing_served_from_cache',
  FETCH_ERROR: 'pricing_fetch_error',
  EXPR_PARSE_ERROR: 'pricing_expr_parse_error',
} as const

export type PricingAnalyticsEvent =
  (typeof PRICING_ANALYTICS_EVENTS)[keyof typeof PRICING_ANALYTICS_EVENTS]

export type PricingAnalyticsProps = Record<
  string,
  string | number | boolean | null | undefined
>

/**
 * Fire a pricing analytics event. Safe no-op when no global tracker exists.
 */
export function trackPricingEvent(
  event: PricingAnalyticsEvent,
  props: PricingAnalyticsProps = {}
): void {
  try {
    const globalTracker = (
      globalThis as {
        __OMNIMUX_TRACK__?: (
          name: string,
          payload?: PricingAnalyticsProps
        ) => void
      }
    ).__OMNIMUX_TRACK__

    if (typeof globalTracker === 'function') {
      globalTracker(event, props)
      return
    }

    if (import.meta.env.DEV) {
      // eslint-disable-next-line no-console
      console.debug('[pricing-analytics]', event, props)
    }
  } catch {
    // Analytics must never break the pricing page.
  }
}
