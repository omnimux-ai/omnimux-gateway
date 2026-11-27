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
import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('../api', () => ({ uploadReferenceAsset: vi.fn() }))

const KEY = 'influencer:motion-resolution'

beforeEach(() => {
  localStorage.clear()
  vi.resetModules()
})
afterEach(() => vi.restoreAllMocks())

async function mountMotion() {
  const { useMotion } = await import('../hooks/use-motion')
  return renderHook(() => useMotion())
}

describe('motion resolution preference', () => {
  it('defaults to 1080p when no quality has been chosen', async () => {
    const { result } = await mountMotion()
    expect(result.current.resolution).toBe('1080p')
    expect(localStorage.getItem(KEY)).toBeNull()
  })

  it('restores 1080p after a fresh page mount', async () => {
    const form = await mountMotion()
    act(() => form.result.current.setResolution('1080p'))
    expect(localStorage.getItem(KEY)).toBe('1080p')
    form.unmount()
    vi.resetModules()
    const restored = await mountMotion()
    expect(restored.result.current.resolution).toBe('1080p')
  })

  it('keeps 1080p when resetting the form and switching modes', async () => {
    const { result } = await mountMotion()
    act(() => result.current.setResolution('1080p'))
    act(() => result.current.reset())
    act(() => result.current.setMode('replace'))
    act(() => result.current.setMode('motion'))
    expect(result.current.resolution).toBe('1080p')
    expect(localStorage.getItem(KEY)).toBe('1080p')
  })

  it.each(['480p', '720p'])(
    'falls back to 1080p when saved legacy quality %s is unsupported',
    async (legacy) => {
      localStorage.setItem(KEY, legacy)
      const { result } = await mountMotion()
      expect(result.current.resolution).toBe('1080p')
    }
  )

  it('ignores attempts to set legacy unsupported resolutions', async () => {
    const { result } = await mountMotion()
    act(() => result.current.setResolution('720p'))
    expect(result.current.resolution).toBe('1080p')
    expect(localStorage.getItem(KEY)).toBeNull()
  })

  it('remains usable when preference storage cannot be read or written', async () => {
    vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => {
      throw new Error('unavailable')
    })
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('unavailable')
    })
    const { result } = await mountMotion()
    expect(result.current.resolution).toBe('1080p')
    act(() => result.current.setResolution('1080p'))
    expect(result.current.resolution).toBe('1080p')
  })
})
