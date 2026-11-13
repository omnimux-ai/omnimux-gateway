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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useRef } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { ReferencePickerShell } from '../components/reference-picker-shell'

let slotTop = 360
let workspaceHeight = 800
let scrollTop = 0
let observerCallback: ResizeObserverCallback | undefined
const disconnect = vi.fn()

function Fixture(props: { open?: boolean; onClose: () => void }) {
  const anchorRef = useRef<HTMLDivElement>(null)
  return (
    <div data-testid='workspace'>
      <div className='inf-builder-scroll' data-testid='sidebar'>
        <div>
          <div ref={anchorRef} data-testid='slot'>
            Video slot
          </div>
        </div>
      </div>
      <ReferencePickerShell
        open={props.open ?? true}
        title='Select reference videos'
        anchorRef={anchorRef}
        onClose={props.onClose}
      >
        Candidate grid
      </ReferencePickerShell>
    </div>
  )
}

function mockGeometry() {
  slotTop = 360
  workspaceHeight = 800
  scrollTop = 0
  observerCallback = undefined
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(
    function (this: HTMLElement) {
      if (this.dataset.testid === 'workspace') {
        return new DOMRect(10, 80, 1000, workspaceHeight + 2)
      }
      if (this.classList.contains('inf-builder-scroll')) {
        return new DOMRect(22, 100, 320, 700)
      }
      if (this.dataset.testid === 'slot') {
        return new DOMRect(38, slotTop - scrollTop, 280, 160)
      }
      return new DOMRect()
    }
  )
  vi.spyOn(HTMLElement.prototype, 'offsetParent', 'get').mockImplementation(
    function (this: HTMLElement) {
      return window.innerWidth < 768
        ? null
        : this.closest('[data-testid="workspace"]')
    }
  )
  vi.spyOn(HTMLElement.prototype, 'clientTop', 'get').mockReturnValue(1)
  vi.spyOn(HTMLElement.prototype, 'clientHeight', 'get').mockImplementation(
    function (this: HTMLElement) {
      return this.dataset.testid === 'workspace' ? workspaceHeight : 700
    }
  )
  vi.spyOn(HTMLElement.prototype, 'scrollTop', 'get').mockImplementation(
    function (this: HTMLElement) {
      return this.classList.contains('inf-builder-scroll') ? scrollTop : 0
    }
  )
  vi.spyOn(HTMLElement.prototype, 'scrollTop', 'set').mockImplementation(
    (value) => {
      scrollTop = value
    }
  )
  vi.stubGlobal(
    'ResizeObserver',
    class {
      constructor(callback: ResizeObserverCallback) {
        observerCallback = callback
      }
      observe() {}
      disconnect = disconnect
    }
  )
}

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('reference picker slot alignment', () => {
  it('when first opening on mobile preserves the shared dialog instead of closing before breakpoint detection', () => {
    mockGeometry()
    const originalWidth = window.innerWidth
    Object.defineProperty(window, 'innerWidth', {
      configurable: true,
      value: 390,
    })
    const onClose = vi.fn()
    try {
      render(<Fixture onClose={onClose} />)
      expect(
        screen.getByRole('dialog', { name: 'Select reference videos' })
      ).toBeInTheDocument()
      expect(onClose).not.toHaveBeenCalled()
    } finally {
      Object.defineProperty(window, 'innerWidth', {
        configurable: true,
        value: originalWidth,
      })
    }
  })
  it('when opening at an offset workspace aligns the panel to the slot and caps remaining height', () => {
    mockGeometry()
    render(<Fixture onClose={vi.fn()} />)
    const panel = screen.getByRole('complementary')
    expect(panel).toHaveStyle({
      top: '279px',
      maxHeight: '396px',
      visibility: 'visible',
    })
  })

  it('when the sidebar scrolls follows the visible slot instead of remaining at the workspace top', async () => {
    mockGeometry()
    render(<Fixture onClose={vi.fn()} />)
    scrollTop = 60
    fireEvent.scroll(screen.getByTestId('sidebar'))
    await waitFor(() =>
      expect(screen.getByRole('complementary')).toHaveStyle({ top: '219px' })
    )
  })

  it('when content geometry changes remeasures the current slot', async () => {
    mockGeometry()
    render(<Fixture onClose={vi.fn()} />)
    slotTop = 400
    act(() => observerCallback?.([], {} as ResizeObserver))
    await waitFor(() =>
      expect(screen.getByRole('complementary')).toHaveStyle({ top: '319px' })
    )
  })

  it('when opening near the bottom minimally scrolls the sidebar to make room without breaking alignment', () => {
    mockGeometry()
    slotTop = 700
    render(<Fixture onClose={vi.fn()} />)
    expect(scrollTop).toBe(184)
    expect(screen.getByRole('complementary')).toHaveStyle({
      top: '435px',
      maxHeight: '240px',
    })
  })

  it('when the window shrinks recalculates available panel height', async () => {
    mockGeometry()
    render(<Fixture onClose={vi.fn()} />)
    workspaceHeight = 500
    fireEvent.resize(window)
    await waitFor(() =>
      expect(screen.getByRole('complementary')).toHaveStyle({
        maxHeight: '209px',
      })
    )
  })

  it('when the slot scrolls completely out of view closes the panel', async () => {
    mockGeometry()
    const onClose = vi.fn()
    render(<Fixture onClose={onClose} />)
    scrollTop = 500
    fireEvent.scroll(screen.getByTestId('sidebar'))
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
  })

  it('when the anchor is removed closes rather than showing a detached panel', async () => {
    mockGeometry()
    const onClose = vi.fn()
    render(<Fixture onClose={onClose} />)
    screen.getByTestId('slot').remove()
    await waitFor(() => expect(onClose).toHaveBeenCalledOnce())
  })

  it('when closed disconnects geometry observers and ignores later scroll events', async () => {
    mockGeometry()
    const onClose = vi.fn()
    const view = render(<Fixture onClose={onClose} />)
    view.rerender(<Fixture open={false} onClose={onClose} />)
    expect(disconnect).toHaveBeenCalled()
    scrollTop = 500
    fireEvent.scroll(screen.getByTestId('sidebar'))
    await act(async () => {
      await Promise.resolve()
    })
    expect(onClose).not.toHaveBeenCalled()
  })
})
