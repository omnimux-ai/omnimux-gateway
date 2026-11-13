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
import { describe, expect, test } from 'vitest'

import type { PricingData, PricingModel } from '../../types'
import { transformPricingDataToCards } from '../transform'

function makeModel(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'demo-model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    ...overrides,
  }
}

function makePricingData(
  models: PricingModel[],
  overrides: Partial<PricingData> = {}
): PricingData {
  return {
    success: true,
    data: models,
    vendors: [
      {
        id: 10,
        name: 'OpenAI',
        icon: 'openai.svg',
      },
      {
        id: 20,
        name: 'Anthropic',
        icon: 'anthropic.svg',
      },
    ],
    group_ratio: { default: 1 },
    usable_group: { default: 'Default' },
    supported_endpoint: {},
    auto_groups: [],
    pricing_version: 'pv-test',
    ...overrides,
  }
}

describe('transformPricingDataToCards — boundaries', () => {
  test.each([
    {
      name: 'jina-reader-v1',
      description: 'Custom reader',
      expected: 'Custom reader',
    },
    {
      name: 'jina-reader-v1',
      description: 'undefined',
      expected:
        'Jina Reader 网页内容提取与搜索（将任意 URL 转换为大模型友好结构化 Markdown 文本）',
    },
    {
      name: 'jina-reader-v1',
      description: '',
      expected:
        'Jina Reader 网页内容提取与搜索（将任意 URL 转换为大模型友好结构化 Markdown 文本）',
    },
    {
      name: 'jina-reader-v1',
      description: undefined,
      expected:
        'Jina Reader 网页内容提取与搜索（将任意 URL 转换为大模型友好结构化 Markdown 文本）',
    },
    { name: 'other-model', description: 'undefined', expected: 'undefined' },
    { name: 'other-model', description: '', expected: '' },
  ])(
    'preserves the description fallback for $name / $description',
    ({ name, description, expected }) => {
      const cards = transformPricingDataToCards(
        makePricingData([makeModel({ model_name: name, description })])
      )
      expect(cards[0]?.rawModel.description).toBe(expected)
    }
  )

  test('returns empty array for undefined / empty pricing data', () => {
    expect(transformPricingDataToCards(undefined)).toEqual([])
    expect(
      transformPricingDataToCards(makePricingData([]), {
        price: 1,
        usd_exchange_rate: 6.8,
      })
    ).toEqual([])
  })

  test('skips models with empty model_name', () => {
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({ model_name: '' }),
        makeModel({ id: 2, model_name: 'ok-model', tags: 'Text Generation' }),
      ])
    )
    expect(cards).toHaveLength(1)
    expect(cards[0]?.modelId).toBe('ok-model')
  })
})

