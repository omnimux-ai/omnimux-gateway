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
import { act, renderHook } from '@testing-library/react'
import { assert, beforeEach, describe, expect, it, vi } from 'vitest'

import { useMotion } from '../hooks/use-motion'
import { readBrowserVideoMetadata } from '../lib/browser-video-metadata'
import { MOTION_PRESETS } from '../lib/motion-presets'

vi.mock('../api', () => ({ uploadReferenceAsset: vi.fn() }))
vi.mock('../lib/browser-video-metadata', () => ({
  readBrowserVideoMetadata: vi.fn(),
}))

beforeEach(() => {
  localStorage.clear()
  vi.mocked(readBrowserVideoMetadata).mockResolvedValue({
    duration: 5,
    video_duration: 5,
    width: 720,
    height: 1280,
    frame_rate: 24,
    video_codec: 'h264',
    audio_codec: 'aac',
    container: 'mp4',
    size_bytes: 1024,
  })
})

const style = {
  id: 'motion-cel-animation',
  nameKey: 'Hand-painted cel animation',
  summaryKey: 'Stable outlines and layered colours',
  mode: 'motion' as const,
  prompt: 'Render the whole clip as hand-painted cel animation.',
}

describe('motion visual style state', () => {
  it('selecting a replacement shelf clip keeps its reference video and excludes display text from the task', async () => {
    const preset = MOTION_PRESETS.find((item) => item.mode === 'replace')
    assert(preset, 'The replacement shelf fixture must exist')
    const { result } = renderHook(() => useMotion())
    act(() => result.current.setMode('replace'))

    await act(async () => {
      await result.current.applyStyle(preset)
    })

    expect(result.current.style?.id).toBe(preset.id)
    expect(result.current.video).toMatchObject({
      kind: 'remote',
      url: preset.referenceVideoUrl,
    })
    expect(result.current.effectivePrompt).toBe('')
  })

  it('a replacement shelf clip submits personal text only while the prompt switch is enabled', async () => {
    const preset = MOTION_PRESETS.find((item) => item.mode === 'replace')
    assert(preset, 'The replacement shelf fixture must exist')
    const { result } = renderHook(() => useMotion())
    act(() => result.current.setMode('replace'))
    await act(async () => {
      await result.current.applyStyle(preset)
    })
    act(() => result.current.setPrompt('  Replace the jacket only.  '))
    expect(result.current.effectivePrompt).toBe('')

    act(() => result.current.setPromptEnabled(true))
    expect(result.current.effectivePrompt).toBe('Replace the jacket only.')

    act(() => result.current.setPromptEnabled(false))
    expect(result.current.effectivePrompt).toBe('')
    expect(result.current.prompt).toBe('  Replace the jacket only.  ')
  })

  it('choosing a visual style without a video keeps the video empty and generation unavailable', () => {
    const { result } = renderHook(() => useMotion())
    act(() => result.current.applyStyle(style))
    expect(result.current.video).toBeNull()
    expect(result.current.canSubmit).toBe(false)
    expect(result.current.effectivePrompt).toBe(style.prompt)
  })

  it('deleting a motion reference keeps its visual style selected', () => {
    const { result } = renderHook(() => useMotion())
    act(() => result.current.applyStyle(style))
    act(() => result.current.clearVideo())
    expect(result.current.style?.id).toBe(style.id)
    expect(result.current.effectivePrompt).toBe(style.prompt)
  })

  it('switching modes excludes the other mode style and restores each selection on return', () => {
    const { result } = renderHook(() => useMotion())
    const replacement = {
      id: 'replace-only',
      nameKey: 'Replacement',
      mode: 'replace' as const,
      prompt: 'Keep the replacement scene.',
      durationSec: 5,
    }
    act(() => result.current.applyStyle(style))
    act(() => result.current.setMode('replace'))
    expect(result.current.style).toBeNull()
    expect(result.current.effectivePrompt).toBe('')
    act(() => result.current.applyStyle(replacement))
    act(() => result.current.setMode('motion'))
    expect(result.current.style?.id).toBe(style.id)
    expect(result.current.effectivePrompt).toBe(style.prompt)
    act(() => result.current.setMode('replace'))
    expect(result.current.style?.id).toBe(replacement.id)
    expect(result.current.effectivePrompt).toBe(replacement.prompt)
  })

  it('enabled user text joins the style unchanged even when the words contradict the selected look', () => {
    const { result } = renderHook(() => useMotion())
    act(() => {
      result.current.applyStyle(style)
      result.current.setPrompt(
        '  Use realistic photography.\nKeep @Image 1 recognisable.  '
      )
      result.current.setPromptEnabled(true)
    })
    expect(result.current.effectivePrompt).toBe(
      'Render the whole clip as hand-painted cel animation.\n\nUse realistic photography.\nKeep @Image 1 recognisable.'
    )
    act(() => result.current.setPromptEnabled(false))
    expect(result.current.effectivePrompt).toBe(style.prompt)
    expect(result.current.prompt).toBe(
      '  Use realistic photography.\nKeep @Image 1 recognisable.  '
    )
    act(() => result.current.setPromptEnabled(true))
    act(() => result.current.applyStyle(null))
    expect(result.current.effectivePrompt).toBe(
      'Use realistic photography.\nKeep @Image 1 recognisable.'
    )
  })
})
