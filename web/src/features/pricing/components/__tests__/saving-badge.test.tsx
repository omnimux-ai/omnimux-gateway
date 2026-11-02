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
import i18next from 'i18next'
import { beforeAll, describe, expect, test } from 'vitest'

import { SavingBadge } from '../saving-badge'

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Official 1:1': 'Official 1:1',
    'Save {{percent}}%': 'Save {{percent}}%',
  })
})

describe('SavingBadge', () => {
  test('renders em dash for none kind', () => {
    render(<SavingBadge badge={{ kind: 'none', percent: null }} />)
    expect(screen.getByText('—')).toBeInTheDocument()
  })

  test('renders Official 1:1 capsule for same kind by default', () => {
    render(<SavingBadge badge={{ kind: 'same', percent: 0 }} />)
    const badge = screen.getByText('Official 1:1')
    expect(badge).toBeInTheDocument()
    expect(badge).toHaveClass('rounded-full')
    expect(badge).toHaveAttribute('title', 'Official 1:1')
  })

  test('renders Official 1:1 inline without capsule styles', () => {
    render(
      <SavingBadge
        badge={{ kind: 'same', percent: 0 }}
        variant='inline'
      />
    )
    const badge = screen.getByText('Official 1:1')
    expect(badge).toBeInTheDocument()
    expect(badge).not.toHaveClass('rounded-full')
  })

  test('renders percent + arrow for off kind in inline variant', () => {
    render(
      <SavingBadge
        badge={{ kind: 'off', percent: 30 }}
        variant='inline'
        showArrow
      />
    )
    expect(screen.getByText('30%')).toBeInTheDocument()
    expect(screen.getByText('30%').parentElement).toHaveClass(
      'text-emerald-600'
    )
    expect(
      screen.getByText('30%').parentElement?.querySelector('svg')
    ).not.toBeNull()
  })

  test('renders Save {{percent}}% capsule for off kind by default', () => {
    render(<SavingBadge badge={{ kind: 'off', percent: 25 }} />)
    const label = screen.getByText('Save 25%')
    expect(label).toBeInTheDocument()
    const capsule = label.parentElement
    expect(capsule).toHaveClass('rounded-full')
    expect(capsule).toHaveAttribute('title', 'Save 25%')
  })

  test('hides arrow when showArrow is false on off badge', () => {
    render(
      <SavingBadge
        badge={{ kind: 'off', percent: 18 }}
        variant='inline'
        showArrow={false}
      />
    )
    expect(screen.getByText('18%')).toBeInTheDocument()
    expect(
      screen.getByText('18%').parentElement?.querySelector('svg')
    ).toBeNull()
  })
})
