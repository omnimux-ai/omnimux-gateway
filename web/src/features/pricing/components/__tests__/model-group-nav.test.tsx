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
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import { ModelGroupNav } from '../model-group-nav'
import type { PricingModel } from '../../types'

function mockModel(enableGroups: string[]): PricingModel {
  return {
    id: 1,
    model_name: 'gpt-image-2.5',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: enableGroups,
  }
}

describe('ModelGroupNav', () => {
  const usableGroup = {
    default: 'Standard Default',
    'gpt-image-2.5-economy': 'Economy Tier',
    'gpt-image-2.5-pro': 'Pro Tier',
  }

  const groupRatio = {
    default: 1,
    'gpt-image-2.5-economy': 0.112455,
    'gpt-image-2.5-pro': 0.281248,
  }

  it('renders nothing when the model only has one available group', () => {
    const model = mockModel(['default'])
    const onSelect = vi.fn()
    const { container } = render(
      <ModelGroupNav
        model={model}
        usableGroup={usableGroup}
        groupRatio={groupRatio}
        selectedGroup='default'
        onSelectGroup={onSelect}
      />
    )
    expect(container.firstChild).toBeNull()
  })

  it('renders full raw group identifier tabs when multiple groups are available', () => {
    const model = mockModel([
      'default',
      'gpt-image-2.5-economy',
      'gpt-image-2.5-pro',
    ])
    const onSelect = vi.fn()
    render(
      <ModelGroupNav
        model={model}
        usableGroup={usableGroup}
        groupRatio={groupRatio}
        selectedGroup='gpt-image-2.5-economy'
        onSelectGroup={onSelect}
      />
    )

    expect(screen.getByRole('navigation', { name: /Model versions/i })).toBeInTheDocument()
    // Full raw group identifier names
    expect(screen.getByRole('button', { name: /^default$/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^gpt-image-2\.5-economy$/i })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /^gpt-image-2\.5-pro$/i })).toBeInTheDocument()
  })

  it('fires onSelectGroup when clicking a different group tab', async () => {
    const user = userEvent.setup()
    const model = mockModel([
      'default',
      'gpt-image-2.5-economy',
      'gpt-image-2.5-pro',
    ])
    const onSelect = vi.fn()
    render(
      <ModelGroupNav
        model={model}
        usableGroup={usableGroup}
        groupRatio={groupRatio}
        selectedGroup='default'
        onSelectGroup={onSelect}
      />
    )

    const economyBtn = screen.getByRole('button', { name: /^gpt-image-2\.5-economy$/i })
    await user.click(economyBtn)
    expect(onSelect).toHaveBeenCalledWith('gpt-image-2.5-economy')
  })
})
