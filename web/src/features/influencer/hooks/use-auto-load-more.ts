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
import { useEffect, useRef, type RefObject } from 'react'

/**
 * Watches a sentinel at the end of a picker grid and asks for the next page
 * shortly before it becomes visible. The caller keeps the manual button for
 * keyboard users and for browsers without IntersectionObserver.
 */
export function useAutoLoadMore(props: {
  enabled: boolean
  rootRef: RefObject<HTMLElement | null>
  onLoadMore: () => void
}): RefObject<HTMLDivElement | null> {
  const sentinelRef = useRef<HTMLDivElement>(null)
  const loadMoreRef = useRef(props.onLoadMore)
  loadMoreRef.current = props.onLoadMore

  useEffect(() => {
    const root = props.rootRef.current
    const sentinel = sentinelRef.current
    if (
      !props.enabled ||
      !root ||
      !sentinel ||
      typeof IntersectionObserver === 'undefined'
    ) {
      return
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          loadMoreRef.current()
        }
      },
      { root, rootMargin: '200px 0px' }
    )
    observer.observe(sentinel)
    return () => observer.disconnect()
  }, [props.enabled, props.rootRef])

  return sentinelRef
}
