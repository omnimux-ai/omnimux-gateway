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

import { compileBillingExpression } from '@/features/pricing/lib/billing-expression/parser'
import { evaluateBillingExpression } from '@/features/pricing/lib/billing-expression/runtime'
import { quoteFixedImageExpression } from '@/features/pricing/lib/credits'

import type { PricingRow, PricingResponse } from '../types'

export const SUNBURST_MODEL = 'gpt-image-2.5-sunburst'
export const IMAGE_QUOTE_GROUP = 'gpt-image-2.5-sunburst-image'
/**
 * Audited image route that predates the Sunburst workbench. The gateway still
 * sells it (GoEasy economy line), so the workbench keeps generating when the
 * Sunburst route is staged off or otherwise unusable for this account.
 */
export const ECONOMY_MODEL = 'gpt-image-2.5'
export const ECONOMY_QUOTE_GROUP = 'gpt-image-2.5-economy'

const IMAGE_ENDPOINT = 'image-generation'
const IMAGE_TAG = 'Image Generation'

export interface ImageRoute {
  model: string
  group: string
}

/** Preferred order: verified high-quality route first, audited economy after. */
const IMAGE_ROUTES: readonly ImageRoute[] = [
  { model: SUNBURST_MODEL, group: IMAGE_QUOTE_GROUP },
  { model: ECONOMY_MODEL, group: ECONOMY_QUOTE_GROUP },
]

/**
 * Image-generation models, read straight from the gateway pricing payload.
 *
 * The gateway is the single source of truth: whatever it advertises as image
 * generation is offered here, with no local allow/deny list. Adding or
 * delisting a model upstream is therefore reflected without a code change.
 */
export function imageModels(
  pricing: PricingResponse | undefined
): PricingRow[] {
  if (!pricing) return []
  return (pricing.data ?? []).filter(
    (m) =>
      (m.supported_endpoint_types ?? []).includes(IMAGE_ENDPOINT) ||
      (m.tags ?? []).includes(IMAGE_TAG)
  )
}

export function vendorName(
  pricing: PricingResponse | undefined,
  vendorId: number
): string {
  const v = pricing?.vendors?.find((x) => x.id === vendorId)
  return v?.name ?? `Vendor ${vendorId}`
}

/** Estimate one image from the active catalog price; unknown expressions stay blank. */
export function quoteCredits(
  row: PricingRow | undefined,
  group: string | undefined,
  groupRatio: Record<string, number> | undefined
): { usd: number; credits: number } | null {
  if (!row || !group) return null
  const ratio = groupRatio?.[group]
  if (ratio == null || !Number.isFinite(ratio) || ratio < 0) return null
  let unitPrice = row.model_price
  if (row.billing_mode === 'tiered_expr') {
    if (row.model_name === 'gpt-image-2.5-sunburst') {
      return quoteFixedImageExpression(row.billing_expr ?? '', ratio)
    }
    const compiled = compileBillingExpression(row.billing_expr ?? '')
    if (
      compiled.status !== 'ready' ||
      [...compiled.variables].some((name) => name !== 'image_count') ||
      [...compiled.functions].some(
        (name) => name !== 'tier' && name !== 'fixed'
      )
    ) {
      return null
    }
    const evaluated = evaluateBillingExpression(compiled, { imageCount: 1 })
    if (evaluated.status !== 'success' || evaluated.billingUnit !== 'request') {
      return null
    }
    unitPrice = evaluated.cost / 1_000_000
  }
  if (unitPrice == null || !Number.isFinite(unitPrice) || unitPrice < 0) {
    return null
  }
  const usd = unitPrice * ratio
  if (!Number.isFinite(usd)) return null
  return { usd, credits: usd * 10 }
}

/** Existing image price anchor; the gateway owns compatible supplier selection. */
export function pickImageQuoteGroup(
  row: PricingRow | undefined,
  userGroups: Record<string, { ratio: number; desc?: string }> | undefined,
  group: string = IMAGE_QUOTE_GROUP
): string | null {
  const expectedModel =
    group === IMAGE_QUOTE_GROUP ? SUNBURST_MODEL : ECONOMY_MODEL
  if (row?.model_name !== expectedModel) return null
  if (!row.enable_groups?.includes(group) || !userGroups?.[group]) return null
  return group
}

/**
 * The gateway is the source of truth for which image route this account can
 * actually buy. Prefer the verified Sunburst route and fall back to the audited
 * economy route when the gateway does not advertise a usable Sunburst group
 * (route staged off, unavailable supplier, or a broken quote).
 */
export function resolveImageRoute(
  models: PricingRow[],
  userGroups: Record<string, { ratio: number; desc?: string }> | undefined,
  groupRatio: Record<string, number> | undefined
): ImageRoute | null {
  for (const route of IMAGE_ROUTES) {
    const row = models.find((m) => m.model_name === route.model)
    const group = pickImageQuoteGroup(row, userGroups, route.group)
    if (!group) continue
    if (!quoteCredits(row, group, groupRatio)) continue
    return { model: route.model, group }
  }
  return null
}

export interface GroupOption {
  name: string
  ratio: number
  desc?: string
}

/** Groups the picked model can run in, intersected with the user's usable groups. */
export function availableGroups(
  row: PricingRow | undefined,
  userGroups: Record<string, { ratio: number; desc?: string }>,
  groupRatio: Record<string, number> | undefined
): GroupOption[] {
  const names = row?.enable_groups?.length
    ? row.enable_groups
    : Object.keys(userGroups)
  const seen = new Set<string>()
  const out: GroupOption[] = []
  for (const name of names) {
    if (seen.has(name)) continue
    seen.add(name)
    if (userGroups && Object.keys(userGroups).length && !userGroups[name]) {
      continue
    }
    out.push({
      name,
      ratio: groupRatio?.[name] ?? userGroups[name]?.ratio ?? 1,
      desc: userGroups[name]?.desc,
    })
  }
  return out
}
