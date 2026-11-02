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
import { fireEvent, render, screen, within } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { TaskPreviewDialog } from '../components/task-preview-dialog'
import type { TaskRecord } from '../types'

const task: TaskRecord = {
  id: 1,
  task_id: 'one',
  status: 'SUCCESS',
  artifacts: [{ key: 'image', content_url: '/image.webp' }],
  properties: {
    input: JSON.stringify({
      tier: 'normal',
      selection: {},
      brief: 'A character in daylight',
    }),
  },
}
function setup(record = task, extra = {}) {
  const props = {
    task: record,
    taxonomy: null,
    onClose: vi.fn(),
    onRegenerate: vi.fn(),
    onDelete: vi.fn(),
    onGenerateMultiView: vi.fn(),
    ...extra,
  }
  return { props, ...render(<TaskPreviewDialog {...props} />) }
}
describe('history detail preview', () => {
  it('unknown character tier keeps the image visible but disables reuse without a hint', () => {
    setup({
      ...task,
      properties: { input: JSON.stringify({ tier: 'unknown', selection: {} }) },
    })
    expect(screen.getByRole('img', { name: 'Preview' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Regenerate' })).toBeDisabled()
    expect(
      screen.queryByText('This character type cannot be reused')
    ).not.toBeInTheDocument()
  })
  it('offers image-to-video after the primary regeneration action without submitting generation', () => {
    const onTurnToMotion = vi.fn()
    const { props } = setup(task, { onTurnToMotion })
    const action = screen.getByRole('button', { name: 'Turn to motion' })
    expect(
      action.compareDocumentPosition(
        screen.getByRole('button', { name: 'Regenerate' })
      ) & Node.DOCUMENT_POSITION_PRECEDING
    ).toBeTruthy()
    fireEvent.click(action)
    expect(onTurnToMotion).toHaveBeenCalledWith(task)
    expect(props.onRegenerate).not.toHaveBeenCalled()
  })
  it('blocks image-to-video when the reference capacity reason is supplied', () => {
    const onTurnToMotion = vi.fn()
    setup(task, {
      onTurnToMotion,
      turnToMotionDisabledReason: 'Up to 30 reference images',
    })
    const action = screen.getByRole('button', { name: 'Turn to motion' })
    expect(action).toBeDisabled()
    fireEvent.click(action)
    expect(onTurnToMotion).not.toHaveBeenCalled()
  })
  it('disables image-to-video after the displayed image fails', () => {
    setup(task, { onTurnToMotion: vi.fn() })
    fireEvent.error(screen.getByRole('img', { name: 'Preview' }))
    expect(
      screen.getByRole('button', { name: 'Turn to motion' })
    ).toBeDisabled()
  })
  it('does not offer image-to-video for a video or missing image', () => {
    const { rerender } = setup(
      { ...task, artifacts: [] },
      { onTurnToMotion: vi.fn() }
    )
    expect(
      screen.queryByRole('button', { name: 'Turn to motion' })
    ).not.toBeInTheDocument()
    rerender(
      <TaskPreviewDialog
        task={{ ...task, action: 'influencer_motion' }}
        taxonomy={null}
        onClose={vi.fn()}
        onRegenerate={vi.fn()}
        onDelete={vi.fn()}
        onGenerateMultiView={vi.fn()}
        onTurnToMotion={vi.fn()}
      />
    )
    expect(
      screen.queryByRole('button', { name: 'Turn to motion' })
    ).not.toBeInTheDocument()
  })
  it('shows stored description without making a generation request on opening', () => {
    const { props } = setup()
    expect(screen.getByText('A character in daylight')).toBeInTheDocument()
    expect(props.onRegenerate).not.toHaveBeenCalled()
    expect(props.onGenerateMultiView).not.toHaveBeenCalled()
  })
  it('keeps actions available after image failure', () => {
    const { props } = setup()
    fireEvent.error(screen.getByRole('img', { name: 'Preview' }))
    expect(screen.getByText('Loading preview…')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Regenerate' }))
    expect(props.onRegenerate).toHaveBeenCalledWith(task)
    expect(
      screen.queryByRole('link', { name: 'Download' })
    ).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    expect(props.onDelete).not.toHaveBeenCalled()
    fireEvent.click(
      within(screen.getByRole('alertdialog')).getByRole('button', {
        name: 'Delete',
      })
    )
    expect(props.onDelete).toHaveBeenCalledWith(task)
  })
  it('uses a video with controls rather than treating a motion artifact as an image', () => {
    const { container } = setup({
      ...task,
      action: 'influencer_motion',
      artifacts: [{ key: 'video', content_url: '/clip.mp4' }],
    })
    expect(container.ownerDocument.querySelector('video')).toHaveAttribute(
      'controls'
    )
    expect(
      screen.queryByRole('button', { name: 'Generate multi-view' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Download' })).toHaveAttribute(
      'href',
      '/clip.mp4'
    )
  })
  it('omits unreadable historical input instead of exposing raw metadata', () => {
    setup({ ...task, properties: { input: '{invalid' } })
    expect(screen.queryByText('Direction')).not.toBeInTheDocument()
    expect(screen.queryByText('{invalid')).not.toBeInTheDocument()
  })
  it('blocks removal while a derived generation still runs and explains why', () => {
    const { props } = setup(task, {
      deleteDisabledReason: 'Remove after the multi-view generation finishes',
    })
    const remove = screen.getByRole('button', { name: 'Delete' })
    expect(remove).toBeDisabled()
    expect(remove).toHaveAccessibleDescription(
      'Remove after the multi-view generation finishes'
    )
    fireEvent.click(remove)
    expect(props.onDelete).not.toHaveBeenCalled()
  })
  it('cancels removal without calling the parent callback', () => {
    const { props } = setup()
    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))
    const confirmation = screen.getByRole('alertdialog')
    expect(confirmation).toHaveTextContent('Source files will not be deleted.')
    fireEvent.click(
      within(confirmation).getByRole('button', { name: 'Cancel' })
    )
    expect(props.onDelete).not.toHaveBeenCalled()
  })
  it('disables regeneration when stored motion settings cannot be read', () => {
    const { props } = setup({ ...task, action: 'influencer_motion' })
    const regenerate = screen.getByRole('button', { name: 'Regenerate' })
    expect(regenerate).toBeDisabled()
    fireEvent.click(regenerate)
    expect(props.onRegenerate).not.toHaveBeenCalled()
  })
  it('reuses readable motion settings through the existing callback only', () => {
    const motion: TaskRecord = {
      ...task,
      action: 'influencer_motion',
      properties: {
        input: JSON.stringify({
          mode: 'motion',
          prompt: 'Walk slowly',
          subject_image_url: 'https://example.com/character.webp',
          video_url: 'https://example.com/reference.mp4',
          duration_sec: 7,
          size: '9:16',
          resolution: '720p',
        }),
      },
    }
    const { props } = setup(motion)
    fireEvent.click(screen.getByRole('button', { name: 'Regenerate' }))
    expect(props.onRegenerate).toHaveBeenCalledExactlyOnceWith(motion)
    expect(props.onGenerateMultiView).not.toHaveBeenCalled()
  })
  it('keeps the gateway download delivery contract', () => {
    setup({
      ...task,
      artifacts: [
        {
          content_url:
            'https://omnimux.ai/v1/tasks/one/artifacts/image/content',
        },
      ],
    })
    expect(screen.getByRole('link', { name: 'Download' })).toHaveAttribute(
      'href',
      '/v1/tasks/one/artifacts/image/content?delivery=direct&download=1'
    )
  })
  it('does not offer download for failed tasks or absent media', () => {
    setup({ ...task, status: 'FAILED' })
    expect(
      screen.queryByRole('link', { name: 'Download' })
    ).not.toBeInTheDocument()
  })
  it('exposes publication only when provided by the authorized parent', () => {
    setup(task, { onPublish: vi.fn() })
    expect(screen.getByRole('button', { name: 'Publish' })).toBeInTheDocument()
  })
})
