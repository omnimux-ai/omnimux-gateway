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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { GatewayTransport } from '../gateway-transport'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))

vi.mock('@/lib/api', () => ({ api: { get, post } }))

const MODEL_ID = 'seedance-2-5'

type JsonRecord = globalThis.Record<string, unknown>

/** Routes list and artifact reads to their own responses, in any order. */
function respondWith(
  items: JsonRecord[],
  artifacts: JsonRecord[] = [],
  extraArtifactData: JsonRecord = {}
): void {
  get.mockImplementation(async (url: string) => {
    if (url === '/api/task/self') {
      return { data: { success: true, data: { items } } }
    }
    if (url.endsWith('/artifacts')) {
      return { data: { success: true, data: { artifacts, ...extraArtifactData } } }
    }
    throw new Error(`unexpected GET ${url}`)
  })
}

function taskRecord(overrides: JsonRecord = {}): JsonRecord {
  return {
    task_id: 'task_1',
    status: 'SUCCESS',
    quota: 250_000,
    submit_time: 1_700_000_000,
    properties: { origin_model_name: MODEL_ID, prompt: 'a cat' },
    ...overrides,
  }
}

let transport: GatewayTransport

beforeEach(() => {
  get.mockReset()
  post.mockReset()
  transport = new GatewayTransport()
})

describe('GatewayTransport submission', () => {
  test('reads the task id from the flat host-fallback body', async () => {
    post.mockResolvedValue({ data: { task_id: 'task_abc', id: 'task_abc' } })

    await expect(
      transport.submit({ modelId: MODEL_ID, params: { prompt: 'a cat' } })
    ).resolves.toEqual({ taskId: 'task_abc' })
  })

  test('falls back to the id field when task_id is absent', async () => {
    post.mockResolvedValue({ data: { id: 'task_only_id' } })

    await expect(
      transport.submit({ modelId: MODEL_ID, params: {} })
    ).resolves.toEqual({ taskId: 'task_only_id' })
  })

  test('accepts synchronous image generation response carrying task_id and id', async () => {
    post.mockResolvedValue({
      data: {
        id: 'task_sync_image_123',
        task_id: 'task_sync_image_123',
        created: 1790658579,
        data: [{ url: 'https://cdn.example/image.png' }],
      },
    })

    await expect(
      transport.submit({ modelId: MODEL_ID, params: { prompt: 'a prompt' } })
    ).resolves.toEqual({ taskId: 'task_sync_image_123' })
  })

  test('rejects a submission the gateway accepted without a task id', async () => {
    post.mockResolvedValue({ data: { status: 'queued' } })

    await expect(
      transport.submit({ modelId: MODEL_ID, params: {} })
    ).rejects.toThrow(/task id/i)
  })

  test('pins a resolved group through the header the gateway reads', async () => {
    post.mockResolvedValue({ data: { task_id: 'task_tier' } })

    await transport.submit({
      modelId: 'wan-3.0',
      params: { resolution: '720p' },
      group: 'wan-3.0-720p',
    })

    expect(post).toHaveBeenCalledWith(
      '/pg/video/generations',
      { model: 'wan-3.0', resolution: '720p' },
      {
        skipErrorHandler: true,
        headers: { 'X-Omnimux-Group': 'wan-3.0-720p' },
      }
    )
  })

  test('sends no group header when the caller resolved no group', async () => {
    post.mockResolvedValue({ data: { task_id: 'task_plain' } })

    await transport.submit({ modelId: MODEL_ID, params: {} })

    expect(post).toHaveBeenCalledWith(
      '/pg/video/generations',
      { model: MODEL_ID },
      { skipErrorHandler: true }
    )
  })
})

