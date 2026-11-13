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
import { StrictMode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useBuilder } from '../hooks/use-builder'
import { rollableCount } from '../lib/randomize'
import { clearSession } from '../lib/workbench-session'
import type { InfluencerTaxonomy } from '../types'

const LEGACY_KEY = 'influencer:character-draft:v1'
const tax: InfluencerTaxonomy = {
  version: 1,
  source: 'fixture',
  category_priority: [],
  tier_group: {
    options: ['total', 'normal'].map((id) => ({
      id,
      label_en: id,
      visibleIn: [],
    })),
    comingSoonImageUrls: [],
  },
  categories: [
    {
      id: 'gender',
      label_en: 'Gender',
      kind: 'media',
      max: 1,
      options: ['male', 'female'].map((id) => ({
        id,
        label_en: id,
        visibleIn: ['normal', 'total'],
      })),
    },
    {
      id: 'distinctive',
      label_en: 'Distinctive',
      kind: 'text',
      max: 3,
      options: ['scar', 'piercing', 'tattoo'].map((id) => ({
        id,
        label_en: id,
        visibleIn: ['normal', 'total'],
      })),
    },
    {
      id: 'hair',
      label_en: 'Hair',
      kind: 'text',
      max: 1,
      options: ['bob', 'long'].map((id) => ({
        id,
        label_en: id,
        visibleIn: ['total'],
      })),
    },
  ],
  prompt_map: { tiers: {}, categories: {} },
}
beforeEach(() => {
  clearSession()
  sessionStorage.clear()
  localStorage.clear()
})
afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

