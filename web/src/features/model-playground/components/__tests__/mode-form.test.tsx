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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import type { ReactElement } from 'react'
import { describe, expect, test, vi } from 'vitest'

import type { PricingModel } from '@/features/pricing/types'

import { getGenerationTransport } from '../../transport'
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
    contract: {
      model_id: 'seedance-2-5',
      family: 'video',
      prompt_max_chars: 5000,
      operations: [
        {
          id: 'text_to_video',
          executable: true,
          inputs: [{ slot: 'prompt', type: 'text', required: true }],
          parameters: [],
          output: { type: 'video' },
        },
        {
          id: 'first_frame',
          executable: true,
          inputs: [
            { slot: 'prompt', type: 'text', required: true },
            {
              slot: 'first_frame',
              type: 'image',
              role: 'first_frame',
              required: true,
              min: 1,
              max: 1,
            },
          ],
          parameters: [
            {
              name: 'aspect_ratio',
              type: 'enum',
              enum: ['adaptive'],
              default: 'adaptive',
              fixed: 'adaptive',
            },
          ],
          output: { type: 'video' },
        },
        {
          id: 'first_last_frame',
          executable: true,
          inputs: [
            { slot: 'prompt', type: 'text', required: true },
            {
              slot: 'first_frame',
              type: 'image',
              role: 'first_frame',
              required: true,
              min: 1,
              max: 1,
            },
            {
              slot: 'last_frame',
              type: 'image',
              role: 'last_frame',
              required: true,
              min: 1,
              max: 1,
            },
          ],
          parameters: [],
          output: { type: 'video' },
        },
        {
          id: 'video_multi_ref',
          executable: true,
          inputs: [{ slot: 'prompt', type: 'text', required: true }],
          parameters: [],
          output: { type: 'video' },
        },
        {
          id: 'video_edit',
          executable: true,
          inputs: [
            { slot: 'prompt', type: 'text', required: true },
            {
              slot: 'source_videos',
              type: 'video',
              role: 'source',
              required: true,
              min: 1,
              max: 1,
            },
          ],
          parameters: [
            {
              name: 'duration',
              type: 'integer',
              minimum: -1,
              maximum: -1,
              default: -1,
              fixed: -1,
            },
          ],
          output: { type: 'video' },
        },
        {
          id: 'video_extend',
          executable: true,
          inputs: [
            { slot: 'prompt', type: 'text', required: true },
            {
              slot: 'source_videos',
              type: 'video',
              role: 'source',
              required: true,
              min: 1,
              max: 1,
            },
          ],
          parameters: [],
          output: { type: 'video' },
        },
      ],
    },
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

function muse(max: number): PricingModel {
  const model = seedance()
  model.model_name = 'muse-video'
  model.contract = {
    model_id: model.model_name,
    family: 'video',
    operations: [
      {
        id: 'video_multi_ref',
        executable: true,
        inputs: [
          { slot: 'prompt', type: 'text', required: true },
          {
            slot: 'reference_images',
            type: 'image',
            firstItemRole: 'first_frame',
            allowedMimes: ['image/png'],
            required: true,
            min: 2,
            max,
          },
        ],
        parameters: [],
        output: { type: 'video' },
      },
      {
        id: 'first_frame',
        executable: true,
        inputs: [
          { slot: 'prompt', type: 'text', required: true },
          {
            slot: 'first_frame',
            type: 'image',
            allowedMimes: ['image/png'],
            required: true,
            min: 1,
            max: 1,
          },
        ],
        parameters: [],
        output: { type: 'video' },
      },
    ],
  }
  return model
}

