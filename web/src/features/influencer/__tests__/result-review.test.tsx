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
import { render, screen, cleanup } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { MultiViewDialog } from '../components/multi-view-dialog'
import { TaskCard } from '../components/task-card'
import { multiViewState } from '../lib/multiview'
import { hasUnverifiedResult } from '../lib/task-status'
import type { TaskRecord } from '../types'

const newMessage = 'Checking the original result. Do not submit again.'
const oldMessage =
  'Confirming the result. Unconfirmed tasks fail automatically within about 60 minutes and are never resent.'
function task(status: string, result_unverified?: boolean): TaskRecord {
  return {
    id: 1,
    task_id: 'same-original',
    status,
    result_unverified,
    action: 'influencer_sheet',
    progress: '30%',
  }
}
const card = (record: TaskRecord) => (
  <TaskCard task={record} child={null} onView={vi.fn()} />
)
const board = (record: TaskRecord) => (
  <MultiViewDialog
    open
    regenerating={false}
    task={record}
    onOpenChange={vi.fn()}
    onRegenerate={vi.fn()}
    onDelete={vi.fn()}
  />
)
afterEach(cleanup)

describe('original result reconciliation', () => {
  it.each(['IN_PROGRESS', 'DISPATCHING'])(
    'shows %s plus the readonly marker as review without a deadline or retry',
    (status) => {
      render(card(task(status, true)))
      expect(screen.getByText('Needs review')).toBeInTheDocument()
      expect(screen.getByText(newMessage)).toBeInTheDocument()
      expect(screen.queryByText(oldMessage)).not.toBeInTheDocument()
      expect(screen.queryByText('Generating…')).not.toBeInTheDocument()
      expect(
        screen.queryByRole('button', { name: 'Retry' })
      ).not.toBeInTheDocument()
      expect(
        screen.queryByRole('link', { name: 'Download' })
      ).not.toBeInTheDocument()
    }
  )
  it('preserves the bounded legacy dispatch message when the new marker is absent', () => {
    render(
      card({
        ...task('DISPATCHING'),
        fail_reason: 'Submission needs review; do not resubmit.',
      })
    )
    expect(screen.getByText(oldMessage)).toBeInTheDocument()
    expect(screen.queryByText(newMessage)).not.toBeInTheDocument()
  })
  it('keeps confirmed original generation in the normal generating state after the marker is cleared', () => {
    const view = render(card(task('IN_PROGRESS', true)))
    view.rerender(card(task('IN_PROGRESS', false)))
    expect(screen.getByText('Generating…')).toBeInTheDocument()
    expect(screen.queryByText('Needs review')).not.toBeInTheDocument()
  })
  it('replaces review with the same original finished image when a late success arrives', () => {
    const view = render(card(task('IN_PROGRESS', true)))
    view.rerender(
      card({
        ...task('SUCCESS', false),
        artifacts: [
          {
            key: 'image',
            mime_type: 'image/png',
            content_url: '/original-result.png',
          },
        ],
      })
    )
    expect(screen.queryByText('Needs review')).not.toBeInTheDocument()
    expect(screen.queryByText(newMessage)).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Download' })).toHaveAttribute(
      'href',
      '/original-result.png'
    )
  })
  it.each(['IN_PROGRESS', 'DISPATCHING'])(
    'uses the same %s reconciliation state in a multiview and keeps re-generation disabled',
    (status) => {
      const record = task(status, true)
      expect(multiViewState(record)).toBe('review')
      render(board(record))
      expect(screen.getByText('Needs review')).toBeInTheDocument()
      expect(screen.getByText(newMessage)).toBeInTheDocument()
      expect(screen.queryByText(oldMessage)).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Regenerate' })).toBeDisabled()
      expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled()
    }
  )
  it('restores the same multiview preview and actions after the original late success', () => {
    const view = render(board(task('IN_PROGRESS', true)))
    view.rerender(
      board({
        ...task('SUCCESS', false),
        artifacts: [{ content_url: '/original-board.png' }],
      })
    )
    expect(screen.queryByText('Needs review')).not.toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Multi-view' })).toHaveAttribute(
      'src',
      '/original-board.png'
    )
    expect(screen.getByRole('button', { name: 'Regenerate' })).toBeEnabled()
  })
  it('preserves the bounded legacy multiview message without the new marker', () => {
    render(
      board({ ...task('DISPATCHING'), fail_reason: 'legacy expiry policy' })
    )
    expect(screen.getByText(oldMessage)).toBeInTheDocument()
    expect(screen.queryByText(newMessage)).not.toBeInTheDocument()
  })
  it.each(['SUCCESS', 'SUCCEEDED', 'FAILURE', 'FAILED'])(
    'lets confirmed %s override an older reconciliation marker',
    (status) => {
      expect(hasUnverifiedResult(task(status, true))).toBe(false)
      expect(multiViewState(task(status, true))).not.toBe('review')
    }
  )
})
