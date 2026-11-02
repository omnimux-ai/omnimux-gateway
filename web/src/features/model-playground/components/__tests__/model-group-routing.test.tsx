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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { PricingModel } from '@/features/pricing/types'

import { getGenerationTransport } from '../../transport'
import { ModelPlaygroundSection } from '../model-playground-section'

function mockImageModel(): PricingModel {
  return {
    id: 1,
    model_name: 'gpt-image-2.5',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    model_price: 0.013072,
    enable_groups: ['default', 'gpt-image-2.5-economy', 'gpt-image-2.5-pro'],
    input_modalities: ['text'],
    output_modalities: ['image'],
  }
}

/** The public video id whose quality tiers are token groups. */
function mockWanModel(): PricingModel {
  return {
    id: 156,
    model_name: 'wan-3.0',
    quota_type: 1,
    model_ratio: 1,
    completion_ratio: 1,
    model_price: 0.176471,
    enable_groups: ['wan-3.0-720p', 'wan-3.0-1080p'],
    tags: 'Video Generation,async,task',
    output_modalities: ['video'],
  }
}

function renderWithClient(ui: React.ReactElement) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(<QueryClientProvider client={client}>{ui}</QueryClientProvider>)
}

describe('ModelPlaygroundSection group routing and pricing', () => {
  const groupRatio = {
    default: 1,
    'gpt-image-2.5-economy': 0.112455,
    'gpt-image-2.5-pro': 0.281248,
  }

  it('reflects selected group in JSON preview body', async () => {
    const user = userEvent.setup()
    const model = mockImageModel()
    renderWithClient(
      <ModelPlaygroundSection
        model={model}
        authenticated
        selectedGroup='gpt-image-2.5-economy'
        groupRatio={groupRatio}
      />
    )

    // Switch to JSON view in input form
    const jsonBtn = screen.getByRole('button', { name: /json mode/i })
    await user.click(jsonBtn)

    // Check that textarea includes "group": "gpt-image-2.5-economy"
    const textarea = screen.getByRole('textbox', { name: /request json/i })
    expect((textarea as HTMLTextAreaElement).value).toContain(
      '"group": "gpt-image-2.5-economy"'
    )
  })

  it('omits group property when selected group is default', async () => {
    const user = userEvent.setup()
    const model = mockImageModel()
    renderWithClient(
      <ModelPlaygroundSection
        model={model}
        authenticated
        selectedGroup='default'
        groupRatio={groupRatio}
      />
    )

    // Switch to JSON view in input form
    const jsonBtn = screen.getByRole('button', { name: /json mode/i })
    await user.click(jsonBtn)

    const textarea = screen.getByRole('textbox', { name: /request json/i })
    expect((textarea as HTMLTextAreaElement).value).not.toContain(
      '"group": "default"'
    )
  })

  it('defaults preview stage to the latest completed task with media from history', async () => {
    const model = mockImageModel()
    const transport = getGenerationTransport()
    const spy = vi.spyOn(transport, 'listHistory').mockResolvedValue([
      {
        taskId: 'task_latest_done',
        modelId: 'gpt-image-2.5',
        status: 'succeeded' as const,
        progress: 100,
        params: { prompt: 'puppy portrait' },
        estimatedUsd: 0.0015,
        settledUsd: 0.0015,
        createdAt: 100,
        finishedAt: 105,
        media: [
          {
            url: 'https://example.com/result-dog.png',
            kind: 'image' as const,
          },
        ],
      },
    ])

    try {
      renderWithClient(
        <ModelPlaygroundSection
          model={model}
          authenticated
        />
      )

      const img = await screen.findByAltText('Generated result')
      expect(img.getAttribute('src')).toBe('https://example.com/result-dog.png')
      expect(screen.queryByText('No Task Running')).toBeNull()
    } finally {
      spy.mockRestore()
    }
  })
})

describe('wan-3.0 resolution tiers', () => {
  const wanGroupRatio = { 'wan-3.0-720p': 0.5, 'wan-3.0-1080p': 1.5 }

  function renderWan() {
    return renderWithClient(
      <ModelPlaygroundSection
        model={mockWanModel()}
        authenticated
        selectedGroup='default'
        groupRatio={wanGroupRatio}
      />
    )
  }

  function mockSubmit() {
    const transport = getGenerationTransport()
    const submit = vi
      .spyOn(transport, 'submit')
      .mockResolvedValue({ taskId: 'task_wan' })
    const getTask = vi.spyOn(transport, 'getTask').mockResolvedValue({
      taskId: 'task_wan',
      modelId: 'wan-3.0',
      status: 'queued' as const,
      progress: 0,
      params: {},
      estimatedUsd: null,
      settledUsd: null,
      createdAt: 0,
      media: [],
    })
    return {
      submit,
      restore: () => {
        submit.mockRestore()
        getTask.mockRestore()
      },
    }
  }

  it('offers the two provisioned tiers and prices the selected one', async () => {
    const user = userEvent.setup()
    renderWan()

    expect(screen.queryByRole('button', { name: '480p' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Prime' })).toBeNull()

    const hd = screen.getByRole('button', { name: '720p' })
    expect(hd).toHaveAttribute('aria-pressed', 'true')

    // 5s at 720p: 0.176471 USD/s × 0.5 × 5 = 0.4411775 USD = 4.411775 credits.
    await user.click(screen.getByRole('button', { name: '5' }))
    expect(screen.getByText(/~\$0\.441/)).toBeInTheDocument()
    expect(screen.getByText(/4\.41 credits/)).toBeInTheDocument()

    // 5s at 1080p: 0.176471 USD/s × 1.5 × 5 = 1.3235325 USD = 13.235325 credits.
    await user.click(screen.getByRole('button', { name: '1080p' }))
    expect(screen.getByText(/~\$1\.324/)).toBeInTheDocument()
    expect(screen.getByText(/13\.24 credits/)).toBeInTheDocument()
  })

  it('pins the resolution tier group on submit and keeps the body group-free', async () => {
    const user = userEvent.setup()
    const mocked = mockSubmit()
    try {
      renderWan()

      await user.click(screen.getByRole('button', { name: 'Generate' }))

      await waitFor(() => expect(mocked.submit).toHaveBeenCalledTimes(1))
      const request = mocked.submit.mock.calls[0][0]
      expect(request.modelId).toBe('wan-3.0')
      expect(request.group).toBe('wan-3.0-720p')
      expect(request.params.group).toBeUndefined()
      expect(request.params.quality_tier).toBeUndefined()
      expect(request.params.resolution).toBe('720p')

      await user.click(screen.getByRole('button', { name: '1080p' }))
      await user.click(screen.getByRole('button', { name: 'Generate' }))

      await waitFor(() => expect(mocked.submit).toHaveBeenCalledTimes(2))
      expect(mocked.submit.mock.calls[1][0].group).toBe('wan-3.0-1080p')
    } finally {
      mocked.restore()
    }
  })
})
