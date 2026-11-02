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
import assert from 'node:assert/strict'

import { render, waitFor } from '@testing-library/react'
import { afterEach, describe, test } from 'vitest'

const { createInstance } = await import('i18next')
const { I18nextProvider, initReactI18next } = await import('react-i18next')
const { QueryClient, QueryClientProvider } =
  await import('@tanstack/react-query')
const { api } = await import('@/lib/api')
const { HostedPoolPage } = await import('../../pages/pool-page')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Hosted channel pool': 'Hosted channel pool',
        'Admin API connected': 'Admin API connected',
        'This page reads live hosted tables. Gateway is not wired. Selling is not open.':
          'This page reads live hosted tables. Gateway is not wired. Selling is not open.',
        'Pool size': 'Pool size',
        'Allocatable stock': 'Allocatable stock',
        'Warming up': 'Warming up',
        'Do not allocate': 'Do not allocate',
        Remark: 'Remark',
        'Phone ID': 'Phone ID',
        Group: 'Group',
        Country: 'Country',
        Status: 'Status',
        Readiness: 'Readiness',
        Buyer: 'Buyer',
        Allocatable: 'Allocatable',
        Unassigned: 'Unassigned',
        Yes: 'Yes',
        No: 'No',
        Reserved: 'Reserved',
        Available: 'Available',
        Assigned: 'Assigned',
        Ready: 'Ready',
        Active: 'Active',
        Quarantined: 'Quarantined',
        'Search pool': 'Search pool',
        'Remark, phone ID, or buyer': 'Remark, phone ID, or buyer',
        'All statuses': 'All statuses',
        'Allocatable only': 'Allocatable only',
        'Toggle allocatable for {{phone}}': 'Toggle allocatable for {{phone}}',
        'Loading hosted data': 'Loading hosted data',
        'No hosted channels yet': 'No hosted channels yet',
        'Sync now': 'Sync now',
        Detail: 'Detail',
        Open: 'Open',
      },
    },
  },
})

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = { get: ApiMethod }
const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get

describe('hosted pool page', () => {
  afterEach(() => {
    apiClient.get = originalGet
  })

  test('renders the live admin banner and reserved monthly seat without credentials', async () => {
    apiClient.get = async () => ({
      data: {
        success: true,
        data: [
          {
            id: 10,
            phone_id: '630202623882428895',
            remark: '10 美区',
            group: '美区',
            country_name: 'USA',
            platform: 'tiktok',
            region: 'us',
            status: 'reserved',
            allocatable: false,
            readiness: 0,
            buyer: null,
            brand: null,
          },
        ],
      },
    })

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const { container: root } = render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <HostedPoolPage />
        </I18nextProvider>
      </QueryClientProvider>
    )
    await waitFor(() => {
      assert.match(root.textContent ?? '', /630202623882428895/)
    })

    assert.match(root.textContent ?? '', /Admin API connected/)
    assert.match(root.textContent ?? '', /630202623882428895/)
    assert.match(root.textContent ?? '', /Search pool/)
    assert.match(root.textContent ?? '', /Allocatable only/)
    assert.match(root.textContent ?? '', /Sync now/)
    assert.match(root.textContent ?? '', /Open/)
    assert.ok(root.querySelector('#hosted-pool-query'))
    assert.ok(root.querySelector('#hosted-pool-status'))
    assert.ok(
      root.querySelector(
        '[aria-label="Toggle allocatable for 630202623882428895"]'
      )
    )
    assert.doesNotMatch(root.textContent ?? '', /credentials/i)
    assert.doesNotMatch(root.textContent ?? '', /password/i)
  })

  test('shows an empty state when the admin pool has no rows', async () => {
    apiClient.get = async () => ({
      data: { success: true, data: [] },
    })

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const { container: root } = render(
      <QueryClientProvider client={queryClient}>
        <I18nextProvider i18n={i18n}>
          <HostedPoolPage />
        </I18nextProvider>
      </QueryClientProvider>
    )
    await waitFor(() => {
      assert.match(root.textContent ?? '', /No hosted channels yet/)
    })

    assert.match(root.textContent ?? '', /No hosted channels yet/)
    assert.doesNotMatch(root.textContent ?? '', /630202623882428895/)
  })
})
