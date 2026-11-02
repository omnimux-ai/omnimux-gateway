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
import { useEffect, useState } from 'react'

/**
 * Top offset of the sticky site header, in pixels.
 *
 * The benchmark page reserves the same amount (`scroll-mt-[176px]`) so an
 * anchored section lands below the header rather than underneath it.
 */
export const MODEL_PAGE_SCROLL_MARGIN_PX = 176

/**
 * Reports which anchored section is currently in view.
 *
 * The benchmark model page highlights the section the reader has scrolled to,
 * which requires observing sections instead of tracking a selected tab.
 * Callers must pass a stable array (a module-level constant), otherwise the
 * observer is torn down and rebuilt on every render.
 */
export function useActiveSection(sectionIds: readonly string[]): string {
  const [activeId, setActiveId] = useState(sectionIds[0] ?? '')

  useEffect(() => {
    // Non-browser and older test environments have no IntersectionObserver;
    // the nav then simply keeps its initial section rather than crashing.
    if (typeof IntersectionObserver === 'undefined') return

    const elements = sectionIds
      .map((id) => document.getElementById(id))
      .filter((element): element is HTMLElement => element !== null)
    if (elements.length === 0) return

    const observer = new IntersectionObserver(
      (entries) => {
        const visible = [...entries]
          .filter((entry) => entry.isIntersecting)
          .sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top)
        if (visible.length > 0) setActiveId(visible[0].target.id)
      },
      { rootMargin: `-${MODEL_PAGE_SCROLL_MARGIN_PX}px 0px -60% 0px` }
    )

    elements.forEach((element) => observer.observe(element))
    return () => observer.disconnect()
  }, [sectionIds])

  return activeId
}
