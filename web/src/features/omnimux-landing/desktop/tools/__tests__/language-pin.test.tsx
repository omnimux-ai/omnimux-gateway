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
import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  LANDING_LANG_STORAGE_KEY,
  LandingLangProvider,
  useLanding,
} from '../../i18n'
import { CnLangMenu } from '../../components/cn/lang-menu'

function Probe() {
  const landing = useLanding()
  return (
    <button type='button' onClick={() => landing.setLang('en')}>
      {landing.lang}
    </button>
  )
}

describe('pinned landing language', () => {
  afterEach(() => {
    window.localStorage.clear()
  })

  it('renders the pinned language even when a different language is stored', () => {
    window.localStorage.setItem(LANDING_LANG_STORAGE_KEY, 'ja')
    render(
      <LandingLangProvider lang='zh-CN'>
        <Probe />
      </LandingLangProvider>
    )
    expect(screen.getByRole('button')).toHaveTextContent('zh-CN')
  })

  it('hands a language switch to the page instead of rewriting storage', () => {
    const onLangChange = vi.fn()
    render(
      <LandingLangProvider lang='zh-CN' onLangChange={onLangChange}>
        <Probe />
      </LandingLangProvider>
    )
    fireEvent.click(screen.getByRole('button'))
    expect(onLangChange).toHaveBeenCalledWith('en')
    expect(screen.getByRole('button')).toHaveTextContent('zh-CN')
    expect(window.localStorage.getItem(LANDING_LANG_STORAGE_KEY)).toBeNull()
  })
})

describe('pinned page language links', () => {
  it('lists only the languages the page publishes, each linking to its own URL', async () => {
    const user = userEvent.setup()
    const onLangChange = vi.fn()
    render(
      <div className='omc'>
        <LandingLangProvider
          lang='en'
          onLangChange={onLangChange}
          langs={['en', 'zh-CN']}
          langHref={(next) => (next === 'en' ? '/tools/x' : '/zh/tools/x')}
        >
          <CnLangMenu />
        </LandingLangProvider>
      </div>
    )
    await user.click(screen.getByRole('button', { name: /English/ }))
    const menu = document.querySelector('.cn-langmenu') as HTMLElement
    const links = within(menu).getAllByRole('menuitem')
    expect(links.map((link) => link.getAttribute('href'))).toEqual([
      '/tools/x',
      '/zh/tools/x',
    ])
    expect(links[0]).toHaveAttribute('aria-current', 'page')
    expect(links[1]).not.toHaveAttribute('aria-current')

    await user.click(links[1])
    expect(onLangChange).toHaveBeenCalledWith('zh-CN')
  })
})
