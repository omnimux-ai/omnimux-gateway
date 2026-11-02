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
import { cleanup, renderHook, waitFor } from '@testing-library/react'
import { createInstance } from 'i18next'
import type { ReactNode } from 'react'
import { I18nextProvider } from 'react-i18next'
import { afterEach, beforeEach, describe, expect, test } from 'vitest'

import { useTopNavLinks } from '@/hooks/use-top-nav-links'
import en from '@/i18n/locales/en.json'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

/**
 * Guards the influencer studio entry: it is a first-class destination that
 * opens the tool page in one click, never a dropdown that hides a single item.
 */

type ApiMethod = (url: string) => Promise<{ data: unknown }>
type MockableApi = { get: ApiMethod }

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get

beforeEach(() => {
  apiClient.get = async (url) => {
    if (url !== '/api/status') throw new Error(`Unexpected GET ${url}`)
    return { data: { success: true, data: { system_name: 'OmniMux' } } }
  }
})

afterEach(() => {
  apiClient.get = originalGet
  cleanup()
})

async function renderLinks() {
  const i18n = createInstance()
  await i18n.init({
    lng: 'en',
    fallbackLng: 'en',
    resources: { en },
    interpolation: { escapeValue: false },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const wrapper = (props: { children: ReactNode }) => (
    <QueryClientProvider client={client}>
      <I18nextProvider i18n={i18n}>{props.children}</I18nextProvider>
    </QueryClientProvider>
  )
  const { result } = renderHook(() => useTopNavLinks(), { wrapper })
  await waitFor(() => {
    expect(
      result.current.some((link) => link.href === '/influencer')
    ).toBe(true)
  })
  return result
}

describe('Top navigation influencer entry', () => {
  test('links straight to the tool page instead of opening a dropdown', async () => {
    const result = await renderLinks()
    const entry = result.current.find((link) => link.href === '/influencer')

    expect(entry?.title).toBe(en.translation['AI Influencer'])
    expect(entry?.items).toBeUndefined()
  })

  test('drops the tools dropdown that only wrapped that single entry', async () => {
    const result = await renderLinks()
    const titles = result.current.map((link) => link.title)

    expect(titles).not.toContain(en.translation['Tools'])
    expect(titles).not.toContain(en.translation['Avatar generator'])
  })
})

describe('Top navigation for a signed-in visitor', () => {
  test('keeps the create entry as the only destination', async () => {
    useAuthStore.getState().auth.setUser({ id: 1, username: 'ops', role: 1 })
    try {
      const result = await renderLinks()

      expect(result.current).toEqual([
        { title: en.translation['Create Video'], href: '/influencer' },
      ])
    } finally {
      useAuthStore.getState().auth.setUser(null)
    }
  })
})