describe('transformPricingDataToCards — per-request (quota_type=1)', () => {
  test('image model uses /pic and converts USD → Credits (×10)', () => {
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          id: 101,
          model_name: 'flux-pro',
          vendor_id: 10,
          quota_type: 1,
          model_price: 0.05,
          tags: 'Image Generation',
        }),
      ]),
      { price: 1, usd_exchange_rate: 6.8 }
    )

    expect(cards).toHaveLength(1)
    const card = cards[0]
    expect(card).toBeDefined()
    if (!card) return
    expect(card.category).toBe('image')
    expect(card.provider).toBe('OpenAI')
    expect(card.vendorIcon).toBe('openai.svg')
    expect(card.name).toBe('flux-pro')
    expect(card.tiers).toHaveLength(1)
    expect(card.tiers[0]?.spec).toBe('Per request')
    expect(card.tiers[0]?.unitSuffix).toBe('/pic')
    // 0.05 USD * group_ratio(1) * 10 = 0.5 Credits
    expect(card.tiers[0]?.credits).toBeCloseTo(0.5)
    expect(card.tiers[0]?.officialUsd).toBeUndefined()
    expect(card.tiers[0]?.savingPercent).toBeUndefined()
  })

  test('video / audio request units are /sec; social uses /gen', () => {
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          id: 1,
          model_name: 'kling-v1',
          quota_type: 1,
          model_price: 0.1,
          tags: 'Video Generation',
        }),
        makeModel({
          id: 2,
          model_name: 'whisper-1',
          quota_type: 1,
          model_price: 0.02,
          tags: 'Audio Generation',
        }),
        makeModel({
          id: 3,
          model_name: 'tiktok-search',
          quota_type: 1,
          model_price: 0.01,
          tags: 'social-data',
        }),
      ])
    )

    expect(cards.map((c) => c.tiers[0]?.unitSuffix)).toEqual([
      '/sec',
      '/sec',
      '/gen',
    ])
    expect(cards.map((c) => c.category)).toEqual(['video', 'audio', 'social'])
  })

  test('applies group_ratio to per-request price', () => {
    const cards = transformPricingDataToCards(
      makePricingData(
        [
          makeModel({
            model_name: 'img-x',
            quota_type: 1,
            model_price: 1,
            tags: 'Image Generation',
            enable_groups: ['default', 'vip'],
          }),
        ],
        { group_ratio: { default: 1, vip: 0.5 } }
      ),
      undefined,
      { selectedGroup: 'vip' }
    )

    // 1 USD * 0.5 * 10 = 5 Credits
    expect(cards[0]?.tiers[0]?.credits).toBeCloseTo(5)
  })
})

describe('transformPricingDataToCards — token (quota_type=0)', () => {
  test('emits Input / Output / Cache rows with /M and correct Credits', () => {
    // Input USD/1M = model_ratio * 2 * group = 0.5 * 2 * 1 = 1
    // Output = 1 * completion_ratio(2) = 2
    // Cache = 1 * cache_ratio(0.1) = 0.1
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          id: 7,
          model_name: 'gpt-4o',
          vendor_id: 10,
          quota_type: 0,
          model_ratio: 0.5,
          completion_ratio: 2,
          cache_ratio: 0.1,
          tags: 'Text Generation',
        }),
      ])
    )

    const card = cards[0]
    expect(card).toBeDefined()
    if (!card) return
    expect(card.category).toBe('text')
    expect(card.tiers.map((t) => t.spec)).toEqual(['Input', 'Output', 'Cache'])
    expect(card.tiers.every((t) => t.unitSuffix === '/M')).toBe(true)
    expect(card.tiers[0]?.credits).toBeCloseTo(10) // $1 → 10 Credits
    expect(card.tiers[1]?.credits).toBeCloseTo(20) // $2 → 20 Credits
    expect(card.tiers[2]?.credits).toBeCloseTo(1) // $0.1 → 1 Credit
  })

  test('falls back to input_modalities when tags are absent', () => {
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          model_name: 'vision-x',
          quota_type: 0,
          model_ratio: 1,
          completion_ratio: 1,
          input_modalities: ['image', 'text'],
        }),
      ])
    )
    expect(cards[0]?.category).toBe('image')
  })
})

describe('transformPricingDataToCards — tiered_expr', () => {
  test('expands billing_expr tiers into Input/Output SPEC rows', () => {
    const expr =
      'len <= 32 ? tier("≤32K", p * 3 + c * 15) : tier(">32K", p * 6 + c * 30)'
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          id: 9,
          model_name: 'claude-opus',
          vendor_id: 20,
          quota_type: 0,
          billing_mode: 'tiered_expr',
          billing_expr: expr,
          tags: 'Text Generation',
        }),
      ])
    )

    const card = cards[0]
    expect(card).toBeDefined()
    if (!card) return
    expect(card.provider).toBe('Anthropic')
    expect(card.tiers.length).toBeGreaterThanOrEqual(2)
    expect(card.tiers.some((t) => t.spec.includes('≤32K'))).toBe(true)
    expect(card.tiers.some((t) => t.spec.includes('>32K'))).toBe(true)
    // First tier input: $3/M → 30 Credits
    const firstInput = card.tiers.find((t) => t.spec.includes('≤32K · Input'))
    expect(firstInput?.credits).toBeCloseTo(30)
    const firstOutput = card.tiers.find((t) => t.spec.includes('≤32K · Output'))
    expect(firstOutput?.credits).toBeCloseTo(150)
  })

  test('degrades to Dynamic pricing row when expression cannot be parsed', () => {
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          model_name: 'broken-expr',
          billing_mode: 'tiered_expr',
          billing_expr: 'not-a-valid-tier-expression',
          tags: 'Text Generation',
        }),
      ])
    )

    expect(cards[0]?.tiers).toHaveLength(1)
    expect(cards[0]?.tiers[0]?.spec).toBe('Dynamic pricing')
    expect(cards[0]?.tiers[0]?.note).toMatch(/parse failed/i)
  })
})

