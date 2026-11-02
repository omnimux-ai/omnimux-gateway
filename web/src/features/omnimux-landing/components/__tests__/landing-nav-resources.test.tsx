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
import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { CnNav } from '../../desktop/components/cn/nav'
import { LandingLangProvider } from '../../desktop/i18n'

// Forward the props Base UI puts on the rendered element (role, tabIndex) so
// the menu item behaves the same here as it does with the real Link.
vi.mock('@tanstack/react-router', () => ({
  Link: (props: {
    to: string
    children: React.ReactNode
    role?: string
    tabIndex?: number
  }) => (
    <a href={props.to} role={props.role} tabIndex={props.tabIndex}>
      {props.children}
    </a>
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
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <div className='omc'>
        <LandingLangProvider lang='zh-CN'>
          <CnNav />
        </LandingLangProvider>
      </div>
    </QueryClientProvider>
  )
}

/**
 * The two resource destinations used to live only in the footer. The header
 * now carries a resources menu so both are reachable without scrolling.
 */
describe('landing nav resources menu', () => {
  it('sits directly after the pricing entry and opens the two destinations', async () => {
    renderNav()
    const navRow = screen.getByRole('navigation', { name: '主导航' })
    const entries = within(navRow)
      .getAllByRole('link')
      .map((node) => node.textContent ?? '')
    expect(entries).toContain('价格')

    const trigger = within(navRow).getByRole('button', { name: /资源/ })
    const triggerIndex = [...navRow.querySelectorAll('a, button')].indexOf(
      trigger
    )
    const pricingIndex = [...navRow.querySelectorAll('a, button')].indexOf(
      within(navRow).getByRole('link', { name: '价格' })
    )
    expect(triggerIndex).toBe(pricingIndex + 1)

    fireEvent.mouseDown(trigger, { button: 0 })
    fireEvent.click(trigger)

    const menu = await screen.findByRole('menu', {}, { timeout: 2000 })
    // Base UI renders menu items as `menuitem`, not `link`, even when the
    // item renders an anchor.
    const inspiration = within(menu).getByRole('menuitem', { name: '灵感社区' })
    const docs = within(menu).getByRole('menuitem', { name: '开发文档' })
    expect(inspiration).toHaveAttribute('href', '/inspiration')
    expect(docs).toHaveAttribute('href', 'https://docs.omnimux.ai')
    expect(docs).toHaveAttribute('target', '_blank')
    expect(docs).toHaveAttribute('rel', 'noopener noreferrer')
  })

  it('keeps both destinations out of the top-level row and the footer stays intact', () => {
    renderNav()
    const navRow = screen.getByRole('navigation', { name: '主导航' })
    expect(within(navRow).queryByRole('link', { name: '灵感社区' })).toBeNull()
    expect(within(navRow).queryByRole('link', { name: '开发文档' })).toBeNull()
  })
})
