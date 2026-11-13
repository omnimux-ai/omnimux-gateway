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
import { act, fireEvent, render, screen } from '@testing-library/react'
import { StrictMode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { TaskCard } from '../components/task-card'
import { TaskPreviewDialog } from '../components/task-preview-dialog'
import type { TaskRecord } from '../types'

const image: TaskRecord = {
  id: 1,
  task_id: 'recover-image',
  status: 'SUCCESS',
  artifacts: [
    {
      key: 'image',
      content_url:
        '/v1/tasks/recover-image/artifacts/image/content?access=fixture',
    },
  ],
}
function card(task = image) {
  const onView = vi.fn()
  const view = render(<TaskCard task={task} child={null} onView={onView} />)
  return { ...view, onView }
}
describe('successful task preview recovery', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
    vi.spyOn(HTMLMediaElement.prototype, 'load').mockImplementation(() => {})
  })
  afterEach(() => {
    vi.useRealTimers()
    vi.restoreAllMocks()
  })

  it('retries a transient direct-image failure through the stable proxy without generating again', async () => {
    const view = card()
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    await act(() => vi.advanceTimersByTimeAsync(1000))
    const retried = view.container.querySelector('img') as HTMLImageElement
    expect(retried).toHaveAttribute(
      'src',
      '/v1/tasks/recover-image/artifacts/image/content?access=fixture'
    )
    fireEvent.load(retried)
    expect(screen.queryByText('Preview unavailable')).not.toBeInTheDocument()
  })

  it('stops after three attempts and offers a media-only manual reload', async () => {
    const view = card()
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    await act(() => vi.advanceTimersByTimeAsync(1000))
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    await act(() => vi.advanceTimersByTimeAsync(2000))
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    expect(
      screen.getByRole('button', { name: 'Reload preview' })
    ).toBeInTheDocument()
    await act(() => vi.advanceTimersByTimeAsync(60000))
    expect(view.container.querySelector('img')).toBeNull()
    fireEvent.click(screen.getByRole('button', { name: 'Reload preview' }))
    expect(view.container.querySelector('img')).toBeInTheDocument()
    expect(view.onView).not.toHaveBeenCalled()
  })

  it('keeps the cover and details/download access when video loading fails', () => {
    const view = card({
      ...image,
      action: 'influencer_motion',
      poster_url: '/cover.jpg',
      artifacts: [{ key: 'video', content_url: '/clip.mp4' }],
    })
    fireEvent.error(view.container.querySelector('video') as HTMLVideoElement)
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      '/cover.jpg'
    )
    expect(screen.getByRole('link', { name: 'Download' })).toBeInTheDocument()
    fireEvent.click(view.container.querySelector('.inf-card') as HTMLElement)
    expect(view.onView).toHaveBeenCalledOnce()
  })

  it('keeps a valid video visible if its cover cannot load', async () => {
    const view = card({
      ...image,
      action: 'influencer_motion',
      poster_url: '/cover.jpg',
      artifacts: [{ key: 'video', content_url: '/clip.mp4' }],
    })
    fireEvent.loadedData(
      view.container.querySelector('video') as HTMLVideoElement
    )
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    expect(view.container.querySelector('video')).not.toHaveClass('opacity-0')
    expect(screen.queryByText('Preview unavailable')).not.toBeInTheDocument()
  })

  it('keeps an available cover while a successful video address is still missing', () => {
    const view = card({
      ...image,
      action: 'influencer_motion',
      poster_url: '/cover.jpg',
      artifacts: [],
    })
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      '/cover.jpg'
    )
    expect(screen.getByText('Preview unavailable')).toBeInTheDocument()
  })

  it('retains video loading after StrictMode replays cleanup', () => {
    const task = {
      ...image,
      action: 'influencer_motion',
      artifacts: [{ key: 'video', content_url: '/clip.mp4' }],
    }
    const view = render(
      <StrictMode>
        <TaskCard task={task} child={null} onView={vi.fn()} />
      </StrictMode>
    )
    expect(view.container.querySelector('video')).toHaveAttribute(
      'src',
      '/clip.mp4'
    )
  })

  it('releases the old video source when recovery removes its attempt', () => {
    const view = card({
      ...image,
      action: 'influencer_motion',
      artifacts: [{ key: 'video', content_url: '/clip.mp4' }],
    })
    const old = view.container.querySelector('video') as HTMLVideoElement
    fireEvent.error(old)
    expect(old).not.toHaveAttribute('src')
    expect(HTMLMediaElement.prototype.load).toHaveBeenCalled()
  })

  it('does not hide the cover before the first video frame or when autoplay is refused', async () => {
    vi.mocked(HTMLMediaElement.prototype.play).mockRejectedValue(
      new DOMException('blocked', 'NotAllowedError')
    )
    const view = card({
      ...image,
      action: 'influencer_motion',
      poster_url: '/cover.jpg',
      artifacts: [{ key: 'video', content_url: '/clip.mp4' }],
    })
    fireEvent.mouseEnter(
      view.container.querySelector('.inf-card') as HTMLElement
    )
    expect(view.container.querySelector('img')).not.toHaveClass('opacity-0')
    fireEvent.loadedData(
      view.container.querySelector('video') as HTMLVideoElement
    )
    await act(async () => {})
    expect(view.container.querySelector('img')).not.toHaveClass('opacity-0')
    expect(screen.queryByText('Loading preview…')).not.toBeInTheDocument()
  })

  it('does not let a stale source failure poison a new source or a return to the old one', () => {
    const view = card()
    const old = view.container.querySelector('img') as HTMLImageElement
    fireEvent.error(old)
    const changed = {
      ...image,
      artifacts: [{ key: 'image', content_url: '/new.jpg' }],
    }
    view.rerender(<TaskCard task={changed} child={null} onView={view.onView} />)
    fireEvent.error(old)
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      '/new.jpg'
    )
    view.rerender(<TaskCard task={image} child={null} onView={view.onView} />)
    expect(view.container.querySelector('img')).toBeInTheDocument()
  })

  it('does not rewrite external media into a gateway proxy during retry', async () => {
    const view = card({
      ...image,
      artifacts: [
        {
          key: 'image',
          content_url: 'https://cdn.example/ordinary.jpg?signature=fixture',
        },
      ],
    })
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    await act(() => vi.advanceTimersByTimeAsync(1000))
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      'https://cdn.example/ordinary.jpg?signature=fixture'
    )
  })

  it('preserves slash-backslash external authority instead of creating a same-origin retry', async () => {
    const external =
      '/\\files.example/v1/tasks/t/artifacts/image/content?access=external&delivery=direct'
    const view = card({
      ...image,
      artifacts: [{ key: 'image', content_url: external }],
    })
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    await act(() => vi.advanceTimersByTimeAsync(1000))
    expect(view.container.querySelector('img')).toHaveAttribute('src', external)
  })

  it('contains malformed external authority as a local preview failure rather than crashing the page', async () => {
    const source = '/\\['
    const view = card({
      ...image,
      artifacts: [{ key: 'image', content_url: source }],
    })
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    await act(() => vi.advanceTimersByTimeAsync(1000))
    expect(view.container.querySelector('img')).toHaveAttribute('src', source)
    expect(screen.getByRole('link', { name: 'Download' })).toBeInTheDocument()
  })

  it('pauses scheduled recovery while hidden and resumes the remaining attempt when visible', async () => {
    const view = card()
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden')
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    await act(() => vi.advanceTimersByTimeAsync(10000))
    expect(view.container.querySelector('img')).toBeNull()
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    fireEvent(document, new Event('visibilitychange'))
    await act(() => vi.advanceTimersByTimeAsync(1000))
    expect(view.container.querySelector('img')).toBeInTheDocument()
  })

  it('cleans pending recovery when unmounted', async () => {
    const view = card()
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    view.unmount()
    expect(vi.getTimerCount()).toBe(0)
  })

  it('does not keep a successful artifact visible when the open task is confirmed failed', () => {
    render(
      <TaskPreviewDialog
        task={{
          ...image,
          status: 'FAILURE',
          fail_reason: '提交结果待核对，不会自动重发',
        }}
        taxonomy={null}
        onClose={vi.fn()}
        onRegenerate={vi.fn()}
        onDelete={vi.fn()}
        onGenerateMultiView={vi.fn()}
      />
    )
    expect(
      screen.queryByRole('img', { name: 'Preview' })
    ).not.toBeInTheDocument()
    // The gateway's Chinese review marker is shown as localized copy; the
    // original sentence is only kept behind the Details disclosure.
    expect(
      screen.getByText(
        'The submission outcome needs review. This take will not be sent again.'
      )
    ).toBeInTheDocument()
    // Confirmed reasons keep the original text one click away.
    expect(screen.getByRole('button', { name: 'Details' })).toBeInTheDocument()
  })

  it('uses the same bounded recovery in the open detail without regeneration', async () => {
    const regenerate = vi.fn()
    render(
      <TaskPreviewDialog
        task={image}
        taxonomy={null}
        onClose={vi.fn()}
        onRegenerate={regenerate}
        onDelete={vi.fn()}
        onGenerateMultiView={vi.fn()}
      />
    )
    fireEvent.error(screen.getByRole('img', { name: 'Preview' }))
    await act(() => vi.advanceTimersByTimeAsync(1000))
    const retry = screen.getByRole('img', { name: 'Preview' })
    expect(retry).toHaveAttribute(
      'src',
      '/v1/tasks/recover-image/artifacts/image/content?access=fixture'
    )
    fireEvent.load(retry)
    expect(regenerate).not.toHaveBeenCalled()
  })
})
