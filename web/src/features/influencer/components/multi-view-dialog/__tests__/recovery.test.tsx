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

import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { TaskRecord } from '../../../types'
import { MultiViewDialog } from '../../multi-view-dialog'

const ready: TaskRecord = {
  id: 1,
  task_id: 'board1',
  status: 'SUCCESS',
  artifacts: [{ content_url: 'https://files.example/board.png' }],
}
const callbacks = () => ({
  onOpenChange: vi.fn(),
  onRegenerate: vi.fn(),
  onDelete: vi.fn(),
})
afterEach(cleanup)
describe('multi-view preview recovery', () => {
  it('replaces a broken image with an empty preview and restores it for another task', () => {
    const props = { open: true, regenerating: false, ...callbacks() }
    const view = render(<MultiViewDialog {...props} task={ready} />)
    fireEvent.error(screen.getByRole('img', { name: 'Multi-view' }))
    expect(
      screen.queryByRole('img', { name: 'Multi-view' })
    ).not.toBeInTheDocument()
    expect(screen.getByText('No preview available')).toBeInTheDocument()
    view.rerender(
      <MultiViewDialog
        {...props}
        task={{ ...ready, id: 2, task_id: 'board2' }}
      />
    )
    expect(screen.getByRole('img', { name: 'Multi-view' })).toBeInTheDocument()
  })
  it('renders a fallback for successful tasks with no image and keeps regeneration available', async () => {
    const props = callbacks()
    render(
      <MultiViewDialog
        open
        regenerating={false}
        {...props}
        task={{ ...ready, artifacts: undefined }}
      />
    )
    expect(screen.getByText('No preview available')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Regenerate' }))
    expect(props.onRegenerate).toHaveBeenCalledOnce()
  })
  it('preserves disabled reasons and generating delete restrictions', () => {
    render(
      <MultiViewDialog
        open
        regenerating={false}
        disabledReason='Source is unavailable'
        {...callbacks()}
        task={{ ...ready, status: 'IN_PROGRESS', progress: '35%' }}
      />
    )
    expect(screen.getByRole('button', { name: 'Regenerate' })).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Regenerate' })
    ).toHaveAccessibleDescription('Source is unavailable')
    expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled()
    expect(screen.getByText('Generating…')).toBeInTheDocument()
  })
  it('explains a reviewed board without exposing the internal reason', () => {
    const view = render(
      <MultiViewDialog
        open
        regenerating={false}
        {...callbacks()}
        task={{
          ...ready,
          artifacts: undefined,
          status: 'DISPATCHING',
          fail_reason: '提交结果待核对（阶段：submit；代码：submission_failed）',
        }}
      />
    )
    expect(screen.getByText('Needs review')).toBeInTheDocument()
    expect(
      screen.getByText(
        'Confirming the result. Unconfirmed tasks fail automatically within about 60 minutes and are never resent.'
      )
    ).toBeInTheDocument()
    expect(screen.queryByText(/submission_failed/)).toBeNull()
    expect(view.baseElement.querySelector('.inf-progress')).toBeNull()
  })
  it.each([
    ['WAITING', '', 'Waiting for a free slot'],
    ['QUEUED', '', 'Queued'],
    ['CLAIMED', '', 'Submitting…'],
    ['DISPATCHING', '', 'Submitting…'],
    ['DISPATCHING', 'Submission needs review', 'Needs review'],
    ['IN_PROGRESS', '', 'Generating…'],
  ])(
    'keeps %s locked even when the caller is not regenerating',
    (status, fail_reason, label) => {
      render(
        <MultiViewDialog
          open
          regenerating={false}
          {...callbacks()}
          task={{ ...ready, artifacts: undefined, status, fail_reason }}
        />
      )
      expect(screen.getByText(label)).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Regenerate' })).toBeDisabled()
      expect(screen.getByRole('button', { name: 'Delete' })).toBeDisabled()
    }
  )

  it('preserves delete and download on completed boards', async () => {
    const props = callbacks()
    render(
      <MultiViewDialog open regenerating={false} {...props} task={ready} />
    )
    await userEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(props.onDelete).toHaveBeenCalledOnce()
    expect(screen.getByRole('button', { name: 'Download' })).toHaveAttribute(
      'download'
    )
  })
})
