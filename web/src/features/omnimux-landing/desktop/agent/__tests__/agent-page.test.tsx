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

import { AGENT_LOCALES, AGENT_PATHS, type AgentLocale } from '../content'
import { AgentPage } from '../index'
import { AGENT_PAGES } from '../locales'

vi.mock('@tanstack/react-router', () => ({
  Link: (props: { to: string; children: React.ReactNode }) => (
    <a href={props.to}>{props.children}</a>
  ),
  useNavigate: () => vi.fn(),
  useRouter: () => ({ subscribe: () => () => {} }),
}))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (select: (state: unknown) => unknown) =>
    select({ auth: { user: null } }),
}))
vi.mock('@/hooks/use-sidebar-config', () => ({
  useIsSidebarModuleVisible: () => false,
}))
vi.mock('@/hooks/use-copy-to-clipboard', () => ({
  useCopyToClipboard: () => ({
    copiedText: null,
    copyToClipboard: () => Promise.resolve(true),
  }),
}))

function renderPage(kind: 'cli' | 'skills', locale: AgentLocale = 'zh-CN') {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>
      <AgentPage kind={kind} locale={locale} />
    </QueryClientProvider>
  )
}

describe('Agent entry pages', () => {
  it('swaps the install step and its command when another step is chosen', () => {
    renderPage('cli')

    // Step 1 owns the CLI installer command.
    expect(
      screen.getByText('curl -fsSL https://omnimux.ai/install.sh | bash')
    ).toBeInTheDocument()

    const rail = screen.getByRole('tablist', { name: '安装步骤' })
    fireEvent.click(within(rail).getByRole('tab', { name: /配置 API Key/ }))

    expect(
      screen.getByText('export OMNIMUX_ACCESS_TOKEN="sk-xxx"')
    ).toBeInTheDocument()
    expect(
      screen.queryByText('curl -fsSL https://omnimux.ai/install.sh | bash')
    ).toBeNull()
  })

  it('keeps the step rail state in aria attributes', () => {
    renderPage('cli')
    const rail = screen.getByRole('tablist', { name: '安装步骤' })
    const first = within(rail).getByRole('tab', { name: /安装 CLI/ })
    const second = within(rail).getByRole('tab', { name: /获取 API Key/ })

    expect(first).toHaveAttribute('aria-current', 'step')
    fireEvent.click(second)
    expect(second).toHaveAttribute('aria-current', 'step')
    expect(first).not.toHaveAttribute('aria-current')
  })

  it('gives the Skills page its own first step and the extra capability cards', () => {
    renderPage('skills')

    expect(screen.getByText('omnimux skill install --user')).toBeInTheDocument()
    expect(screen.getByText('AI 视频分析')).toBeInTheDocument()
    expect(screen.getByText('TikTok 带货图文生成器')).toBeInTheDocument()
  })

  it('does not carry the Skills-only cards on the CLI page', () => {
    renderPage('cli')
    expect(screen.queryByText('AI 视频分析')).toBeNull()
    expect(screen.queryByText('TikTok 带货图文生成器')).toBeNull()
  })

  it('links each Agent page to its sibling and to the language mirror', () => {
    renderPage('cli')
    const tabs = screen.getByRole('navigation', { name: 'Agent 入口' })
    expect(within(tabs).getByRole('link', { name: 'CLI' })).toHaveAttribute(
      'href',
      '/zh/cli'
    )
    expect(within(tabs).getByRole('link', { name: 'Skills' })).toHaveAttribute(
      'href',
      '/zh/skills'
    )
  })

  it('renders English copy and English paths for the en locale', () => {
    renderPage('skills', 'en')
    expect(
      screen.getByRole('heading', { name: 'Give any AI OmniMux skills' })
    ).toBeInTheDocument()
    const tabs = screen.getByRole('navigation', { name: 'Agent entry' })
    expect(within(tabs).getByRole('link', { name: 'CLI' })).toHaveAttribute(
      'href',
      '/cli'
    )
  })
})

describe('Agent entry pages across all six languages', () => {
  it.each(AGENT_LOCALES)(
    'publishes distinct copy and a path for every page in %s',
    (locale) => {
      for (const kind of ['cli', 'skills'] as const) {
        const page = AGENT_PAGES[locale][kind]
        expect(page.path).toBe(AGENT_PATHS[locale][kind])
        expect(page.title.length).toBeGreaterThan(0)
        expect(page.description.length).toBeGreaterThan(0)
        expect(page.h1.length).toBeGreaterThan(0)
        expect(page.steps.length).toBe(3)
        expect(page.faq.length).toBeGreaterThan(0)
      }
    }
  )

  it('gives every language its own URL so no two pages share a canonical', () => {
    const seen = new Set<string>()
    for (const locale of AGENT_LOCALES) {
      for (const kind of ['cli', 'skills'] as const) {
        const path = AGENT_PATHS[locale][kind]
        expect(seen.has(path)).toBe(false)
        seen.add(path)
      }
    }
    expect(seen.size).toBe(AGENT_LOCALES.length * 2)
  })

  it('keeps the CLI installer command identical across languages', () => {
    const command = 'curl -fsSL https://omnimux.ai/install.sh | bash'
    for (const locale of AGENT_LOCALES) {
      expect(AGENT_PAGES[locale].cli.steps[0].command).toBe(command)
    }
  })

  it('carries the Skills-only cards in every language', () => {
    for (const locale of AGENT_LOCALES) {
      const cli = AGENT_PAGES[locale].cli.cards.length
      const skills = AGENT_PAGES[locale].skills.cards.length
      expect(skills).toBe(cli + 2)
    }
  })

  it('renders Japanese copy and its own sibling paths', () => {
    renderPage('cli', 'ja')
    expect(
      screen.getByRole('heading', {
        name: '1 行のコマンドで、どの AI でも OmniMux の機能を',
      })
    ).toBeInTheDocument()
    const tabs = screen.getByRole('navigation', { name: 'Agent の入口' })
    expect(within(tabs).getByRole('link', { name: 'CLI' })).toHaveAttribute(
      'href',
      '/ja/cli'
    )
    expect(within(tabs).getByRole('link', { name: 'Skills' })).toHaveAttribute(
      'href',
      '/ja/skills'
    )
  })

  it('renders Traditional Chinese copy with its own sibling paths', () => {
    renderPage('skills', 'zh-TW')
    expect(
      screen.getByRole('heading', {
        name: '給任意 AI 裝上 OmniMux Skills',
      })
    ).toBeInTheDocument()
    const tabs = screen.getByRole('navigation', { name: 'Agent 入口' })
    expect(within(tabs).getByRole('link', { name: 'CLI' })).toHaveAttribute(
      'href',
      '/zh-tw/cli'
    )
  })

  it.each([
    ['es', 'Un comando y OmniMux en cualquier IA', '/es/cli'],
    ['id', 'Satu perintah, kemampuan OmniMux di AI mana pun', '/id/cli'],
  ] as const)('renders the %s hero and its own path', (locale, h1, path) => {
    renderPage('cli', locale)
    expect(screen.getByRole('heading', { name: h1 })).toBeInTheDocument()
    const tabs = screen.getByRole('navigation', {
      name: /Entrada del Agent|Masuk Agent/,
    })
    expect(within(tabs).getByRole('link', { name: 'CLI' })).toHaveAttribute(
      'href',
      path
    )
  })
})
