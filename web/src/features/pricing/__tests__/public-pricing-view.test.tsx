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
import { screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { beforeAll, beforeEach, describe, expect, test, vi } from 'vitest'

import { PublicPricingView } from '../public-pricing-view'
import type { PricingData, PricingVendor, PublicModelCardItem } from '../types'
import { renderPricing } from './render-pricing'

vi.mock('../components/pricing-download-cta', () => ({
  PricingDownloadCta: () => null,
}))

const forceRefetchMock = vi.fn()

type PricingHookState = {
  rawPricingData?: PricingData
  cards: PublicModelCardItem[]
  vendors: PricingVendor[]
  isLoading: boolean
  isFetching: boolean
  isUsingCache: boolean
  lastUpdated: number | null
  error: Error | null
  forceRefetch: typeof forceRefetchMock
  dataSource: 'network' | 'cache' | 'none'
}

let pricingHookState: PricingHookState

vi.mock('../hooks/use-pricing-data', () => ({
  usePricingData: () => pricingHookState,
}))

vi.mock('@/lib/lobe-icon-lazy', () => ({
  getLobeIcon: (name: string) => <span data-testid='vendor-icon'>{name}</span>,
}))

vi.mock('@/hooks/use-copy-to-clipboard', () => ({
  useCopyToClipboard: () => ({
    copiedText: null,
    copyToClipboard: vi.fn(async () => true),
  }),
}))

function makeCard(
  overrides: Partial<PublicModelCardItem> &
    Pick<
      PublicModelCardItem,
      'id' | 'name' | 'modelId' | 'provider' | 'category'
    >
): PublicModelCardItem {
  return {
    tiers: [
      {
        spec: 'Default',
        credits: 1,
        unitSuffix: '/call',
      },
    ],
    tierCount: 1,
    rawModel: {
      id: 1,
      model_name: overrides.modelId,
      quota_type: 1,
      model_ratio: 1,
      completion_ratio: 1,
      enable_groups: ['default'],
      vendor_id: overrides.provider === 'OpenAI' ? 1 : 2,
    },
    ...overrides,
  }
}

const VENDORS: PricingVendor[] = [
  { id: 1, name: 'OpenAI' },
  { id: 2, name: 'Anthropic' },
]

const CARDS: PublicModelCardItem[] = [
  makeCard({
    id: 'kling-v1-5',
    name: 'Kling v1.5',
    modelId: 'kling-v1-5',
    provider: 'Kuaishou',
    category: 'video',
  }),
  makeCard({
    id: 'gpt-4o',
    name: 'GPT-4o',
    modelId: 'gpt-4o',
    provider: 'OpenAI',
    category: 'text',
  }),
  makeCard({
    id: 'dall-e-3',
    name: 'DALL·E 3',
    modelId: 'dall-e-3',
    provider: 'OpenAI',
    category: 'image',
  }),
  makeCard({
    id: 'claude-sonnet',
    name: 'Claude Sonnet',
    modelId: 'claude-3-5-sonnet',
    provider: 'Anthropic',
    category: 'text',
  }),
]

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    Pricing: 'Pricing',
    'View all {{count}} models and technical specs':
      'View all {{count}} models and technical specs',
    'Pay for what you use': 'Pay for what you use',
    'Every model on OmniMux is pay-as-you-go. No subscription, no seat fees, no minimums — top up credits and call {{count}}+ production models through one API.':
      'Every model on OmniMux is pay-as-you-go. No subscription, no seat fees, no minimums — top up credits and call {{count}}+ production models through one API.',
    'Every model on OmniMux is pay-as-you-go. No subscription, no seat fees, no minimums — top up credits and call production-grade models through one API.':
      'Every model on OmniMux is pay-as-you-go. No subscription, no seat fees, no minimums — top up credits and call production-grade models through one API.',
    'No monthly fee — prepaid credits': 'No monthly fee — prepaid credits',
    'API key without a credit card': 'API key without a credit card',
    'Credits never expire': 'Credits never expire',
    'Failed generations are free': 'Failed generations are free',
    'View model prices': 'View model prices',
    'Enterprise pricing': 'Enterprise pricing',
    'Opening enterprise pricing contact… sales@omnimux.ai':
      'Opening enterprise pricing contact… sales@omnimux.ai',
    'How billing works': 'How billing works',
    'One balance, every model. Metered per call, visible live in the console.':
      'One balance, every model. Metered per call, visible live in the console.',
    'Top up credits': 'Top up credits',
    'Pay with card, Alipay/WeChat, or wire transfer.':
      'Pay with card, Alipay/WeChat, or wire transfer.',
    '1 credit = ${{usd}} USD': '1 credit = ${{usd}} USD',
    'Top up more, pay less': 'Top up more, pay less',
    'Discounts apply to preset amounts only':
      'Discounts apply to preset amounts only',
    Credits: 'Credits',
    'Save {{percent}}%': 'Save {{percent}}%',
    'Call any model': 'Call any model',
    'Video and audio billed per second, images per generation, LLMs per token. Unit prices sit on each model card — no hidden multipliers.':
      'Video and audio billed per second, images per generation, LLMs per token. Unit prices sit on each model card — no hidden multipliers.',
    'One API → {{count}} models': 'One API → {{count}} models',
    'One API → production models': 'One API → production models',
    'Every cent is traceable': 'Every cent is traceable',
    'Usage logs update in milliseconds with tokens, latency, and cost per call. Export by API key, model, and time range.':
      'Usage logs update in milliseconds with tokens, latency, and cost per call. Export by API key, model, and time range.',
    'No minimums · failed calls free': 'No minimums · failed calls free',
    'Showing cached prices that may be outdated. Last synced: {{time}}.':
      'Showing cached prices that may be outdated. Last synced: {{time}}.',
    'Refresh now': 'Refresh now',
    'unknown time': 'unknown time',
    ALL: 'ALL',
    IMAGE: 'IMAGE',
    VIDEO: 'VIDEO',
    LLM: 'LLM',
    AUDIO: 'AUDIO',
    'Model category': 'Model category',
    'Search models, IDs, or providers…': 'Search models, IDs, or providers…',
    'All providers': 'All providers',
    model: 'model',
    models: 'models',
    Never: 'Never',
    'Synced {{time}}': 'Synced {{time}}',
    'Refresh prices': 'Refresh prices',
    Refresh: 'Refresh',
    'Unable to load prices': 'Unable to load prices',
    'The pricing service is temporarily unavailable. Please try again.':
      'The pricing service is temporarily unavailable. Please try again.',
    Retry: 'Retry',
    'No models found': 'No models found',
    'No results for "{{query}}". Try adjusting your search or filters.':
      'No results for "{{query}}". Try adjusting your search or filters.',
    'No models match your current filters.':
      'No models match your current filters.',
    'Clear all filters': 'Clear all filters',
    'Price tiers': 'Price tiers',
    SPEC: 'SPEC',
    'OUR PRICE': 'OUR PRICE',
    'OFFICIAL PRICE': 'OFFICIAL PRICE',
    SAVING: 'SAVING',
    '1 PRICE TIER': '1 PRICE TIER',
    '{{count}} PRICE TIERS': '{{count}} PRICE TIERS',
    'Copy model ID': 'Copy model ID',
    'Copied model ID': 'Copied model ID',
    'Copied to clipboard': 'Copied to clipboard',
    '{{model}} ({{rate}} credits/s)': '{{model}} ({{rate}} credits/s)',
    '{{model}} (from {{rate}} credits/s)':
      '{{model}} (from {{rate}} credits/s)',
    '{{model}} ({{rate}} credits/video)': '{{model}} ({{rate}} credits/video)',
    '{{model}} (from {{rate}} credits/video)':
      '{{model}} (from {{rate}} credits/video)',
    '{{model}} (quoted at run time)': '{{model}} (quoted at run time)',
    'Per video': 'Per video',
    'From {{amount}} credits': 'From {{amount}} credits',
    'From ≈ {{amount}}': 'From ≈ {{amount}}',
    'Quoted at run time': 'Quoted at run time',
    '{{amount}} credits': '{{amount}} credits',
    '{{value}} seconds': '{{value}} seconds',
    'Cost estimator': 'Cost estimator',
    'Roughly how much is a 15-second product video?':
      'Roughly how much is a 15-second product video?',
    'Video duration': 'Video duration',
    'Estimator model': 'Estimator model',
    'Estimated cost': 'Estimated cost',
    'View recommended video model rates': 'View recommended video model rates',
  })
})

