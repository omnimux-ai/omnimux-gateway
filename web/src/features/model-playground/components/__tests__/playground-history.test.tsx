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
import { createInstance } from 'i18next'
import { I18nextProvider } from 'react-i18next'
import { expect, it, vi } from 'vitest'

import en from '@/i18n/locales/en.json'
import zh from '@/i18n/locales/zh.json'

import { PlaygroundHistory } from '../playground-history'
import type { GenerationTask, TaskStatus } from '../../types'

function mockTask(overrides: Partial<GenerationTask> = {}): GenerationTask {
  return {
    taskId: 'task_abc123456789',
    modelId: 'nano-banana-pro',
    status: 'succeeded' as TaskStatus,
    progress: 100,
    params: {
      prompt: 'a minimalist red mug',
      aspect_ratio: '1:1',
      resolution: '1K',
    },
    estimatedUsd: 0.0429,
    settledUsd: 0.0429,
    createdAt: 1789222513,
    finishedAt: 1789222537,
    media: [{ url: 'https://example.com/mug.jpg', kind: 'image' }],
    ...overrides,
  }
}

it('renders table headers and row content matching reference specifications', () => {
  render(<PlaygroundHistory tasks={[mockTask()]} loading={false} />)

  // Headers
  expect(screen.getByText('Content')).toBeTruthy()
  expect(screen.getByText('Params')).toBeTruthy()
  expect(screen.getByText('Status')).toBeTruthy()
  expect(screen.getByText('Completed')).toBeTruthy()
  expect(screen.getByText('Cost')).toBeTruthy()

  // Row content
  expect(screen.getByText('a minimalist red mug')).toBeTruthy()
  expect(screen.getByText('task_abc123456789')).toBeTruthy()
  expect(screen.getByText('1:1')).toBeTruthy()
  expect(screen.getByText('1K')).toBeTruthy()
  expect(screen.getByText('succeeded')).toBeTruthy()

  // The settled charge keeps USD precision and shows its 10:1 credit equivalent.
  expect(screen.getByText('$0.0429')).toBeTruthy()
  expect(screen.getByText('0.43 credits')).toBeVisible()

  // Footer
  expect(screen.getByText('1 local records')).toBeTruthy()
  const viewAll = screen.getByText('View all records')
  expect(viewAll.getAttribute('href')).toBe('/panel/tasks')
})

it.each(['running', 'queued'] as const)(
  'localizes pending settlement and credit units for %s tasks when language changes',
  async (status) => {
    const i18n = createInstance()
    await i18n.init({
      lng: 'en',
      fallbackLng: 'en',
      resources: { en, zhCN: zh },
    })
    render(
      <I18nextProvider i18n={i18n}>
        <PlaygroundHistory
          tasks={[
            mockTask({ taskId: 'pending', status, settledUsd: null }),
            mockTask({ taskId: 'settled' }),
          ]}
          loading={false}
        />
      </I18nextProvider>
    )

    expect(screen.getByText('Pending settlement')).toBeVisible()
    expect(screen.getByText('0.43 credits')).toBeVisible()
    expect(screen.getAllByText('$0.0429')).toHaveLength(1)

    await act(() => i18n.changeLanguage('zhCN'))

    expect(screen.getByText('等待结算')).toBeVisible()
    expect(screen.queryByText('Pending settlement')).not.toBeInTheDocument()
    expect(screen.getByText('0.43 积分')).toBeVisible()
    expect(screen.queryByText('0.43 credits')).not.toBeInTheDocument()
    expect(screen.getAllByText('$0.0429')).toHaveLength(1)
  }
)

it('allows dismissing a task row locally via the close button', () => {
  const task1 = mockTask({ taskId: 'task_1', params: { prompt: 'mug 1' } })
  const task2 = mockTask({ taskId: 'task_2', params: { prompt: 'mug 2' } })
  render(<PlaygroundHistory tasks={[task1, task2]} loading={false} />)

  expect(screen.getByText('mug 1')).toBeTruthy()
  expect(screen.getByText('mug 2')).toBeTruthy()
  expect(screen.getByText('2 local records')).toBeTruthy()

  const dismissButtons = screen.getAllByRole('button', { name: 'Dismiss' })
  fireEvent.click(dismissButtons[0])

  expect(screen.queryByText('mug 1')).toBeNull()
  expect(screen.getByText('mug 2')).toBeTruthy()
  expect(screen.getByText('1 local records')).toBeTruthy()
})

