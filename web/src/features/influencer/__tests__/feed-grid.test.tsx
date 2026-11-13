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
import { afterEach, describe, expect, it, vi } from 'vitest'

import { FeedGrid } from '../components/feed-grid'
import type { TaskRecord } from '../types'

function rec(over: Partial<TaskRecord> = {}): TaskRecord {
  return { id: 1, task_id: 'task_1', status: 'SUCCESS', ...over }
}

function done(over: Partial<TaskRecord> = {}): TaskRecord {
  return rec({
    artifacts: [
      { key: 'image', content_url: '/api/task/task_1/artifacts/image' },
    ],
    ...over,
  })
}

type GridProps = React.ComponentProps<typeof FeedGrid>

function grid(over: Partial<GridProps> = {}) {
  const props: GridProps = {
    tasks: [],
    pending: null,
    isInitialLoading: false,
    hasMore: false,
    isFetchingMore: false,
    loadMoreFailed: false,
    onLoadMore: vi.fn(),
    onView: vi.fn(),
    ...over,
  }
  return { props, ...render(<FeedGrid {...props} />) }
}

/** Captures the observer callback so a test can drive intersection itself. */
class ObserverStub {
  static latest: ObserverStub | null = null
  readonly root = null
  readonly rootMargin = ''
  readonly thresholds: number[] = []
  private readonly callback: IntersectionObserverCallback
  constructor(callback: IntersectionObserverCallback) {
    this.callback = callback
    ObserverStub.latest = this
  }
  observe(): void {}
  unobserve(): void {}
  disconnect(): void {}
  takeRecords(): IntersectionObserverEntry[] {
    return []
  }
  intersect(): void {
    this.callback(
      [{ isIntersecting: true } as IntersectionObserverEntry],
      this as unknown as IntersectionObserver
    )
  }
}

afterEach(() => {
  vi.unstubAllGlobals()
  ObserverStub.latest = null
})

describe('FeedGrid loading states', () => {
  it('shows the shimmer skeleton while the first page is loading with nothing to paint', () => {
    const { container } = grid({ isInitialLoading: true })
    expect(container.querySelector('[style*="aspect-ratio"]')).toHaveStyle({
      aspectRatio: '1.75578',
    })

    expect(
      screen.getByRole('status', { name: 'Loading history…' })
    ).toBeInTheDocument()
  })

  it('keeps cached records on screen instead of the skeleton while refreshing', () => {
    grid({ tasks: [done()], isInitialLoading: true })

    expect(
      screen.queryByRole('status', { name: 'Loading history…' })
    ).toBeNull()
    expect(screen.getByRole('button')).toBeInTheDocument()
  })

  it('shows the empty state once loading finished with no records', () => {
    grid()

    expect(screen.getByText('Nothing here yet')).toBeInTheDocument()
  })
})

