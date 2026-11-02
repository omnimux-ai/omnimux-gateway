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
import { render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { describe, expect, it, vi } from 'vitest'

import {
  HistoryPreviewDialog,
  type ReferencePreviewActions,
  type TaskPreviewActions,
} from '../components/history-preview-dialog'
import type { PreviewEntry } from '../lib/preview-reference'
import type { UploadRecord } from '../lib/upload-library'
import type { TaskRecord } from '../types'

const motion: TaskRecord = {
  id: 1,
  task_id: 'motion',
  action: 'influencer_motion',
  status: 'SUCCESS',
  artifacts: [{ key: 'video', content_url: '/clip.mp4' }],
  properties: {
    input: JSON.stringify({
      subject_image_urls: ['https://files.example.com/photos/face.png'],
      video_url: 'https://files.example.com/videos/dance.mp4',
    }),
  },
}
const imageUpload: UploadRecord = {
  kind: 'image',
  url: 'https://files.example.com/photos/face.png',
  fileName: 'face.png',
  uploadedAt: 1,
  expiresAt: Date.UTC(2026, 9, 18),
}
const videoUpload: UploadRecord = {
  kind: 'video',
  url: 'https://files.example.com/videos/dance.mp4',
  fileName: 'dance.mp4',
  uploadedAt: 1,
  expiresAt: Date.UTC(2026, 9, 18),
  metadata: {
    rawDurationSec: 7.2,
    durationSec: 7,
    width: 1080,
    height: 1920,
    aspectRatio: '9:16',
  },
}

function taskActions(): TaskPreviewActions {
  return {
    onRegenerate: vi.fn(),
    onDelete: vi.fn(),
    onGenerateMultiView: vi.fn(),
  }
}

function Harness(props: {
  initial: PreviewEntry[]
  references: Record<string, PreviewEntry>
  referenceActions?: ReferencePreviewActions
  onClose?: () => void
}) {
  const [stack, setStack] = useState(props.initial)
  if (!stack.length) return <p>closed</p>
  return (
    <HistoryPreviewDialog
      stack={stack}
      taxonomy={null}
      onClose={() => {
        setStack([])
        props.onClose?.()
      }}
      onBack={() => setStack((current) => current.slice(0, -1))}
      onOpenReference={(url) => {
        const entry = props.references[url]
        if (entry) setStack((current) => [...current, entry])
      }}
      taskActions={taskActions}
      referenceActions={props.referenceActions}
    />
  )
}

function referenceActions(
  extra: Partial<ReferencePreviewActions> = {}
): ReferencePreviewActions {
  return { onUseImage: vi.fn(), onUseVideo: vi.fn(), ...extra }
}

describe('history preview reference stack', () => {
  it.each(['edit', 'masked_edit'])(
    'does not offer multi-view generation for a completed %s image',
    (image_mode) => {
      const task: TaskRecord = {
        id: 2,
        task_id: 'edited-image',
        status: 'SUCCESS',
        action: 'influencer_sheet',
        properties: {
          input: JSON.stringify({
            tier: 'normal',
            selection: {},
            image_mode,
            image_url: 'https://files.example.com/source.png',
            edit_instruction: 'Change the background',
          }),
        },
        artifacts: [
          { key: 'image', mime_type: 'image/png', content_url: '/edited.png' },
        ],
      }
      render(<Harness initial={[{ kind: 'task', task }]} references={{}} />)
      expect(
        screen.queryByRole('button', { name: 'Generate multi-view' })
      ).not.toBeInTheDocument()
      expect(
        screen.getByRole('button', { name: 'Regenerate' })
      ).toBeInTheDocument()
    }
  )

  it('switches the same window to an uploaded image and returns one level with Back', async () => {
    const user = userEvent.setup()
    render(
      <Harness
        initial={[{ kind: 'task', task: motion }]}
        references={{
          [imageUpload.url]: { kind: 'upload', record: imageUpload },
        }}
        referenceActions={referenceActions()}
      />
    )
    expect(
      screen.queryByRole('button', { name: 'Back' })
    ).not.toBeInTheDocument()
    const dialog = screen.getByRole('dialog', { name: 'Preview' })
    await user.click(screen.getByRole('button', { name: 'Reference image 1' }))
    expect(screen.getAllByRole('dialog')).toHaveLength(1)
    expect(within(dialog).getByText('face.png')).toBeInTheDocument()
    expect(within(dialog).getByText('Uploaded')).toBeInTheDocument()
    expect(within(dialog).getByText('PNG')).toBeInTheDocument()
    expect(
      within(dialog).queryByRole('button', { name: 'Regenerate' })
    ).not.toBeInTheDocument()
    await user.click(within(dialog).getByRole('button', { name: 'Back' }))
    expect(
      within(dialog).getByRole('button', { name: 'Regenerate' })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Back' })
    ).not.toBeInTheDocument()
  })
  it('closing from a nested reference closes the whole window', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(
      <Harness
        initial={[{ kind: 'task', task: motion }]}
        references={{
          [imageUpload.url]: { kind: 'upload', record: imageUpload },
        }}
        referenceActions={referenceActions()}
        onClose={onClose}
      />
    )
    await user.click(screen.getByRole('button', { name: 'Reference image 1' }))
    await user.keyboard('{Escape}')
    expect(onClose).toHaveBeenCalledOnce()
    expect(screen.getByText('closed')).toBeInTheDocument()
  })
  it('fills the video form from an uploaded image without other actions', async () => {
    const user = userEvent.setup()
    const actions = referenceActions()
    render(
      <Harness
        initial={[{ kind: 'upload', record: imageUpload }]}
        references={{}}
        referenceActions={actions}
      />
    )
    const buttons = within(screen.getByRole('dialog')).getAllByRole('button')
    expect(buttons.map((b) => b.textContent)).not.toContain('Download')
    await user.click(screen.getByRole('button', { name: 'Turn to motion' }))
    expect(actions.onUseImage).toHaveBeenCalledExactlyOnceWith(
      imageUpload.url,
      'face.png'
    )
  })
  it('blocks the image action with the supplied capacity reason', () => {
    const actions = referenceActions({
      useImageDisabledReason: () => 'Up to 30 reference images',
    })
    render(
      <Harness
        initial={[{ kind: 'upload', record: imageUpload }]}
        references={{}}
        referenceActions={actions}
      />
    )
    expect(
      screen.getByRole('button', { name: 'Turn to motion' })
    ).toBeDisabled()
  })
  it('shows checked video facts and selects the uploaded clip as the motion reference', async () => {
    const user = userEvent.setup()
    const actions = referenceActions()
    render(
      <Harness
        initial={[{ kind: 'upload', record: videoUpload }]}
        references={{}}
        referenceActions={actions}
      />
    )
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText('MP4')).toBeInTheDocument()
    expect(within(dialog).getByText('1080×1920')).toBeInTheDocument()
    expect(within(dialog).getByText('9:16')).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Use this motion' }))
    expect(actions.onUseVideo).toHaveBeenCalledExactlyOnceWith({
      kind: 'upload',
      record: videoUpload,
    })
  })
  it('labels an untraceable reference as unknown without inventing upload facts', () => {
    render(
      <Harness
        initial={[
          {
            kind: 'unknown',
            url: 'https://other.example.com/x.webp',
            media: 'image',
          },
        ]}
        references={{}}
        referenceActions={referenceActions()}
      />
    )
    const dialog = screen.getByRole('dialog')
    expect(within(dialog).getByText('Unknown')).toBeInTheDocument()
    expect(within(dialog).getByText('WEBP')).toBeInTheDocument()
    expect(within(dialog).queryByText('File name')).not.toBeInTheDocument()
    expect(within(dialog).queryByText('Expires at')).not.toBeInTheDocument()
  })
  it('offers no motion action for an untraceable video, which has no checked metadata', () => {
    render(
      <Harness
        initial={[
          {
            kind: 'unknown',
            url: 'https://other.example.com/x.mp4',
            media: 'video',
          },
        ]}
        references={{}}
        referenceActions={referenceActions()}
      />
    )
    expect(
      screen.queryByRole('button', { name: 'Use this motion' })
    ).not.toBeInTheDocument()
  })
  it('shows a read-only generated reference with download only', () => {
    render(
      <Harness
        initial={[{ kind: 'task', task: motion, readOnly: true }]}
        references={{}}
      />
    )
    expect(screen.getByRole('link', { name: 'Download' })).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Regenerate' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Delete' })
    ).not.toBeInTheDocument()
  })
})
