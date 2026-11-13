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
import { beforeAll, describe, expect, test, vi } from 'vitest'

import { BillingHero, PRICING_MODELS_SECTION_ID } from '../billing-hero'

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
  })
})

describe('BillingHero', () => {
  test('renders heading, feature pills, and both CTAs', () => {
    render(<BillingHero modelCount={80} />)

    expect(
      screen.getByRole('heading', { name: 'Pay for what you use' })
    ).toBeInTheDocument()
    // 订阅套餐已下线，hero 不得再宣称「可选套餐」。
    expect(screen.queryByText(/optional plans/i)).not.toBeInTheDocument()
    expect(screen.queryByText(/prepaid credits/i)).not.toBeInTheDocument()
    expect(screen.getByText('Check prices and wallet allowances')).toBeInTheDocument()
    expect(screen.getByText('Review generation history')).toBeInTheDocument()
    expect(screen.getByText('Failed tasks return generation credits')).toBeInTheDocument()
    expect(screen.queryByText(/no subscription|never expire|no minimums/i)).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: /View model prices/i })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: /Enterprise pricing/i })
    ).toBeInTheDocument()
    expect(screen.getByText(/80 available models/)).toBeInTheDocument()
  })

  test('primary CTA scrolls to the pricing models section by default', async () => {
    const user = userEvent.setup()
    const target = document.createElement('div')
    target.id = PRICING_MODELS_SECTION_ID
    const scrollIntoView = vi.fn()
    Object.defineProperty(target, 'scrollIntoView', {
      configurable: true,
      value: scrollIntoView,
    })
    document.body.appendChild(target)

    render(<BillingHero />)
    await user.click(screen.getByRole('button', { name: /View model prices/i }))

    expect(scrollIntoView).toHaveBeenCalledWith({
      behavior: 'smooth',
      block: 'start',
    })

    target.remove()
  })

  test('invokes onConsultEnterprise when provided', async () => {
    const user = userEvent.setup()
    const onConsultEnterprise = vi.fn()

    render(<BillingHero onConsultEnterprise={onConsultEnterprise} />)
    await user.click(screen.getByRole('button', { name: /Enterprise pricing/i }))

    expect(onConsultEnterprise).toHaveBeenCalledTimes(1)
  })
})
