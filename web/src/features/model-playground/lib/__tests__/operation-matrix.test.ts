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
import { describe, expect, test } from 'vitest'

import {
  mediaCap,
  hasOperations,
  operationMedia,
  operationsFor,
  vendorSubTaskFields,
} from '../operation-matrix'

describe('operationsFor', () => {
  test('declares all six seedance-2-5 modes', () => {
    expect(operationsFor('seedance-2-5')).toEqual([
      'text_to_video',
      'first_frame',
      'first_last_frame',
      'video_multi_ref',
      'video_edit',
      'video_extend',
    ])
  })

  test('resolves a dotted catalog id to the hyphenated model id', () => {
    expect(operationsFor('seedance-2.5')).toEqual(operationsFor('seedance-2-5'))
  })

  test('ignores surrounding whitespace and case', () => {
    expect(operationsFor('  Seedance-2-5 ')).toEqual(operationsFor('seedance-2-5'))
  })

  test('omits edit and extend on the seedance-2-0 family', () => {
    const modes = operationsFor('seedance-2-0')
    expect(modes).not.toContain('video_edit')
    expect(modes).not.toContain('video_extend')
    expect(modes).toContain('video_multi_ref')
  })

  test('gives grok only the two modes its contract allows', () => {
    expect(operationsFor('grok-imagine-video-1-5')).toEqual([
      'text_to_video',
      'video_multi_ref',
    ])
  })

  test('returns nothing for a model with no declared modes', () => {
    expect(operationsFor('kling-v3')).toEqual([])
    expect(hasOperations('kling-v3')).toBe(false)
    expect(hasOperations('seedance-2-5')).toBe(true)
  })
})

describe('operationMedia', () => {
  test('gives text_to_video no media slot', () => {
    expect(operationMedia('text_to_video')).toEqual([])
  })

  test('marks the first frame as required and single', () => {
    const [slot] = operationMedia('first_frame')
    expect(slot.key).toBe('first_frame')
    expect(slot.required).toBe(true)
    expect(slot.single).toBe(true)
  })

  test('renders reference material as optional multi slots', () => {
    const slots = operationMedia('video_multi_ref')
    expect(slots.map((slot) => slot.key)).toEqual([
      'reference_images',
      'reference_videos',
      'reference_audios',
    ])
    expect(slots.every((slot) => !slot.required && !slot.single)).toBe(true)
  })

  test('never mixes the frame family with the reference family', () => {
    for (const operation of [
      'first_frame',
      'first_last_frame',
      'video_multi_ref',
    ] as const) {
      const keys = operationMedia(operation).map((slot) => slot.key)
      const hasFrame = keys.includes('first_frame') || keys.includes('last_frame')
      const hasReference = keys.some((key) => key.startsWith('reference_'))
      expect(hasFrame && hasReference).toBe(false)
    }
  })
})

describe('vendorSubTaskFields', () => {
  test('maps the seedance-2-5 reference family onto omni_reference_task_type', () => {
    expect(vendorSubTaskFields('seedance-2-5', 'video_multi_ref')).toEqual({
      omni_reference_task_type: 'reference',
    })
    expect(vendorSubTaskFields('seedance-2-5', 'video_edit')).toEqual({
      omni_reference_task_type: 'edit',
    })
    expect(vendorSubTaskFields('seedance-2-5', 'video_extend')).toEqual({
      omni_reference_task_type: 'extend',
    })
  })

  test('omits the field for the frame and text modes', () => {
    expect(vendorSubTaskFields('seedance-2-5', 'text_to_video')).toEqual({})
    expect(vendorSubTaskFields('seedance-2-5', 'first_frame')).toEqual({})
    expect(vendorSubTaskFields('seedance-2-5', 'first_last_frame')).toEqual({})
  })

  test('maps wan-3.0 onto generation_type instead', () => {
    expect(vendorSubTaskFields('wan-3.0', 'first_frame')).toEqual({
      generation_type: 'frame',
    })
    expect(vendorSubTaskFields('wan-3.0', 'video_multi_ref')).toEqual({
      generation_type: 'reference',
    })
  })

  test('sends nothing for a model that carries neither field', () => {
    expect(vendorSubTaskFields('minimax-h3', 'video_multi_ref')).toEqual({})
  })
})

describe('mediaCap', () => {
  test('caps seedance-2-5 at the documented 30 / 10 / 10', () => {
    expect(mediaCap('seedance-2-5', 'image')).toBe(30)
    expect(mediaCap('seedance-2-5', 'video')).toBe(10)
    expect(mediaCap('seedance-2-5', 'audio')).toBe(10)
  })

  test('caps the seedance-2-0 family at 9 / 3 / 3', () => {
    expect(mediaCap('seedance-2-0-fast', 'image')).toBe(9)
    expect(mediaCap('seedance-2-0-fast', 'video')).toBe(3)
  })

  test('publishes no image ceiling for grok, rejects video slots, but allows 3 audio presets', () => {
    expect(mediaCap('grok-imagine-video-1-5', 'image')).toBeUndefined()
    expect(mediaCap('grok-imagine-video-1-5', 'video')).toBe(0)
    expect(mediaCap('grok-imagine-video-1-5', 'audio')).toBe(3)
  })

  test('caps kling-v3 video references at 1 and images at 7', () => {
    expect(mediaCap('kling-v3', 'video')).toBe(1)
    expect(mediaCap('kling-v3', 'image')).toBe(7)
  })
})
