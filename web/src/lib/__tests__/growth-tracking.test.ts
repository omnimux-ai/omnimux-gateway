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

import {
  FIRST_TOUCH_STORAGE_KEY,
  GROWTH_CAMPAIGN,
  TOOL_CTA_CLICK_EVENT,
  TOOL_CTA_IDS,
  TOOL_CTA_THROTTLE_MS,
  UTM_VALUE_MAX_LENGTH,
  buildToolCtaClick,
  captureFirstTouch,
  cleanUtmValue,
  parseGrowthTouch,
  readFirstTouch,
  resetToolCtaThrottle,
  trackToolCtaClick,
  type ToolCtaClick,
} from '../growth-tracking'

const CONTENT = 'before_after_miracle-cleaning-paste_v1'
const tracker = vi.fn()

function q(params: Record<string, string>): string {
  return `?${new URLSearchParams(params).toString()}`
}

const click: ToolCtaClick = {
  tool: 'video',
  locale: 'en',
  cta_id: 'workbench_recreate',
  cta_variant: 'conservative',
  placement: 'result_primary',
}

beforeEach(() => {
  ;(globalThis as { __OMNIMUX_TRACK__?: unknown }).__OMNIMUX_TRACK__ = tracker
  tracker.mockReset()
  resetToolCtaThrottle()
  window.localStorage.clear()
})

afterEach(() => {
  delete (globalThis as { __OMNIMUX_TRACK__?: unknown }).__OMNIMUX_TRACK__
  vi.useRealTimers()
  window.localStorage.clear()
})

describe('utm_content allowlist', () => {
  it('keeps the single allowed content id', () => {
    expect(
      parseGrowthTouch(
        q({ utm_campaign: GROWTH_CAMPAIGN, utm_content: CONTENT })
      )?.content
    ).toBe(CONTENT)
  })

  it.each([
    'before_after_storage_v1',
    'before_after_tote_v1',
    'before_after_stand_v1',
    'before_after_miracle-cleaning-paste_v2',
    `${CONTENT}x`,
    CONTENT.toUpperCase(),
    'tool_cta',
    'https://www.tiktok.com/@a/video/1?x=1',
  ])('strips %s but keeps the rest of the touch', (content) => {
    const touch = parseGrowthTouch(
      q({
        utm_campaign: GROWTH_CAMPAIGN,
        utm_source: 'x',
        utm_medium: 'organic_social',
        utm_content: content,
      })
    )
    expect(touch).toEqual({
      campaign: GROWTH_CAMPAIGN,
      source: 'x',
      medium: 'organic_social',
      content: '',
    })
  })

  it('only records the experiment campaign', () => {
    expect(
      parseGrowthTouch(q({ utm_campaign: 'other', utm_content: CONTENT }))
    ).toBeNull()
    expect(parseGrowthTouch(q({ utm_content: CONTENT }))).toBeNull()
    expect(parseGrowthTouch('')).toBeNull()
  })
})

describe('utm_source / utm_medium cleaning', () => {
  it('keeps inbound values as-is when already clean (no enum mapping)', () => {
    const touch = parseGrowthTouch(
      q({
        utm_campaign: GROWTH_CAMPAIGN,
        utm_source: 'Discord',
        utm_medium: 'referral',
      })
    )
    expect(touch?.source).toBe('Discord')
    expect(touch?.medium).toBe('referral')
  })

  it('strips unsafe characters and caps the length', () => {
    expect(cleanUtmValue('  x<script>alert(1)</script> ')).toBe(
      'xscriptalert1script'
    )
    expect(cleanUtmValue('news.example.com')).toBe('news.example.com')
    expect(cleanUtmValue('小红书 xhs')).toBe('xhs')
    expect(cleanUtmValue('a'.repeat(200))).toHaveLength(UTM_VALUE_MAX_LENGTH)
    expect(cleanUtmValue('https://t.co/a?b=c')).toBe('httpst.coabc')
  })
})

describe('empty source stays empty', () => {
  it('leaves source and medium empty when the visit has none', () => {
    const touch = parseGrowthTouch(q({ utm_campaign: GROWTH_CAMPAIGN }))
    expect(touch).toEqual({
      campaign: GROWTH_CAMPAIGN,
      source: '',
      medium: '',
      content: '',
    })
  })

  it('never invents unknown/direct and reports source as absent', () => {
    captureFirstTouch(q({ utm_campaign: GROWTH_CAMPAIGN, utm_source: '<>' }))
    const payload = buildToolCtaClick(click)
    expect(payload?.acq_source).toBe('')
    expect(payload?.acq_medium).toBe('')
    expect(payload?.acq_source_present).toBe(false)
    expect(JSON.stringify(payload)).not.toMatch(/unknown|direct/)
  })

  it('leaves every acq field empty without any first touch', () => {
    expect(buildToolCtaClick(click)).toMatchObject({
      acq_campaign: '',
      acq_source: '',
      acq_medium: '',
      acq_content: '',
      acq_source_present: false,
    })
  })
})

