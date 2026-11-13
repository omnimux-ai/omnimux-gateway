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
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  communityTrendPresets,
  motionExplorePresets,
} from '../../../lib/motion-explore'
import { MotionPresetDialog } from '../../motion-preset-dialog'

beforeEach(() => {
  vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
  vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(
    () => undefined
  )
  vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(
    () => undefined
  )
})

afterEach(() => vi.unstubAllGlobals())

describe('inspiration video preview', () => {
  it.each([
    ['motion preset', motionExplorePresets()[0]],
    ['community trend', communityTrendPresets()[0]],
  ] as const)(
    'opens a %s with its real title, playback controls and original source',
    (_, preset) => {
      render(
        <MotionPresetDialog
          preset={preset}
          onClose={vi.fn()}
          onApply={vi.fn()}
        />
      )
      const dialog = screen.getByRole('dialog', { name: preset.title })
      const video = within(dialog).getByLabelText(preset.title)
      expect(video).toHaveAttribute('src', preset.videoUrl)
      expect(video).toHaveAttribute('controls')
      expect(video).toHaveAttribute('loop')
      expect((video as HTMLVideoElement).muted).toBe(true)
    }
  )

  it('closes with Escape without applying a preset', async () => {
    const onClose = vi.fn()
    const onApply = vi.fn()
    render(
      <MotionPresetDialog
        preset={motionExplorePresets()[0]}
        onClose={onClose}
        onApply={onApply}
      />
    )
    await userEvent.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledOnce()
    expect(onApply).not.toHaveBeenCalled()
  })

  it('copies the complete community description rather than a visible excerpt', async () => {
    const user = userEvent.setup()
    const copy = vi
      .spyOn(navigator.clipboard, 'writeText')
      .mockResolvedValue(undefined)
    const preset = communityTrendPresets()[0]
    render(
      <MotionPresetDialog preset={preset} onClose={vi.fn()} onApply={vi.fn()} />
    )
    await user.click(screen.getByRole('button', { name: 'Copy' }))
    expect(copy).toHaveBeenCalledWith(preset.description)
  })

  it('shows real mode and duration without claiming a provider model', () => {
    render(
      <MotionPresetDialog
        preset={motionExplorePresets()[0]}
        onClose={vi.fn()}
        onApply={vi.fn()}
      />
    )
    expect(screen.getByText('Object swap')).toBeInTheDocument()
    expect(screen.getByText('14 seconds')).toBeInTheDocument()
    expect(screen.queryByText('Model')).not.toBeInTheDocument()
  })

  it('applies only the exact opened preset through the existing callback', async () => {
    const onApply = vi.fn()
    const preset = motionExplorePresets()[0]
    render(
      <MotionPresetDialog preset={preset} onClose={vi.fn()} onApply={onApply} />
    )
    await userEvent.click(
      screen.getByRole('button', { name: 'Use this motion' })
    )
    expect(onApply).toHaveBeenCalledExactlyOnceWith(preset)
  })

  it('keeps failure feedback separate from the still available apply action', () => {
    const onApply = vi.fn()
    render(
      <MotionPresetDialog
        preset={motionExplorePresets()[0]}
        onClose={vi.fn()}
        onApply={onApply}
      />
    )
    fireEvent.error(
      screen.getByLabelText(motionExplorePresets()[0].title, {
        selector: 'video',
      })
    )
    expect(screen.getByRole('status')).toHaveTextContent('Loading preview…')
    expect(
      screen.getByRole('button', { name: 'Use this motion' })
    ).toBeEnabled()
    expect(onApply).not.toHaveBeenCalled()
  })

  it('downloads public video bytes without credentials and never applies a preset', async () => {
    const fetchMedia = vi.fn().mockResolvedValue({
      ok: true,
      blob: async () => new Blob(['video'], { type: 'video/mp4' }),
    })
    vi.stubGlobal('fetch', fetchMedia)
    const createUrl = vi.fn().mockReturnValue('blob:preview-download')
    const revokeUrl = vi.fn()
    vi.stubGlobal(
      'URL',
      class extends URL {
        static createObjectURL = createUrl
        static revokeObjectURL = revokeUrl
      }
    )
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, 'click')
      .mockImplementation(() => undefined)
    const onApply = vi.fn()
    const preset = motionExplorePresets()[0]
    render(
      <MotionPresetDialog preset={preset} onClose={vi.fn()} onApply={onApply} />
    )
    await userEvent.click(screen.getByRole('button', { name: 'Download' }))
    await waitFor(() => expect(click).toHaveBeenCalledOnce())
    expect(fetchMedia).toHaveBeenCalledWith(
      preset.videoUrl,
      expect.objectContaining({
        credentials: 'omit',
        cache: 'no-store',
        signal: expect.any(AbortSignal),
      })
    )
    expect(revokeUrl).toHaveBeenCalledWith('blob:preview-download')
    expect(onApply).not.toHaveBeenCalled()
  })

  it('reports a failed download and allows another explicit attempt', async () => {
    const fetchMedia = vi
      .fn()
      .mockRejectedValue(new Error('fixture network failure'))
    vi.stubGlobal('fetch', fetchMedia)
    const errorToast = vi.spyOn(toast, 'error')
    render(
      <MotionPresetDialog
        preset={motionExplorePresets()[0]}
        onClose={vi.fn()}
        onApply={vi.fn()}
      />
    )
    await userEvent.click(screen.getByRole('button', { name: 'Download' }))
    await waitFor(() =>
      expect(errorToast).toHaveBeenCalledWith(
        'Could not download this video. Please try again.'
      )
    )
    expect(screen.queryByRole('alert')).toBeNull()
    expect(screen.getByRole('button', { name: 'Download' })).toBeEnabled()
    await userEvent.click(screen.getByRole('button', { name: 'Download' }))
    expect(fetchMedia).toHaveBeenCalledTimes(2)
  })

  it('aborts an unfinished download when the preview closes', async () => {
    const fetchMedia = vi
      .fn()
      .mockImplementation(() => new Promise(() => undefined))
    vi.stubGlobal('fetch', fetchMedia)
    const view = render(
      <MotionPresetDialog
        preset={motionExplorePresets()[0]}
        onClose={vi.fn()}
        onApply={vi.fn()}
      />
    )
    await userEvent.click(screen.getByRole('button', { name: 'Download' }))
    const signal = fetchMedia.mock.calls[0][1].signal as AbortSignal
    view.unmount()
    expect(signal.aborted).toBe(true)
  })

  it('does not expose a preview when no preset is selected', () => {
    render(
      <MotionPresetDialog preset={null} onClose={vi.fn()} onApply={vi.fn()} />
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('changes media and resets expanded description when another preset opens', async () => {
    const props = { onClose: vi.fn(), onApply: vi.fn() }
    const view = render(
      <MotionPresetDialog {...props} preset={communityTrendPresets()[0]} />
    )
    await userEvent.click(screen.getByRole('button', { name: 'See all' }))
    view.rerender(
      <MotionPresetDialog {...props} preset={motionExplorePresets()[0]} />
    )
    expect(
      screen.getByLabelText(motionExplorePresets()[0].title, {
        selector: 'video',
      })
    ).toHaveAttribute('src', motionExplorePresets()[0].videoUrl)
    expect(screen.getByRole('button', { name: 'See all' })).toHaveAttribute(
      'aria-expanded',
      'false'
    )
  })
})
