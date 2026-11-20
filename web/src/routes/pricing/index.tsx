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
import { createFileRoute, redirect } from '@tanstack/react-router'
import z from 'zod'

import { Pricing } from '@/features/pricing'
import { getModuleAccessForGuard } from '@/lib/nav-modules'
import { useAuthStore } from '@/stores/auth-store'

// Router search parsing can decode a bare numeric vendor ID or quota type as
// a number before validation. Keep their text values for the catalog links.
const catalogSearchText = z
  .union([
    z.string(),
    z.number().transform(String),
    z.boolean().transform(String),
  ])
  .optional()
  .catch(undefined)

const pricingSearchSchema = z.object({
  search: catalogSearchText,
  sort: z.string().optional(),
  vendor: catalogSearchText,
  group: catalogSearchText,
  quotaType: catalogSearchText,
  endpointType: catalogSearchText,
  tag: catalogSearchText,
  tokenUnit: z.enum(['M', 'K']).optional(),
  view: z.enum(['card', 'table']).optional().catch(undefined),
  rechargePrice: z.boolean().optional(),
  modality: z
    .enum(['all', 'text', 'image', 'video', 'audio', 'social'])
    .optional()
    .catch(undefined),
})

export const Route = createFileRoute('/pricing/')({
  validateSearch: pricingSearchSchema,
  beforeLoad: async ({ context, location }) => {
    const access = await getModuleAccessForGuard(context.queryClient, 'pricing')
    if (!access.enabled) {
      throw redirect({ to: '/' })
    }
    if (access.requireAuth) {
      const { auth } = useAuthStore.getState()
      if (!auth.user) {
        throw redirect({
          to: '/sign-in',
          search: { redirect: location.href },
        })
      }
    }
  },
  component: PricingRoute,
})

function PricingRoute() {
  const { search, vendor, modality, tag, group, quotaType, endpointType } =
    Route.useSearch()
  const filters = {
    search,
    vendor,
    modality,
    tag,
    group,
    quotaType,
    endpointType,
  }
  return <Pricing key={JSON.stringify(filters)} search={filters} />
}