describe('first touch', () => {
  const first = q({
    utm_campaign: GROWTH_CAMPAIGN,
    utm_source: 'x',
    utm_medium: 'organic_social',
    utm_content: CONTENT,
  })

  it('is not replaced by a later tagged visit', () => {
    captureFirstTouch(first)
    captureFirstTouch(
      q({
        utm_campaign: GROWTH_CAMPAIGN,
        utm_source: 'tiktok',
        utm_medium: 'dm',
      })
    )
    expect(readFirstTouch()).toEqual({
      campaign: GROWTH_CAMPAIGN,
      source: 'x',
      medium: 'organic_social',
      content: CONTENT,
    })
  })

  it('is not rewritten by in-site CTA clicks', () => {
    captureFirstTouch(first)
    const before = window.localStorage.getItem(FIRST_TOUCH_STORAGE_KEY)
    for (const cta_id of TOOL_CTA_IDS) trackToolCtaClick({ ...click, cta_id })
    expect(window.localStorage.getItem(FIRST_TOUCH_STORAGE_KEY)).toBe(before)
    expect(tracker.mock.calls[0][1]).toMatchObject({
      acq_campaign: GROWTH_CAMPAIGN,
      acq_source: 'x',
      acq_medium: 'organic_social',
      acq_content: CONTENT,
      acq_source_present: true,
    })
  })

  it('does not store untagged visits, so a later tagged visit still counts', () => {
    expect(captureFirstTouch('?utm_source=x')).toBeNull()
    expect(window.localStorage.getItem(FIRST_TOUCH_STORAGE_KEY)).toBeNull()
    captureFirstTouch(first)
    expect(readFirstTouch()?.source).toBe('x')
  })

  it('re-applies the rules to a tampered snapshot', () => {
    window.localStorage.setItem(
      FIRST_TOUCH_STORAGE_KEY,
      JSON.stringify({
        campaign: GROWTH_CAMPAIGN,
        source: 'x<b>',
        medium: 'dm',
        content: 'before_after_storage_v1',
      })
    )
    expect(readFirstTouch()).toEqual({
      campaign: GROWTH_CAMPAIGN,
      source: 'xb',
      medium: 'dm',
      content: '',
    })
  })
})

describe('tool_cta_click', () => {
  it.each(TOOL_CTA_IDS)(
    'reports cta_id %s with the contract fields',
    (cta_id) => {
      trackToolCtaClick({ ...click, cta_id })
      expect(tracker).toHaveBeenCalledTimes(1)
      const [name, payload] = tracker.mock.calls[0]
      expect(name).toBe(TOOL_CTA_CLICK_EVENT)
      expect(Object.keys(payload).sort()).toEqual(
        [
          'acq_campaign',
          'acq_content',
          'acq_medium',
          'acq_source',
          'acq_source_present',
          'cta_id',
          'cta_variant',
          'locale',
          'placement',
          'tool',
        ].sort()
      )
      expect(payload.cta_id).toBe(cta_id)
    }
  )

  it('lists exactly the five contract cta_ids', () => {
    expect([...TOOL_CTA_IDS]).toEqual([
      'recreate_steps',
      'pricing',
      'studio_fallback',
      'workbench_recreate',
      'desktop_download',
    ])
  })

  it.each([
    { cta_id: 'tool_cta' },
    { cta_id: 'start_free' },
    { cta_variant: 'strong' },
    { placement: 'hero' },
    { tool: 'audio' },
    { locale: 'ja' },
  ])('drops the event for an out-of-enum field %o', (bad) => {
    trackToolCtaClick({ ...click, ...bad } as unknown as ToolCtaClick)
    expect(tracker).not.toHaveBeenCalled()
  })

  it('does not copy extra keys such as a pasted link', () => {
    trackToolCtaClick({
      ...click,
      url: 'https://www.tiktok.com/@a/video/1',
      utm_source: 'internal',
    } as unknown as ToolCtaClick)
    const payload = tracker.mock.calls[0][1]
    expect(payload).not.toHaveProperty('url')
    expect(payload).not.toHaveProperty('utm_source')
    expect(JSON.stringify(payload)).not.toContain('tiktok.com')
  })

  it('throttles the same cta_id for one second, per cta_id', () => {
    vi.useFakeTimers()
    vi.setSystemTime(10_000)
    trackToolCtaClick(click)
    trackToolCtaClick(click)
    trackToolCtaClick({ ...click, cta_id: 'desktop_download' })
    expect(tracker).toHaveBeenCalledTimes(2)
    vi.setSystemTime(10_000 + TOOL_CTA_THROTTLE_MS)
    trackToolCtaClick(click)
    expect(tracker).toHaveBeenCalledTimes(3)
  })

  describe('without a global tracker', () => {
    beforeEach(() => {
      delete (globalThis as { __OMNIMUX_TRACK__?: unknown }).__OMNIMUX_TRACK__
    })
    afterEach(() => {
      vi.unstubAllEnvs()
    })

    it('reports nothing and stays silent in a production build', () => {
      vi.stubEnv('DEV', false)
      vi.stubEnv('PROD', true)
      const debug = vi.spyOn(console, 'debug').mockImplementation(() => {})
      expect(() => trackToolCtaClick(click)).not.toThrow()
      expect(tracker).not.toHaveBeenCalled()
      expect(debug).not.toHaveBeenCalled()
    })

    it('only logs to console.debug in development, still without reporting', () => {
      vi.stubEnv('DEV', true)
      vi.stubEnv('PROD', false)
      const debug = vi.spyOn(console, 'debug').mockImplementation(() => {})
      trackToolCtaClick(click)
      expect(tracker).not.toHaveBeenCalled()
      expect(debug).toHaveBeenCalledTimes(1)
      expect(debug).toHaveBeenCalledWith(
        '[growth-analytics]',
        TOOL_CTA_CLICK_EVENT,
        expect.objectContaining({ cta_id: 'workbench_recreate' })
      )
    })
  })

  it('never throws when the tracker throws', () => {
    tracker.mockImplementation(() => {
      throw new Error('boom')
    })
    expect(() => trackToolCtaClick(click)).not.toThrow()
  })
})