describe('transformPricingDataToCards — official / saving', () => {
  test('computes officialCredits and savingPercent when map provides reference', () => {
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          model_name: 'cheap-img',
          quota_type: 1,
          model_price: 0.5,
          tags: 'Image Generation',
        }),
      ]),
      undefined,
      {
        officialPriceMap: { 'cheap-img': 1 },
      }
    )

    const tier = cards[0]?.tiers[0]
    // our $0.5 → 5 Credits; official $1 → 10 Credits; saving 50%
    expect(tier?.credits).toBeCloseTo(5)
    expect(tier?.officialUsd).toBe(1)
    expect(tier?.officialCredits).toBeCloseTo(10)
    expect(tier?.savingPercent).toBe(50)
  })

  test('applies curated ledger official prices for known catalog models', () => {
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          model_name: 'mj-v6.1',
          quota_type: 1,
          model_price: 0.035,
          tags: 'Image Generation',
        }),
        makeModel({
          model_name: 'gpt-4o',
          quota_type: 0,
          model_ratio: 2.5,
          completion_ratio: 4,
          cache_ratio: 0.5,
          tags: 'Text Generation',
        }),
      ])
    )

    const mj = cards.find((card) => card.modelId === 'mj-v6.1')
    expect(mj?.tiers[0]?.officialUsd).toBe(0.05)
    expect(mj?.tiers[0]?.savingPercent).toBe(30)

    const gpt = cards.find((card) => card.modelId === 'gpt-4o')
    const inputTier = gpt?.tiers.find((tier) => tier.spec === 'Input')
    // model_ratio 2.5 → $5 / 1M sell vs official $2.5 / 1M Input from ledger
    expect(inputTier?.officialUsd).toBe(2.5)
    expect(inputTier?.officialCredits).toBeCloseTo(25)
  })

  test('applies compiled ledger official prices to newly onboarded models', () => {
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          model_name: 'minimax-h3-turbo',
          quota_type: 1,
          model_price: 0.035,
          tags: 'Video Generation',
        }),
        makeModel({
          model_name: 'minimax-h3',
          quota_type: 1,
          model_price: 0.0714,
          tags: 'Video Generation',
        }),
        makeModel({
          model_name: 'gpt-image-2.5',
          quota_type: 1,
          model_price: 0.013072,
          tags: 'Image Generation',
        }),
        makeModel({
          model_name: 'tiktok-search',
          quota_type: 1,
          model_price: 0.002857,
          tags: 'social-data',
        }),
      ])
    )

    const turbo = cards.find((c) => c.modelId === 'minimax-h3-turbo')
    expect(turbo?.tiers[0]?.credits).toBeCloseTo(0.35)
    expect(turbo?.tiers[0]?.officialUsd).toBe(0.04)
    expect(turbo?.tiers[0]?.officialCredits).toBeCloseTo(0.4)
    expect(turbo?.tiers[0]?.savingPercent).toBe(13)

    const h3 = cards.find((c) => c.modelId === 'minimax-h3')
    expect(h3?.tiers[0]?.credits).toBeCloseTo(0.714)
    expect(h3?.tiers[0]?.officialUsd).toBe(0.08)
    expect(h3?.tiers[0]?.officialCredits).toBeCloseTo(0.8)
    expect(h3?.tiers[0]?.savingPercent).toBe(11)

    const gptImg = cards.find((c) => c.modelId === 'gpt-image-2.5')
    expect(gptImg?.tiers[0]?.officialUsd).toBe(0.04)
    expect(gptImg?.tiers[0]?.savingPercent).toBe(67)

    const tiktok = cards.find((c) => c.modelId === 'tiktok-search')
    expect(tiktok?.tiers[0]?.officialUsd).toBeUndefined()
    expect(tiktok?.tiers[0]?.savingPercent).toBeUndefined()
  })

  test('handles tiered_expr fixedPrice request models correctly without Price unavailable', () => {
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          model_name: 'minimax-h3-task',
          billing_mode: 'tiered_expr',
          billing_expr: 'tier("request", fixed(0.3781))',
          tags: 'Video Generation',
          quota_type: 1,
        }),
        makeModel({
          model_name: 'seedance-2-5-task',
          billing_mode: 'tiered_expr',
          billing_expr: 'tier("request", fixed(0.558824))',
          tags: 'Video Generation',
          quota_type: 1,
        }),
        makeModel({
          model_name: 'seedance-2-0-task',
          billing_mode: 'tiered_expr',
          billing_expr: 'tier("request", fixed(0.294118))',
          tags: 'Video Generation',
          quota_type: 1,
        }),
        makeModel({
          model_name: 'gpt-image-2.5',
          billing_mode: 'tiered_expr',
          billing_expr: 'tier("image", fixed(0.013072))',
          tags: 'Image Generation',
          quota_type: 1,
        }),
        makeModel({
          model_name: 'tiktok-search',
          billing_mode: 'tiered_expr',
          billing_expr: 'tier("request", fixed(0.002857))',
          tags: 'social-data',
          quota_type: 1,
        }),
      ])
    )

    const h3Task = cards.find((c) => c.modelId === 'minimax-h3-task')
    expect(h3Task?.tiers[0]?.spec).toBe('Per request')
    expect(h3Task?.tiers[0]?.credits).toBeCloseTo(3.781)
    expect(h3Task?.tiers[0]?.unitSuffix).toBe('/task')
    expect(h3Task?.tiers[0]?.note).toBeUndefined()
    expect(h3Task?.tiers[0]?.officialUsd).toBe(0.58)
    expect(h3Task?.tiers[0]?.officialCredits).toBeCloseTo(5.8)
    expect(h3Task?.tiers[0]?.savingPercent).toBe(35)

    const s25 = cards.find((c) => c.modelId === 'seedance-2-5-task')
    expect(s25?.tiers[0]?.spec).toBe('Per request')
    expect(s25?.tiers[0]?.credits).toBeCloseTo(5.588)
    expect(s25?.tiers[0]?.unitSuffix).toBe('/task')
    expect(s25?.tiers[0]?.note).toBeUndefined()
    expect(s25?.tiers[0]?.officialUsd).toBe(3.088)
    expect(s25?.tiers[0]?.officialCredits).toBeCloseTo(30.88)
    expect(s25?.tiers[0]?.savingPercent).toBe(82)

    const s20 = cards.find((c) => c.modelId === 'seedance-2-0-task')
    expect(s20?.tiers[0]?.spec).toBe('Per request')
    expect(s20?.tiers[0]?.credits).toBeCloseTo(2.941)
    expect(s20?.tiers[0]?.unitSuffix).toBe('/task')
    expect(s20?.tiers[0]?.note).toBeUndefined()
    expect(s20?.tiers[0]?.officialUsd).toBe(0.993)
    expect(s20?.tiers[0]?.officialCredits).toBeCloseTo(9.93)
    expect(s20?.tiers[0]?.savingPercent).toBe(70)

    const gpt = cards.find((c) => c.modelId === 'gpt-image-2.5')
    expect(gpt?.tiers[0]?.spec).toBe('Per request')
    expect(gpt?.tiers[0]?.credits).toBeCloseTo(0.131)
    expect(gpt?.tiers[0]?.unitSuffix).toBe('/pic')
    expect(gpt?.tiers[0]?.note).toBeUndefined()
    expect(gpt?.tiers[0]?.officialUsd).toBe(0.04)
    expect(gpt?.tiers[0]?.savingPercent).toBe(67)

    const tiktok = cards.find((c) => c.modelId === 'tiktok-search')
    expect(tiktok?.tiers[0]?.spec).toBe('Per request')
    expect(tiktok?.tiers[0]?.credits).toBeCloseTo(0.029)
    expect(tiktok?.tiers[0]?.unitSuffix).toBe('/gen')
    expect(tiktok?.tiers[0]?.note).toBeUndefined()
    expect(tiktok?.tiers[0]?.officialUsd).toBeUndefined()
  })

  test('keeps savingPercent 0 when sell price matches official (Official 1:1)', () => {
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          model_name: 'same-price-img',
          quota_type: 1,
          model_price: 0.04,
          tags: 'Image Generation',
        }),
      ]),
      undefined,
      {
        officialPriceMap: { 'same-price-img': 0.04 },
      }
    )

    expect(cards[0]?.tiers[0]?.officialUsd).toBe(0.04)
    expect(cards[0]?.tiers[0]?.savingPercent).toBe(0)
  })

  test('prefers ledger explicit savingPercent 0 over float-derived drift (DeepSeek)', () => {
    // deepseek-r1 ledger marks savingPercent: 0 (Official 1:1).
    // Use a slightly off sell price so pure float derivation would not be 0.
    const cards = transformPricingDataToCards(
      makePricingData([
        makeModel({
          model_name: 'deepseek-r1',
          quota_type: 1,
          model_price: 0.053, // vs official primary 0.55 would be nonsense for request;
          // Use request path with official map primary? Ledger resolves via
          // resolveOfficialUsdForTier → primary usd 0.55 for non-token specs.
          tags: 'Text Generation',
        }),
      ])
    )

    const tier = cards[0]?.tiers[0]
    expect(tier?.officialUsd).toBe(0.55)
    // Without explicit preference this would round to ~90% OFF; ledger forces 0.
    expect(tier?.savingPercent).toBe(0)
  })
})

