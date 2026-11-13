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
import { api } from '@/lib/api'

import type {
  HostedAccountDetail,
  HostedAccountStatus,
  HostedOrderKind,
  HostedOrderRow,
  HostedOrderStatus,
  HostedPoolRow,
  HostedSkuRow,
  HostedTaskRow,
  HostedTaskStatus,
  HostedTaskType,
} from './types'

type ApiEnvelope<T> = {
  success: boolean
  message?: string
  data?: T
}

type AdminPoolDTO = {
  id: number
  phone_id: string
  remark: string
  group: string
  country_name: string
  platform: string
  region: string
  status: HostedAccountStatus
  allocatable: boolean
  readiness: number
  buyer: string | null
  brand: string | null
}

type AdminOrderDTO = {
  id: number
  user: string
  sku: string
  kind: HostedOrderKind
  status: HostedOrderStatus
  quantity: number
  delivered: number
  amount_usd: number
  deadline: string
}

type AdminTaskDTO = {
  id: number
  channel_id: number
  type: HostedTaskType
  status: HostedTaskStatus
  receipt: { views: number; likes: number; comments: number } | null
  note: string
}

type AdminSkuDTO = {
  id: number
  platform: string
  region: string
  price_usd: number
  enabled: boolean
  stock: number
  open_orders: number
  est_delivery_hours: number
}

async function readList<T>(path: string): Promise<T[]> {
  const res = await api.get<ApiEnvelope<T[]>>(path)
  return res.data.data ?? []
}

export function mapPoolRow(row: AdminPoolDTO): HostedPoolRow {
  return {
    id: row.id,
    phoneId: row.phone_id,
    remark: row.remark,
    group: row.group,
    countryName: row.country_name,
    platform: row.platform,
    region: row.region,
    status: row.status,
    allocatable: row.allocatable,
    readiness: row.readiness,
    buyer: row.buyer,
    brand: row.brand,
  }
}

export function mapOrderRow(row: AdminOrderDTO): HostedOrderRow {
  return {
    id: row.id,
    user: row.user,
    sku: row.sku,
    kind: row.kind,
    status: row.status,
    quantity: row.quantity,
    delivered: row.delivered,
    amountUsd: row.amount_usd ?? 0,
    deadline: row.deadline,
  }
}

export function mapTaskRow(row: AdminTaskDTO): HostedTaskRow {
  return {
    id: row.id,
    channelId: row.channel_id,
    type: row.type,
    status: row.status,
    receipt: row.receipt,
    note: row.note ?? '',
  }
}

export function mapSkuRow(row: AdminSkuDTO): HostedSkuRow {
  return {
    id: row.id,
    platform: row.platform,
    region: row.region,
    priceUsd: row.price_usd,
    enabled: row.enabled,
    stock: row.stock,
    openOrders: row.open_orders,
    estDeliveryHours: row.est_delivery_hours,
  }
}

export async function getAdminPool(): Promise<HostedPoolRow[]> {
  const rows = await readList<AdminPoolDTO>('/api/hosted/admin/v1/pool')
  return rows.map(mapPoolRow)
}

export async function getAdminOrders(): Promise<HostedOrderRow[]> {
  const rows = await readList<AdminOrderDTO>('/api/hosted/admin/v1/orders')
  return rows.map(mapOrderRow)
}

export async function getAdminTasks(): Promise<HostedTaskRow[]> {
  const rows = await readList<AdminTaskDTO>('/api/hosted/admin/v1/tasks')
  return rows.map(mapTaskRow)
}

export async function cancelAdminTask(id: number): Promise<HostedTaskRow> {
  const res = await api.post<ApiEnvelope<AdminTaskDTO>>(
    `/api/hosted/admin/v1/tasks/${id}/cancel`
  )
  if (!res.data.data) {
    throw new Error(res.data.message || 'hosted task cancel failed')
  }
  return mapTaskRow(res.data.data)
}

export async function retryAdminTask(id: number): Promise<HostedTaskRow> {
  const res = await api.post<ApiEnvelope<AdminTaskDTO>>(
    `/api/hosted/admin/v1/tasks/${id}/retry`
  )
  if (!res.data.data) {
    throw new Error(res.data.message || 'hosted task retry failed')
  }
  return mapTaskRow(res.data.data)
}

export async function getAdminSkus(): Promise<HostedSkuRow[]> {
  const rows = await readList<AdminSkuDTO>('/api/hosted/admin/v1/skus')
  return rows.map(mapSkuRow)
}

export async function setAllocatable(
  id: number,
  allocatable: boolean
): Promise<HostedPoolRow> {
  const res = await api.patch<ApiEnvelope<AdminPoolDTO>>(
    `/api/hosted/admin/v1/accounts/${id}/allocatable`,
    { allocatable }
  )
  if (!res.data.data) {
    throw new Error(res.data.message || 'hosted allocatable update failed')
  }
  return mapPoolRow(res.data.data)
}

export async function setSkuEnabled(
  id: number,
  enabled: boolean
): Promise<HostedSkuRow> {
  const res = await api.patch<ApiEnvelope<AdminSkuDTO>>(
    `/api/hosted/admin/v1/skus/${id}`,
    { enabled }
  )
  if (!res.data.data) {
    throw new Error(res.data.message || 'hosted sku update failed')
  }
  return mapSkuRow(res.data.data)
}

export async function syncAdminPool(): Promise<{ upserted: number }> {
  const res = await api.post<ApiEnvelope<{ upserted: number }>>(
    '/api/hosted/admin/v1/sync'
  )
  return { upserted: res.data.data?.upserted ?? 0 }
}

type AdminAccountDetailDTO = AdminPoolDTO & {
  tasks?: AdminTaskDTO[]
}

export function mapAccountDetail(row: AdminAccountDetailDTO): HostedAccountDetail {
  return {
    ...mapPoolRow(row),
    tasks: (row.tasks ?? []).map(mapTaskRow),
  }
}

export async function getAdminAccount(id: number): Promise<HostedAccountDetail> {
  const res = await api.get<ApiEnvelope<AdminAccountDetailDTO>>(
    `/api/hosted/admin/v1/accounts/${id}`
  )
  if (!res.data.data) {
    throw new Error(res.data.message || 'hosted account not found')
  }
  return mapAccountDetail(res.data.data)
}

export const HOSTED_ADMIN_KEYS = {
  pool: ['hosted-admin', 'pool'] as const,
  orders: ['hosted-admin', 'orders'] as const,
  tasks: ['hosted-admin', 'tasks'] as const,
  skus: ['hosted-admin', 'skus'] as const,
  account: (id: number) => ['hosted-admin', 'account', id] as const,
}
