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
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ContextType } from 'react'
import { afterEach, assert, describe, expect, it, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { useAuthStore, type AuthBundle } from '@/stores/auth-store'

import { LANDING_COPIES, LandingLangProvider } from '../../i18n'
import { DownloadContext } from '../cn/download-context'
import { CnNav } from '../cn/nav'

const copy = LANDING_COPIES['zh-CN']
const downloadLabel = copy.nav.download

function makeBundle(user: Partial<AuthBundle['user']> = {}): AuthBundle {
  return {
    access_token: 'access-token',
    token_type: 'Bearer',
    access_expires_at: Math.floor(Date.now() / 1000) + 600,
    user: {
      id: 7,
      username: 'seller-li',
      display_name: '李掌柜',
      role: ROLE.USER,
      group: 'default',
      ...user,
    },
    session: {
      sid: 'session-a',
      current: true,
      login_method: 'password',
      ip: '127.0.0.1',
      user_agent: 'test',
      created_at: 100,
      last_active_at: 100,
      expires_at: 1000,
    },
  }
}

async function renderNav(download?: ContextType<typeof DownloadContext>) {
  const root = createRootRoute()
  const index = createRoute({
    getParentRoute: () => root,
    path: '/',
    component: () => (
      <div className='omc'>
        <LandingLangProvider lang='zh-CN'>
          {download ? (
            <DownloadContext.Provider value={download}>
              <CnNav />
            </DownloadContext.Provider>
          ) : (
            <CnNav />
          )}
        </LandingLangProvider>
      </div>
    ),
  })
  const router = createRouter({
    routeTree: root.addChildren([index]),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await screen.findByRole('button', { name: downloadLabel })
  return view
}

afterEach(() => {
  cleanup()
  useAuthStore.getState().auth.reset('idle')
})

describe('desktop CN nav auth state', () => {
  it('shows the login button when anonymous', async () => {
    useAuthStore.getState().auth.reset('complete')
    const { container } = await renderNav()
    expect(screen.getByRole('link', { name: '登录' })).toHaveAttribute(
      'href',
      '/sign-in'
    )
    expect(screen.queryByRole('button', { name: '账户菜单' })).toBeNull()
    await userEvent.click(
      screen.getByRole('button', { name: copy.nav.menuOpen })
    )
    const sheet = container.querySelector<HTMLElement>('#cn-sheet')
    assert(sheet)
    expect(within(sheet).getByRole('link', { name: '登录' })).toHaveAttribute(
      'href',
      '/sign-in'
    )
  })

  it('shows an avatar that opens the account menu when authenticated', async () => {
    useAuthStore.getState().auth.setBundle(makeBundle())
    await renderNav()

    expect(screen.queryByRole('link', { name: '登录' })).toBeNull()
    const trigger = screen.getByRole('button', { name: '账户菜单' })
    await userEvent.click(trigger)

    expect(await screen.findByText('李掌柜')).toBeTruthy()
    const menu = document.querySelector('.cn-usermenu')
    expect(menu).toBeTruthy()
    const { getByText } = within(menu as HTMLElement)
    expect(getByText('控制台')).toBeTruthy()
    expect(getByText('个人资料')).toBeTruthy()
    expect(getByText('退出登录')).toBeTruthy()
  })

  it('places the download entry left of the avatar when authenticated', async () => {
    useAuthStore.getState().auth.setBundle(makeBundle())
    await renderNav()

    const download = screen.getByRole('button', { name: downloadLabel })
    const avatar = screen.getByRole('button', { name: '账户菜单' })
    const order = download.compareDocumentPosition(avatar)
    expect(order & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })

  it('keeps sign-out and personal links in the drawer and respects sidebar visibility', async () => {
    useAuthStore.getState().auth.setBundle(
      makeBundle({
        sidebar_modules: JSON.stringify({
          personal: { enabled: true, topup: false, security: false },
        }),
      })
    )
    const { container } = await renderNav()
    await userEvent.click(
      screen.getByRole('button', { name: copy.nav.menuOpen })
    )
    const drawer = container.querySelector<HTMLElement>('#cn-sheet')
    assert(drawer)
    const sheet = within(drawer)

    expect(sheet.getByRole('link', { name: '控制台' })).toHaveAttribute(
      'href',
      '/influencer'
    )
    expect(sheet.getByRole('link', { name: '个人资料' })).toHaveAttribute(
      'href',
      '/profile'
    )
    expect(sheet.getByRole('button', { name: '退出登录' })).toBeVisible()
    expect(sheet.queryByRole('link', { name: '钱包' })).not.toBeInTheDocument()
    expect(
      sheet.queryByRole('link', { name: '安全与访问' })
    ).not.toBeInTheDocument()
    expect(
      sheet.queryByRole('link', { name: '系统设置' })
    ).not.toBeInTheDocument()
  })

  it.each([true, false])(
    'handles direct download result %s',
    async (hasInstaller) => {
      const downloadDirect = vi.fn(() => hasInstaller)
      const open = vi.fn()
      await renderNav({
        open,
        downloadDirect,
        platform: hasInstaller ? 'win' : null,
        macArch: null,
        release: { status: 'loading' },
      })

      await userEvent.click(screen.getByRole('button', { name: downloadLabel }))
      const intent = {
        cta_id: 'nav_download',
        cta_position: 'header',
        cta_variant: 'nav',
      }
      expect(downloadDirect).toHaveBeenCalledWith(intent)
      if (hasInstaller) {
        expect(open).not.toHaveBeenCalled()
      } else {
        expect(open).toHaveBeenCalledWith(undefined, intent)
      }
    }
  )

  it('disables the download button without a published installer', async () => {
    await renderNav({
      open: vi.fn(),
      downloadDirect: vi.fn(),
      platform: null,
      macArch: null,
      release: { status: 'unavailable' },
    })
    expect(screen.getByRole('button', { name: downloadLabel })).toBeDisabled()
  })
})