describe('Sunburst native fixed image quote', () => {
  const model = makeModel({
    model_name: 'gpt-image-2.5-sunburst',
    quota_type: 1,
    model_price: 999,
    model_ratio: 999,
    billing_mode: 'tiered_expr',
    billing_expr: 'tier("image", fixed(0.013072)) * image_count',
    enable_groups: ['default'],
    tags: 'Image Generation',
  })
  test.each([
    [1, 0.13072],
    [0.9, 0.11764],
    [0.8, 0.10458],
    [0.112455, 0.0147],
  ])(
    'rounds the single-image quota before showing credits at ratio %s',
    (ratio, credits) => {
      const cards = transformPricingDataToCards(
        makePricingData([model], { group_ratio: { default: ratio } })
      )
      expect(cards[0]?.tiers[0]?.credits).toBeCloseTo(credits, 12)
      expect(cards[0]?.tiers[0]?.unitSuffix).toBe('/pic')
      expect(cards[0]?.tiers[0]?.officialUsd).toBeUndefined()
      expect(cards[0]?.tiers[0]?.savingPercent).toBeUndefined()
    }
  )
  test.each([
    '',
    'tier("image", ???)',
    'tier("image", fixed(0.01))',
    'tier("tokens", p * 2)',
    'true ? tier("image", fixed(0.01)) * image_count : tier("image", fixed(0.02)) * image_count',
  ])(
    'never presents a damaged or noncanonical image expression as a legacy or free quote: %s',
    (billing_expr) => {
      const cards = transformPricingDataToCards(
        makePricingData([{ ...model, billing_expr }])
      )
      expect(cards[0]?.tiers).toEqual([])
    }
  )
  test('retains an explicitly free fixed image service', () => {
    const cards = transformPricingDataToCards(
      makePricingData([
        { ...model, billing_expr: 'tier("image", fixed(0)) * image_count' },
      ])
    )
    expect(cards[0]?.tiers).toHaveLength(1)
    expect(cards[0]?.tiers[0]?.credits).toBe(0)
    expect(cards[0]?.tiers[0]?.unitSuffix).toBe('/pic')
  })
})

