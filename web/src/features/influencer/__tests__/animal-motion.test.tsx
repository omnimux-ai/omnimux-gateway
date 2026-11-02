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

import { act, cleanup, renderHook } from '@testing-library/react'
import { afterEach, expect, it } from 'vitest'

import { useMotion } from '../hooks/use-motion'
import { buildMotionSubmitPayload } from '../lib/submit-payload'
import type { MotionCharacterSource } from '../types'

afterEach(cleanup)
it.each(['motion', 'replace'] as const)(
  'an unknown first reference mixed with cat and dog in %s never guesses human or borrows the later species',
  (mode) => {
    const { result } = renderHook(useMotion)
    act(() =>
      result.current.addCharacter({
        url: 'https://files.example/unknown.png',
        fileName: 'Unknown',
      })
    )
    act(() =>
      result.current.addCharacter({
        url: 'https://files.example/cat.png',
        fileName: 'Cat',
        characterType: 'cat',
        sourceTaskId: 'cat',
      })
    )
    act(() =>
      result.current.addCharacter({
        url: 'https://files.example/dog.png',
        fileName: 'Dog',
        characterType: 'dog',
        sourceTaskId: 'dog',
      })
    )
    const payload = buildMotionSubmitPayload({
      mode,
      prompt: '',
      character: result.current.character,
      characters: result.current.characters,
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
    expect(payload).toMatchObject({
      subject_image_url: 'https://files.example/unknown.png',
      subject_image_urls: [
        'https://files.example/unknown.png',
        'https://files.example/cat.png',
        'https://files.example/dog.png',
      ],
    })
    expect(payload?.subject_character_type).toBeUndefined()
    expect(payload?.source_task_id).toBeUndefined()
  }
)
it('removing the first cat reference makes the dog first and the actual payload carries dog source metadata', () => {
  const { result } = renderHook(useMotion)
  const cat = {
    url: 'https://files.example/cat.png',
    fileName: 'Cat',
    characterType: 'cat',
    sourceTaskId: 'cat-sheet',
  } as MotionCharacterSource
  const dog = {
    url: 'https://files.example/dog.png',
    fileName: 'Dog',
    characterType: 'dog',
    sourceTaskId: 'dog-sheet',
  } as MotionCharacterSource
  act(() => result.current.addCharacter(cat))
  act(() => result.current.addCharacter(dog))
  act(() => result.current.removeCharacter(0))
  const payload = buildMotionSubmitPayload({
    mode: 'motion',
    prompt: '',
    character: result.current.character,
    characters: result.current.characters,
    video: {
      kind: 'upload',
      url: 'https://files.example/clip.mp4',
      fileName: 'Clip',
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
  expect(payload).toMatchObject({
    subject_image_url: dog.url,
    subject_character_type: 'dog',
    source_task_id: 'dog-sheet',
  })
})
