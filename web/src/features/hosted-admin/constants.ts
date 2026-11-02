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
  HostedOrderKind,
  HostedOrderStatus,
  HostedTaskStatus,
  HostedTaskType,
} from './types'

export const ACCOUNT_STATUS_LABEL: Record<HostedAccountStatus, string> = {
  procuring: 'Procuring',
  available: 'Available',
  reserved: 'Reserved',
  assigned: 'Assigned',
  warming_up: 'Warming up',
  ready: 'Ready',
  active: 'Active',
  suspended: 'Suspended',
  quarantined: 'Quarantined',
  recycled: 'Recycled',
}

export const ORDER_STATUS_LABEL: Record<HostedOrderStatus, string> = {
  created: 'Created',
  paid: 'Paid',
  allocated: 'Allocated',
  pending_fulfillment: 'Pending fulfillment',
  delivered: 'Delivered',
  cancelled: 'Cancelled',
  refund_partial: 'Partial refund',
}

export const ORDER_KIND_LABEL: Record<HostedOrderKind, string> = {
  initial: 'Initial',
  addon: 'Addon',
  renew: 'Renew',
}

export const TASK_TYPE_LABEL: Record<HostedTaskType, string> = {
  profile_edit: 'Profile edit',
  distribute: 'Distribute',
  internal_warmup: 'Internal warmup',
  internal_metrics: 'Internal metrics',
}

export const TASK_STATUS_LABEL: Record<HostedTaskStatus, string> = {
  pending: 'Pending',
  queued: 'Queued',
  running: 'Running',
  succeeded: 'Succeeded',
  failed: 'Failed',
  cancelled: 'Cancelled',
}

export const DEMO_BANNER_TITLE = 'Admin API connected'
export const DEMO_BANNER_BODY =
  'This page reads live hosted tables. Gateway is not wired. Selling is not open.'
