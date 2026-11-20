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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import {
  DESKTOP_RELEASE_ORIGIN,
  LATEST_URLS,
} from '@/features/omnimux-landing/desktop/components/cn/desktop-release'

import { PricingDownloadCta } from '../pricing-download-cta'

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function renderDownloadCta() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
  return render(
    <QueryClientProvider client={client}>
      <PricingDownloadCta />
    </QueryClientProvider>
  )
}

test('links to each stable installer and reports a Windows footer download', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(Response.json({ version: '2.3.4' }))
  )
  const tracker = vi.fn()
  vi.stubGlobal('__OMNIMUX_TRACK__', tracker)

  renderDownloadCta()

  const mac = screen.getByRole('link', { name: 'Download for macOS' })
  const windows = screen.getByRole('link', { name: 'Download for Windows' })
  await waitFor(() => {
    expect(mac).toHaveAttribute(
      'href',
      `${DESKTOP_RELEASE_ORIGIN}/releases/v2.3.4/OmniMux-2.3.4-arm64.dmg`
    )
    expect(windows).toHaveAttribute(
      'href',
      `${DESKTOP_RELEASE_ORIGIN}/releases/v2.3.4/OmniMux-2.3.4-x64-Setup.exe`
    )
  })

  windows.addEventListener('click', (event) => event.preventDefault())
  await userEvent.click(windows)
  await waitFor(() => {
    expect(tracker).toHaveBeenCalledWith('download_cta_click', {
      page: 'pricing',
      cta_id: 'pricing_download_windows',
      cta_position: 'footer',
      cta_variant: 'direct',
      locale: 'en',
      destination: new URL(DESKTOP_RELEASE_ORIGIN).hostname,
    })
  })
})

test('keeps legacy installer links if the stable manifest is temporarily unavailable', async () => {
  const warn = vi.spyOn(console, 'warn').mockImplementation(() => {})
  const fetchManifest = vi
    .fn()
    .mockResolvedValue(new Response(null, { status: 503 }))
  vi.stubGlobal('fetch', fetchManifest)

  renderDownloadCta()

  await waitFor(() => expect(warn).toHaveBeenCalled())
  expect(
    screen.getByRole('link', { name: 'Download for macOS' })
  ).toHaveAttribute('href', LATEST_URLS.arm64)
  expect(
    screen.getByRole('link', { name: 'Download for Windows' })
  ).toHaveAttribute('href', LATEST_URLS.win)
})

test('removes download links when there is no stable release', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn().mockResolvedValue(new Response(null, { status: 404 }))
  )

  renderDownloadCta()

  expect(screen.getByRole('link', { name: 'Download for macOS' })).toBeVisible()
  await waitFor(() =>
    expect(
      screen.queryByRole('link', { name: 'Download for macOS' })
    ).not.toBeInTheDocument()
  )
  expect(
    screen.queryByRole('link', { name: 'Download for Windows' })
  ).not.toBeInTheDocument()
})