describe('Sunburst configured group quote', () => {
  test.each([Number.NaN, Number.POSITIVE_INFINITY, -1, undefined])(
    'does not turn an invalid or missing group price into a valid default quote: %s',
    (ratio) => {
      const model = makeModel({
        model_name: 'gpt-image-2.5-sunburst',
        billing_mode: 'tiered_expr',
        billing_expr: 'tier("image", fixed(0.01)) * image_count',
        tags: 'Image Generation',
      })
      const cards = transformPricingDataToCards(
        makePricingData([model], {
          group_ratio: ratio === undefined ? {} : { default: ratio },
        })
      )
      expect(cards[0]?.tiers).toEqual([])
    }
  )
})

describe('Sunburst viewer-specific price visibility', () => {
  test('retains the accessible image quote when another enabled group is not available to the viewer', () => {
    const model = makeModel({
      model_name: 'gpt-image-2.5-sunburst',
      billing_mode: 'tiered_expr',
      billing_expr: 'tier("image", fixed(0.1)) * image_count',
      enable_groups: ['gpt-image-2.5-sunburst-image', 'private-image'],
      tags: 'Image Generation',
    })
    const cards = transformPricingDataToCards(
      makePricingData([model], {
        group_ratio: { 'gpt-image-2.5-sunburst-image': 1 },
        usable_group: { 'gpt-image-2.5-sunburst-image': 'Image' },
      })
    )
    expect(cards[0]?.tiers).toEqual([
      {
        spec: 'Per request',
        credits: 1,
        unitSuffix: '/pic',
        officialUsd: undefined,
        officialCredits: undefined,
        savingPercent: undefined,
        note: undefined,
      },
    ])
  })
})

