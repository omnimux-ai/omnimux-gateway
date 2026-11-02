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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen } from '@testing-library/react'
import type { ReactElement } from 'react'
import { describe, expect, test } from 'vitest'

import type { PricingModel } from '@/features/pricing/types'

import { ModelPlaygroundSection } from '../model-playground-section'

function seedance(): PricingModel {
  return {
    id: 1,
    model_name: 'seedance-2-5',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 1,
    enable_groups: ['default'],
    tags: 'Video Generation,async,task',
    output_modalities: ['video'],
  }
}

function imageModel(): PricingModel {
  return {
    id: 2,
    model_name: 'gpt-image-2.5',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    model_price: 0.01,
    enable_groups: ['default'],
    input_modalities: ['text', 'image'],
    output_modalities: ['image'],
  }
}

function renderSection(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>
  )
}

function pickMode(label: string) {
  fireEvent.click(screen.getByRole('button', { name: 'Generation mode' }))
  fireEvent.click(screen.getByRole('option', { name: label }))
}

describe('playground mode-aware form', () => {
  test('offers a mode picker when the model declares modes', () => {
    renderSection(<ModelPlaygroundSection model={seedance()} authenticated />)
    expect(
      screen.getByRole('button', { name: 'Generation mode' })
    ).toBeInTheDocument()
  })

  test('keeps the flat form for a model with no declared modes', () => {
    renderSection(<ModelPlaygroundSection model={imageModel()} authenticated />)
    expect(screen.queryByRole('button', { name: 'Generation mode' })).toBeNull()
  })

  test('renders only the slots the chosen mode accepts', () => {
    const { container } = renderSection(
      <ModelPlaygroundSection model={seedance()} authenticated />
    )

    expect(container.querySelector('[data-parity="playground.form.first-frame"]')).toBeNull()

    pickMode('First & last frame')

    expect(
      container.querySelector('[data-parity="playground.form.first-frame"]')
    ).not.toBeNull()
    expect(
      container.querySelector('[data-parity="playground.form.last-frame"]')
    ).not.toBeNull()
  })

  test('pins the aspect ratio in the frame modes and says why', () => {
    const { container } = renderSection(
      <ModelPlaygroundSection model={seedance()} authenticated />
    )

    pickMode('First frame')

    const locked = container.querySelector(
      '[data-parity="playground.form.ratio"] [data-parity="playground.form.locked"]'
    )
    expect(locked).not.toBeNull()
    expect(locked?.textContent).toBe('adaptive')
    expect(
      screen.getByText('Aspect ratio follows the input frame')
    ).toBeInTheDocument()
  })

  test('locks the duration to auto in video edit', () => {
    const { container } = renderSection(
      <ModelPlaygroundSection model={seedance()} authenticated />
    )

    pickMode('Video edit')

    const locked = container.querySelector(
      '[data-parity="playground.form.duration"] [data-parity="playground.form.locked"]'
    )
    expect(locked?.textContent).toBe('Auto')
    expect(
      screen.getByText('Duration follows the input video')
    ).toBeInTheDocument()
  })

  test('keeps the prompt when the mode changes', () => {
    renderSection(<ModelPlaygroundSection model={seedance()} authenticated />)

    const prompt = screen.getByPlaceholderText('Describe the result you want')
    fireEvent.change(prompt, { target: { value: 'a paper boat on a river' } })

    pickMode('Video edit')

    expect(
      screen.getByPlaceholderText('Describe the result you want')
    ).toHaveValue('a paper boat on a river')
  })

  test('blocks submit while the mode still misses its required input', () => {
    renderSection(<ModelPlaygroundSection model={seedance()} authenticated />)

    pickMode('Video edit')

    expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled()
    expect(
      screen.getByText('Add the required input: Source video')
    ).toBeInTheDocument()
  })

  test('warns when an edit prompt states no edit intent, and offers a prefix', () => {
    renderSection(<ModelPlaygroundSection model={seedance()} authenticated />)

    const prompt = screen.getByPlaceholderText('Describe the result you want')
    fireEvent.change(prompt, { target: { value: 'a girl dancing' } })
    pickMode('Video edit')

    const complete = screen.getByRole('button', { name: 'Complete the prompt' })
    fireEvent.click(complete)

    expect(
      screen.getByPlaceholderText('Describe the result you want')
    ).toHaveValue('Edit the video: a girl dancing')
  })

  test('does not warn once the prompt states the edit', () => {
    renderSection(<ModelPlaygroundSection model={seedance()} authenticated />)

    const prompt = screen.getByPlaceholderText('Describe the result you want')
    fireEvent.change(prompt, { target: { value: 'Remove the logo from the cup' } })
    pickMode('Video edit')

    expect(
      screen.queryByRole('button', { name: 'Complete the prompt' })
    ).toBeNull()
  })

  test('previews the request body with the operation and vendor sub-task', () => {
    renderSection(<ModelPlaygroundSection model={seedance()} authenticated />)

    pickMode('Video edit')
    fireEvent.click(screen.getByRole('button', { name: 'JSON mode' }))

    const editor = screen.getByRole('textbox', {
      name: 'Request JSON',
    }) as HTMLTextAreaElement
    expect(editor.value).toContain('"operation": "video_edit"')
    expect(editor.value).toContain('"omni_reference_task_type": "edit"')
  })
})
