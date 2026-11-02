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

import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, expect, it, vi } from 'vitest'

import taxonomy from '../../../../../service/influencer/data/taxonomy.json'
import { BuilderPanel } from '../components/builder-panel'
import type { InfluencerTaxonomy } from '../types'

afterEach(cleanup)
it('character type exposes five choices without an illustration caption and shows only cat settings', () => {
  const change = vi.fn()
  const tax = taxonomy as unknown as InfluencerTaxonomy
  render(
    <BuilderPanel
      taxonomy={tax}
      tier='cats'
      selection={{}}
      locks={{}}
      openGroups={new Set(tax.categories.map((c) => c.id))}
      onTier={change}
      onPick={vi.fn()}
      onToggleGroup={vi.fn()}
    />
  )
  expect(
    screen.getByRole('button', { name: /Character type/ })
  ).toBeInTheDocument()
  expect(screen.queryByText('Type illustrations only')).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Dogs' }))
  expect(change).toHaveBeenCalledWith('dogs')
  expect(
    screen.queryByRole('button', { name: /Ethnicity/ })
  ).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Siamese' })).toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Corgi' })
  ).not.toBeInTheDocument()
})

it('marks a hand-picked tile as locked and names that state for assistive tech', () => {
  const tax = taxonomy as unknown as InfluencerTaxonomy
  const cat = tax.categories.find((c) => c.id === 'gender')!
  const option = cat.options.find((o) => o.visibleIn.includes('total'))!
  render(
    <BuilderPanel
      taxonomy={tax}
      tier='total'
      selection={{ gender: [option.id] }}
      locks={{ gender: [option.id] }}
      openGroups={new Set(['gender'])}
      onTier={vi.fn()}
      onPick={vi.fn()}
      onToggleGroup={vi.fn()}
    />
  )
  const tile = screen.getByRole('button', { name: option.label_en })
  expect(tile).toHaveAttribute('data-locked', 'true')
  expect(tile).toHaveAttribute('aria-describedby', 'inf-lock-desc')
  expect(tile.querySelector('.inf-opt-lock')).not.toBeNull()
})

