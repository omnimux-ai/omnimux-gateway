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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  initChatwoot,
  readChatwootConfig,
  resetChatwootControllerForTest,
  syncChatwootLocale,
  syncChatwootUser,
  toChatwootLocale,
} from '../chatwoot'

function mountConfigScript(payload: string) {
  const existing = document.querySelector('#omnimux-chatwoot-config')
  existing?.remove()
  const script = document.createElement('script')
  script.id = 'omnimux-chatwoot-config'
  script.type = 'application/json'
  script.textContent = payload
  document.head.append(script)
}

function flushIdleLoad() {
  vi.runOnlyPendingTimers()
}

async function flushMicrotasks() {
  await Promise.resolve()
  await Promise.resolve()
}

function dispatchWidgetLoaded(origin: string) {
  window.dispatchEvent(
    new MessageEvent('message', {
      origin,
      data: `chatwoot-widget:${JSON.stringify({ event: 'loaded', config: {} })}`,
    })
  )
}

describe('chatwoot integration bridge', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    resetChatwootControllerForTest()
    document.querySelector('#omnimux-chatwoot-config')?.remove()
    document.documentElement.lang = 'en'
  })

  afterEach(() => {
    resetChatwootControllerForTest()
    document.querySelector('#omnimux-chatwoot-config')?.remove()
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('maps landing and console language codes to Chatwoot locales', () => {
    expect(toChatwootLocale('zhCN')).toBe('zh_CN')
    expect(toChatwootLocale('zh-CN')).toBe('zh_CN')
    expect(toChatwootLocale('zh-Hans')).toBe('zh_CN')
    expect(toChatwootLocale('zh')).toBe('zh_CN')
    expect(toChatwootLocale('zhTW')).toBe('zh_TW')
    expect(toChatwootLocale('zh-TW')).toBe('zh_TW')
    expect(toChatwootLocale('zh-Hant')).toBe('zh_TW')
    expect(toChatwootLocale('en-US')).toBe('en')
    expect(toChatwootLocale('ja')).toBe('ja')
    expect(toChatwootLocale('fr-FR')).toBe('fr')
    expect(toChatwootLocale('ru')).toBe('ru')
    expect(toChatwootLocale('vi')).toBe('vi')
    expect(toChatwootLocale('es-ES')).toBe('es')
    expect(toChatwootLocale('id')).toBe('id')
    expect(toChatwootLocale('unknown')).toBe('en')
    expect(toChatwootLocale(undefined)).toBe('en')
  })

  it('validates and normalizes DOM-injected Chatwoot configuration', () => {
    mountConfigScript(
      JSON.stringify({
        baseUrl: 'https://chat.example.com///',
        websiteToken: ' cw_token_abc ',
        darkMode: 'auto',
      })
    )
    expect(readChatwootConfig()).toEqual({
      baseUrl: 'https://chat.example.com',
      websiteToken: 'cw_token_abc',
      darkMode: 'auto',
    })

    mountConfigScript(
      JSON.stringify({
        baseUrl: 'javascript:alert(1)',
        websiteToken: 'cw_token_abc',
        darkMode: 'dark',
      })
    )
    expect(readChatwootConfig()).toBeNull()

    mountConfigScript(
      JSON.stringify({
        baseUrl: 'https://chat.example.com',
        websiteToken: 'bad token',
      })
    )
    expect(readChatwootConfig()).toBeNull()
  })

  it('loads sdk.js once across repeated init calls and applies queued user on chatwoot:ready', () => {
    mountConfigScript(
      JSON.stringify({
        baseUrl: 'https://chat.example.com',
        websiteToken: 'cw_token_abc',
        darkMode: 'dark',
      })
    )

    initChatwoot({ locale: 'zh-CN' })
    initChatwoot({ locale: 'zhTW' })
    syncChatwootUser({
      id: 42,
      username: 'alice',
      display_name: '',
      email: ' ',
      group: 'vip',
      chatwoot_identifier_hash: 'hmac-42',
    })
    flushIdleLoad()

    const scripts = document.querySelectorAll('#chatwoot-sdk-script')
    expect(scripts).toHaveLength(1)
    const sdkScript = scripts[0] as HTMLScriptElement
    expect(sdkScript.src).toBe('https://chat.example.com/packs/js/sdk.js')

    const win = window as unknown as {
      chatwootSettings?: Record<string, unknown>
      chatwootSDK?: { run: (opts: unknown) => void }
      $chatwoot?: {
        setUser: (id: string, user: Record<string, unknown>) => void
        setCustomAttributes: (attrs: Record<string, string>) => void
        setLocale: (locale: string) => void
        setColorScheme: (mode: string) => void
        reset: () => void
      }
    }
    expect(win.chatwootSettings).toMatchObject({
      hideMessageBubble: false,
      position: 'right',
      type: 'standard',
      darkMode: 'dark',
      locale: 'zh_TW',
      useBrowserLanguage: false,
    })

    const runSpy = vi.fn()
    const setUserSpy = vi.fn()
    const setAttrsSpy = vi.fn()
    const setLocaleSpy = vi.fn()
    const setColorSpy = vi.fn()
    win.chatwootSDK = { run: runSpy }
    win.$chatwoot = {
      setUser: setUserSpy,
      setCustomAttributes: setAttrsSpy,
      setLocale: setLocaleSpy,
      setColorScheme: setColorSpy,
      reset: vi.fn(),
    }

    sdkScript.dispatchEvent(new Event('load'))
    expect(runSpy).toHaveBeenCalledWith({
      websiteToken: 'cw_token_abc',
      baseUrl: 'https://chat.example.com',
    })

    window.dispatchEvent(new Event('chatwoot:ready'))
    expect(setColorSpy).toHaveBeenCalledWith('dark')
    expect(setLocaleSpy).toHaveBeenCalledWith('zh_TW')
    expect(setUserSpy).toHaveBeenCalledWith('42', {
      name: 'alice',
      identifier_hash: 'hmac-42',
    })
    expect(setAttrsSpy).toHaveBeenCalledWith({
      username: 'alice',
      group: 'vip',
    })

    syncChatwootLocale('ja')
    expect(setLocaleSpy).toHaveBeenLastCalledWith('ja')
  })

  it('clears SDK identity fields and recovers via widget loaded postMessage after reset() on switch or logout', async () => {
    mountConfigScript(
      JSON.stringify({
        baseUrl: 'https://chat.example.com',
        websiteToken: 'cw_token_abc',
        darkMode: 'dark',
      })
    )

    const widgetState: {
      user?: Record<string, unknown>
      identifier?: string
      locale?: string
      darkMode?: string
      setUser: (id: string, user: Record<string, unknown>) => void
      setCustomAttributes: (attrs: Record<string, string>) => void
      setLocale: (locale: string) => void
      setColorScheme: (mode: string) => void
      reset: () => void
    } = {
      setUser: vi.fn((id, user) => {
        widgetState.identifier = id
        widgetState.user = user
      }),
      setCustomAttributes: vi.fn(),
      setLocale: vi.fn(),
      setColorScheme: vi.fn(),
      reset: vi.fn(() => {
        // Upstream Chatwoot requires $chatwoot.user and $chatwoot.identifier
        // to already be absent before reload so events.loaded will not resend them.
        expect(widgetState.user).toBeUndefined()
        expect(widgetState.identifier).toBeUndefined()
      }),
    }

    const win = window as unknown as {
      chatwootSDK?: { run: (opts: unknown) => void }
      $chatwoot?: typeof widgetState
    }
    win.chatwootSDK = { run: vi.fn() }
    win.$chatwoot = widgetState

    initChatwoot({ locale: 'en' })
    flushIdleLoad()
    document
      .querySelector('#chatwoot-sdk-script')
      ?.dispatchEvent(new Event('load'))
    window.dispatchEvent(new Event('chatwoot:ready'))

    syncChatwootUser({
      id: 1,
      username: 'first',
      display_name: 'First User',
      email: 'first@example.com',
    })
    expect(widgetState.setUser).toHaveBeenCalledTimes(1)

    // Switch account: triggers reset() and waits for reloaded iframe's loaded postMessage
    // (upstream Chatwoot sets resetTriggered=true and never fires chatwoot:ready again).
    syncChatwootUser({
      id: 2,
      username: 'second',
      display_name: 'Second User',
      email: 'second@example.com',
      chatwoot_identifier_hash: 'hmac-2',
    })
    expect(widgetState.reset).toHaveBeenCalledTimes(1)
    expect(widgetState.setUser).toHaveBeenCalledTimes(1)

    // Foreign-origin messages must be ignored.
    dispatchWidgetLoaded('https://evil.example')
    await flushMicrotasks()
    expect(widgetState.setUser).toHaveBeenCalledTimes(1)

    // Valid origin loaded postMessage completes reset without chatwoot:ready.
    dispatchWidgetLoaded('https://chat.example.com')
    await flushMicrotasks()
    expect(widgetState.setUser).toHaveBeenCalledTimes(2)
    expect(widgetState.setUser).toHaveBeenLastCalledWith('2', {
      name: 'Second User',
      email: 'second@example.com',
      identifier_hash: 'hmac-2',
    })

    // Logout triggers reset(), and a later SPA re-login + locale sync must still work.
    syncChatwootUser(null)
    expect(widgetState.reset).toHaveBeenCalledTimes(2)
    dispatchWidgetLoaded('https://chat.example.com')
    await flushMicrotasks()

    syncChatwootUser({
      id: 3,
      username: 'third',
      display_name: 'Third User',
      email: 'third@example.com',
    })
    expect(widgetState.setUser).toHaveBeenCalledTimes(3)
    expect(widgetState.setUser).toHaveBeenLastCalledWith('3', {
      name: 'Third User',
      email: 'third@example.com',
    })

    syncChatwootLocale('fr')
    expect(widgetState.setLocale).toHaveBeenLastCalledWith('fr')
  })
})
