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
export type HostedAccountStatus =
  | 'procuring'
  | 'available'
  | 'reserved'
  | 'assigned'
  | 'warming_up'
  | 'ready'
  | 'active'
  | 'suspended'
  | 'quarantined'
  | 'recycled'

export type HostedOrderStatus =
  | 'created'
  | 'paid'
  | 'allocated'
  | 'pending_fulfillment'
  | 'delivered'
  | 'cancelled'
  | 'refund_partial'

export type HostedOrderKind = 'initial' | 'addon' | 'renew'

export type HostedTaskType =
  | 'profile_edit'
  | 'distribute'
  | 'internal_warmup'
  | 'internal_metrics'

export type HostedTaskStatus =
  | 'pending'
  | 'queued'
  | 'running'
  | 'succeeded'
  | 'failed'
  | 'cancelled'

export type HostedPoolRow = {
  id: number
  phoneId: string
  remark: string
  group: string
  countryName: string
  platform: string
  region: string
  status: HostedAccountStatus
  allocatable: boolean
  readiness: number
  buyer: string | null
  brand: string | null
}

export type HostedOrderRow = {
  id: number
  user: string
  sku: string
  kind: HostedOrderKind
  status: HostedOrderStatus
  quantity: number
  delivered: number
  amountUsd: number
  deadline: string
}

export type HostedTaskReceipt = {
  views: number
  likes: number
  comments: number
}

export type HostedTaskRow = {
  id: number
  channelId: number
  type: HostedTaskType
  status: HostedTaskStatus
  receipt: HostedTaskReceipt | null
  note: string
}

export type HostedSkuRow = {
  id: number
  platform: string
  region: string
  priceUsd: number
  enabled: boolean
  stock: number
  openOrders: number
  estDeliveryHours: number
}

export type HostedAccountDetail = HostedPoolRow & {
  tasks: HostedTaskRow[]
}
