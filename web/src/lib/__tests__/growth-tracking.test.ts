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
  it.each([
    ['Discord', 'referral'],
    ['tel2026', 'telco_campaign'],
    ['teladext2026', 'tel20261122-referral'],
    ['2026-11-22', 'release-v27.11.5'],
    ['2026.11.22', '20261122-v27.11.5'],
    ['2026_11_22-int42', 'v27.11.5-int42'],
    ['20261122-x', 'v27.11.5-poste'],
    ['2028-02-29-referral', 'release-v1234.5678.90-build'],
    ['campaign12345678901234567890', 'f47ac10b-58cc-4372-a567-000000000001'],
    ['paid-social_2026', 'news.example.com'],
    ['v2026.11.22-1', 'v2026.11.22-rc.1'],
    ['catalog-4.1-2025-04-14', 'catalog-5.2-2025-12-11'],
    ['catalog-5-2025-08-07', 'v2026.11.22-12345'],
  ])('keeps %s / %s as open channel labels', (source, medium) => {
    const touch = parseGrowthTouch(
      q({
        utm_campaign: GROWTH_CAMPAIGN,
        utm_source: source,
        utm_medium: medium,
      })
    )
    expect(touch).toMatchObject({ source, medium })
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

describe('personal values in tool attribution', () => {
  it.each([
    '192.0.2.44',
    '2001:db8::55',
    '[::ffff:192.0.2.44]:443',
    '202-555-0134',
    '+1 (202) 555-0134',
    '+1 (202) 555-0138 ext. (4-2)',
    'tel:+1-202-555-0134;ext=42',
    'tel:55501;phone-context=pbx.example.test',
    '202–555–0134',
    'sample@example.test',
  ])('omits %s without losing the event or an adjacent label', (value) => {
    captureFirstTouch(
      q({
        utm_campaign: GROWTH_CAMPAIGN,
        utm_source: value,
        utm_medium: 'paid-social_2026',
      })
    )

    expect(buildToolCtaClick(click)).toMatchObject({
      ...click,
      acq_campaign: GROWTH_CAMPAIGN,
      acq_source: '',
      acq_medium: 'paid-social_2026',
      acq_source_present: false,
    })
  })

  it.each([
    '202.555.0134',
    '1202555-0138ext.4-2',
    'tel12025550134',
    'tel1-202-555-0134ext42',
    'tel1-202-555-0134isub42',
    'tel1-202-555-0138npdi',
    'tel12025550138rn12025550100npdi',
    'tel1-202-555-0138foobar',
    'tel12025550138foobar',
    'tel1-202-555-0138ext-.',
    'tel55501phone-contextpbx.example.test',
    'tel1A2Bphone-contextpbx.example.test',
    'telphone-contextpbx.example.test',
    'tel1-202-555-0134ext-42',
    'tel5550142extn.42',
    'tel5550142poste42',
    'tel5550142x',
  ])('omits saved phone-shaped medium %s and retains the source', (medium) => {
    window.localStorage.setItem(
      FIRST_TOUCH_STORAGE_KEY,
      JSON.stringify({
        campaign: GROWTH_CAMPAIGN,
        source: 'Discord',
        medium,
        content: CONTENT,
      })
    )

    expect(buildToolCtaClick(click)).toMatchObject({
      acq_source: 'Discord',
      acq_medium: '',
      acq_content: CONTENT,
      acq_source_present: true,
    })
  })
})

describe.each(['source', 'medium'] as const)(
  'phone-shaped values in utm_%s',
  (field) => {
    it.each([
      ['+1 (202) 555-0142 extn. 42', '1202555-0142extn.42'],
      ['+1 (202) 555-0142 poste 42', '1202555-0142poste42'],
      ['+1 (202) 555-0142 extn. ٤٢', '1202555-0142extn.'],
      ['+1 (202) 555-0142 poste ٤٢', '1202555-0142poste'],
      ['+1 (202) 555-0142 poste n° 42', '1202555-0142posten42'],
      ['+1 (202) 555-0142 anexo n.º ४२', '1202555-0142anexon.'],
      ['+1 (202) 555-0142 x٤٢', '1202555-0142x'],
      ['+1 (202) 555-0142, ext: 42#', '1202555-0142ext42'],
      ['+1 (202) 555-0142 extensión: 42', '1202555-0142extensin42'],
      ['+1 (202) 555-0142 extensão: 42', '1202555-0142extenso42'],
      ['+1 (202) 555-0142 XTn. 42', '1202555-0142XTn.42'],
      ['+1 (202) 555-0142 int. 42', '1202555-0142int.42'],
      ['+1 (202) 555-0142 interno 42', '1202555-0142interno42'],
      ['+1 (202) 555-0142 ramal 42', '1202555-0142ramal42'],
      ['+1 (202) 555-0142 anexo 42', '1202555-0142anexo42'],
      ['+1 (202) 555-0142 dob. 42', '1202555-0142dob.42'],
      ['+1 (202) 555-0142 доб. 42', '1202555-0142.42'],
      ['+1 (202) 555-0142 x.42', '1202555-0142x.42'],
      ['+1 (202) 555-0142 #42', '1202555-014242'],
      ['+1 (202) 555-0142 ｅｘｔｎ．４２', '1202555-0142'],
      ['+1 (202) 555-0142 poste numéro 42', '1202555-0142postenumro42'],
      ['Tél. +1 (202) 555-0142 poste 42', 'Tl.1202555-0142poste42'],
      [
        'Durchwahl (202) 555-0142 Anschluss 42',
        'Durchwahl202555-0142Anschluss42',
      ],
      ['Móvil: 202_555_0142 extensión 42', 'Mvil202_555_0142extensin42'],
      ['电话 202 555-0142 内线 42 callback', '202555-014242callback'],
      ['Phone: 𝟚02-555-0142 switchboard 42', 'Phone02-555-0142switchboard42'],
      ['Call 123-456-789', 'Call123-456-789'],
      ['Desk 1234567890123456789', 'Desk1234567890123456789'],
      ['555-0142 poste numéro 42', '555-0142postenumro42'],
      ['Tél. 555-0142', 'Tl.555-0142'],
      ['Tél. 5555-0142 poste numéro 42', 'Tl.5555-0142postenumro42'],
      ['2026-02-31', '2026-02-31'],
      ['2026-11-22-referral-202-555-0142', '2026-11-22-referral-202-555-0142'],
      ['release-v32.4.7-202-555-0142', 'release-v32.4.7-202-555-0142'],
      [
        'release-v1234.5678.90 - 202-555-0142',
        'release-v1234.5678.90-202-555-0142',
      ],
      ['2026-11-22 _ 011-44-20-7946-0958', '2026-11-22_011-44-20-7946-0958'],
      ['011-44-20-7946-0958 - 2026-11-22', '011-44-20-7946-0958-2026-11-22'],
      ['v2026.11.22 - 011-44-20-7946-0958', 'v2026.11.22-011-44-20-7946-0958'],
      [
        'v2026.11.22-12345 _ 011-44-20-7946-0958',
        'v2026.11.22-12345_011-44-20-7946-0958',
      ],
      [
        'v2026.11.22-123456 _ 011-44-20-7946-0958',
        'v2026.11.22-123456_011-44-20-7946-0958',
      ],
      [
        'catalog-4.1-2025-04-14-202-555-0142',
        'catalog-4.1-2025-04-14-202-555-0142',
      ],
    ])('clears %s on arrival and from the old first touch', (raw, saved) => {
      const adjacent = { source: 'Discord', medium: 'organic_social' }
      const expected = {
        ...click,
        acq_campaign: GROWTH_CAMPAIGN,
        acq_content: CONTENT,
        acq_source: field === 'source' ? '' : adjacent.source,
        acq_medium: field === 'medium' ? '' : adjacent.medium,
        acq_source_present: field !== 'source',
      }

      captureFirstTouch(
        q({
          utm_campaign: GROWTH_CAMPAIGN,
          utm_content: CONTENT,
          utm_source: adjacent.source,
          utm_medium: adjacent.medium,
          [`utm_${field}`]: raw,
        })
      )
      const arrival = buildToolCtaClick(click)

      window.localStorage.setItem(
        FIRST_TOUCH_STORAGE_KEY,
        JSON.stringify({
          campaign: GROWTH_CAMPAIGN,
          content: CONTENT,
          ...adjacent,
          [field]: saved,
        })
      )
      expect({ arrival, revisit: buildToolCtaClick(click) }).toEqual({
        arrival: expected,
        revisit: expected,
      })
    })
  }
)

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
