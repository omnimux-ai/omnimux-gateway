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
import { useLayoutEffect, type RefObject } from 'react'

/** Align the desktop reference panel without changing its selection lifecycle. */
export function useReferencePickerPosition(props: {
  enabled: boolean
  panelRef: RefObject<HTMLElement | null>
  anchorRef?: RefObject<HTMLElement | null>
  onClose: () => void
}): void {
  const { enabled, panelRef, anchorRef, onClose } = props
  useLayoutEffect(() => {
    // The shared mobile detector settles after the first layout effect.
    if (!enabled || !anchorRef || window.innerWidth < 768) return
    const panel = panelRef.current
    const anchor = anchorRef.current
    const workspace = panel?.offsetParent
    const sidebar = anchor?.closest<HTMLElement>('.inf-builder-scroll')
    if (!panel || !anchor || !(workspace instanceof HTMLElement) || !sidebar) {
      onClose()
      return
    }
    const view = panel.ownerDocument.defaultView
    if (!view) return
    const bottomGap = 12
    const minimumHeight = 240
    let frame = 0
    let closed = false
    const update = (makeRoom: boolean): void => {
      if (closed) return
      let rect = anchor.getBoundingClientRect()
      const visible = sidebar.getBoundingClientRect()
      if (
        !anchor.isConnected ||
        rect.height === 0 ||
        rect.bottom <= visible.top ||
        rect.top >= visible.bottom
      ) {
        closed = true
        panel.style.visibility = 'hidden'
        onClose()
        return
      }
      const origin = workspace.getBoundingClientRect().top + workspace.clientTop
      const bottom =
        Math.min(origin + workspace.clientHeight, view.innerHeight) - bottomGap
      if (makeRoom && bottom - rect.top < minimumHeight) {
        sidebar.scrollTop += Math.min(
          minimumHeight - (bottom - rect.top),
          Math.max(0, rect.top - visible.top)
        )
        rect = anchor.getBoundingClientRect()
      }
      panel.style.top = `${rect.top - origin + workspace.scrollTop}px`
      panel.style.maxHeight = `${Math.max(0, bottom - rect.top)}px`
      panel.style.visibility = 'visible'
    }
    const schedule = (): void => {
      if (frame || closed) return
      frame = view.requestAnimationFrame(() => {
        frame = 0
        update(false)
      })
    }
    update(true)
    const observer = new ResizeObserver(schedule)
    observer.observe(workspace)
    for (
      let element: HTMLElement | null = anchor;
      element;
      element = element.parentElement
    ) {
      observer.observe(element)
      if (element === sidebar) break
    }
    const mutations = new MutationObserver(schedule)
    mutations.observe(sidebar, { childList: true, subtree: true })
    sidebar.addEventListener('scroll', schedule, { passive: true })
    view.addEventListener('resize', schedule)
    view.addEventListener('scroll', schedule, { passive: true })
    return () => {
      observer.disconnect()
      mutations.disconnect()
      sidebar.removeEventListener('scroll', schedule)
      view.removeEventListener('resize', schedule)
      view.removeEventListener('scroll', schedule)
      if (frame) view.cancelAnimationFrame(frame)
    }
  }, [enabled, panelRef, anchorRef, onClose])
}
