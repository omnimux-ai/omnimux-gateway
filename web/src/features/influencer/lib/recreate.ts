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
import type { RecreateMeta } from '../types'

/**
 * The generation quote for a recreation, straight from the server's pricing
 * map. `usd` only satisfies the shared GenerateBar quote shape; the bar renders
 * credits. Returns null when the server supplied no price for that tier.
 */
export function recreateQuoteFor(
  meta: RecreateMeta | null | undefined,
  resolution: string
): { usd: number; credits: number } | null {
  const pricing = meta?.pricing
  let credits: number | undefined
  if (resolution === '720p') credits = pricing?.['720p']
  if (resolution === '480p') credits = pricing?.['480p']
  if (typeof credits !== 'number' || !Number.isFinite(credits)) return null
  return { usd: 0, credits }
}
