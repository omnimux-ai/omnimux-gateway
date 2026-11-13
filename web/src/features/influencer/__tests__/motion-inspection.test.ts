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
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { inspectMotionVideo } from '../api'

vi.mock('@/lib/api', () => ({ api: { post: vi.fn() } }))
afterEach(() => vi.resetAllMocks())

describe('reference video inspection request', () => {
  it('checks the full original URL for the requested mode and keeps a legal nonstandard ratio', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: {
        success: true,
        data: {
          duration: 9.708333,
          width: 1280,
          height: 1024,
          frame_rate: 30,
          video_codec: 'hevc',
          container: 'mov',
        },
      },
    })
    const signal = new AbortController().signal
    await expect(
      inspectMotionVideo(
        'https://files.example/original.mov',
        'replace',
        signal
      )
    ).resolves.toEqual({
      rawDurationSec: 9.708333,
      durationSec: 10,
      width: 1280,
      height: 1024,
      aspectRatio: null,
    })
    expect(api.post).toHaveBeenCalledWith(
      '/pg/influencer/motion/inspect',
      {
        video_url: 'https://files.example/original.mov',
        mode: 'replace',
      },
      { signal }
    )
  })

  it('keeps a business rejection as a failed selection with its real reason', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: false, message: 'The original video is too short.' },
    })
    await expect(
      inspectMotionVideo('https://files.example/a.mp4', 'replace')
    ).rejects.toThrow('The original video is too short.')
  })

  it('does not accept incomplete successful inspection data', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: true, data: { duration: 8, width: 1280 } },
    })
    await expect(
      inspectMotionVideo('https://files.example/a.mp4', 'motion')
    ).rejects.toThrow('Could not prepare video')
  })
})
