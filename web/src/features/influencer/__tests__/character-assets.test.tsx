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
import { describe, expect, it, vi } from 'vitest'

import { CharacterPickerPanel } from '../components/character-picker-panel'
import { CharacterSlot } from '../components/character-slot'
import type { MotionCharacterSource, TaskRecord } from '../types'

const history: TaskRecord[] = [
  {
    id: 1,
    task_id: 'portrait-task',
    status: 'SUCCESS',
    properties: {
      input: JSON.stringify({ tier: 't1', selection: {}, brief: 'Portrait' }),
    },
    artifacts: [{ url: 'https://files.example/portrait.png' }],
  },
  {
    id: 2,
    task_id: 'sheet-task',
    status: 'SUCCESS',
    properties: {
      input: JSON.stringify({ tier: 't1', selection: {}, brief: 'Sheet' }),
    },
    artifacts: [{ url: 'https://files.example/sheet.png' }],
  },
]

function renderPicker(characters: MotionCharacterSource[] = [], busy = false) {
  const onToggleUrl = vi.fn()
  render(
    <CharacterPickerPanel
      open
      onClose={vi.fn()}
      characters={characters}
      historyItems={history}
      busy={busy}
      onUploadFiles={vi.fn()}
      onToggleUrl={onToggleUrl}
    />
  )
  return onToggleUrl
}

