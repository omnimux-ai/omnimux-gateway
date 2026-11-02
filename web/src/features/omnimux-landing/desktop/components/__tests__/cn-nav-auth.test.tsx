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
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { afterEach, describe, expect, it } from 'vitest'

import { ROLE } from '@/lib/roles'
import { useAuthStore, type AuthBundle } from '@/stores/auth-store'

import { CnNav } from '../cn/nav'

function makeBundle(): AuthBundle {
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

async function renderNav() {
  const root = createRootRoute()
  const index = createRoute({
    getParentRoute: () => root,
    path: '/',
    component: () => (
      <div className='omc'>
        <CnNav />
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
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await screen.findByRole('button', { name: '下载' })
}

afterEach(() => {
  cleanup()
  useAuthStore.getState().auth.reset('idle')
})

describe('desktop CN nav auth state', () => {
  it('shows the login button when anonymous', async () => {
    useAuthStore.getState().auth.reset('complete')
    await renderNav()
    // 桌面导航与移动端 sheet 各一个登录入口
    expect(screen.getAllByRole('link', { name: '登录' })).toHaveLength(2)
    expect(screen.queryByRole('button', { name: '账户菜单' })).toBeNull()
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

    const download = screen.getByRole('button', { name: '下载' })
    const avatar = screen.getByRole('button', { name: '账户菜单' })
    const order = download.compareDocumentPosition(avatar)
    expect(order & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy()
  })
})
