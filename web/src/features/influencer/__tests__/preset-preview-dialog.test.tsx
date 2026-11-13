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
import { describe, expect, it, vi } from 'vitest'

import { PresetPreviewDialog } from '../components/preset-preview-dialog'
import type { ExplorePreset } from '../types'

const preset: ExplorePreset = {
  id: 'p1',
  name: 'Preset 1',
  preview: { path: '/influencer/presets/a.webp', width: 400, height: 600 },
  sheet: { path: '/influencer/presets/b.webp', width: 1200, height: 700 },
  tier: 'freak',
  selection: { hair: ['hair_bowl'] },
}

describe('PresetPreviewDialog blocked apply', () => {
  it('keeps the blocked reason on the disabled control instead of a paragraph', () => {
    render(
      <PresetPreviewDialog
        preset={preset}
        taxonomy={null}
        onClose={vi.fn()}
        onApply={vi.fn()}
      />
    )

    const reason = 'Preset settings are being updated. Please try again later.'
    const apply = screen.getByRole('button', { name: 'Recreate' })
    expect(apply).toBeDisabled()
    expect(apply).toHaveAttribute('title', reason)
    const description = screen.getByText(reason)
    expect(description).toHaveClass('sr-only')
    expect(apply.getAttribute('aria-describedby')).toBe(description.id)
  })
})
