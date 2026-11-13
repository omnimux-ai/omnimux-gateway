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
 * Public surface marker.
 *
 * Since 2026-10-27 the desktop landing page is the `omnimux.ai` homepage, so the
 * server injects `data-surface="desktop"` and `data-pricing-group="desktop-720p"`
 * on <html> for every public page (see router/web-surface.go). `d.omnimux.ai` is
 * retired and redirects here, so no hostname selects a surface any more.
 *
 * `?surface=web|desktop` remains for static previews and local QA.
 */
export type OmniMuxSurface = 'web' | 'desktop'

export function resolveSurface(): OmniMuxSurface {
  if (typeof document !== 'undefined') {
    const injected = document.documentElement.dataset.surface
    if (injected === 'desktop' || injected === 'web') return injected
  }
  if (typeof window !== 'undefined') {
    const forced = new URLSearchParams(window.location.search).get('surface')
    if (forced === 'desktop' || forced === 'web') return forced
  }
  return 'web'
}

export function isDesktopSurface(): boolean {
  return resolveSurface() === 'desktop'
}

/**
 * Group the price catalogue should price against on this surface.
 * The desktop groups are deliberately absent from the Web page: the catalogue
 * renders the cheapest enabled group, so exposing both would undercut Web.
 */
export function resolvePricingGroup(): string {
  const injected =
    typeof document !== 'undefined'
      ? document.documentElement.dataset.pricingGroup
      : undefined
  if (injected) return injected
  return isDesktopSurface() ? 'desktop-720p' : 'default'
}

export const DESKTOP_PRICING_GROUPS = ['desktop-480p', 'desktop-720p'] as const
export const WEB_PRICING_GROUPS = [
  'seedance-2-5-480p',
  'seedance-2-5-720p',
] as const

export function pricingGroupAllowlist(
  surface: OmniMuxSurface = resolveSurface()
): readonly string[] | null {
  return surface === 'desktop' ? DESKTOP_PRICING_GROUPS : null
}
