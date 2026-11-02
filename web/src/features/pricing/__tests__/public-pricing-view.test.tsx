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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { beforeAll, beforeEach, describe, expect, test, vi } from 'vitest'

import type { PublicModelCardItem, PricingVendor } from '../types'
import { PublicPricingView } from '../public-pricing-view'

const forceRefetchMock = vi.fn()

type PricingHookState = {
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

vi.mock('@/lib/lobe-icon', () => ({
  getLobeIcon: (name: string) => (
    <span data-testid='vendor-icon'>{name}</span>
  ),
}))

vi.mock('@/hooks/use-copy-to-clipboard', () => ({
  useCopyToClipboard: () => ({
    copiedText: null,
    copyToClipboard: vi.fn(async () => true),
  }),
}))

function makeCard(
  overrides: Partial<PublicModelCardItem> &
    Pick<PublicModelCardItem, 'id' | 'name' | 'modelId' | 'provider' | 'category'>
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
  })
})

describe('PublicPricingView', () => {
  beforeEach(() => {
    forceRefetchMock.mockReset()
    pricingHookState = {
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

  test('renders hero, billing steps, toolbar, and model cards when data is ready', async () => {
    const user = userEvent.setup()
    render(<PublicPricingView />)

    expect(
      screen.getByRole('heading', { name: 'Pay for what you use' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: 'How billing works' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: 'Top up more, pay less' })
    ).toBeInTheDocument()
    expect(document.querySelector('#pricing-models')).not.toBeNull()

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

  test('shows loading skeleton while pricing data is loading', () => {
    pricingHookState = {
      ...pricingHookState,
      cards: [],
      isLoading: true,
      dataSource: 'none',
    }

    const { container } = render(<PublicPricingView />)

    expect(
      screen.getByRole('heading', { name: 'Pay for what you use' })
    ).toBeInTheDocument()
    expect(screen.queryByText('GPT-4o')).not.toBeInTheDocument()
    expect(container.querySelectorAll('[data-slot="skeleton"]').length).toBeGreaterThan(0)
  })

  test('shows offline banner when serving from cache and refresh triggers forceRefetch', async () => {
    const user = userEvent.setup()
    pricingHookState = {
      ...pricingHookState,
      isUsingCache: true,
      dataSource: 'cache',
      lastUpdated: Date.UTC(2026, 2, 1, 8, 30, 0),
    }

    render(<PublicPricingView />)

    expect(
      screen.getByText(/Showing cached prices that may be outdated/i)
    ).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: 'Refresh now' }))
    expect(forceRefetchMock).toHaveBeenCalledTimes(1)
  })

  test('filters cards by modality tab', async () => {
    const user = userEvent.setup()
    render(<PublicPricingView />)

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

    render(<PublicPricingView />)

    const alert = screen.getByRole('alert')
    expect(
      within(alert).getByRole('heading', { name: 'Unable to load prices' })
    ).toBeInTheDocument()
    expect(within(alert).getByText('network down')).toBeInTheDocument()

    await user.click(within(alert).getByRole('button', { name: /Retry/i }))
    expect(forceRefetchMock).toHaveBeenCalledTimes(1)
  })

  test('shows empty state when filters yield no cards', async () => {
    const user = userEvent.setup()
    render(<PublicPricingView />)

    await user.click(screen.getByRole('tab', { name: /^AUDIO/ }))

    expect(screen.getByText('No models found')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Clear all filters' })
    ).toBeInTheDocument()
  })
})