it('renders empty prompt without invasive EmptyState when loading with zero tasks (D-08)', () => {
  render(<PlaygroundHistory tasks={[]} loading />)

  // Should NOT render EmptyState
  expect(screen.queryByText('Loading')).toBeNull()

  // Should render empty notice
  const emptyNotice = document.querySelector("[data-parity='playground.history.empty']")
  expect(emptyNotice).not.toBeNull()
  expect(screen.getByText('Generation history will appear here')).toBeTruthy()
})

it('renders empty prompt when not loading and tasks list is empty', () => {
  render(<PlaygroundHistory tasks={[]} loading={false} />)

  const emptyNotice = document.querySelector("[data-parity='playground.history.empty']")
  expect(emptyNotice).not.toBeNull()
  expect(screen.getByText('Generation history will appear here')).toBeTruthy()
})

it('calls onSelectTask when a row is clicked and ignores dismissal click', () => {
  const onSelect = vi.fn()
  const task1 = mockTask({ taskId: 'task_1', params: { prompt: 'mug 1' } })
  render(
    <PlaygroundHistory
      tasks={[task1]}
      loading={false}
      onSelectTask={onSelect}
    />
  )

  const promptCell = screen.getByText('mug 1')
  fireEvent.click(promptCell)
  expect(onSelect).toHaveBeenCalledWith(task1)

  onSelect.mockClear()
  const dismissButton = screen.getByRole('button', { name: 'Dismiss' })
  fireEvent.click(dismissButton)
  expect(onSelect).not.toHaveBeenCalled()
})

it('highlights the selected task row matching selectedTaskId', () => {
  const task1 = mockTask({ taskId: 'task_1', params: { prompt: 'mug 1' } })
  const task2 = mockTask({ taskId: 'task_2', params: { prompt: 'mug 2' } })
  render(
    <PlaygroundHistory
      tasks={[task1, task2]}
      loading={false}
      selectedTaskId='task_2'
    />
  )

  const row1 = document.querySelector("tr[data-task-id='task_1']")
  const row2 = document.querySelector("tr[data-task-id='task_2']")

  expect(row1?.getAttribute('data-selected')).toBe('false')
  expect(row2?.getAttribute('data-selected')).toBe('true')
})

it('renders video thumbnail with media fragment time offset and preload', () => {
  const videoTask = mockTask({
    taskId: 'task_video',
    media: [{ url: 'https://cdn.example/preview.mp4', kind: 'video' }],
  })
  render(<PlaygroundHistory tasks={[videoTask]} loading={false} />)

  const video = document.querySelector('video')
  expect(video).not.toBeNull()
  expect(video?.getAttribute('src')).toBe('https://cdn.example/preview.mp4#t=0.001')
  expect(video?.getAttribute('preload')).toBe('metadata')
  expect(video?.muted).toBe(true)
  expect(video?.hasAttribute('playsinline')).toBe(true)
})

it('renders image thumbnail with lazy loading, async decoding, dimensions and handles load error gracefully', () => {
  const imageTask = mockTask({
    taskId: 'task_image',
    media: [{ url: 'https://cdn.example/photo.png', kind: 'image' }],
  })
  render(<PlaygroundHistory tasks={[imageTask]} loading={false} />)

  const img = document.querySelector('img')
  expect(img).not.toBeNull()
  expect(img?.getAttribute('src')).toBe('https://cdn.example/photo.png')
  expect(img?.getAttribute('loading')).toBe('lazy')
  expect(img?.getAttribute('decoding')).toBe('async')
  expect(img?.getAttribute('width')).toBe('40')
  expect(img?.getAttribute('height')).toBe('40')

  // 触发图片错误事件，断言优雅 fallback 到 AI 占位图标，杜绝破图
  if (!img) throw new Error('Expected an image thumbnail before simulating failure')
  fireEvent.error(img)
  expect(document.querySelector('img')).toBeNull()
  expect(screen.getByText('AI')).toBeTruthy()
})

it('enforces scroll container max height for 10 records and sticky header', () => {
  const tasks = Array.from({ length: 15 }, (_, i) =>
    mockTask({
      taskId: `task_${i + 1}`,
      params: { prompt: `generation prompt ${i + 1}` },
    })
  )

  render(<PlaygroundHistory tasks={tasks} loading={false} />)

  const scrollArea = document.querySelector("[data-parity='playground.history.scroll-area']")
  expect(scrollArea).not.toBeNull()
  expect(scrollArea?.className).toContain('overflow-auto')
  expect(scrollArea?.className).toContain('max-h-[680px]')

  const thead = document.querySelector('thead')
  expect(thead).not.toBeNull()
  expect(thead?.className).toContain('sticky')
  expect(thead?.className).toContain('top-0')

  expect(screen.getByText('15 local records')).toBeTruthy()
})
