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
 * Influencer workbench analytics events. Same contract as
 * `pricing-analytics.ts`: routed through `__OMNIMUX_TRACK__` when a global
 * tracker exists, logged in DEV, and always safe to call.
 */
export const INFLUENCER_ANALYTICS_EVENTS = {
  PAGE_VIEW: 'influencer_page_view',
  /** A guest hit a write action and was offered the login gate. */
  LOGIN_CTA: 'influencer_login_cta',
  /** A guest applied an inspiration preset to the sidebar. */
  PRESET_APPLY_GUEST: 'influencer_preset_apply_guest',
  /** The wallet could not pay for a generation and the paywall took over. */
  PAYWALL_VIEW: 'influencer_paywall_view',
  /** The paywall payment completed and the generation was resumed. */
  PAYWALL_SUBSCRIBED: 'influencer_paywall_subscribed',
} as const

export type InfluencerAnalyticsEvent =
  (typeof INFLUENCER_ANALYTICS_EVENTS)[keyof typeof INFLUENCER_ANALYTICS_EVENTS]

export type InfluencerAnalyticsProps = Record<
  string,
  string | number | boolean | null | undefined
>

/** Where the login gate was triggered from — the conversion funnel detail. */
export type LoginGateSource =
  | 'generate'
  | 'regenerate'
  | 'multiview'
  | 'publish'
  | 'delete'
  | 'upload'
  | 'header'

export function trackInfluencerEvent(
  event: InfluencerAnalyticsEvent,
  props: InfluencerAnalyticsProps = {}
): void {
  try {
    const globalTracker = (
      globalThis as {
        __OMNIMUX_TRACK__?: (
          name: string,
          payload?: InfluencerAnalyticsProps
        ) => void
      }
    ).__OMNIMUX_TRACK__

    if (typeof globalTracker === 'function') {
      globalTracker(event, props)
      return
    }

    if (import.meta.env.DEV) {
      // eslint-disable-next-line no-console
      console.debug('[influencer-analytics]', event, props)
    }
  } catch {
    // Analytics must never break the workbench.
  }
}
