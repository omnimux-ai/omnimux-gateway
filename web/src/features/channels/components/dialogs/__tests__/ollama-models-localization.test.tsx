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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import i18next from 'i18next'
import { useEffect } from 'react'
import { toast } from 'sonner'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { channelSchema } from '../../../types'
import { ChannelsProvider, useChannels } from '../../channels-provider'
import { OllamaModelsDialog } from '../ollama-models-dialog'

const channel = channelSchema.parse({
  id: 41,
  type: 4,
  key: '',
  status: 1,
  name: 'Local test',
  base_url: 'http://127.0.0.1:11434',
  created_time: 1,
  test_time: 0,
  response_time: 0,
  balance_updated_time: 0,
})

function DialogHarness() {
  const { currentRow, setCurrentRow } = useChannels()
  useEffect(() => {
    setCurrentRow(channel)
  }, [setCurrentRow])
  return (
    <OllamaModelsDialog open={currentRow !== null} onOpenChange={vi.fn()} />
  )
}

afterEach(async () => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
  useAuthStore.getState().auth.reset()
  await i18next.changeLanguage('en')
})

it.each([true, false])(
  'keeps the SSE success model intact with translation metadata=%s and finishes refreshing',
  async (metadata) => {
    const key = 'Model {{model}} pulled successfully'
    const model = 'local:{{x}}-$t(Pull)'
    const original = `Model ${model} pulled successfully`
    i18next.addResourceBundle('vi', 'translation', {
      [key]: 'Đã tải mô hình {{model}} thành công',
      Pull: 'Tải',
    })
    await i18next.changeLanguage('vi')
    useAuthStore.setState((state) => ({
      auth: {
        ...state.auth,
        accessToken: 'local-test-token',
        accessExpiresAt: 9999999999,
      },
    }))
    const list = vi
      .spyOn(api, 'get')
      .mockResolvedValue({ data: { success: true, data: [] } })
    const event = {
      message: original,
      ...(metadata ? { message_key: key, message_params: { model } } : {}),
    }
    const fetch = vi.fn<typeof globalThis.fetch>(
      async () =>
        new Response(`data: ${JSON.stringify(event)}\n\ndata: [DONE]\n\n`, {
          status: 200,
          headers: { 'Content-Type': 'text/event-stream' },
        })
    )
    vi.stubGlobal('fetch', fetch)
    const success = vi.spyOn(toast, 'success').mockReturnValue('success')
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const invalidate = vi.spyOn(client, 'invalidateQueries')
    render(
      <QueryClientProvider client={client}>
        <ChannelsProvider>
          <DialogHarness />
        </ChannelsProvider>
      </QueryClientProvider>
    )
    const input = await screen.findByRole('textbox', { name: 'Pull model' })
    await waitFor(() => {
      expect(list).toHaveBeenCalledTimes(1)
    })
    fireEvent.change(input, { target: { value: model } })
    fireEvent.click(screen.getByRole('button', { name: 'Tải' }))

    await waitFor(() => {
      expect(success).toHaveBeenCalledWith(
        metadata ? `Đã tải mô hình ${model} thành công` : original
      )
      expect(list).toHaveBeenCalledTimes(2)
    })
    expect(success).toHaveBeenCalledTimes(1)
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(fetch.mock.calls[0]?.[0]).toBe('/api/channel/ollama/pull/stream')
    expect(invalidate).toHaveBeenCalled()
    expect(input).toHaveValue('')
    expect(screen.getByRole('button', { name: 'Tải' })).toBeEnabled()
    client.clear()
  }
)
