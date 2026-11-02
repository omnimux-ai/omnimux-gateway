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

vi.mock('@tanstack/react-router', () => ({
  Link: (props: { to: string; children: React.ReactNode }) => (
    <a href={props.to}>{props.children}</a>
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
  // CnNav reads the sidebar visibility toggle through useStatus (React Query).
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

describe('landing nav tools menu', () => {
  it('groups the creative tool pages and the Agent entry pages by name only', async () => {
    renderNav()
    const trigger = screen.getByRole('button', { name: /工具/ })
    fireEvent.mouseDown(trigger, { button: 0 })
    fireEvent.click(trigger)

    const menu = await screen.findByRole('menu', {}, { timeout: 2000 })
    expect(within(menu).getByText('创意')).toBeInTheDocument()
    expect(within(menu).getByText('Agent')).toBeInTheDocument()
    const video = within(menu).getByRole('link', { name: 'TikTok 视频下载' })
    const photo = within(menu).getByRole('link', { name: 'TikTok 图文下载' })
    expect(video).toHaveAttribute('href', '/zh/tools/tiktok-video-downloader')
    expect(photo).toHaveAttribute('href', '/zh/tools/tiktok-photo-downloader')
    // The two Agent entry pages hang off the same menu.
    expect(within(menu).getByRole('link', { name: 'CLI' })).toHaveAttribute(
      'href',
      '/zh/cli'
    )
    expect(within(menu).getByRole('link', { name: 'Skills' })).toHaveAttribute(
      'href',
      '/zh/skills'
    )
    // Names only: no descriptions, and no "全部工具" entry (no such page).
    expect(within(menu).queryByText(/MP4|原图/)).toBeNull()
    expect(within(menu).queryByText(/全部工具/)).toBeNull()
  })

  it('no longer offers a For AI navigation entry', () => {
    renderNav()
    expect(screen.queryByRole('link', { name: 'For AI' })).toBeNull()
  })
})
