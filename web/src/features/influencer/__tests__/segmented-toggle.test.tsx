/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
License, or (at your option) any later version.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { SegmentedToggle, type SegmentedOption } from '../components/segmented-toggle'

type TabKey = 'tab1' | 'tab2'

const MOCK_OPTIONS: SegmentedOption<TabKey>[] = [
  { value: 'tab1', label: 'First Tab', icon: <span data-testid='icon-1'>1</span> },
  { value: 'tab2', label: 'Second Tab', icon: <span data-testid='icon-2'>2</span> },
]

describe('SegmentedToggle hierarchy and styling', () => {
  it('renders primary variant with large rounded rect and without outer border', () => {
    const onChange = vi.fn()
    render(
      <SegmentedToggle
        value='tab1'
        onChange={onChange}
        options={MOCK_OPTIONS}
        ariaLabel='Primary group'
        variant='primary'
      />
    )

    const group = screen.getByRole('radiogroup', { name: 'Primary group' })
    expect(group).toHaveClass('rounded-[10px]')
    expect(group.className).not.toContain('border')

    const firstRadio = screen.getByRole('radio', { name: 'First Tab' })
    expect(firstRadio).toHaveClass('rounded-[8px]')
    expect(firstRadio).toHaveAttribute('aria-checked', 'true')
  })

  it('renders pill variant with fully rounded capsule and without outer border', () => {
    const onChange = vi.fn()
    render(
      <SegmentedToggle
        value='tab2'
        onChange={onChange}
        options={MOCK_OPTIONS}
        ariaLabel='Pill group'
        variant='pill'
      />
    )

    const group = screen.getByRole('radiogroup', { name: 'Pill group' })
    expect(group).toHaveClass('rounded-full')
    expect(group.className).not.toContain('border')

    const secondRadio = screen.getByRole('radio', { name: 'Second Tab' })
    expect(secondRadio).toHaveClass('rounded-full')
    expect(secondRadio).toHaveAttribute('aria-checked', 'true')
  })

  it('renders icon-only mode hiding visible text while preserving accessibility labels', async () => {
    const user = userEvent.setup()
    const onChange = vi.fn()
    render(
      <SegmentedToggle
        value='tab1'
        onChange={onChange}
        options={MOCK_OPTIONS}
        ariaLabel='Icon group'
        variant='pill'
        iconOnly
      />
    )

    const firstRadio = screen.getByRole('radio', { name: 'First Tab' })
    expect(firstRadio).toHaveClass('h-7', 'w-7')
    expect(firstRadio).toHaveAttribute('title', 'First Tab')
    expect(screen.queryByText('First Tab')).not.toBeInTheDocument()
    expect(screen.getByTestId('icon-1')).toBeInTheDocument()

    const secondRadio = screen.getByRole('radio', { name: 'Second Tab' })
    await user.click(secondRadio)
    expect(onChange).toHaveBeenCalledWith('tab2')
  })
})
