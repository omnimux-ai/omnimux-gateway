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
along with this program. If not, see <http://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import type { ModelContract, ModelFamily, PricingModel } from '../types'

/** Families that actually have a generation form. */
const PLAYGROUND_FAMILIES: ReadonlySet<ModelFamily> = new Set([
  'video',
  'image',
])

/**
 * Whether the model page should mount the generation form.
 *
 * An unpublished contract returns true, so a model the catalog does not yet
 * describe keeps exactly the behaviour it had before contracts existed. A
 * published non-media family is a definite "no": a text, data or audio model has
 * no aspect ratio, resolution or duration for a form to offer.
 */
export function hasPlayground(model: PricingModel): boolean {
  const family = model.contract?.family
  if (!family) return true
  return PLAYGROUND_FAMILIES.has(family)
}

/** The declared capability family, or undefined when none was published. */
export function declaredFamily(model: PricingModel): ModelFamily | undefined {
  return model.contract?.family
}

/**
 * The declared contract, or undefined when the catalog published none.
 *
 * Callers must treat undefined as "not established" and fall back to their own
 * previous behaviour rather than substituting a default of their own.
 */
export function declaredContract(
  model: PricingModel
): ModelContract | undefined {
  return model.contract
}
