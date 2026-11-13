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
import { readFileSync } from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import { TOPUP_LADDER } from '../../constants'
import { TopupLadder } from '../topup-ladder'

/**
 * Canonical ladder applied to production by
 * `scripts/ops/apply-payment-amount-ladder.js`. The public page must never
 * advertise a discount the backend does not honour, so the frontend mirror is
 * pinned to this file.
 */
const LADDER_JSON = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '../../../../../../scripts/ops/payment/amount-ladder.json'
)

type LadderFile = {
  amount_options: number[]
  amount_discount: Record<string, number>
}

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Top up more, pay less': 'Top up more, pay less',
    'Discounts apply to preset amounts only':
      'Discounts apply to preset amounts only',
    Credits: 'Credits',
    'Save {{percent}}%': 'Save {{percent}}%',
  })
})

describe('TopupLadder', () => {
  test('renders one rung per preset amount with credits and USD usage', () => {
    render(<TopupLadder ladder={TOPUP_LADDER} />)

    expect(
      screen.getByRole('heading', { name: 'Top up more, pay less' })
    ).toBeInTheDocument()
    expect(
      screen.getByText('Discounts apply to preset amounts only')
    ).toBeInTheDocument()

    const rungs = screen.getAllByRole('listitem')
    expect(rungs).toHaveLength(TOPUP_LADDER.length)

    expect(screen.getByText('10,000')).toBeInTheDocument()
    expect(screen.getByText('$1,000')).toBeInTheDocument()
  })

  test('shows a saving badge only on discounted rungs', () => {
    render(<TopupLadder ladder={TOPUP_LADDER} />)

    const discounted = TOPUP_LADDER.filter((rung) => rung.discount < 1)
    expect(discounted).toHaveLength(3)

    // 5% / 10% / 15% — the three cheapest preset rungs carry no badge.
    expect(screen.getByText('Save 5%')).toBeInTheDocument()
    expect(screen.getByText('Save 10%')).toBeInTheDocument()
    expect(screen.getByText('Save 15%')).toBeInTheDocument()
    expect(screen.queryByText('Save 0%')).toBeNull()
  })

  test('renders nothing when the ladder is empty', () => {
    const { container } = render(<TopupLadder ladder={[]} />)

    expect(container).toBeEmptyDOMElement()
  })

  test('mirrors the canonical ops ladder used for production writes', () => {
    const canonical = JSON.parse(
      readFileSync(LADDER_JSON, 'utf8')
    ) as LadderFile

    expect(TOPUP_LADDER.map((rung) => rung.amountUsd)).toEqual(
      canonical.amount_options
    )

    for (const rung of TOPUP_LADDER) {
      const expectedDiscount =
        canonical.amount_discount[String(rung.amountUsd)] ?? 1
      expect(rung.discount).toBe(expectedDiscount)
    }

    // Every discount in the ops file must be advertised by the page, and the
    // credits shown must follow the 1 USD = 10 credits rule.
    expect(Object.keys(canonical.amount_discount).map(Number).sort()).toEqual(
      TOPUP_LADDER.filter((rung) => rung.discount < 1)
        .map((rung) => rung.amountUsd)
        .sort()
    )
    for (const rung of TOPUP_LADDER) {
      expect(rung.credits).toBe(rung.amountUsd / 0.1)
    }
  })
})
