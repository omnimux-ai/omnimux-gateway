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
import { fireEvent, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { CharacterPickerPanel } from '../components/character-picker-panel'
import { CharacterSlot } from '../components/character-slot'
import type { MotionCharacterSource, TaskRecord } from '../types'

it('an uploaded external image hands its complete address to the shared preview', () => {
  const onPreviewImage = vi.fn()
  render(
    <CharacterSlot
      label='Character'
      hint='Assets'
      characters={[
        {
          url: 'https://uploads.example/photo.png?token=fixture',
          fileName: 'Upload',
        },
      ]}
      onOpenPicker={vi.fn()}
      onRemoveCharacter={vi.fn()}
      onPickDirectFiles={vi.fn()}
      onPreviewImage={onPreviewImage}
    />
  )
  const image = screen.getByRole('button', { name: 'Preview: Upload' })
  expect(image.querySelector('img')).toHaveAttribute(
    'src',
    'https://uploads.example/photo.png?token=fixture'
  )
  fireEvent.click(image)
  expect(onPreviewImage).toHaveBeenCalledWith(
    'https://uploads.example/photo.png?token=fixture'
  )
})

describe('CharacterSlot', () => {
  it('renders default empty slot when no characters selected', async () => {
    const user = userEvent.setup()
    const onOpenPicker = vi.fn()
    render(
      <CharacterSlot
        label='Character'
        hint='Face, hair and outfit'
        characters={[]}
        onOpenPicker={onOpenPicker}
        onRemoveCharacter={vi.fn()}
        onPickDirectFiles={vi.fn()}
      />
    )

    const emptyBtn = screen.getByRole('button', { name: /Character/ })
    expect(emptyBtn).toBeInTheDocument()
    expect(screen.getByText('Face, hair and outfit')).toBeInTheDocument()

    await user.click(emptyBtn)
    expect(onOpenPicker).toHaveBeenCalledTimes(1)
  })

  it('keeps the same outer slot through adding and clearing a character', () => {
    const props = {
      label: 'Character',
      hint: 'Face, hair and outfit',
      onOpenPicker: vi.fn(),
      onRemoveCharacter: vi.fn(),
      onPickDirectFiles: vi.fn(),
    }
    const view = render(<CharacterSlot {...props} characters={[]} />)
    const slot = view.container.firstChild
    view.rerender(
      <CharacterSlot
        {...props}
        characters={[{ url: '/one.png', fileName: 'one.png' }]}
      />
    )
    expect(view.container.firstChild).toBe(slot)
    expect(slot).toHaveClass('inf-slot')
    expect(
      screen.getByRole('button', { name: 'Preview: one.png' })
    ).toBeInTheDocument()
    view.rerender(<CharacterSlot {...props} characters={[]} />)
    expect(view.container.firstChild).toBe(slot)
    expect(screen.getByText(props.hint)).toBeInTheDocument()
  })

  it('touch selects only the current card without opening the picker and clears on blank space', async () => {
    const user = userEvent.setup()
    const onOpenPicker = vi.fn()
    const view = render(
      <CharacterSlot
        label='Character'
        hint='Assets'
        characters={[
          { url: '/one.png', fileName: 'one' },
          { url: '/two.png', fileName: 'two' },
        ]}
        onOpenPicker={onOpenPicker}
        onRemoveCharacter={vi.fn()}
        onPickDirectFiles={vi.fn()}
      />
    )
    const first = screen.getByRole('button', { name: 'Preview: one' })
    const second = screen.getByRole('button', { name: 'Preview: two' })
    await user.pointer([
      { keys: '[TouchA>]', target: first },
      { keys: '[/TouchA]' },
    ])
    expect(first.closest('.inf-character-tile')).toHaveAttribute(
      'data-active',
      'true'
    )
    await user.pointer([
      { keys: '[TouchA>]', target: second },
      { keys: '[/TouchA]' },
    ])
    expect(first.closest('.inf-character-tile')).not.toHaveAttribute(
      'data-active'
    )
    expect(second.closest('.inf-character-tile')).toHaveAttribute(
      'data-active',
      'true'
    )
    expect(onOpenPicker).not.toHaveBeenCalled()
    await user.pointer([
      { keys: '[TouchA>]', target: view.container.firstChild as HTMLElement },
      { keys: '[/TouchA]' },
    ])
    expect(second.closest('.inf-character-tile')).not.toHaveAttribute(
      'data-active'
    )
  })

  it('disables adding and removal while busy', async () => {
    const user = userEvent.setup()
    const onOpenPicker = vi.fn()
    const onRemoveCharacter = vi.fn()
    render(
      <CharacterSlot
        label='Character'
        hint='Face, hair and outfit'
        characters={[{ url: '/one.png', fileName: 'one.png' }]}
        busy
        onOpenPicker={onOpenPicker}
        onRemoveCharacter={onRemoveCharacter}
        onPickDirectFiles={vi.fn()}
      />
    )
    expect(
      screen.getByRole('button', { name: 'Add reference image' })
    ).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Remove' })).toBeDisabled()
    await user.click(screen.getByRole('button', { name: 'Remove' }))
    expect(onRemoveCharacter).not.toHaveBeenCalled()
    expect(onOpenPicker).not.toHaveBeenCalled()
  })

  it('allows dropping an image into an already populated slot', () => {
    const onPickDirectFiles = vi.fn()
    const view = render(
      <CharacterSlot
        label='Character'
        hint='Face, hair and outfit'
        characters={[{ url: '/one.png', fileName: 'one.png' }]}
        onOpenPicker={vi.fn()}
        onRemoveCharacter={vi.fn()}
        onPickDirectFiles={onPickDirectFiles}
      />
    )
    const file = new File(['image'], 'two.png', { type: 'image/png' })
    fireEvent.drop(view.container.firstChild as HTMLElement, {
      dataTransfer: { files: [file] },
    })
    expect(onPickDirectFiles).toHaveBeenCalledWith([file])
  })

  it('shows an ordered gallery and plus button below the 30-image limit', async () => {
    const user = userEvent.setup()
    const onOpenPicker = vi.fn()
    const onRemoveCharacter = vi.fn()
    const chars: MotionCharacterSource[] = [
      { url: 'https://example.com/1.png', fileName: 'one.png' },
      { url: 'https://example.com/2.png', fileName: 'two.png' },
    ]

    render(
      <CharacterSlot
        label='Character'
        hint='Face, hair and outfit'
        characters={chars}
        onOpenPicker={onOpenPicker}
        onRemoveCharacter={onRemoveCharacter}
        onPickDirectFiles={vi.fn()}
      />
    )

    // Should not render the large empty button
    expect(screen.queryByText('Face, hair and outfit')).not.toBeInTheDocument()
    // Should display selected count
    expect(
      screen.getByText(
        (_, element) =>
          element?.className === 'sr-only' &&
          !!element.textContent?.includes('Selected') &&
          !!element.textContent?.includes('2')
      )
    ).toBeInTheDocument()

    // Plus button should be visible
    const plusBtn = screen.getByRole('button', { name: /Add reference image/ })
    expect(plusBtn).toBeInTheDocument()

    // Clicking plus calls onOpenPicker
    await user.click(plusBtn)
    expect(onOpenPicker).toHaveBeenCalledTimes(1)

    // Removing an item
    const removeBtns = screen.getAllByRole('button', { name: /Remove/ })
    expect(removeBtns).toHaveLength(2)
    await user.click(removeBtns[0])
    expect(onRemoveCharacter).toHaveBeenCalledWith(0)
    expect(onOpenPicker).toHaveBeenCalledTimes(1)
  })

  it.each(['busy', 'full'])('rejects dropping more images when %s', (state) => {
    const onPickDirectFiles = vi.fn()
    const characters = Array.from(
      { length: state === 'full' ? 30 : 1 },
      (_, index) => ({
        url: `/image-${index}.png`,
        fileName: `image-${index}.png`,
      })
    )
    const view = render(
      <CharacterSlot
        label='Character'
        hint='Face, hair and outfit'
        characters={characters}
        busy={state === 'busy'}
        onOpenPicker={vi.fn()}
        onRemoveCharacter={vi.fn()}
        onPickDirectFiles={onPickDirectFiles}
      />
    )
    fireEvent.drop(view.container.firstChild as HTMLElement, {
      dataTransfer: {
        files: [new File(['image'], 'next.png', { type: 'image/png' })],
      },
    })
    expect(onPickDirectFiles).not.toHaveBeenCalled()
  })

  it('hides plus button when character count reaches 30', () => {
    const chars: MotionCharacterSource[] = Array.from(
      { length: 30 },
      (_, i) => ({
        url: `https://example.com/${i}.png`,
        fileName: `img-${i}.png`,
      })
    )

    render(
      <CharacterSlot
        label='Character'
        hint='Face, hair and outfit'
        characters={chars}
        onOpenPicker={vi.fn()}
        onRemoveCharacter={vi.fn()}
        onPickDirectFiles={vi.fn()}
      />
    )

    // 30 items rendered
    const removeBtns = screen.getAllByRole('button', { name: /Remove/ })
    expect(removeBtns).toHaveLength(30)

    // Plus button MUST NOT be displayed
    expect(
      screen.queryByRole('button', { name: /Add reference image/ })
    ).toBeNull()
  })
})

describe('CharacterPickerPanel', () => {
  const dummyHistory: TaskRecord[] = [
    {
      id: 1,
      task_id: 'task-1',
      status: 'SUCCESS',
      properties: {
        input: JSON.stringify({
          tier: 't1',
          selection: {},
          brief: 'Gothic dress girl',
        }),
      },
      artifacts: [{ url: 'https://example.com/gothic.png' }],
      created_at: '2026-10-08T10:00:00Z',
    },
    {
      id: 2,
      task_id: 'task-2',
      status: 'SUCCESS',
      properties: {
        input: JSON.stringify({
          tier: 't1',
          selection: {},
          brief: 'Brown suit man',
        }),
      },
      artifacts: [{ url: 'https://example.com/suit.png' }],
      created_at: '2026-10-08T11:00:00Z',
    },
  ]

  it('renders panel with upload button and history cards', async () => {
    const user = userEvent.setup()
    const onToggleUrl = vi.fn()
    const onClose = vi.fn()

    render(
      <CharacterPickerPanel
        open
        onClose={onClose}
        characters={[
          { url: 'https://example.com/gothic.png', fileName: 'gothic' },
        ]}
        historyItems={dummyHistory}
        onUploadFiles={vi.fn()}
        onToggleUrl={onToggleUrl}
      />
    )

    expect(
      screen.getByRole('complementary', { name: 'Select reference images' })
    ).toBeInTheDocument()
    expect(screen.getByRole('tab', { name: 'All references' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(
      screen.getByRole('button', { name: /Upload local photo/i })
    ).toBeInTheDocument()
    expect(screen.getByText('Gothic dress girl')).toBeInTheDocument()
    expect(screen.getByText('Brown suit man')).toBeInTheDocument()

    // Click on suit to toggle
    await user.click(screen.getByText('Brown suit man'))
    expect(onToggleUrl).toHaveBeenCalledWith(
      expect.objectContaining({ url: 'https://example.com/suit.png' })
    )

    // Close button
    await user.click(screen.getByRole('button', { name: /Close panel/i }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('renders null when open is false', () => {
    const { container } = render(
      <CharacterPickerPanel
        open={false}
        onClose={vi.fn()}
        characters={[]}
        historyItems={dummyHistory}
        onUploadFiles={vi.fn()}
        onToggleUrl={vi.fn()}
      />
    )
    expect(container.firstChild).toBeNull()
  })
})
