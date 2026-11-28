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
export type ChatwootDarkMode = 'dark' | 'auto' | 'light'

export interface ChatwootConfig {
  baseUrl: string
  websiteToken: string
  darkMode: ChatwootDarkMode
}

export interface ChatwootUserProfile {
  id: number
  username: string
  display_name?: string
  email?: string
  group?: string
  chatwoot_identifier_hash?: string
}

interface ChatwootSDKRuntime {
  run: (options: { websiteToken: string; baseUrl: string }) => void
}

interface ChatwootSetUserOptions {
  name: string
  email?: string
  identifier_hash?: string
}

interface ChatwootWidgetAPI {
  user?: unknown
  identifier?: string | number
  locale?: string
  darkMode?: ChatwootDarkMode
  setUser: (identifier: string, user: ChatwootSetUserOptions) => void
  setCustomAttributes: (attributes: Record<string, string>) => void
  setLocale: (locale: string) => void
  setColorScheme?: (darkMode: ChatwootDarkMode) => void
  reset: () => void
}

interface ChatwootSettings {
  hideMessageBubble: boolean
  position: 'left' | 'right'
  type: 'standard' | 'expanded_bubble'
  darkMode: ChatwootDarkMode
  locale: string
  useBrowserLanguage: boolean
  showPopoutButton: boolean
  showUnreadMessagesDialog: boolean
}

interface ChatwootController {
  config: ChatwootConfig
  baseOrigin: string
  scriptScheduled: boolean
  scriptLoaded: boolean
  failed: boolean
  ready: boolean
  resetPending: boolean
  resetEpoch: number
  locale: string
  desiredUser: ChatwootUserProfile | null
  identifiedUserId: string | null
  identifiedKey: string | null
  themeObserver: MutationObserver | null
  onReady: () => void
  onMessage: (event: MessageEvent) => void
}

interface ChatwootWindow extends Window {
  chatwootSettings?: ChatwootSettings
  chatwootSDK?: ChatwootSDKRuntime
  $chatwoot?: ChatwootWidgetAPI
  __OMNIMUX_CHATWOOT_CONTROLLER__?: ChatwootController
}

const CHATWOOT_CONFIG_SELECTOR = '#omnimux-chatwoot-config'
const CHATWOOT_SCRIPT_ID = 'chatwoot-sdk-script'
const CHATWOOT_SCRIPT_SELECTOR = `#${CHATWOOT_SCRIPT_ID}`
const CHATWOOT_HOST_STYLE_ID = 'omnimux-chatwoot-host-style'
const CHATWOOT_HOST_STYLE_SELECTOR = `#${CHATWOOT_HOST_STYLE_ID}`
const CHATWOOT_WIDGET_IFRAME_SELECTOR = '#chatwoot_live_chat_widget'
const CHATWOOT_MESSAGE_PREFIX = 'chatwoot-widget:'

const CHATWOOT_HOST_STYLES = `
.woot-widget-bubble {
  background: #6757e7;
  box-shadow:
    0 10px 28px rgba(0, 0, 0, 0.45),
    0 0 0 1px rgba(255, 255, 255, 0.14) !important;
}
.woot-widget-bubble:active {
  transform: scale(0.96);
}
html.light .woot-widget-bubble {
  box-shadow:
    0 10px 24px rgba(103, 87, 231, 0.28),
    0 0 0 1px rgba(103, 87, 231, 0.22) !important;
}
@media only screen and (min-width: 668px) {
  .woot-widget-holder:not(.has-unread-view) {
    background: #0a0a0b;
    border: 1px solid rgba(255, 255, 255, 0.12);
    border-radius: 16px;
    bottom: 92px;
    box-shadow:
      0 24px 64px rgba(0, 0, 0, 0.62),
      0 0 0 1px rgba(255, 255, 255, 0.05) !important;
  }
  html.light .woot-widget-holder:not(.has-unread-view) {
    background: #ffffff;
    border: 1px solid rgba(15, 23, 42, 0.1);
    box-shadow: 0 20px 50px rgba(15, 23, 42, 0.14) !important;
  }
  .woot-widget-bubble:not(.woot-widget--expanded) {
    width: 56px;
    height: 56px;
  }
}
@media only screen and (max-width: 667px) {
  .woot-widget-holder:not(.has-unread-view) {
    border: 0;
    border-radius: 0;
  }
  .woot-widget-bubble:not(.woot-widget--expanded) {
    width: 52px;
    height: 52px;
  }
}
`.trim()

