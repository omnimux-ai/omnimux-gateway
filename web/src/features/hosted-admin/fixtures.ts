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
import type {
  HostedOrderRow,
  HostedPoolRow,
  HostedSkuRow,
  HostedTaskRow,
} from './types'

export const HOSTED_POOL_FIXTURE: HostedPoolRow[] = [
  {
    id: 10,
    phoneId: '630202623882428895',
    remark: '10 美区',
    group: '美区',
    countryName: 'USA',
    platform: 'tiktok',
    region: 'us',
    status: 'reserved',
    allocatable: false,
    readiness: 0,
    buyer: null,
    brand: null,
  },
  {
    id: 21,
    phoneId: '631031944452248713',
    remark: '21 美区',
    group: '美区',
    countryName: 'USA',
    platform: 'tiktok',
    region: 'us',
    status: 'available',
    allocatable: true,
    readiness: 0,
    buyer: null,
    brand: null,
  },
  {
    id: 22,
    phoneId: '631031944452248714',
    remark: '22 美区',
    group: '美区',
    countryName: 'USA',
    platform: 'tiktok',
    region: 'us',
    status: 'warming_up',
    allocatable: false,
    readiness: 42,
    buyer: 'studio-north',
    brand: 'North Lab',
  },
  {
    id: 23,
    phoneId: '631031944452248715',
    remark: '23 美区',
    group: '美区',
    countryName: 'USA',
    platform: 'tiktok',
    region: 'us',
    status: 'assigned',
    allocatable: false,
    readiness: 8,
    buyer: 'studio-north',
    brand: 'North Lab',
  },
  {
    id: 24,
    phoneId: '631031944452248716',
    remark: '24 美区',
    group: '美区',
    countryName: 'USA',
    platform: 'tiktok',
    region: 'us',
    status: 'ready',
    allocatable: false,
    readiness: 100,
    buyer: 'harbor-co',
    brand: 'Harbor Goods',
  },
  {
    id: 25,
    phoneId: '631031944452248717',
    remark: '25 美区',
    group: '美区',
    countryName: 'USA',
    platform: 'tiktok',
    region: 'us',
    status: 'active',
    allocatable: false,
    readiness: 100,
    buyer: 'harbor-co',
    brand: 'Harbor Goods',
  },
  {
    id: 26,
    phoneId: '631031944452248718',
    remark: '26 美区',
    group: '美区',
    countryName: 'USA',
    platform: 'tiktok',
    region: 'us',
    status: 'available',
    allocatable: true,
    readiness: 0,
    buyer: null,
    brand: null,
  },
  {
    id: 27,
    phoneId: '631031944452248719',
    remark: '27 美区',
    group: '美区',
    countryName: 'USA',
    platform: 'tiktok',
    region: 'us',
    status: 'quarantined',
    allocatable: false,
    readiness: 0,
    buyer: null,
    brand: null,
  },
]

export const HOSTED_ORDER_FIXTURE: HostedOrderRow[] = [
  {
    id: 801,
    user: 'studio-north',
    sku: 'tiktok / us',
    kind: 'initial',
    status: 'pending_fulfillment',
    quantity: 2,
    delivered: 1,
    amountUsd: 198,
    deadline: '2026-08-19 11:00 UTC',
  },
  {
    id: 802,
    user: 'harbor-co',
    sku: 'tiktok / us',
    kind: 'initial',
    status: 'delivered',
    quantity: 2,
    delivered: 2,
    amountUsd: 198,
    deadline: '2026-08-12 09:00 UTC',
  },
  {
    id: 803,
    user: 'harbor-co',
    sku: 'tiktok / us',
    kind: 'addon',
    status: 'paid',
    quantity: 1,
    delivered: 0,
    amountUsd: 50,
    deadline: '2026-08-19 18:00 UTC',
  },
]

export const HOSTED_TASK_FIXTURE: HostedTaskRow[] = [
  {
    id: 9101,
    channelId: 22,
    type: 'internal_warmup',
    status: 'running',
    receipt: null,
    note: 'Day 3 browse',
  },
  {
    id: 9102,
    channelId: 25,
    type: 'distribute',
    status: 'succeeded',
    receipt: { views: 1280, likes: 46, comments: 7 },
    note: 'Distributed',
  },
  {
    id: 9103,
    channelId: 24,
    type: 'profile_edit',
    status: 'queued',
    receipt: null,
    note: 'Brand snapshot pending',
  },
  {
    id: 9104,
    channelId: 27,
    type: 'internal_metrics',
    status: 'failed',
    receipt: null,
    note: 'Analytics timeout',
  },
]

export const HOSTED_SKU_FIXTURE: HostedSkuRow[] = [
  {
    id: 1,
    platform: 'tiktok',
    region: 'us',
    priceUsd: 99,
    enabled: true,
    stock: 2,
    openOrders: 2,
    estDeliveryHours: 72,
  },
  {
    id: 2,
    platform: 'tiktok',
    region: 'sea',
    priceUsd: 99,
    enabled: false,
    stock: 0,
    openOrders: 0,
    estDeliveryHours: 72,
  },
]