function setReferenceImages(references: string[]) {
  fireEvent.click(screen.getByRole('button', { name: 'JSON mode' }))
  fireEvent.change(screen.getByRole('textbox', { name: 'Request JSON' }), {
    target: {
      value: JSON.stringify({
        prompt: 'Use the reference images',
        reference_images: references,
      }),
    },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Form mode' }))
}

function referenceUrls(count: number): string[] {
  return Array.from(
    { length: count },
    (_, index) => `https://example.com/image-${index + 1}.png`
  )
}

function requestParameters() {
  fireEvent.click(screen.getByRole('button', { name: 'JSON mode' }))
  const editor = screen.getByRole<HTMLTextAreaElement>('textbox', {
    name: 'Request JSON',
  })
  return JSON.parse(editor.value)
}

describe('playground mode-aware form', () => {
  test('offers a mode picker when the model declares modes', () => {
    renderSection(<ModelPlaygroundSection model={seedance()} authenticated />)
    expect(
      screen.getByRole('button', { name: 'Generation mode' })
    ).toBeInTheDocument()
  })

  test('requires two ordered image references and swaps their submitted order', () => {
    renderSection(<ModelPlaygroundSection model={muse(2)} authenticated />)
    expect(screen.getByRole('button', { name: 'Generate' })).toBeDisabled()
    expect(screen.getByText('Add two reference images')).toBeInTheDocument()

    const [first, second] = referenceUrls(2)
    setReferenceImages([first, second])
    expect(screen.getByRole('button', { name: 'Generate' })).toBeEnabled()
    fireEvent.click(screen.getByRole('button', { name: 'Swap order' }))
    expect(requestParameters().image_urls).toEqual([second, first])
  })

  test('promotes the tenth reference and swaps only the selected and starting positions', async () => {
    const user = userEvent.setup()
    renderSection(<ModelPlaygroundSection model={muse(10)} authenticated />)
    const urls = referenceUrls(10)
    setReferenceImages(urls)

    await user.click(screen.getByRole('button', { name: 'Image 10 · Actions' }))
    await user.click(
      await screen.findByRole('menuitem', { name: 'Use as starting image' })
    )

    expect(requestParameters().image_urls).toEqual([
      urls[9],
      ...urls.slice(1, 9),
      urls[0],
    ])
  })

  test.each([
    [2, 1],
    [10, 5],
  ])(
    'replaces a reference in place and restores focus at the limit of %i',
    async (max, index) => {
      const user = userEvent.setup()
      const replacementUrl = 'https://example.com/replacement.png'
      const upload = vi
        .spyOn(getGenerationTransport(), 'uploadReference')
        .mockResolvedValue({ url: replacementUrl })
      try {
        renderSection(
          <ModelPlaygroundSection model={muse(max)} authenticated />
        )
        const urls = referenceUrls(max)
        setReferenceImages(urls)

        const actionName = `Image ${index} · Actions`
        const action = screen.getByRole('button', { name: actionName })
        action.focus()
        await user.keyboard('{Enter}')
        await user.click(
          await screen.findByRole('menuitem', { name: 'Replace reference' })
        )
        const file = new File(['test'], 'replacement.png', {
          type: 'image/png',
        })
        fireEvent.change(screen.getByLabelText('Replace reference'), {
          target: { files: [file] },
        })

        await waitFor(() =>
          expect(screen.getByAltText('replacement.png')).toBeInTheDocument()
        )
        expect(screen.getByRole('button', { name: actionName })).toHaveFocus()
        expect(requestParameters().image_urls).toEqual([
          ...urls.slice(0, index - 1),
          replacementUrl,
          ...urls.slice(index),
        ])
      } finally {
        upload.mockRestore()
      }
    }
  )

  test('shares the start with single-frame mode and restores the ordered request', () => {
    renderSection(<ModelPlaygroundSection model={muse(10)} authenticated />)
    const urls = referenceUrls(10)
    setReferenceImages(urls)

    pickMode('First frame')
    const frameRequest = requestParameters()
    expect(frameRequest.image_with_roles).toEqual([
      { url: urls[0], role: 'first_frame' },
    ])
    expect(frameRequest).not.toHaveProperty('image_urls')

    pickMode('Multiple image references')
    const referencesRequest = requestParameters()
    expect(referencesRequest.image_urls).toEqual(urls)
    expect(referencesRequest).not.toHaveProperty('image_with_roles')
  })

  test('adopts a new single frame already present later in the ordered list without duplicating it', async () => {
    const urls = referenceUrls(4)
    const upload = vi
      .spyOn(getGenerationTransport(), 'uploadReference')
      .mockResolvedValue({ url: urls[2] })
    try {
      const { container } = renderSection(
        <ModelPlaygroundSection model={muse(10)} authenticated />
      )
      setReferenceImages(urls)
      pickMode('First frame')
      fireEvent.click(
        screen.getByRole('button', { name: 'Remove image-1.png' })
      )
      const file = new File(['test'], 'image-3.png', { type: 'image/png' })
      const input = container.querySelector(
        '[data-parity="playground.form.first-frame"] input[type=file]'
      )
      if (!input) throw new Error('Expected a first-frame upload input')
      fireEvent.change(input, { target: { files: [file] } })
      await screen.findByAltText('image-3.png')

      pickMode('Multiple image references')
      expect(requestParameters().image_urls).toEqual([
        urls[2],
        urls[1],
        urls[0],
        urls[3],
      ])
    } finally {
      upload.mockRestore()
    }
  })

  test('replaces a single frame only after its upload succeeds', async () => {
    const user = userEvent.setup()
    const [startingUrl, secondUrl] = referenceUrls(2)
    const replacementUrl = 'https://example.com/replacement.png'
    const upload = vi
      .spyOn(getGenerationTransport(), 'uploadReference')
      .mockRejectedValueOnce(new Error('Upload failed'))
      .mockResolvedValueOnce({ url: replacementUrl })
    try {
      renderSection(<ModelPlaygroundSection model={muse(10)} authenticated />)
      setReferenceImages([startingUrl, secondUrl])
      pickMode('First frame')
      const action = () =>
        screen.getByRole('button', { name: 'Image 1 · Actions' })
      const replace = async () => {
        await user.click(action())
        await user.click(
          await screen.findByRole('menuitem', { name: 'Replace reference' })
        )
        const file = new File(['test'], 'replacement.png', {
          type: 'image/png',
        })
        fireEvent.change(screen.getByLabelText('Replace reference'), {
          target: { files: [file] },
        })
        await waitFor(() => expect(action()).toBeEnabled())
      }

      await replace()
      expect(screen.getByAltText('image-1.png')).toBeInTheDocument()
      expect(screen.queryByAltText('replacement.png')).not.toBeInTheDocument()
      await replace()
      expect(screen.getByAltText('replacement.png')).toBeInTheDocument()
      expect(screen.queryByAltText('image-1.png')).not.toBeInTheDocument()

      const frameRequest = requestParameters()
      expect(frameRequest.image_with_roles).toEqual([
        { url: replacementUrl, role: 'first_frame' },
      ])
      expect(frameRequest).not.toHaveProperty('image_urls')
      pickMode('Multiple image references')
      expect(requestParameters().image_urls).toEqual([
        replacementUrl,
        secondUrl,
      ])
    } finally {
      upload.mockRestore()
    }
  })

  test('blocks an eleven-image JSON request with the configured limit', () => {
    renderSection(<ModelPlaygroundSection model={muse(10)} authenticated />)
    setReferenceImages(referenceUrls(11))

    const generate = screen.getByRole('button', { name: 'Generate' })
    expect(generate).toBeDisabled()
    expect(generate).toHaveAccessibleDescription(
      'Choose at most 10 reference images'
    )
  })

  test('keeps the flat form for a model with no declared modes', () => {
    renderSection(<ModelPlaygroundSection model={imageModel()} authenticated />)
    expect(screen.queryByRole('button', { name: 'Generation mode' })).toBeNull()
  })

  test('renders only the slots the chosen mode accepts', () => {
    const { container } = renderSection(
      <ModelPlaygroundSection model={seedance()} authenticated />
    )

    expect(
      container.querySelector('[data-parity="playground.form.first-frame"]')
    ).toBeNull()

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
      screen.getByText('Locked by the service contract')
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
      screen.getByText('Locked by the service contract')
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
    fireEvent.change(prompt, {
      target: { value: 'Remove the logo from the cup' },
    })
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
