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
  act,
  cleanup,
  fireEvent,
  render,
  screen,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { requestStripePayment } from '@/features/wallet/api'
import { useTopupInfo } from '@/features/wallet/hooks/use-topup-info'
import type { TopupInfo } from '@/features/wallet/types'
import en from '@/i18n/locales/en.json'
import { getSelf } from '@/lib/api'
import { useAuthStore, type AuthUser } from '@/stores/auth-store'

import { SubscribePaywall } from '../components/subscribe-paywall'

vi.mock('@/features/wallet/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/features/wallet/api')>()
  return { ...actual, requestStripePayment: vi.fn() }
})
vi.mock('@/features/wallet/hooks/use-topup-info', () => ({
  useTopupInfo: vi.fn(),
}))
vi.mock('@/lib/api', () => ({ getSelf: vi.fn() }))
vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: { price: 1 }, loading: false, error: null }),
}))
vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({
    currency: {
      quotaDisplayType: 'USD',
      usdExchangeRate: 1,
      customCurrencyExchangeRate: 1,
    },
    loading: false,
    logoLoaded: false,
  }),
}))
vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
    info: vi.fn(),
    success: vi.fn(),
    warning: vi.fn(),
  },
}))

const topupInfo: TopupInfo = {
  enable_online_topup: true,
  enable_stripe_topup: true,
  enable_creem_topup: false,
  pay_methods: [],
  min_topup: 1,
  stripe_min_topup: 1,
  amount_options: [10, 50, 100, 300, 1000, 3000],
  discount: { 300: 0.95, 1000: 0.9, 3000: 0.85 },
}

const presetAmounts = [
  { value: 10 },
  { value: 50 },
  { value: 100 },
  { value: 300, discount: 0.95 },
  { value: 1000, discount: 0.9 },
  { value: 3000, discount: 0.85 },
]

const useTopupInfoMock = vi.mocked(useTopupInfo)

function mockTopupInfo(
  overrides: Partial<ReturnType<typeof useTopupInfo>> = {}
) {
  useTopupInfoMock.mockReturnValue({
    topupInfo,
    presetAmounts,
    loading: false,
    refetch: vi.fn(),
    ...overrides,
  })
}

