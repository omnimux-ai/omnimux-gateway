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
import { describe, expect, it } from 'vitest'

import { useCoverLoadState } from '../use-cover-load-state'

/**
 * A cover the browser has already decoded: it reports itself complete and never
 * fires `load` again, which is the case the ref callback exists for.
 */
function settledImage(naturalWidth: number): HTMLImageElement {
  const node = document.createElement('img')
  Object.defineProperty(node, 'complete', { value: true })
  Object.defineProperty(node, 'naturalWidth', { value: naturalWidth })
  return node
}

describe('useCoverLoadState', () => {
  it('rests in the pending state until the cover settles', () => {
    const { result } = renderHook(() => useCoverLoadState('/a.jpg'))

    expect(result.current.pending).toBe(true)
    expect(result.current.loaded).toBe(false)
    expect(result.current.failed).toBe(false)
  })

  it('settles the cover once the image reports a load', () => {
    const { result } = renderHook(() => useCoverLoadState('/a.jpg'))

    act(() => result.current.onLoad())

    expect(result.current.loaded).toBe(true)
    expect(result.current.pending).toBe(false)
  })

  it('settles the cover as failed when the image errors, so the skeleton cannot pulse forever', () => {
    const { result } = renderHook(() => useCoverLoadState('/a.jpg'))

    act(() => result.current.onError())

    expect(result.current.failed).toBe(true)
    expect(result.current.pending).toBe(false)
  })

  it('settles an image the browser already had cached', () => {
    const { result } = renderHook(() => useCoverLoadState('/a.jpg'))

    act(() => result.current.imageRef(settledImage(1200)))

    expect(result.current.loaded).toBe(true)
    expect(result.current.pending).toBe(false)
  })

  it('fails a cached image that decoded to nothing rather than leaving it pending', () => {
    const { result } = renderHook(() => useCoverLoadState('/a.jpg'))

    act(() => result.current.imageRef(settledImage(0)))

    expect(result.current.failed).toBe(true)
    expect(result.current.pending).toBe(false)
  })

  it('restarts the cycle when the tile is handed a different cover', () => {
    const { result, rerender } = renderHook(
      ({ src }: { src: string }) => useCoverLoadState(src),
      { initialProps: { src: '/a.jpg' } }
    )

    act(() => result.current.onLoad())
    expect(result.current.loaded).toBe(true)

    rerender({ src: '/b.jpg' })

    expect(result.current.loaded).toBe(false)
    expect(result.current.failed).toBe(false)
    expect(result.current.pending).toBe(true)
  })
})
