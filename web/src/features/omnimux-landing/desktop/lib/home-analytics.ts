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

import { TRACKER_NAME, type AnalyticsTracker } from '@/lib/analytics-bootstrap'

export const HOME_DESKTOP_DOWNLOAD_PAGE = 'https://omnimux.ai/#download'
export const HOME_HERO_CTA_EVENT = 'home_hero_cta_click'
export const HOME_PRICING_TEASER_EVENT = 'home_pricing_teaser_click'
export const HOME_HERO_CTA_TYPES = [
  'download',
  'copy_link',
  'scroll_workflow',
] as const
export const HOME_HERO_CTA_PLATFORMS = [
  'mac-arm64',
  'mac-x64',
  'win',
  'mobile_fallback',
] as const
export const HOME_PRICING_TEASER_IDS = [
  'full_pricing',
  'download_first',
] as const

function trackHomeEvent(
  name: typeof HOME_HERO_CTA_EVENT | typeof HOME_PRICING_TEASER_EVENT,
  props: object
): void {
  try {
    const tracker = (globalThis as { [TRACKER_NAME]?: AnalyticsTracker })[
      TRACKER_NAME
    ]
    tracker?.(name, props)
  } catch {
    // Analytics must not affect the homepage action.
  }
}

export function trackHomeHeroCtaClick(
  ctaType: (typeof HOME_HERO_CTA_TYPES)[number],
  platform: (typeof HOME_HERO_CTA_PLATFORMS)[number]
): void {
  trackHomeEvent(HOME_HERO_CTA_EVENT, { cta_type: ctaType, platform })
}

export function trackHomePricingTeaserClick(
  ctaId: (typeof HOME_PRICING_TEASER_IDS)[number]
): void {
  trackHomeEvent(HOME_PRICING_TEASER_EVENT, { cta_id: ctaId })
}
