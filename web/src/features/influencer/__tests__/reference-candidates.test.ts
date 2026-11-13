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

import { describe, expect, it } from 'vitest'

import {
  referenceImageTasks,
  referenceVideoCandidate,
  referenceVideoCandidates,
} from '../lib/reference-candidates'
import type { TaskRecord } from '../types'

function motion(overrides: Partial<TaskRecord> = {}): TaskRecord {
  return {
    id: 1,
    task_id: 'motion-1',
    action: 'influencer_motion',
    status: 'SUCCESS',
    artifacts: [
      {
        key: 'video',
        content_url:
          'https://media.example.com/clip.mp4?access=permission&x=%2F',
      },
    ],
    ...overrides,
  }
}

type TypedArtifact = NonNullable<TaskRecord['artifacts']>[number] & {
  type?: string
  mime_type?: string
}

function artifactTask(
  artifact: TypedArtifact,
  overrides: Partial<TaskRecord> = {}
): TaskRecord {
  return motion({ artifacts: [artifact], ...overrides })
}

describe('referenceVideoCandidates', () => {
  it('空列表返回空候选', () => {
    expect(referenceVideoCandidates([])).toEqual([])
  })

  it('过滤隐藏失败无身份记录且保持输入顺序', () => {
    const items = [
      motion({ task_id: 'visible-2' }),
      motion({ task_id: 'hidden' }),
      motion({ status: 'FAILURE' }),
      motion({ task_id: '' }),
      motion({ task_id: 'visible-1' }),
    ]
    expect(
      referenceVideoCandidates(items, new Set(['hidden'])).map(
        (candidate) => candidate.taskId
      )
    ).toEqual(['visible-2', 'visible-1'])
  })
})

describe('referenceImageTasks', () => {
  it('成功角色图片与多视角保留原始任务，不限制张数', () => {
    const sheet = motion({
      action: 'influencer_sheet',
      artifacts: [{ key: 'image', content_url: '/sheet.png' }],
    })
    const multiview = motion({
      action: undefined,
      status: 'succeeded',
      properties: { input: '{"kind":"multiview"}' },
      artifacts: [{ key: 'image', content_url: '/turnaround.png' }],
    })
    expect(referenceImageTasks([sheet, multiview])).toEqual([sheet, multiview])
    expect(
      referenceImageTasks(Array.from({ length: 31 }, () => sheet))
    ).toHaveLength(31)
  })

  it('失败排队motion无身份无产物图片不入选', () => {
    expect(
      referenceImageTasks([
        motion(),
        motion({
          action: 'influencer_sheet',
          status: 'FAILURE',
          result_url: '/sheet.png',
        }),
        motion({
          action: 'influencer_sheet',
          status: 'QUEUED',
          result_url: '/sheet.png',
        }),
        motion({
          action: 'influencer_sheet',
          task_id: '',
          result_url: '/sheet.png',
        }),
        motion({ action: 'influencer_sheet', artifacts: [] }),
      ])
    ).toEqual([])
  })

  it.each([
    { key: 'video', content_url: '/clip' },
    { key: 'render', type: 'video', content_url: '/clip' },
    { key: 'render', mime_type: 'video/mp4', content_url: '/clip' },
    { key: 'render', content_url: '/clip.mp4' },
  ])('非motion但明确视频产物%s从图片候选排除', (artifact) => {
    expect(
      referenceImageTasks([
        artifactTask(artifact, { action: 'influencer_sheet' }),
      ])
    ).toEqual([])
  })

  it('声明video却缺视频地址时不退回旧图片', () => {
    expect(
      referenceImageTasks([
        motion({
          action: 'influencer_sheet',
          artifacts: [{ key: 'video' }],
          result_url: '/sheet.png',
        }),
      ])
    ).toEqual([])
  })

  it('旧成功角色图片保留但旧视频结果排除', () => {
    const sheet = motion({
      action: 'influencer_sheet',
      artifacts: [],
      result_url: '/sheet.jpg',
    })
    expect(
      referenceImageTasks([
        sheet,
        motion({
          action: 'influencer_sheet',
          artifacts: [],
          result_url: '/clip.mp4',
        }),
      ])
    ).toEqual([sheet])
  })
})

