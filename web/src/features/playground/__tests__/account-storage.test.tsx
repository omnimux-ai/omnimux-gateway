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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  cleanup,
  render,
  renderHook,
  screen,
} from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { STATUS_QUERY_KEY } from '@/lib/status-query'
import { useAuthStore } from '@/stores/auth-store'

import { Playground } from '..'
import { API_ENDPOINTS, STORAGE_KEYS } from '../constants'
import { usePlaygroundState } from '../hooks/use-playground-state'
import { loadMessages, saveConfig, saveMessages } from '../lib/storage/storage'
import type { Message } from '../types'

function messagesFor(content: string): Message[] {
  return [{ key: content, from: 'user', versions: [{ id: content, content }] }]
}

beforeEach(() => {
  localStorage.clear()
  useAuthStore.getState().auth.reset()
})

afterEach(() => {
  cleanup()
  vi.useRealTimers()
  vi.restoreAllMocks()
  localStorage.clear()
  useAuthStore.getState().auth.reset()
})

test('a mounted playground switches visible history and clears it on logout without adopting archived data', async () => {
  const archived = JSON.stringify(messagesFor('Unowned legacy chat'))
  localStorage.setItem(STORAGE_KEYS.MESSAGES, archived)
  saveConfig({ model: 'A model' }, 1)
  saveMessages(messagesFor('Account A private chat'), 1)
  saveMessages(messagesFor('Account B private chat'), 2)
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === API_ENDPOINTS.USER_GROUPS) {
      return { data: { success: true, data: {} } }
    }
    if (url === API_ENDPOINTS.USER_MODELS) {
      return { data: { success: true, data: [] } }
    }
    throw new Error(`Unexpected request: ${url}`)
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  client.setQueryData(STATUS_QUERY_KEY, {})
  useAuthStore.getState().auth.setUser({ id: 1, username: 'A', role: 1 })
  render(
    <QueryClientProvider client={client}>
      <Playground />
    </QueryClientProvider>
  )
  expect(await screen.findByText('Account A private chat')).toBeVisible()

  act(() =>
    useAuthStore.getState().auth.setUser({ id: 2, username: 'B', role: 1 })
  )
  expect(screen.queryByText('Account A private chat')).toBeNull()
  expect(await screen.findByText('Account B private chat')).toBeVisible()

  act(() =>
    useAuthStore.getState().auth.setUser({ id: 1, username: 'A', role: 1 })
  )
  expect(await screen.findByText('Account A private chat')).toBeVisible()
  expect(screen.queryByText('Account B private chat')).toBeNull()

  act(() => useAuthStore.getState().auth.reset())
  expect(screen.queryByText('Account A private chat')).toBeNull()
  expect(screen.queryByText('Account B private chat')).toBeNull()
  expect(screen.queryByText('Unowned legacy chat')).toBeNull()
  expect(localStorage.getItem(STORAGE_KEYS.MESSAGES)).toBe(archived)
  expect(loadMessages(1)).toEqual(messagesFor('Account A private chat'))
  expect(loadMessages(2)).toEqual(messagesFor('Account B private chat'))
  client.clear()
})

test.each(['timer', 'unmount'] as const)(
  'a pending %s save keeps its original owner when the active account changes',
  (flush) => {
    vi.useFakeTimers()
    const other = messagesFor('Account B stays unchanged')
    saveMessages(other, 2)
    useAuthStore.getState().auth.setUser({ id: 1, username: 'A', role: 1 })
    const view = renderHook(() => usePlaygroundState(1))
    act(() => vi.runOnlyPendingTimers())
    act(() =>
      view.result.current.updateMessages(messagesFor('Last edit from A'))
    )
    useAuthStore.getState().auth.setUser({ id: 2, username: 'B', role: 1 })

    if (flush === 'timer') act(() => vi.runOnlyPendingTimers())
    else view.unmount()

    expect(loadMessages(1)).toEqual(messagesFor('Last edit from A'))
    expect(loadMessages(2)).toEqual(other)
  }
)

test('pending writes after logout save only to the original owner and leave shared keys absent', () => {
  vi.useFakeTimers()
  useAuthStore.getState().auth.setUser({ id: 1, username: 'A', role: 1 })
  const view = renderHook(() => usePlaygroundState(1))
  act(() => vi.runOnlyPendingTimers())
  act(() =>
    view.result.current.updateMessages(messagesFor('Last edit before logout'))
  )
  useAuthStore.getState().auth.reset()
  view.unmount()
  expect(loadMessages()).toBeNull()
  expect(loadMessages(1)).toEqual(messagesFor('Last edit before logout'))
  for (const key of Object.values(STORAGE_KEYS)) {
    expect(localStorage.getItem(key)).toBeNull()
  }
})
