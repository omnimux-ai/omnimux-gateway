import { fireEvent, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { TaskCard } from '../components/task-card'
import type { TaskRecord } from '../types'

const image: TaskRecord = {
  id: 1,
  task_id: 'history-image',
  status: 'SUCCESS',
  artifacts: [{ key: 'image', content_url: '/fixture/image.webp' }],
}

function setup(task = image, disabled = false) {
  const onView = vi.fn()
  const onTurnToMotion = vi.fn()
  const result = render(
    <TaskCard
      task={task}
      child={null}
      onView={onView}
      onTurnToMotion={onTurnToMotion}
      turnToMotionDisabledReason={() =>
        disabled ? 'Reference limit reached' : undefined
      }
    />
  )
  return { ...result, onView, onTurnToMotion }
}

describe('history card activation', () => {
  it.each(['{Enter}', ' '])(
    'opens the finished card once with %s',
    async (key) => {
      const result = setup()
      const card = result.container.querySelector('.inf-card') as HTMLElement
      card.focus()
      await userEvent.keyboard(key)
      expect(result.onView).toHaveBeenCalledExactlyOnceWith(image)
    }
  )

  it('download does not open the parent preview', () => {
    const result = setup()
    const download = screen.getByRole('link', { name: 'Download' })
    download.addEventListener('click', (event) => event.preventDefault())
    fireEvent.click(download)
    expect(result.onView).not.toHaveBeenCalled()
    expect(download).toHaveAttribute('download')
  })

  it('motion action calls only its existing preparation callback', async () => {
    const result = setup()
    await userEvent.click(
      screen.getByRole('button', { name: 'Turn to motion' })
    )
    expect(result.onTurnToMotion).toHaveBeenCalledExactlyOnceWith(image)
    expect(result.onView).not.toHaveBeenCalled()
  })

  it('disabled motion action does not call preparation', async () => {
    const result = setup(image, true)
    const button = screen.getByRole('button', { name: 'Turn to motion' })
    expect(button).toBeDisabled()
    await userEvent.click(button)
    expect(result.onTurnToMotion).not.toHaveBeenCalled()
  })

  it('queued cards are not keyboard preview targets', () => {
    const result = setup({ ...image, status: 'QUEUED', artifacts: [] })
    const card = result.container.querySelector('.inf-card')
    expect(card).not.toHaveAttribute('role', 'button')
    expect(card).not.toHaveAttribute('tabindex')
  })

  it('failed cards share a unified friendly idle state and keep details in the hover layer without red styling or a retry action', () => {
    const failedTask: TaskRecord = {
      id: 9,
      task_id: 'failed-video',
      status: 'FAILURE',
      fail_reason: 'video_generation_failed: 内容安全审核拦截',
    }
    const result = setup(failedTask)
    const failCard = result.container.querySelector('.inf-fail-card')
    const idleLayer = result.container.querySelector('.inf-fail-idle')
    const detailLayer = result.container.querySelector('.inf-fail-detail')

    expect(failCard).not.toBeNull()
    expect(idleLayer).not.toHaveTextContent(/.+/)
    expect(idleLayer?.querySelector('svg')).not.toBeNull()
    expect(detailLayer).toHaveTextContent('Why this take paused')
    // The stored reason is upstream prose, so the primary copy is the localized
    // fallback and the original text lives behind the Details disclosure.
    expect(detailLayer).toHaveTextContent(
      'Generation failed. Please try again.'
    )
    const details = within(detailLayer as HTMLElement).getByRole('button', {
      name: 'Details',
    })
    expect(details).toHaveAttribute('aria-expanded', 'false')
    fireEvent.click(details)
    expect(details).toHaveAttribute('aria-expanded', 'true')
    expect(detailLayer).toHaveTextContent(
      'video_generation_failed: 内容安全审核拦截'
    )
    fireEvent.click(details)
    expect(details).toHaveAttribute('aria-expanded', 'false')
    expect(detailLayer).not.toHaveTextContent(
      'video_generation_failed: 内容安全审核拦截'
    )
    expect(result.container.querySelector('[class*="inf-danger"]')).toBeNull()
    expect(
      screen.queryByRole('button', { name: 'Retry' })
    ).not.toBeInTheDocument()
  })
})

const motion: TaskRecord = {
  ...image,
  action: 'influencer_motion',
  poster_url: '/fixture/poster.webp',
  artifacts: [{ key: 'video', content_url: '/fixture/motion.mp4' }],
}

describe('history video preview lifecycle', () => {
  beforeEach(() => {
    vi.spyOn(HTMLMediaElement.prototype, 'play').mockResolvedValue(undefined)
    vi.spyOn(HTMLMediaElement.prototype, 'pause').mockImplementation(() => {})
  })
  afterEach(() => vi.restoreAllMocks())

  it('desktop hover plays silently and leaving resets the video', () => {
    const result = setup(motion)
    const card = result.container.querySelector('.inf-card') as HTMLElement
    const video = result.container.querySelector('video') as HTMLVideoElement
    const play = vi.mocked(HTMLMediaElement.prototype.play)
    const pause = vi.mocked(HTMLMediaElement.prototype.pause)
    play.mockClear()
    pause.mockClear()
    fireEvent.mouseEnter(card)
    expect(play).toHaveBeenCalledOnce()
    expect(video.muted).toBe(true)
    expect(video).toHaveAttribute('loop')
    video.currentTime = 3
    fireEvent.mouseLeave(card)
    expect(pause).toHaveBeenCalledOnce()
    expect(video.currentTime).toBe(0)
  })

  it('desktop keyboard focus starts preview and blur restores the poster', () => {
    const result = setup(motion)
    const card = result.container.querySelector('.inf-card') as HTMLElement
    const play = vi.mocked(HTMLMediaElement.prototype.play)
    play.mockClear()
    fireEvent.focus(card)
    expect(play).toHaveBeenCalledOnce()
    fireEvent.blur(card)
    const poster = result.container.querySelector('img')
    expect(poster).not.toHaveClass('opacity-0')
  })

  it('touch hover and focus do not automatically play the video', () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: true,
    } as MediaQueryList)
    const result = setup(motion)
    const card = result.container.querySelector('.inf-card') as HTMLElement
    const play = vi.mocked(HTMLMediaElement.prototype.play)
    play.mockClear()
    fireEvent.mouseEnter(card)
    fireEvent.focus(card)
    expect(play).not.toHaveBeenCalled()
  })
})
