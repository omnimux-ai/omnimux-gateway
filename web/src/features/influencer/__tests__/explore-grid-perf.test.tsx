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
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { ExplorePresetGrid } from '../components/explore-preset-grid'
import { clearPrefetchCache, isPrefetched } from '../lib/prefetch'
import type { ExplorePreset } from '../types'

function makePreset(id: string): ExplorePreset {
  return {
    id,
    name: `Preset ${id}`,
    tier: 'average',
    selection: { gender: ['male'] },
    brief: 'brief',
    preview: { path: `/preview-${id}.webp`, width: 400, height: 600 },
    sheet: { path: `/sheet-${id}.webp`, width: 800, height: 600 },
  }
}

describe('ExplorePresetGrid Performance & Interactivity', () => {
  beforeEach(() => {
    clearPrefetchCache()
  })

  afterEach(() => {
    clearPrefetchCache()
  })

  it('uses a derivative once, preserves original preview actions, and resets for changed media', () => {
    const preset = {
      ...makePreset('thumb'),
      thumbnail: { path: '/thumb.webp', width: 640, height: 960 },
    }
    const onPreview = vi.fn()
    const props = { presets: [preset], onPreview, onApply: vi.fn() }
    const view = render(<ExplorePresetGrid {...props} />)
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      '/thumb.webp'
    )
    fireEvent.mouseEnter(view.container.querySelector('article') as HTMLElement)
    expect(isPrefetched(preset.sheet.path)).toBe(true)
    fireEvent.click(screen.getByLabelText(preset.name))
    expect(onPreview).toHaveBeenCalledWith(preset)
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      preset.preview.path
    )
    fireEvent.error(view.container.querySelector('img') as HTMLImageElement)
    expect(view.container.querySelector('img')).toBeNull()
    view.rerender(
      <ExplorePresetGrid
        {...props}
        presets={[
          {
            ...preset,
            thumbnail: { ...preset.thumbnail, path: '/new-thumb.webp' },
          },
        ]}
      />
    )
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      '/new-thumb.webp'
    )
  })

  it('reserves the oriented derivative ratio even after falling back to its rotated original', () => {
    const preset = {
      ...makePreset('rotated'),
      preview: { path: '/rotated.jpg', width: 1920, height: 1080 },
      thumbnail: { path: '/oriented.webp', width: 640, height: 1138 },
    }
    const view = render(
      <ExplorePresetGrid
        presets={[preset]}
        onPreview={vi.fn()}
        onApply={vi.fn()}
      />
    )
    const thumbnail = view.container.querySelector('img') as HTMLImageElement
    expect(thumbnail).toHaveAttribute('width', '640')
    expect(thumbnail).toHaveAttribute('height', '1138')
    expect(thumbnail).toHaveStyle({ aspectRatio: '640 / 1138' })
    fireEvent.error(thumbnail)
    const original = view.container.querySelector('img') as HTMLImageElement
    expect(original).toHaveAttribute('src', '/rotated.jpg')
    expect(original).toHaveAttribute('width', '640')
    expect(original).toHaveAttribute('height', '1138')
    expect(original).toHaveStyle({ aspectRatio: '640 / 1138' })
  })

  it('marks first 4 cards with eager loading and high fetchPriority', () => {
    const presets = Array.from({ length: 6 }, (_, i) => makePreset(`p${i}`))
    const { container } = render(
      <ExplorePresetGrid
        presets={presets}
        onPreview={vi.fn()}
        onApply={vi.fn()}
      />
    )

    const images = container.querySelectorAll('img')
    expect(images).toHaveLength(6)

    // First 4 should have eager and high priority
    for (let i = 0; i < 4; i++) {
      expect(images[i].getAttribute('loading')).toBe('eager')
      expect(images[i].getAttribute('fetchpriority')).toBe('high')
    }

    // Subsequent should have lazy loading and auto priority
    for (let i = 4; i < 6; i++) {
      expect(images[i].getAttribute('loading')).toBe('lazy')
      expect(images[i].getAttribute('fetchpriority')).toBe('auto')
    }
  })

  it('hides a card completely when its preview image fails to load', () => {
    const presets = [makePreset('p1'), makePreset('p2')]
    const { container } = render(
      <ExplorePresetGrid
        presets={presets}
        onPreview={vi.fn()}
        onApply={vi.fn()}
      />
    )

    expect(screen.getByLabelText('Preset p1')).toBeInTheDocument()
    expect(screen.getByLabelText('Preset p2')).toBeInTheDocument()

    const firstImage = container.querySelectorAll('img')[0]
    fireEvent.error(firstImage)

    expect(screen.queryByLabelText('Preset p1')).not.toBeInTheDocument()
    expect(screen.getByLabelText('Preset p2')).toBeInTheDocument()
  })

  it('prefetches the sheet image when mouse hovers over a preset card', () => {
    const presets = [makePreset('p1')]
    const { container } = render(
      <ExplorePresetGrid
        presets={presets}
        onPreview={vi.fn()}
        onApply={vi.fn()}
      />
    )

    expect(isPrefetched('/sheet-p1.webp')).toBe(false)

    const card = container.querySelector('article')
    expect(card).not.toBeNull()
    if (card) {
      fireEvent.mouseEnter(card)
    }

    expect(isPrefetched('/sheet-p1.webp')).toBe(true)
  })

  it('prefetches the sheet image when card receives keyboard focus', () => {
    const presets = [makePreset('p2')]
    render(
      <ExplorePresetGrid
        presets={presets}
        onPreview={vi.fn()}
        onApply={vi.fn()}
      />
    )

    expect(isPrefetched('/sheet-p2.webp')).toBe(false)

    const button = screen.getByLabelText('Preset p2')
    fireEvent.focus(button)

    expect(isPrefetched('/sheet-p2.webp')).toBe(true)
  })
})
