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

const desktopCopy = {
  page: 'home',
  locale: 'zh-TW',
  cta_id: 'hero_copy_download_link',
  cta_position: 'hero',
  cta_variant: 'copy_link',
  destination: 'desktop_download_page',
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
  vi.stubGlobal(Symbol.for('omnimux.analytics.public-ids'), undefined)
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

  it('delivers the home CTA events to Umami through the installed tracker', async () => {
    const { installAnalytics } = await import('../analytics-bootstrap')
    const adapter = await import('../analytics')
    const { trackHomeHeroCtaClick, trackHomePricingTeaserClick } =
      await import('@/features/omnimux-landing/desktop/lib/home-analytics')
    const sent = captureUmami()
    installAnalytics(async () => adapter)

    trackHomeHeroCtaClick('download', 'mac-x64')
    trackHomeHeroCtaClick('copy_link', 'mobile_fallback')
    trackHomeHeroCtaClick('scroll_workflow', 'win')
    trackHomePricingTeaserClick('full_pricing')
    trackHomePricingTeaserClick('download_first')
    await vi.advanceTimersByTimeAsync(0)

    expect(sent.map(({ name, data }) => ({ name, data }))).toEqual([
      {
        name: 'home_hero_cta_click',
        data: { cta_type: 'download', platform: 'mac-x64' },
      },
      {
        name: 'home_hero_cta_click',
        data: { cta_type: 'copy_link', platform: 'mobile_fallback' },
      },
      {
        name: 'home_hero_cta_click',
        data: { cta_type: 'scroll_workflow', platform: 'win' },
      },
      {
        name: 'home_pricing_teaser_click',
        data: { cta_id: 'full_pricing' },
      },
      {
        name: 'home_pricing_teaser_click',
        data: { cta_id: 'download_first' },
      },
    ])
  })

  it('rejects malformed home CTA events and sends only allowed properties', async () => {
    const { createAnalyticsTracker } = await import('../analytics')
    const { DOWNLOAD_DESTINATION } = await import('../download-cta')
    const sent = captureUmami()
    const track = createAnalyticsTracker()
    const privateUrl = 'https://app.example/?token=secret'

    const installer = { ...downloadClick, destination: DOWNLOAD_DESTINATION }
    for (const overrides of [
      { locale: privateUrl },
      { page: 'other' },
      { cta_position: 'other' },
      { cta_id: privateUrl },
      { cta_variant: 'copy_link' },
      { destination: undefined },
      { destination: privateUrl },
    ]) {
      track(
        analyticsEvent('download_cta_click', { ...installer, ...overrides })
      )
    }
    track(analyticsEvent('download_cta_click', desktopCopy))
    track(analyticsEvent('home_hero_cta_click', { cta_type: 'download' }))
    track(
      analyticsEvent('home_hero_cta_click', {
        cta_type: 'download',
        platform: privateUrl,
      })
    )
    track(
      analyticsEvent('home_hero_cta_click', {
        cta_type: 'unknown',
        platform: 'win',
      })
    )
    track(analyticsEvent('home_pricing_teaser_click', { cta_id: privateUrl }))
    track(analyticsEvent('home_pricing_teaser_click'))
    expect(sent).toEqual([])

    track(
      analyticsEvent('download_cta_click', { ...installer, url: privateUrl })
    )
    track(
      analyticsEvent('home_hero_cta_click', {
        cta_type: 'download',
        platform: 'mac-arm64',
        prompt: 'private prompt',
      })
    )
    track(
      analyticsEvent('home_pricing_teaser_click', {
        cta_id: 'download_first',
        href: privateUrl,
      })
    )

    expect(sent.map(({ name, data }) => ({ name, data }))).toEqual([
      { name: 'download_cta_click', data: installer },
      {
        name: 'home_hero_cta_click',
        data: { cta_type: 'download', platform: 'mac-arm64' },
      },
      {
        name: 'home_pricing_teaser_click',
        data: { cta_id: 'download_first' },
      },
    ])
  })

  it('keeps home analytics best effort with missing and failing trackers', async () => {
    const { trackHomeHeroCtaClick, trackHomePricingTeaserClick } =
      await import('@/features/omnimux-landing/desktop/lib/home-analytics')
    const report = () => {
      trackHomeHeroCtaClick('scroll_workflow', 'mobile_fallback')
      trackHomePricingTeaserClick('full_pricing')
    }

    expect(report).not.toThrow()
    expect(window.localStorage.length).toBe(0)
    const tracker = installGlobalSpy().mockImplementation(() => {
      throw new Error('tracker failure')
    })
    expect(report).not.toThrow()
    expect(tracker).toHaveBeenCalledTimes(2)
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

  it('preserves all twelve public CLI and Skills paths on navigation downloads', async () => {
    const { createAnalyticsTracker } = await import('../analytics')
    const { DOWNLOAD_DESTINATION } = await import('../download-cta')
    const { AGENT_PATHS } =
      await import('@/features/omnimux-landing/desktop/agent/content')
    const sent = captureUmami()
    const track = createAnalyticsTracker()
    const expected = Object.entries(AGENT_PATHS).flatMap(([locale, paths]) =>
      Object.values(paths).map((url) => ({
        url,
        name: 'download_cta_click',
        data: {
          page: 'tool',
          cta_id: 'nav_download',
          cta_position: 'header',
          cta_variant: 'nav',
          locale,
          destination: DOWNLOAD_DESTINATION,
        },
      }))
    )

    for (const { url, name, data } of expected) {
      track({ ...analyticsEvent(name, data), pathname: url })
    }

    expect(expected).toHaveLength(12)
    expect(sent).toMatchObject(expected)
  })

  it.each([
    ['pricing_model_id_copy', 'modelId', '2001:db8::55'],
    ['pricing_vendor_filter', 'vendorId', '202-555-0147'],
    ['influencer_preset_apply_guest', 'preset', 'fixture-account-id-55'],
    [
      'influencer_preset_apply_guest',
      'preset',
      '12345678-1234-4123-8123-123456789013',
    ],
  ])(
    'omits an unverified %s identifier without dropping the event',
    async (name, key, value) => {
      const { createAnalyticsTracker } = await import('../analytics')
      const sent = captureUmami()

      createAnalyticsTracker()(analyticsEvent(name, { [key]: value }))

      expect(sent).toMatchObject([{ name, data: {} }])
      expect(sent[0].data).toEqual({})
    }
  )

  it('keeps valid public IDs across separate entry caches and filters personal or refreshed IDs', async () => {
    const { setPublicAnalyticsIds } = await import('../analytics-public-ids')
    const publicIds = [
      ['pricing_model_id_copy', 'modelId', 'public/text-v2', '192.0.2.44'],
      ['pricing_vendor_filter', 'vendorId', '42', '202-555-0147'],
      [
        'influencer_preset_apply_guest',
        'preset',
        '12345678-1234-4123-8123-123456789012',
        'sample@example.test',
      ],
    ] as const

    for (const [, key, id, personalValue] of publicIds) {
      setPublicAnalyticsIds(key, [id, personalValue, '2001:db8::55'])
    }
    vi.resetModules()
    const { createAnalyticsTracker } = await import('../analytics')
    const sent = captureUmami()
    const track = createAnalyticsTracker()

    for (const [name, key, id, personalValue] of publicIds) {
      track(analyticsEvent(name, { [key]: id }))
      track(analyticsEvent(name, { [key]: personalValue }))
      track(analyticsEvent(name, { [key]: '2001:db8::55' }))
      setPublicAnalyticsIds(key, ['next-id'])
      track(analyticsEvent(name, { [key]: id }))
    }

    expect(sent.map(({ name, data }) => ({ name, data }))).toEqual(
      publicIds.flatMap(([name, key, id]) => [
        { name, data: { [key]: id } },
        { name, data: {} },
        { name, data: {} },
        { name, data: {} },
      ])
    )
  })

  it('omits an ID removed from the catalog while an event waits for Umami', async () => {
    const { createAnalyticsTracker } = await import('../analytics')
    const { setPublicAnalyticsIds } = await import('../analytics-public-ids')
    const track = createAnalyticsTracker()
    setPublicAnalyticsIds('modelId', ['removed-model', 'stays-public'])
    track(analyticsEvent('pricing_model_id_copy', { modelId: 'removed-model' }))
    track(analyticsEvent('pricing_model_id_copy', { modelId: 'stays-public' }))

    setPublicAnalyticsIds('modelId', ['stays-public'])
    const sent = captureUmami()
    await vi.advanceTimersByTimeAsync(100)

    expect(sent.map(({ name, data }) => ({ name, data }))).toEqual([
      { name: 'pricing_model_id_copy', data: {} },
      { name: 'pricing_model_id_copy', data: { modelId: 'stays-public' } },
    ])
  })

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
  it('ignores a link that does not activate an installer on the download origin', async () => {
    const track = installGlobalSpy()
    const { DOWNLOAD_DESTINATION, trackDownloadCtaClick } =
      await import('../download-cta')
    const { HOME_DESKTOP_DOWNLOAD_PAGE } =
      await import('@/features/omnimux-landing/desktop/lib/home-analytics')

    trackDownloadCtaClick(downloadClick, HOME_DESKTOP_DOWNLOAD_PAGE)
    trackDownloadCtaClick(
      downloadClick,
      `https://${DOWNLOAD_DESTINATION}/releases/latest/`
    )
    trackDownloadCtaClick(downloadClick, 'https://example.com/installer.dmg')

    expect(track).not.toHaveBeenCalled()
  })

  it('counts at ten seconds across tabs even after a rejected click just before the boundary', async () => {
    const track = installGlobalSpy()
    const firstTab = await import('../download-cta')
    const href = `https://${firstTab.DOWNLOAD_DESTINATION}/installer.dmg?token=secret`
    const request = vi.fn(async (_name: string, callback: () => void) =>
      callback()
    )
    Object.defineProperty(window.navigator, 'locks', {
      configurable: true,
      value: { request },
    })

    firstTab.trackDownloadCtaClick(downloadClick, href)
    vi.resetModules()
    const secondTab = await import('../download-cta')
    vi.advanceTimersByTime(9_999)
    secondTab.trackDownloadCtaClick(downloadClick, href)
    expect(track).toHaveBeenCalledExactlyOnceWith('download_cta_click', {
      ...downloadClick,
      destination: firstTab.DOWNLOAD_DESTINATION,
    })
    expect(request).toHaveBeenCalledTimes(2)
    expect(window.localStorage.getItem(firstTab.DEVICE_ID_STORAGE_KEY)).toMatch(
      /^[0-9a-f-]{36}$/i
    )

    vi.advanceTimersByTime(1)
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
    const href = `https://${DOWNLOAD_DESTINATION}/installer.exe`
    expect(() => trackDownloadCtaClick(downloadClick, href)).not.toThrow()
    trackDownloadCtaClick(downloadClick, href)
    trackDownloadCtaClick(
      { ...downloadClick, cta_id: 'hero_download_mac_x64' },
      href
    )
    expect(track).toHaveBeenCalledTimes(2)
  })
})
