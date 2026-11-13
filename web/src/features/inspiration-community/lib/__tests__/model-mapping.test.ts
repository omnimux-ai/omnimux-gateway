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

import { resolveInspirationPlaygroundModelId } from '../model-mapping'

describe('resolveInspirationPlaygroundModelId', () => {
  test('resolves Seedance 2.5 and 2.0 to hyphenated OmniMux IDs', () => {
    expect(resolveInspirationPlaygroundModelId('seedance-2.5')).toBe('seedance-2-5')
    expect(resolveInspirationPlaygroundModelId('seedance-2-5')).toBe('seedance-2-5')
    expect(resolveInspirationPlaygroundModelId('Seedance-2.5')).toBe('seedance-2-5')
    expect(resolveInspirationPlaygroundModelId('seedance-2.0')).toBe('seedance-2-0')
    expect(resolveInspirationPlaygroundModelId('seedance-2-0')).toBe('seedance-2-0')
  })

  test('resolves Wan 3.0 to wan-3.0', () => {
    expect(resolveInspirationPlaygroundModelId('wan-3.0')).toBe('wan-3.0')
    expect(resolveInspirationPlaygroundModelId('wan-3-0')).toBe('wan-3.0')
  })

  test('resolves GPT Image 2 and 2.5 to gpt-image-2.5', () => {
    expect(resolveInspirationPlaygroundModelId('gpt-image-2')).toBe('gpt-image-2.5')
    expect(resolveInspirationPlaygroundModelId('gpt-image-2.5')).toBe('gpt-image-2.5')
    expect(resolveInspirationPlaygroundModelId('gpt-image-2-5')).toBe('gpt-image-2.5')
    // Exclude flare and sunburst variants: always map to official gpt-image-2.5
    expect(resolveInspirationPlaygroundModelId('gpt-image-2.5-flare')).toBe('gpt-image-2.5')
    expect(resolveInspirationPlaygroundModelId('gpt-image-2.5-sunburst')).toBe('gpt-image-2.5')
    expect(resolveInspirationPlaygroundModelId('image2')).toBe('gpt-image-2.5')
  })

  test('resolves Nano Banana Pro to nano-banana-pro', () => {
    expect(resolveInspirationPlaygroundModelId('nano-banana-pro')).toBe('nano-banana-pro')
  })

  test('resolves Grok Imagine based on mediaType', () => {
    expect(resolveInspirationPlaygroundModelId('grok-imagine', 'video')).toBe(
      'grok-imagine-video-1-5'
    )
    expect(resolveInspirationPlaygroundModelId('grok-imagine', 'image')).toBe(
      'grok-imagine-image-2-0'
    )
    expect(resolveInspirationPlaygroundModelId('grok-imagine')).toBe(
      'grok-imagine-video-1-5'
    )
  })

  test('falls back gracefully to normalized model key for other models', () => {
    expect(resolveInspirationPlaygroundModelId('seedream-5-0-pro')).toBe(
      'seedream-5-0-pro'
    )
    expect(resolveInspirationPlaygroundModelId('  qwen-image-3-0  ')).toBe(
      'qwen-image-3-0'
    )
  })
})
