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
  cleanup,
  render,
  screen,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import {
  LANDING_COPIES,
  LANDING_LANG_STORAGE_KEY,
  LANDING_LANGS,
  resolveLandingLang,
} from '../../i18n'
import type { LandingLang } from '../../i18n/types'
import { DesktopLanding } from '../../index'

afterEach(cleanup)

function stubNavigatorLangs(tags: readonly string[]) {
  Object.defineProperty(window.navigator, 'language', {
    value: tags[0],
    configurable: true,
  })
  Object.defineProperty(window.navigator, 'languages', {
    value: [...tags],
    configurable: true,
  })
}

beforeEach(() => {
  window.localStorage.clear()
  window.history.replaceState(null, '', '/')
  stubNavigatorLangs(['zh-CN'])
})

async function renderLanding(awaitText: string | RegExp = '下载') {
  const root = createRootRoute()
  const index = createRoute({
    getParentRoute: () => root,
    path: '/',
    component: DesktopLanding,
  })
  const router = createRouter({
    routeTree: root.addChildren([index]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await screen.findAllByText(awaitText)
}

describe('resolveLandingLang', () => {
  it('prefers an explicit ?lang= parameter over storage', () => {
    window.localStorage.setItem(LANDING_LANG_STORAGE_KEY, 'ja')
    window.history.replaceState(null, '', '/?lang=es')
    expect(resolveLandingLang()).toBe('es')
  })

  it('falls back to a stored choice when no parameter is present', () => {
    window.localStorage.setItem(LANDING_LANG_STORAGE_KEY, 'id')
    expect(resolveLandingLang()).toBe('id')
  })

  it('defaults to Simplified Chinese for unsupported browser languages', () => {
    stubNavigatorLangs(['fr-FR'])
    expect(resolveLandingLang()).toBe('zh-CN')
  })

  it('maps Traditional-Chinese browser languages to zh-TW', () => {
    stubNavigatorLangs(['zh-Hant-TW', 'en-US'])
    expect(resolveLandingLang()).toBe('zh-TW')
  })
})

describe('landing locale data', () => {
  it('ships one complete copy dictionary per supported language', () => {
    const codes = LANDING_LANGS.map((option) => option.code)
    expect(codes).toEqual(['en', 'id', 'es', 'ja', 'zh-TW', 'zh-CN'])
    for (const code of codes) {
      expect(LANDING_COPIES[code as LandingLang].meta.title).toBeTruthy()
    }
  })

  it('keeps capability boundaries out of every locale', () => {
    for (const copy of Object.values(LANDING_COPIES)) {
      const blob = JSON.stringify(copy)
      expect(blob).not.toMatch(/50万\+|1000万\+|官方认证|guaranteed viral|TikTok Shop official/i)
    }
  })
})

describe('DesktopLanding language switching', () => {
  it('renders Simplified Chinese by default and a compact language trigger', async () => {
    await renderLanding()
    const trigger = screen.getByRole('button', { name: '页面语言：简体中文' })
    expect(trigger.textContent).toBe('中')
    expect(screen.getByText('发现、复刻和裂变')).toBeTruthy()
  })

  it('switches the page to English and persists the choice', async () => {
    const user = userEvent.setup()
    await renderLanding()
    await user.click(screen.getByRole('button', { name: '页面语言：简体中文' }))
    const menu = document.querySelector('.cn-langmenu')
    expect(menu).toBeTruthy()
    await user.click(within(menu as HTMLElement).getByText('English'))
    expect(await screen.findByText('Find, recreate & scale')).toBeTruthy()
    expect(window.localStorage.getItem(LANDING_LANG_STORAGE_KEY)).toBe('en')
    expect(new URL(window.location.href).searchParams.get('lang')).toBe('en')
    expect(document.documentElement.lang).toBe('en')
    expect(document.title).toBe(LANDING_COPIES.en.meta.title)
  })

  it('honours a stored language on initial render', async () => {
    window.localStorage.setItem(LANDING_LANG_STORAGE_KEY, 'ja')
    await renderLanding('ダウンロード')
    expect(
      screen.getByRole('button', { name: 'ページの言語：日本語' })
    ).toBeTruthy()
    expect(document.documentElement.lang).toBe('ja')
    expect(document.title).toBe(LANDING_COPIES.ja.meta.title)
  })
})