describe('character look ownership', () => {
  it('initializes one visible random look in Extreme when the session is empty', () => {
    const { result, rerender } = renderHook(({ data }) => useBuilder(data), {
      initialProps: { data: null as InfluencerTaxonomy | null },
      wrapper: StrictMode,
    })
    expect(result.current.state.selection).toEqual({})
    rerender({ data: tax })
    expect(result.current.state.tier).toBe('total')
    expect(Object.keys(result.current.state.selection)).toEqual(
      expect.arrayContaining(['gender', 'distinctive', 'hair'])
    )
    expect(result.current.locks).toEqual({})
    const first = result.current.state.selection
    rerender({ data: { ...tax } })
    expect(result.current.state.selection).toEqual(first)
  })

  it('keeps a manual single choice locked through repeated random rolls', () => {
    const { result } = renderHook(() => useBuilder(tax))
    act(() => result.current.resetFeatures())
    act(() => result.current.pick('gender', 'female'))
    for (let i = 0; i < 4; i++) {
      act(() => result.current.randomize())
      expect(result.current.state.selection.gender).toEqual(['female'])
    }
  })

  it('locks only the manual multi choice instead of also locking an earlier random one', () => {
    vi.spyOn(Math, 'random').mockReturnValue(0)
    const { result } = renderHook(() => useBuilder(tax))
    expect(result.current.state.selection.distinctive).toEqual(['scar'])
    act(() => result.current.pick('distinctive', 'piercing'))
    act(() => result.current.pick('distinctive', 'tattoo'))
    act(() => result.current.randomize())
    expect(result.current.state.selection.distinctive).toEqual([
      'piercing',
      'tattoo',
    ])
    expect(Object.keys(result.current.locks)).toEqual(['distinctive'])
  })

  it('keeps the look and locks on a route remount without writing storage', () => {
    const h = renderHook(() => useBuilder(tax))
    act(() => h.result.current.resetFeatures())
    act(() => h.result.current.pick('gender', 'female'))
    act(() => h.result.current.randomize())
    const before = h.result.current.state.selection
    h.unmount()
    expect(sessionStorage.getItem(LEGACY_KEY)).toBeNull()
    const next = renderHook(() => useBuilder(tax))
    expect(next.result.current.state.selection).toEqual(before)
    act(() => next.result.current.randomize())
    expect(next.result.current.state.selection.gender).toEqual(['female'])
  })

  it('starts a brand new look when the page loads again', () => {
    const h = renderHook(() => useBuilder(tax))
    act(() => h.result.current.pick('gender', 'female'))
    h.unmount()
    // A reload re-evaluates the session module: nothing survives.
    clearSession()
    const next = renderHook(() => useBuilder(tax))
    expect(next.result.current.locks).toEqual({})
    expect(next.result.current.state.tier).toBe('total')
  })

  it('returns a category to the random pool after its last manual option is cancelled', () => {
    const { result } = renderHook(() => useBuilder(tax))
    act(() => result.current.resetFeatures())
    act(() => result.current.pick('gender', 'female'))
    act(() => result.current.pick('gender', 'female'))
    expect(result.current.locks).toEqual({})
    vi.spyOn(Math, 'random').mockReturnValue(0)
    act(() => result.current.randomize())
    expect(result.current.state.selection.gender).toEqual(['male'])
  })

  it('keeps an intentional feature reset across a remount', () => {
    const h = renderHook(() => useBuilder(tax))
    act(() => {
      h.result.current.switchTier('normal')
      h.result.current.setSeed('42')
      h.result.current.setImageUrl('https://files.example/ref.png')
    })
    act(() => h.result.current.resetFeatures())
    expect(h.result.current.state).toMatchObject({
      tier: 'normal',
      selection: {},
      brief: '',
      seed: '42',
      imageUrl: 'https://files.example/ref.png',
    })
    h.unmount()
    const next = renderHook(() => useBuilder(tax))
    expect(next.result.current.state.selection).toEqual({})
    expect(next.result.current.state.tier).toBe('normal')
    act(() => next.result.current.randomize())
    expect(next.result.current.state.selection.gender).toHaveLength(1)
  })

  it('does not overwrite a whole look applied before taxonomy loads', () => {
    const { result, rerender } = renderHook(({ data }) => useBuilder(data), {
      initialProps: { data: null as InfluencerTaxonomy | null },
    })
    act(() =>
      result.current.applyLook({
        tier: 'normal',
        values: { gender: ['female'] },
      })
    )
    rerender({ data: tax })
    expect(result.current.state.selection).toEqual({ gender: ['female'] })
  })

  it('applies a whole look without claiming any category for the dice', () => {
    const { result } = renderHook(() => useBuilder(tax))
    act(() =>
      result.current.applyLook({
        tier: 'total',
        values: { gender: ['female'], hair: ['bob'] },
      })
    )
    expect(result.current.locks).toEqual({})
    vi.spyOn(Math, 'random').mockReturnValue(0)
    act(() => result.current.randomize())
    expect(result.current.state.selection.gender).toEqual(['male'])
    expect(result.current.state.selection.hair).toEqual(['bob'])
  })

  it('keeps only the locks a carried draft recorded', () => {
    const { result } = renderHook(() => useBuilder(tax))
    act(() =>
      result.current.applyLook({
        tier: 'total',
        values: { gender: ['female'], hair: ['bob'] },
        locks: { gender: ['female'] },
      })
    )
    expect(Object.keys(result.current.locks)).toEqual(['gender'])
    vi.spyOn(Math, 'random').mockReturnValue(0)
    act(() => result.current.randomize())
    expect(result.current.state.selection.gender).toEqual(['female'])
    expect(result.current.state.selection.hair).toEqual(['bob'])
  })

  it('cleans removed options on a taxonomy refresh without rerolling valid picks', () => {
    const { result, rerender } = renderHook(({ data }) => useBuilder(data), {
      initialProps: { data: tax },
    })
    act(() =>
      result.current.applyLook({
        values: { gender: ['female'], hair: ['bob'] },
      })
    )
    const refreshed = {
      ...tax,
      categories: tax.categories.map((c) =>
        c.id === 'hair'
          ? { ...c, options: c.options.filter((o) => o.id !== 'bob') }
          : c
      ),
    }
    rerender({ data: refreshed })
    expect(result.current.state.selection).toEqual({ gender: ['female'] })
    expect(result.current.locks).toEqual({})
  })

  it('drops a lock hidden by the new tier while keeping the other locks', () => {
    const { result } = renderHook(() => useBuilder(tax))
    act(() => result.current.resetFeatures())
    act(() => result.current.pick('gender', 'female'))
    act(() => result.current.pick('hair', 'bob'))
    expect(Object.keys(result.current.locks).sort()).toEqual(['gender', 'hair'])
    act(() => result.current.switchTier('normal'))
    expect(Object.keys(result.current.locks)).toEqual(['gender'])
    act(() => result.current.switchTier('total'))
    vi.spyOn(Math, 'random').mockReturnValue(0.9)
    act(() => result.current.randomize())
    expect(result.current.state.selection.hair).toEqual(['long'])
    expect(result.current.state.selection.gender).toEqual(['female'])
  })

  it('rolls around a locked conflict instead of overwriting the lock', () => {
    const conflictTax: InfluencerTaxonomy = {
      ...tax,
      categories: [
        ...tax.categories,
        {
          id: 'height',
          label_en: 'Height',
          kind: 'text' as const,
          max: 1,
          options: [{ id: 'h_tall', label_en: 'Tall', visibleIn: ['total'] }],
        },
        {
          id: 'body_type',
          label_en: 'Build',
          kind: 'text' as const,
          max: 1,
          options: ['body_heavy', 'body_slim'].map((id) => ({
            id,
            label_en: id,
            visibleIn: ['total'],
          })),
        },
      ],
    }
    const { result } = renderHook(() => useBuilder(conflictTax))
    act(() => result.current.resetFeatures())
    act(() => result.current.pick('height', 'h_tall'))
    let message: string | null = null
    act(() => {
      message = result.current.pick('body_type', 'body_heavy')
    })
    expect(message).not.toBeNull()
    act(() => result.current.randomize())
    expect(result.current.state.selection.height).toEqual(['h_tall'])
    expect(result.current.state.selection.body_type).toEqual(['body_slim'])
  })

  it('reports no rollable category once every visible category is locked', () => {
    const { result } = renderHook(() => useBuilder(tax))
    act(() => result.current.resetFeatures())
    act(() => result.current.pick('gender', 'female'))
    expect(
      rollableCount(tax, result.current.state.tier, result.current.locks)
    ).toBe(2)
    act(() => result.current.pick('distinctive', 'scar'))
    act(() => result.current.pick('hair', 'long'))
    expect(
      rollableCount(tax, result.current.state.tier, result.current.locks)
    ).toBe(0)
  })

  it('drops the previous release session record instead of restoring it', () => {
    sessionStorage.setItem(
      LEGACY_KEY,
      JSON.stringify({
        version: 1,
        tier: 'normal',
        initialized: true,
        selection: { gender: ['female'] },
        manualSelection: { gender: ['female'] },
      })
    )
    const { result } = renderHook(() => useBuilder(tax))
    expect(sessionStorage.getItem(LEGACY_KEY)).toBeNull()
    expect(result.current.state.tier).toBe('total')
    expect(result.current.locks).toEqual({})
    expect(result.current.state.selection.gender).toHaveLength(1)
  })

  it('keeps editing usable when the browser blocks session storage', () => {
    vi.spyOn(Storage.prototype, 'removeItem').mockImplementation(() => {
      throw new Error('storage disabled')
    })
    const { result } = renderHook(() => useBuilder(tax))
    act(() => result.current.resetFeatures())
    act(() => result.current.pick('gender', 'female'))
    act(() => result.current.randomize())
    expect(result.current.state.selection.gender).toEqual(['female'])
  })
})
