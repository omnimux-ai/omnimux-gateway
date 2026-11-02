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
import { describe, expect, it, vi } from 'vitest'

import { GenerateBar } from '../components/generate-bar'
import { recreateQuoteFor } from '../lib/recreate'

describe('randomize control', () => {
  it('disables the dice with a reason once every category is locked', () => {
    render(
      <GenerateBar
        quote={{ usd: 0.1, credits: 1 }}
        canGenerate
        submitting={false}
        onRandomize={vi.fn()}
        randomizeDisabled
        randomizeHint='All traits are locked'
        onGenerate={vi.fn()}
      />
    )
    const dice = screen.getByRole('button', { name: 'Randomize' })
    expect(dice).toBeDisabled()
    expect(dice).toHaveAttribute('title', 'All traits are locked')
    expect(dice).toHaveAttribute('aria-disabled', 'true')
  })

  it('keeps the dice enabled while any category is still rollable', () => {
    render(
      <GenerateBar
        quote={{ usd: 0.1, credits: 1 }}
        canGenerate
        submitting={false}
        onRandomize={vi.fn()}
        randomizeDisabled={false}
        onGenerate={vi.fn()}
      />
    )
    const dice = screen.getByRole('button', { name: 'Randomize' })
    expect(dice).toBeEnabled()
    expect(dice).toHaveAttribute('title', 'Randomize')
  })
})

describe('generation submission feedback', () => {
  it.each(['Generate', 'Generate video'])(
    'shows a loading icon and blocks %s while submitting',
    (ctaLabel) => {
      const onGenerate = vi.fn()
      render(
        <GenerateBar
          quote={{ usd: 0.1, credits: 1 }}
          canGenerate
          submitting
          ctaLabel={ctaLabel}
          onGenerate={onGenerate}
        />
      )
      const button = screen.getByRole('button', { name: ctaLabel })
      expect(button).toBeDisabled()
      expect(button).toHaveAttribute('aria-busy', 'true')
      expect(screen.getByRole('status', { name: 'Loading' })).toBeVisible()
      expect(button).not.toHaveTextContent('✦')
      fireEvent.click(button)
      expect(onGenerate).not.toHaveBeenCalled()
    }
  )

  it('names the running step when the action reports a busy label', () => {
    render(
      <GenerateBar
        quote={null}
        canGenerate={false}
        submitting
        ctaLabel='Generate video'
        busyLabel='Analyzing reference video'
        onGenerate={() => {}}
      />
    )
    const button = screen.getByRole('button', {
      name: 'Analyzing reference video',
    })
    expect(button).toBeDisabled()
    expect(button).toHaveTextContent('Analyzing reference video')
    expect(screen.getByRole('status', { name: 'Loading' })).toBeVisible()
  })

  it('restores the label, price and click action when submission ends', () => {
    const onGenerate = vi.fn()
    const props = {
      quote: { usd: 0.1, credits: 1 },
      canGenerate: true,
      onGenerate,
    }
    const { rerender } = render(<GenerateBar {...props} submitting />)
    rerender(<GenerateBar {...props} submitting={false} />)
    const button = screen.getByRole('button', { name: /Generate/ })
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(button).toBeEnabled()
    expect(button).toHaveTextContent('✦ 1')
    fireEvent.click(button)
    expect(onGenerate).toHaveBeenCalledOnce()
  })
})

describe('generation price display', () => {
  it('omits the permanent capacity hint without disabling another submission', () => {
    render(
      <GenerateBar
        quote={null}
        canGenerate
        submitting={false}
        onGenerate={() => {}}
      />
    )
    expect(
      screen.queryByText(
        'Images and videos share 5 generation slots. Additional tasks wait in queue.'
      )
    ).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Generate' })).toBeEnabled()
  })
  it('updates the shown credits when the recreation quality changes', () => {
    const onGenerate = () => {}
    const meta = { pricing: { '480p': 6.47, '720p': 13.89 } }
    const { rerender } = render(
      <GenerateBar
        quote={recreateQuoteFor(meta, '480p')}
        canGenerate
        submitting={false}
        ctaLabel='Generate video'
        onGenerate={onGenerate}
      />
    )
    const button = screen.getByRole('button', { name: /Generate video/ })
    expect(button).toHaveTextContent('✦ 6.47')

    rerender(
      <GenerateBar
        quote={recreateQuoteFor(meta, '720p')}
        canGenerate
        submitting={false}
        ctaLabel='Generate video'
        onGenerate={onGenerate}
      />
    )
    expect(button).toHaveTextContent('✦ 13.89')
  })

  it('shows enough precision for a fractional credit charge', () => {
    render(
      <GenerateBar
        quote={{ usd: 0.00147, credits: 0.0147 }}
        canGenerate
        submitting={false}
        onGenerate={() => {}}
      />
    )
    expect(screen.getByRole('button', { name: /Generate/ })).toHaveTextContent(
      '0.0147'
    )
  })
})
