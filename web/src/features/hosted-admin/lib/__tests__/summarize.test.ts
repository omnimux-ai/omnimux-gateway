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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  HOSTED_ORDER_FIXTURE,
  HOSTED_POOL_FIXTURE,
  HOSTED_SKU_FIXTURE,
  HOSTED_TASK_FIXTURE,
} from '../../fixtures'
import {
  assertNoPublicCredentials,
  assignCountdown,
  availableStock,
  countByStatus,
  filterPoolRows,
  openOrderCount,
  skuStockTotal,
  toggleAllocatable,
  undeliveredSeats,
} from '../summarize'

describe('hosted admin fixture summaries', () => {
  test('counts allocatable available phones and ignores reserved monthly seats', () => {
    assert.equal(availableStock(HOSTED_POOL_FIXTURE), 2)
    assert.equal(
      HOSTED_POOL_FIXTURE.some((row) => row.phoneId === '630202623882428895'),
      true
    )
    assert.equal(
      HOSTED_POOL_FIXTURE.find((row) => row.phoneId === '630202623882428895')
        ?.allocatable,
      false
    )
  })

  test('sums stock only from enabled SKUs', () => {
    assert.equal(skuStockTotal(HOSTED_SKU_FIXTURE), 2)
    assert.equal(skuStockTotal([]), 0)
  })

  test('counts statuses for empty and mixed rows', () => {
    assert.deepEqual(countByStatus([]), {})
    assert.equal(countByStatus(HOSTED_TASK_FIXTURE).succeeded, 1)
    assert.equal(countByStatus(HOSTED_ORDER_FIXTURE).pending_fulfillment, 1)
  })

  test('filters pool rows by allocatable, status, and query', () => {
    const allocatable = filterPoolRows(HOSTED_POOL_FIXTURE, {
      query: '',
      status: 'all',
      allocatableOnly: true,
    })
    assert.equal(allocatable.length, 2)
    assert.equal(
      allocatable.every((row) => row.allocatable),
      true
    )

    const reserved = filterPoolRows(HOSTED_POOL_FIXTURE, {
      query: '630202623882428895',
      status: 'reserved',
      allocatableOnly: false,
    })
    assert.equal(reserved.length, 1)
    assert.equal(reserved[0]?.phoneId, '630202623882428895')
  })

  test('toggles allocatable on the matching fixture row only', () => {
    const next = toggleAllocatable(HOSTED_POOL_FIXTURE, 10)
    assert.equal(
      next.find((row) => row.id === 10)?.allocatable,
      true
    )
    assert.equal(
      next.find((row) => row.id === 21)?.allocatable,
      true
    )
    assert.equal(HOSTED_POOL_FIXTURE.find((row) => row.id === 10)?.allocatable, false)
  })

  test('counts only undelivered orders as open', () => {
    assert.equal(openOrderCount(HOSTED_ORDER_FIXTURE), 2)
    assert.equal(undeliveredSeats(HOSTED_ORDER_FIXTURE), 2)
    assert.equal(openOrderCount([]), 0)
  })

  test('formats assign countdown as remaining, overdue, or delivered', () => {
    const now = Date.parse('2026-08-19T03:00:00Z')
    assert.deepEqual(
      assignCountdown(
        { delivered: 1, quantity: 2, deadline: '2026-08-19 11:00 UTC' },
        now
      ),
      { kind: 'remaining', hours: 8, minutes: 0 }
    )
    assert.deepEqual(
      assignCountdown(
        { delivered: 0, quantity: 1, deadline: '2026-08-18 18:00 UTC' },
        now
      ),
      { kind: 'overdue', hours: 9, minutes: 0 }
    )
    assert.deepEqual(
      assignCountdown(
        { delivered: 2, quantity: 2, deadline: '2026-08-12 09:00 UTC' },
        now
      ),
      { kind: 'delivered' }
    )
  })

  test('rejects credential fields on public fixture rows', () => {
    assert.deepEqual(
      assertNoPublicCredentials([
        ...HOSTED_POOL_FIXTURE,
        ...HOSTED_ORDER_FIXTURE,
        ...HOSTED_TASK_FIXTURE,
        ...HOSTED_SKU_FIXTURE,
      ]),
      []
    )
    assert.deepEqual(
      assertNoPublicCredentials([{ id: 1, credentials_enc: 'secret' }]),
      ['["id","credentials_enc"]']
    )
  })
})
