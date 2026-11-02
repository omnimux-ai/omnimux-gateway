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

import {
  act,
  cleanup,
  renderHook,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { submitMotion } from '../api'
import { CharacterPickerPanel } from '../components/character-picker-panel'
import { CharacterSlot } from '../components/character-slot'
import { useMotion } from '../hooks/use-motion'
import { buildMotionSubmitPayload } from '../lib/submit-payload'
import type { TaskRecord } from '../types'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})
it('selecting a historical cat multiview uses parent species while binding the selected child artifact', () => {
  const parent: TaskRecord = {
    id: 1,
    task_id: 'cat-parent',
    status: 'SUCCESS',
    properties: {
      input: JSON.stringify({ tier: 'cats', selection: {}, brief: 'Cat' }),
    },
    artifacts: [{ url: 'https://files.example/cat.png' }],
  }
  const child: TaskRecord = {
    id: 2,
    task_id: 'cat-child',
    status: 'SUCCESS',
    properties: {
      input: JSON.stringify({
        kind: 'multiview',
        parent_task_id: 'cat-parent',
        brief: 'Views',
      }),
    },
    artifacts: [{ url: 'https://files.example/views.png' }],
  }
  const toggle = vi.fn()
  render(
    <CharacterPickerPanel
      open
      onClose={vi.fn()}
      characters={[]}
      historyItems={[parent, child]}
      onUploadFiles={vi.fn()}
      onToggleUrl={toggle}
    />
  )
  fireEvent.click(screen.getByRole('button', { name: 'Views' }))
  expect(toggle).toHaveBeenCalledWith(
    expect.objectContaining({
      url: 'https://files.example/views.png',
      sourceTaskId: 'cat-child',
      characterType: 'cat',
    })
  )
})
it('selecting an old dog multiview whose parent is outside the loaded page reads that parent before assigning species', async () => {
  const parent: TaskRecord = {
    id: 1,
    task_id: 'dog-parent',
    status: 'SUCCESS',
    properties: { input: JSON.stringify({ tier: 'dogs', selection: {} }) },
  }
  const child: TaskRecord = {
    id: 2,
    task_id: 'dog-child',
    status: 'SUCCESS',
    properties: {
      input: JSON.stringify({
        kind: 'multiview',
        parent_task_id: 'dog-parent',
        brief: 'Dog views',
      }),
    },
    artifacts: [{ url: 'https://files.example/dog-views.png' }],
  }
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: { items: [parent], total: 1 } },
  })
  const toggle = vi.fn()
  render(
    <CharacterPickerPanel
      open
      onClose={vi.fn()}
      characters={[]}
      historyItems={[child]}
      onUploadFiles={vi.fn()}
      onToggleUrl={toggle}
    />
  )
  fireEvent.click(screen.getByRole('button', { name: 'Dog views' }))
  await waitFor(() =>
    expect(toggle).toHaveBeenCalledWith(
      expect.objectContaining({
        sourceTaskId: 'dog-child',
        characterType: 'dog',
      })
    )
  )
})