function ensureChatwootHostStyle(): void {
  if (typeof document === 'undefined') return
  if (document.querySelector(CHATWOOT_HOST_STYLE_SELECTOR)) return
  const style = document.createElement('style')
  style.id = CHATWOOT_HOST_STYLE_ID
  style.textContent = CHATWOOT_HOST_STYLES
  const target = document.head ?? document.body
  target?.append(style)
}

function resolveColorScheme(
  configuredMode: ChatwootDarkMode
): 'dark' | 'light' {
  if (configuredMode === 'light') return 'light'
  if (configuredMode === 'dark') return 'dark'
  if (typeof document !== 'undefined') {
    const classes = document.documentElement.classList
    if (classes.contains('light') && !classes.contains('dark')) {
      return 'light'
    }
  }
  return 'dark'
}

function applyColorSchemeToWidget(controller: ChatwootController): void {
  const win = window as ChatwootWindow
  const scheme = resolveColorScheme(controller.config.darkMode)
  if (win.$chatwoot) {
    win.$chatwoot.darkMode = scheme
  }
  if (win.chatwootSettings) {
    win.chatwootSettings.darkMode = scheme
  }
  if (!controller.ready || controller.resetPending) return
  try {
    if (typeof win.$chatwoot?.setColorScheme === 'function') {
      win.$chatwoot.setColorScheme(scheme)
      return
    }
    const iframe = document.querySelector<HTMLIFrameElement>(
      CHATWOOT_WIDGET_IFRAME_SELECTOR
    )
    iframe?.contentWindow?.postMessage(
      `${CHATWOOT_MESSAGE_PREFIX}${JSON.stringify({
        event: 'set-color-scheme',
        darkMode: scheme,
      })}`,
      controller.baseOrigin
    )
  } catch {
    // Ignore transient widget frame errors.
  }
}

function normalizeChatwootBaseURL(raw?: string | null): string | null {
  const trimmed = raw?.trim()
  if (!trimmed) return null
  try {
    const parsed = new URL(trimmed)
    const scheme = parsed.protocol.toLowerCase()
    if (scheme !== 'https:' && scheme !== 'http:') return null
    if (!parsed.host || parsed.username || parsed.password) return null
    if (parsed.search || parsed.hash) return null
    return parsed.toString().replace(/\/+$/, '')
  } catch {
    return null
  }
}

function normalizeChatwootToken(raw?: string | null): string | null {
  const token = raw?.trim()
  if (!token) return null
  for (let i = 0; i < token.length; i += 1) {
    const code = token.charCodeAt(i)
    if (code <= 0x20 || code === 0x7f) return null
  }
  return token
}

function normalizeDarkMode(raw?: string | null): ChatwootDarkMode {
  const normalized = raw?.trim().toLowerCase()
  if (normalized === 'light') return 'light'
  if (normalized === 'auto') return 'auto'
  return 'dark'
}

export function toChatwootLocale(language?: string | null): string {
  const normalized = language?.trim().replaceAll('_', '-').toLowerCase()
  if (!normalized) return 'en'

  if (
    normalized === 'zhtw' ||
    normalized === 'zh-tw' ||
    normalized === 'zh-hk' ||
    normalized === 'zh-mo' ||
    normalized.startsWith('zh-hant')
  ) {
    return 'zh_TW'
  }
  if (
    normalized === 'zhcn' ||
    normalized === 'zh-cn' ||
    normalized.startsWith('zh-hans') ||
    normalized === 'zh' ||
    normalized.startsWith('zh-')
  ) {
    return 'zh_CN'
  }

  const base = normalized.split('-')[0]
  switch (base) {
    case 'en':
    case 'ja':
    case 'fr':
    case 'ru':
    case 'vi':
    case 'es':
      return base
    case 'jp':
      return 'ja'
    case 'id':
    case 'in':
      return 'id'
    default:
      return 'en'
  }
}

export function readChatwootConfig(): ChatwootConfig | null {
  if (typeof document === 'undefined') return null

  const configElement = document.querySelector<HTMLScriptElement>(
    CHATWOOT_CONFIG_SELECTOR
  )
  let rawBaseUrl: string | undefined
  let rawToken: string | undefined
  let rawDarkMode: string | undefined

  if (configElement?.textContent) {
    try {
      const parsed = JSON.parse(configElement.textContent) as {
        baseUrl?: unknown
        websiteToken?: unknown
        darkMode?: unknown
      }
      if (typeof parsed.baseUrl === 'string') rawBaseUrl = parsed.baseUrl
      if (typeof parsed.websiteToken === 'string') {
        rawToken = parsed.websiteToken
      }
      if (typeof parsed.darkMode === 'string') rawDarkMode = parsed.darkMode
    } catch {
      return null
    }
  } else {
    rawBaseUrl = import.meta.env.VITE_CHATWOOT_BASE_URL
    rawToken = import.meta.env.VITE_CHATWOOT_WEBSITE_TOKEN
    rawDarkMode = import.meta.env.VITE_CHATWOOT_DARK_MODE
  }

  const baseUrl = normalizeChatwootBaseURL(rawBaseUrl)
  const websiteToken = normalizeChatwootToken(rawToken)
  if (!baseUrl || !websiteToken) return null

  return {
    baseUrl,
    websiteToken,
    darkMode: normalizeDarkMode(rawDarkMode),
  }
}

