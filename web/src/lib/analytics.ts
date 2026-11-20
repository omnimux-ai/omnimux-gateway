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

import {
  HOME_HERO_CTA_EVENT,
  HOME_HERO_CTA_PLATFORMS,
  HOME_HERO_CTA_TYPES,
  HOME_PRICING_TEASER_EVENT,
  HOME_PRICING_TEASER_IDS,
} from '@/features/omnimux-landing/desktop/lib/home-analytics'
import { PRICING_ANALYTICS_EVENTS } from '@/features/pricing/lib/pricing-analytics'

import {
  ANALYTICS_QUEUE_MAX,
  ANALYTICS_QUEUE_TIMEOUT_MS,
  warnAnalytics,
  type AnalyticsEvent,
} from './analytics-bootstrap'
import {
  DOWNLOAD_CTA_NAME,
  DOWNLOAD_DESTINATION,
  DOWNLOAD_LOCALES,
  DOWNLOAD_PAGES,
  DOWNLOAD_POSITIONS,
  DOWNLOAD_VARIANTS,
  HOME_DESKTOP_COPY_CTA,
} from './download-cta'
import {
  buildToolCtaClick,
  TOOL_CTA_CLICK_EVENT,
  type ToolCtaClick,
} from './growth-tracking'

type AnalyticsValue = string | number | boolean
type AnalyticsProps = Record<string, AnalyticsValue>
type Field = (value: unknown) => AnalyticsValue | undefined
type EventField = Field | { required: Field }
type UmamiDefaults = {
  website?: string
  hostname?: string
  language?: string
  screen?: string
}
type UmamiPayload = UmamiDefaults & {
  url: string
  referrer: string
  name: string
  data: AnalyticsProps
}

declare global {
  interface Window {
    umami?: {
      track: (
        build: (defaults: UmamiDefaults) => UmamiPayload
      ) => void | Promise<void>
    }
  }
}

const allowed =
  (values: readonly (string | number)[]): Field =>
  (value) =>
    (typeof value === 'string' || typeof value === 'number') &&
    values.includes(value)
      ? value
      : undefined
const boundedNumber =
  (min: number, max: number): Field =>
  (value) =>
    typeof value === 'number' &&
    Number.isFinite(value) &&
    value >= min &&
    value <= max
      ? value
      : undefined
const boundedInteger = (min: number, max: number): Field => {
  const validate = boundedNumber(min, max)
  return (value) =>
    typeof value === 'number' && Number.isInteger(value)
      ? validate(value)
      : undefined
}
const required = (validate: Field): EventField => ({ required: validate })
const identifier = (value: unknown) =>
  typeof value === 'string' &&
  /^[A-Za-z0-9][A-Za-z0-9._:/+-]{0,119}$/.test(value) &&
  !value.includes('://') &&
  !value.includes('..') &&
  !/^(?:\d{1,3}\.){3}\d{1,3}$/.test(value)
    ? value
    : undefined
const boolean: Field = (value) =>
  typeof value === 'boolean' ? value : undefined
const timestamp: Field = (value) => {
  if (
    typeof value === 'string' &&
    /^\d{4}-\d\d-\d\dT[\d:.+-]+Z?$/.test(value) &&
    value.length <= 40
  ) {
    return value
  }
  return boundedNumber(0, Number.MAX_SAFE_INTEGER)(value)
}

const homeDesktopCopyFields = {
  page: allowed([HOME_DESKTOP_COPY_CTA.page]),
  cta_id: allowed([HOME_DESKTOP_COPY_CTA.cta_id]),
  cta_position: allowed([HOME_DESKTOP_COPY_CTA.cta_position]),
  cta_variant: allowed([HOME_DESKTOP_COPY_CTA.cta_variant]),
  locale: allowed(DOWNLOAD_LOCALES),
  destination: allowed([HOME_DESKTOP_COPY_CTA.destination]),
}

const eventFields: Record<string, Record<string, EventField>> = {
  [DOWNLOAD_CTA_NAME]: {
    page: allowed(DOWNLOAD_PAGES),
    cta_id: (value) =>
      typeof value === 'string' && /^[a-z][a-z0-9_-]{0,63}$/.test(value)
        ? value
        : undefined,
    cta_position: allowed(DOWNLOAD_POSITIONS),
    cta_variant: allowed(DOWNLOAD_VARIANTS),
    locale: allowed(DOWNLOAD_LOCALES),
    destination: allowed([DOWNLOAD_DESTINATION]),
  },
  [HOME_HERO_CTA_EVENT]: {
    cta_type: required(allowed(HOME_HERO_CTA_TYPES)),
    platform: required(allowed(HOME_HERO_CTA_PLATFORMS)),
  },
  [HOME_PRICING_TEASER_EVENT]: {
    cta_id: required(allowed(HOME_PRICING_TEASER_IDS)),
  },
  pricing_page_view: {
    dataSource: allowed(['network', 'cache', 'none']),
    modelCount: boundedNumber(0, 100_000),
  },
  pricing_modality_switch: {
    modality: allowed(['all', 'text', 'image', 'video', 'audio', 'social']),
  },
  pricing_vendor_filter: { vendorId: identifier },
  pricing_search_used: { queryLength: boundedNumber(0, 500) },
  pricing_model_id_copy: { modelId: identifier },
  pricing_cta_view_models: {},
  pricing_cta_enterprise: {},
  pricing_force_refetch: {},
  pricing_served_from_cache: { lastUpdated: timestamp },
  pricing_fetch_error: {},
  pricing_expr_parse_error: {},
  [PRICING_ANALYTICS_EVENTS.TOPUP_CLICK]: {
    cta_position: required(allowed(['hero', 'ladder', 'sticky_mobile'])),
    amount_usd: allowed([10, 50, 100, 300, 1000, 3000]),
    is_authenticated: required(boolean),
  },
  [PRICING_ANALYTICS_EVENTS.ESTIMATOR_INTERACT]: {
    model_id: required(allowed(['seedance-2-5', 'wan-3.0', 'seedance-2-0'])),
    seconds: required(boundedInteger(5, 30)),
  },
  [PRICING_ANALYTICS_EVENTS.CATALOG_EXPAND]: {
    expanded: required(boolean),
    model_count: required(boundedInteger(0, 100_000)),
  },
  [PRICING_ANALYTICS_EVENTS.FAQ_TOGGLE]: {
    faq_id: required(allowed(['q1', 'q2', 'q3', 'q4', 'q5'])),
    expanded: required(boolean),
  },
  influencer_page_view: { guest: boolean },
  influencer_login_cta: {
    action: allowed(['sign_in', 'sign_up']),
    source: allowed([
      'generate',
      'regenerate',
      'multiview',
      'publish',
      'delete',
      'upload',
      'header',
    ]),
  },
  influencer_preset_apply_guest: { preset: identifier },
  influencer_paywall_view: {
    origin: allowed(['image', 'video', 'multiview']),
    required_credits: boundedNumber(-1, 1_000_000_000),
  },
  influencer_paywall_subscribed: {
    origin: allowed(['image', 'video', 'multiview']),
  },
}

