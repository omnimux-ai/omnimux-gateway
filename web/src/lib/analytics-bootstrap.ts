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

export const ANALYTICS_QUEUE_MAX = 50
export const ANALYTICS_QUEUE_TIMEOUT_MS = 10_000
export const TRACKER_NAME = '__OMNIMUX_TRACK__'

export type AnalyticsEvent = {
  name: string
  props: object | undefined
  pathname: string
  referrer: string
  timestamp: number
}

export type AnalyticsTracker = (name: string, props?: object) => void

type AnalyticsGlobal = typeof globalThis & {
  [TRACKER_NAME]?: AnalyticsTracker
}

const reported = new Set<string>()

export function warnAnalytics(message: string): void {
  if (reported.has(message)) return
  reported.add(message)
  console.warn('[analytics] ' + message)
}

export function installAnalytics(
  loadAdapter: () => Promise<{
    trackEvent(event: AnalyticsEvent): void
  }> = () => import('./analytics')
): void {
  const pending: AnalyticsEvent[] = []
  let trackEvent: ((event: AnalyticsEvent) => void) | undefined
  let loading = false
  let timeout: number | undefined

  ;(globalThis as AnalyticsGlobal)[TRACKER_NAME] = (name, props) => {
    const event = {
      name,
      props,
      pathname: window.location.pathname,
      referrer: document.referrer,
      timestamp: Date.now(),
    }
    if (trackEvent) {
      trackEvent(event)
      return
    }
    if (pending.length === ANALYTICS_QUEUE_MAX) pending.shift()
    pending.push(event)
    timeout ??= window.setTimeout(() => {
      pending.length = 0
      timeout = undefined
      warnAnalytics('Umami tracker unavailable')
    }, ANALYTICS_QUEUE_TIMEOUT_MS)
    if (loading) return

    loading = true
    void loadAdapter().then(
      (adapter) => {
        trackEvent = adapter.trackEvent
        window.clearTimeout(timeout)
        timeout = undefined
        for (const queued of pending.splice(0)) trackEvent(queued)
      },
      () => {
        loading = false
        pending.length = 0
        window.clearTimeout(timeout)
        timeout = undefined
        warnAnalytics('Umami tracker unavailable')
      }
    )
  }
}
