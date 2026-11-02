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

/** The subscription tiers OmniMux sells, in the order the price page lists them. */
export const PLAN_TIER_IDS = ['discovery', 'starter', 'creator'] as const

export type PlanTierId = (typeof PLAN_TIER_IDS)[number]

/**
 * `upgrade_group` of each tier as configured on the live plans. It is language
 * independent, so it is the first thing the tier resolution looks at. The
 * Discovery plan carries no group at all and is matched by its title instead.
 */
const TIER_BY_UPGRADE_GROUP: Record<string, PlanTierId> = {
  'sub-basic': 'starter',
  'sub-pro': 'creator',
}

/**
 * Tokens that identify a tier inside a plan title, for records that carry no
 * upgrade_group. Both the Chinese titles the database stores (simplified and
 * traditional) and the English product names are recognised.
 */
const TIER_TITLE_TOKENS: Record<PlanTierId, string[]> = {
  discovery: ['体验包', '體驗包', 'discovery'],
  starter: ['入门版', '入門版', 'starter'],
  creator: ['创作版', '創作版', 'creator'],
}

/** The plan fields the tier resolution reads. */
export interface TierIdentifiablePlan {
  title?: string
  upgrade_group?: string
}

/** Lowercase and drop every whitespace run, so titles compare loosely. */
function normalizeTierToken(value: string): string {
  return value.trim().toLowerCase().replaceAll(/\s+/g, '')
}

/**
 * The tier a plan record belongs to, or null when it matches none of the known
 * tiers. The upgrade group wins because it is language independent; the title
 * is only consulted for records that carry no group, such as Discovery.
 */
export function resolvePlanTierId(
  plan: TierIdentifiablePlan
): PlanTierId | null {
  const group = normalizeTierToken(plan.upgrade_group || '')
  const byGroup = group ? TIER_BY_UPGRADE_GROUP[group] : undefined
  if (byGroup) return byGroup

  const title = normalizeTierToken(plan.title || '')
  if (!title) return null
  for (const tier of PLAN_TIER_IDS) {
    if (TIER_TITLE_TOKENS[tier].some((token) => title.includes(token))) {
      return tier
    }
  }
  return null
}

/**
 * The localized display name of a known tier. Plan records store their title in
 * the database in Chinese only, so the name a visitor reads must come from here
 * instead; both the wallet plan cards and the public price page resolve through
 * this function, which is what keeps the two surfaces in step.
 *
 * `zh` / `zh-TW` hold the approved 「中文名 · English name」 form; every other
 * locale holds that locale's own name and no Chinese.
 *
 * Every branch is a literal t('…') call so the i18n extractor registers the key
 * (a dynamic lookup would have to be listed in src/i18n/static-keys.ts).
 */
export function planTierName(tier: PlanTierId, t: TFunction): string {
  switch (tier) {
    case 'discovery':
      return t('plan_tier.discovery')
    case 'starter':
      return t('plan_tier.starter')
    case 'creator':
      return t('plan_tier.creator')
  }
}

/**
 * The name to render for a plan record: the localized tier name when the record
 * is one of the known tiers, otherwise `fallback` — the raw database title, so
 * an unrecognised plan keeps exactly the name it shows today. Never empty.
 */
export function planTierDisplayName(
  plan: TierIdentifiablePlan,
  t: TFunction,
  fallback: string
): string {
  const tier = resolvePlanTierId(plan)
  return (tier ? planTierName(tier, t) : '') || fallback
}