describe('FeedGrid paging', () => {
  it('requests the next page when the trailing sentinel scrolls into view', () => {
    vi.stubGlobal('IntersectionObserver', ObserverStub)
    const { props } = grid({ tasks: [done()], hasMore: true })

    expect(ObserverStub.latest).not.toBeNull()
    ObserverStub.latest?.intersect()

    expect(props.onLoadMore).toHaveBeenCalledTimes(1)
  })

  it('offers a manual button when the environment has no IntersectionObserver', () => {
    vi.stubGlobal('IntersectionObserver', undefined)
    const { props } = grid({ tasks: [done()], hasMore: true })

    fireEvent.click(screen.getByRole('button', { name: 'Load more' }))

    expect(props.onLoadMore).toHaveBeenCalledTimes(1)
  })

  it('marks the end of the list and stops watching once every page is loaded', () => {
    vi.stubGlobal('IntersectionObserver', ObserverStub)
    grid({ tasks: [done()], hasMore: false })

    expect(ObserverStub.latest).toBeNull()
    expect(screen.getByText('No more records')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Load more' })).toBeNull()
  })

  it('shows a shimmer row while the next page is in flight', () => {
    grid({ tasks: [done()], hasMore: true, isFetchingMore: true })

    expect(
      screen.getByRole('status', { name: 'Loading more…' })
    ).toBeInTheDocument()
  })

  it('offers a retry when the next page failed to load', () => {
    vi.stubGlobal('IntersectionObserver', undefined)
    const { props } = grid({
      tasks: [done()],
      hasMore: true,
      loadMoreFailed: true,
    })

    fireEvent.click(
      screen.getByRole('button', { name: 'Failed to load history' })
    )

    expect(props.onLoadMore).toHaveBeenCalledTimes(1)
  })
})

describe('FeedGrid cards', () => {
  it('marks an uncertain submission for review with its bounded wait and no duplicate retry', () => {
    const { container } = grid({
      tasks: [
        rec({
          status: 'DISPATCHING',
          fail_reason: 'Submission outcome needs review; it will not be resent',
        }),
      ],
    })
    expect(screen.getByText('Needs review')).toBeInTheDocument()
    expect(
      screen.getByText(
        'Confirming the result. Unconfirmed tasks fail automatically within about 60 minutes and are never resent.'
      )
    ).toBeInTheDocument()
    expect(
      screen.queryByText(
        'Submission outcome needs review; it will not be resent'
      )
    ).toBeNull()
    expect(screen.queryByText('Submitting…')).toBeNull()
    expect(screen.queryByRole('button', { name: 'Retry' })).toBeNull()
    expect(container.querySelector('.inf-shimmer')).toBeNull()
    expect(container.querySelector('.inf-progress')).toBeNull()
  })
  it.each([
    ['WAITING', 'Waiting for a free slot'],
    ['CLAIMED', 'Submitting…'],
    ['DISPATCHING', 'Submitting…'],
  ])(
    'shows local queue status %s without claiming generation has started',
    (status, label) => {
      grid({ tasks: [rec({ status })] })
      expect(screen.getByText(label)).toBeInTheDocument()
      expect(screen.queryByText('Generating…')).toBeNull()
    }
  )
  it.each(['QUEUED', 'SUBMITTED', 'NOT_START'])(
    'keeps provider-side status %s labelled as queued',
    (status) => {
      grid({ tasks: [rec({ status })] })
      expect(screen.getByText('Queued')).toBeInTheDocument()
      expect(screen.queryByText('Waiting for a free slot')).toBeNull()
    }
  )
  it('uses the fixed grid frame even when a legacy timeline preference exists', () => {
    localStorage.setItem('influencer:view-mode', 'timeline')
    try {
      const { container } = grid({ tasks: [done()] })
      expect(
        container.querySelector('.inf-card')?.firstElementChild
      ).toHaveStyle({
        aspectRatio: '1.75578',
      })
    } finally {
      localStorage.removeItem('influencer:view-mode')
    }
  })
  it.each(['QUEUED', 'IN_PROGRESS', 'FAILURE'])(
    'uses the default landscape frame without size metadata for status %s',
    (status) => {
      const { container } = grid({ tasks: [rec({ status })] })
      expect(
        container.querySelector('.inf-card')?.firstElementChild
      ).toHaveStyle({
        aspectRatio: '1.75578',
      })
    }
  )

  it('shows both sides of a landscape character sheet in grid without hover cropping', () => {
    const { container } = grid({
      tasks: [
        done({ properties: { input: JSON.stringify({ size: '16:9' }) } }),
      ],
    })

    const image = container.querySelector('img')
    expect(image?.closest('.inf-history-media')).toHaveStyle({
      aspectRatio: '1.75578',
    })
    expect(image).toHaveClass('object-contain')
    expect(image?.className).not.toContain('scale-')
  })

  it.each(['QUEUED', 'IN_PROGRESS', 'FAILURE'])(
    'keeps the landscape sheet frame through status %s',
    (status) => {
      const { container } = grid({
        tasks: [
          rec({
            status,
            properties: { input: JSON.stringify({ size: '16:9' }) },
          }),
        ],
      })

      const frame = container.querySelector('.inf-card')?.firstElementChild
      expect(frame).toHaveStyle({ aspectRatio: '1.75578' })
    }
  )

  it.each(['9:16', '1024x1536'])(
    'contains legacy sheet size %s inside the fixed landscape grid frame',
    (size) => {
      const { container } = grid({
        tasks: [done({ properties: { input: JSON.stringify({ size }) } })],
      })
      const image = container.querySelector('img')
      expect(image?.closest('.inf-history-media')).toHaveStyle({
        aspectRatio: '1.75578',
      })
      expect(image).toHaveClass('object-contain')
    }
  )

  it.each([undefined, 'invalid'])(
    'uses the landscape grid default when sheet size is %s',
    (size) => {
      const { container } = grid({
        tasks: [done({ properties: { input: JSON.stringify({ size }) } })],
      })
      const frame = container.querySelector('.inf-history-media')
      expect(frame).toHaveStyle({ aspectRatio: '1.75578' })
    }
  )

  it('contains the whole landscape sheet in the grid when provider dimensions differ from metadata', () => {
    const { container } = grid({
      tasks: [
        done({ properties: { input: JSON.stringify({ size: '16:9' }) } }),
      ],
    })
    const image = container.querySelector('img') as HTMLImageElement
    Object.defineProperties(image, {
      naturalWidth: { value: 1672 },
      naturalHeight: { value: 940 },
    })
    fireEvent.load(image)

    expect(image.closest('.inf-history-media')).toHaveStyle({
      aspectRatio: '1.75578',
    })
    expect(image).toHaveClass('loaded', 'object-contain')
    expect(image).not.toHaveClass('object-cover')
    expect(image.className).not.toContain('scale-')
  })

  it('keeps the derived board shortcut inside the uniform landscape grid frame', () => {
    const parent = done({
      task_id: 'landscape-parent',
      properties: { input: JSON.stringify({ size: '16:9' }) },
    })
    const child = done({
      task_id: 'board-child',
      properties: { input: JSON.stringify({ kind: 'multiview' }) },
    })
    const onOpen = vi.fn()
    const { container } = grid({
      tasks: [
        parent,
        done({
          task_id: 'old-portrait',
          properties: { input: JSON.stringify({ size: '9:16' }) },
        }),
      ],
      childByParent: new Map([['landscape-parent', child]]),
      onOpenMultiView: onOpen,
    })
    const card = container.querySelector('.inf-card') as HTMLElement
    expect(card).toHaveClass('self-start')
    const frame = card.firstElementChild as HTMLElement
    const badge = screen.getByRole('button', { name: 'View multi-view' })
    expect(frame.contains(badge)).toBe(true)
    expect(card.contains(badge)).toBe(true)
    fireEvent.click(badge)
    expect(onOpen).toHaveBeenCalledWith(parent, child)
  })

  it('retains the landscape frame when a successful sheet preview cannot load', () => {
    const { container } = grid({
      tasks: [
        done({ properties: { input: JSON.stringify({ size: '16:9' }) } }),
      ],
    })
    fireEvent.error(container.querySelector('img') as HTMLImageElement)
    expect(screen.getByText('Loading preview…')).toBeInTheDocument()
    expect(container.querySelector('.inf-card')?.firstElementChild).toHaveStyle(
      { aspectRatio: '1.75578' }
    )
  })

  it('shows a completed video without using its mp4 address as an image poster', () => {
    const { container } = grid({
      tasks: [
        done({
          action: 'influencer_motion',
          artifacts: [
            { key: 'video', type: 'video', content_url: '/ready-video.mp4' },
          ],
        }),
      ],
    })
    expect(container.querySelector('img')).toBeNull()
    expect(container.querySelector('video')).not.toHaveAttribute('poster')
    expect(container.querySelector('video')).not.toHaveClass('opacity-0')
  })

  it('contains motion videos inside the same landscape grid frame', () => {
    const { container } = grid({
      tasks: [
        done({
          action: 'influencer_motion',
          properties: {
            input: JSON.stringify({ size: '16:9', kind: 'motion' }),
          },
          artifacts: [
            { key: 'video', content_url: '/api/task/video/artifacts/video' },
          ],
        }),
      ],
    })
    const video = container.querySelector('video')
    expect(video).toHaveClass('object-contain')
    const frame = container.querySelector('.inf-card')
      ?.firstElementChild as HTMLElement
    expect(frame).toHaveStyle({ aspectRatio: '1.75578' })
  })

  it('contains independent multi-view images inside the uniform landscape grid frame', () => {
    const { container } = grid({
      tasks: [
        done({
          properties: {
            input: JSON.stringify({ kind: 'multiview', size: '16:9' }),
          },
        }),
      ],
    })
    const image = container.querySelector('img')
    expect(image?.closest('.inf-history-media')).toHaveStyle({
      aspectRatio: '1.75578',
    })
    expect(image).toHaveClass('object-contain')
    expect(image?.className).not.toContain('scale-')
  })

  it('keeps the in-flight task first and drops the duplicate from the list', () => {
    grid({ tasks: [done({ status: 'IN_PROGRESS' })], pending: done() })

    expect(screen.getAllByRole('button')).toHaveLength(1)
  })

  it('retries a failed preview when the same task receives a new media address', () => {
    const initial = done()
    const { container, props, rerender } = grid({ tasks: [initial] })
    fireEvent.error(container.querySelector('img') as HTMLImageElement)
    expect(screen.getByText('Loading preview…')).toBeInTheDocument()

    rerender(
      <FeedGrid
        {...props}
        tasks={[
          done({
            artifacts: [{ key: 'image', content_url: '/recovered-image' }],
          }),
        ]}
      />
    )

    expect(container.querySelector('img')).toHaveAttribute(
      'src',
      '/recovered-image'
    )
    expect(screen.queryByText('Preview unavailable')).not.toBeInTheDocument()
  })

  it('keeps a confirmed success preview when history enrichment temporarily omits its media', () => {
    const { container } = grid({
      pending: done(),
      tasks: [done({ artifacts: [] })],
    })
    expect(container.querySelector('img')).toHaveAttribute(
      'src',
      '/api/task/task_1/artifacts/image'
    )
    expect(screen.queryByText('Preview unavailable')).not.toBeInTheDocument()
  })

  it('does not preserve a previous preview over a confirmed failure', () => {
    grid({
      pending: done(),
      tasks: [done({ status: 'FAILURE', artifacts: [] })],
    })
    expect(screen.getByText('Generation failed')).toBeInTheDocument()
  })

  it('falls back to a labelled placeholder when the preview fails to load', () => {
    const { container } = grid({ tasks: [done()] })

    const image = container.querySelector('img')
    expect(image).not.toBeNull()
    fireEvent.error(image as HTMLImageElement)

    expect(screen.getByText('Loading preview…')).toBeInTheDocument()
  })

  it('reserves the fixed grid frame so a portrait load cannot shift the list', () => {
    const { container } = grid({
      tasks: [
        done({ properties: { input: JSON.stringify({ size: '9:16' }) } }),
      ],
    })

    const box = container.querySelector('[style*="aspect-ratio"]')
    expect(box).not.toBeNull()
    expect(box?.getAttribute('style')).toContain('1.75578')
  })

  it('contains pixel-sized legacy images inside the fixed grid frame', () => {
    const { container } = grid({
      tasks: [
        done({ properties: { input: JSON.stringify({ size: '1024x1536' }) } }),
      ],
    })

    const box = container.querySelector('[style*="aspect-ratio"]')
    expect(box?.getAttribute('style')).toContain('1.75578')
  })

  it('renders a motion video card with play icon and triggers onView on click', () => {
    const onView = vi.fn()
    const videoTask: TaskRecord = {
      id: 2,
      task_id: 'task_motion_1',
      status: 'SUCCESS',
      properties: { input: JSON.stringify({ kind: 'motion' }) },
      artifacts: [
        {
          key: 'video',
          content_url: '/v1/tasks/task_motion_1/artifacts/video/content',
        },
      ],
    }
    const { container } = grid({ tasks: [videoTask], onView })

    const video = container.querySelector('video')
    expect(video).not.toBeNull()
    expect(video?.getAttribute('src')).toBe(
      '/v1/tasks/task_motion_1/artifacts/video/content?delivery=direct'
    )

    const cardButton = container.querySelector('[role="button"]')
    expect(cardButton).not.toBeNull()
    if (cardButton) {
      fireEvent.click(cardButton)
    }
    expect(onView).toHaveBeenCalledWith(videoTask)
  })
})

describe('history card thumbnails', () => {
  it('uses and falls back from the multi-view derivative while opening its original record', () => {
    const task = done()
    const child = done({
      task_id: 'board',
      artifacts: [{ content_url: '/board.png', thumbnail_url: '/board.webp' }],
    })
    const onOpenMultiView = vi.fn()
    grid({
      tasks: [task],
      childByParent: new Map([['task_1', child]]),
      onOpenMultiView,
    })
    const badge = screen.getByRole('button', { name: 'View multi-view' })
    expect(badge.querySelector('img')).toHaveAttribute('src', '/board.webp')
    fireEvent.error(badge.querySelector('img') as HTMLImageElement)
    expect(badge.querySelector('img')).toHaveAttribute('src', '/board.png')
    fireEvent.click(badge)
    expect(onOpenMultiView).toHaveBeenCalledWith(task, child)
  })

  it('falls back once while downloads and preview keep the original, then resets for a new derivative', () => {
    const task = done({
      artifacts: [
        {
          key: 'image',
          content_url: '/original.png',
          thumbnail_url: '/thumb.webp',
        },
        {
          key: 'image-2',
          content_url: '/second.png',
          thumbnail_url: '/second-thumb.webp',
        },
      ],
    })
    const view = grid({ tasks: [task] })
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      '/thumb.webp'
    )
    expect(
      screen.getByRole('link', { name: 'Download' }).getAttribute('href')
    ).toContain('/original.png')
    fireEvent.click(view.container.querySelector('.inf-card') as HTMLElement)
    expect(view.props.onView).toHaveBeenCalledWith(task)
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      '/original.png'
    )
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    expect(view.container.querySelector('img')).toBeNull()
    view.rerender(
      <FeedGrid
        {...view.props}
        tasks={[
          {
            ...task,
            artifacts: [
              {
                key: 'image',
                content_url: '/original.png',
                thumbnail_url: '/new.webp',
              },
            ],
          },
        ]}
      />
    )
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      '/new.webp'
    )
  })
})