const staticPaths = new Set([
  '/',
  '/pricing',
  '/login',
  '/register',
  '/sign-in',
  '/sign-up',
  '/models',
  '/influencer',
  '/ai-influencer-studio',
  '/agent',
  '/tools/tiktok-photo-downloader',
  '/tools/tiktok-video-downloader',
  '/zh/tools/tiktok-photo-downloader',
  '/zh/tools/tiktok-video-downloader',
])

function staticPath(pathname: string): string {
  return staticPaths.has(pathname) ? pathname : '/'
}

export function sanitizeReferrer(raw: string): string {
  if (raw.length > 2048) return ''
  try {
    const url = new URL(raw)
    if (
      (url.protocol !== 'https:' && url.protocol !== 'http:') ||
      url.username ||
      url.password ||
      !/^[a-z0-9.-]+\.[a-z][a-z0-9-]*$/i.test(url.hostname)
    ) {
      return ''
    }
    return url.origin + staticPath(url.pathname)
  } catch {
    return ''
  }
}

function sanitizeProperties(event: AnalyticsEvent): AnalyticsProps | null {
  if (event.name === TOOL_CTA_CLICK_EVENT) {
    return buildToolCtaClick((event.props ?? {}) as ToolCtaClick)
  }
  if (!Object.hasOwn(eventFields, event.name)) return null

  const input = (event.props ?? {}) as Record<string, unknown>
  const fields =
    event.name === DOWNLOAD_CTA_NAME &&
    input.cta_id === HOME_DESKTOP_COPY_CTA.cta_id
      ? homeDesktopCopyFields
      : eventFields[event.name]
  const data: AnalyticsProps = {}
  for (const [key, field] of Object.entries(fields)) {
    const isRequired = typeof field !== 'function'
    const validate = isRequired ? field.required : field
    const value = Object.hasOwn(input, key) ? validate(input[key]) : undefined
    if (value === undefined) {
      if (isRequired || event.name === DOWNLOAD_CTA_NAME) return null
    } else {
      data[key] = value
    }
  }
  return data
}

type PendingEvent = Pick<AnalyticsEvent, 'name' | 'timestamp'> & {
  url: string
  referrer: string
  data: AnalyticsProps
}

export function createAnalyticsTracker(): (event: AnalyticsEvent) => void {
  const pending: PendingEvent[] = []
  let timer: number | undefined

  const flush = () => {
    timer = undefined
    const tracker = window.umami
    const now = Date.now()
    while (
      pending.length > 0 &&
      now - pending[0].timestamp >= ANALYTICS_QUEUE_TIMEOUT_MS
    ) {
      pending.shift()
      warnAnalytics('Umami tracker unavailable')
    }
    if (!pending.length) return
    if (typeof tracker?.track !== 'function') {
      timer = window.setTimeout(flush, 100)
      return
    }
    for (const event of pending.splice(0)) {
      try {
        // The object callback overrides Umami's default URL, referrer and
        // title, which can carry query strings and user-entered values.
        const sent = tracker.track(
          ({ website, hostname, language, screen }) => ({
            website,
            hostname,
            language,
            screen,
            url: event.url,
            referrer: event.referrer,
            name: event.name,
            data: event.data,
          })
        )
        void Promise.resolve(sent).catch(() =>
          warnAnalytics('Umami could not send an event')
        )
      } catch {
        warnAnalytics('Umami could not send an event')
      }
    }
  }

  return (event) => {
    try {
      const data = sanitizeProperties(event)
      if (!data) return
      if (pending.length === ANALYTICS_QUEUE_MAX) pending.shift()
      pending.push({
        name: event.name,
        timestamp: event.timestamp,
        url: staticPath(event.pathname),
        referrer: sanitizeReferrer(event.referrer),
        data,
      })
      window.clearTimeout(timer)
      flush()
    } catch {
      // Tracking is best effort and must not affect the originating action.
    }
  }
}

export const trackEvent = createAnalyticsTracker()
