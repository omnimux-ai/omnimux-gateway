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
import { rawQuotaToUsd, usdToCredits } from '@/features/pricing/lib/credits'
import { getServerErrorCode } from '@/lib/server-error-message'

/**
 * Backend codes that mean "the account cannot pay for this request".
 *
 * Ordering note: task submissions answer HTTP 403 with a task error
 * `{ code, message }`, while the OpenAI text boundary rewrites the same local
 * failure to HTTP 402 + `insufficient_quota`. The code set is the only stable
 * discriminator, so callers must never branch on 402/403 alone.
 */
export const INSUFFICIENT_QUOTA_CODES = new Set([
  'insufficient_user_quota',
  'insufficient_quota',
])

/** Raw quota units → product credits (500,000 raw = $1 = 10 credits). */
export function creditsFromQuota(rawQuota: number | null | undefined): number {
  const raw = Number(rawQuota ?? 0)
  if (!Number.isFinite(raw)) return 0
  const credits = usdToCredits(rawQuotaToUsd(raw))
  return Number.isFinite(credits) ? credits : 0
}

/** Which workbench form hit the paywall. */
export type PaywallOrigin = 'image' | 'video' | 'multiview'

/** True when the failure means the account balance cannot cover the request. */
export function isInsufficientQuotaError(error: unknown): boolean {
  const code = getServerErrorCode(error)
  return typeof code === 'string' && INSUFFICIENT_QUOTA_CODES.has(code)
}
