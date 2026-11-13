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

import { getServerErrorCode } from '@/lib/server-error-message'

describe('getServerErrorCode', () => {
  it('reads the task submission code from the response payload', () => {
    const error = {
      isAxiosError: true,
      response: {
        status: 403,
        data: { code: 'insufficient_user_quota', message: 'quota', data: null },
      },
    }
    expect(getServerErrorCode(error)).toBe('insufficient_user_quota')
  })

  it('reads the OpenAI boundary code nested under error', () => {
    const error = {
      isAxiosError: true,
      response: {
        status: 402,
        data: {
          error: { code: 'insufficient_quota', type: 'insufficient_quota' },
        },
      },
    }
    expect(getServerErrorCode(error)).toBe('insufficient_quota')
  })

  it('reads a plain action failure payload carried as the error cause', () => {
    const error = new Error('failed', {
      cause: { success: false, code: 'AUTH_INTERNAL_ERROR' },
    })
    expect(getServerErrorCode(error)).toBe('AUTH_INTERNAL_ERROR')
  })

  it('ignores the transport code when the server sent no payload code', () => {
    const error = {
      isAxiosError: true,
      code: 'ERR_BAD_RESPONSE',
      response: { data: 'oops' },
    }
    expect(getServerErrorCode(error)).toBeUndefined()
  })

  it('returns undefined for a message-only failure', () => {
    expect(
      getServerErrorCode(new Error('Something went wrong'))
    ).toBeUndefined()
  })
})
