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
import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { PromptItem } from '../../types'
import { PromptPreviewModal } from '../prompt-preview-modal'

const FULL_PROMPT =
  '8 seconds, 9:16 vertical, cinematic close-up of a couple, one continuous take.'
const DESCRIPTION =
  'A couple interacting naturally, with subtle micro-expressions.'

const videoItem: PromptItem = {
  id: 'loomi:seedance-2.5:couple-closeup',
  slug: 'couple-closeup',
  modelKey: 'seedance-2.5',
  modelLabel: 'Seedance 2.5',
  mediaType: 'video',
  title: 'Couple close-up, natural micro-expressions',
  description: DESCRIPTION,
  prompt: FULL_PROMPT,
  category: 'cinematic-showcase',
  useCases: ['cinematic-showcase'],
  styles: ['cinematic-realistic'],
  tags: ['cinematic-showcase'],
  previewImageUrl: 'https://cdn.example.com/cover.jpg',
  previewVideoUrl: 'https://cdn.example.com/clip.mp4',
  posterUrl: 'https://cdn.example.com/cover.jpg',
  authorName: 'Loomi',
  sourcePlatform: 'x',
  sourceUrl: 'https://x.com/example/status/1',
  publishedAt: '2026-09-11T06:35:58',
  viewCount: 1_250_000,
  originalViewCount: 0,
  aspectRatio: '9 / 16',
}

const secondItem: PromptItem = {
  ...videoItem,
  id: 'loomi:seedance-2.5:second',
  slug: 'second',
  title: 'Second tile',
  previewVideoUrl: '',
  mediaType: 'image',
}

const thirdItem: PromptItem = { ...secondItem, id: 'loomi:seedance-2.5:third', slug: 'third' }

const LIST: PromptItem[] = [videoItem, secondItem, thirdItem]

function renderModal(
  overrides: Partial<Parameters<typeof PromptPreviewModal>[0]> = {}
) {
  const props = {
    items: LIST,
    index: 0,
    onIndexChange: vi.fn(),
    onClose: vi.fn(),
    onUsePrompt: vi.fn(),
    ...overrides,
  }
  const view = render(<PromptPreviewModal {...props} />)
  return { ...props, ...view }
}

/** The detail layer owns the prompt: the tile deliberately does not carry it. */
describe('PromptPreviewModal detail layer', () => {
  it('prints the full prompt together with the model, title and description', () => {
    renderModal()

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText(FULL_PROMPT)).toBeInTheDocument()
    expect(screen.getByText(videoItem.modelLabel)).toBeInTheDocument()
    expect(
      screen.getByRole('heading', { name: videoItem.title })
    ).toBeInTheDocument()
    expect(screen.getByText(DESCRIPTION)).toBeInTheDocument()
  })

  it('hands the item to the generation entry point', async () => {
    const onUsePrompt = vi.fn()
    const user = userEvent.setup()
    renderModal({ onUsePrompt })

    await user.click(screen.getByRole('button', { name: /use this prompt/i }))

    expect(onUsePrompt).toHaveBeenCalledWith(videoItem)
  })

  it('closes on Escape without touching the generation entry point', async () => {
    const user = userEvent.setup()
    const { onClose, onUsePrompt } = renderModal()

    await user.keyboard('{Escape}')

    expect(onClose).toHaveBeenCalled()
    expect(onUsePrompt).not.toHaveBeenCalled()
  })

  it('shows the poster first and only loads the video once playback is asked for', async () => {
    const user = userEvent.setup()
    renderModal()

    expect(document.querySelector('video')).toBeNull()

    await user.click(
      screen.getByRole('button', { name: /play the rendered video/i })
    )

    expect(document.querySelector('video')).toBeInTheDocument()
  })

  it('offers no playback control for an item with no video', () => {
    renderModal({ index: 1 })

    expect(
      screen.queryByRole('button', { name: /play the rendered video/i })
    ).not.toBeInTheDocument()
  })

  it('renders nothing when the index is out of range', () => {
    renderModal({ items: [], index: -1 })

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('fills the viewport and gives the reading pane its own scroll', () => {
    renderModal()

    const dialog = screen.getByRole('dialog')
    expect(dialog.className).toContain('h-[100dvh]')
    expect(dialog.className).toContain('w-screen')

    const scroller = document.querySelector('[data-modal-scroll]')
    expect(scroller?.className).toContain('overflow-y-auto')
  })

  it('keeps the actions outside the scrolling pane so they stay in view', () => {
    renderModal()

    const scroller = document.querySelector('[data-modal-scroll]')
    const usePrompt = screen.getByRole('button', { name: /use this prompt/i })

    // Anything inside the scroller moves out of view with the prompt text.
    expect(scroller).not.toContainElement(usePrompt)
    expect(scroller?.parentElement).toContainElement(usePrompt)
  })

  it('shows the position of the open tile inside the list', () => {
    renderModal({ index: 1 })

    expect(screen.getByText('2 / 3')).toBeInTheDocument()
  })

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

  it('steps the tile even when the dialog stops the arrow key from bubbling', () => {
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

  it('does not step past either end of the list', () => {
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

  it('flips the tile on a wheel or trackpad flick', () => {
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
    const scroller = document.querySelector('[data-modal-scroll]') as HTMLElement
    Object.defineProperty(scroller, 'scrollHeight', { value: 1200 })
    Object.defineProperty(scroller, 'clientHeight', { value: 400 })
    scroller.scrollTop = 0

    fireEvent.wheel(scroller, { deltaY: 120 })

    expect(onIndexChange).not.toHaveBeenCalled()
  })

  it('flips the tile on a horizontal swipe', () => {
    const onIndexChange = vi.fn()
    renderModal({ index: 1, onIndexChange })
    const dialog = screen.getByRole('dialog')

    fireEvent.touchStart(dialog, { touches: [{ clientX: 300, clientY: 400 }] })
    fireEvent.touchEnd(dialog, {
      changedTouches: [{ clientX: 180, clientY: 405 }],
    })

    expect(onIndexChange).toHaveBeenLastCalledWith(2)
  })

  it('puts the poster back when the visitor flips to another tile', async () => {
    const user = userEvent.setup()
    const props = renderModal({ index: 0 })
    expect(document.querySelector('video')).toBeNull()

    await user.click(
      screen.getByRole('button', { name: /play the rendered video/i })
    )
    expect(document.querySelector('video')).toBeInTheDocument()

    props.rerender(
      <PromptPreviewModal {...props} items={LIST} index={2} />
    )
    expect(document.querySelector('video')).toBeNull()
  })
})
