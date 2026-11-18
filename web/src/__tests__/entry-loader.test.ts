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
import { beforeEach, describe, expect, it, vi } from 'vitest'

import type { EntryWindow } from '../entry-loader'

describe('loadEntry', () => {
  beforeEach(() => {
    vi.resetModules()
    document.head.innerHTML = ''
    delete (window as EntryWindow).__OMNIMUX_ENTRIES__
  })

  it('loads each manifest entry once in order and marks scripts non-async', async () => {
    ;(window as EntryWindow).__OMNIMUX_ENTRIES__ = {
      landing: ['/static/css/landing.css', '/static/js/landing.js'],
      app: ['/static/js/app.js'],
    }
    const { loadEntry } = await import('../entry-loader')

    loadEntry('landing')
    loadEntry('landing')

    const links = document.head.querySelectorAll('link[rel="stylesheet"]')
    const scripts = document.head.querySelectorAll('script')
    expect(links).toHaveLength(1)
    expect(links[0].getAttribute('href')).toBe('/static/css/landing.css')
    expect(scripts).toHaveLength(1)
    expect(scripts[0].getAttribute('src')).toBe('/static/js/landing.js')
    expect(scripts[0].async).toBe(false)
  })

  it('fails clearly when the build manifest is missing', async () => {
    const { loadEntry } = await import('../entry-loader')

    expect(() => loadEntry('landing')).toThrow(
      'OmniMux entry manifest is not available'
    )
  })
})
