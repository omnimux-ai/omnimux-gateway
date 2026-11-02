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
import { fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  LANDING_LANG_STORAGE_KEY,
  LandingLangProvider,
  useLanding,
} from '../../i18n'

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
