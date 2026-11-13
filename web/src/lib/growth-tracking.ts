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
 * AI growth experiment attribution (campaign `ai_growth_20261109`).
 *
 * - First touch: an inbound visit tagged with the experiment campaign stores a
 *   snapshot of its own `utm_*` values once. Later visits and in-site CTAs
 *   never overwrite it. `utm_content` is kept only when it is the single
 *   allowed content id; `utm_source` / `utm_medium` keep their inbound value
 *   after a length/character clean-up and stay empty when absent.
 * - `tool_cta_click`: in-site tool CTAs report controlled enum fields only.
 *   They never add `utm_*` to links and never carry the pasted link.
 *
 * Events go through `__OMNIMUX_TRACK__` when a global tracker exists, like
 * the influencer and pricing analytics. Without one this is a no-op.
 */

export const GROWTH_CAMPAIGN = 'ai_growth_20261109'
export const GROWTH_CONTENT_ALLOWLIST = [
  'before_after_miracle-cleaning-paste_v1',
] as const

export const FIRST_TOUCH_STORAGE_KEY = 'omnimux.growth.first_touch.v1'

/** Longest kept `utm_source` / `utm_medium` value after cleaning. */
export const UTM_VALUE_MAX_LENGTH = 64

export type FirstTouch = {
  campaign: typeof GROWTH_CAMPAIGN
  /** Cleaned inbound value; empty when the visit had none. */
  source: string
  /** Cleaned inbound value; empty when the visit had none. */
  medium: string
  /** Allowed content id, or empty when absent or not allowed. */
  content: string
}

/**
 * Keeps letters, digits and `.`, `_`, `-`; drops everything else and caps the
 * length. Never invents a value: missing or fully stripped input becomes ''.
 */
export function cleanUtmValue(raw: string | null | undefined): string {
  if (typeof raw !== 'string') return ''
  return raw
    .trim()
    .replaceAll(/[^A-Za-z0-9._-]/g, '')
    .slice(0, UTM_VALUE_MAX_LENGTH)
}

function allowedContent(raw: string | null): string {
  const value = raw?.trim() ?? ''
  return (GROWTH_CONTENT_ALLOWLIST as readonly string[]).includes(value)
    ? value
    : ''
}

/** The experiment touch carried by a query string, or null when untagged. */
export function parseGrowthTouch(search: string): FirstTouch | null {
  let params: URLSearchParams
  try {
    params = new URLSearchParams(search)
  } catch {
    return null
  }
  if (params.get('utm_campaign')?.trim() !== GROWTH_CAMPAIGN) return null
  return {
    campaign: GROWTH_CAMPAIGN,
    source: cleanUtmValue(params.get('utm_source')),
    medium: cleanUtmValue(params.get('utm_medium')),
    content: allowedContent(params.get('utm_content')),
  }
}

function storage(): Storage | null {
  try {
    return globalThis.localStorage ?? null
  } catch {
    return null
  }
}

function isFirstTouch(value: unknown): value is FirstTouch {
  if (!value || typeof value !== 'object') return false
  const touch = value as Record<string, unknown>
  return (
    touch.campaign === GROWTH_CAMPAIGN &&
    typeof touch.source === 'string' &&
    typeof touch.medium === 'string' &&
    typeof touch.content === 'string'
  )
}

export function readFirstTouch(): FirstTouch | null {
  try {
    const raw = storage()?.getItem(FIRST_TOUCH_STORAGE_KEY)
    if (!raw) return null
    const parsed: unknown = JSON.parse(raw)
    if (!isFirstTouch(parsed)) return null
    // Re-apply the rules so a hand-edited snapshot cannot widen them.
    return {
      campaign: GROWTH_CAMPAIGN,
      source: cleanUtmValue(parsed.source),
      medium: cleanUtmValue(parsed.medium),
      content: allowedContent(parsed.content),
    }
  } catch {
    return null
  }
}

/**
 * Stores the first experiment touch from `search` (defaults to the current
 * URL). An existing snapshot is never replaced. Returns the kept snapshot.
 */
