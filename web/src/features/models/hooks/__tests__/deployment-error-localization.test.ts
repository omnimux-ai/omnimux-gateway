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
import { act, renderHook, waitFor } from '@testing-library/react'
import i18next from 'i18next'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { getServerErrorMessage } from '@/lib/server-error-message'

import {
  clearConnectionCache,
  useModelDeploymentSettings,
} from '../use-model-deployment-settings'

afterEach(async () => {
  clearConnectionCache()
  vi.restoreAllMocks()
  await act(() => i18next.changeLanguage('en'))
  i18next.removeResourceBundle('zhCN', 'translation')
})

it('preserves server error parameters for the initial connection check and manual retry', async () => {
  clearConnectionCache()
  i18next.addResourceBundle('zhCN', 'translation', {
    'Task {{task}} failed': '任务 {{task}} 失败',
  })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: { enabled: true } },
  })
  const post = vi.spyOn(api, 'post').mockResolvedValue({
    data: {
      success: false,
      message_key: 'Task {{task}} failed',
      message_params: { task: 'connection-42' },
      message: 'Task connection-42 failed',
    },
  })
  const { result } = renderHook(() => useModelDeploymentSettings())
  await waitFor(() => expect(result.current.connectionOk).toBe(false))
  const error = result.current.connectionError
  expect(error).toBeInstanceOf(Error)
  expect(getServerErrorMessage(error)).toBe('Task connection-42 failed')

  await act(() => i18next.changeLanguage('zhCN'))

  expect(getServerErrorMessage(result.current.connectionError)).toBe(
    '任务 connection-42 失败'
  )
  expect(post).toHaveBeenCalledTimes(1)
  await act(() => result.current.testConnection())
  expect(getServerErrorMessage(result.current.connectionError)).toBe(
    '任务 connection-42 失败'
  )
  expect(post).toHaveBeenCalledTimes(2)
})
