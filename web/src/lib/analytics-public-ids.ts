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

export const PUBLIC_ANALYTICS_ID_FIELDS = [
  'modelId',
  'vendorId',
  'preset',
] as const
export type PublicAnalyticsId = (typeof PUBLIC_ANALYTICS_ID_FIELDS)[number]

// The app entry and the lazy adapter have separate module caches. Share only
// the latest public ID sets on the page while the route or adapter loads.
const publicIdsKey = Symbol.for('omnimux.analytics.public-ids')
type AnalyticsGlobal = typeof globalThis & {
  [publicIdsKey]?: Partial<Record<PublicAnalyticsId, ReadonlySet<string>>>
}

export function setPublicAnalyticsIds(
  field: PublicAnalyticsId,
  ids: readonly string[]
): void {
  const scope = globalThis as AnalyticsGlobal
  const publicIds = (scope[publicIdsKey] ??= {})
  publicIds[field] = new Set(ids)
}

export function hasPublicAnalyticsId(
  field: PublicAnalyticsId,
  id: string
): boolean {
  const scope = globalThis as AnalyticsGlobal
  return scope[publicIdsKey]?.[field]?.has(id) ?? false
}
