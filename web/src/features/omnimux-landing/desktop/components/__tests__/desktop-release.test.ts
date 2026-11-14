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
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  LATEST_URLS,
  fetchDesktopRelease,
  releaseUrls,
} from '../cn/desktop-release'

function jsonResponse(body: unknown, status = 200): Response {
  return {
    status,
    ok: status >= 200 && status < 300,
    json: () => Promise.resolve(body),
  } as Response
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('fetchDesktopRelease', () => {
  it('pins versioned URLs when the manifest has a version', async () => {
    const fetchImpl = vi
      .fn()
      .mockResolvedValue(
        jsonResponse({ version: '2.0.21' })
      ) as unknown as typeof fetch
    const release = await fetchDesktopRelease(fetchImpl)
    expect(release).toEqual({
      status: 'ready',
      version: '2.0.21',
      urls: {
        arm64:
          'https://dl.omnimux.ai/releases/v2.0.21/OmniMux-2.0.21-arm64.dmg',
        x64: 'https://dl.omnimux.ai/releases/v2.0.21/OmniMux-2.0.21-x64.dmg',
        win: 'https://dl.omnimux.ai/releases/v2.0.21/OmniMux-2.0.21-x64-Setup.exe',
      },
    })
  })

  it('treats a 404 as unavailable so the UI can disable', async () => {
    const fetchImpl = vi
      .fn()
      .mockResolvedValue(jsonResponse({}, 404)) as unknown as typeof fetch
    await expect(fetchDesktopRelease(fetchImpl)).resolves.toEqual({
      status: 'unavailable',
    })
  })

  it('treats a manifest without a version as unavailable', async () => {
    const fetchImpl = vi
      .fn()
      .mockResolvedValue(
        jsonResponse({ version: '' })
      ) as unknown as typeof fetch
    await expect(fetchDesktopRelease(fetchImpl)).resolves.toEqual({
      status: 'unavailable',
    })
  })

  it('falls back to the latest symlinks on a network error', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    const fetchImpl = vi
      .fn()
      .mockRejectedValue(new Error('offline')) as unknown as typeof fetch
    await expect(fetchDesktopRelease(fetchImpl)).resolves.toEqual({
      status: 'fallback',
      urls: LATEST_URLS,
    })
  })

  it('falls back to the latest symlinks on a 5xx', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {})
    const fetchImpl = vi
      .fn()
      .mockResolvedValue(jsonResponse({}, 500)) as unknown as typeof fetch
    await expect(fetchDesktopRelease(fetchImpl)).resolves.toEqual({
      status: 'fallback',
      urls: LATEST_URLS,
    })
  })
})

describe('releaseUrls', () => {
  it('returns null only when the channel is unavailable', () => {
    expect(releaseUrls({ status: 'unavailable' })).toBeNull()
  })

  it('keeps the latest symlinks usable while loading', () => {
    expect(releaseUrls({ status: 'loading' })).toEqual(LATEST_URLS)
  })

  it('uses the versioned URLs once ready', () => {
    const urls = { arm64: 'a', x64: 'x', win: 'w' }
    expect(releaseUrls({ status: 'ready', version: '1', urls })).toEqual(urls)
  })
})