describe('GatewayTransport status mapping', () => {
  test('folds IN_PROGRESS to running and parses the percentage progress', async () => {
    respondWith([
      taskRecord({ status: 'IN_PROGRESS', progress: '42%', quota: 0 }),
    ])

    const task = await transport.getTask('task_1')

    expect(task.status).toBe('running')
    expect(task.progress).toBe(42)
  })

  test('maps SUCCESS to succeeded and carries playable media from artifacts', async () => {
    respondWith(
      [taskRecord()],
      [
        {
          key: 'video',
          type: 'video',
          mime_type: 'video/mp4',
          content_url: 'https://cdn.example/v.mp4',
        },
      ]
    )

    const task = await transport.getTask('task_1')

    expect(task.status).toBe('succeeded')
    expect(task.media).toEqual([
      { url: 'https://cdn.example/v.mp4', kind: 'video' },
    ])
  })

  test('carries legacy_content_url as playable video media when artifacts array is empty', async () => {
    respondWith([taskRecord()], [], {
      legacy_content_url: 'https://gateway.example/v1/tasks/task_1/artifacts/video/content',
    })

    const task = await transport.getTask('task_1')

    expect(task.status).toBe('succeeded')
    expect(task.media).toEqual([
      {
        url: 'https://gateway.example/v1/tasks/task_1/artifacts/video/content',
        kind: 'video',
      },
    ])
  })

  test('reports an unresolved task as pending rather than failed', async () => {
    respondWith([taskRecord({ status: 'UNKNOWN', progress: '' })])

    await expect(transport.getTask('task_1')).resolves.toMatchObject({
      status: 'queued',
      progress: 0,
    })
  })

  test('keeps the polled status when the media projection fails', async () => {
    get.mockImplementation(async (url: string) => {
      if (url === '/api/task/self') {
        return { data: { success: true, data: { items: [taskRecord()] } } }
      }
      throw new Error('artifacts unavailable')
    })

    await expect(transport.getTask('task_1')).resolves.toMatchObject({
      status: 'succeeded',
      media: [],
    })
  })

  test('falls back to result_url when media projection is empty', async () => {
    respondWith([
      taskRecord({
        result_url: 'https://omnimux.ai/v1/images/task_1/content',
      }),
    ])

    const task = await transport.getTask('task_1')
    expect(task.status).toBe('succeeded')
    expect(task.media).toEqual([
      {
        url: 'https://omnimux.ai/v1/images/task_1/content',
        kind: 'image',
      },
    ])
  })

  test('identifies mp4 video url in result_url as video instead of image', async () => {
    respondWith([
      taskRecord({
        result_url: 'https://cdn.example/output.mp4?sign=xyz',
      }),
    ])

    const task = await transport.getTask('task_1')
    expect(task.status).toBe('succeeded')
    expect(task.media).toEqual([
      {
        url: 'https://cdn.example/output.mp4?sign=xyz',
        kind: 'video',
      },
    ])
  })

  test('surfaces a task the session cannot read instead of inventing one', async () => {
    respondWith([])

    await expect(transport.getTask('task_missing')).rejects.toThrow(
      /not readable/i
    )
  })

  test('charges only a succeeded task and only from the recorded quota', async () => {
    respondWith([taskRecord({ quota: 250_000 })])

    await expect(transport.getTask('task_1')).resolves.toMatchObject({
      settledUsd: 0.5,
    })
  })
})

describe('GatewayTransport history', () => {
  test('keeps only records belonging to the requested model', async () => {
    respondWith([
      taskRecord({ task_id: 'mine' }),
      taskRecord({
        task_id: 'other',
        properties: { origin_model_name: 'other-model' },
      }),
    ])

    const history = await transport.listHistory(MODEL_ID)

    expect(history.map((task) => task.taskId)).toEqual(['mine'])
  })

  test('projects image artifacts for succeeded image tasks', async () => {
    respondWith([taskRecord({ task_id: 'task_img_1', status: 'SUCCESS' })])
    get.mockImplementation(async (path: string) => {
      if (path === '/api/task/self') {
        return {
          data: {
            success: true,
            data: {
              items: [taskRecord({ task_id: 'task_img_1', status: 'SUCCESS' })],
            },
          },
        }
      }
      if (path === '/api/task/task_img_1/artifacts') {
        return {
          data: {
            success: true,
            data: {
              artifacts: [
                {
                  key: 'image',
                  type: 'image',
                  mime_type: 'image/png',
                  content_url: 'https://gateway.example/v1/tasks/task_img_1/artifacts/image/content',
                },
              ],
            },
          },
        }
      }
      throw new Error(`unexpected get path: ${path}`)
    })

    const history = await transport.listHistory(MODEL_ID)

    expect(history[0]).toMatchObject({
      taskId: 'task_img_1',
      status: 'succeeded',
      media: [
        {
          kind: 'image',
          url: 'https://gateway.example/v1/tasks/task_img_1/artifacts/image/content',
        },
      ],
    })
  })

  test('skips secondary requests when artifacts are embedded by backend', async () => {
    respondWith([
      taskRecord({
        task_id: 'task_embedded_1',
        status: 'SUCCESS',
        artifacts: [
          {
            key: 'image',
            type: 'image',
            mime_type: 'image/png',
            content_url: 'https://cdn.example/embedded_image.png',
          },
        ],
      }),
    ])

    const history = await transport.listHistory(MODEL_ID)

    expect(history[0].media).toEqual([
      {
        url: 'https://cdn.example/embedded_image.png',
        kind: 'image',
      },
    ])
    // Only /api/task/self should be called, no /api/task/.../artifacts calls
    expect(get.mock.calls.map((call) => call[0])).toEqual(['/api/task/self'])
  })

  test('skips the media projection for a task that has not succeeded', async () => {
    respondWith([taskRecord({ status: 'FAILURE', fail_reason: 'upstream 500' })])

    const history = await transport.listHistory(MODEL_ID)

    expect(history[0]).toMatchObject({
      status: 'failed',
      failReason: 'upstream 500',
      media: [],
    })
    expect(get.mock.calls.map((call) => call[0])).toEqual(['/api/task/self'])
  })
})

describe('GatewayTransport uploads', () => {
  test('returns the stored file URL', async () => {
    post.mockResolvedValue({
      data: { success: true, data: { file_url: 'https://cdn.example/a.png' } },
    })

    const file = new File(['x'], 'a.png', { type: 'image/png' })

    await expect(transport.uploadReference(file)).resolves.toEqual({
      url: 'https://cdn.example/a.png',
    })
  })

  test('rejects an upload the gateway reported as failed', async () => {
    post.mockResolvedValue({
      data: { success: false, msg: '配额已耗尽' },
    })

    const file = new File(['x'], 'a.png', { type: 'image/png' })

    await expect(transport.uploadReference(file)).rejects.toThrow(/配额已耗尽/)
  })
})
