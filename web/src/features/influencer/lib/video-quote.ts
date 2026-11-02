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

import type { PricingResponse } from '../types'

/**
 * Public model id for the workbench's video surface. The gateway sells one id
 * and maps the requested resolution to a tier group (see
 * `controller/influencer_motion.go`), so the quote must use that same group.
 */
export const VIDEO_MODEL = 'seedance-2-5'

/** Resolution → price tier group, mirroring the gateway's pinned routing. */
export const VIDEO_QUOTE_GROUPS: Record<string, string> = {
  '480p': 'seedance-2-5-480p',
  '720p': 'seedance-2-5-720p',
}

/** The gateway rejects output lengths outside this window. */
const MIN_OUTPUT_SECONDS = 4
const MAX_OUTPUT_SECONDS = 30
const CREDITS_PER_USD = 10

export interface VideoQuote {
  usd: number
  credits: number
}

/** The tier group the gateway pins for a resolution, or null when unsupported. */
export function videoQuoteGroup(resolution: string): string | null {
  return VIDEO_QUOTE_GROUPS[resolution] ?? null
}

/**
 * Estimates one character-video generation from the gateway catalog.
 *
 * The gateway bills per second: `ModelPrice × group ratio × seconds`, then
 * converts USD to credits (1 USD = 10 credits). The ratio must come from the
 * account's own groups payload, because a subscription replaces the group ratio
 * with the account's special ratio instead of stacking a second discount.
 *
 * Returns null whenever the catalog cannot price the request; callers hide the
 * number instead of inventing one, and never use the quote to gate submission.
 */
export function quoteVideoCredits(
  pricing: PricingResponse | undefined,
  groups: Record<string, { ratio: number }> | undefined,
  resolution: string,
  seconds: number
): VideoQuote | null {
  if (!pricing || !groups) return null

  const group = videoQuoteGroup(resolution)
  if (!group) return null

  if (
    !Number.isInteger(seconds) ||
    seconds < MIN_OUTPUT_SECONDS ||
    seconds > MAX_OUTPUT_SECONDS
  ) {
    return null
  }

  const row = (pricing.data ?? []).find(
    (model) => model.model_name === VIDEO_MODEL
  )
  if (!row || !row.enable_groups?.includes(group)) return null
  // The flat `model_price` path only matches the gateway's per-second charge.
  // An expression-priced row must be quoted by the expression, not by a guess.
  if (row.billing_mode === 'tiered_expr') return null

  const ratio = groups[group]?.ratio
  if (ratio == null || !Number.isFinite(ratio) || ratio <= 0) return null

  const pricePerSecond = row.model_price
  if (
    pricePerSecond == null ||
    !Number.isFinite(pricePerSecond) ||
    pricePerSecond <= 0
  ) {
    return null
  }

  const usd = pricePerSecond * ratio * seconds
  if (!Number.isFinite(usd)) return null

  return { usd, credits: usd * CREDITS_PER_USD }
}
