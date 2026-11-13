import { vi } from 'vitest'

/** jsdom has no layout; provide only the browser geometry used by the real picker. */
export function mockReferenceLayout(): void {
  vi.spyOn(HTMLElement.prototype, 'offsetParent', 'get').mockImplementation(
    function (this: HTMLElement) {
      return this.parentElement
    }
  )
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockReturnValue(800)
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(
    function (this: HTMLElement) {
      if (this.classList.contains('inf-builder-scroll')) {
        return new DOMRect(12, 60, 320, 700)
      }
      if (this.classList.contains('inf-slot')) {
        return new DOMRect(32, 300, 280, 160)
      }
      return new DOMRect(0, 48, 1200, 800)
    }
  )
}
