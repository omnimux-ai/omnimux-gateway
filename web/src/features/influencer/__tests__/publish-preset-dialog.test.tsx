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

import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { toast } from 'sonner'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { PublishPresetDialog } from '../components/publish-preset-dialog'
import type { TaskRecord } from '../types'

const BRIEF = '街头银发造型，冷调高反差，夜景霓虹反光'

/** A finished character sheet: the record an administrator publishes. */
function sheetTask(over: Partial<TaskRecord> = {}): TaskRecord {
  return {
    id: 1,
    task_id: 'sheet_1',
    status: 'SUCCESS',
    artifacts: [
      { key: 'image', content_url: '/api/task/sheet_1/artifacts/image' },
    ],
    properties: {
      input: JSON.stringify({
        tier: 'freak',
        selection: { hair: ['hair_bowl'] },
        brief: BRIEF,
      }),
    },
    ...over,
  }
}

/** A derived multi-view board, attached to the sheet it came from. */
function boardTask(): TaskRecord {
  return {
    id: 2,
    task_id: 'board_1',
    status: 'SUCCESS',
    artifacts: [
      { key: 'image', content_url: '/api/task/board_1/artifacts/image' },
    ],
    properties: {
      input: JSON.stringify({
        kind: 'multiview',
        parent_task_id: 'sheet_1',
        selection: { hair: ['hair_bowl'] },
      }),
    },
  }
}

type Props = React.ComponentProps<typeof PublishPresetDialog>

function dialog(over: Partial<Props> = {}) {
  const props: Props = {
    open: true,
    onOpenChange: vi.fn(),
    task: sheetTask(),
    child: null,
    onSubmit: vi.fn().mockResolvedValue(undefined),
    submitting: false,
    ...over,
  }
  return { props, ...render(<PublishPresetDialog {...props} />) }
}

afterEach(() => {
  vi.useRealTimers()
})

describe('PublishPresetDialog naming', () => {
  it('suggests the task brief as the name', () => {
    dialog()

    expect(screen.getByLabelText('Name')).toHaveValue(BRIEF)
  })

  it('shortens a long brief so the suggestion stays card-sized', () => {
    const long = 'a'.repeat(80)
    dialog({
      task: sheetTask({
        properties: {
          input: JSON.stringify({
            tier: 'freak',
            selection: { hair: ['hair_bowl'] },
            brief: long,
          }),
        },
      }),
    })

    expect(screen.getByLabelText('Name')).toHaveValue('a'.repeat(40))
  })

  it('falls back to a dated name when the task carries no brief', () => {
    vi.useFakeTimers({ toFake: ['Date'] })
    vi.setSystemTime(new Date(2026, 9, 5, 12, 0, 0))

    dialog({
      task: sheetTask({
        properties: {
          input: JSON.stringify({ tier: 'freak', selection: { hair: ['hair_bowl'] } }),
        },
      }),
    })

    expect(screen.getByLabelText('Name')).toHaveValue('AI Creator 2026-10-05')
  })
})

describe('PublishPresetDialog submission', () => {
  it('disables the confirm button while the name is empty', () => {
    dialog()

    fireEvent.change(screen.getByLabelText('Name'), { target: { value: '   ' } })

    expect(screen.getByRole('button', { name: 'Publish' })).toBeDisabled()
  })

  it('submits the name the user typed', async () => {
    const { props } = dialog()

    fireEvent.change(screen.getByLabelText('Name'), {
      target: { value: 'Neon night' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Publish' }))

    await waitFor(() => {
      expect(props.onSubmit).toHaveBeenCalledWith({ name: 'Neon night' })
    })
  })

  it('reports a rejected submit through the top toast without inline copy', async () => {
    const errorToast = vi.spyOn(toast, 'error')
    const onSubmit = vi.fn().mockRejectedValue(new Error('gateway refused'))
    dialog({ onSubmit })

    fireEvent.click(screen.getByRole('button', { name: 'Publish' }))

    await waitFor(() => expect(errorToast).toHaveBeenCalledWith('gateway refused'))
    expect(screen.queryByRole('alert')).toBeNull()
  })
})

describe('PublishPresetDialog artworks', () => {
  it('captions the character render on its own', () => {
    dialog()

    expect(screen.getByText('Character render')).toBeInTheDocument()
    expect(screen.queryByText('Multi-view board')).toBeNull()
  })

  it('adds the multi-view board when the character has one', () => {
    dialog({ child: boardTask() })

    expect(screen.getByText('Multi-view board')).toBeInTheDocument()
  })
})
