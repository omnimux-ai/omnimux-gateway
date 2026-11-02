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

import { detectMacArch, detectPlatform } from '../cn/detect-platform'

const MAC_UA =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36'
const WIN_UA =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36'
const IPHONE_UA =
  'Mozilla/5.0 (iPhone; CPU iPhone OS 17_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Mobile/15E148 Safari/604.1'
const IPAD_DESKTOP_UA =
  'Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.0 Safari/605.1.15'
const ANDROID_UA =
  'Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36'
const LINUX_UA =
  'Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36'

function stubNavigator(init: {
  userAgent: string
  platform?: string
  userAgentDataPlatform?: string
  userAgentDataArchitecture?: string | null
  maxTouchPoints?: number
}) {
  const userAgentData =
    init.userAgentDataPlatform || init.userAgentDataArchitecture !== undefined
      ? {
          platform: init.userAgentDataPlatform ?? '',
          getHighEntropyValues: () =>
            init.userAgentDataArchitecture === null
              ? Promise.reject(new Error('unsupported hint'))
              : Promise.resolve({ architecture: init.userAgentDataArchitecture }),
        }
      : undefined
  vi.stubGlobal('navigator', {
    userAgent: init.userAgent,
    platform: init.platform ?? '',
    maxTouchPoints: init.maxTouchPoints ?? 0,
    ...(userAgentData ? { userAgentData } : {}),
  })
}

// Wrap the Mac-arm detection fixtures so the stubbed navigator reports a
// desktop Mac; every detectMacArch case needs that base.
function stubMacNavigator(
  overrides: Omit<Parameters<typeof stubNavigator>[0], 'userAgent'> = {}
) {
  stubNavigator({ userAgent: MAC_UA, platform: 'MacIntel', ...overrides })
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('detectPlatform', () => {
  it('picks the Windows installer for a Windows desktop', () => {
    stubNavigator({ userAgent: WIN_UA, platform: 'Win32' })
    expect(detectPlatform()).toBe('win')
  })

  it('picks the macOS installer for an Apple desktop', () => {
    stubNavigator({ userAgent: MAC_UA, platform: 'MacIntel' })
    expect(detectPlatform()).toBe('mac')
  })

  it('trusts userAgentData.platform when the UA string is ambiguous', () => {
    // A UA-reduced browser drops the OS token; the structured hint still knows.
    stubNavigator({
      userAgent: 'Mozilla/5.0 AppleWebKit/537.36 (KHTML, like Gecko) Safari/537.36',
      platform: '',
      userAgentDataPlatform: 'Windows',
    })
    expect(detectPlatform()).toBe('win')
  })

  it('refuses to guess on a phone', () => {
    stubNavigator({ userAgent: IPHONE_UA, platform: 'iPhone' })
    expect(detectPlatform()).toBeNull()
  })

  it('refuses to guess on Android', () => {
    stubNavigator({ userAgent: ANDROID_UA, platform: 'Linux armv8l' })
    expect(detectPlatform()).toBeNull()
  })

  it('treats a touch-enabled Mac UA as iPadOS, not macOS', () => {
    stubNavigator({
      userAgent: IPAD_DESKTOP_UA,
      platform: 'MacIntel',
      maxTouchPoints: 5,
    })
    expect(detectPlatform()).toBeNull()
  })

  it('refuses to guess on a desktop it does not recognise', () => {
    stubNavigator({ userAgent: LINUX_UA, platform: 'Linux x86_64' })
    expect(detectPlatform()).toBeNull()
  })
})

describe('detectMacArch', () => {
  it('returns null when the visitor is not on a Mac', async () => {
    stubNavigator({ userAgent: WIN_UA, platform: 'Win32' })
    await expect(detectMacArch()).resolves.toBeNull()
  })

  it('defaults to arm64 when no high-entropy API exists', async () => {
    stubMacNavigator()
    await expect(detectMacArch()).resolves.toBe('arm64')
  })

  it('defaults to arm64 when getHighEntropyValues rejects', async () => {
    stubMacNavigator({ userAgentDataArchitecture: null })
    await expect(detectMacArch()).resolves.toBe('arm64')
  })

  it('picks x64 for an Intel Mac', async () => {
    stubMacNavigator({ userAgentDataArchitecture: 'x86' })
    await expect(detectMacArch()).resolves.toBe('x64')
  })

  it('defaults to arm64 for an arm or unknown architecture value', async () => {
    stubMacNavigator({ userAgentDataArchitecture: 'arm' })
    await expect(detectMacArch()).resolves.toBe('arm64')
  })
})
