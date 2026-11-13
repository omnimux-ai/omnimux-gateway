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
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import type { PublicPriceTier } from '../../types'
import { TierPriceTable } from '../tier-price-table'

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Price tiers': 'Price tiers',
    SPEC: 'SPEC',
    'OUR PRICE': 'OUR PRICE',
    'OFFICIAL PRICE': 'OFFICIAL PRICE',
    SAVING: 'SAVING',
    'Official 1:1': 'Official 1:1',
    'Save {{percent}}%': 'Save {{percent}}%',
    Credits: 'Credits',
  })
})

const TIERS: PublicPriceTier[] = [
  {
    spec: '1K · Standard',
    credits: 0.15,
    unitSuffix: 'pic',
    officialUsd: 0.01875,
    officialCredits: 0.1875,
    savingPercent: 20,
    note: 'fast queue',
  },
  {
    spec: '2K · HD',
    credits: 0.3,
    unitSuffix: '/pic',
  },
  {
    spec: 'Official same',
    credits: 0.4,
    unitSuffix: '/pic',
    officialUsd: 0.04,
    officialCredits: 0.4,
    savingPercent: 0,
  },
]

describe('TierPriceTable', () => {
  test('renders four-column headers and formats our/official/saving rows', () => {
    render(<TierPriceTable tiers={TIERS} />)

    expect(screen.getByRole('columnheader', { name: 'SPEC' })).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'OUR PRICE' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'OFFICIAL PRICE' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'SAVING' })
    ).toBeInTheDocument()

    expect(screen.getByText('1K · Standard')).toBeInTheDocument()
    expect(screen.getByText('fast queue')).toBeInTheDocument()

    const rows = screen.getAllByRole('row').slice(1)
    expect(rows.length).toBeGreaterThanOrEqual(2)
    const discountedRowElement = rows[0]
    expect(discountedRowElement).toBeTruthy()
    const discountedRow = within(discountedRowElement as HTMLElement)

    expect(discountedRow.getByText('0.15')).toBeInTheDocument()
    expect(discountedRow.getAllByText('Credits/pic')).toHaveLength(2)
    expect(discountedRow.getByText('~$0.015/pic')).toBeInTheDocument()

    expect(discountedRow.getByText('0.1875')).toBeInTheDocument()
    // formatUsdValue keeps up to 4 decimals for values in (0.01, 1)
    expect(discountedRow.getByText('~$0.0188/pic')).toBeInTheDocument()

    const officialCell = discountedRow.getByText('0.1875').closest('div')
    expect(officialCell).toHaveClass('line-through')

    const saving = discountedRow.getByText('20%')
    expect(saving).toBeInTheDocument()
    expect(saving.parentElement).toHaveClass('text-emerald-600')
    expect(
      saving.parentElement?.querySelector('svg')
    ).not.toBeNull()
  })

  test('shows em dashes when official price and saving are absent', () => {
    render(<TierPriceTable tiers={TIERS} />)

    const rows = screen.getAllByRole('row').slice(1)
    expect(rows.length).toBeGreaterThanOrEqual(2)
    const plainRowElement = rows[1]
    expect(plainRowElement).toBeTruthy()
    const plainRow = within(plainRowElement as HTMLElement)
    expect(plainRow.getAllByText('—')).toHaveLength(2)
  })

  test('renders Official 1:1 badge without line-through when savingPercent is 0', () => {
    render(<TierPriceTable tiers={TIERS} />)

    const rows = screen.getAllByRole('row').slice(1)
    const sameRowElement = rows[2]
    expect(sameRowElement).toBeTruthy()
    const sameRow = within(sameRowElement as HTMLElement)
    expect(sameRow.getByText('Official 1:1')).toBeInTheDocument()
    expect(sameRow.getAllByText('0.4').length).toBeGreaterThanOrEqual(1)
    const officialCells = sameRow
      .getAllByText('~$0.04/pic')
      .map((node) => node.closest('div'))
    expect(officialCells.length).toBeGreaterThan(0)
    for (const cell of officialCells) {
      expect(cell).not.toHaveClass('line-through')
    }
  })

  test('hides OFFICIAL PRICE header on small screens via sm:table-cell', () => {
    render(<TierPriceTable tiers={TIERS} />)
    const officialHeader = screen.getByRole('columnheader', {
      name: 'OFFICIAL PRICE',
    })
    expect(officialHeader).toHaveClass('hidden')
    expect(officialHeader).toHaveClass('sm:table-cell')
  })
})
