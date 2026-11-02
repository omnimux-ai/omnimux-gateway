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

import {
  act,
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { StrictMode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { MotionStyleDialog } from '../components/motion-style-dialog'
import { MOTION_VISUAL_STYLES } from '../lib/motion-visual-styles'
import type { MotionPreset } from '../types'

function preset(over: Partial<MotionPreset> = {}): MotionPreset {
  return {
    id: 'p1',
    nameKey: 'Umbrella Transformation',
    mode: 'replace',
    prompt: 'keep the umbrella',
    durationSec: 5,
    thumbUrl: 'https://cdn/umbrella.webp',
    videoUrl: 'https://cdn/umbrella.mp4',
    ...over,
  }
}

const presets = [
  preset(),
  preset({
    id: 'p2',
    nameKey: 'Selfie Walk',
    videoUrl: 'https://cdn/selfie.mp4',
  }),
]

// jsdom has no media pipeline; preserve its API boundary while exercising the picker.
const originalPlay = HTMLMediaElement.prototype.play
const originalPause = HTMLMediaElement.prototype.pause
const originalLoad = HTMLMediaElement.prototype.load
let play: ReturnType<typeof vi.fn>
let pause: ReturnType<typeof vi.fn>

beforeEach(() => {
  play = vi.fn().mockResolvedValue(undefined)
  pause = vi.fn()
  HTMLMediaElement.prototype.play = play as unknown as () => Promise<void>
  HTMLMediaElement.prototype.pause = pause as unknown as () => void
  HTMLMediaElement.prototype.load = vi.fn()
})

afterEach(() => {
  cleanup()
  HTMLMediaElement.prototype.play = originalPlay
  HTMLMediaElement.prototype.pause = originalPause
  HTMLMediaElement.prototype.load = originalLoad
})

function renderDialog(selectedId: string | null = null) {
  return render(
    <MotionStyleDialog
      open
      presets={presets}
      selectedId={selectedId}
      onOpenChange={vi.fn()}
      onApply={vi.fn()}
    />
  )
}

describe('MotionStyleDialog visual styles', () => {
  it('shows all 20 styles with lazy covers and short descriptions', () => {
    render(
      <MotionStyleDialog
        open
        presets={MOTION_VISUAL_STYLES}
        selectedId={null}
        onOpenChange={vi.fn()}
        onApply={vi.fn()}
      />
    )
    const first = screen.getByRole('option', {
      name: MOTION_VISUAL_STYLES[0].nameKey,
    })
    expect(first).toHaveTextContent(MOTION_VISUAL_STYLES[0].summaryKey)
    expect(screen.getAllByRole('option')).toHaveLength(21)
    expect(first.querySelector('img')).toHaveAttribute(
      'src',
      MOTION_VISUAL_STYLES[0].previewImageUrl
    )
    expect(first.querySelector('img')).toHaveAttribute('loading', 'lazy')
    expect(document.querySelector('video')).toBeNull()
    expect(
      screen.queryByText(MOTION_VISUAL_STYLES[0].prompt)
    ).not.toBeInTheDocument()
  })

  it('stages a keyboard choice and only returns the original style on Apply', async () => {
    const user = userEvent.setup()
    const onApply = vi.fn()
    const onOpenChange = vi.fn()
    render(
      <MotionStyleDialog
        open
        presets={MOTION_VISUAL_STYLES}
        selectedId={null}
        onOpenChange={onOpenChange}
        onApply={onApply}
      />
    )
    const option = screen.getByRole('option', {
      name: MOTION_VISUAL_STYLES[1].nameKey,
    })
    act(() => option.focus())
    await user.keyboard('{Enter}')
    expect(option).toHaveAttribute('aria-selected', 'true')
    expect(onApply).not.toHaveBeenCalled()
    await user.click(screen.getByRole('button', { name: 'Apply style' }))
    expect(onApply).toHaveBeenCalledWith(MOTION_VISUAL_STYLES[1])
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  it('clears a selected style only after No style is applied', async () => {
    const user = userEvent.setup()
    const onApply = vi.fn()
    render(
      <MotionStyleDialog
        open
        presets={MOTION_VISUAL_STYLES}
        selectedId={MOTION_VISUAL_STYLES[0].id}
        onOpenChange={vi.fn()}
        onApply={onApply}
      />
    )
    await user.click(screen.getByRole('option', { name: 'No style' }))
    expect(onApply).not.toHaveBeenCalled()
    expect(document.querySelector('video')).toBeNull()
    await user.click(screen.getByRole('button', { name: 'Apply style' }))
    expect(onApply).toHaveBeenCalledWith(null)
  })

  it('discards a staged change when the picker closes and reopens', async () => {
    const user = userEvent.setup()
    const onApply = vi.fn()
    const props = {
      presets: MOTION_VISUAL_STYLES,
      selectedId: MOTION_VISUAL_STYLES[0].id,
      onOpenChange: vi.fn(),
      onApply,
    }
    const view = render(<MotionStyleDialog {...props} open />)
    await user.click(
      screen.getByRole('option', { name: MOTION_VISUAL_STYLES[1].nameKey })
    )
    view.rerender(<MotionStyleDialog {...props} open={false} />)
    view.rerender(<MotionStyleDialog {...props} open />)
    expect(
      screen.getByRole('option', { name: MOTION_VISUAL_STYLES[0].nameKey })
    ).toHaveAttribute('aria-selected', 'true')
    expect(
      screen.getByRole('option', { name: MOTION_VISUAL_STYLES[1].nameKey })
    ).toHaveAttribute('aria-selected', 'false')
    expect(onApply).not.toHaveBeenCalled()
  })

  it('keeps No style available when the catalogue is empty', async () => {
    const user = userEvent.setup()
    const onApply = vi.fn()
    render(
      <MotionStyleDialog
        open
        presets={[]}
        selectedId={null}
        onOpenChange={vi.fn()}
        onApply={onApply}
      />
    )
    expect(screen.getAllByRole('option')).toHaveLength(1)
    await user.click(screen.getByRole('button', { name: 'Apply style' }))
    expect(onApply).toHaveBeenCalledWith(null)
  })

  it('navigates by arrows, Home and End without applying, and tabs to Apply', async () => {
    const user = userEvent.setup()
    const onApply = vi.fn()
    render(
      <MotionStyleDialog
        open
        presets={MOTION_VISUAL_STYLES}
        selectedId={null}
        onOpenChange={vi.fn()}
        onApply={onApply}
      />
    )
    const options = screen.getAllByRole('option')
    await waitFor(() => expect(options[0]).toHaveFocus())
    expect(options.filter((option) => option.tabIndex === 0)).toHaveLength(1)
    await user.keyboard('{ArrowRight}')
    expect(options[1]).toHaveFocus()
    await user.keyboard('{ArrowDown}')
    expect(options[4]).toHaveFocus()
    await user.keyboard('{ArrowUp}{ArrowLeft}')
    expect(options[0]).toHaveFocus()
    await user.keyboard('{End}')
    expect(options.at(-1)).toHaveFocus()
    expect(options[0]).toHaveAttribute('aria-selected', 'true')
    await user.keyboard(' ')
    expect(options.at(-1)).toHaveAttribute('aria-selected', 'true')
    expect(onApply).not.toHaveBeenCalled()
    await user.tab()
    expect(screen.getByRole('button', { name: 'Apply style' })).toHaveFocus()
    act(() => options.at(-1)?.focus())
    await user.keyboard('{Home}')
    expect(options[0]).toHaveFocus()
  })
})

describe('MotionStyleDialog clip preview', () => {
  it('restores the preview source after strict-mode effect replay', async () => {
    const user = userEvent.setup()
    render(
      <StrictMode>
        <MotionStyleDialog
          open
          presets={presets}
          selectedId={null}
          onOpenChange={vi.fn()}
          onApply={vi.fn()}
        />
      </StrictMode>
    )
    await waitFor(() =>
      expect(screen.getByRole('option', { name: 'No style' })).toHaveFocus()
    )
    await user.hover(screen.getByRole('option', { name: 'Selfie Walk' }))
    const video = document.querySelector('video') as HTMLVideoElement
    expect(pause.mock.contexts).toContain(video)
    expect(video).toHaveAttribute('src', 'https://cdn/selfie.mp4')
    expect(
      play.mock.contexts.filter((element) => element === video)
    ).toHaveLength(2)
  })

  it('loads no video until a card is viewed, then starts silent inline playback', async () => {
    const user = userEvent.setup()
    renderDialog()
    await waitFor(() =>
      expect(screen.getByRole('option', { name: 'No style' })).toHaveFocus()
    )
    expect(document.querySelector('video')).toBeNull()
    const card = screen.getByRole('option', { name: 'Umbrella Transformation' })
    await user.hover(card)
    const video = card.querySelector('video')
    expect(video).toHaveAttribute('src', 'https://cdn/umbrella.mp4')
    expect(video).toHaveProperty('muted', true)
    expect(video).toHaveProperty('loop', true)
    expect(video).toHaveProperty('playsInline', true)
    expect(video).not.toHaveClass('on')
    fireEvent.playing(video as HTMLVideoElement)
    expect(video).toHaveClass('on')
    fireEvent.waiting(video as HTMLVideoElement)
    expect(video).not.toHaveClass('on')
  })

  it('preserves a replacement preset media and applies the original object', async () => {
    const user = userEvent.setup()
    const legacy = preset({ nameKey: 'Scene Recast' })
    const onApply = vi.fn()
    render(
      <MotionStyleDialog
        open
        presets={[legacy]}
        selectedId={null}
        onOpenChange={vi.fn()}
        onApply={onApply}
      />
    )
    const option = screen.getByRole('option', { name: 'Scene Recast' })
    expect(option.querySelector('img')).toHaveAttribute('src', legacy.thumbUrl)
    await user.click(option)
    expect(option.querySelector('video')).toHaveAttribute(
      'src',
      legacy.videoUrl
    )
    await user.click(screen.getByRole('button', { name: 'Apply style' }))
    expect(onApply).toHaveBeenCalledWith(legacy)
  })

  it('gives hover and keyboard focus priority over the selected preview, one at a time', async () => {
    const user = userEvent.setup()
    renderDialog('p1')
    await waitFor(() =>
      expect(
        screen.getByRole('option', { name: 'Umbrella Transformation' })
      ).toHaveFocus()
    )
    const selected = screen.getByRole('option', {
      name: 'Umbrella Transformation',
    })
    const other = screen.getByRole('option', { name: 'Selfie Walk' })
    const selectedVideo = selected.querySelector('video') as HTMLVideoElement
    await user.hover(other)
    expect(document.querySelectorAll('video[src]')).toHaveLength(1)
    expect(other.querySelector('video')).toHaveAttribute(
      'src',
      'https://cdn/selfie.mp4'
    )
    expect(selectedVideo).not.toHaveAttribute('src')
    expect(pause.mock.contexts).toContain(selectedVideo)
    await user.keyboard('{ArrowRight}{ArrowLeft}')
    expect(selected.querySelector('video')).toHaveAttribute(
      'src',
      'https://cdn/umbrella.mp4'
    )
    expect(other.querySelector('video')).toBeNull()
  })

  it('stops and releases a preview on pointer leave and dialog close', async () => {
    const user = userEvent.setup()
    const props = {
      presets,
      selectedId: null,
      onOpenChange: vi.fn(),
      onApply: vi.fn(),
    }
    const view = render(<MotionStyleDialog {...props} open />)
    await waitFor(() =>
      expect(screen.getByRole('option', { name: 'No style' })).toHaveFocus()
    )
    const card = screen.getByRole('option', { name: 'Umbrella Transformation' })
    await user.hover(card)
    const firstVideo = card.querySelector('video') as HTMLVideoElement
    await user.unhover(card)
    expect(pause.mock.contexts).toContain(firstVideo)
    expect(firstVideo).not.toHaveAttribute('src')
    expect(document.querySelector('video')).toBeNull()
    await user.click(card)
    const selectedVideo = card.querySelector('video') as HTMLVideoElement
    view.rerender(<MotionStyleDialog {...props} open={false} />)
    expect(pause.mock.contexts).toContain(selectedVideo)
    expect(selectedVideo).not.toHaveAttribute('src')
    expect(document.querySelector('video')).toBeNull()
  })

  it('retains the cover when playback is rejected or reports a media error', async () => {
    const user = userEvent.setup()
    play.mockRejectedValueOnce(new Error('autoplay unavailable'))
    renderDialog()
    await waitFor(() =>
      expect(screen.getByRole('option', { name: 'No style' })).toHaveFocus()
    )
    const first = screen.getByRole('option', {
      name: 'Umbrella Transformation',
    })
    await user.hover(first)
    await waitFor(() =>
      expect(first.querySelector('video')).not.toHaveAttribute('src')
    )
    expect(first.querySelector('img')).toHaveAttribute(
      'src',
      'https://cdn/umbrella.webp'
    )
    const other = screen.getByRole('option', { name: 'Selfie Walk' })
    await user.hover(other)
    const video = other.querySelector('video') as HTMLVideoElement
    fireEvent.playing(video)
    fireEvent.error(video)
    expect(video).not.toHaveClass('on')
    expect(video).not.toHaveAttribute('src')
    expect(other.querySelector('img')).not.toBeNull()
  })

  it('keeps a style selectable and applicable when its cover cannot load', async () => {
    const user = userEvent.setup()
    const onApply = vi.fn()
    const style = MOTION_VISUAL_STYLES[0]
    render(
      <MotionStyleDialog
        open
        presets={[style]}
        selectedId={null}
        onOpenChange={vi.fn()}
        onApply={onApply}
      />
    )
    const option = screen.getByRole('option', { name: style.nameKey })
    fireEvent.error(option.querySelector('img') as HTMLImageElement)
    expect(option.querySelector('img')).toBeNull()
    expect(option).toHaveTextContent(style.summaryKey)
    await user.click(option)
    expect(option).toHaveAttribute('aria-selected', 'true')
    await user.click(screen.getByRole('button', { name: 'Apply style' }))
    expect(onApply).toHaveBeenCalledWith(style)
  })

  it('previews a touch selection even when the browser keeps the previous keyboard focus', async () => {
    const onApply = vi.fn()
    render(
      <MotionStyleDialog
        open
        presets={presets}
        selectedId={null}
        onOpenChange={vi.fn()}
        onApply={onApply}
      />
    )
    const noStyle = screen.getByRole('option', { name: 'No style' })
    await waitFor(() => expect(noStyle).toHaveFocus())
    const option = screen.getByRole('option', { name: 'Selfie Walk' })
    fireEvent.pointerEnter(option, { pointerType: 'touch' })
    fireEvent.click(option)
    expect(option).toHaveAttribute('aria-selected', 'true')
    expect(option.querySelector('video')).toHaveAttribute(
      'src',
      presets[1].videoUrl
    )
    expect(onApply).not.toHaveBeenCalled()
  })
})
