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

import { readFileSync, readdirSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { createAnalyticsTracker } from '../analytics'
import { setPublicAnalyticsIds } from '../analytics-public-ids'
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
    ['2024-02-29', '2024_02_29'],
    [
      '12345678-1234-4123-8123-123456789012',
      'f47ac10b-58cc-4372-a567-000000000001',
    ],
    ['00000000-0000-0000-0000-000000000000', 'organic_social'],
    ['paid-social_2026', 'news.example.com'],
    ['v2026.11.22-1', 'v2026.11.22-rc.1'],
    ['catalog-4.1-2025-04-14', 'catalog-5.2-2025-12-11'],
    ['catalog-5-2025-08-07', 'v2026.11.22-12345'],
    ['catalog-44.20-20251122', 'catalog-12.2-2025-08-07'],
    ['catalog-123-2025-08-07', 'catalog-123.45.6-2025-08-07'],
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
      ['20261122', '20261122'],
      ['2026/11/22', '20261122'],
      ['2026-11-22-referral-202-555-0142', '2026-11-22-referral-202-555-0142'],
      ['release-v32.4.7-202-555-0142', 'release-v32.4.7-202-555-0142'],
      [
        'release-v1234.5678.90 - 202-555-0142',
        'release-v1234.5678.90-202-555-0142',
      ],
      ['2026-11-22 _ 011-44-20-7946-0958', '2026-11-22_011-44-20-7946-0958'],
      ['011-44-20-7946-0958 - 2026-11-22', '011-44-20-7946-0958-2026-11-22'],
      ['2026-11-22 _ 011-44-20-20251122', '2026-11-22_011-44-20-20251122'],
      ['2026-11-22 - 011-44-20-20251122', '2026-11-22-011-44-20-20251122'],
      ['011-44-20-20251122 - 2026-11-22', '011-44-20-20251122-2026-11-22'],
      [
        '2026-11-22 - 011-44-20-20251122 - 2026-11-23',
        '2026-11-22-011-44-20-20251122-2026-11-23',
      ],
      ['2026.11.22 _ 011-44-20-20251122', '2026.11.22_011-44-20-20251122'],
      ['2026_11_22 _ 011-44-20-20251122', '2026_11_22_011-44-20-20251122'],
      ['011-44-20-20251122', '011-44-20-20251122'],
      ['202-555-20250929', '202-555-20250929'],
      ['Phone 202-555-20250929', 'Phone202-555-20250929'],
      ['Phone 44-20-20251122', 'Phone44-20-20251122'],
      ['Phone-44-20-20251122', 'Phone-44-20-20251122'],
      ['Tél-44-20-20251122', 'Tl-44-20-20251122'],
      ['Fax-44-20-20251122', 'Fax-44-20-20251122'],
      ['Tel-44-20-20251122', 'Tel-44-20-20251122'],
      ['Telephone-44-20-20251122', 'Telephone-44-20-20251122'],
      ['Téléphone-44-20-20251122', 'Tlphone-44-20-20251122'],
      ['Teléfono-44-20-20251122', 'Telfono-44-20-20251122'],
      ['Telephone-20-20251122', 'Telephone-20-20251122'],
      ['Téléphone-44.20-20251122', 'Tlphone-44.20-20251122'],
      ['Teléfono-20-20251122', 'Telfono-20-20251122'],
      ['campaign-44-20-20251122', 'campaign-44-20-20251122'],
      ['catalog-4-20250229', 'catalog-4-20250229'],
      ['Phone 202-555-0142 _ 202-555-0187', 'Phone202-555-0142_202-555-0187'],
      ['20251122-011-44-20-7946-0958', '20251122-011-44-20-7946-0958'],
      ['20251122 011 44 20 7946 0958', '20251122011442079460958'],
      ['(202) 555 0142 (415) 555 0100', '20255501424155550100'],
      ['202-555-0142 202-555-0187', '202-555-0142202-555-0187'],
      ['Phone 202-555-0142 202-555-0187', 'Phone202-555-0142202-555-0187'],
      ['12345678901234567890', '12345678901234567890'],
      ['123456789012345678901', '123456789012345678901'],
      ['campaign12345678901234567890', 'campaign12345678901234567890'],
      ['campaign-1234567890-1234567890', 'campaign-1234567890-1234567890'],
      ['12345_67890', '12345_67890'],
      ['v2026.11.22-123456789', 'v2026.11.22-123456789'],
      ['v1.2.123456789', 'v1.2.123456789'],
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
      expect(buildToolCtaClick(click)).toEqual(expected)
      const firstSnapshot = window.localStorage.getItem(FIRST_TOUCH_STORAGE_KEY)
      expect(JSON.parse(firstSnapshot ?? 'null')).toEqual({
        campaign: GROWTH_CAMPAIGN,
        source: expected.acq_source,
        medium: expected.acq_medium,
        content: CONTENT,
      })

      const oldSnapshot = JSON.stringify({
        campaign: GROWTH_CAMPAIGN,
        content: CONTENT,
        ...adjacent,
        [field]: saved,
      })
      window.localStorage.setItem(FIRST_TOUCH_STORAGE_KEY, oldSnapshot)
      const writeSnapshot = vi.spyOn(window.Storage.prototype, 'setItem')
      try {
        captureFirstTouch('')
        expect(buildToolCtaClick(click)).toEqual(expected)
        expect(window.localStorage.getItem(FIRST_TOUCH_STORAGE_KEY)).toBe(
          oldSnapshot
        )
        expect(writeSnapshot).not.toHaveBeenCalled()
      } finally {
        writeSnapshot.mockRestore()
      }
    })
  }
)