describe('PublicPricingView', () => {
  beforeEach(() => {
    forceRefetchMock.mockReset()
    pricingHookState = {
      rawPricingData: {
        success: true,
        data: [
          {
            id: 1,
            model_name: 'kling-v1-5',
            quota_type: 1,
            model_ratio: 1,
            completion_ratio: 1,
            model_price: 0.2,
            enable_groups: ['default'],
            tags: 'Video Generation',
            contract: {
              model_id: 'kling-v1-5',
              family: 'video',
              billing: {
                unit: 'per_second',
                pricing_basis: 'duration_seconds',
                unit_price_usd: 0.2,
              },
            },
          },
        ],
        vendors: VENDORS,
        group_ratio: { default: 1 },
        usable_group: { default: 'Standard' },
        supported_endpoint: {},
        auto_groups: [],
      },
      cards: CARDS,
      vendors: VENDORS,
      isLoading: false,
      isFetching: false,
      isUsingCache: false,
      lastUpdated: Date.UTC(2026, 2, 1, 12, 0, 0),
      error: null,
      forceRefetch: forceRefetchMock,
      dataSource: 'network',
    }
  })

  test('keeps the catalog collapsed by default and preserves model tabs when opened', async () => {
    const user = userEvent.setup()
    renderPricing(<PublicPricingView />)

    expect(
      screen.getByRole('heading', {
        name: 'Pay for what you use — no subscription required',
      })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('heading', {
        name: 'Roughly how much is a 15-second product video?',
      })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: 'Top up more, pay less' })
    ).toBeInTheDocument()
    expect(document.querySelector('#pricing-models')).not.toBeNull()

    const trigger = screen.getByRole('button', { name: /view all 4 models/i })
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(
      screen.queryByRole('tab', { name: /^VIDEO/ })
    ).not.toBeInTheDocument()
    await user.click(trigger)
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    // Tab order: VIDEO, LLM, IMAGE, AUDIO, ALL
    expect(screen.getByRole('tab', { name: /^VIDEO/ })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /^LLM/ })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /^IMAGE/ })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /^AUDIO/ })).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: /^ALL/ })).toBeInTheDocument()

    // Default tab is VIDEO -> Kling v1.5 is visible
    expect(screen.getByText('Kling v1.5')).toBeInTheDocument()

    // Switching to ALL shows all cards
    await user.click(screen.getByRole('tab', { name: /^ALL/ }))
    expect(screen.getByText('GPT-4o')).toBeInTheDocument()
    expect(screen.getByText('DALL·E 3')).toBeInTheDocument()
    expect(screen.getByText('Claude Sonnet')).toBeInTheDocument()
  })

  test('shows the catalog loading state on a video deep link', () => {
    pricingHookState = {
      ...pricingHookState,
      cards: [],
      isLoading: true,
      dataSource: 'none',
    }

    const { container } = renderPricing(
      <PublicPricingView search={{ modality: 'video' }} />
    )

    expect(
      screen.getByRole('heading', {
        name: 'Pay for what you use — no subscription required',
      })
    ).toBeInTheDocument()
    expect(screen.queryByText('GPT-4o')).not.toBeInTheDocument()
    expect(
      container.querySelectorAll('[data-slot="skeleton"]').length
    ).toBeGreaterThan(0)
  })

  test('shows offline banner when serving from cache and refresh triggers forceRefetch', async () => {
    const user = userEvent.setup()
    pricingHookState = {
      ...pricingHookState,
      isUsingCache: true,
      dataSource: 'cache',
      lastUpdated: Date.UTC(2026, 2, 1, 8, 30, 0),
    }

    renderPricing(<PublicPricingView search={{ modality: 'video' }} />)

    expect(
      screen.getByText(/Showing cached prices that may be outdated/i)
    ).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Refresh now' }))
    expect(forceRefetchMock).toHaveBeenCalledTimes(1)
  })

  test('filters cards by modality tab', async () => {
    const user = userEvent.setup()
    renderPricing(<PublicPricingView />)

    await user.click(screen.getByRole('button', { name: /view all 4 models/i }))
    await user.click(screen.getByRole('tab', { name: /^IMAGE/ }))

    expect(screen.getByText('DALL·E 3')).toBeInTheDocument()
    expect(screen.queryByText('GPT-4o')).not.toBeInTheDocument()
    expect(screen.queryByText('Claude Sonnet')).not.toBeInTheDocument()
  })

  test('shows hard error state with retry when fetch fails and no cards exist', async () => {
    const user = userEvent.setup()
    pricingHookState = {
      ...pricingHookState,
      cards: [],
      error: new Error('network down'),
      dataSource: 'none',
    }

    renderPricing(<PublicPricingView search={{ modality: 'video' }} />)

    const alert = screen.getByRole('alert')
    expect(
      within(alert).getByRole('heading', { name: 'Unable to load prices' })
    ).toBeInTheDocument()
    expect(within(alert).getByText('network down')).toBeInTheDocument()

    await user.click(within(alert).getByRole('button', { name: /Retry/i }))
    expect(forceRefetchMock).toHaveBeenCalledTimes(1)
  })

  test('opens a filtered pricing URL with its modality, provider and search applied', () => {
    renderPricing(
      <PublicPricingView
        search={{ modality: 'text', vendor: '1', search: 'gpt' }}
      />
    )

    expect(
      screen.getByRole('button', { name: /view all 4 models/i })
    ).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('tab', { name: /^LLM/ })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(screen.getByText('GPT-4o')).toBeInTheDocument()
    expect(screen.queryByText('Claude Sonnet')).not.toBeInTheDocument()
  })

  test('uses the visible group from a deep link for the video estimate', async () => {
    const user = userEvent.setup()
    const group = 'wan-3.0-720p'
    pricingHookState.rawPricingData = {
      success: true,
      data: [
        {
          id: 7,
          model_name: 'wan-3.0',
          quota_type: 1,
          model_ratio: 1,
          completion_ratio: 1,
          model_price: 0.176471,
          enable_groups: [group],
          tags: 'Video Generation',
          contract: {
            model_id: 'wan-3.0',
            family: 'video',
            billing: {
              unit: 'per_second',
              pricing_basis: 'duration_seconds',
              unit_price_usd: 0.176471,
            },
          },
        },
      ],
      vendors: [],
      group_ratio: { [group]: 0.5 },
      usable_group: { [group]: 'Wan 3.0 720p' },
      supported_endpoint: {},
      auto_groups: [],
    }

    renderPricing(<PublicPricingView search={{ group }} />)
    await user.selectOptions(
      screen.getByRole('combobox', { name: 'Estimator model' }),
      'wan-3.0'
    )
    expect(
      screen.getByRole('option', { name: 'wan-3.0 (0.882 credits/s)' })
    ).toBeInTheDocument()
    expect(screen.getByText('13.2 credits')).toBeInTheDocument()
    expect(screen.getByText('≈ $1.32')).toBeInTheDocument()

    // Decoupled recommended rates table still preserves its curated rows.
    await user.click(
      screen.getByRole('button', { name: 'View recommended video model rates' })
    )
    expect(screen.getByText('Seedance 2.5')).toBeInTheDocument()
  })

  test('shows empty state when filters yield no cards', async () => {
    const user = userEvent.setup()
    renderPricing(<PublicPricingView />)

    await user.click(screen.getByRole('button', { name: /view all 4 models/i }))
    await user.click(screen.getByRole('tab', { name: /^AUDIO/ }))

    expect(screen.getByText('No models found')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Clear all filters' })
    ).toBeInTheDocument()
  })
})
