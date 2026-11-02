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
import { api } from '@/lib/api'
import { createServerError, requireServerSuccess } from '@/lib/server-error-message'

import type {
  GenerationTask,
  GenerationTransport,
  PlaygroundParams,
  SubmitRequest,
  TaskMedia,
  TaskStatus,
  UploadResult,
} from '../types'

/**
 * Session-authenticated gateway endpoints.
 *
 * `POST /pg/video/generations` is the only fork-added route: submission needs a
 * relay path because `/v1/video/generations` requires a personal access token.
 * Reading tasks back reuses the stock dashboard endpoints, which already accept
 * the session and are the sanctioned source for a task's media.
 */
const VIDEO_SUBMIT_PATH = '/pg/video/generations'
const IMAGE_SUBMIT_PATH = '/pg/images/generations'
const IMAGE_EDIT_SUBMIT_PATH = '/pg/images/edits'
const TASK_LIST_PATH = '/api/task/self'
const UPLOAD_PATH = '/api/v1/files/upload/stream'

/**
 * An image submission carrying reference material goes to the edit surface: the
 * same chain, but the adaptor selects the upstream's images edit endpoint.
 * Without a reference the generate path is untouched.
 */
function imageSubmitPath(params: PlaygroundParams): string {
  const references = params.images
  if (Array.isArray(references) && references.length > 0) {
    return IMAGE_EDIT_SUBMIT_PATH
  }
  return IMAGE_SUBMIT_PATH
}

const HISTORY_PAGE_SIZE = 20

/** raw_quota ÷ 500000 = USD (see AGENTS.md, quota display). */
const QUOTA_PER_USD = 500_000

/**
 * Backend task statuses (`model/task.go` TaskStatus) folded onto the page
 * vocabulary. UNKNOWN stays queued so an unresolved task keeps polling instead
 * of being reported as a failure it may not be.
 */
const STATUS_BY_BACKEND: Record<string, TaskStatus> = {
  NOT_START: 'queued',
  SUBMITTED: 'queued',
  QUEUED: 'queued',
  IN_PROGRESS: 'running',
  SUCCESS: 'succeeded',
  FAILURE: 'failed',
}

type TaskRecord = {
  task_id?: string
  status?: string
  progress?: string
  fail_reason?: string
  quota?: number
  submit_time?: number
  finish_time?: number
  properties?: { origin_model_name?: string }
  result_url?: string
  artifacts?: ArtifactItem[]
  legacy_content_url?: string
}

type ArtifactItem = {
  key?: string
  type?: string
  mime_type?: string
  content_url?: string
}

type TaskListEnvelope = {
  success?: boolean
  message?: string
  data?: { items?: TaskRecord[] }
}

type ArtifactEnvelope = {
  success?: boolean
  message?: string
  data?: { artifacts?: ArtifactItem[]; legacy_content_url?: string }
}

type UploadEnvelope = {
  success?: boolean
  msg?: string
  data?: { file_url?: string }
}

type SubmitEnvelope = { task_id?: string; id?: string }

function toTaskStatus(status: string | undefined): TaskStatus {
  if (!status) return 'queued'
  return STATUS_BY_BACKEND[status.toUpperCase()] ?? 'queued'
}

/** Backend progress is a display string such as "42%"; the page wants 0-100. */
function toProgress(progress: string | undefined): number {
  if (!progress) return 0
  const digits = progress.match(/\d+(?:\.\d+)?/)
  if (!digits) return 0
  return Math.min(100, Math.max(0, Number(digits[0])))
}

/** Keeps only values the page's parameter contract can carry. */
function toParams(source: unknown): PlaygroundParams {
  if (!source || typeof source !== 'object') return {}

  const params: PlaygroundParams = {}
  for (const [key, value] of Object.entries(source as Record<string, unknown>)) {
    if (
      typeof value === 'string' ||
      typeof value === 'number' ||
      typeof value === 'boolean'
    ) {
      params[key] = value
      continue
    }
    if (Array.isArray(value) && value.every((item) => typeof item === 'string')) {
      params[key] = value
    }
  }
  return params
}

function toMedia(item: ArtifactItem): TaskMedia | null {
  if (!item.content_url) return null
  const marker = `${item.mime_type ?? ''} ${item.type ?? ''} ${item.key ?? ''}`
  return {
    url: item.content_url,
    kind: marker.toLowerCase().includes('video') ? 'video' : 'image',
  }
}

function isVideoUrl(url: string): boolean {
  if (/\.(mp4|webm|mov|m4v|mkv|avi|flv)($|\?)/i.test(url)) return true
  if (url.includes('/videos/') || url.includes('/video/')) return true
  return false
}