describe('public model IDs in analytics', () => {
  const repoRoot = resolve(
    dirname(fileURLToPath(import.meta.url)),
    '../../../..'
  )
  const contracts = JSON.parse(
    readFileSync(resolve(repoRoot, 'modelcontracts/contracts.json'), 'utf8')
  ) as { models: Record<string, unknown> }
  const relayChannels = resolve(repoRoot, 'relay/channel')
  // The gateway also advertises compatibility models from these literal lists.
  const compatibilityModelIds = readdirSync(relayChannels, {
    encoding: 'utf8',
    recursive: true,
  })
    .filter((path) => path.endsWith('.go') && !path.endsWith('_test.go'))
    .flatMap((path) => {
      const source = readFileSync(resolve(relayChannels, path), 'utf8')
      const list = /^var ModelList = \[\]string\{([^}]*)\}/m.exec(source)?.[1]
      if (!list) return []
      const entries = list.replaceAll(/\/\/[^\n]*/g, '')
      return Array.from(entries.matchAll(/"([^"\n]+)"/g), ([, id]) => id)
    })
  // Compatibility aliases declared in plugin metadata, computed adapter lists,
  // and the pricing fallback, plus archived catalog snapshots.
  const additionalModelIds = [
    'I2V-01',
    'I2V-01-Director',
    'I2V-01-live',
    'MiniMax-H3',
    'MiniMax-Hailuo-02',
    'MiniMax-Hailuo-2.3',
    'MiniMax-Hailuo-2.3-Fast',
    'S2V-01',
    'T2V-01',
    'T2V-01-Director',
    'black-forest-labs/flux-1.1-pro',
    'doubao-seedance-1-0-lite-i2v',
    'doubao-seedance-1-0-lite-t2v',
    'doubao-seedance-1-5-pro-251215',
    'doubao-seedance-2-0-260128',
    'doubao-seedance-2-0-fast-260128',
    'doubao-seedance-2-0-mini-260615',
    'doubao-seedance-2-5-260628',
    'doubao-seedream-4-5-251128',
    'doubao-seedream-5-0-260128',
    'doubao-seedream-5-0-flash-260915',
    'doubao-seedream-5-0-lite-260128',
    'doubao-seedream-5-0-pro-260628',
    'eleven-multilingual-v2',
    'flux-1-pro',
    'flux-1-schnell',
    'gemini-omni-1.1-flash',
    'gpt-image-2.5-async',
    'grok-4.20-0309-non-reasoning',
    'grok-4.20-0309-reasoning',
    'grok-4.20-multi-agent-0309',
    'grok-4.3',
    'grok-build-0.1',
    'grok-composer-2.5-fast',
    'grok-imagine-video-1.5',
    'grok-imagine-video-1.5-lite',
    'image2.5-pro-async',
    'jimeng_vgfm_t2v_l20',
    'kling-v1',
    'kling-v1-6',
    'kling-v1.6-pro',
    'kling-v2-master',
    'ltx2.3-kj',
    'ltx2.3-kj-async',
    'luma-dream-machine',
    'midjourney',
    'minimax-m2.5',
    'minimax-m2.7',
    'minimax-m3',
    'minimax/h3-max',
    'minimax/h3-max-turbo',
    'minimax/h3-max-turbo/image-to-video',
    'minimax/h3-max-turbo/text-to-video',
    'minimax/h3-max/image-to-video',
    'minimax/h3-max/lip-sync/image-to-video',
    'minimax/h3-max/reference-to-video',
    'minimax/h3-max/text-to-video',
    'mj-v6.1',
    'mj_blend',
    'mj_custom_zoom',
    'mj_describe',
    'mj_edits',
    'mj_high_variation',
    'mj_imagine',
    'mj_inpaint',
    'mj_low_variation',
    'mj_modal',
    'mj_pan',
    'mj_reroll',
    'mj_shorten',
    'mj_upload',
    'mj_upscale',
    'mj_variation',
    'mj_video',
    'mj_zoom',
    'nova-canvas-v1:0',
    'nova-lite-v1:0',
    'nova-micro-v1:0',
    'nova-premier-v1:0',
    'nova-pro-v1:0',
    'nova-reel-v1:0',
    'nova-reel-v1:1',
    'nova-sonic-v1:0',
    'qwen-image',
    'qwen-image-2.0',
    'qwen-image-2.0-2026-03-03',
    'qwen-image-2.0-pro',
    'qwen-image-2.0-pro-2026-03-03',
    'qwen-image-2.0-pro-2026-04-22',
    'qwen-image-2.0-pro-2026-06-22',
    'qwen-image-3.0',
    'qwen-image-3.0-pro',
    'qwen-image-edit',
    'qwen-image-edit-max',
    'qwen-image-edit-max-2026-01-16',
    'qwen-image-edit-plus',
    'qwen-image-edit-plus-2025-10-30',
    'qwen-image-edit-plus-2025-12-15',
    'qwen-image-max',
    'qwen-image-max-2025-12-30',
    'qwen-image-plus',
    'qwen-image-plus-2026-01-09',
    'runway-gen3-alpha',
    'suno-v4',
    'suno_lyrics',
    'suno_music',
    'swap_face',
    'vidu1.5',
    'vidu2.0',
    'viduq1',
    'viduq2',
    'volc-voice-clone',
    'wan-2-2-animate',
    'wan2.2-i2v-flash',
    'wan2.2-i2v-plus',
    'wan2.2-kf2v-flash',
    'wan2.2-s2v',
    'wan2.2-t2i-flash',
    'wan2.2-t2i-plus',
    'wan2.2-t2v-plus',
    'wan2.5-i2i-preview',
    'wan2.5-i2v-preview',
    'wan2.5-t2i-preview',
    'wan2.5-t2v-preview',
    'wan2.6-i2v',
    'wan2.6-i2v-flash',
    'wan2.6-i2v-us',
    'wan2.6-image',
    'wan2.6-t2i',
    'wan2.6-t2v',
    'wan2.6-t2v-us',
    'wan2.7-i2v',
    'wan2.7-i2v-2026-04-25',
    'wan2.7-image',
    'wan2.7-image-pro',
    'wan2.7-t2v',
    'wan2.7-t2v-2026-04-25',
    'wan2.7-t2v-2026-06-12',
    'wan3.0-video',
    'wan3.0-video-prime',
    'wanx2.0-t2i-turbo',
    'wanx2.1-i2v-plus',
    'wanx2.1-i2v-turbo',
    'wanx2.1-imageedit',
    'wanx2.1-t2i-plus',
    'wanx2.1-t2i-turbo',
    'wanx2.1-t2v-plus',
    'wanx2.1-t2v-turbo',
    'z-image-turbo',
    'zimage-makeup',
    'zimage-makeup-async',
  ]
  const modelIds = [
    ...new Set([
      ...Object.keys(contracts.models),
      ...compatibilityModelIds,
      ...additionalModelIds,
      'claude-sonnet-4-5-20250929',
      'claude-haiku-4-5-20251001',
      'claude-sonnet-4-20250514',
      'claude-3-5-sonnet-20240620',
      'claude-3-7-sonnet-20250219-thinking',
      'gpt-5-2025-08-07',
      'gpt-4.1-2025-04-14',
      'gpt-5.2-2025-12-11',
    ]),
  ]

  afterEach(() => vi.unstubAllGlobals())

  it.each(modelIds)('keeps the catalog analytics data for %s', (modelId) => {
    vi.stubGlobal(Symbol.for('omnimux.analytics.public-ids'), undefined)
    const send = vi.fn<NonNullable<Window['umami']>['track']>()
    vi.stubGlobal('umami', { track: send })
    setPublicAnalyticsIds('modelId', [modelId])

    createAnalyticsTracker()({
      name: 'pricing_model_id_copy',
      props: { modelId },
      pathname: '/pricing',
      referrer: '',
      timestamp: Date.now(),
    })

    expect(send).toHaveBeenCalledTimes(1)
    const payload = send.mock.calls[0][0]({})
    expect(payload.name).toBe('pricing_model_id_copy')
    // Provider routing syntax starting with @ is excluded by the existing field grammar.
    expect(payload.data).toEqual(modelId.startsWith('@') ? {} : { modelId })
    if (/^[A-Za-z0-9._-]{1,64}$/.test(modelId)) {
      expect(cleanUtmValue(modelId)).toBe(modelId)
    }
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
