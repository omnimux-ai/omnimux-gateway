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
import { afterEach, beforeEach, expect, it } from 'vitest'

import { useBuilder } from '../hooks/use-builder'
import { clearSession, writeSession } from '../lib/workbench-session'
import { conflictMessage } from '../lib/taxonomy'
import type { InfluencerTaxonomy } from '../types'

const tax: InfluencerTaxonomy = {
  version: 1,
  source: 'approved animal selection',
  category_priority: [],
  tier_group: {
    options: ['normal', 'freak', 'total', 'cats', 'dogs'].map((id) => ({
      id,
      label_en: id,
      visibleIn: [],
    })),
    comingSoonImageUrls: [],
  },
  categories: [
    {
      id: 'body_type',
      label_en: 'Build',
      kind: 'text',
      max: 1,
      options: [
        {
          id: 'body_heavy',
          label_en: 'Heavy',
          visibleIn: ['total', 'cats', 'dogs'],
        },
      ],
    },
    {
      id: 'height',
      label_en: 'Height',
      kind: 'text',
      max: 1,
      options: [
        {
          id: 'h_tall',
          label_en: 'Tall',
          visibleIn: ['total', 'cats', 'dogs'],
        },
      ],
    },
    {
      id: 'animal_breed',
      label_en: 'Breed',
      kind: 'text',
      max: 1,
      options: [
        { id: 'cat_siamese', label_en: 'Siamese', visibleIn: ['cats'] },
        { id: 'dog_corgi', label_en: 'Corgi', visibleIn: ['dogs'] },
      ],
    },
  ],
  prompt_map: { tiers: {}, categories: {} },
}
beforeEach(() => {
  sessionStorage.clear()
  clearSession()
})
afterEach(cleanup)
it('a tier-only animal session restores natural defaults without falling back to a person', () => {
  writeSession({ version: 1, tier: 'cats', selection: {}, locks: {}, initialized: true })
  const { result } = renderHook(() => useBuilder(tax))
  expect(result.current.state.tier).toBe('cats')
  expect(result.current.state.selection).toEqual({})
})
it('animal manual heavy and tall picks remain legal rather than running human conflicts', () => {
  const { result } = renderHook(() => useBuilder(tax))
  act(() => result.current.switchTier('cats'))
  act(() => result.current.resetFeatures())
  act(() => result.current.pick('body_type', 'body_heavy'))
  act(() => result.current.pick('height', 'h_tall'))
  expect(result.current.state.selection).toEqual({
    body_type: ['body_heavy'],
    height: ['h_tall'],
  })
  expect(conflictMessage('cats', result.current.state.selection)).toBeNull()
})
it('animal randomization keeps manual build and fills the visible tall option without human repair', () => {
  const { result } = renderHook(() => useBuilder(tax))
  act(() => result.current.switchTier('dogs'))
  act(() => result.current.resetFeatures())
  act(() => result.current.pick('body_type', 'body_heavy'))
  act(() => result.current.randomize())
  expect(result.current.state.selection).toEqual({
    body_type: ['body_heavy'],
    height: ['h_tall'],
    animal_breed: ['dog_corgi'],
  })
})
it('a cat cannot select a dog breed through the actual builder pick boundary', () => {
  const { result } = renderHook(() => useBuilder(tax))
  act(() => result.current.switchTier('cats'))
  act(() => result.current.resetFeatures())
  act(() => result.current.pick('animal_breed', 'dog_corgi'))
  expect(result.current.state.selection).toEqual({})
})
it('switching cat to dog removes cat breed, preserves common manual traits, and persists empty animal reset', () => {
  const first = renderHook(() => useBuilder(tax))
  act(() => first.result.current.switchTier('cats'))
  act(() => first.result.current.resetFeatures())
  act(() => first.result.current.pick('animal_breed', 'cat_siamese'))
  act(() => first.result.current.pick('body_type', 'body_heavy'))
  act(() => first.result.current.switchTier('dogs'))
  expect(first.result.current.state.selection).toEqual({
    body_type: ['body_heavy'],
  })
  act(() => first.result.current.randomize())
  expect(first.result.current.state.selection.animal_breed).toEqual([
    'dog_corgi',
  ])
  act(() => first.result.current.resetFeatures())
  first.unmount()
  const next = renderHook(() => useBuilder(tax))
  expect(next.result.current.state.tier).toBe('dogs')
  expect(next.result.current.state.selection).toEqual({})
})