function getController(): ChatwootController | undefined {
  if (typeof window === 'undefined') return undefined
  return (window as ChatwootWindow).__OMNIMUX_CHATWOOT_CONTROLLER__
}

function buildUserIdentityKey(user: ChatwootUserProfile): string {
  return [
    user.id,
    user.display_name ?? '',
    user.username,
    user.email ?? '',
    user.group ?? '',
    user.chatwoot_identifier_hash ?? '',
  ].join('\u0000')
}

function applyUserToWidget(
  controller: ChatwootController,
  user: ChatwootUserProfile
): void {
  const widget = (window as ChatwootWindow).$chatwoot
  if (!widget || !controller.ready || controller.resetPending) return

  const nextKey = buildUserIdentityKey(user)
  if (nextKey === controller.identifiedKey) return

  const payload: ChatwootSetUserOptions = {
    name:
      user.display_name?.trim() || user.username.trim() || `User #${user.id}`,
  }
  const email = user.email?.trim()
  if (email) payload.email = email
  const identifierHash = user.chatwoot_identifier_hash?.trim()
  if (identifierHash) payload.identifier_hash = identifierHash

  const identifier = String(user.id)
  try {
    widget.setUser(identifier, payload)
    widget.setCustomAttributes({
      username: user.username,
      group: user.group?.trim() || 'default',
    })
    controller.identifiedUserId = identifier
    controller.identifiedKey = nextKey
  } catch {
    // Keep host UI responsive if widget runtime rejects identification.
  }
}

function triggerWidgetReset(controller: ChatwootController): void {
  controller.identifiedUserId = null
  controller.identifiedKey = null
  const win = window as ChatwootWindow
  const widget = win.$chatwoot
  if (!controller.ready || !widget) return

  // Upstream Chatwoot IFrameHelper.events.loaded re-posts window.$chatwoot.user
  // across iframe reloads and suppresses chatwoot:ready when resetTriggered is set.
  // Clear stale identity fields and sync the latest locale before reloading.
  delete widget.user
  delete widget.identifier
  widget.locale = controller.locale
  const nextScheme = resolveColorScheme(controller.config.darkMode)
  widget.darkMode = nextScheme
  if (win.chatwootSettings) {
    win.chatwootSettings.locale = controller.locale
    win.chatwootSettings.darkMode = nextScheme
  }

  controller.resetEpoch += 1
  controller.ready = false
  controller.resetPending = true
  try {
    widget.reset()
  } catch {
    controller.resetPending = false
    controller.ready = true
  }
}

function loadChatwootSdk(controller: ChatwootController): void {
  if (controller.scriptLoaded || controller.failed) return
  if (document.querySelector(CHATWOOT_SCRIPT_SELECTOR)) {
    controller.scriptLoaded = true
    return
  }

  ensureChatwootHostStyle()
  const win = window as ChatwootWindow
  win.chatwootSettings = {
    hideMessageBubble: false,
    position: 'right',
    type: 'standard',
    darkMode: resolveColorScheme(controller.config.darkMode),
    locale: controller.locale,
    useBrowserLanguage: false,
    showPopoutButton: false,
    showUnreadMessagesDialog: false,
  }

  const script = document.createElement('script')
  script.id = CHATWOOT_SCRIPT_ID
  script.src = `${controller.config.baseUrl}/packs/js/sdk.js`
  script.async = true
  script.defer = true
  script.addEventListener('load', () => {
    controller.scriptLoaded = true
    try {
      win.chatwootSDK?.run({
        websiteToken: controller.config.websiteToken,
        baseUrl: controller.config.baseUrl,
      })
    } catch {
      controller.failed = true
    }
  })
  script.addEventListener('error', () => {
    controller.failed = true
  })

  const target = document.head ?? document.body
  target?.append(script)
}

function scheduleSdkLoad(controller: ChatwootController): void {
  if (
    controller.scriptScheduled ||
    controller.scriptLoaded ||
    controller.failed
  ) {
    return
  }
  controller.scriptScheduled = true
  const start = () => loadChatwootSdk(controller)
  if (typeof window.requestIdleCallback === 'function') {
    window.requestIdleCallback(start, { timeout: 2000 })
    return
  }
  window.setTimeout(start, 0)
}

