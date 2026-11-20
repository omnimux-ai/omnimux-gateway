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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { assert, describe, expect, it, vi } from 'vitest'

import { CnNav } from '../../desktop/components/cn/nav'
import { LandingLangProvider } from '../../desktop/i18n'
import { zhCN } from '../../desktop/i18n/locales/zh-CN'

vi.mock('@tanstack/react-router', () => ({
  Link: ({ to, ...props }: React.ComponentProps<'a'> & { to: string }) => (
    <a href={to} {...props} />
  ),
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (select: (state: unknown) => unknown) =>
    select({ auth: { user: null } }),
}))
vi.mock('@/hooks/use-sidebar-config', () => ({
  useIsSidebarModuleVisible: () => false,
}))

function renderNav() {
  const client = new QueryClient()
  return render(
    <QueryClientProvider client={client}>
      <div className='omc'>
        <LandingLangProvider lang='zh-CN'>
          <CnNav />
        </LandingLangProvider>
      </div>
    </QueryClientProvider>
  )
}

describe('landing primary navigation', () => {
  it('lists the four approved destinations in order', () => {
    renderNav()

    const nav = screen.getByRole('navigation', { name: '主导航' })
    expect(
      within(nav)
        .getAllByRole('link')
        .map((link) => [link.textContent, link.getAttribute('href')])
    ).toEqual([
      ['产品能力', '#capabilities'],
      ['工作流', '#workflow'],
      ['价格', '/pricing'],
      ['灵感社区', '/inspiration'],
    ])
    expect(within(nav).queryByRole('button')).not.toBeInTheDocument()
  })

  it('opens by keyboard, exposes the drawer links, and returns focus on Escape', async () => {
    const user = userEvent.setup()
    const { container } = renderNav()

    const trigger = screen.getByRole('button', { name: zhCN.nav.menuOpen })
    const sheet = container.querySelector<HTMLElement>('#cn-sheet')
    assert(sheet)
    expect(trigger).toHaveAttribute('aria-controls', 'cn-sheet')
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(sheet).not.toBeVisible()
    expect(within(sheet).queryByRole('link')).not.toBeInTheDocument()

    trigger.focus()
    await user.keyboard('{Enter}')

    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(trigger).toHaveAccessibleName(zhCN.nav.menuClose)
    expect(
      within(sheet)
        .getAllByRole('link')
        .map((link) => link.getAttribute('href'))
    ).toEqual([
      '#capabilities',
      '#workflow',
      '/pricing',
      '/inspiration',
      '#download',
      '/sign-in',
    ])
    const languages = within(sheet).getByRole('group', {
      name: zhCN.nav.langTitle,
    })
    expect(within(languages).getAllByRole('button')).toHaveLength(6)
    expect(
      within(languages).getByRole('button', { name: '简体中文', pressed: true })
    ).toBeVisible()

    await user.tab()
    expect(within(sheet).getByRole('link', { name: '产品能力' })).toHaveFocus()
    await user.keyboard('{Escape}')

    expect(sheet).not.toBeVisible()
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(trigger).toHaveFocus()

    await user.keyboard('{Enter}')
    await user.click(within(sheet).getByRole('link', { name: '下载桌面端' }))

    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(sheet).not.toBeVisible()
  })
})