function toGenerationTask(
  record: TaskRecord,
  media: TaskMedia[],
  settleUsd: boolean
): GenerationTask {
  const status = toTaskStatus(record.status)
  const resolvedMedia = [...media]
  if (
    resolvedMedia.length === 0 &&
    record.result_url &&
    status === 'succeeded'
  ) {
    const isVideo = isVideoUrl(record.result_url)
    resolvedMedia.push({
      url: record.result_url,
      kind: isVideo ? 'video' : 'image',
    })
  }

  const task: GenerationTask = {
    taskId: record.task_id ?? '',
    modelId: record.properties?.origin_model_name ?? '',
    status,
    progress: toProgress(record.progress),
    params: toParams(record.properties),
    estimatedUsd: null,
    settledUsd: null,
    createdAt: record.submit_time ?? 0,
    media: resolvedMedia,
  }

  if (record.finish_time) task.finishedAt = record.finish_time
  if (record.fail_reason) task.failReason = record.fail_reason
  // A succeeded task's quota is what was actually charged. Anything unresolved
  // is left null rather than reported as a charge.
  if (settleUsd && status === 'succeeded' && typeof record.quota === 'number') {
    task.settledUsd = record.quota / QUOTA_PER_USD
  }

  return task
}

export class GatewayTransport implements GenerationTransport {
  readonly mocked = false

  /**
   * A reference image is uploaded once and handed to the gateway by URL, so the
   * task submission stays a plain JSON body.
   */
  async uploadReference(file: File): Promise<UploadResult> {
    const form = new FormData()
    form.append('file', file)

    const res = await api.post(UPLOAD_PATH, form)
    const body = res.data as UploadEnvelope
    // The upload envelope carries its detail in `msg`, which the shared message
    // reader does not look at, so it is passed as the fallback explicitly.
    if (body.success === false) {
      throw createServerError(body, body.msg)
    }

    const url = body.data?.file_url
    if (!url) {
      throw createServerError(body, body.msg)
    }
    return { url }
  }

  async submit(request: SubmitRequest): Promise<{ taskId: string }> {
    const submitPath =
      request.modality === 'image'
        ? imageSubmitPath(request.params)
        : VIDEO_SUBMIT_PATH
    // An explicit group travels as the header the gateway reads, so the group
    // the caller chose is the one routing and billing both use. No group means
    // the option is left out entirely, keeping the gateway's own selection.
    const options: Record<string, unknown> = { skipErrorHandler: true }
    if (request.group) {
      options.headers = { 'X-Omnimux-Group': request.group }
    }
    const res = await api.post(
      submitPath,
      { model: request.modelId, ...request.params },
      options
    )

    const body = res.data as SubmitEnvelope
    const taskId = body?.task_id || body?.id
    if (!taskId) {
      throw new Error('Gateway did not return a task id')
    }
    return { taskId }
  }

  async getTask(taskId: string): Promise<GenerationTask> {
    const record = await fetchTaskRecord(taskId)
    if (!record) {
      throw new Error(`Task ${taskId} is not readable from this session`)
    }

    // Media is projected by a second endpoint; a projection failure must not
    // hide the status the page is polling for.
    const media = await fetchTaskMedia(taskId).catch(() => [])
    return toGenerationTask(record, media, true)
  }

  async listHistory(modelId: string): Promise<GenerationTask[]> {
    const res = await api.get(TASK_LIST_PATH, {
      params: { page: 1, page_size: HISTORY_PAGE_SIZE, with_artifacts: true },
    })
    const body = res.data as TaskListEnvelope
    requireServerSuccess(body)

    const items = body.data?.items ?? []
    // A record without an origin model cannot be attributed to this page, so it
    // is left out rather than shown under the wrong model.
    const scoped = items.filter(
      (record) => record.properties?.origin_model_name === modelId
    )

    const withMedia = await Promise.all(
      scoped.map(async (record) => {
        if (toTaskStatus(record.status) !== 'succeeded') {
          return toGenerationTask(record, [], false)
        }
        // When backend embeds artifacts, resolve them without secondary N+1 requests
        if (Array.isArray(record.artifacts) || record.legacy_content_url) {
          const directMedia: TaskMedia[] = (record.artifacts ?? [])
            .map(toMedia)
            .filter((item): item is TaskMedia => item !== null)
          if (directMedia.length === 0 && record.legacy_content_url) {
            directMedia.push({
              url: record.legacy_content_url,
              kind: 'video',
            })
          }
          return toGenerationTask(record, directMedia, true)
        }
        // Fallback for legacy endpoints
        const media = await fetchTaskMedia(record.task_id ?? '').catch(() => [])
        return toGenerationTask(record, media, true)
      })
    )

    return withMedia
  }
}

async function fetchTaskRecord(taskId: string): Promise<TaskRecord | null> {
  const res = await api.get(TASK_LIST_PATH, {
    params: { task_id: taskId, page: 1, page_size: 1 },
  })
  const body = res.data as TaskListEnvelope
  requireServerSuccess(body)

  const items = body.data?.items ?? []
  return items.find((record) => record.task_id === taskId) ?? items[0] ?? null
}

async function fetchTaskMedia(taskId: string): Promise<TaskMedia[]> {
  if (!taskId) return []

  const res = await api.get(`/api/task/${encodeURIComponent(taskId)}/artifacts`)
  const body = res.data as ArtifactEnvelope
  requireServerSuccess(body)

  const media = (body.data?.artifacts ?? [])
    .map(toMedia)
    .filter((item): item is TaskMedia => item !== null)

  if (media.length === 0 && body.data?.legacy_content_url) {
    media.push({
      url: body.data.legacy_content_url,
      kind: 'video',
    })
  }

  return media
}
