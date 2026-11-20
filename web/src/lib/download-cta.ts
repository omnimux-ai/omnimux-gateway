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

import { DESKTOP_RELEASE_ORIGIN } from '@/features/omnimux-landing/desktop/components/cn/desktop-release'

import { TRACKER_NAME, type AnalyticsTracker } from './analytics-bootstrap'

export const DOWNLOAD_PAGES = ['home', 'pricing', 'role', 'tool'] as const
export const DOWNLOAD_POSITIONS = [
  'header',
  'hero',
  'pricing_card',
  'footer',
  'tool_result',
] as const
export const DOWNLOAD_LOCALES = [
  'en',
  'id',
  'es',
  'ja',
  'zh-TW',
  'zh-CN',
  'fr',
  'ru',
  'vi',
] as const
export const DOWNLOAD_VARIANTS = [
  'primary',
  'secondary',
  'composer',
  'menu',
  'dialog',
  'direct',
  'section',
  'card',
  'nav',
  'inline',
] as const
export const DOWNLOAD_DESTINATION = new URL(DESKTOP_RELEASE_ORIGIN).hostname
export const DOWNLOAD_CTA_NAME = 'download_cta_click'
export const DOWNLOAD_CLICK_WINDOW_MS = 10_000
export const DEVICE_ID_STORAGE_KEY = 'omnimux.analytics.device_id'
const LOCK_NAME = 'omnimux.analytics.downloads'
const LAST_CLICK_PREFIX = 'omnimux.analytics.last_download'
const ANONYMOUS_DEVICE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i

export type DownloadCtaPage = (typeof DOWNLOAD_PAGES)[number]
type DownloadCtaPosition = (typeof DOWNLOAD_POSITIONS)[number]
type DownloadCtaVariant = (typeof DOWNLOAD_VARIANTS)[number]
export type DownloadCtaIntent = {
  cta_id: string
  cta_position: DownloadCtaPosition
  cta_variant: DownloadCtaVariant
}
type DownloadCtaClick = DownloadCtaIntent & {
  page: DownloadCtaPage
  locale: string
}

const memoryClicks = new Map<string, number>()

function persistDownloadClick(ctaID: string, now: number): boolean {
  const store = window.localStorage
  let deviceID = store.getItem(DEVICE_ID_STORAGE_KEY)
  if (!deviceID || !ANONYMOUS_DEVICE.test(deviceID)) {
    deviceID = window.crypto.randomUUID()
    store.setItem(DEVICE_ID_STORAGE_KEY, deviceID)
  }
  const key = LAST_CLICK_PREFIX + ':' + ctaID
  const [storedDevice, storedTime] = (store.getItem(key) ?? '').split(':')
  const age = now - Number(storedTime)
  if (
    storedDevice === deviceID &&
    age > -DOWNLOAD_CLICK_WINDOW_MS &&
    age < DOWNLOAD_CLICK_WINDOW_MS
  ) {
    return false
  }
  store.setItem(key, deviceID + ':' + now)
  return true
}

/** Report only a real installer link. The link's normal activation keeps its
 * own navigation timing, even while an analytics lock or the tracker loads.
 */
export function trackDownloadCtaClick(
  input: DownloadCtaClick,
  href: string
): void {
  try {
    if (new URL(href).origin !== DESKTOP_RELEASE_ORIGIN) return
    const tracker = (globalThis as { [TRACKER_NAME]?: AnalyticsTracker })[
      TRACKER_NAME
    ]
    if (!tracker) return

    const id = input.cta_id
    const now = Date.now()
    for (const [key, previous] of memoryClicks) {
      if (now - previous >= DOWNLOAD_CLICK_WINDOW_MS) memoryClicks.delete(key)
    }
    if (memoryClicks.has(id)) return
    memoryClicks.set(id, now)

    const report = () => {
      try {
        tracker(DOWNLOAD_CTA_NAME, {
          ...input,
          destination: DOWNLOAD_DESTINATION,
        })
      } catch {
        // A tracker failure must not affect downloading.
      }
    }
    const claim = () => {
      try {
        if (!persistDownloadClick(id, now)) return
      } catch {
        // Storage and UUID generation can be restricted. This page has
        // already claimed the click in memory.
      }
      report()
    }
    if (typeof navigator.locks?.request === 'function') {
      void navigator.locks.request(LOCK_NAME, claim).catch(() => claim())
    } else {
      claim()
    }
  } catch {
    // A restricted browser API must not affect downloading.
  }
}
