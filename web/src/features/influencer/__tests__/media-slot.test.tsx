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

import { act, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ComponentProps } from 'react'
import { toast } from 'sonner'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { MediaSlot } from '../components/media-slot'

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))

beforeEach(() => {
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
})

/**
 * The visible affordance is the styled button; the file input behind it is
 * `hidden`, so the upload path is driven through that input the way the browser
 * does once the picker closes.
 */
function renderSlot(overrides: Partial<ComponentProps<typeof MediaSlot>> = {}) {
  const onPick = vi.fn()
  const onClear = vi.fn()
  const view = render(
    <MediaSlot
      kind='image'
      label='Character'
      hint='The face, hair and outfit to keep'
      accept='image/png,image/jpeg'
      onPick={onPick}
      onClear={onClear}
      {...overrides}
    />
  )
  return { ...view, onPick, onClear }
}

function fileInput(container: HTMLElement): HTMLInputElement {
  const input = container.querySelector('input[type=file]')
  if (!(input instanceof HTMLInputElement)) {
    throw new Error('the slot rendered no file input')
  }
  return input
}

describe('MediaSlot reference panel entry', () => {
  it('opens the provided panel instead of the native file chooser', async () => {
    const openPanel = vi.fn()
    renderSlot({ kind: 'video', onOpenPicker: openPanel })
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Character' }))
    expect(openPanel).toHaveBeenCalledOnce()
  })
  it('rejects several dropped videos rather than picking the first', () => {
    const { container, onPick } = renderSlot({ kind: 'video' })
    fireDrop(container.firstElementChild as HTMLElement, [
      new File(['clip'], 'one.mp4', { type: 'video/mp4' }),
      new File(['clip'], 'two.mp4', { type: 'video/mp4' }),
    ])
    expect(onPick).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalledWith('Choose one video only')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })
})

describe('MediaSlot empty state', () => {
  it('exposes the slot label as the accessible name', () => {
    renderSlot()
    expect(
      screen.getByRole('button', { name: 'Character' })
    ).toBeInTheDocument()
  })

  it('shows the hint so the caller can explain what belongs here', () => {
    renderSlot()
    expect(
      screen.getByText('The face, hair and outfit to keep')
    ).toBeInTheDocument()
  })

  it('passes the picked file to onPick', async () => {
    const user = userEvent.setup()
    const { container, onPick } = renderSlot()
    const file = new File(['bytes'], 'face.png', { type: 'image/png' })

    await user.upload(fileInput(container), file)

    expect(onPick).toHaveBeenCalledWith(file)
  })

  it('passes a dropped file to onPick', async () => {
    const { container, onPick } = renderSlot()
    const file = new File(['bytes'], 'face.png', { type: 'image/png' })

    fireDrop(container.firstElementChild as HTMLElement, [file])

    expect(onPick).toHaveBeenCalledWith(file)
  })

  it('does nothing when a drop carries no file', () => {
    const { container, onPick } = renderSlot()

    fireDrop(container.firstElementChild as HTMLElement, [])

    expect(onPick).not.toHaveBeenCalled()
  })
})

describe('MediaSlot reference preview', () => {
  it('automatically loops a selected video silently and inline', () => {
    const { container } = renderSlot({ kind: 'video', previewUrl: '/clip.mp4' })
    const video = container.querySelector('video')
    if (!video) throw new Error('selected video missing')
    expect(video).toHaveAttribute('autoplay')
    expect(video).toHaveAttribute('loop')
    expect(video.muted).toBe(true)
    expect(video).toHaveAttribute('playsinline')
  })

  it('hands a selected video to the shared playback window without replacing it', async () => {
    const openPicker = vi.fn()
    const onPreviewVideo = vi.fn()
    const { onClear } = renderSlot({
      kind: 'video',
      previewUrl: '/clip.mp4',
      onOpenPicker: openPicker,
      onPreviewVideo,
    })
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Preview: Character' }))
    expect(onPreviewVideo).toHaveBeenCalledWith('/clip.mp4')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(openPicker).not.toHaveBeenCalled()
    expect(onClear).not.toHaveBeenCalled()
  })

  it('hands a selected image to the shared preview window without clearing it', async () => {
    const onPreviewImage = vi.fn()
    const { onClear } = renderSlot({
      kind: 'image',
      previewUrl: '/face.png',
      variant: 'reference',
      onPreviewImage,
    })
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Preview: Character' }))
    expect(onPreviewImage).toHaveBeenCalledWith('/face.png')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(onClear).not.toHaveBeenCalled()
  })

  it('offers manual playback only after autoplay is refused and recovers after playing', async () => {
    vi.mocked(HTMLMediaElement.prototype.play).mockRejectedValueOnce(
      new DOMException('Blocked', 'NotAllowedError')
    )
    const { container } = renderSlot({ kind: 'video', previewUrl: '/clip.mp4' })
    const play = await screen.findByRole('button', { name: 'Play video' })
    await userEvent.setup().click(play)
    act(() =>
      container.querySelector('video')?.dispatchEvent(new Event('playing'))
    )
    expect(
      screen.queryByRole('button', { name: 'Play video' })
    ).not.toBeInTheDocument()
  })

  it('does not offer a preview when a template has no media address', () => {
    renderSlot({ kind: 'video', caption: 'Club dance' })
    expect(
      screen.queryByRole('button', { name: /Preview:/ })
    ).not.toBeInTheDocument()
  })
})

describe('MediaSlot filled state', () => {
  it('renders the caption and an explicit remove control', () => {
    renderSlot({ previewUrl: '/face.png', caption: 'face.png' })
    expect(screen.getByText('face.png')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Remove/ })).toBeInTheDocument()
  })

  it('calls onClear when the remove control is used', async () => {
    const user = userEvent.setup()
    const { onClear } = renderSlot({ previewUrl: '/face.png' })

    await user.click(screen.getByRole('button', { name: /Remove/ }))

    expect(onClear).toHaveBeenCalledTimes(1)
  })

  it('offers a replace control instead of the empty-state button', () => {
    renderSlot({ previewUrl: '/face.png' })
    expect(screen.getByRole('button', { name: /Replace/ })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Character' })).toBeNull()
  })

  it('shows the supplied badge without waiting for media metadata', () => {
    renderSlot({ kind: 'video', previewUrl: '/clip.mp4', badge: '10s' })
    expect(screen.getByText('10s')).toBeInTheDocument()
  })

  it('treats a chosen template as filled even though it has no file yet', () => {
    renderSlot({ kind: 'video', caption: 'Club dance', badge: '10s' })
    expect(screen.queryByRole('button', { name: 'Character' })).toBeNull()
    expect(screen.getByRole('button', { name: /Remove/ })).toBeInTheDocument()
    expect(screen.getByText('Club dance')).toBeInTheDocument()
    expect(screen.getByText('10s')).toBeInTheDocument()
  })
})

/** jsdom has no drag pipeline, so the drop event is dispatched directly. */
function fireDrop(target: HTMLElement, files: File[]): void {
  const event = new Event('drop', { bubbles: true, cancelable: true })
  Object.defineProperty(event, 'dataTransfer', { value: { files } })
  act(() => target.dispatchEvent(event))
}
