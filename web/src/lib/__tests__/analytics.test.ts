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

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { AnalyticsEvent, AnalyticsTracker } from '../analytics-bootstrap'

const BASE = {
  pathname: '/pricing',
  referrer: 'https://source.example/register?email=secret#ignored',
}

const downloadClick = {
  page: 'home',
  locale: 'en',
  cta_id: 'hero_download_primary',
  cta_position: 'hero',
  cta_variant: 'primary',
} as const

function captureUmami() {
  const payloads: { name: string; data: Record<string, unknown> }[] = []
  const track: NonNullable<Window['umami']>['track'] = (build) => {
    const defaults = {
      website: 'test-site',
      hostname: 'app.example',
      language: 'en',
      url: '/pricing?email=secret',
      referrer: 'https://source.example/?password=secret',
      title: 'private page title',
    }
    payloads.push(build(defaults))
  }
  window.umami = { track: vi.fn(track) }
  return payloads
}

function analyticsEvent(name: string, props?: object): AnalyticsEvent {
  return { ...BASE, name, props, timestamp: Date.now() }
}

function installGlobalSpy() {
  const spy = vi.fn<AnalyticsTracker>()
  vi.stubGlobal('__OMNIMUX_TRACK__', spy)
  return spy
}

beforeEach(() => {
  vi.resetModules()
  vi.useFakeTimers()
  vi.setSystemTime(new Date('2026-11-19T09:00:00Z'))
  window.localStorage.clear()
  delete window.umami
})

afterEach(() => {
  Reflect.deleteProperty(window.navigator, 'locks')
  Reflect.deleteProperty(globalThis, '__OMNIMUX_TRACK__')
  delete window.umami
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  vi.clearAllTimers()
  vi.useRealTimers()
})

