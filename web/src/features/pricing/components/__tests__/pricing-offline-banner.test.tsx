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

import { PricingOfflineBanner } from '../pricing-offline-banner'

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Showing cached prices that may be outdated. Last synced: {{time}}.':
      'Showing cached prices that may be outdated. Last synced: {{time}}.',
    'unknown time': 'unknown time',
    'Refresh now': 'Refresh now',
  })
})

describe('PricingOfflineBanner', () => {
  test('renders nothing when not using cache', () => {
    const { container } = render(
      <PricingOfflineBanner isUsingCache={false} />
    )
    expect(container).toBeEmptyDOMElement()
  })

  test('shows last synced time and triggers force refresh', async () => {
    const user = userEvent.setup()
    const onRefresh = vi.fn()
    const lastUpdated = Date.UTC(2026, 7, 30, 8, 0, 0)

    render(
      <PricingOfflineBanner
        isUsingCache
        lastUpdated={lastUpdated}
        onRefresh={onRefresh}
      />
    )

    expect(
      screen.getByText(/Showing cached prices that may be outdated/i)
    ).toBeInTheDocument()
    expect(screen.getByText(/2026-08-30/)).toBeInTheDocument()

    await user.click(screen.getByRole('button', { name: /Refresh now/i }))
    expect(onRefresh).toHaveBeenCalledTimes(1)
  })
})
