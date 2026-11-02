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
import { describe, expect, it } from 'vitest'

import {
  creditsFromQuota,
  INSUFFICIENT_QUOTA_CODES,
  isInsufficientQuotaError,
} from '../lib/insufficient-credits'

describe('creditsFromQuota', () => {
  it('converts the product credit scale (500,000 raw quota = 10 credits)', () => {
    expect(creditsFromQuota(500_000)).toBe(10)
    expect(creditsFromQuota(50_000)).toBe(1)
    expect(creditsFromQuota(7_350)).toBeCloseTo(0.147, 6)
  })

  it('treats missing or malformed quotas as zero', () => {
    expect(creditsFromQuota(null)).toBe(0)
    expect(creditsFromQuota(undefined)).toBe(0)
    expect(creditsFromQuota(Number.NaN)).toBe(0)
  })
})

describe('isInsufficientQuotaError', () => {
  it('matches the task submission quota code', () => {
    const error = {
      isAxiosError: true,
      response: {
        status: 403,
        data: { code: 'insufficient_user_quota', message: 'quota' },
      },
    }
    expect(isInsufficientQuotaError(error)).toBe(true)
  })

  it('matches the OpenAI boundary 402 body', () => {
    const error = {
      isAxiosError: true,
      response: {
        status: 402,
        data: {
          error: { code: 'insufficient_quota', type: 'insufficient_quota' },
        },
      },
    }
    expect(isInsufficientQuotaError(error)).toBe(true)
  })

  it('matches a bare task error payload without transport metadata', () => {
    expect(isInsufficientQuotaError({ code: 'insufficient_user_quota' })).toBe(
      true
    )
  })

  it('does not treat other failures as quota failures', () => {
    expect(
      isInsufficientQuotaError({
        isAxiosError: true,
        response: { status: 403, data: { code: 'AUTH_SESSION_LIMIT' } },
      })
    ).toBe(false)
    expect(isInsufficientQuotaError(new Error('network down'))).toBe(false)
    expect(isInsufficientQuotaError(null)).toBe(false)
  })

  it('keeps the public error-code set stable', () => {
    expect([...INSUFFICIENT_QUOTA_CODES].sort()).toEqual([
      'insufficient_quota',
      'insufficient_user_quota',
    ])
  })
})