describe('Sunburst missing visible group price', () => {
  const model = makeModel({
    model_name: 'gpt-image-2.5-sunburst',
    billing_mode: 'tiered_expr',
    billing_expr: 'tier("image", fixed(0.1)) * image_count',
    enable_groups: ['gpt-image-2.5-sunburst-image', 'other-image'],
    tags: 'Image Generation',
  })
  test('closes the quote when an accessible image group lacks a price instead of borrowing another group price', () => {
    const cards = transformPricingDataToCards(
      makePricingData([model], {
        group_ratio: { 'other-image': 0.5 },
        usable_group: {
          'gpt-image-2.5-sunburst-image': 'Image',
          'other-image': 'Other',
        },
      })
    )
    expect(cards[0]?.tiers).toEqual([])
  })
  test('allows the priced accessible group when the group without a price is inaccessible', () => {
    const cards = transformPricingDataToCards(
      makePricingData([model], {
        group_ratio: { 'other-image': 0.5 },
        usable_group: { 'other-image': 'Other' },
      })
    )
    expect(cards[0]?.tiers[0]?.credits).toBe(0.5)
  })
  test('never quotes an inaccessible group even if its ratio remains in the payload', () => {
    const cards = transformPricingDataToCards(
      makePricingData([model], {
        group_ratio: { 'gpt-image-2.5-sunburst-image': 1, 'other-image': 0.5 },
        usable_group: { 'gpt-image-2.5-sunburst-image': 'Image' },
      })
    )
    expect(cards[0]?.tiers[0]?.credits).toBe(1)
  })
})
