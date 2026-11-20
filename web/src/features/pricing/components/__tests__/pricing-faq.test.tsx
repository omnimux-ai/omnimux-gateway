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
import { afterEach, expect, test, vi } from 'vitest'

import { PricingFaq } from '../pricing-faq'

afterEach(() => vi.unstubAllGlobals())

test('opens the first answer without reporting a toggle and records keyboard changes', async () => {
  const tracker = vi.fn()
  vi.stubGlobal('__OMNIMUX_TRACK__', tracker)
  const user = userEvent.setup()

  render(<PricingFaq />)

  const firstQuestion = screen.getByRole('button', {
    name: 'How are credits deducted?',
  })
  const secondQuestion = screen.getByRole('button', {
    name: 'What happens if a generation fails?',
  })
  expect(firstQuestion).toHaveAttribute('aria-expanded', 'true')
  expect(secondQuestion).toHaveAttribute('aria-expanded', 'false')
  expect(tracker).not.toHaveBeenCalled()

  await user.tab()
  await user.tab()
  expect(secondQuestion).toHaveFocus()
  await user.keyboard('{Enter}')
  expect(firstQuestion).toHaveAttribute('aria-expanded', 'true')
  expect(secondQuestion).toHaveAttribute('aria-expanded', 'true')
  expect(tracker).toHaveBeenNthCalledWith(1, 'pricing_faq_toggle', {
    faq_id: 'q2',
    expanded: true,
  })

  await user.keyboard('{Enter}')
  expect(secondQuestion).toHaveAttribute('aria-expanded', 'false')
  expect(tracker).toHaveBeenNthCalledWith(2, 'pricing_faq_toggle', {
    faq_id: 'q2',
    expanded: false,
  })
  expect(tracker).toHaveBeenCalledTimes(2)
})