describe('character asset interactions', () => {
  it('separates uploaded images from generated images without changing selection', async () => {
    const source = {
      url: 'https://media.example.com/upload.webp',
      fileName: 'Uploaded photo',
    }
    const onToggleUrl = vi.fn()
    render(
      <CharacterPickerPanel
        open
        onClose={vi.fn()}
        characters={[]}
        uploadedCharacters={[source]}
        historyItems={history}
        onUploadFiles={vi.fn()}
        onToggleUrl={onToggleUrl}
      />
    )
    const user = userEvent.setup()
    expect(screen.getByRole('tab', { name: 'All references' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    await user.click(screen.getByRole('tab', { name: 'Uploaded' }))
    expect(
      screen.queryByRole('button', { name: 'Portrait' })
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Uploaded photo' }))
    expect(onToggleUrl).toHaveBeenCalledWith(source)
    await user.click(screen.getByRole('tab', { name: 'Generated' }))
    expect(
      screen.queryByRole('button', { name: 'Uploaded photo' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Portrait' })).toBeInTheDocument()
  })
  it('keeps the upload entry free of status copy while a file uploads', () => {
    render(
      <CharacterPickerPanel
        open
        onClose={vi.fn()}
        characters={[]}
        historyItems={[]}
        uploads={[
          {
            id: 'batch-1',
            fileName: 'first.png',
            previewUrl: 'blob:first',
            progress: 40,
            status: 'uploading',
          },
        ]}
        busy
        onUploadFiles={vi.fn()}
        onToggleUrl={vi.fn()}
      />
    )
    const uploadEntry = screen.getByRole('button', {
      name: 'Upload local photo',
    })
    expect(within(uploadEntry).queryByRole('status')).toBeNull()
    expect(uploadEntry).toHaveTextContent('Upload local photo')
    // The running file's state lives on its own card instead.
    expect(
      within(screen.getByRole('group', { name: 'first.png' })).getByText(
        'Uploading…'
      )
    ).toBeInTheDocument()
    expect(screen.queryByText('No characters yet')).not.toBeInTheDocument()
  })

  it('hands every picked file to one upload batch instead of only the first', async () => {
    const user = userEvent.setup()
    const onUploadFiles = vi.fn()
    render(
      <CharacterPickerPanel
        open
        onClose={vi.fn()}
        characters={[]}
        historyItems={[]}
        onUploadFiles={onUploadFiles}
        onToggleUrl={vi.fn()}
      />
    )
    const input = document.querySelector(
      'input[accept="image/png,image/jpeg,image/webp"]'
    )
    if (!(input instanceof HTMLInputElement)) {
      throw new Error('the picker rendered no file input')
    }
    expect(input.multiple).toBe(true)
    const files = [
      new File(['one'], 'one.png', { type: 'image/png' }),
      new File(['two'], 'two.png', { type: 'image/png' }),
    ]

    await user.upload(input, files)

    expect(onUploadFiles).toHaveBeenCalledTimes(1)
    expect(onUploadFiles).toHaveBeenCalledWith(files)
  })

  it('shows one card per queued file with progress, retry and removal', async () => {
    const user = userEvent.setup()
    const onRetryUpload = vi.fn()
    const onDismissUpload = vi.fn()
    render(
      <CharacterPickerPanel
        open
        onClose={vi.fn()}
        characters={[]}
        historyItems={[]}
        uploads={[
          {
            id: 'first',
            fileName: 'first.png',
            previewUrl: 'blob:first',
            progress: 42,
            status: 'uploading',
          },
          {
            id: 'second',
            fileName: 'second.png',
            previewUrl: 'blob:second',
            progress: 100,
            status: 'error',
          },
        ]}
        onUploadFiles={vi.fn()}
        onRetryUpload={onRetryUpload}
        onDismissUpload={onDismissUpload}
        onToggleUrl={vi.fn()}
      />
    )

    const progressCard = screen.getByRole('group', { name: 'first.png' })
    expect(within(progressCard).getByText('Uploading…')).toBeInTheDocument()
    expect(within(progressCard).queryByText('42%')).toBeNull()
    expect(within(progressCard).queryByRole('status')).toBeNull()

    const failedCard = screen.getByRole('group', { name: 'second.png' })
    expect(within(failedCard).getByText('Upload failed')).toBeInTheDocument()
    await user.click(
      within(failedCard).getByRole('button', {
        name: 'Retry upload: second.png',
      })
    )
    expect(onRetryUpload).toHaveBeenCalledWith('second')

    await user.click(
      within(failedCard).getByRole('button', { name: 'Remove: second.png' })
    )
    expect(onDismissUpload).toHaveBeenCalledWith('second')
  })

  it('shows queued files without presenting them as uploading or live regions', () => {
    render(
      <CharacterPickerPanel
        open
        onClose={vi.fn()}
        characters={[]}
        historyItems={[]}
        uploads={[
          {
            id: 'queued',
            fileName: 'queued.png',
            previewUrl: 'blob:queued',
            progress: 0,
            status: 'waiting',
          },
        ]}
        onUploadFiles={vi.fn()}
        onToggleUrl={vi.fn()}
      />
    )

    const queued = screen.getByRole('group', { name: 'queued.png' })
    expect(within(queued).getByText('Queued')).toBeInTheDocument()
    expect(within(queued).queryByText('Uploading…')).toBeNull()
    expect(within(queued).queryByRole('status')).toBeNull()
  })

  it('keeps the upload entry usable while a batch is still uploading', () => {
    render(
      <CharacterPickerPanel
        open
        onClose={vi.fn()}
        characters={[]}
        historyItems={[]}
        uploads={[
          {
            id: 'running',
            fileName: 'running.png',
            previewUrl: 'blob:running',
            progress: 30,
            status: 'uploading',
          },
        ]}
        onUploadFiles={vi.fn()}
        onToggleUrl={vi.fn()}
      />
    )
    expect(
      screen.getByRole('button', { name: 'Upload local photo' })
    ).toBeEnabled()
    expect(document.querySelector('input[type=file]')).toBeEnabled()
  })

  it('puts upload first and exposes the current multi-selection to assistive technology', async () => {
    const user = userEvent.setup()
    const onToggleUrl = renderPicker([
      { url: 'https://files.example/portrait.png', fileName: 'Portrait' },
    ])
    const panel = screen.getByRole('complementary')
    expect(within(panel).getAllByRole('button')[1]).toHaveAccessibleName(
      'Upload local photo'
    )
    expect(screen.getByRole('button', { name: 'Portrait' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    const sheet = screen.getByRole('button', { name: 'Sheet' })
    expect(sheet).toHaveAttribute('aria-pressed', 'false')
    await user.click(sheet)
    expect(onToggleUrl).toHaveBeenCalledWith({
      url: 'https://files.example/sheet.png',
      previewUrl: 'https://files.example/sheet.png',
      fileName: 'Sheet',
    })
  })

  it('hands a history image to the shared preview without toggling selection', async () => {
    const onToggleUrl = vi.fn()
    const onPreviewImage = vi.fn()
    render(
      <CharacterPickerPanel
        open
        onClose={vi.fn()}
        characters={[]}
        historyItems={history}
        onUploadFiles={vi.fn()}
        onToggleUrl={onToggleUrl}
        onPreviewImage={onPreviewImage}
      />
    )
    await userEvent
      .setup()
      .click(screen.getByRole('button', { name: 'Preview: Sheet' }))
    expect(onPreviewImage).toHaveBeenCalledWith(
      'https://files.example/sheet.png'
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(onToggleUrl).not.toHaveBeenCalled()
  })

  it('at capacity blocks new selections and upload but allows removal of an existing selection', async () => {
    const user = userEvent.setup()
    const characters = [
      { url: 'https://files.example/portrait.png', fileName: 'Portrait' },
      ...Array.from({ length: 29 }, (_, i) => ({
        url: `/other-${i}.png`,
        fileName: `Other ${i}`,
      })),
    ]
    const onToggleUrl = renderPicker(characters)
    expect(
      screen.getByRole('button', { name: 'Upload local photo' })
    ).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Sheet' })).toBeDisabled()
    const portrait = screen.getByRole('button', {
      name: 'Portrait',
    })
    expect(portrait).toBeEnabled()
    await user.click(portrait)
    expect(onToggleUrl).toHaveBeenCalledWith({
      url: 'https://files.example/portrait.png',
      previewUrl: 'https://files.example/portrait.png',
      fileName: 'Portrait',
    })
  })

  it('opens a selected image by keyboard and keeps deletion separate from preview', async () => {
    const user = userEvent.setup()
    const onRemoveCharacter = vi.fn()
    const onOpenPicker = vi.fn()
    const onPreviewImage = vi.fn()
    render(
      <CharacterSlot
        label='Character'
        hint='Reference'
        characters={[
          { url: 'https://files.example/sheet.png', fileName: 'Sheet' },
        ]}
        onOpenPicker={onOpenPicker}
        onRemoveCharacter={onRemoveCharacter}
        onPickDirectFiles={vi.fn()}
        onPreviewImage={onPreviewImage}
      />
    )
    screen.getByRole('button', { name: 'Preview: Sheet' }).focus()
    await user.keyboard('{Enter}')
    expect(onPreviewImage).toHaveBeenCalledWith(
      'https://files.example/sheet.png'
    )
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Remove' }))
    expect(onRemoveCharacter).toHaveBeenCalledWith(0)
    expect(onOpenPicker).not.toHaveBeenCalled()
  })

  it('while busy blocks upload, selection and deletion without hiding the image preview', () => {
    renderPicker([], true)
    expect(
      screen.getByRole('button', { name: 'Upload local photo' })
    ).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Portrait' })).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Preview: Portrait' })
    ).toBeEnabled()
  })
})
