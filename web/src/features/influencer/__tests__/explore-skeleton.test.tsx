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

import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'

import { ExploreSkeleton } from '../components/explore-skeleton'

describe('ExploreSkeleton', () => {
  it('renders a status role with loading accessibility label', () => {
    const { container } = render(<ExploreSkeleton />)
    const status = screen.getByRole('status')
    expect(status).toBeInTheDocument()
    expect(status).toHaveAttribute('aria-label')
    expect(container.querySelectorAll('.inf-shimmer')).toHaveLength(8)
  })

  it('renders a custom count of skeleton cards', () => {
    const { container } = render(<ExploreSkeleton count={4} />)
    expect(container.querySelectorAll('.inf-shimmer')).toHaveLength(4)
  })
})
