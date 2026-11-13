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

import { beforeEach, describe, expect, it } from 'vitest'

import { takeGuestDraft, writeGuestDraft } from '../lib/guest'

const KEY = 'influencer:guest-draft:v1'

beforeEach(() => {
  sessionStorage.clear()
})

describe('guest draft', () => {
  it('round-trips the assembled look', () => {
    writeGuestDraft({
      tier: 'freak',
      selection: { body: ['curvy'] },
      brief: 'night market',
      seed: '42',
      imageUrl: 'https://example.com/ref.webp',
      imageMode: 'reference',
      model: 'gpt-image-2.5-sunburst',
      group: 'gpt-image-2.5-sunburst-image',
      vendorId: 7,
    })
    const draft = takeGuestDraft()
    expect(draft).toEqual({
      tier: 'freak',
      selection: { body: ['curvy'] },
      brief: '',
      seed: '42',
      imageUrl: 'https://example.com/ref.webp',
      imageMode: 'reference',
      model: 'gpt-image-2.5-sunburst',
      group: 'gpt-image-2.5-sunburst-image',
      vendorId: 7,
    })
  })

  it.each(['edit', 'masked_edit'] as const)(
    'normalizes a legacy %s login draft from its actual image without restoring editing instructions',
    (imageMode) => {
      sessionStorage.setItem(
        KEY,
        JSON.stringify({
          selection: {},
          brief: 'A new character',
          imageUrl: 'https://example.com/ref.webp',
          imageMode,
          maskUrl: 'https://example.com/mask.png',
          editInstruction: 'Change the background',
        })
      )
      expect(takeGuestDraft()).toMatchObject({
        imageMode: 'reference',
        brief: '',
      })
      sessionStorage.setItem(KEY, JSON.stringify({ imageMode, imageUrl: '  ' }))
      const emptyDraft = takeGuestDraft()
      expect(emptyDraft?.imageMode).toBe('text')
      expect(emptyDraft?.brief).toBe('')
      expect(emptyDraft).not.toHaveProperty('maskUrl')
      expect(emptyDraft).not.toHaveProperty('editInstruction')
    }
  )

  it('writes only ordinary creation intent even if a caller holds legacy editing fields', () => {
    writeGuestDraft({
      tier: 'total',
      selection: {},
      brief: 'A new character',
      seed: '',
      imageUrl: ' https://example.com/style.png ',
      imageMode: 'masked_edit',
      maskUrl: 'https://example.com/mask.png',
      editInstruction: 'Change the cup',
      model: '',
      group: '',
      vendorId: null,
    })
    const stored = JSON.parse(sessionStorage.getItem(KEY) ?? '{}')
    expect(stored).toMatchObject({
      imageUrl: 'https://example.com/style.png',
      imageMode: 'reference',
      brief: '',
    })
    expect(stored).not.toHaveProperty('maskUrl')
    expect(stored).not.toHaveProperty('editInstruction')
  })

  it('is consumed once — a second read returns null', () => {
    writeGuestDraft({
      tier: 'total',
      selection: {},
      brief: '',
      seed: '',
      imageUrl: '',
      model: '',
      group: '',
      vendorId: null,
    })
    expect(takeGuestDraft()).not.toBeNull()
    expect(takeGuestDraft()).toBeNull()
  })

  it('returns null when nothing was saved', () => {
    expect(takeGuestDraft()).toBeNull()
  })

  it('tolerates corrupt stored json without throwing', () => {
    sessionStorage.setItem(KEY, 'not-json{{{')
    expect(takeGuestDraft()).toBeNull()
  })

  it('fills missing fields with safe defaults on partial data', () => {
    sessionStorage.setItem(KEY, JSON.stringify({ brief: 'x' }))
    const draft = takeGuestDraft()
    expect(draft?.brief).toBe('')
    expect(draft?.tier).toBe('total')
    expect(draft?.selection).toEqual({})
    expect(draft?.vendorId).toBeNull()
  })
})
