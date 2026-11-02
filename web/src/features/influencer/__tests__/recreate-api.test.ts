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

import { fetchRecreateAnalysis, startRecreateAnalysis } from '../api'

afterEach(() => {
  vi.restoreAllMocks()
})

const envelope = (data: unknown) => ({
  data: { success: true, message: '', data },
})

describe('startRecreateAnalysis', () => {
  it('returns the queued task id from the 202 envelope', async () => {
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      status: 202,
      ...envelope({ task_id: 'task-7', status: 'waiting', progress: '0%' }),
    } as never)

    const started = await startRecreateAnalysis({
      video_url: 'https://files.example/clip.mp4',
      image_urls: ['https://files.example/front.png'],
      image_tags: [],
      prompt: 'keep the turn',
      resolution: '480p',
    })

    expect(started).toEqual({
      task_id: 'task-7',
      status: 'waiting',
      progress: '0%',
    })
    expect(post).toHaveBeenCalledTimes(1)
    const [path, body, config] = post.mock.calls[0]
    expect(path).toBe('/pg/influencer/recreate/analyze')
    expect(body).toMatchObject({ resolution: '480p' })
    expect(config).not.toHaveProperty('timeout')
  })

  it('rejects a success envelope without a task id', async () => {
    vi.spyOn(api, 'post').mockResolvedValue({
      status: 202,
      ...envelope({ status: 'waiting' }),
    } as never)

    await expect(
      startRecreateAnalysis({
        video_url: 'https://files.example/clip.mp4',
        image_urls: ['https://files.example/front.png'],
        resolution: '480p',
      })
    ).rejects.toThrow(
      'Could not analyze the reference video. Please try again.'
    )
  })
})

describe('fetchRecreateAnalysis', () => {
  it('reads a running poll without requiring the final payload', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue(
      envelope({
        task_id: 'task-7',
        status: 'running',
        progress: '40%',
      }) as never
    )

    await expect(fetchRecreateAnalysis('task-7')).resolves.toEqual({
      task_id: 'task-7',
      status: 'running',
      progress: '40%',
      message: undefined,
    })
    expect(get.mock.calls[0][0]).toBe('/pg/influencer/recreate/task-7')
  })

  it('returns the compiled prompt on success and the message on failure', async () => {
    vi.spyOn(api, 'get')
      .mockResolvedValueOnce(
        envelope({
          task_id: 'task-7',
          status: 'success',
          prompt: 'Reference image 1 is the hero product.',
          analysis: { video_overview: { one_line: 'One take.' } },
          image_analysis: [],
          meta: { duration_sec: 8, pricing: { '480p': 6.47 } },
        }) as never
      )
      .mockResolvedValueOnce(
        envelope({
          task_id: 'task-7',
          status: 'failed',
          message: 'Reference video analysis timed out. Try again later.',
        }) as never
      )

    await expect(fetchRecreateAnalysis('task-7')).resolves.toMatchObject({
      status: 'success',
      prompt: 'Reference image 1 is the hero product.',
      meta: { duration_sec: 8 },
    })
    await expect(fetchRecreateAnalysis('task-7')).resolves.toMatchObject({
      status: 'failed',
      message: 'Reference video analysis timed out. Try again later.',
    })
  })
})
