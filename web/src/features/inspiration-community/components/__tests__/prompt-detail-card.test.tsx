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
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { PromptItem } from '../../types'
import { PromptDetailCard } from '../prompt-detail-card'

const DESCRIPTION =
  'A couple interacting naturally, with subtle micro-expressions.'

const videoItem: PromptItem = {
  id: 'loomi:seedance-2.5:couple-closeup',
  slug: 'couple-closeup',
  modelKey: 'seedance-2.5',
  modelLabel: 'Seedance 2.5',
  mediaType: 'video',
  title: 'Couple close-up, natural micro-expressions',
  description: DESCRIPTION,
  prompt: '8 seconds, 9:16 vertical, cinematic close-up of a couple.',
  category: 'cinematic-showcase',
  useCases: ['cinematic-showcase'],
  styles: ['cinematic-realistic'],
  tags: ['cinematic-showcase'],
  previewImageUrl: 'https://cdn.example.com/cover.jpg',
  previewVideoUrl: 'https://cdn.example.com/clip.mp4',
  posterUrl: 'https://cdn.example.com/cover.jpg',
  authorName: 'Loomi',
  sourcePlatform: 'x',
  sourceUrl: 'https://x.com/example/status/1',
  publishedAt: '2026-09-11T06:35:58',
  viewCount: 1_250_000,
  originalViewCount: 0,
  aspectRatio: '9 / 16',
}

const imageItem: PromptItem = {
  ...videoItem,
  id: 'loomi:gpt-image-2:poster',
  slug: 'poster',
  mediaType: 'image',
  title: 'Product poster',
  previewVideoUrl: '',
  aspectRatio: '1200 / 900',
}

describe('PromptDetailCard browsing tile', () => {
  it('is a single control that opens the detail layer under the item title', async () => {
    const onOpenPreview = vi.fn()
    const user = userEvent.setup()
    render(
      <PromptDetailCard item={videoItem} onOpenPreview={onOpenPreview} />
    )

    const controls = screen.getAllByRole('button')

    expect(controls).toHaveLength(1)
    await user.click(screen.getByRole('button', { name: videoItem.title }))
    expect(onOpenPreview).toHaveBeenCalledWith(videoItem)
  })

  it('opens the detail layer from the keyboard', async () => {
    const onOpenPreview = vi.fn()
    const user = userEvent.setup()
    render(
      <PromptDetailCard item={videoItem} onOpenPreview={onOpenPreview} />
    )

    await user.tab()
    expect(screen.getByRole('button', { name: videoItem.title })).toHaveFocus()

    await user.keyboard('{Enter}')
    expect(onOpenPreview).toHaveBeenCalledWith(videoItem)
  })

  it('reserves the poster aspect ratio so the waterfall columns do not jump while images load', () => {
    const { container } = render(
      <PromptDetailCard item={videoItem} onOpenPreview={vi.fn()} />
    )

    const mediaBox = container.querySelector<HTMLElement>(
      '[style*="aspect-ratio"]'
    )

    expect(mediaBox?.getAttribute('style')).toContain('aspect-ratio')
    expect(mediaBox?.getAttribute('style')).toContain('9 / 16')
  })

  it('keeps the prompt, its copy control and the expander out of the browsing layer', () => {
    render(<PromptDetailCard item={videoItem} onOpenPreview={vi.fn()} />)

    expect(screen.queryByText(videoItem.prompt)).not.toBeInTheDocument()
    expect(screen.queryByText(DESCRIPTION)).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: /show prompt|copy/i })
    ).not.toBeInTheDocument()
  })

  it('captions the tile with the model label and the title', () => {
    render(<PromptDetailCard item={imageItem} onOpenPreview={vi.fn()} />)

    expect(screen.getByText(imageItem.modelLabel)).toBeInTheDocument()
    expect(screen.getByText(imageItem.title)).toBeInTheDocument()
  })

  it('renders a still cover in an image element', () => {
    const { container } = render(
      <PromptDetailCard item={imageItem} onOpenPreview={vi.fn()} />
    )

    const image = container.querySelector('img')
    expect(image).toHaveAttribute('src', imageItem.posterUrl)
    expect(container.querySelector('video')).toBeNull()
  })

  it('renders a cover that is really a video file as no media element at all', () => {
    const videoCoverItem: PromptItem = {
      ...videoItem,
      posterUrl: 'https://cdn.example.com/clip-cover.mp4',
    }
    const { container } = render(
      <PromptDetailCard item={videoCoverItem} onOpenPreview={vi.fn()} />
    )

    // The tile is a browsing surface: it never mounts a player, and an `<img>`
    // pointed at an `.mp4` would draw a broken frame, so the cover is dropped
    // and the box keeps its flat backdrop.
    expect(container.querySelector('video')).toBeNull()
    expect(container.querySelector('img')).toBeNull()

    const mediaBox = container.querySelector<HTMLElement>(
      '[style*="aspect-ratio"]'
    )
    expect(mediaBox?.className).toContain('bg-muted')
    expect(mediaBox?.getAttribute('style')).toContain('9 / 16')
  })
})

describe('PromptDetailCard cover skeleton', () => {
  it('shows a skeleton over the cover box until the image loads', () => {
    const { container } = render(
      <PromptDetailCard item={imageItem} onOpenPreview={vi.fn()} />
    )

    expect(screen.getByTestId('prompt-card-skeleton')).toBeInTheDocument()
    expect(container.querySelector('img')).toHaveClass('opacity-0')
  })

  it('holds the skeleton in the cover box itself, so finishing the load does not move the tile', () => {
    render(<PromptDetailCard item={imageItem} onOpenPreview={vi.fn()} />)

    const skeleton = screen.getByTestId('prompt-card-skeleton')

    // Same box as the cover, clipped by the tile's own rounded corner.
    expect(skeleton).toHaveClass('absolute', 'inset-0', 'size-full')
  })

  it('drops the skeleton and fades the image in once the cover loads', () => {
    const { container } = render(
      <PromptDetailCard item={imageItem} onOpenPreview={vi.fn()} />
    )

    fireEvent.load(container.querySelector('img') as HTMLImageElement)

    expect(screen.queryByTestId('prompt-card-skeleton')).toBeNull()
    const image = container.querySelector('img')
    expect(image).toHaveClass('opacity-100')
    expect(image).toHaveClass('duration-300')
  })

  it('drops the skeleton when the cover fails, leaving the box backdrop instead', () => {
    const { container } = render(
      <PromptDetailCard item={imageItem} onOpenPreview={vi.fn()} />
    )

    fireEvent.error(container.querySelector('img') as HTMLImageElement)

    // Never stuck pulsing over a tile that has nothing to fade in.
    expect(screen.queryByTestId('prompt-card-skeleton')).toBeNull()
    const mediaBox = container.querySelector<HTMLElement>(
      '[style*="aspect-ratio"]'
    )
    expect(mediaBox?.className).toContain('bg-muted')
  })
})