describe('Umami adapter', () => {
  it('keeps only whitelisted properties and removes URL, title and referrer secrets', async () => {
    const { createAnalyticsTracker } = await import('../analytics')
    const { DOWNLOAD_DESTINATION } = await import('../download-cta')
    const sent = captureUmami()
    const track = createAnalyticsTracker()

    track(
      analyticsEvent('download_cta_click', {
        ...downloadClick,
        destination: DOWNLOAD_DESTINATION,
        href: 'https://example.com/download?token=secret',
        email: 'private@example.com',
      })
    )
    track(
      analyticsEvent('pricing_fetch_error', {
        message: 'secret upstream response',
      })
    )
    track(
      analyticsEvent('influencer_login_cta', {
        action: 'sign_in',
        source: 'publish',
      })
    )
    track(
      analyticsEvent('download_cta_click', {
        ...downloadClick,
        destination: 'other.example',
      })
    )
    track(analyticsEvent('constructor', { foo: 'bar' }))

    expect(sent).toMatchObject([
      {
        name: 'download_cta_click',
        data: { ...downloadClick, destination: DOWNLOAD_DESTINATION },
      },
      { name: 'pricing_fetch_error', data: {} },
      {
        name: 'influencer_login_cta',
        data: { action: 'sign_in', source: 'publish' },
      },
    ])
    expect(sent[0]).toEqual({
      website: 'test-site',
      hostname: 'app.example',
      language: 'en',
      screen: undefined,
      url: '/pricing',
      referrer: 'https://source.example/register',
      name: 'download_cta_click',
      data: { ...downloadClick, destination: DOWNLOAD_DESTINATION },
    })
    expect(JSON.stringify(sent)).not.toMatch(/secret|private/)
  })

  it('sends the four pricing interactions through the pricing bridge and sanitizes their fields', async () => {
    const { installAnalytics } = await import('../analytics-bootstrap')
    const adapter = await import('../analytics')
    const { trackPricingEvent, PRICING_ANALYTICS_EVENTS: events } =
      await import('@/features/pricing/lib/pricing-analytics')
    const sent = captureUmami()
    installAnalytics(async () => adapter)

    trackPricingEvent(events.TOPUP_CLICK, {
      cta_position: 'ladder',
      amount_usd: 300,
      is_authenticated: false,
      token: 'private-token',
    })
    trackPricingEvent(events.ESTIMATOR_INTERACT, {
      model_id: 'wan-3.0',
      seconds: 15,
      prompt: 'private-prompt',
    })
    trackPricingEvent(events.CATALOG_EXPAND, {
      expanded: true,
      model_count: 16,
      query: 'private-query',
    })
    trackPricingEvent(events.FAQ_TOGGLE, {
      faq_id: 'q1',
      expanded: false,
      email: 'private@example.com',
    })
    await vi.advanceTimersByTimeAsync(0)

    expect(sent.map(({ name, data }) => ({ name, data }))).toEqual([
      {
        name: 'pricing_topup_click',
        data: {
          cta_position: 'ladder',
          amount_usd: 300,
          is_authenticated: false,
        },
      },
      {
        name: 'pricing_estimator_interact',
        data: { model_id: 'wan-3.0', seconds: 15 },
      },
      {
        name: 'pricing_catalog_expand',
        data: { expanded: true, model_count: 16 },
      },
      {
        name: 'pricing_faq_toggle',
        data: { faq_id: 'q1', expanded: false },
      },
    ])
  })

  it('drops malformed pricing interactions and omits invalid optional amounts', async () => {
    const { createAnalyticsTracker } = await import('../analytics')
    const sent = captureUmami()
    const track = createAnalyticsTracker()

    track(analyticsEvent('pricing_topup_click', { cta_position: 'hero' }))
    track(
      analyticsEvent('pricing_estimator_interact', {
        model_id: 'private-token',
        seconds: 15,
      })
    )
    for (const seconds of [15.5, 31]) {
      track(
        analyticsEvent('pricing_estimator_interact', {
          model_id: 'wan-3.0',
          seconds,
        })
      )
    }
    track(
      analyticsEvent('pricing_catalog_expand', {
        expanded: true,
        model_count: -1,
      })
    )
    track(
      analyticsEvent('pricing_faq_toggle', { faq_id: 'q6', expanded: false })
    )
    track(
      analyticsEvent('pricing_topup_click', {
        cta_position: 'sticky_mobile',
        amount_usd: '300',
        is_authenticated: true,
      })
    )

    expect(sent.map(({ name, data }) => ({ name, data }))).toEqual([
      {
        name: 'pricing_topup_click',
        data: { cta_position: 'sticky_mobile', is_authenticated: true },
      },
    ])
  })

  it.each(['fr', 'ru', 'vi'])(
    'accepts %s on a pricing footer download',
    async (locale) => {
      const { createAnalyticsTracker } = await import('../analytics')
      const { DOWNLOAD_DESTINATION } = await import('../download-cta')
      const sent = captureUmami()
      const track = createAnalyticsTracker()
      const data = {
        page: 'pricing',
        cta_id: 'pricing_download_mac',
        cta_position: 'footer',
        cta_variant: 'direct',
        locale,
        destination: DOWNLOAD_DESTINATION,
      }

      track(analyticsEvent('download_cta_click', data))

      expect(sent).toMatchObject([{ name: 'download_cta_click', data }])
    }
  )

  it('preserves the tracked route and flushes only the bounded queue when Umami loads', async () => {
    const { createAnalyticsTracker } = await import('../analytics')
    const track = createAnalyticsTracker()
    for (let queryLength = 0; queryLength < 52; queryLength++) {
      track(analyticsEvent('pricing_search_used', { queryLength }))
    }

    const sent = captureUmami()
    await vi.advanceTimersByTimeAsync(100)
    expect(sent).toHaveLength(50)
    expect(sent[0]).toMatchObject({ url: '/pricing', data: { queryLength: 2 } })
    expect(sent.at(-1)).toMatchObject({ data: { queryLength: 51 } })
  })

  it('drops expired events, warns once, and resumes with new events', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const { createAnalyticsTracker } = await import('../analytics')
    const track = createAnalyticsTracker()

    track(analyticsEvent('pricing_page_view'))
    await vi.advanceTimersByTimeAsync(10_000)
    track(analyticsEvent('pricing_page_view'))
    await vi.advanceTimersByTimeAsync(10_000)

    const sent = captureUmami()
    track(analyticsEvent('pricing_cta_view_models'))
    expect(warn).toHaveBeenCalledTimes(1)
    expect(sent).toMatchObject([{ name: 'pricing_cta_view_models' }])
  })

  it('handles a rejected send without throwing into the click handler', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const { createAnalyticsTracker } = await import('../analytics')
    window.umami = {
      track: vi.fn().mockRejectedValue(new Error('private detail')),
    }
    const track = createAnalyticsTracker()

    expect(() => track(analyticsEvent('pricing_page_view'))).not.toThrow()
    await vi.advanceTimersByTimeAsync(0)
    expect(warn).toHaveBeenCalledExactlyOnceWith(
      '[analytics] Umami could not send an event'
    )
  })

  it('installs the global before lazy loading and preserves early calls', async () => {
    const spy = installGlobalSpy()
    const { installAnalytics } = await import('../analytics-bootstrap')
    const trackEvent = vi.fn()
    let finish!: (adapter: { trackEvent: typeof trackEvent }) => void
    const load = vi.fn(
      () =>
        new Promise<{ trackEvent: typeof trackEvent }>((resolve) => {
          finish = resolve
        })
    )
    installAnalytics(load)
    const tracker = (globalThis as { __OMNIMUX_TRACK__?: AnalyticsTracker })
      .__OMNIMUX_TRACK__
    expect(typeof tracker).toBe('function')
    expect(tracker).not.toBe(spy)
    expect(load).not.toHaveBeenCalled()
    tracker?.('pricing_page_view')
    tracker?.('influencer_page_view', { guest: true })
    expect(load).toHaveBeenCalledTimes(1)
    finish({ trackEvent })
    await vi.advanceTimersByTimeAsync(0)
    expect(trackEvent.mock.calls.map(([event]) => event.name)).toEqual([
      'pricing_page_view',
      'influencer_page_view',
    ])
  })

  it('keeps a fresh event when the previous event expires during adapter loading', async () => {
    const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
    const { installAnalytics } = await import('../analytics-bootstrap')
    const trackEvent = vi.fn()
    let finish!: (adapter: { trackEvent: typeof trackEvent }) => void
    installAnalytics(
      () =>
        new Promise((resolve) => {
          finish = resolve
        })
    )
    const tracker = (globalThis as { __OMNIMUX_TRACK__?: AnalyticsTracker })
      .__OMNIMUX_TRACK__
    tracker?.('pricing_page_view')
    await vi.advanceTimersByTimeAsync(9900)
    tracker?.('influencer_page_view', { guest: true })
    await vi.advanceTimersByTimeAsync(200)

    finish({ trackEvent })
    await vi.advanceTimersByTimeAsync(0)
    expect(warn).toHaveBeenCalledTimes(1)
    expect(trackEvent.mock.calls.map(([event]) => event.name)).toEqual([
      'influencer_page_view',
    ])
  })

  it('retries an interrupted adapter load and replays the click once', async () => {
    const { installAnalytics } = await import('../analytics-bootstrap')
    const trackEvent = vi.fn()
    const load = vi
      .fn()
      .mockRejectedValueOnce(new Error('download canceled script load'))
      .mockResolvedValue({ trackEvent })
    installAnalytics(load)
    const tracker = (globalThis as { __OMNIMUX_TRACK__?: AnalyticsTracker })
      .__OMNIMUX_TRACK__
    tracker?.('download_cta_click', downloadClick)

    await vi.advanceTimersByTimeAsync(500)
    expect(load).toHaveBeenCalledTimes(2)
    expect(trackEvent).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        name: 'download_cta_click',
        props: downloadClick,
      })
    )
  })
})

