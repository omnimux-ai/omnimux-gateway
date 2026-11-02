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
import i18next from 'i18next'
import { beforeAll, describe, expect, test, vi } from 'vitest'

import { FILTER_ALL, MODALITY_FILTERS } from '../../constants'
import { PublicPricingToolbar } from '../public-pricing-toolbar'

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Model category': 'Model category',
    ALL: 'ALL',
    IMAGE: 'IMAGE',
    VIDEO: 'VIDEO',
    LLM: 'LLM',
    AUDIO: 'AUDIO',
    'Search models, IDs, or providers…': 'Search models, IDs, or providers…',
    'Search models': 'Search models',
    'Clear search': 'Clear search',
    'All providers': 'All providers',
    model: 'model',
    models: 'models',
    Never: 'Never',
    'Synced {{time}}': 'Synced {{time}}',
    Refresh: 'Refresh',
    'Refresh prices': 'Refresh prices',
  })
})

describe('PublicPricingToolbar', () => {
  test('exposes public modality tabs and vendor filter', async () => {
    const user = userEvent.setup()
    const onModalityChange = vi.fn()
    const onVendorChange = vi.fn()

    render(
      <PublicPricingToolbar
        modality={MODALITY_FILTERS.ALL}
        onModalityChange={onModalityChange}
        search=''
        onSearchChange={vi.fn()}
        onSearchClear={vi.fn()}
        vendorId={FILTER_ALL}
        onVendorChange={onVendorChange}
        vendors={[
          { id: 1, name: 'OpenAI' },
          { id: 2, name: 'Anthropic' },
        ]}
        filteredCount={12}
      />
    )

    const tabs = screen.getAllByRole('tab').map((t) => t.textContent)
    expect(tabs).toEqual(['VIDEO', 'LLM', 'IMAGE', 'AUDIO', 'ALL'])
    expect(screen.queryByRole('tab', { name: /Social/i })).not.toBeInTheDocument()

    await user.click(screen.getByRole('tab', { name: 'IMAGE' }))
    expect(onModalityChange).toHaveBeenCalledWith(MODALITY_FILTERS.IMAGE)

    const vendorSelect = screen.getByLabelText('All providers')
    await user.selectOptions(vendorSelect, '2')
    expect(onVendorChange).toHaveBeenCalledWith('2')

    expect(screen.getByText('12')).toBeInTheDocument()
  })

  test('refresh button shows spinner while refreshing', () => {
    render(
      <PublicPricingToolbar
        modality={MODALITY_FILTERS.ALL}
        onModalityChange={vi.fn()}
        search=''
        onSearchChange={vi.fn()}
        onSearchClear={vi.fn()}
        vendorId={FILTER_ALL}
        onVendorChange={vi.fn()}
        vendors={[]}
        filteredCount={0}
        isRefreshing
        onRefresh={vi.fn()}
      />
    )

    expect(screen.getByRole('button', { name: /Refresh prices/i })).toBeDisabled()
  })
})
