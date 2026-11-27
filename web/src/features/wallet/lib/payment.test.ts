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
import { describe, expect, test, vi } from 'vitest'

import { PAYMENT_TYPES } from '../constants'
import {
  dispatchSelectedPayment,
  isStripePayment,
  isWaffoPayment,
  isWaffoPancakePayment,
  submitPaymentForm,
} from './payment'

describe('payment type classification', () => {
  test('keeps Waffo and Waffo Pancake on their dedicated flows', () => {
    expect(isWaffoPayment(PAYMENT_TYPES.WAFFO)).toBe(true)
    expect(isWaffoPayment(PAYMENT_TYPES.WAFFO_PANCAKE)).toBe(false)
    expect(isWaffoPancakePayment(PAYMENT_TYPES.WAFFO_PANCAKE)).toBe(true)
    expect(isWaffoPancakePayment(PAYMENT_TYPES.WAFFO)).toBe(false)
    expect(isStripePayment(PAYMENT_TYPES.STRIPE)).toBe(true)
  })
})

describe('payment dispatch', () => {
  test('keeps the selected Waffo method index through confirmation', async () => {
    const calls: string[] = []
    const success = await dispatchSelectedPayment(
      { name: 'Waffo Card', type: PAYMENT_TYPES.WAFFO },
      120,
      3,
      {
        regular: async () => {
          calls.push('regular')
          return false
        },
        waffo: async (amount, index) => {
          calls.push(`waffo:${amount}:${index}`)
          return true
        },
        waffoPancake: async () => {
          calls.push('pancake')
          return false
        },
      }
    )

    expect(success).toBe(true)
    expect(calls).toEqual(['waffo:120:3'])
  })

  test('does not create a Waffo order without a selected method index', async () => {
    let called = false
    const success = await dispatchSelectedPayment(
      { name: 'Waffo Card', type: PAYMENT_TYPES.WAFFO },
      120,
      null,
      {
        regular: async () => false,
        waffo: async () => {
          called = true
          return true
        },
        waffoPancake: async () => false,
      }
    )

    expect(success).toBe(false)
    expect(called).toBe(false)
  })
})

describe('payment form submission', () => {
  test('rejects non-HTTP(S) and control-character form action URLs', () => {
    const submitSpy = vi
      .spyOn(HTMLFormElement.prototype, 'submit')
      .mockImplementation(() => {})

    const unsafeUrls = [
      '',
      'javascript:alert(1)',
      'data:text/html,<script>alert(1)</script>',
      '/relative/checkout',
      'file:///etc/passwd',
      'https://pay.example.com/\r\ncheckout',
    ]

    for (const url of unsafeUrls) {
      expect(submitPaymentForm(url, { order_id: 'ord_1' })).toBe(false)
    }

    expect(submitSpy).not.toHaveBeenCalled()
  })

  test('submits valid HTTPS checkout forms with opener isolation', () => {
    const appendChild = document.body.appendChild.bind(document.body)
    let createdForm: HTMLFormElement | null = null
    vi.spyOn(document.body, 'appendChild').mockImplementation((node) => {
      if (node instanceof HTMLFormElement) {
        createdForm = node
      }
      return appendChild(node)
    })
    const submitSpy = vi
      .spyOn(HTMLFormElement.prototype, 'submit')
      .mockImplementation(() => {})

    const submitted = submitPaymentForm('https://pay.example.com/checkout', {
      order_id: 'ord_42',
    })

    expect(submitted).toBe(true)
    expect(submitSpy).toHaveBeenCalledTimes(1)
    if (!createdForm) {
      throw new Error('Expected payment form to be appended')
    }
    const form: HTMLFormElement = createdForm
    expect(form.action).toBe('https://pay.example.com/checkout')
    expect(form.method).toBe('post')
    expect(form.target).toBe('_blank')
    expect(form.rel).toBe('noopener noreferrer')
    expect(document.body.contains(form)).toBe(false)
  })
})
