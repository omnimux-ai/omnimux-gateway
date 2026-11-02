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
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { VideoPickerPanel } from '../components/video-picker-panel'
import type { TaskRecord } from '../types'

vi.mock('sonner', () => ({ toast: { error: vi.fn() } }))
beforeEach(() => vi.mocked(toast.error).mockClear())

const video: TaskRecord = {
  id: 1,
  task_id: 'dance',
  status: 'SUCCESS',
  action: 'influencer_motion',
  properties: { input: JSON.stringify({ selection: {}, brief: 'Dance' }) },
  artifacts: [
    {
      key: 'video',
      content_url: 'https://media.example.com/api/task/dance/video?access=test',
    },
  ],
}
const secondVideo: TaskRecord = {
  ...video,
  id: 2,
  task_id: 'walk',
  poster_url: '/walk.jpg',
  properties: { input: JSON.stringify({ selection: {}, brief: 'Walk' }) },
  artifacts: [{ key: 'video', content_url: '/api/task/walk/video' }],
}
const baseProps = {
  open: true,
  onClose: vi.fn(),
  historyItems: [video, secondVideo],
  onLoadMore: vi.fn(),
  onUploadFile: vi.fn(async () => undefined),
  onSelectTask: vi.fn(),
  onPreviewVideo: vi.fn(),
}

