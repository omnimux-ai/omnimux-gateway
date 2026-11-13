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
import { useCallback, useRef, useState } from 'react'

export interface CoverLoadState {
  /** The cover bytes are on screen; the tile may fade the image in. */
  loaded: boolean
  /** The cover will never arrive; the tile has to fall back to its own backdrop. */
  failed: boolean
  /** Whether the tile should keep showing its skeleton. */
  pending: boolean
  onLoad: () => void
  onError: () => void
  /** Ref callback for the cover `<img>`; see the cache note below. */
  imageRef: (node: HTMLImageElement | null) => void
}

/**
 * Load state of one card cover, shared by both card grids so they behave the
 * same way.
 *
 * The skeleton is a card's resting state until its bytes land, so every escape
 * from it has to be covered:
 *
 * - `onError` settles a cover that will never arrive, instead of leaving the
 *   skeleton pulsing over a tile that has nothing to fade in;
 * - the ref settles a cover the browser already had in its cache, because a
 *   decoded image does not fire `load` again — without this a cached tile would
 *   keep its skeleton forever;
 * - a changed `src` restarts the cycle, because the grids swap items into the
 *   tiles they already have when a page or a filter changes, and the state of
 *   the previous cover must not carry over to the next one.
 */
export function useCoverLoadState(src: string): CoverLoadState {
  const [loaded, setLoaded] = useState(false)
  const [failed, setFailed] = useState(false)
  const [trackedSrc, setTrackedSrc] = useState(src)
  const settled = useRef(false)

  // React's documented way to drop state when an input changes: reset during
  // the render that sees the new value, so nothing is painted with stale state.
  if (trackedSrc !== src) {
    setTrackedSrc(src)
    setLoaded(false)
    setFailed(false)
    settled.current = false
  }

  const onLoad = useCallback(() => {
    settled.current = true
    setLoaded(true)
  }, [])

  const onError = useCallback(() => {
    settled.current = true
    setFailed(true)
  }, [])

  const imageRef = useCallback((node: HTMLImageElement | null) => {
    if (!node || settled.current || !node.complete) {
      return
    }
    settled.current = true
    if (node.naturalWidth > 0) {
      setLoaded(true)
      return
    }
    setFailed(true)
  }, [])

  return {
    loaded,
    failed,
    pending: !loaded && !failed,
    onLoad,
    onError,
    imageRef,
  }
}
