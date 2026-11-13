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
const { HostedTasksPage } = await import('../../pages/tasks-page')

const i18n = createInstance()
await i18n.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Hosted task log': 'Hosted task log',
        'Admin API connected': 'Admin API connected',
        'This page reads live hosted tables. Gateway is not wired. Selling is not open.':
          'This page reads live hosted tables. Gateway is not wired. Selling is not open.',
        Running: 'Running',
        Queued: 'Queued',
        Succeeded: 'Succeeded',
        Failed: 'Failed',
        Task: 'Task',
        Channel: 'Channel',
        Type: 'Type',
        Status: 'Status',
        Receipt: 'Receipt',
        Note: 'Note',
        Actions: 'Actions',
        Cancel: 'Cancel',
        Retry: 'Retry',
        Distribute: 'Distribute',
        Pending: 'Pending',
        'No receipt yet': 'No receipt yet',
        'Loading hosted data': 'Loading hosted data',
        'No hosted tasks yet': 'No hosted tasks yet',
      },
    },
  },
})

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = { get: ApiMethod }
const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get

describe('hosted tasks page', () => {
  afterEach(() => {
    apiClient.get = originalGet
  })

  test('shows cancel for pending tasks and retry for failed tasks', async () => {
    apiClient.get = async () => ({
      data: {
        success: true,
        data: [
          {
            id: 11,
            channel_id: 22,
            type: 'distribute',
            status: 'pending',
            receipt: null,
            note: '',
          },
          {
            id: 12,
            channel_id: 22,
            type: 'distribute',
            status: 'failed',
            receipt: null,
            note: 'boom',
          },
          {
            id: 13,
            channel_id: 22,
            type: 'distribute',
            status: 'succeeded',
            receipt: { views: 1, likes: 0, comments: 0 },
            note: '',
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
          <HostedTasksPage />
        </I18nextProvider>
      </QueryClientProvider>
    )
    await waitFor(() => {
      assert.match(root.textContent ?? '', /#11/)
    })

    const text = root.textContent ?? ''
    assert.match(text, /Cancel/)
    assert.match(text, /Retry/)
    assert.match(text, /boom/)
    assert.doesNotMatch(text, /credentials/)
  })
})
