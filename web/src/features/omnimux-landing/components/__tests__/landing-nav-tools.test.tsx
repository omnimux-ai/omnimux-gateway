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

import { CnCapabilities } from '../../desktop/components/cn/capabilities'
import { CnNav } from '../../desktop/components/cn/nav'
import { LandingLangProvider } from '../../desktop/i18n'
import { zhCN } from '../../desktop/i18n/locales/zh-CN'
import { docsQuickstartUrl } from '../../lib/landing-urls'

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

describe('landing navigation outside the homepage', () => {
  it('returns to homepage sections and offers the languages published by the tool page', async () => {
    const user = userEvent.setup()
    const onLangChange = vi.fn()
    const client = new QueryClient()
    const { container } = render(
      <QueryClientProvider client={client}>
        <div className='omc'>
          <LandingLangProvider
            lang='zh-CN'
            langs={['en', 'zh-CN']}
            onLangChange={onLangChange}
          >
            <CnNav anchorBase='/' />
          </LandingLangProvider>
        </div>
      </QueryClientProvider>
    )

    expect(
      screen.getByRole('link', { name: zhCN.nav.brandAria })
    ).toHaveAttribute('href', '/')
    const nav = screen.getByRole('navigation', { name: zhCN.nav.navAria })
    expect(
      within(nav)
        .getAllByRole('link')
        .map((link) => link.getAttribute('href'))
    ).toEqual(['/#capabilities', '/#workflow', '/pricing', '/inspiration'])

    await user.click(screen.getByRole('button', { name: zhCN.nav.menuOpen }))
    const sheet = container.querySelector<HTMLElement>('#cn-sheet')
    assert(sheet)
    expect(
      within(sheet)
        .getAllByRole('link')
        .map((link) => link.getAttribute('href'))
    ).toEqual([
      '/#capabilities',
      '/#workflow',
      '/pricing',
      '/inspiration',
      '/#download',
      '/sign-in',
    ])
    const languages = within(sheet).getByRole('group', {
      name: zhCN.nav.langTitle,
    })
    expect(
      within(languages)
        .getAllByRole('button')
        .map((button) => button.textContent)
    ).toEqual(['English', '简体中文'])
    await user.click(within(languages).getByRole('button', { name: 'English' }))
    expect(onLangChange).toHaveBeenCalledWith('en')
  })
})

it('keeps tool, Agent, and documentation destinations in the capability section', () => {
  render(
    <LandingLangProvider lang='zh-CN'>
      <CnCapabilities />
    </LandingLangProvider>
  )

  const resources = screen.getByRole('navigation', { name: zhCN.nav.resources })
  expect(
    within(resources)
      .getAllByRole('link')
      .map((link) => [link.textContent, link.getAttribute('href')])
  ).toEqual([
    ['TikTok 视频下载', '/zh/tools/tiktok-video-downloader'],
    ['TikTok 图文下载', '/zh/tools/tiktok-photo-downloader'],
    ['CLI', '/zh/cli'],
    ['Skills', '/zh/skills'],
    ['开发文档', docsQuickstartUrl('zh-CN')],
  ])
})
