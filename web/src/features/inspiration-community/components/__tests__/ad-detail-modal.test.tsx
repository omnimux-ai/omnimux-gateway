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
/**
 * The detail layer of the ad library: the payload's own fields, TikTok's player
 * only when there is one, and the same neighbour browsing as the prompt layer.
 *
 * `mediaUrls` is deliberately not a fallback for playback: every object behind
 * it answers 404 today, so a player built from it would open on an error page.
 */
import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import {
  LONG_CAPTION,
  PHOTO_ROW,
  UNPLAYABLE_VIDEO_ROW,
  VIDEO_ROW,
} from '../../api/__tests__/fixtures/tiktok-rows'
import { mapTikTokAdItem } from '../../api/tiktok'
import type { TikTokAdItem } from '../../types'
import { AdDetailModal } from '../ad-detail-modal'

const LIST: TikTokAdItem[] = [
  mapTikTokAdItem({ ...VIDEO_ROW, id: 9101, title: 'First creative' }),
  mapTikTokAdItem({ ...PHOTO_ROW, id: 9102, title: 'Second creative' }),
  mapTikTokAdItem({
    ...UNPLAYABLE_VIDEO_ROW,
    id: 9103,
    title: 'Third creative',
  }),
]

function renderModal(
  overrides: {
    items?: TikTokAdItem[]
    index?: number
    onIndexChange?: (index: number) => void
    onClose?: () => void
    onRecreate?: (item: TikTokAdItem) => void
  } = {}
) {
  return render(
    <AdDetailModal
      items={overrides.items ?? LIST}
      index={overrides.index ?? 0}
      onIndexChange={overrides.onIndexChange ?? vi.fn()}
      onClose={overrides.onClose ?? vi.fn()}
      onRecreate={overrides.onRecreate ?? vi.fn()}
    />
  )
}

/** The dialog renders into a portal, so structural reads go to the document. */
function inDocument(selector: string): Element | null {
  return document.querySelector(selector)
}

