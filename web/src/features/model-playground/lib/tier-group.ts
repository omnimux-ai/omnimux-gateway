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
import type { PricingModel } from '@/features/pricing/types'

import type { PlaygroundParams } from '../types'

/**
 * Resolution tiers of a public video model.
 *
 * A public video model keeps ONE id per capability and realizes its quality
 * tiers as token groups, so the request's resolution is what selects the group
 * that routing and billing both use. This table carries only the group NAMES —
 * the price multiplier is never restated here, it stays in the configured
 * `GroupRatio` map, and the base rate stays in the catalog's `ModelPrice`.
 *
 * fork: wan30-resolution-tier-group — mirrors the gateway's own declaration in
 * `middleware/video_tier_group.go`; both sides must name the same groups.
 */
const TIER_GROUPS: Record<string, Record<string, string>> = {
  'wan-3.0': {
    '720p': 'wan-3.0-720p',
    '1080p': 'wan-3.0-1080p',
  },
}

/** A model's catalog row, as far as tier resolution needs to read it. */
type TierSubject = Pick<PricingModel, 'model_name' | 'enable_groups'>

/**
 * Tier groups the model's public catalog actually provisions.
 *
 * A tier is offered only while its group is enabled for the model: naming an
 * unprovisioned group would pin a request to a group that cannot serve it,
 * where leaving the header off keeps the gateway's previous behaviour.
 */
export function provisionedTierGroups(
  model: TierSubject
): Record<string, string> | undefined {
  const declared = TIER_GROUPS[model.model_name]
  if (!declared) return undefined

  const enabled = new Set(model.enable_groups ?? [])
  const provisioned: Record<string, string> = {}
  for (const [tier, group] of Object.entries(declared)) {
    if (enabled.has(group)) provisioned[tier] = group
  }
  return Object.keys(provisioned).length > 0 ? provisioned : undefined
}

/**
 * Resolutions the provisioned tiers cover, in declaration order.
 *
 * Absent for a model with no tier table and for one whose tiers are not
 * provisioned yet, so callers keep their previous domain in both cases.
 */
export function tierResolutions(model: TierSubject): string[] | undefined {
  const groups = provisionedTierGroups(model)
  return groups ? Object.keys(groups) : undefined
}

/**
 * The tier group the request's resolution selects.
 *
 * Returns undefined when the model declares no tier, when its tiers are not
 * provisioned, or when the value is not one of the declared tiers — the caller
 * then sends no group at all rather than guessing one.
 */
export function tierGroupForResolution(
  model: TierSubject,
  params: PlaygroundParams
): string | undefined {
  const groups = provisionedTierGroups(model)
  if (!groups) return undefined

  const raw = params.resolution
  if (typeof raw !== 'string') return undefined
  const resolution = raw.trim().toLowerCase()
  return resolution ? groups[resolution] : undefined
}