export function captureFirstTouch(search?: string): FirstTouch | null {
  const existing = readFirstTouch()
  if (existing) return existing
  const query =
    search ??
    (typeof window === 'undefined' ? '' : (window.location?.search ?? ''))
  const touch = parseGrowthTouch(query)
  if (!touch) return null
  try {
    storage()?.setItem(FIRST_TOUCH_STORAGE_KEY, JSON.stringify(touch))
  } catch {
    // Storage may be blocked; attribution simply stays empty.
  }
  return touch
}

export const TOOL_CTA_CLICK_EVENT = 'tool_cta_click'

export const TOOL_CTA_IDS = [
  'recreate_steps',
  'pricing',
  'studio_fallback',
  'workbench_recreate',
  'desktop_download',
] as const
export const TOOL_CTA_VARIANTS = ['conservative', 'strong_licensed'] as const
export const TOOL_CTA_PLACEMENTS = [
  'result_primary',
  'inline_secondary',
  'footer',
] as const
export const TOOL_CTA_TOOLS = ['photo', 'video'] as const
export const TOOL_CTA_LOCALES = ['en', 'zh-CN'] as const

export type ToolCtaId = (typeof TOOL_CTA_IDS)[number]
export type ToolCtaVariant = (typeof TOOL_CTA_VARIANTS)[number]
export type ToolCtaPlacement = (typeof TOOL_CTA_PLACEMENTS)[number]

export type ToolCtaClick = {
  tool: (typeof TOOL_CTA_TOOLS)[number]
  locale: (typeof TOOL_CTA_LOCALES)[number]
  cta_id: ToolCtaId
  cta_variant: ToolCtaVariant
  placement: ToolCtaPlacement
}

export type ToolCtaClickPayload = ToolCtaClick & {
  acq_campaign: string
  acq_source: string
  acq_medium: string
  acq_content: string
  acq_source_present: boolean
}

/** Same `cta_id` clicks within this window report once. */
export const TOOL_CTA_THROTTLE_MS = 1000

const lastClickAt = new Map<string, number>()

/** Test hook: forget throttle state. */
export function resetToolCtaThrottle(): void {
  lastClickAt.clear()
}

function oneOf<T extends string>(list: readonly T[], value: unknown): boolean {
  return (
    typeof value === 'string' && (list as readonly string[]).includes(value)
  )
}

/**
 * Builds the event payload, or null when any field is outside its enum
 * (fail-closed: the whole event is dropped). Unknown keys are not copied.
 */
export function buildToolCtaClick(
  input: ToolCtaClick
): ToolCtaClickPayload | null {
  if (
    !oneOf(TOOL_CTA_TOOLS, input.tool) ||
    !oneOf(TOOL_CTA_LOCALES, input.locale) ||
    !oneOf(TOOL_CTA_IDS, input.cta_id) ||
    !oneOf(TOOL_CTA_VARIANTS, input.cta_variant) ||
    !oneOf(TOOL_CTA_PLACEMENTS, input.placement)
  ) {
    return null
  }
  const touch = readFirstTouch()
  return {
    tool: input.tool,
    locale: input.locale,
    cta_id: input.cta_id,
    cta_variant: input.cta_variant,
    placement: input.placement,
    acq_campaign: touch?.campaign ?? '',
    acq_source: touch?.source ?? '',
    acq_medium: touch?.medium ?? '',
    acq_content: touch?.content ?? '',
    acq_source_present: Boolean(touch?.source),
  }
}

/** Reports an in-site tool CTA click. Always safe to call; never throws. */
export function trackToolCtaClick(input: ToolCtaClick): void {
  try {
    const payload = buildToolCtaClick(input)
    if (!payload) return
    const now = Date.now()
    const last = lastClickAt.get(payload.cta_id)
    if (last !== undefined && now - last < TOOL_CTA_THROTTLE_MS) return
    lastClickAt.set(payload.cta_id, now)

    const globalTracker = (
      globalThis as {
        __OMNIMUX_TRACK__?: (name: string, payload?: object) => void
      }
    ).__OMNIMUX_TRACK__
    if (typeof globalTracker === 'function') {
      globalTracker(TOOL_CTA_CLICK_EVENT, payload)
      return
    }
    if (import.meta.env.DEV) {
      // eslint-disable-next-line no-console
      console.debug('[growth-analytics]', TOOL_CTA_CLICK_EVENT, payload)
    }
  } catch {
    // Analytics must never break the page.
  }
}
