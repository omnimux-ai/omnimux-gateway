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
import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { useBuilder } from '../hooks/use-builder'
import { parseTaskMeta } from '../lib/history'

describe('automatic style reference state', () => {
  it('derives text and reference intent from the actual source URL', () => {
    const { result } = renderHook(() => useBuilder(null))
    expect(result.current.state.imageMode).toBe('text')
    act(() => result.current.setImageUrl('https://example.com/style.png'))
    expect(result.current.state.imageMode).toBe('reference')
    act(() => result.current.setImageUrl('   '))
    expect(result.current.state.imageMode).toBe('text')
  })

  it.each(['edit', 'masked_edit'] as const)(
    'restores old %s input from a whole look as plain style inspiration',
    () => {
      const { result } = renderHook(() => useBuilder(null))
      act(() =>
        result.current.applyLook({
          tier: 'normal',
          values: {},
          image_url: 'https://example.com/source.png',
        })
      )
      expect(result.current.state).toMatchObject({
        imageMode: 'reference',
        imageUrl: 'https://example.com/source.png',
        brief: '',
      })
      expect(result.current.state).not.toHaveProperty('maskUrl')
      expect(result.current.state).not.toHaveProperty('editInstruction')
      act(() => result.current.setImageUrl(''))
      expect(result.current.state.imageMode).toBe('text')
    }
  )

  it('reads the same edit fields from a native historical task', () => {
    expect(
      parseTaskMeta({
        id: 1,
        task_id: 'edited',
        status: 'SUCCESS',
        properties: {
          input: JSON.stringify({
            tier: 'normal',
            selection: {},
            brief: 'Original historical description',
            image_mode: 'masked_edit',
            image_url: 'https://example.com/source.png',
            mask_url: 'https://example.com/mask.png',
            edit_instruction: 'Change the cup',
          }),
        },
      })
    ).toMatchObject({
      brief: 'Original historical description',
      image_mode: 'masked_edit',
      mask_url: 'https://example.com/mask.png',
      edit_instruction: 'Change the cup',
    })
  })
})