describe('AdDetailModal fields', () => {
  it('prints the creator, the region, the category and the metrics', () => {
    renderModal()

    // The creator line is one definition value split across two nodes.
    expect(
      screen.getByText(
        (_content, element) =>
          element?.tagName.toLowerCase() === 'dd' &&
          element.textContent === 'demo_creator @demo_creator'
      )
    ).toBeInTheDocument()
    expect(screen.getByText('US')).toBeInTheDocument()
    expect(screen.getByText('Health & Wellness')).toBeInTheDocument()
    expect(screen.getByText('95.60M')).toBeInTheDocument()
    expect(screen.getByText('95,600,000')).toBeInTheDocument()
    expect(screen.getByText('0:24')).toBeInTheDocument()
    expect(screen.getByText('Organic')).toBeInTheDocument()
  })

  it('never prints a revenue or ROAS metric', () => {
    renderModal()

    expect(screen.queryByText('Revenue')).not.toBeInTheDocument()
    expect(screen.queryByText('ROAS')).not.toBeInTheDocument()
    expect(screen.queryByText(/^\$/u)).not.toBeInTheDocument()
  })

  it('hides the region, the category, the creator and the date when absent', () => {
    renderModal({
      items: [
        mapTikTokAdItem({
          ...PHOTO_ROW,
          country: '',
          category: '',
          creator: null,
        }),
      ],
    })

    expect(screen.queryByText('Creator')).not.toBeInTheDocument()
    expect(screen.queryByText('Health & Wellness')).not.toBeInTheDocument()
    expect(screen.queryByText('DE')).not.toBeInTheDocument()
    // The clip length is unknown for a photo post, so it prints the placeholder.
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  it('wraps a caption long enough to overflow the pane', () => {
    renderModal({
      items: [mapTikTokAdItem({ ...VIDEO_ROW, caption: LONG_CAPTION })],
    })

    expect(LONG_CAPTION.length).toBeGreaterThan(400)
    expect(screen.getByText(LONG_CAPTION)).toBeInTheDocument()
  })

  it('renders nothing when the index is out of range', () => {
    renderModal({ items: [], index: -1 })

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})

describe('AdDetailModal layout', () => {
  it('fills the viewport and gives the reading pane its own scroll', () => {
    renderModal()

    const dialog = screen.getByRole('dialog')
    expect(dialog.className).toContain('h-[100dvh]')
    expect(dialog.className).toContain('w-screen')

    const scroller = inDocument('[data-modal-scroll]')
    expect(scroller?.className).toContain('overflow-y-auto')
  })

  it('shows the position of the open creative inside the list', () => {
    renderModal({ index: 1 })

    expect(screen.getByText('2 / 3')).toBeInTheDocument()
  })
})

describe('AdDetailModal playback', () => {
  it('mounts the official player only after the visitor asks for it', async () => {
    renderModal()

    expect(inDocument('iframe')).toBeNull()

    await userEvent.click(screen.getByRole('button', { name: 'Play video' }))

    const frame = inDocument('iframe')
    expect(frame?.getAttribute('src')).toBe(
      'https://www.tiktok.com/player/v1/9001'
    )
    expect(frame?.getAttribute('allow')).toContain('autoplay')
  })

  it('puts the cover back when the visitor flips to another creative', async () => {
    const onIndexChange = vi.fn()
    const { rerender } = renderModal({ index: 0, onIndexChange })

    await userEvent.click(screen.getByRole('button', { name: 'Play video' }))
    expect(inDocument('iframe')).not.toBeNull()

    rerender(
      <AdDetailModal
        items={LIST}
        index={1}
        onIndexChange={onIndexChange}
        onClose={vi.fn()}
        onRecreate={vi.fn()}
      />
    )

    // A player that is still mounted would otherwise follow the visitor.
    expect(inDocument('iframe')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Play video' })).toBeNull()
  })

  it('offers no player for a creative whose embed URL is absent', () => {
    renderModal({ index: 1 })

    expect(screen.queryByRole('button', { name: 'Play video' })).toBeNull()
    expect(inDocument('iframe')).toBeNull()
    // The cover is still the preview of the post.
    expect(inDocument('img')?.getAttribute('src')).toBe(
      '/api/inspiration/v1/public/media/inspiration-covers/9002'
    )
  })

  it('offers no player for an mp4 the service cannot serve', () => {
    renderModal({ index: 2 })

    expect(screen.queryByRole('button', { name: 'Play video' })).toBeNull()
    expect(inDocument('iframe')).toBeNull()
    expect(inDocument('video')).toBeNull()
  })
})

describe('AdDetailModal browsing', () => {
  it('steps forward and back with the arrow keys', () => {
    // Key events land on the layer from anywhere on the page, so they are
    // driven straight at the window the layer listens on.
    const onIndexChange = vi.fn()
    const { unmount } = renderModal({ index: 1, onIndexChange })

    fireEvent.keyDown(window, { key: 'ArrowRight' })
    expect(onIndexChange).toHaveBeenLastCalledWith(2)
    unmount()

    renderModal({ index: 1, onIndexChange })
    fireEvent.keyDown(window, { key: 'ArrowLeft' })
    expect(onIndexChange).toHaveBeenLastCalledWith(0)
  })

  it('steps the card even when the dialog stops the arrow key from bubbling', () => {
    // The layer sits on Base UI, whose popup calls `stopPropagation()` on every
    // arrow key it sees. A listener on the bubble phase is therefore cut off
    // before the event reaches the window — which is why the arrows only ever
    // broke in a real browser and passed a test that dispatches at the window.
    const onIndexChange = vi.fn()
    renderModal({ index: 1, onIndexChange })

    const dialog = screen.getByRole('dialog')
    dialog.addEventListener('keydown', (event) => event.stopPropagation())
    fireEvent.keyDown(screen.getByRole('button', { name: /next/i }), {
      key: 'ArrowRight',
    })

    expect(onIndexChange).toHaveBeenLastCalledWith(2)
  })

  it('does not step past either end of the loaded list', () => {
    const onIndexChange = vi.fn()
    const { unmount } = renderModal({ index: 0, onIndexChange })

    expect(screen.getByRole('button', { name: /previous/i })).toBeDisabled()
    fireEvent.keyDown(window, { key: 'ArrowLeft' })
    expect(onIndexChange).not.toHaveBeenCalled()
    unmount()

    renderModal({ index: LIST.length - 1, onIndexChange })
    expect(screen.getByRole('button', { name: /next/i })).toBeDisabled()
    fireEvent.keyDown(window, { key: 'ArrowRight' })
    expect(onIndexChange).not.toHaveBeenCalled()
  })

  it('steps through the arrows', async () => {
    const user = userEvent.setup()
    const onIndexChange = vi.fn()
    renderModal({ index: 1, onIndexChange })

    await user.click(screen.getByRole('button', { name: /next/i }))
    expect(onIndexChange).toHaveBeenLastCalledWith(2)

    await user.click(screen.getByRole('button', { name: /previous/i }))
    expect(onIndexChange).toHaveBeenLastCalledWith(0)
  })

  it('flips the creative on a wheel or trackpad flick', () => {
    const onIndexChange = vi.fn()
    const { unmount } = renderModal({ index: 1, onIndexChange })

    fireEvent.wheel(screen.getByRole('dialog'), { deltaY: 120 })
    expect(onIndexChange).toHaveBeenLastCalledWith(2)
    unmount()

    // Flipping back is a fresh gesture, not the tail of the previous flick.
    renderModal({ index: 1, onIndexChange })
    fireEvent.wheel(screen.getByRole('dialog'), { deltaX: -120 })
    expect(onIndexChange).toHaveBeenLastCalledWith(0)
  })

  it('leaves a wheel flick to the reading pane while that pane can still scroll', () => {
    const onIndexChange = vi.fn()
    renderModal({ index: 1, onIndexChange })
    const scroller = inDocument('[data-modal-scroll]') as HTMLElement
    Object.defineProperty(scroller, 'scrollHeight', { value: 1200 })
    Object.defineProperty(scroller, 'clientHeight', { value: 400 })
    scroller.scrollTop = 0

    fireEvent.wheel(scroller, { deltaY: 120 })

    expect(onIndexChange).not.toHaveBeenCalled()
  })

  it('flips the creative on a horizontal swipe', () => {
    const onIndexChange = vi.fn()
    renderModal({ index: 1, onIndexChange })
    const dialog = screen.getByRole('dialog')

    fireEvent.touchStart(dialog, { touches: [{ clientX: 300, clientY: 400 }] })
    fireEvent.touchEnd(dialog, {
      changedTouches: [{ clientX: 180, clientY: 405 }],
    })

    expect(onIndexChange).toHaveBeenLastCalledWith(2)
  })
})

describe('AdDetailModal original post link', () => {
  it('links to the original post when the payload carries one', () => {
    renderModal()

    const link = screen.getByRole('link', { name: 'View original post' })
    expect(link).toHaveAttribute(
      'href',
      'https://www.tiktok.com/@demo/video/9001'
    )
    expect(link).toHaveAttribute('target', '_blank')
    expect(link.getAttribute('rel')).toContain('noopener')
  })

  it('renders no external link when the payload has no real one', () => {
    renderModal({ index: 1 })

    expect(
      screen.queryByRole('link', { name: 'View original post' })
    ).not.toBeInTheDocument()
  })

  it('keeps the recreation action available', async () => {
    const onRecreate = vi.fn()
    renderModal({ index: 0, onRecreate })

    await userEvent.click(
      screen.getByRole('button', { name: 'Recreate this ad' })
    )

    expect(onRecreate).toHaveBeenCalledWith(LIST[0])
  })
})
