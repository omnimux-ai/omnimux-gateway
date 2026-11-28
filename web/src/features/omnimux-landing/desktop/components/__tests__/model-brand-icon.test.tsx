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

import { ModelBrandIcon } from '../cn/model-brand-icon'

describe('ModelBrandIcon', () => {
  it.each([
    ['Doubao.Color', 'Doubao'],
    ['OpenAI', 'OpenAI'],
    ['Gemini.Color', 'Gemini'],
    ['Qwen.Color', 'Qwen'],
    ['Minimax.Color', 'Minimax'],
    ['Grok', 'Grok'],
    ['Midjourney', 'Midjourney'],
    ['Suno', 'Suno'],
  ])('renders the static %s icon at the requested size', (icon, title) => {
    const { container } = render(<ModelBrandIcon icon={icon} size={18} />)
    expect(container.querySelector('svg')).toHaveAttribute('width', '18')
    expect(screen.getByTitle(title)).toBeInTheDocument()
  })

  it('renders nothing for an icon outside the landing whitelist', () => {
    const { container } = render(<ModelBrandIcon icon='Mistral' />)
    expect(container).toBeEmptyDOMElement()
  })
})
