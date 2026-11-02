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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { PickerMediaCard } from '../components/picker-media-card'

function imageCard(
  overrides: Partial<Parameters<typeof PickerMediaCard>[0]> = {}
) {
  return (
    <PickerMediaCard
      title='Character'
      displaySrc='/thumb.webp'
      fallbackSrc='/original.png'
      kind='image'
      selected={false}
      priority
      onSelect={vi.fn()}
      onPreview={vi.fn()}
      {...overrides}
    />
  )
}

describe('PickerMediaCard', () => {
  it('shows the derivative first with an eager decode and a skeleton until load', async () => {
    const view = render(imageCard())
    const image = view.container.querySelector('img') as HTMLImageElement
    expect(image).toHaveAttribute('src', '/thumb.webp')
    expect(image).toHaveAttribute('loading', 'eager')
    expect(image).toHaveAttribute('fetchpriority', 'high')
    expect(image).toHaveAttribute('decoding', 'async')
    expect(view.container.querySelector('.inf-card-skeleton')).not.toBeNull()
    fireEvent.load(image)
    await waitFor(() =>
      expect(view.container.querySelector('.inf-card-skeleton')).toBeNull()
    )
  })

  it('lazy-loads cards below the first rows', () => {
    const view = render(imageCard({ priority: false }))
    const image = view.container.querySelector('img') as HTMLImageElement
    expect(image).toHaveAttribute('loading', 'lazy')
    expect(image).toHaveAttribute('fetchpriority', 'low')
  })

  it('falls back once to the original and then shows a static placeholder', () => {
    const view = render(imageCard())
    const first = view.container.querySelector('img') as HTMLImageElement
    fireEvent.error(first)
    const second = view.container.querySelector('img') as HTMLImageElement
    expect(second).toHaveAttribute('src', '/original.png')
    fireEvent.error(second)
    expect(view.container.querySelector('img')).toBeNull()
    expect(view.container.querySelector('.inf-card-fallback')).not.toBeNull()
  })

  it('renders a video poster, play badge and duration without mounting a video element', () => {
    const view = render(
      imageCard({
        kind: 'video',
        displaySrc: '/poster.webp',
        fallbackSrc: null,
        durationLabel: '00:05',
      })
    )
    expect(view.container.querySelector('img')).toHaveAttribute(
      'src',
      '/poster.webp'
    )
    expect(view.container.querySelector('video')).toBeNull()
    expect(view.container.querySelector('.inf-card-play')).not.toBeNull()
    expect(screen.getByText('00:05')).toBeInTheDocument()
  })

  it('keeps selection state and preview action accessible', () => {
    const onPreview = vi.fn()
    render(imageCard({ selected: true, onPreview }))
    expect(screen.getByRole('button', { name: 'Character' })).toHaveAttribute(
      'aria-pressed',
      'true'
    )
    fireEvent.click(screen.getByRole('button', { name: 'Preview: Character' }))
    expect(onPreview).toHaveBeenCalledOnce()
  })
})
