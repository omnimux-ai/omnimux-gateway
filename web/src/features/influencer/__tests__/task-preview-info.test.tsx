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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { TaskPreviewInfo } from '../components/task-preview-info'
import { taskPreviewInfo } from '../lib/task-preview-info'
import type { TaskRecord } from '../types'

const task: TaskRecord = { id: 1, task_id: 'info', status: 'SUCCESS' }

describe('stored task information', () => {
  it('shows only actual stored settings and safe reference materials', () => {
    render(
      <TaskPreviewInfo
        task={{
          ...task,
          action: 'influencer_motion',
          submit_time: 1700000000,
          properties: {
            input: JSON.stringify({
              resolution: '720p',
              size: '9:16',
              duration_sec: 7,
              subject_image_urls: ['https://example.com/character.webp'],
              video_url: 'https://example.com/reference.mp4',
              upstream_model: 'private-model',
              bitrate: 1000,
            }),
          },
        }}
      />
    )
    expect(screen.getByText('720p')).toBeInTheDocument()
    expect(screen.getByText('9:16')).toBeInTheDocument()
    expect(screen.getByText('7')).toBeInTheDocument()
    expect(
      screen
        .getByRole('button', { name: 'Reference image 1' })
        .querySelector('img')
    ).toHaveAttribute('src', 'https://example.com/character.webp')
    expect(
      screen
        .getByRole('button', { name: 'Reference video' })
        .querySelector('video')
    ).toHaveAttribute('src', 'https://example.com/reference.mp4')
    expect(screen.queryByRole('link')).not.toBeInTheDocument()
    expect(screen.queryByText('private-model')).not.toBeInTheDocument()
    expect(screen.queryByText('1000')).not.toBeInTheDocument()
  })
  it('reports the clicked image reference to the parent instead of opening another window', async () => {
    const user = userEvent.setup()
    const onOpenReference = vi.fn()
    render(
      <TaskPreviewInfo
        onOpenReference={onOpenReference}
        task={{
          ...task,
          properties: {
            input: JSON.stringify({
              subject_image_urls: [
                'https://example.com/first.webp',
                'https://example.com/second.webp',
              ],
            }),
          },
        }}
      />
    )
    await user.click(screen.getByRole('button', { name: 'Reference image 2' }))
    expect(onOpenReference).toHaveBeenCalledExactlyOnceWith(
      'https://example.com/second.webp',
      'image'
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
  it('reports a keyboard-selected video reference while its thumbnail stays paused', async () => {
    const user = userEvent.setup()
    const onOpenReference = vi.fn()
    render(
      <TaskPreviewInfo
        onOpenReference={onOpenReference}
        task={{
          ...task,
          properties: {
            input: JSON.stringify({
              video_url: 'https://example.com/reference.mp4',
            }),
          },
        }}
      />
    )
    const card = screen.getByRole('button', { name: 'Reference video' })
    expect(card.querySelector('video')).not.toHaveAttribute('autoplay')
    card.focus()
    await user.keyboard('{Enter}')
    expect(onOpenReference).toHaveBeenCalledExactlyOnceWith(
      'https://example.com/reference.mp4',
      'video'
    )
  })
  it('disables reference cards when no switching handler is supplied', () => {
    render(
      <TaskPreviewInfo
        task={{
          ...task,
          properties: {
            input: JSON.stringify({
              subject_image_urls: ['https://example.com/first.webp'],
            }),
          },
        }}
      />
    )
    expect(
      screen.getByRole('button', { name: 'Reference image 1' })
    ).toBeDisabled()
  })
  it('does not infer dimensions or duration when historical input is unreadable', () => {
    render(
      <TaskPreviewInfo task={{ ...task, properties: { input: '{broken' } }} />
    )
    expect(screen.getByText('Character')).toBeInTheDocument()
    expect(screen.getByText('Success')).toBeInTheDocument()
    expect(screen.queryByText('Resolution')).not.toBeInTheDocument()
    expect(screen.queryByText('Duration (seconds)')).not.toBeInTheDocument()
    expect(screen.queryByText('{broken')).not.toBeInTheDocument()
  })
  it('deduplicates image references and refuses executable URLs and invalid duration', () => {
    expect(
      taskPreviewInfo({
        ...task,
        properties: {
          input: JSON.stringify({
            image_url: 'https://example.com/a.webp',
            subject_image_url: 'https://example.com/a.webp',
            subject_image_urls: [
              'javascript:alert(1)',
              'https://example.com/b.webp',
            ],
            video_url: 'data:video/mp4;base64,abc',
            duration_sec: -1,
          }),
        },
      })
    ).toEqual({
      images: ['https://example.com/a.webp', 'https://example.com/b.webp'],
      resolution: undefined,
      size: undefined,
      durationSec: undefined,
      video: undefined,
    })
  })
  it('handles non-object metadata without displaying fabricated fields', () => {
    expect(taskPreviewInfo({ ...task, properties: { input: '[]' } })).toEqual({
      images: [],
    })
    expect(taskPreviewInfo(task)).toEqual({ images: [] })
  })
})