it('a selected external HTTPS history image stays selected while thumbnails and preview retain its origin', () => {
  const task: TaskRecord = {
    id: 1,
    task_id: 'cat',
    status: 'SUCCESS',
    properties: { input: '{"tier":"cats","brief":"Cat"}' },
    artifacts: [{ url: 'https://files.example/cat.png?access=fixture' }],
  }
  const toggle = vi.fn()
  const onPreviewImage = vi.fn()
  render(
    <CharacterPickerPanel
      open
      onClose={vi.fn()}
      characters={[
        {
          url: 'https://files.example/cat.png?access=fixture',
          fileName: 'Cat',
        },
      ]}
      historyItems={[task]}
      onUploadFiles={vi.fn()}
      onToggleUrl={toggle}
      onPreviewImage={onPreviewImage}
    />
  )
  const choice = screen.getByRole('button', { name: 'Cat' })
  expect(choice).toHaveAttribute('aria-pressed', 'true')
  expect(choice.querySelector('img')).toHaveAttribute(
    'src',
    'https://files.example/cat.png?access=fixture'
  )
  fireEvent.click(screen.getByRole('button', { name: 'Preview: Cat' }))
  expect(onPreviewImage).toHaveBeenCalledWith(
    'https://files.example/cat.png?access=fixture'
  )
  expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  expect(toggle).not.toHaveBeenCalled()
})
it.each(['/relative.png', 'http://files.example/plain.png'])(
  'a non-HTTPS history artifact %s stays previewable but cannot be submitted',
  (url) => {
    const toggle = vi.fn()
    render(
      <CharacterPickerPanel
        open
        onClose={vi.fn()}
        characters={[]}
        historyItems={[
          {
            id: 1,
            task_id: 'bad',
            status: 'SUCCESS',
            properties: { input: '{"tier":"cats","brief":"Cat"}' },
            artifacts: [{ url }],
          },
        ]}
        onUploadFiles={vi.fn()}
        onToggleUrl={toggle}
      />
    )
    expect(screen.getByRole('button', { name: 'Cat' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Preview: Cat' })).toBeEnabled()
  }
)
it.each(['motion', 'replace'] as const)(
  'the actual %s request preserves the selected history HTTPS image and its cat identity',
  async (mode) => {
    const motion = renderHook(useMotion)
    const post = vi
      .spyOn(api, 'post')
      .mockResolvedValue({ data: { task_id: 'accepted' } })
    const pickerView = render(
      <CharacterPickerPanel
        open
        onClose={vi.fn()}
        characters={[]}
        historyItems={[
          {
            id: 1,
            task_id: 'cat',
            status: 'SUCCESS',
            properties: { input: '{"tier":"cats","brief":"Cat"}' },
            artifacts: [
              { url: 'https://files.example/cat.png?access=fixture' },
            ],
          },
        ]}
        onUploadFiles={vi.fn()}
        onToggleUrl={motion.result.current.toggleCharacter}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Cat' }))
    const selected = motion.result.current.character
    expect(selected).toMatchObject({
      url: 'https://files.example/cat.png?access=fixture',
      previewUrl: 'https://files.example/cat.png?access=fixture',
      characterType: 'cat',
      sourceTaskId: 'cat',
    })
    if (!selected) throw new Error('missing selected source')
    act(() => motion.result.current.addCharacter(selected))
    expect(motion.result.current.characters).toHaveLength(1)
    expect(motion.result.current.character).toBe(selected)
    act(() =>
      motion.result.current.addCharacter({
        url: 'https://uploads.example/other.png',
        fileName: 'Other',
      })
    )
    act(() => motion.result.current.removeCharacter(1))
    expect(motion.result.current.character).toBe(selected)
    pickerView.unmount()
    const preview = vi.fn()
    render(
      <CharacterSlot
        label='Character'
        hint='Assets'
        characters={motion.result.current.characters}
        onOpenPicker={motion.result.current.openCharacterPicker}
        onRemoveCharacter={motion.result.current.removeCharacter}
        onPickDirectFiles={motion.result.current.pickCharacters}
        onPreviewImage={preview}
      />
    )
    const selectedImage = screen.getByRole('button', { name: 'Preview: Cat' })
    expect(selectedImage.querySelector('img')).toHaveAttribute(
      'src',
      'https://files.example/cat.png?access=fixture'
    )
    fireEvent.click(selectedImage)
    expect(preview).toHaveBeenCalledWith(
      'https://files.example/cat.png?access=fixture'
    )
    const body = buildMotionSubmitPayload({
      mode,
      prompt: '',
      character: motion.result.current.character,
      characters: motion.result.current.characters,
      video: {
        kind: 'remote',
        url: 'https://files.example/clip.mp4',
        name: 'Clip',
        metadata: {
          version: 1,
          browserMetadata: {
            duration: 5,
            video_duration: 5,
            width: 1280,
            height: 720,
            frame_rate: 30,
            video_codec: 'h264',
            audio_codec: 'aac',
            container: 'mp4',
            size_bytes: 4,
          },
          rawDurationSec: 5,
          durationSec: 5,
          width: 1280,
          height: 720,
          aspectRatio: '16:9',
        },
      },
      resolution: '720p',
    })
    expect(body).not.toBeNull()
    if (!body) throw new Error('missing payload')
    await submitMotion(body)
    expect(post).toHaveBeenCalledWith(
      expect.stringContaining('/motion'),
      expect.objectContaining({
        subject_image_url: 'https://files.example/cat.png?access=fixture',
        subject_image_urls: ['https://files.example/cat.png?access=fixture'],
        subject_character_type: 'cat',
        source_task_id: 'cat',
      }),
      expect.anything()
    )
  }
)
it('an unknown historical type keeps its real artifact preview without inventing species or source identity', async () => {
  const motion = renderHook(useMotion)
  const picker = render(
    <CharacterPickerPanel
      open
      onClose={vi.fn()}
      characters={[]}
      historyItems={[
        {
          id: 1,
          task_id: 'unknown',
          status: 'SUCCESS',
          properties: { input: '{"tier":"unrecognized","brief":"Unknown"}' },
          artifacts: [
            { content_url: 'https://files.example/unknown.png?access=fixture' },
          ],
        },
      ]}
      onUploadFiles={vi.fn()}
      onToggleUrl={motion.result.current.toggleCharacter}
    />
  )
  fireEvent.click(screen.getByRole('button', { name: 'Unknown' }))
  await waitFor(() => expect(motion.result.current.characters).toHaveLength(1))
  expect(motion.result.current.character).toEqual({
    url: 'https://files.example/unknown.png?access=fixture',
    previewUrl: 'https://files.example/unknown.png?access=fixture',
    fileName: 'Unknown',
  })
  picker.unmount()
  const preview = vi.fn()
  render(
    <CharacterSlot
      label='Character'
      hint='Assets'
      characters={motion.result.current.characters}
      onOpenPicker={motion.result.current.openCharacterPicker}
      onRemoveCharacter={motion.result.current.removeCharacter}
      onPickDirectFiles={motion.result.current.pickCharacters}
      onPreviewImage={preview}
    />
  )
  const image = screen.getByRole('button', { name: 'Preview: Unknown' })
  expect(image.querySelector('img')).toHaveAttribute(
    'src',
    'https://files.example/unknown.png?access=fixture'
  )
  fireEvent.click(image)
  expect(preview).toHaveBeenCalledWith(
    'https://files.example/unknown.png?access=fixture'
  )
})

it('closing the picker before its parent lookup finishes never applies a late selection', async () => {
  let finish: (value: unknown) => void = () => {
    throw new Error('lookup not started')
  }
  vi.spyOn(api, 'get').mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve
      })
  )
  const toggle = vi.fn()
  const props = {
    onClose: vi.fn(),
    characters: [],
    historyItems: [
      {
        id: 2,
        task_id: 'views',
        status: 'SUCCESS',
        properties: {
          input: '{"kind":"multiview","parent_task_id":"cat","brief":"Views"}',
        },
        artifacts: [{ url: 'https://files.example/views.png' }],
      },
    ],
    onUploadFiles: vi.fn(),
    onToggleUrl: toggle,
  }
  const view = render(<CharacterPickerPanel {...props} open />)
  fireEvent.click(screen.getByRole('button', { name: 'Views' }))
  view.rerender(<CharacterPickerPanel {...props} open={false} />)
  await act(async () =>
    finish({
      data: {
        success: true,
        data: {
          items: [
            {
              id: 1,
              task_id: 'cat',
              status: 'SUCCESS',
              properties: { input: '{"tier":"cats"}' },
            },
          ],
          total: 1,
        },
      },
    })
  )
  expect(toggle).not.toHaveBeenCalled()
})
