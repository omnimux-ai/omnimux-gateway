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
import type { TFunction } from 'i18next'
import { Boxes, ListChecks, Package, ShoppingCart } from 'lucide-react'

import type { NavGroup, SidebarView } from '../types'

function getHostedAdminNavGroups(t: TFunction): NavGroup[] {
  return [
    {
      id: 'hosted-admin',
      title: t('Hosted channels'),
      items: [
        {
          title: t('Channel pool'),
          url: '/hosted-admin/pool',
          icon: Boxes,
        },
        {
          title: t('Order fulfillment'),
          url: '/hosted-admin/orders',
          icon: ShoppingCart,
        },
        {
          title: t('Task log'),
          url: '/hosted-admin/tasks',
          icon: ListChecks,
        },
        {
          title: t('SKU catalog'),
          url: '/hosted-admin/skus',
          icon: Package,
        },
      ],
    },
  ]
}

export const HOSTED_ADMIN_VIEW: SidebarView = {
  id: 'hosted-admin',
  pathPattern: /^\/hosted-admin(\/|$)/,
  parent: {
    to: '/dashboard/overview',
    label: 'Back to Dashboard',
  },
  getNavGroups: getHostedAdminNavGroups,
}
