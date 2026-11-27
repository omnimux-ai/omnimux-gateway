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
import { act, render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeEach, expect, it, vi } from 'vitest'

import { BillingHistoryDialog } from '@/features/wallet/components/dialogs/billing-history-dialog'
import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'
import { api } from '@/lib/api'

import { RechargeFormCard } from '../components/recharge-form-card'
import { SocialSeatsCard } from '../components/social-seats-card'

const i18n = createInstance()

beforeEach(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'zh',
    fallbackLng: false,
    nsSeparator: false,
    resources: { en, zh },
    interpolation: { escapeValue: false },
  })
})

it('translates billing history statuses and updates them when the language changes', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        total: 3,
        items: ['success', 'pending', 'expired'].map((status, id) => ({
          id,
          status,
          amount: 10,
          money: 10,
          trade_no: `order-${id}`,
          payment_method: 'stripe',
          create_time: 1,
        })),
      },
    },
  })
  render(
    <I18nextProvider i18n={i18n}>
      <BillingHistoryDialog open onOpenChange={vi.fn()} />
    </I18nextProvider>
  )
  await screen.findByText('order-0')
  for (const label of ['成功', '待确认', '已过期']) {
    expect(screen.getByText(label)).toBeVisible()
  }
  await act(() => i18n.changeLanguage('en'))
  for (const label of ['Success', 'Pending', 'Expired']) {
    expect(screen.getByText(label)).toBeVisible()
  }
})

it.each([{ priceRatio: 7.3 }, { priceRatio: 0.000123 }])(
  'renders the recharge panel at price ratio $priceRatio without breaking the redemption flow',
  async ({ priceRatio }) => {
    render(
      <I18nextProvider i18n={i18n}>
        <RechargeFormCard
          topupInfo={{
            enable_online_topup: true,
            enable_stripe_topup: false,
            pay_methods: [],
            min_topup: 1,
            stripe_min_topup: 1,
            amount_options: [100, 20],
            discount: {},
          }}
          presetAmounts={[{ value: 100, discount: 0.8 }, { value: 20 }]}
          selectedPreset={null}
          topupAmount={1}
          paymentAmount={priceRatio}
          priceRatio={priceRatio}
          calculating={false}
          paymentLoading={null}
          redemptionCode=''
          redeeming={false}
          onSelectPreset={vi.fn()}
          onTopupAmountChange={vi.fn()}
          onPaymentMethodSelect={vi.fn()}
          onRedemptionCodeChange={vi.fn()}
          onRedeem={vi.fn()}
        />
      </I18nextProvider>
    )
    // Pay-as-you-go is the only credit product: the amount input and the preset
    // grid render, and neither breaks at an extreme configured price ratio.
    expect(screen.getByRole('spinbutton')).toBeVisible()
    expect(screen.getAllByRole('button').length).toBeGreaterThan(1)

    // The redemption card stays: its input and CTA are still reachable.
    expect(screen.getByPlaceholderText('输入您的兑换码')).toBeVisible()
    expect(screen.getByRole('button', { name: '兑换额度' })).toBeVisible()

    await act(() => i18n.changeLanguage('en'))

    expect(
      screen.getByPlaceholderText('Enter your redemption code')
    ).toBeVisible()
    expect(screen.getByRole('button', { name: 'Redeem' })).toBeVisible()
    expect(screen.getByRole('spinbutton')).toBeVisible()
  }
)

it('renders SocialSeatsCard in seat mode with localized copy and vacant slots, and hides it in off mode', async () => {
  const spy = vi.spyOn(api, 'get').mockResolvedValueOnce({
    data: {
      success: true,
      data: {
        billing_mode: 'seat',
        seats_purchased: 4,
        seats_occupied: 2,
        seats_vacant: 2,
        seats_cap: 20,
        next_expiry: '2026-12-27 19:00',
        seat_price_usd: 2,
        seat_price_credits: 20,
        vacant_seats: [
          {
            id: 11,
            expires_at: '2026-12-27T11:00:00Z',
            swaps_remaining: 2,
            same_account_only: false,
          },
          {
            id: 12,
            expires_at: '2026-12-30T11:00:00Z',
            swaps_remaining: 0,
            same_account_only: true,
          },
        ],
      },
    },
  })

  const { container } = render(
    <I18nextProvider i18n={i18n}>
      <SocialSeatsCard />
    </I18nextProvider>
  )

  await screen.findByText('社媒绑定席位')
  expect(screen.getByText('20 积分（$2 / 30 天）')).toBeVisible()
  expect(screen.getByText('4 / 20')).toBeVisible()
  expect(screen.getByText('剩余 2 次换绑')).toBeVisible()
  expect(screen.getByText('仅限原号')).toBeVisible()
  expect(container.textContent).not.toContain('·')

  await act(() => i18n.changeLanguage('en'))
  expect(screen.getByText('Social account binding seats')).toBeVisible()
  expect(screen.getByText('20 credits ($2 / 30d)')).toBeVisible()
  expect(screen.getByText('2 swaps left')).toBeVisible()
  expect(screen.getByText('Same account only')).toBeVisible()

  spy.mockResolvedValueOnce({
    data: { success: true, data: { billing_mode: 'off' } },
  })
  const offView = render(
    <I18nextProvider i18n={i18n}>
      <SocialSeatsCard />
    </I18nextProvider>
  )
  await act(async () => {
    await Promise.resolve()
  })
  expect(offView.container).toBeEmptyDOMElement()
})