function parseWidgetLoadedMessage(
  event: MessageEvent,
  expectedOrigin: string
): boolean {
  if (event.origin !== expectedOrigin) return false
  if (typeof event.data !== 'string') return false
  if (!event.data.startsWith(CHATWOOT_MESSAGE_PREFIX)) return false
  try {
    const payload = JSON.parse(
      event.data.slice(CHATWOOT_MESSAGE_PREFIX.length)
    ) as { event?: unknown }
    return payload.event === 'loaded'
  } catch {
    return false
  }
}

export function initChatwoot(options?: { locale?: string | null }): void {
  if (typeof window === 'undefined' || typeof document === 'undefined') return
  const config = readChatwootConfig()
  if (!config) return

  ensureChatwootHostStyle()
  const win = window as ChatwootWindow
  let controller = win.__OMNIMUX_CHATWOOT_CONTROLLER__
  if (!controller) {
    const baseOrigin = new URL(config.baseUrl).origin
    const initialLocale = toChatwootLocale(
      options?.locale ?? document.documentElement.lang
    )
    const created: ChatwootController = {
      config,
      baseOrigin,
      scriptScheduled: false,
      scriptLoaded: false,
      failed: false,
      ready: false,
      resetPending: false,
      resetEpoch: 0,
      locale: initialLocale,
      desiredUser: null,
      identifiedUserId: null,
      identifiedKey: null,
      themeObserver: null,
      onReady: () => {
        created.ready = true
        created.resetPending = false
        try {
          applyColorSchemeToWidget(created)
          win.$chatwoot?.setLocale(created.locale)
        } catch {
          // Ignore transient widget frame errors.
        }
        if (created.desiredUser) {
          applyUserToWidget(created, created.desiredUser)
        }
      },
      onMessage: (event: MessageEvent) => {
        if (!parseWidgetLoadedMessage(event, created.baseOrigin)) return
        const epoch = created.resetEpoch
        // Defer one microtask so Chatwoot's window.onmessage runs config-set
        // and any initial chatwoot:ready dispatch before we post user/locale updates.
        queueMicrotask(() => {
          if (created.resetEpoch !== epoch) return
          if (!created.ready || created.resetPending) {
            created.onReady()
          }
        })
      },
    }
    if (config.darkMode === 'auto' && typeof MutationObserver === 'function') {
      const observer = new MutationObserver(() => {
        applyColorSchemeToWidget(created)
      })
      observer.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ['class'],
      })
      created.themeObserver = observer
    }
    win.__OMNIMUX_CHATWOOT_CONTROLLER__ = created
    window.addEventListener('chatwoot:ready', created.onReady)
    window.addEventListener('message', created.onMessage)
    controller = created
  } else if (options?.locale) {
    syncChatwootLocale(options.locale)
  }

  scheduleSdkLoad(controller)
}

export function syncChatwootLocale(language?: string | null): void {
  const controller = getController()
  if (!controller) return
  const nextLocale = toChatwootLocale(language)
  controller.locale = nextLocale
  const win = window as ChatwootWindow
  if (win.$chatwoot) {
    win.$chatwoot.locale = nextLocale
  }
  if (!controller.ready || controller.resetPending) return
  try {
    win.$chatwoot?.setLocale(nextLocale)
  } catch {
    // Ignore transient widget frame errors.
  }
}

export function syncChatwootUser(user: ChatwootUserProfile | null): void {
  const controller = getController()
  if (!controller) return

  controller.desiredUser = user
  if (!user) {
    if (controller.identifiedUserId !== null) {
      triggerWidgetReset(controller)
    }
    return
  }

  const nextUserId = String(user.id)
  if (
    controller.identifiedUserId !== null &&
    controller.identifiedUserId !== nextUserId
  ) {
    triggerWidgetReset(controller)
    return
  }

  applyUserToWidget(controller, user)
}

export function resetChatwootControllerForTest(): void {
  if (typeof window === 'undefined') return
  const win = window as ChatwootWindow
  const existing = win.__OMNIMUX_CHATWOOT_CONTROLLER__
  if (existing) {
    existing.themeObserver?.disconnect()
    existing.themeObserver = null
    window.removeEventListener('chatwoot:ready', existing.onReady)
    window.removeEventListener('message', existing.onMessage)
    delete win.__OMNIMUX_CHATWOOT_CONTROLLER__
  }
  delete win.chatwootSettings
  delete win.chatwootSDK
  delete win.$chatwoot
  document.querySelector(CHATWOOT_SCRIPT_SELECTOR)?.remove()
  document.querySelector(CHATWOOT_HOST_STYLE_SELECTOR)?.remove()
}