describe('video reference picker', () => {
  it('decorates poster cards with a play badge and falls back to a first frame', () => {
    const { container } = render(
      <VideoPickerPanel
        {...baseProps}
        uploadedVideos={[
          { kind: 'upload', url: '/local.mp4', fileName: 'Local clip' },
        ]}
      />
    )
    // The upload has no local derivative here, so its card shows the video
    // first frame; poster cards never mount a video.
    expect(container.querySelectorAll('video').length).toBeGreaterThan(0)
    expect(
      screen
        .getByRole('button', { name: 'Walk' })
        .querySelector('.inf-card-play')
    ).not.toBeNull()
    expect(
      screen
        .getByRole('button', { name: 'Local clip' })
        .querySelector('.inf-card-play')
    ).not.toBeNull()
  })

  it('hands an uploaded video to the shared playback window without selecting it', async () => {
    const onSelectUpload = vi.fn()
    const onPreviewVideo = vi.fn()
    render(
      <VideoPickerPanel
        {...baseProps}
        uploadedVideos={[
          { kind: 'upload', url: '/local.mp4', fileName: 'Local clip' },
        ]}
        onSelectUpload={onSelectUpload}
        onPreviewVideo={onPreviewVideo}
      />
    )
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Preview: Local clip' }))
    expect(onPreviewVideo).toHaveBeenCalledWith('/local.mp4')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(onSelectUpload).not.toHaveBeenCalled()
  })
  it('defaults to all, separates uploaded and generated videos, and retains uploads after reopening', async () => {
    const source = {
      kind: 'upload' as const,
      url: 'https://media.example.com/local.mp4',
      fileName: 'Local clip',
    }
    const onSelectUpload = vi.fn()
    const props = {
      ...baseProps,
      uploadedVideos: [source],
      onSelectUpload,
      selectedUploadUrl: source.url,
    }
    const view = render(<VideoPickerPanel {...props} />)
    const user = userEvent.setup()
    expect(screen.getByRole('tab', { name: 'All references' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(screen.getByRole('button', { name: 'Local clip' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    await user.click(screen.getByRole('tab', { name: 'Uploaded' }))
    expect(
      screen.queryByRole('button', { name: 'Dance' })
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Local clip' }))
    expect(onSelectUpload).toHaveBeenCalledWith(source)
    await user.click(screen.getByRole('tab', { name: 'Generated' }))
    expect(
      screen.queryByRole('button', { name: 'Local clip' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Dance' })).toBeInTheDocument()
    view.rerender(<VideoPickerPanel {...props} open={false} />)
    view.rerender(<VideoPickerPanel {...props} />)
    expect(
      screen.getByRole('button', { name: 'Local clip' })
    ).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'All references' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
  })

  it('shows local upload progress inside the upload card without a separate status row', () => {
    render(
      <VideoPickerPanel {...baseProps} busy preparationPhase='uploading' />
    )
    const card = screen.getByRole('button', { name: 'Upload local video' })
    expect(within(card).getByRole('status')).toHaveTextContent(
      'Uploading reference video…'
    )
    expect(screen.getAllByRole('status')).toHaveLength(1)
    expect(card).toBeDisabled()
  })

  it('never uses a protocol-relative poster rejected by the candidate projection', () => {
    render(
      <VideoPickerPanel
        {...baseProps}
        historyItems={[{ ...video, poster_url: '//unsafe.example/cover.jpg' }]}
      />
    )
    expect(
      screen.getByRole('button', { name: 'Dance' }).querySelector('img')
    ).toBeNull()
  })
  it('hands a declared video artifact to the shared playback window', async () => {
    const onPreviewVideo = vi.fn()
    render(
      <VideoPickerPanel
        {...baseProps}
        historyItems={[
          {
            ...video,
            artifacts: [
              {
                key: 'clip',
                type: 'video',
                content_url:
                  'https://media.example.com/native/content?access=signed',
              },
            ],
          },
        ]}
        onPreviewVideo={onPreviewVideo}
      />
    )
    const card = screen.getByRole('button', { name: 'Dance' })
    expect(card.querySelector('video')).not.toBeNull()
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Preview: Dance' }))
    expect(onPreviewVideo).toHaveBeenCalledWith('/native/content?access=signed')
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
  it('keeps task and artifact selected when the signed playback address changes', () => {
    render(
      <VideoPickerPanel
        {...baseProps}
        selectedTaskId='dance'
        selectedArtifactKey='video'
        selectedVideoUrl='/old-address'
      />
    )
    expect(screen.getByRole('button', { name: 'Dance' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
  })
  it('uses the shared task title for motion histories whose metadata contains only a prompt', () => {
    render(
      <VideoPickerPanel
        {...baseProps}
        historyItems={[
          {
            ...video,
            properties: { input: JSON.stringify({ prompt: '真实动作描述' }) },
          },
        ]}
      />
    )
    expect(
      screen.getByRole('button', { name: '真实动作描述' })
    ).toBeInTheDocument()
  })

  it('matches the selected task and artifact together when the parent supplies both identities', () => {
    render(
      <VideoPickerPanel
        {...baseProps}
        selectedTaskId='dance'
        selectedVideoUrl='/replaced-video.mp4'
      />
    )
    expect(screen.getByRole('button', { name: 'Dance' })).toHaveAttribute(
      'aria-pressed',
      'false'
    )
  })

  it('shows upload failure without removing history and allows a later upload to recover', async () => {
    const onUploadFile = vi
      .fn()
      .mockRejectedValueOnce(new Error('bad media'))
      .mockResolvedValue(undefined)
    const view = render(
      <VideoPickerPanel {...baseProps} onUploadFile={onUploadFile} />
    )
    const input = view.container.querySelector(
      'input[type=file]'
    ) as HTMLInputElement
    const user = userEvent.setup()
    const file = new File(['video'], 'local.mp4', { type: 'video/mp4' })
    await user.upload(input, file)
    expect(toast.error).toHaveBeenCalledExactlyOnceWith('bad media')
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Dance' })).toBeEnabled()
    await user.upload(input, file)
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('uses a poster image when available and a first-frame video otherwise', () => {
    const view = render(<VideoPickerPanel {...baseProps} />)
    const dance = screen.getByRole('button', { name: 'Dance' })
    expect(dance.querySelector('video')).not.toBeNull()
    expect(dance.querySelector('img')).toBeNull()
    expect(dance.querySelector('.inf-card-fallback')).toBeNull()
    expect(
      screen.getByRole('button', { name: 'Walk' }).querySelector('img')
    ).toHaveAttribute('src', '/walk.jpg')
    expect(
      screen.getByRole('button', { name: 'Walk' }).querySelector('video')
    ).toBeNull()
    view.rerender(
      <VideoPickerPanel {...baseProps} busy preparingTaskId='dance' />
    )
    expect(screen.getByRole('button', { name: 'Dance' })).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Upload local video' })
    ).toBeDisabled()
    expect(
      screen.getByLabelText('Preparing reference: Dance')
    ).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Preview: Dance' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Close panel' })).toBeEnabled()
  })

  it('accepts one local file and rejects multiple dragged files instead of silently choosing the first', async () => {
    const onUploadFile = vi.fn(async () => undefined)
    const view = render(
      <VideoPickerPanel {...baseProps} onUploadFile={onUploadFile} />
    )
    const file = new File(['video'], 'local.mov', { type: 'video/quicktime' })
    const input = view.container.querySelector(
      'input[type=file]'
    ) as HTMLInputElement
    await userEvent.setup().upload(input, file)
    expect(onUploadFile).toHaveBeenCalledWith(file)
    onUploadFile.mockClear()
    fireEvent.drop(screen.getByRole('complementary'), {
      dataTransfer: { files: [file, file] },
    })
    expect(onUploadFile).not.toHaveBeenCalled()
    expect(toast.error).toHaveBeenCalledExactlyOnceWith(
      'Choose one video at a time'
    )
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  it('distinguishes initial loading from empty history and offers retry after a read failure', async () => {
    const onRetry = vi.fn()
    const view = render(
      <VideoPickerPanel
        {...baseProps}
        historyItems={[]}
        isInitialLoading
        onRetry={onRetry}
      />
    )
    expect(screen.getByRole('status')).toHaveTextContent('Loading videos')
    expect(screen.queryByText('No videos yet')).not.toBeInTheDocument()
    view.rerender(
      <VideoPickerPanel
        {...baseProps}
        historyItems={[]}
        error={new Error('read failed')}
        onRetry={onRetry}
      />
    )
    expect(screen.getByRole('alert')).toHaveTextContent('Could not load videos')
    await userEvent.setup().click(screen.getByRole('button', { name: 'Retry' }))
    expect(onRetry).toHaveBeenCalledOnce()
    view.rerender(<VideoPickerPanel {...baseProps} historyItems={[]} />)
    expect(screen.queryByText('No videos yet')).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Upload local video' })
    ).toBeEnabled()
  })

  it('keeps loaded candidates when more pages fail and lets the user retry loading more', async () => {
    const onLoadMore = vi.fn()
    render(
      <VideoPickerPanel
        {...baseProps}
        hasMore
        loadMoreFailed
        onLoadMore={onLoadMore}
      />
    )
    expect(screen.getByRole('button', { name: 'Dance' })).toBeEnabled()
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Could not load more videos'
    )
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Retry loading more' }))
    expect(onLoadMore).toHaveBeenCalledOnce()
  })

  it('hands the original external video address to the shared playback window without changing selection', async () => {
    const onSelectTask = vi.fn()
    const onPreviewVideo = vi.fn()
    render(
      <VideoPickerPanel
        {...baseProps}
        onSelectTask={onSelectTask}
        onPreviewVideo={onPreviewVideo}
      />
    )
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Preview: Dance' }))
    expect(onPreviewVideo).toHaveBeenCalledWith(
      'https://media.example.com/api/task/dance/video?access=test'
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(onSelectTask).not.toHaveBeenCalled()
  })

  it('keeps upload first and immediately selects or toggles the selected history task by keyboard', async () => {
    const user = userEvent.setup()
    const onSelectTask = vi.fn()
    render(
      <VideoPickerPanel
        {...baseProps}
        selectedTaskId='dance'
        onSelectTask={onSelectTask}
      />
    )
    const panel = screen.getByRole('complementary')
    expect(within(panel).getAllByRole('button')[1]).toHaveAccessibleName(
      'Upload local video'
    )
    const selected = screen.getByRole('button', { name: 'Dance' })
    expect(selected).toHaveAttribute('aria-pressed', 'true')
    selected.focus()
    await user.keyboard('{Enter}')
    expect(onSelectTask).toHaveBeenLastCalledWith(video)
    await user.click(screen.getByRole('button', { name: 'Walk' }))
    expect(onSelectTask).toHaveBeenLastCalledWith(secondVideo)
  })
})
