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

import { beforeEach, describe, expect, it, vi } from 'vitest'

import { submitSheet } from '../api'

const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: { post } }))

describe('character sheet submission', () => {
  beforeEach(() => {
    post.mockReset()
    post.mockResolvedValue({ data: { task_id: 'sheet-1' } })
  })

  it.each(['text', 'reference', 'edit', 'masked_edit'] as const)(
    'normalizes legacy %s intent to style reference and strips edit fields',
    async (image_mode) => {
      const body = {
        model: 'gpt-image-2.5-sunburst',
        group: 'gpt-image-2.5-sunburst-image',
        tier: 'normal',
        seed: 42,
        selection: { body: ['curvy'] },
        brief: 'Character',
        size: '9:16',
        image_mode,
        image_url: '  https://example.com/owned-source.png  ',
        edit_instruction: 'Change the background',
        mask_url: 'https://example.com/owned-mask.png',
      }
      await expect(submitSheet(body)).resolves.toBe('sheet-1')
      expect(post.mock.calls[0]?.[1]).toEqual({
        model: body.model,
        group: body.group,
        tier: body.tier,
        selection: body.selection,
        brief: '',
        size: '1024x576',
        image_mode: 'reference',
        image_url: 'https://example.com/owned-source.png',
      })
      expect(post.mock.calls[0]?.[1]).not.toHaveProperty('seed')
      expect(post.mock.calls[0]?.[1]).not.toHaveProperty('mask_url')
      expect(post.mock.calls[0]?.[1]).not.toHaveProperty('edit_instruction')
      expect(body.seed).toBe(42)
      expect(body.size).toBe('9:16')
      expect(body.edit_instruction).toBe('Change the background')
      expect(body.brief).toBe('Character')
    }
  )
  it.each([undefined, '', '   '])(
    'uses text intent without a valid image URL (%s) and never converts edit text to the brief',
    async (image_url) => {
      await submitSheet({
        model: 'gpt-image-2.5-sunburst',
        tier: 'normal',
        selection: {},
        brief: '',
        size: '16:9',
        image_mode: 'masked_edit',
        image_url,
        mask_url: 'https://example.com/owned-mask.png',
        edit_instruction: 'Change the background',
      })
      expect(post.mock.calls[0]?.[1]).toMatchObject({
        image_mode: 'text',
        image_url: undefined,
        brief: '',
      })
      expect(post.mock.calls[0]?.[1]).not.toHaveProperty('mask_url')
      expect(post.mock.calls[0]?.[1]).not.toHaveProperty('edit_instruction')
    }
  )
  it.each(['9:16', '1024x1536', '16:9', ''])(
    'keeps the audited economy route on the 16:9 canvas without workbench-only fields for size %s',
    async (size) => {
      const body = {
        model: 'gpt-image-2.5',
        group: 'gpt-image-2.5-economy',
        tier: 'total',
        selection: { gender: ['female'] },
        brief: 'red coat',
        seed: 42,
        image_url: 'https://example.com/reference.png',
        size,
      }

      await expect(submitSheet(body)).resolves.toBe('sheet-1')

      expect(post).toHaveBeenCalledWith(
        '/pg/influencer/sheet',
        { ...body, brief: '', size: '16:9' },
        {
          headers: { 'X-Omnimux-Group': 'gpt-image-2.5-economy' },
          skipErrorHandler: true,
        }
      )
      expect(post.mock.calls[0]?.[1]).not.toHaveProperty('image_mode')
      expect(body.size).toBe(size)
    }
  )

  it('keeps the reviewed Sunburst request target and mode for the preferred route', async () => {
    const body = {
      model: 'gpt-image-2.5-sunburst',
      group: 'gpt-image-2.5-sunburst-image',
      tier: 'normal',
      selection: { body: ['curvy'] },
      brief: '',
      seed: 42,
      image_url: 'https://example.com/reference.png',
      size: '16:9',
    }

    await expect(submitSheet(body)).resolves.toBe('sheet-1')

    expect(post.mock.calls[0]?.[1]).toMatchObject({
      model: 'gpt-image-2.5-sunburst',
      group: 'gpt-image-2.5-sunburst-image',
      size: '1024x576',
      image_mode: 'reference',
    })
    expect(post.mock.calls[0]?.[1]).not.toHaveProperty('seed')
  })
})
