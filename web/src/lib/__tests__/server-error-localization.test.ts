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
import i18next from 'i18next'
import { afterEach, describe, expect, it } from 'vitest'

import { getServerErrorMessage } from '../server-error-message'

const key = 'No linked OAuth provider is available.'

afterEach(async () => {
  await i18next.changeLanguage('en')
})

describe('shared error localization', () => {
  it('localizes legacy payloads, thrown errors and fallback text in the active language', async () => {
    i18next.addResourceBundle('zhCN', 'translation', {
      [key]: '没有可用的关联登录方式。',
    })
    await i18next.changeLanguage('zhCN')
    for (const value of [
      { message: key },
      { msg: key },
      { error: key },
      { error: { message: key } },
      { title: key },
      new Error(key),
      key,
    ]) {
      expect(getServerErrorMessage(value)).toBe('没有可用的关联登录方式。')
    }
    expect(getServerErrorMessage({}, key)).toBe('没有可用的关联登录方式。')
    await i18next.changeLanguage('en')
    expect(getServerErrorMessage({ message: key })).toBe(key)
  })

  it('preserves unrecognized upstream details and literal interpolation syntax', async () => {
    i18next.addResourceBundle('zhCN', 'translation', {
      'Task timed out': '任务超时',
    })
    await i18next.changeLanguage('zhCN')
    const detail = 'Supplier says {{reason}} and $t(Task timed out)'
    expect(getServerErrorMessage({ message: detail })).toBe(detail)
    expect(getServerErrorMessage(new Error(detail))).toBe(detail)
  })
})
