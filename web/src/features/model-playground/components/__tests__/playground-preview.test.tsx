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
import { expect, it, vi } from 'vitest'

import { PlaygroundPreview } from '../playground-preview'
import type { GenerationTask, TaskStatus } from '../../types'

const RESULT_URL = 'https://omnimux.ai/v1/images/abc/content'

function task(overrides: Partial<GenerationTask>): GenerationTask {
  return {
    taskId: 'task_test',
    modelId: 'seedance-2-5',
    status: 'running' as TaskStatus,
    progress: 9,
    params: {},
    estimatedUsd: null,
    settledUsd: null,
    createdAt: 0,
    media: [],
    ...overrides,
  }
}

function renderPreview(taskOrNull: GenerationTask | null, view: 'preview' | 'json' = 'preview') {
  render(
    <PlaygroundPreview
      task={taskOrNull}
      params={{}}
      view={view}
      onViewChange={vi.fn()}
    />
  )
}

it('shows the percentage and the plain processing label while a task is running', () => {
  renderPreview(task({ status: 'running' as TaskStatus, progress: 9 }))

  const stage = document.querySelector("[data-parity='playground.preview.progress']")
  expect(stage).not.toBeNull()
  expect(screen.getByText('9%')).toBeTruthy()
  expect(screen.getByText('processing')).toBeTruthy()
})

it('keeps waiting instead of showing a finished stage once the task settles without media', () => {
  // A settled task with no media has nothing to show; it must not render the
  // ring-and-percentage stage, which would claim work is still in flight.
  renderPreview(task({ status: 'failed' as TaskStatus, progress: 100, failReason: 'boom' }))

  expect(document.querySelector("[data-parity='playground.preview.progress']")).toBeNull()
  expect(screen.getByText('boom')).toBeTruthy()
})

it('offers expand and download on the finished media, both pointing at the result', () => {
  renderPreview(
    task({
      status: 'succeeded' as TaskStatus,
      progress: 100,
      media: [{ url: RESULT_URL, kind: 'image' }],
    })
  )

  const expand = document.querySelector<HTMLAnchorElement>(
    "[data-parity='playground.preview.expand']"
  )
  const download = document.querySelector<HTMLAnchorElement>(
    "[data-parity='playground.preview.download']"
  )

  expect(expand?.getAttribute('href')).toBe(RESULT_URL)
  expect(expand?.getAttribute('aria-label')).toBe('View full size')
  expect(download?.getAttribute('href')).toBe(RESULT_URL)
  expect(download?.hasAttribute('download')).toBe(true)
})

it('plays video media with autoplay and playsInline by default', () => {
  renderPreview(
    task({
      status: 'succeeded' as TaskStatus,
      progress: 100,
      media: [{ url: 'https://cdn.example/video.mp4', kind: 'video' }],
    })
  )

  const video = document.querySelector('video')
  expect(video).not.toBeNull()
  expect(video?.getAttribute('src')).toBe('https://cdn.example/video.mp4')
  expect(video?.hasAttribute('autoplay')).toBe(true)
  expect(video?.hasAttribute('playsinline')).toBe(true)
})

it('renders an explicit no-media notice when settled with no media and no error', () => {
  renderPreview(task({ status: 'succeeded' as TaskStatus, progress: 100, media: [] }))

  expect(document.querySelector("[data-parity='playground.preview.progress']")).toBeNull()
  expect(document.querySelector("[data-parity='playground.preview.no-media']")).not.toBeNull()
  expect(screen.getByText('No media produced for this task')).toBeTruthy()
})

it('keeps the empty state when no task has run', () => {
  renderPreview(null)

  expect(screen.getByText('No Task Running')).toBeTruthy()
})
