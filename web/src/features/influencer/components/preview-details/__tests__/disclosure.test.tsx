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

import { cleanup, render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { PreviewDetails } from '../../preview-details'

afterEach(cleanup)

describe('preview details disclosure', () => {
  it('shows nine tags initially, expands on request, and collapses when details change', async () => {
    const tags = Array.from(
      { length: 10 },
      (_, index) => `Parameter ${index + 1}`
    )
    const view = render(<PreviewDetails tags={tags} />)
    expect(screen.queryByText('Parameter 10')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'See all' }))
    expect(screen.getByText('Parameter 10')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Show less' })).toHaveAttribute(
      'aria-expanded',
      'true'
    )
    view.rerender(<PreviewDetails tags={[...tags, 'New parameter']} />)
    expect(screen.queryByText('Parameter 10')).not.toBeInTheDocument()
  })
  it('copies the displayed direction and expands the initially bounded description', async () => {
    const user = userEvent.setup()
    const write = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue()
    render(<PreviewDetails brief='Character direction' />)
    expect(screen.getByText('Character direction')).toHaveStyle({
      maxHeight: '160px',
    })
    await user.click(screen.getByRole('button', { name: 'Copy' }))
    expect(write).toHaveBeenCalledWith('Character direction')
    expect(
      await screen.findByRole('button', { name: 'Copied' })
    ).toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'See all' }))
    expect(screen.getByText('Character direction').style.maxHeight).toBe('')
  })
  it('omits empty direction and parameter sections', () => {
    const view = render(<PreviewDetails />)
    expect(view.container).toBeEmptyDOMElement()
  })
})
