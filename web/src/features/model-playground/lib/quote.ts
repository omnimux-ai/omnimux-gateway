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
import { splitBillingExprAndRequestRules } from '@/features/pricing/lib/billing-expr'
import { usdToCredits } from '@/features/pricing/lib/credits'
import {
  evaluateTaskVisualConfig,
  tryParseTaskVisualConfig,
} from '@/features/pricing/lib/task-expr'
import type { BillingUsageSchema } from '@/features/pricing/types'

import type {
  PlaygroundModel,
  PlaygroundParamSchema,
  PlaygroundParams,
} from '../types'
import { isPerCall } from './billing-dimension'

export type PlaygroundQuote = {
  usd: number
  credits: number
}

function toQuote(usd: number): PlaygroundQuote | null {
  if (!Number.isFinite(usd) || usd < 0) return null
  return { usd, credits: usdToCredits(usd) }
}

/**
 * Projects form parameters onto the billing usage schema.
 *
 * Control keys reuse the catalog field names wherever the two overlap, so this
 * stays a direct projection rather than a second mapping table. Fields the
 * caller never collected are simply absent — the evaluator then refuses to
 * price the request, which is the intended behaviour.
 */
function buildUsageFacts(
  params: PlaygroundParams,
  schema: BillingUsageSchema
): Record<string, string | number> {
  const facts: Record<string, string | number> = {}
  for (const [field, definition] of Object.entries(schema)) {
    const raw = params[field]
    if (raw === undefined || raw === null) continue
    if (Array.isArray(raw)) continue
    if (definition.type === 'boolean' || typeof raw === 'boolean') {
      facts[field] = raw === true || raw === 'true' ? 1 : 0
      continue
    }
    if (typeof raw === 'number') {
      facts[field] = raw
      continue
    }
    const numeric = Number(raw)
    facts[field] =
      definition.type === 'number' && Number.isFinite(numeric) ? numeric : raw
  }
  return facts
}

/**
 * Pre-submit price estimate.
 *
 * Returns null whenever the price cannot be derived from the catalog instead of
 * inventing a number; callers render a placeholder in that case. Credits follow
 * the platform rule 1 USD = 10 credits.
 */
export function estimateQuote(
  model: PlaygroundModel,
  params: PlaygroundParams,
  schema: PlaygroundParamSchema,
  groupRatio = 1
): PlaygroundQuote | null {
  const ratioMultiplier =
    Number.isFinite(groupRatio) && groupRatio > 0 ? groupRatio : 1
  const usageSchema = model.billing_usage_schema
  const expression = model.billing_expr

  if (
    model.billing_mode === 'tiered_expr' &&
    expression &&
    usageSchema &&
    Object.keys(usageSchema).length > 0
  ) {
    const { billingExpr } = splitBillingExprAndRequestRules(expression)
    const config = tryParseTaskVisualConfig(billingExpr, usageSchema)
    if (!config) return null
    const facts = buildUsageFacts(params, usageSchema)
    const result = evaluateTaskVisualConfig(config, facts, usageSchema)
    if (!result) return null
    return toQuote(result.total * ratioMultiplier)
  }

  if (model.model_name === 'wan-3.0') {
    // fork: wan30-resolution-tier-group — the public ID carries ONE base rate
    // (`ModelPrice`, USD per output second) and its quality tiers are token
    // groups, so the quote is `base rate × group ratio × duration`. The ratio
    // arrives in `groupRatio` from the configured group table; no tier price is
    // restated here. A catalog row with no base rate stays unpriced rather than
    // being given one.
    const rate = Number(model.model_price)
    if (!Number.isFinite(rate) || rate <= 0) return null
    const duration = Number(params.duration) || 5
    return toQuote(rate * duration * ratioMultiplier)
  }

  // A per-call video model charges `model_price` once per invocation, so the
  // flat price is the whole quote. A per-second model carries a rate there
  // instead, and only the tiered expression above can price it — a flat read of
  // that rate would understate the charge, so refuse rather than mislead.
  const perSecondVideo =
    schema.modality === 'video' && !isPerCall(model.model_name)

  if (
    model.quota_type === 1 &&
    !perSecondVideo &&
    Number.isFinite(model.model_price) &&
    (model.model_price ?? 0) > 0
  ) {
    return toQuote((model.model_price as number) * ratioMultiplier)
  }

  return null
}
