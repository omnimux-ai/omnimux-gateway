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
import { describe, expect, test, vi } from 'vitest'

import {
  PlaygroundModeSelector,
  type ModePriceLine,
} from '../playground-mode-selector'
import type { PlaygroundOperation } from '../../types'

const MODES: PlaygroundOperation[] = [
  'text_to_video',
  'first_frame',
  'first_last_frame',
  'video_multi_ref',
  'video_edit',
  'video_extend',
]

function noop() {}

describe('PlaygroundModeSelector', () => {
  test('shows the current mode collapsed and reports aria-expanded', () => {
    render(
      <PlaygroundModeSelector
        operations={MODES}
        value='text_to_video'
        priceFor={() => null}
        onChange={noop}
      />
    )

    const trigger = screen.getByRole('button', { name: 'Generation mode' })
    expect(trigger).toHaveAttribute('aria-expanded', 'false')
    expect(trigger).toHaveTextContent('Text to video')
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  test('expands into a listbox holding every mode', () => {
    render(
      <PlaygroundModeSelector
        operations={MODES}
        value='text_to_video'
        priceFor={() => null}
        onChange={noop}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Generation mode' }))

    const trigger = screen.getByRole('button', { name: 'Generation mode' })
    expect(trigger).toHaveAttribute('aria-expanded', 'true')
    expect(screen.getByRole('listbox')).toBeInTheDocument()
    expect(screen.getAllByRole('option')).toHaveLength(MODES.length)
  })

  test('marks the current mode as selected', () => {
    render(
      <PlaygroundModeSelector
        operations={MODES}
        value='video_edit'
        priceFor={() => null}
        onChange={noop}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Generation mode' }))

    expect(screen.getByRole('option', { name: 'Video edit' })).toHaveAttribute(
      'aria-selected',
      'true'
    )
    expect(screen.getByRole('option', { name: 'Text to video' })).toHaveAttribute(
      'aria-selected',
      'false'
    )
  })

  test('reports the picked mode and collapses again', () => {
    const onChange = vi.fn()
    render(
      <PlaygroundModeSelector
        operations={MODES}
        value='text_to_video'
        priceFor={() => null}
        onChange={onChange}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Generation mode' }))
    fireEvent.click(screen.getByRole('option', { name: 'Video extend' }))

    expect(onChange).toHaveBeenCalledWith('video_extend')
    expect(screen.queryByRole('listbox')).toBeNull()
  })

  test('does not re-report the mode that is already selected', () => {
    const onChange = vi.fn()
    render(
      <PlaygroundModeSelector
        operations={MODES}
        value='text_to_video'
        priceFor={() => null}
        onChange={onChange}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Generation mode' }))
    fireEvent.click(screen.getByRole('option', { name: 'Text to video' }))

    expect(onChange).not.toHaveBeenCalled()
  })

  test('carries the starting rate in the trigger and on every option', () => {
    const rates: Partial<Record<PlaygroundOperation, ModePriceLine>> = {
      text_to_video: {
        amount: 'from ~$0.3143 / per request',
        credits: '3.143 credits',
      },
      video_edit: {
        amount: 'from ~$0.074 / per second',
        credits: '0.74 credits',
      },
    }

    render(
      <PlaygroundModeSelector
        operations={MODES}
        value='text_to_video'
        priceFor={(operation) => rates[operation] ?? null}
        onChange={noop}
      />
    )

    const trigger = screen.getByRole('button', { name: 'Generation mode' })
    expect(trigger).toHaveTextContent('from ~$0.3143 / per request')
    expect(trigger).toHaveTextContent('3.143 credits')

    fireEvent.click(trigger)

    const edit = screen.getByRole('option', { name: /Video edit/ })
    expect(edit).toHaveTextContent('from ~$0.074 / per second')
    expect(edit).toHaveTextContent('0.74 credits')
  })

  test('drops the rate block when the catalog cannot price the model', () => {
    render(
      <PlaygroundModeSelector
        operations={MODES}
        value='text_to_video'
        priceFor={() => null}
        onChange={noop}
      />
    )

    const trigger = screen.getByRole('button', { name: 'Generation mode' })
    expect(trigger).toHaveTextContent('Text to video')
    expect(trigger).not.toHaveTextContent('$')
  })

  test('falls back to a static label when the model has a single mode', () => {
    render(
      <PlaygroundModeSelector
        operations={['text_to_video']}
        value='text_to_video'
        priceFor={() => null}
        onChange={noop}
      />
    )

    expect(screen.queryByRole('button', { name: 'Generation mode' })).toBeNull()
    expect(screen.getByText('Text to video')).toBeInTheDocument()
  })
})
