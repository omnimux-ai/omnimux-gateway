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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { beforeAll, beforeEach, describe, expect, test, vi } from 'vitest'

import type { PublicModelCardItem } from '../../types'
import { ModelPricingCard } from '../model-pricing-card'

const copyToClipboardMock = vi.fn()
const navigateMock = vi.fn()
let mockedCopiedText: string | null = null

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => navigateMock,
  Link: (props: any) => (
    <a
      href={typeof props.to === 'string' ? props.to : '#'}
      onClick={props.onClick}
      className={props.className}
    >
      {props.children}
    </a>
  ),
}))

vi.mock('@/hooks/use-copy-to-clipboard', () => ({
  useCopyToClipboard: () => ({
    get copiedText() {
      return mockedCopiedText
    },
    copyToClipboard: copyToClipboardMock,
  }),
}))

vi.mock('@/lib/lobe-icon', () => ({
  getLobeIcon: (name: string) => (
    <span data-testid='vendor-icon'>{name}</span>
  ),
}))

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
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
    'Official 1:1': 'Official 1:1',
    'Save {{percent}}%': 'Save {{percent}}%',
    Generate: 'Generate',
  })
})

function makeCard(
  overrides: Partial<PublicModelCardItem> = {}
): PublicModelCardItem {
  return {
    id: 'seedream-3.0',
    name: 'Seedream 3.0',
    modelId: 'seedream-3.0',
    provider: 'ByteDance',
    vendorIcon: 'bytedance.svg',
    category: 'image',
    tierCount: 2,
    tiers: [
      {
        spec: '1K',
        credits: 0.15,
        unitSuffix: '/pic',
        officialUsd: 0.01875,
        officialCredits: 0.1875,
        savingPercent: 20,
      },
      {
        spec: '2K',
        credits: 0.3,
        unitSuffix: '/pic',
      },
    ],
    rawModel: {
      id: 42,
      model_name: 'seedream-3.0',
      quota_type: 1,
      model_ratio: 1,
      completion_ratio: 1,
      enable_groups: ['default'],
      vendor_id: 7,
    },
    ...overrides,
  }
}

describe('ModelPricingCard', () => {
  beforeEach(() => {
    mockedCopiedText = null
    copyToClipboardMock.mockReset()
    copyToClipboardMock.mockImplementation(async (text: string) => {
      mockedCopiedText = text
      return true
    })
  })

  test('renders minimal header, tier badge, saving capsule, and embedded price table', () => {
    render(<ModelPricingCard card={makeCard()} />)

    expect(screen.getByText('Seedream 3.0')).toBeInTheDocument()
    expect(screen.getByText('(seedream-3.0)')).toBeInTheDocument()
    expect(screen.getByText('2 PRICE TIERS')).toBeInTheDocument()
    expect(screen.getByTestId('vendor-icon')).toHaveTextContent('bytedance.svg')
    expect(screen.getByRole('columnheader', { name: 'SPEC' })).toBeInTheDocument()
    expect(screen.getByText('Save 20%')).toBeInTheDocument()
    expect(screen.getByText('20%')).toBeInTheDocument()
    expect(screen.queryByText(/HOT|NEW/i)).not.toBeInTheDocument()
  })

  test('renders Official 1:1 capsule when all tiers are official same price', () => {
    render(
      <ModelPricingCard
        card={makeCard({
          tierCount: 1,
          tiers: [
            {
              spec: 'Standard',
              credits: 0.4,
              unitSuffix: '/pic',
              officialUsd: 0.04,
              officialCredits: 0.4,
              savingPercent: 0,
            },
          ],
        })}
      />
    )

    expect(screen.getAllByText('Official 1:1').length).toBeGreaterThanOrEqual(1)
  })

  test('uses singular PRICE TIER badge for one tier', () => {
    render(
      <ModelPricingCard
        card={makeCard({
          tierCount: 1,
          tiers: [
            {
              spec: 'Standard',
              credits: 1,
              unitSuffix: '/gen',
            },
          ],
        })}
      />
    )

    expect(screen.getByText('1 PRICE TIER')).toBeInTheDocument()
  })

  test('copies modelId and notifies optional callback without navigating', async () => {
    const user = userEvent.setup()
    const onCopyModelId = vi.fn()
    const onCardClick = vi.fn()

    render(
      <ModelPricingCard
        card={makeCard()}
        onCopyModelId={onCopyModelId}
        onCardClick={onCardClick}
      />
    )

    await user.click(screen.getByRole('button', { name: 'Copy model ID' }))

    expect(copyToClipboardMock).toHaveBeenCalledWith('seedream-3.0')
    expect(onCopyModelId).toHaveBeenCalledWith('seedream-3.0')
    expect(onCardClick).not.toHaveBeenCalled()
    expect(navigateMock).not.toHaveBeenCalled()
  })

  test('navigates to model page when card is clicked', async () => {
    const user = userEvent.setup()

    render(<ModelPricingCard card={makeCard()} />)

    await user.click(screen.getByText('Seedream 3.0'))

    expect(navigateMock).toHaveBeenCalledWith({
      to: '/pricing/$modelId',
      params: { modelId: 'seedream-3.0' },
    })
  })

  test('invokes onCardClick when provided', async () => {
    const user = userEvent.setup()
    const onCardClick = vi.fn()

    render(<ModelPricingCard card={makeCard()} onCardClick={onCardClick} />)

    await user.click(screen.getByText('Seedream 3.0'))

    expect(onCardClick).toHaveBeenCalledWith('seedream-3.0')
    expect(navigateMock).not.toHaveBeenCalled()
  })

  test('renders generate button linking to model generation page', () => {
    render(<ModelPricingCard card={makeCard()} />)

    const generateLink = screen.getByRole('link', { name: /generate/i })
    expect(generateLink).toBeInTheDocument()
    expect(generateLink).toHaveAttribute('href', '/pricing/seedream-3.0')
  })
})