describe('referenceVideoCandidate', () => {
  it('外域成功视频保留完整预览来源和未经改写的引用地址', () => {
    expect(referenceVideoCandidate(motion())).toEqual({
      taskId: 'motion-1',
      artifactKey: 'video',
      previewUrl: 'https://media.example.com/clip.mp4?access=permission&x=%2F',
      referenceUrl:
        'https://media.example.com/clip.mp4?access=permission&x=%2F',
      title: '',
    })
  })

  it('持久化的服务端元数据随候选返回，供选片免读取复用', () => {
    const candidate = referenceVideoCandidate(
      artifactTask({
        key: 'video',
        content_url: 'https://media.example.com/clip.mp4?access=permission',
        video_metadata: {
          duration: 7.072,
          video_duration: 7.042,
          width: 480,
          height: 854,
          frame_rate: 24,
          video_codec: 'h264',
          audio_codec: 'aac',
          container: 'mp4',
          size_bytes: 6650581,
        },
      })
    )
    expect(candidate?.metadata).toEqual({
      version: 1,
      browserMetadata: {
        duration: 7.072,
        video_duration: 7.042,
        width: 480,
        height: 854,
        frame_rate: 24,
        video_codec: 'h264',
        audio_codec: 'aac',
        container: 'mp4',
        size_bytes: 6650581,
      },
      rawDurationSec: 7.072,
      durationSec: 7,
      width: 480,
      height: 854,
      aspectRatio: '9:16',
    })
  })

  it('不完整的持久元数据不构成可信记录，保留原有读取回退', () => {
    const candidate = referenceVideoCandidate(
      artifactTask({
        key: 'video',
        content_url: 'https://media.example.com/clip.mp4?access=permission',
        video_metadata: {
          duration: 7.072,
          video_duration: 7.042,
          width: 480,
          height: 854,
          frame_rate: 0,
          video_codec: 'h264',
          audio_codec: 'aac',
          container: 'mp4',
          size_bytes: 6650581,
        },
      })
    )
    expect(candidate).not.toBeNull()
    expect(candidate?.metadata).toBeUndefined()
  })

  it('旧成功状态且输入只含prompt时仍可读取标题和封面', () => {
    const candidate = referenceVideoCandidate(
      motion({
        status: 'succeeded',
        properties: {
          input: JSON.stringify({ prompt: '  a dancer  ', brief: 'brief' }),
        },
        poster_url: 'https://media.example.com/poster.jpg?access=cover',
      })
    )
    expect(candidate).toMatchObject({
      title: 'a dancer',
      posterUrl: '/poster.jpg?access=cover',
    })
  })

  it.each(['FAILURE', 'failed', 'QUEUED', 'IN_PROGRESS', 'complete', ''])(
    '状态%s不作为成功候选',
    (status) => {
      expect(referenceVideoCandidate(motion({ status }))).toBeNull()
    }
  )

  it.each(['', '   '])('无有效任务身份%s拒绝候选', (task_id) => {
    expect(referenceVideoCandidate(motion({ task_id }))).toBeNull()
  })

  it('非motion任务即使有video也拒绝候选', () => {
    expect(
      referenceVideoCandidate(motion({ action: 'influencer_sheet' }))
    ).toBeNull()
  })

  it('motion元数据不要求角色selection且brief可作标题', () => {
    expect(
      referenceVideoCandidate(
        motion({
          action: undefined,
          properties: {
            input: JSON.stringify({
              kind: 'motion',
              brief: '  dancer brief  ',
            }),
          },
        })
      )?.title
    ).toBe('dancer brief')
  })

  it.each([
    { key: 'render', type: 'video' },
    { key: 'render', mime_type: 'video/mp4' },
    { key: 'render' },
  ])('明确视频产物%s保留真实artifact身份并预览自身地址', (artifact) => {
    expect(
      referenceVideoCandidate(
        artifactTask(
          {
            ...artifact,
            content_url: 'https://media.example.com/render.mp4?access=render',
          },
          {
            legacy_content_url: 'https://media.example.com/wrong.mp4',
          }
        )
      )
    ).toMatchObject({
      artifactKey: 'render',
      previewUrl: '/render.mp4?access=render',
    })
  })

  it.each([
    { key: 'image', content_url: 'https://media.example.com/image.png' },
    {
      key: 'video',
      type: 'image',
      content_url: 'https://media.example.com/file',
    },
    {
      key: 'video',
      mime_type: 'image/png',
      content_url: 'https://media.example.com/file',
    },
    { key: 'video', content_url: 'https://media.example.com/image.jpg' },
    { key: 'render', content_url: 'https://media.example.com/file' },
    { key: 'video', content_url: '' },
  ])('无视频产物%s不借旧地址造成误配', (artifact) => {
    expect(
      referenceVideoCandidate(
        artifactTask(artifact, {
          result_url: 'https://media.example.com/old.mp4',
        })
      )
    ).toBeNull()
  })

  it.each([
    {
      legacy_content_url:
        'https://media.example.com/v1/tasks/motion-1/artifacts/video/content?access=legacy',
      expected: '/v1/tasks/motion-1/artifacts/video/content?access=legacy',
      artifactKey: 'video',
    },
    {
      result_url: 'https://media.example.com/legacy.mov?access=result',
      expected: '/legacy.mov?access=result',
      artifactKey: 'video',
    },
  ])(
    '无artifacts的明确旧motion视频%s仍能预览',
    ({ expected, artifactKey, ...urls }) => {
      expect(
        referenceVideoCandidate(motion({ artifacts: undefined, ...urls }))
      ).toMatchObject({ artifactKey, previewUrl: expected })
    }
  )

  it('旧motion的任意result地址不构成明确视频产物', () => {
    expect(
      referenceVideoCandidate(
        motion({ artifacts: [], result_url: 'https://media.example.com/file' })
      )
    ).toBeNull()
  })

  it('content_url为空时使用同产物url而不丢失预览', () => {
    expect(
      referenceVideoCandidate(
        artifactTask({
          key: 'video',
          content_url: '',
          url: 'https://media.example.com/fallback.mp4',
        })
      )
    ).toMatchObject({
      previewUrl: 'https://media.example.com/fallback.mp4',
      referenceUrl: 'https://media.example.com/fallback.mp4',
    })
  })

  it.each([
    '/v1/tasks/motion-1/artifacts/video/content?access=relative',
    'http://media.example.com/clip.mp4',
    'https://user:pass@media.example.com/clip.mp4',
    'https://media.example.com/clip.mp4#section',
    'https://media.example.com/clip.mp4#',
    'https://localhost/clip.mp4',
    'https://localhost./clip.mp4',
    'https://sub.localhost/clip.mp4',
    'https://127.0.0.1/clip.mp4',
    'https://2130706433/clip.mp4',
    'https://0x7f000001/clip.mp4',
    'https://8.8.8.8/clip.mp4',
    'https://[::1]/clip.mp4',
    'https://[2606:4700:4700::1111]/clip.mp4',
    'https://192.168.1.1/clip.mp4',
    'https://camera/clip.mp4',
    'https://camera.local/clip.mp4',
    'https://camera.internal/clip.mp4',
    'https://camera.lan/clip.mp4',
    'https://camera.home.arpa/clip.mp4',
    'https://media.example.com\\@localhost/clip.mp4',
    ' https://media.example.com/clip.mp4',
    'https://media.example.com/clip.mp4\n',
  ])('不安全或不完整引用地址%s不可提交但不拼当前主机', (content_url) => {
    expect(
      referenceVideoCandidate(artifactTask({ key: 'video', content_url }))
        ?.referenceUrl
    ).toBeNull()
  })

  it.each([
    'blob:https://media.example.com/id',
    'data:video/mp4;base64,AAAA',
    '//media.example.com/clip.mp4',
    'javascript:alert(1)',
  ])('不能同源播放的地址%s不显示候选', (content_url) => {
    expect(
      referenceVideoCandidate(artifactTask({ key: 'video', content_url }))
    ).toBeNull()
  })

  it('完整HTTPS含端口和原始查询编码保留原文', () => {
    const content_url =
      'https://Media.Example.com:8443/clip.mp4?access=a%2Fb+z&x=1&x=2'
    expect(
      referenceVideoCandidate(artifactTask({ key: 'video', content_url }))
        ?.referenceUrl
    ).toBe(content_url)
  })

  it.each([{ type: 'video' }, { mime_type: 'video/mp4' }])(
    '无扩展名地址通过明确产物类型%s识别',
    (descriptor) => {
      expect(
        referenceVideoCandidate(
          artifactTask({
            key: 'render',
            ...descriptor,
            content_url: 'https://media.example.com/content?access=typed',
          })
        )?.artifactKey
      ).toBe('render')
    }
  )

  it('混合图片视频产物仅使用真实视频产物地址', () => {
    expect(
      referenceVideoCandidate(
        motion({
          artifacts: [
            {
              key: 'image',
              content_url: 'https://media.example.com/cover.jpg',
            },
            {
              key: 'video',
              content_url: 'https://media.example.com/movie.mp4',
            },
          ],
        })
      )
    ).toMatchObject({
      artifactKey: 'video',
      referenceUrl: 'https://media.example.com/movie.mp4',
    })
  })

  it('无效封面地址不泄漏跨源播放投影', () => {
    expect(
      referenceVideoCandidate(
        motion({ poster_url: '//media.example.com/cover.jpg' })
      )?.posterUrl
    ).toBeUndefined()
  })

  it('视频地址缺失时拒绝候选', () => {
    expect(referenceVideoCandidate(motion({ artifacts: undefined }))).toBeNull()
  })

  it.each(['not json', 'null', '[]', '{"prompt":7,"brief":false}'])(
    '坏元数据%s使用空标题不抛错',
    (input) => {
      expect(
        referenceVideoCandidate(motion({ properties: { input } }))?.title
      ).toBe('')
    }
  )
})
