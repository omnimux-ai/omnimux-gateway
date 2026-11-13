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
import { act, render } from '@testing-library/react'
import { toast } from 'sonner'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import {
  fetchRecreateAnalysis,
  startRecreateAnalysis,
} from '../api'
import { useMotion, type MotionForm } from '../hooks/use-motion'
import type { RecreateResult } from '../types'
import { readBrowserVideoMetadata } from '../lib/browser-video-metadata'

vi.mock('../lib/browser-video-metadata', () => ({
  readBrowserVideoMetadata: vi.fn(),
}))

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    startRecreateAnalysis: vi.fn(),
    fetchRecreateAnalysis: vi.fn(),
  }
})

const startAnalysis = vi.mocked(startRecreateAnalysis)
const pollAnalysis = vi.mocked(fetchRecreateAnalysis)

const VIDEO_METADATA = {
  duration: 8,
  video_duration: 8,
  width: 720,
  height: 1280,
  frame_rate: 30,
  video_codec: 'h264',
  audio_codec: 'aac',
  container: 'mp4',
  size_bytes: 2048,
}

let form: MotionForm

function Harness() {
  form = useMotion()
  return <output data-testid='analyzing'>{String(form.recreateAnalyzing)}</output>
}

beforeEach(() => {
  vi.mocked(readBrowserVideoMetadata).mockResolvedValue(VIDEO_METADATA)
  startAnalysis.mockReset()
  startAnalysis.mockResolvedValue({
    task_id: 'task-1',
    status: 'waiting',
    progress: '0%',
  })
  pollAnalysis.mockReset()
  pollAnalysis.mockResolvedValue({
    task_id: 'task-1',
    status: 'running',
    progress: '20%',
  })
  localStorage.clear()
})

afterEach(() => {
  vi.restoreAllMocks()
  localStorage.clear()
})

async function prepareForm() {
  const view = render(<Harness />)
  await act(async () => {
    form.setMode('recreate')
  })
  await act(async () => {
    await form.setRemoteVideo('https://files.example/clip.mp4', 'clip.mp4')
  })
  act(() => {
    form.addCharacter({
      url: 'https://files.example/front.png',
      fileName: 'front.png',
    })
  })
  return view
}

const SUCCESS = {
  task_id: 'task-1',
  status: 'success',
  prompt: 'Reference image 1 is the hero product.',
  analysis: { video_overview: { one_line: 'A single continuous take.' } },
  image_analysis: [],
  meta: {
    duration_sec: 8,
    reference_duration_sec: 7.6,
    pricing: { '480p': 6.47, '720p': 13.89 },
  },
}

describe('recreate analysis polling', () => {
  it('waits one poll interval and stores the success payload', async () => {
    pollAnalysis.mockReset()
    pollAnalysis.mockResolvedValue(SUCCESS)
    await prepareForm()

    const outcome: { value: RecreateResult | null } = { value: null }
    await act(async () => {
      outcome.value = await form.analyzeRecreate()
    })

    expect(outcome.value?.prompt).toBe('Reference image 1 is the hero product.')
    expect(startAnalysis).toHaveBeenCalledTimes(1)
    expect(form.recreateResult?.prompt).toBe(
      'Reference image 1 is the hero product.'
    )
    expect(form.recreateResult?.durationSec).toBe(8)
    expect(form.recreateAnalyzing).toBe(false)
    expect(form.recreateQuote).toEqual({ usd: 0, credits: 6.47 })
  })

  it('shows the readable failure message and keeps the form', async () => {
    const message = 'Reference video analysis timed out. Try again later.'
    pollAnalysis.mockReset()
    pollAnalysis.mockResolvedValue({
      task_id: 'task-1',
      status: 'failed',
      message,
    })
    const errorSpy = vi.spyOn(toast, 'error')
    await prepareForm()

    const outcome: { value: RecreateResult | null } = { value: null }
    await act(async () => {
      outcome.value = await form.analyzeRecreate()
    })

    expect(outcome.value).toBeNull()
    expect(errorSpy).toHaveBeenCalledWith(message)
    expect(form.recreateResult).toBeNull()
    expect(form.recreateAnalyzing).toBe(false)
  })
})

describe('recreate analysis cancellation', () => {
  it('stops polling when the form unmounts', async () => {
    const view = await prepareForm()
    let result: Promise<RecreateResult | null> | undefined
    await act(async () => {
      result = form.analyzeRecreate()
      // Let the enqueue promise settle; the first poll is still 2s away.
      await Promise.resolve()
    })
    expect(startAnalysis).toHaveBeenCalledTimes(1)

    act(() => {
      view.unmount()
    })
    await act(async () => {
      await result
    })

    // Unmount aborts the sleep before the first GET ever happens.
    expect(pollAnalysis).not.toHaveBeenCalled()
  })

  it('stops polling when the mode changes', async () => {
    await prepareForm()
    let result: Promise<RecreateResult | null> | undefined
    await act(async () => {
      result = form.analyzeRecreate()
      await Promise.resolve()
    })
    expect(startAnalysis).toHaveBeenCalledTimes(1)

    act(() => {
      form.setMode('motion')
    })
    await act(async () => {
      await result
    })

    expect(pollAnalysis).not.toHaveBeenCalled()
  })
})
