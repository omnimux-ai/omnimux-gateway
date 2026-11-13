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

import {
  BARE_ROW,
  PHOTO_ROW,
  UNPLAYABLE_VIDEO_ROW,
  VIDEO_ROW,
} from '../../api/__tests__/fixtures/tiktok-rows'
import { mapTikTokAdItem } from '../../api/tiktok'
import { TikTokAdCard } from '../tiktok-ad-card'

function card(overrides = VIDEO_ROW) {
  return render(
    <TikTokAdCard item={mapTikTokAdItem(overrides)} onOpen={vi.fn()} />
  )
}

describe('TikTokAdCard metrics', () => {
  it('renders the three metrics the service can answer', () => {
    card()

    expect(screen.getByText('Views')).toBeInTheDocument()
    expect(screen.getByText('95.60M')).toBeInTheDocument()
    expect(screen.getByText('Clip length')).toBeInTheDocument()
    expect(screen.getByText('0:24')).toBeInTheDocument()
    expect(screen.getByText('Traffic')).toBeInTheDocument()
    expect(screen.getByText('Organic')).toBeInTheDocument()
  })

  it('never prints the revenue and ROAS columns the service has no field for', () => {
    card()

    expect(screen.queryByText('Revenue')).not.toBeInTheDocument()
    expect(screen.queryByText('ROAS')).not.toBeInTheDocument()
    expect(screen.queryByText(/^\$/u)).not.toBeInTheDocument()
  })

  it('prints a placeholder instead of a zero clip length for a photo post', () => {
    card(PHOTO_ROW)

    expect(screen.getByText('Clip length')).toBeInTheDocument()
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  it('prints a placeholder for an unknown traffic type rather than a bucket', () => {
    card(BARE_ROW)

    // The payload reported `mixed`, which is outside the documented pair.
    expect(screen.queryByText('Organic')).not.toBeInTheDocument()
    expect(screen.getAllByText('—').length).toBeGreaterThan(0)
  })
})

describe('TikTokAdCard region badge', () => {
  it('shows the region code when the payload carries one', () => {
    card()

    expect(screen.getByText('US')).toBeInTheDocument()
  })

  it('renders no badge text at all when the payload carries no region', () => {
    // 1,323 of the 2,954 rows have no region; an empty pill would be a claim.
    card(PHOTO_ROW)

    expect(screen.queryByText('US')).not.toBeInTheDocument()
    expect(screen.queryByText('DE')).not.toBeInTheDocument()
  })
})

describe('TikTokAdCard playback affordance', () => {
  it('offers the play affordance when the creative has a TikTok player', () => {
    const { container } = card()

    expect(container.querySelector('svg.lucide-play')).not.toBeNull()
  })

  it('offers no play affordance for an mp4 that has no playable player', () => {
    const { container } = card(UNPLAYABLE_VIDEO_ROW)

    // `mediaUrls` still lists the file, but every object under /r2/… is 404, so
    // the badge would promise a player that cannot open.
    expect(container.querySelector('svg.lucide-play')).toBeNull()
  })

  it('offers no play affordance for a photo post', () => {
    const { container } = card(PHOTO_ROW)

    expect(container.querySelector('svg.lucide-play')).toBeNull()
  })
})

describe('TikTokAdCard cover and heading', () => {
  it('renders the real cover image', () => {
    const { container } = card()

    expect(container.querySelector('img')?.getAttribute('src')).toBe(
      '/api/inspiration/v1/public/media/inspiration-covers/9001'
    )
  })

  it('drops the cover image when it fails to load, keeping the backdrop', () => {
    const { container } = card()
    const img = container.querySelector('img')

    expect(img).not.toBeNull()
    fireEvent.error(img as HTMLImageElement)

    expect(container.querySelector('img')).toBeNull()
  })

  it('falls back to the caption when the headline is empty', () => {
    card({ ...BARE_ROW, title: '', caption: 'a real caption' })

    expect(screen.getByText('a real caption')).toBeInTheDocument()
  })
})

describe('TikTokAdCard interaction', () => {
  it('opens the detail layer when the card is activated', async () => {
    const item = mapTikTokAdItem(VIDEO_ROW)
    const onOpen = vi.fn()
    render(<TikTokAdCard item={item} onOpen={onOpen} />)

    await userEvent.click(
      screen.getByRole('button', { name: 'A demo creative' })
    )

    expect(onOpen).toHaveBeenCalledWith(item)
  })

  it('still has an accessible name when the payload has no text at all', () => {
    render(
      <TikTokAdCard
        item={mapTikTokAdItem({ ...BARE_ROW, title: '', caption: '' })}
        onOpen={vi.fn()}
      />
    )

    expect(
      screen.getByRole('button', { name: 'Ad details' })
    ).toBeInTheDocument()
  })
})

describe('TikTokAdCard cover skeleton', () => {
  it('shows a skeleton over the cover box until the image loads', () => {
    const { container } = card()

    expect(screen.getByTestId('tiktok-ad-card-skeleton')).toBeInTheDocument()
    expect(container.querySelector('img')).toHaveClass('opacity-0')
  })

  it('holds the skeleton in the cover box itself, so finishing the load does not move the card', () => {
    card()

    expect(screen.getByTestId('tiktok-ad-card-skeleton')).toHaveClass(
      'absolute',
      'inset-0',
      'size-full'
    )
  })

  it('drops the skeleton and fades the image in once the cover loads', () => {
    const { container } = card()

    fireEvent.load(container.querySelector('img') as HTMLImageElement)

    expect(screen.queryByTestId('tiktok-ad-card-skeleton')).toBeNull()
    const image = container.querySelector('img')
    expect(image).toHaveClass('opacity-100')
    expect(image).toHaveClass('duration-300')
  })

  it('drops the skeleton when the cover fails, leaving the gradient backdrop instead', () => {
    const { container } = card()
    fireEvent.error(container.querySelector('img') as HTMLImageElement)

    // Never stuck pulsing over a card that has nothing to fade in.
    expect(screen.queryByTestId('tiktok-ad-card-skeleton')).toBeNull()
    expect(container.querySelector('img')).toBeNull()
  })
})

describe('TikTokAdCard carries a still and no player', () => {
  it('mounts no player even for a creative that has one, leaving playback to the detail layer', () => {
    const { container } = card()

    // The affordance is a badge, not a player: nothing in the grid may play on
    // its own, and TikTok's own frame belongs to the modal.
    expect(container.querySelector('video')).toBeNull()
    expect(container.querySelector('iframe')).toBeNull()
    expect(container.querySelector('svg.lucide-play')).not.toBeNull()
  })
})
