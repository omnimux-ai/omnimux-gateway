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
import { beforeEach, describe, expect, it } from 'vitest'

import { initializeFrontendCache } from '../frontend-cache'
import { FIRST_TOUCH_STORAGE_KEY } from '../growth-tracking'
import { LANDING_LANG_STORAGE_KEY } from '../landing-storage'

describe('initializeFrontendCache', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('keeps landing language and first-touch attribution when resetting stale UI cache', () => {
    const firstTouch = JSON.stringify({
      campaign: 'ai_growth_20261109',
      source: 'fresh',
      medium: '',
      content: '',
    })
    window.localStorage.setItem('newapi:default:cache-version', 'old')
    window.localStorage.setItem(LANDING_LANG_STORAGE_KEY, 'ja')
    window.localStorage.setItem(FIRST_TOUCH_STORAGE_KEY, firstTouch)
    window.localStorage.setItem('stale-ui-cache', 'old')

    initializeFrontendCache()

    expect(window.localStorage.getItem(LANDING_LANG_STORAGE_KEY)).toBe('ja')
    expect(window.localStorage.getItem(FIRST_TOUCH_STORAGE_KEY)).toBe(
      firstTouch
    )
    expect(window.localStorage.getItem('stale-ui-cache')).toBeNull()
  })
})
