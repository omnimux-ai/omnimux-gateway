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
import { screen } from '@testing-library/react'
import { afterEach, describe, expect, test } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import { renderPricing } from '../../__tests__/render-pricing'
import { BillingHero } from '../billing-hero'

afterEach(() => useAuthStore.getState().auth.reset())

describe('BillingHero', () => {
  test('shows the estimate and routes guests to sign-in for a top-up', () => {
    renderPricing(<BillingHero />)

    expect(
      screen.getByRole('heading', {
        name: 'Pay for what you use — no subscription required',
      })
    ).toBeInTheDocument()
    expect(screen.getByText('1 credit = $0.10 USD')).toBeInTheDocument()
    expect(
      screen.getByText('Generation credit refunds follow task records')
    ).toBeInTheDocument()
    expect(
      screen.queryByText(
        /never expire|no minimums|optional plans|prepaid credits/i
      )
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'Estimate a video cost' })
    ).toHaveAttribute('href', '/pricing#estimator')
    expect(
      screen.getByRole('link', { name: 'Top up credits' })
    ).toHaveAttribute('href', '/sign-in?redirect=%2Fwallet')
  })

  test('opens the wallet deposit section for a signed-in visitor', () => {
    useAuthStore.getState().auth.setUser({ id: 7, username: 'member', role: 1 })
    renderPricing(<BillingHero />)

    expect(
      screen.getByRole('link', { name: 'Top up credits' })
    ).toHaveAttribute('href', '/wallet#wallet-add-funds')
  })
})
