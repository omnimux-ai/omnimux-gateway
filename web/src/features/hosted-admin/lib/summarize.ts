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
  HostedAccountStatus,
  HostedOrderRow,
  HostedPoolRow,
  HostedSkuRow,
} from '../types'

export type PoolStatusFilter = 'all' | HostedAccountStatus

export type PoolFilters = {
  query: string
  status: PoolStatusFilter
  allocatableOnly: boolean
}

export type AssignCountdown =
  | { kind: 'remaining'; hours: number; minutes: number }
  | { kind: 'overdue'; hours: number; minutes: number }
  | { kind: 'delivered' }
  | { kind: 'invalid' }

const CREDENTIAL_KEYS = [
  'credentials',
  'credentials_enc',
  'password',
  'totp',
  'secret',
] as const

export function countByStatus<T extends { status: string }>(
  rows: T[]
): Record<string, number> {
  return rows.reduce<Record<string, number>>((acc, row) => {
    acc[row.status] = (acc[row.status] ?? 0) + 1
    return acc
  }, {})
}

export function availableStock(rows: HostedPoolRow[]): number {
  return rows.filter((row) => row.status === 'available' && row.allocatable)
    .length
}

export function skuStockTotal(rows: HostedSkuRow[]): number {
  return rows
    .filter((row) => row.enabled)
    .reduce((sum, row) => sum + row.stock, 0)
}

export function filterPoolRows(
  rows: HostedPoolRow[],
  filters: PoolFilters
): HostedPoolRow[] {
  const query = filters.query.trim().toLowerCase()

  return rows.filter((row) => {
    if (filters.status !== 'all' && row.status !== filters.status) {
      return false
    }
    if (filters.allocatableOnly && !row.allocatable) {
      return false
    }
    if (query.length === 0) {
      return true
    }

    const haystack = [
      row.remark,
      row.phoneId,
      row.group,
      row.countryName,
      row.buyer ?? '',
      row.brand ?? '',
    ]
      .join(' ')
      .toLowerCase()

    return haystack.includes(query)
  })
}

export function toggleAllocatable(
  rows: HostedPoolRow[],
  id: number
): HostedPoolRow[] {
  return rows.map((row) =>
    row.id === id ? { ...row, allocatable: !row.allocatable } : row
  )
}

export function openOrderCount(rows: HostedOrderRow[]): number {
  return rows.filter(
    (row) => row.delivered < row.quantity && row.status !== 'cancelled'
  ).length
}

export function undeliveredSeats(rows: HostedOrderRow[]): number {
  return rows.reduce(
    (sum, row) => sum + Math.max(row.quantity - row.delivered, 0),
    0
  )
}

export function parseDeadlineUtc(deadline: string): number | null {
  const match = /^(\d{4}-\d{2}-\d{2}) (\d{2}:\d{2}) UTC$/.exec(deadline)
  if (!match) {
    return null
  }

  const ms = Date.parse(`${match[1]}T${match[2]}:00Z`)
  return Number.isNaN(ms) ? null : ms
}

export function assignCountdown(
  row: Pick<HostedOrderRow, 'delivered' | 'quantity' | 'deadline'>,
  nowMs: number
): AssignCountdown {
  if (row.delivered >= row.quantity) {
    return { kind: 'delivered' }
  }

  const deadlineMs = parseDeadlineUtc(row.deadline)
  if (deadlineMs === null) {
    return { kind: 'invalid' }
  }

  const diffMs = deadlineMs - nowMs
  const absMs = Math.abs(diffMs)
  const hours = Math.floor(absMs / 3_600_000)
  const minutes = Math.floor((absMs % 3_600_000) / 60_000)

  if (diffMs >= 0) {
    return { kind: 'remaining', hours, minutes }
  }
  return { kind: 'overdue', hours, minutes }
}

export function publicRowHasCredentials(row: object): boolean {
  return Object.keys(row).some((key) =>
    CREDENTIAL_KEYS.includes(key as (typeof CREDENTIAL_KEYS)[number])
  )
}

export function assertNoPublicCredentials(rows: object[]): string[] {
  return rows
    .filter((row) => publicRowHasCredentials(row))
    .map((row) => JSON.stringify(Object.keys(row)))
}