describe('desktop download click', () => {
  it('uses one local device across tabs, ignores another destination, and resets after ten seconds', async () => {
    const track = installGlobalSpy()
    const firstTab = await import('../download-cta')
    const href =
      'https://' + firstTab.DOWNLOAD_DESTINATION + '/installer?token=secret'
    const request = vi.fn(async (_name: string, callback: () => void) =>
      callback()
    )
    Object.defineProperty(window.navigator, 'locks', {
      configurable: true,
      value: { request },
    })

    firstTab.trackDownloadCtaClick(
      downloadClick,
      'https://example.com/download'
    )
    firstTab.trackDownloadCtaClick(downloadClick, href)
    vi.resetModules()
    const secondTab = await import('../download-cta')
    secondTab.trackDownloadCtaClick(downloadClick, href)
    expect(track).toHaveBeenCalledExactlyOnceWith('download_cta_click', {
      ...downloadClick,
      destination: firstTab.DOWNLOAD_DESTINATION,
    })
    expect(request).toHaveBeenCalledTimes(2)
    expect(window.localStorage.getItem(firstTab.DEVICE_ID_STORAGE_KEY)).toMatch(
      /^[0-9a-f-]{36}$/i
    )

    vi.advanceTimersByTime(10_000)
    secondTab.trackDownloadCtaClick(downloadClick, href)
    expect(track).toHaveBeenCalledTimes(2)
  })

  it('deduplicates without storage and sends a distinct CTA independently', async () => {
    const track = installGlobalSpy()
    const { DOWNLOAD_DESTINATION, trackDownloadCtaClick } =
      await import('../download-cta')
    vi.spyOn(window, 'localStorage', 'get').mockImplementation(() => {
      throw new DOMException('blocked', 'SecurityError')
    })
    const href = 'https://' + DOWNLOAD_DESTINATION + '/installer'
    expect(() => trackDownloadCtaClick(downloadClick, href)).not.toThrow()
    trackDownloadCtaClick(downloadClick, href)
    trackDownloadCtaClick(
      { ...downloadClick, cta_id: 'hero_download_mac_x64' },
      href
    )
    expect(track).toHaveBeenCalledTimes(2)
  })
})
