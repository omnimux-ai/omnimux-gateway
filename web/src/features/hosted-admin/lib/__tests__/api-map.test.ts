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
  mapAccountDetail,
  mapOrderRow,
  mapPoolRow,
  mapSkuRow,
  mapTaskRow,
} from '../../api'
import { assertNoPublicCredentials } from '../summarize'

describe('hosted admin API mappers', () => {
  test('maps pool and order rows without credential fields', () => {
    const pool = mapPoolRow({
      id: 10,
      phone_id: '630202623882428895',
      remark: '10 美区',
      group: '美区',
      country_name: 'USA',
      platform: 'tiktok',
      region: 'us',
      status: 'reserved',
      allocatable: false,
      readiness: 0,
      buyer: null,
      brand: null,
    })
    const order = mapOrderRow({
      id: 801,
      user: 'studio-north',
      sku: 'tiktok / us',
      kind: 'initial',
      status: 'paid',
      quantity: 1,
      delivered: 0,
      amount_usd: 99,
      deadline: '2026-08-19 11:00 UTC',
    })
    const task = mapTaskRow({
      id: 9,
      channel_id: 22,
      type: 'distribute',
      status: 'succeeded',
      receipt: { views: 1, likes: 2, comments: 3 },
      note: 'ok',
    })
    const sku = mapSkuRow({
      id: 1,
      platform: 'tiktok',
      region: 'us',
      price_usd: 99,
      enabled: true,
      stock: 0,
      open_orders: 1,
      est_delivery_hours: 72,
    })

    assert.equal(pool.phoneId, '630202623882428895')
    assert.equal(order.amountUsd, 99)
    assert.equal(task.channelId, 22)
    assert.equal(sku.openOrders, 1)
    const detail = mapAccountDetail({
      id: 22,
      phone_id: 'phone-detail',
      remark: '22 美区',
      group: '美区',
      country_name: 'USA',
      platform: 'tiktok',
      region: 'us',
      status: 'assigned',
      allocatable: false,
      readiness: 10,
      buyer: 'studio-north',
      brand: 'north',
      tasks: [
        {
          id: 9,
          channel_id: 22,
          type: 'distribute',
          status: 'succeeded',
          receipt: { views: 4, likes: 1, comments: 0 },
          note: 'ok',
        },
      ],
    })
    assert.equal(detail.tasks.length, 1)
    assert.deepEqual(
      assertNoPublicCredentials([pool, order, task, sku, detail, ...detail.tasks]),
      []
    )
  })
})