async function renderPaywall(overrides: Record<string, unknown> = {}) {
  const i18n = createInstance()
  await i18n.init({ lng: 'en', fallbackLng: 'en', resources: { en } })
  const props = {
    open: true,
    requiredCredits: 42,
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

/** Push a new quota through the mocked getSelf refresh path. */
function refreshWithQuota(quota: number) {
  setQuota(quota)
  vi.mocked(getSelf).mockResolvedValue({ id: 1, quota })
}

beforeEach(() => {
  setQuota(0)
  mockTopupInfo()
  vi.mocked(getSelf).mockImplementation(async () => ({
    id: 1,
    quota: useAuthStore.getState().auth.user?.quota ?? 0,
  }))
  vi.spyOn(window, 'open').mockReturnValue(null)
})

afterEach(() => {
  cleanup()
  useAuthStore.setState({
    auth: { ...useAuthStore.getState().auth, user: null },
  })
  vi.restoreAllMocks()
})

describe('top-up paywall', () => {
  it('opens as a dialog with the top-up ladder and Stripe as the only method', async () => {
    await renderPaywall()
    expect(
      await screen.findByRole('dialog', {
        name: 'Top up to continue this generation',
      })
    ).toBeVisible()
    expect(screen.getByText('Stripe')).toBeVisible()
    expect(screen.getByText('Secure payment')).toBeVisible()
    expect(screen.queryByText('Alipay')).not.toBeInTheDocument()
    expect(screen.queryByText('WeChat Pay')).not.toBeInTheDocument()
    expect(screen.queryByText('Subscribe')).not.toBeInTheDocument()
  })

  it('shows the current balance and the credits still needed', async () => {
    setQuota(500_000) // $1.00 = 10 credits
    await renderPaywall({ requiredCredits: 42 })
    expect(await screen.findByText('Balance')).toBeVisible()
    expect(screen.getByText('Credits needed')).toBeVisible()
    expect(screen.getByText('42')).toBeVisible()
  })

  it('default-selects the smallest rung covering the required credits', async () => {
    // 42 credits = $4.20 → smallest covering rung is $10.
    await renderPaywall({ requiredCredits: 42 })
    const ten = await screen.findByRole('button', { name: /^\$10\b/ })
    expect(ten).toHaveAttribute('aria-pressed', 'true')
  })

  it('selects a larger rung when the required credits exceed the small amounts', async () => {
    // 600 credits = $60 → smallest covering rung is $100.
    await renderPaywall({ requiredCredits: 600 })
    const hundred = await screen.findByRole('button', { name: /^\$100\b/ })
    expect(hundred).toHaveAttribute('aria-pressed', 'true')
  })

  it('shows the resolved volume discount on discounted rungs only', async () => {
    await renderPaywall()
    await screen.findByRole('button', { name: /^\$285\b/ })
    expect(screen.getByText('5% OFF')).toBeVisible()
    expect(screen.getByText('10% OFF')).toBeVisible()
    expect(screen.getByText('15% OFF')).toBeVisible()
    expect(screen.getAllByText(/% OFF/)).toHaveLength(3)
  })

  it('switches the selected rung when another amount is clicked', async () => {
    await renderPaywall()
    const fifty = await screen.findByRole('button', { name: /^\$50\b/ })
    await userEvent.click(fifty)
    expect(fifty).toHaveAttribute('aria-pressed', 'true')
    expect(
      screen.getByRole('button', { name: 'Pay $50 with Stripe' })
    ).toBeVisible()
  })

  it('pays through Stripe, opens the checkout, then resumes once the balance grows', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    try {
      vi.mocked(requestStripePayment).mockResolvedValue({
        success: true,
        message: 'success',
        data: { pay_link: 'https://checkout.stripe.test/session' },
      })
      const props = await renderPaywall()
      await userEvent.click(
        await screen.findByRole('button', { name: 'Pay $10 with Stripe' })
      )
      expect(requestStripePayment).toHaveBeenCalledWith({
        amount: 10,
        payment_method: 'stripe',
      })
      expect(window.open).toHaveBeenCalledWith(
        'https://checkout.stripe.test/session',
        '_blank',
        'noopener'
      )
      expect(
        await screen.findByText(
          'Complete the payment in the new tab. This page continues automatically.'
        )
      ).toBeVisible()

      // Stripe webhook credits the wallet; the next poll detects the growth.
      refreshWithQuota(10 * 500_000) // +$10 → +100 credits
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3_000)
      })
      expect(
        await screen.findByRole('heading', { name: 'Top up successful' })
      ).toBeVisible()

      await userEvent.click(
        screen.getByRole('button', { name: 'Continue generating' })
      )
      expect(props.onSubscribed).toHaveBeenCalledTimes(1)
    } finally {
      vi.useRealTimers()
    }
  })

  it('does not show success when the manual check finds no balance growth', async () => {
    vi.mocked(requestStripePayment).mockResolvedValue({
      success: true,
      message: 'success',
      data: { pay_link: 'https://checkout.stripe.test/session' },
    })
    await renderPaywall()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Pay $10 with Stripe' })
    )
    await userEvent.click(
      await screen.findByRole('button', {
        name: 'I have completed the payment',
      })
    )
    expect(toast.info).toHaveBeenCalledWith('Payment not completed yet')
    expect(
      screen.queryByRole('heading', { name: 'Top up successful' })
    ).not.toBeInTheDocument()
  })

  it('reports a failed Stripe request through the shared toast path', async () => {
    vi.mocked(requestStripePayment).mockResolvedValue({
      success: false,
      message: 'insufficient funds',
      data: { pay_link: '' },
    })
    await renderPaywall()
    await userEvent.click(
      await screen.findByRole('button', { name: 'Pay $10 with Stripe' })
    )
    expect(toast.error).toHaveBeenCalled()
    expect(window.open).not.toHaveBeenCalled()
  })

  it('falls back to Wallet top-up when Stripe top-up is disabled', async () => {
    mockTopupInfo({
      topupInfo: { ...topupInfo, enable_stripe_topup: false },
    })
    await renderPaywall()
    expect(
      await screen.findByRole('heading', { name: 'Top up in Wallet' })
    ).toBeVisible()
    await userEvent.click(
      screen.getByRole('button', { name: 'Add credits in Wallet' })
    )
    expect(window.open).toHaveBeenCalledWith('/wallet', '_blank', 'noopener')
  })

  it('falls back to Wallet top-up when no amounts are configured', async () => {
    mockTopupInfo({ presetAmounts: [] })
    await renderPaywall()
    expect(
      await screen.findByRole('heading', { name: 'Top up in Wallet' })
    ).toBeVisible()
  })

  it('closes on Escape and on the back button', async () => {
    const props = await renderPaywall()
    await screen.findByRole('dialog')
    fireEvent.keyDown(document, { key: 'Escape' })
    expect(props.onClose).toHaveBeenCalledTimes(1)
    await userEvent.click(
      screen.getByRole('button', { name: 'Back to editor' })
    )
    expect(props.onClose).toHaveBeenCalledTimes(2)
  })
})
