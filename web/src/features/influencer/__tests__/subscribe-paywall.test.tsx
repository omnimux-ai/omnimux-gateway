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
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  getPublicPlans,
  paySubscriptionStripe,
} from '@/features/subscriptions/api'
import type { PlanRecord } from '@/features/subscriptions/types'
import en from '@/i18n/locales/en.json'
import { getSelf } from '@/lib/api'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

import { SubscribePaywall } from '../components/subscribe-paywall'

vi.mock('@/features/subscriptions/api', () => ({
  getPublicPlans: vi.fn(),
  paySubscriptionStripe: vi.fn(),
}))
vi.mock('@/lib/api', () => ({ getSelf: vi.fn() }))

function plan(
  id: number,
  title: string,
  price: number,
  credits: number,
  duration_unit: 'month' | 'year' = 'month'
): PlanRecord {
  return {
    plan: {
      id,
      title,
      price_amount: price,
      currency: 'USD',
      duration_unit,
      duration_value: 1,
      quota_reset_period: 'monthly',
      enabled: true,
      sort_order: id,
      max_purchase_per_user: 0,
      total_amount: credits * 50_000,
      allow_balance_pay: true,
      allow_wallet_overflow: true,
      stripe_price_id: `price_${id}`,
    },
  }
}

const monthly = plan(1, 'Monthly', 49, 200)
const yearly = plan(2, 'Yearly', 399, 2400, 'year')

async function renderPaywall(overrides: Record<string, unknown> = {}) {
  const i18n = createInstance()
  await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en } })
  const props = {
    open: true,
    requiredCredits: 0.0147,
    origin: 'image' as const,
    onClose: vi.fn(),
    onSubscribed: vi.fn(),
    ...overrides,
  }
  render(
    <I18nextProvider i18n={i18n}>
      <SubscribePaywall {...props} />
    </I18nextProvider>
  )
  return props
}

function setQuota(quota: number) {
  useAuthStore.setState({
    auth: {
      ...useAuthStore.getState().auth,
      user: { id: 1, quota } as AuthUser,
    },
  })
}

beforeEach(() => {
  setQuota(0)
  vi.mocked(getPublicPlans).mockResolvedValue({
    success: true,
    data: [monthly, yearly],
  })
  vi.mocked(getSelf).mockImplementation(async () => ({
    success: true,
    data: { quota: useAuthStore.getState().auth.user?.quota ?? 0 },
  }))
  vi.spyOn(window, 'open').mockReturnValue(null)
})

afterEach(() => {
  cleanup()
  useAuthStore.setState({
    auth: { ...useAuthStore.getState().auth, user: null },
  })
})

describe('subscription paywall', () => {
  it('takes over the screen with plans and states Stripe as the only method', async () => {
    await renderPaywall()
    expect(
      await screen.findByRole('dialog', {
        name: 'Subscribe to continue this generation',
      })
    ).toBeVisible()
    expect(screen.getByRole('radio', { name: /^Monthly/ })).toHaveAttribute(
      'aria-checked',
      'true'
    )
    expect(screen.getByText('Payment method')).toBeVisible()
    expect(screen.getByText('Stripe')).toBeVisible()
    expect(screen.queryByText('Alipay')).not.toBeInTheDocument()
    expect(screen.queryByText('WeChat Pay')).not.toBeInTheDocument()
  })

  it('shows the required credits against the current balance', async () => {
    setQuota(7_350)
    await renderPaywall()
    expect(await screen.findByText(/needs 0.0147 credits/)).toBeVisible()
    expect(screen.getByText(/balance is 0.147/)).toBeVisible()
  })

  it('localizes a known Chinese database title in the English UI', async () => {
    vi.mocked(getPublicPlans).mockResolvedValue({
      success: true,
      data: [
        {
          plan: {
            ...monthly.plan,
            title: '创作版·Creator（年付）',
            upgrade_group: 'sub-pro',
          },
        },
      ],
    })
    await renderPaywall()
    const option = await screen.findByRole('radio', { name: /^Creator/ })
    expect(option).toBeVisible()
    expect(screen.queryByText(/创作版/)).not.toBeInTheDocument()
    expect(screen.getByText(/200 credits · Monthly/)).toBeVisible()
  })

  it('updates the pay action when another plan is selected', async () => {
    await renderPaywall()
    const yearlyOption = await screen.findByRole('radio', { name: /^Yearly/ })
    await userEvent.click(yearlyOption)
    expect(yearlyOption).toHaveAttribute('aria-checked', 'true')
    expect(
      screen.getByRole('button', { name: /Pay \$399.00 · Subscribe to Yearly/ })
    ).toBeVisible()
  })

  it('opens the Stripe checkout, waits, then resumes once the balance grows', async () => {
    vi.mocked(paySubscriptionStripe).mockResolvedValue({
      success: true,
      message: 'success',
      data: { pay_link: 'https://checkout.stripe.test/session' },
    })
    const props = await renderPaywall()
    await userEvent.click(
      await screen.findByRole('button', {
        name: /Pay \$49.00 · Subscribe to Monthly/,
      })
    )
    expect(paySubscriptionStripe).toHaveBeenCalledWith({ plan_id: 1 })
    expect(window.open).toHaveBeenCalledWith(
      'https://checkout.stripe.test/session',
      '_blank',
      'noopener'
    )
    expect(
      await screen.findByRole('button', { name: /Waiting for payment/ })
    ).toBeDisabled()

    vi.mocked(getSelf).mockResolvedValue({
      success: true,
      data: { quota: 10_000_000 },
    })
    await userEvent.click(
      screen.getByRole('button', { name: 'I have completed the payment' })
    )
    expect(
      await screen.findByRole('heading', { name: 'Subscription active' })
    ).toBeVisible()
    await userEvent.click(
      screen.getByRole('button', { name: 'Continue generating' })
    )
    expect(props.onSubscribed).toHaveBeenCalledTimes(1)
  })

  it('does not resume before the wallet actually grew', async () => {
    vi.mocked(paySubscriptionStripe).mockResolvedValue({
      success: true,
      message: 'success',
      data: { pay_link: 'https://checkout.stripe.test/session' },
    })
    const props = await renderPaywall()
    await userEvent.click(
      await screen.findByRole('button', {
        name: /Pay \$49.00 · Subscribe to Monthly/,
      })
    )
    await userEvent.click(
      await screen.findByRole('button', {
        name: 'I have completed the payment',
      })
    )
    await waitFor(() => {
      expect(
        screen.queryByRole('heading', { name: 'Subscription active' })
      ).not.toBeInTheDocument()
    })
    expect(props.onSubscribed).not.toHaveBeenCalled()
  })

  it('falls back to a wallet top-up when the site sells no plans', async () => {
    vi.mocked(getPublicPlans).mockResolvedValue({ success: true, data: [] })
    await renderPaywall()
    expect(
      await screen.findByRole('heading', {
        name: 'No subscription plans available',
      })
    ).toBeVisible()
    await userEvent.click(
      screen.getByRole('button', { name: 'Add credits in Wallet' })
    )
    expect(window.open).toHaveBeenCalledWith('/wallet', '_blank', 'noopener')
  })

  it('closes on Escape and on the back button', async () => {
    const props = await renderPaywall()
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(props.onClose).toHaveBeenCalledTimes(1)
    await userEvent.click(
      screen.getByRole('button', { name: 'Back to editor' })
    )
    expect(props.onClose).toHaveBeenCalledTimes(2)
  })
})
