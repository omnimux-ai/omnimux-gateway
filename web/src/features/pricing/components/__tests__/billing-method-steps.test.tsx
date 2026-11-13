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
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import { USD_PER_CREDIT } from '../../constants'
import { BillingMethodSteps } from '../billing-method-steps'

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'How billing works': 'How billing works',
    'One balance, every model. Metered per call, visible live in the console.':
      'One balance, every model. Metered per call, visible live in the console.',
    'Top up credits': 'Top up credits',
    'Pay with card, Alipay/WeChat, or wire transfer.':
      'Pay with card, Alipay/WeChat, or wire transfer.',
    '1 credit = ${{usd}} USD': '1 credit = ${{usd}} USD',
    'Call any model': 'Call any model',
    'Video and audio billed per second, images per generation, LLMs per token. Unit prices sit on each model card — no hidden multipliers.':
      'Video and audio billed per second, images per generation, LLMs per token. Unit prices sit on each model card — no hidden multipliers.',
    'One API → {{count}} models': 'One API → {{count}} models',
    'Every cent is traceable': 'Every cent is traceable',
    'Usage logs update in milliseconds with tokens, latency, and cost per call. Export by API key, model, and time range.':
      'Usage logs update in milliseconds with tokens, latency, and cost per call. Export by API key, model, and time range.',
    'Failed tasks return generation credits':
      'Generation credit refunds follow task records',
  })
})

describe('BillingMethodSteps', () => {
  test('renders three billing steps with correct credit rate footer', () => {
    render(<BillingMethodSteps modelCount={75} />)

    expect(
      screen.getByRole('heading', { name: 'How billing works' })
    ).toBeInTheDocument()
    expect(screen.getByText('Top up credits')).toBeInTheDocument()
    expect(screen.getByText('Call any model')).toBeInTheDocument()
    expect(screen.getByText('Every cent is traceable')).toBeInTheDocument()

    const expectedRate = `1 credit = $${USD_PER_CREDIT.toFixed(2)} USD`
    expect(screen.getByText(expectedRate)).toBeInTheDocument()
    expect(USD_PER_CREDIT).toBe(0.1)
    expect(screen.getByText('One API → 75 models')).toBeInTheDocument()
    expect(
      screen.getByText('Generation credit refunds follow task records')
    ).toBeInTheDocument()
    expect(screen.queryByText('No minimums · failed calls free')).toBeNull()
  })

  test('step 01 drops the unimplemented auto-top-up claim and the hardcoded minimum', () => {
    render(<BillingMethodSteps modelCount={75} />)

    // 自动充值没有实现；最低充值额由 /api/user/topup/info 下发，写死在文案里会漂移。
    expect(screen.queryByText(/auto-top-up/i)).toBeNull()
    expect(screen.queryByText(/min top-up/i)).toBeNull()
    expect(
      screen.getByText('Pay with card, Alipay/WeChat, or wire transfer.')
    ).toBeInTheDocument()
  })

  test('uses a responsive three-column grid class on the step container', () => {
    const { container } = render(<BillingMethodSteps />)
    const grid = container.querySelector('.md\\:grid-cols-3')
    expect(grid).not.toBeNull()
  })
})
