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
import i18next from 'i18next'

import { api } from '@/lib/api'
import {
  createServerError,
  requireServerSuccess,
} from '@/lib/server-error-message'

import { SUNBURST_MODEL } from './lib/catalog'
import { motionAspectRatio } from './lib/motion'
import { isTerminalStatus } from './lib/task-status'
import type {
  InfluencerTaxonomy,
  ImageMode,
  MotionAspectRatio,
  MotionMode,
  MotionVideoMetadata,
  PricingResponse,
  PublishedPresetRow,
  RecreateAnalysis,
  RecreateAnalyzeResponse,
  RecreateImageAnalysis,
  RecreateMeta,
  Selection,
  TaskRecord,
} from './types'

const TAXONOMY_PATH = '/api/influencer/taxonomy'
const SHEET_PATH = '/pg/influencer/sheet'
const MULTIVIEW_PATH = '/pg/influencer/multiview'
const PRICING_PATH = '/api/pricing'
const TASK_LIST_PATH = '/api/task/self'
const RECREATE_PATH = '/pg/influencer/recreate'
const RECREATE_ANALYZE_PATH = `${RECREATE_PATH}/analyze`
export const INFLUENCER_ACTION = 'influencer_sheet'
export const SHEET_SIZE = '16:9'
/** Fixed request target of the reviewed Sunburst image contract. */
export const IMAGE_SIZE = '1024x576'
/** Video tasks from the workbench's video tab carry their own action. */
export const INFLUENCER_MOTION_ACTION = 'influencer_motion'
const MOTION_PATH = '/pg/influencer/motion'
const GROUPS_PATH = '/api/user/self/groups'
const UPLOAD_PATH = '/api/v1/files/upload/stream'
const PRESETS_PATH = '/api/influencer/presets'

export async function fetchTaxonomy(): Promise<{
  taxonomy: InfluencerTaxonomy
  etag: string
}> {
  const res = await api.get(TAXONOMY_PATH)
  requireServerSuccess(res.data as Record<string, unknown>)
  const body = res.data as { data: InfluencerTaxonomy; etag: string }
  return { taxonomy: body.data, etag: body.etag }
}

export async function fetchPricing(): Promise<PricingResponse> {
  const res = await api.get(PRICING_PATH)
  const body = res.data as PricingResponse
  requireServerSuccess(body as unknown as Record<string, unknown>)
  return body
}

export async function fetchUserGroups(): Promise<
  Record<string, { ratio: number; desc?: string }>
> {
  const res = await api.get(GROUPS_PATH)
  const body = res.data as {
    success: boolean
    data: Record<string, { ratio: number; desc?: string }>
  }
  requireServerSuccess(body as unknown as Record<string, unknown>)
  return body.data ?? {}
}

export interface SheetSubmitBody {
  model: string
  group?: string
  tier: string
  selection: Selection
  brief: string
  seed?: number
  size: string
  image_url?: string
  image_mode?: ImageMode
  mask_url?: string
  edit_instruction?: string
}

export async function submitSheet(body: SheetSubmitBody): Promise<string> {
  const headers: Record<string, string> = {}
  if (body.group) {
    headers['X-Omnimux-Group'] = body.group
  }
  const sunburst = body.model === SUNBURST_MODEL
  const imageUrl = body.image_url?.trim() || undefined
  const payload: SheetSubmitBody = {
    ...body,
    brief: '',
    size: sunburst ? IMAGE_SIZE : SHEET_SIZE,
    image_url: imageUrl,
  }
  // Legacy draft and history fields are never creation intent in this form.
  delete payload.mask_url
  delete payload.edit_instruction
  if (sunburst) {
    payload.image_mode = imageUrl ? 'reference' : 'text'
    delete payload.seed
  } else {
    // The audited economy route rejects workbench-only fields; text vs
    // reference is derived from `image_url` by the gateway.
    delete payload.image_mode
  }
  const res = await api.post(SHEET_PATH, payload, {
    headers,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  const data = res.data as {
    task_id?: string
    id?: string
    error?: { message?: string }
    message?: string
  }
  const taskId = data?.task_id ?? data?.id
  if (!taskId) {
    throw new Error(
      data?.error?.message ??
        data?.message ??
        'Could not submit the task. Please try again.'
    )
  }
  return taskId
}

export interface MultiViewSubmitBody {
  model: string
  group?: string
  /** The character sheet the board is derived from. */
  parentTaskId: string
  /** Absolute public artifact URL of the parent render. */
  imageUrl: string
  size?: string
}

/**
 * Submits a multi-view turnaround sheet derived from an existing character
 * sheet. The gateway reads the character description from the parent task, so
 * the caller only supplies the reference image and the parent id.
 */
export async function submitMultiView(
  body: MultiViewSubmitBody
): Promise<string> {
  const headers: Record<string, string> = {}
  if (body.group) {
    headers['X-Omnimux-Group'] = body.group
  }
  const res = await api.post(
    MULTIVIEW_PATH,
    {
      model: body.model,
      group: body.group,
      parent_task_id: body.parentTaskId,
      image_url: body.imageUrl,
      size: body.size ?? '16:9',
    },
    {
      headers,
      skipErrorHandler: true,
    } as Record<string, unknown>
  )
  const data = res.data as {
    task_id?: string
    id?: string
    error?: { message?: string }
    message?: string
  }
  const taskId = data?.task_id ?? data?.id
  if (!taskId) {
    throw new Error(
      data?.error?.message ??
        data?.message ??
        'Could not submit the multi-view task. Please try again.'
    )
  }
  return taskId
}

/** Output framing options the bound video lines evidence per resolution. */
export interface MotionOptions {
  version: string
  resolutions: Record<string, { aspect_ratios: MotionAspectRatio[] }>
}

/**
 * Read-only capability surface for the video form. The gateway stays the
 * source of truth: the panel offers only what the bound lines can produce.
 * A fetch failure falls back to the typed local mirror rather than widening
 * the offer beyond the last verified contract.
 */
export async function fetchMotionOptions(): Promise<MotionOptions | null> {
  try {
    const res = await api.get(`${MOTION_PATH}/options`)
    const data = res.data as {
      success?: boolean
      data?: {
        version?: string
        resolutions?: Record<string, { aspect_ratios?: string[] }>
      }
    }
    const body = data?.data
    if (!data?.success || !body?.resolutions) return null
    const resolutions: MotionOptions['resolutions'] = {}
    for (const [key, value] of Object.entries(body.resolutions)) {
      const ratios = (value?.aspect_ratios ?? []).filter(
        (v): v is MotionAspectRatio =>
          v === '16:9' || v === '9:16' || v === '3:4'
      )
      if (ratios.length) resolutions[key] = { aspect_ratios: ratios }
    }
    if (!Object.keys(resolutions).length) return null
    return { version: body.version ?? '', resolutions }
  } catch {
    return null
  }
}

export interface MotionSubmitBody {
  video_metadata?: import('./lib/browser-video-metadata').BrowserVideoMetadata
  mode: MotionMode
  prompt: string
  subject_image_url: string
  subject_image_urls?: string[]
  subject_image_tags?: string[]
  subject_character_type?: 'human' | 'cat' | 'dog'
  source_task_id?: string
  /** Absent for `recreate`: its clip was decomposition material only. */
  video_url?: string
  duration_sec: number
  aspect_ratio: MotionAspectRatio | 'adaptive'
  resolution: string
}

/** The reference clip plus the caller's own images, sent for decomposition. */
export interface RecreateAnalyzeBody {
  video_url: string
  image_urls: string[]
  /** Planner context only; the images' upload order still defines image N. */
  image_tags?: string[]
  prompt?: string
  /** Kept for the price tier only; the output duration comes from the clip. */
  resolution: string
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value) && typeof value === 'object' && !Array.isArray(value)
}

/** The accepted decomposition, as returned by the poll's success state. */
function readRecreateSuccess(
  data: Record<string, unknown>,
  source: unknown
): RecreateAnalyzeResponse {
  const prompt = typeof data.prompt === 'string' ? data.prompt.trim() : ''
  if (!prompt) {
    throw createServerError(
      source,
      'Could not analyze the reference video. Please try again.'
    )
  }
  const analysis = isRecord(data.analysis)
    ? (data.analysis as RecreateAnalysis)
    : null
  const imageAnalysis = Array.isArray(data.image_analysis)
    ? (data.image_analysis.filter(isRecord) as RecreateImageAnalysis[])
    : []
  return {
    prompt,
    analysis,
    image_analysis: imageAnalysis,
    meta: isRecord(data.meta) ? (data.meta as RecreateMeta) : null,
  }
}

/** The queued job accepted by `POST /recreate/analyze` (HTTP 202). */
export interface RecreateAnalysisStart {
  task_id: string
  status?: string
  progress?: string
}

/** One poll of the queued decomposition. */
export interface RecreateAnalysisTask {
  task_id: string
  status: string
  progress?: string
  message?: string
  prompt?: string
  analysis?: RecreateAnalysis | null
  image_analysis?: RecreateImageAnalysis[] | null
  meta?: RecreateMeta | null
}

/**
 * Enqueues the decomposition and returns its task id. The request only
 * enqueues work, so it must not carry a long timeout.
 */
export async function startRecreateAnalysis(
  body: RecreateAnalyzeBody
): Promise<RecreateAnalysisStart> {
  const res = await api.post(RECREATE_ANALYZE_PATH, body, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  const envelope = res.data as { success?: boolean } & Record<string, unknown>
  if (envelope?.success !== true || !isRecord(envelope.data)) {
    throw createServerError(
      envelope,
      'Could not analyze the reference video. Please try again.'
    )
  }
  const taskId = envelope.data.task_id
  if (typeof taskId !== 'string' || !taskId.trim()) {
    throw createServerError(
      envelope,
      'Could not analyze the reference video. Please try again.'
    )
  }
  return {
    task_id: taskId,
    status:
      typeof envelope.data.status === 'string'
        ? envelope.data.status
        : undefined,
    progress:
      typeof envelope.data.progress === 'string'
        ? envelope.data.progress
        : undefined,
  }
}

/**
 * Reads one poll of the analysis. `success` carries the same payload the old
 * synchronous endpoint returned; `failed` carries a user-readable message.
 */
export async function fetchRecreateAnalysis(
  taskId: string,
  signal?: AbortSignal
): Promise<RecreateAnalysisTask> {
  const res = await api.get(`${RECREATE_PATH}/${encodeURIComponent(taskId)}`, {
    signal,
    skipErrorHandler: true,
    disableDuplicate: true,
  } as Record<string, unknown>)
  const envelope = res.data as { success?: boolean } & Record<string, unknown>
  if (envelope?.success !== true || !isRecord(envelope.data)) {
    throw createServerError(
      envelope,
      'Could not analyze the reference video. Please try again.'
    )
  }
  const data = envelope.data
  const status = typeof data.status === 'string' ? data.status : ''
  if (!status) {
    throw createServerError(
      envelope,
      'Could not analyze the reference video. Please try again.'
    )
  }
  const task: RecreateAnalysisTask = {
    task_id:
      typeof data.task_id === 'string' && data.task_id ? data.task_id : taskId,
    status,
    progress: typeof data.progress === 'string' ? data.progress : undefined,
    message: typeof data.message === 'string' ? data.message : undefined,
  }
  if (status === 'success') {
    return { ...task, ...readRecreateSuccess(data, envelope) }
  }
  return task
}

/** The gateway checks the original bytes, including codecs unavailable in the browser. */
export async function inspectMotionVideo(
  videoUrl: string,
  mode: MotionMode,
  signal?: AbortSignal
): Promise<MotionVideoMetadata> {
  const res = await api.post(
    `${MOTION_PATH}/inspect`,
    {
      video_url: videoUrl,
      mode,
    },
    { signal }
  )
  const body = res.data as {
    success?: boolean
    data?: { duration: number; width: number; height: number }
  }
  requireServerSuccess(body as Record<string, unknown>)
  const metadata = body.data
  if (
    !metadata ||
    !Number.isFinite(metadata.duration) ||
    metadata.duration <= 0 ||
    !Number.isInteger(metadata.width) ||
    metadata.width <= 0 ||
    !Number.isInteger(metadata.height) ||
    metadata.height <= 0
  ) {
    throw new Error('Could not prepare video')
  }
  return {
    rawDurationSec: metadata.duration,
    durationSec: Math.round(metadata.duration),
    width: metadata.width,
    height: metadata.height,
    aspectRatio: motionAspectRatio(metadata.width, metadata.height),
  }
}

/**
 * Submits a character video. The gateway picks the price tier from the
 * resolution, frames the prompt for the mode and adds the clip's own sound as
 * reference audio, so the body carries only what the user chose.
 */
export async function submitMotion(body: MotionSubmitBody): Promise<string> {
  let res
  try {
    res = await api.post(MOTION_PATH, body, {
      skipErrorHandler: true,
    } as Record<string, unknown>)
  } catch (e) {
    // A rejected form answers 400 with its reason in the body; surface that
    // reason instead of the transport's generic status text.
    const data = (e as { response?: { data?: Record<string, unknown> } })
      ?.response?.data
    const err = data?.error as { message?: string } | undefined
    const message = err?.message ?? (data?.message as string | undefined)
    throw message ? new Error(message) : e
  }
  const data = res.data as {
    task_id?: string
    id?: string
    error?: { message?: string }
    message?: string
  }
  const taskId = data?.task_id ?? data?.id
  if (!taskId) {
    throw new Error(
      data?.error?.message ??
        data?.message ??
        'Could not submit the video task. Please try again.'
    )
  }
  return taskId
}

export interface TaskListParams {
  /** Which workbench surface to list; defaults to character sheets. */
  action?: string
  /** 1-based page number. The gateway parses it from the `p` query key
   * (`common.GetPageQuery`); a `page` key is ignored and would pin every
   * request to page 1. */
  page?: number
  page_size?: number
  task_id?: string
  /** The artifact projection runs a plugin hook per successful task
   * (`controller.resolveTaskArtifacts`), so a caller that only needs a status
   * passes false. */
  with_artifacts?: boolean
}

export interface TaskPage {
  items: TaskRecord[]
  total: number
}

export async function fetchInfluencerTasks(
  params: TaskListParams = {},
  signal?: AbortSignal
): Promise<TaskPage> {
  const res = await api.get(TASK_LIST_PATH, {
    signal,
    params: {
      p: params.page ?? 1,
      page_size: params.page_size ?? 30,
      action: params.action ?? INFLUENCER_ACTION,
      task_id: params.task_id,
      with_artifacts: params.with_artifacts ?? true,
    },
  })
  const body = res.data as {
    success: boolean
    data: { items?: TaskRecord[]; total?: number }
  }
  requireServerSuccess(body as unknown as Record<string, unknown>)
  return { items: body.data?.items ?? [], total: body.data?.total ?? 0 }
}

/** Single-record read used by task polling and by the `?recreate=` deep link. */
export async function fetchTaskRecord(
  taskId: string,
  opts: { with_artifacts?: boolean; action?: string; signal?: AbortSignal } = {}
): Promise<TaskRecord | null> {
  const { items } = await fetchInfluencerTasks(
    {
      action: opts.action,
      task_id: taskId,
      page_size: 1,
      with_artifacts: opts.with_artifacts ?? false,
    },
    opts.signal
  )
  return items.find((t) => t.task_id === taskId) ?? items[0] ?? null
}

/** Enrich a confirmed terminal record once without undoing its status on read failure. */
export async function loadTaskRecord(
  taskId: string,
  action?: string
): Promise<TaskRecord | null> {
  const record = await fetchTaskRecord(taskId, { action })
  if (!record || record.task_id !== taskId) return null
  if (!isTerminalStatus(record.status)) return record
  try {
    const settled = await fetchTaskRecord(taskId, {
      action,
      with_artifacts: true,
    })
    return settled?.task_id === taskId && settled.status === record.status
      ? settled
      : record
  } catch {
    return record
  }
}

/** Mirrors the gateway's default file retention, used only when a response omits `expires_at`. */
export const DEFAULT_UPLOAD_RETENTION_MS = 72 * 60 * 60 * 1000

export interface UploadedReference {
  url: string
  /** Epoch milliseconds after which the gateway may delete the stored file. */
  expiresAt: number
}

export type UploadProgress = (loaded: number, total: number) => void

function putWithProgress(
  url: string,
  file: File,
  headers: Headers,
  signal: AbortSignal | undefined,
  onProgress: UploadProgress
): Promise<void> {
  return new Promise((resolve, reject) => {
    if (typeof XMLHttpRequest === 'undefined') {
      reject(new Error('Upload progress is unavailable'))
      return
    }
    const request = new XMLHttpRequest()
    const abort = () => request.abort()
    const cleanup = () => signal?.removeEventListener('abort', abort)
    request.open('PUT', url, true)
    request.withCredentials = false
    headers.forEach((value, name) => request.setRequestHeader(name, value))
    request.upload.addEventListener('progress', (event) => {
      if (event.lengthComputable) onProgress(event.loaded, event.total)
    })
    request.addEventListener(
      'load',
      () => {
        cleanup()
        if (request.status >= 200 && request.status < 300) {
          resolve()
        } else {
          reject(
            new Error(
              i18next.t('File upload failed ({{status}}). Please try again.', {
                status: request.status,
              })
            )
          )
        }
      },
      { once: true }
    )
    request.addEventListener(
      'error',
      () => {
        cleanup()
        reject(new Error('File upload failed. Please try again.'))
      },
      { once: true }
    )
    request.addEventListener(
      'abort',
      () => {
        cleanup()
        reject(new DOMException('Cancelled', 'AbortError'))
      },
      { once: true }
    )
    signal?.addEventListener('abort', abort, { once: true })
    request.send(file)
  })
}

interface UploadTicket {
  file_id: string
  upload_url: string
  upload_method: string
  upload_headers: Record<string, string>
}

export async function uploadReferenceAsset(
  file: File,
  signal?: AbortSignal,
  options?: {
    purpose: 'influencer_reference_video'
    videoMetadata: import('./lib/browser-video-metadata').BrowserVideoMetadata
  },
  onProgress?: UploadProgress
): Promise<UploadedReference> {
  if (signal?.aborted) throw new DOMException('Cancelled', 'AbortError')
  const extension = file.name.split('.').pop()?.toLowerCase() ?? ''
  let videoMime = { mp4: 'video/mp4', mov: 'video/quicktime' }[
    extension as 'mp4' | 'mov'
  ]
  if (options) {
    videoMime =
      options.videoMetadata.container === 'mov'
        ? 'video/quicktime'
        : 'video/mp4'
  }
  let ticket: UploadTicket | undefined
  let response: { data: unknown } | undefined
  try {
    const issued = await api.post(
      '/api/v1/files/upload/presign',
      {
        file_name: file.name,
        mime_type:
          videoMime ||
          file.type ||
          (
            {
              mp4: 'video/mp4',
              mov: 'video/quicktime',
              png: 'image/png',
              jpg: 'image/jpeg',
              jpeg: 'image/jpeg',
              webp: 'image/webp',
            } as Record<string, string>
          )[file.name.split('.').pop()?.toLowerCase() ?? ''] ||
          'application/octet-stream',
        file_size: file.size,
        ...(options
          ? { purpose: options.purpose, video_metadata: options.videoMetadata }
          : {}),
      },
      { signal }
    )
    const envelope = issued.data as {
      success?: boolean
      msg?: string
      data?: UploadTicket
    }
    if (envelope.success === false) {
      throw createServerError(envelope, envelope.msg)
    }
    ticket = envelope.data
    if (
      !ticket?.file_id ||
      !ticket.upload_url ||
      ticket.upload_method !== 'PUT'
    ) {
      throw new Error(
        'The upload session is invalid. Please refresh and try again.'
      )
    }
  } catch (error: unknown) {
    const status = (error as { response?: { status?: number } })?.response
      ?.status
    if (status !== 501) throw error
    const form = new FormData()
    if (options && file.type !== videoMime) {
      form.append('file', new Blob([file], { type: videoMime }), file.name)
    } else {
      form.append('file', file)
    }
    if (options) {
      form.append('purpose', options.purpose)
      form.append('video_metadata', JSON.stringify(options.videoMetadata))
    }
    const multipartOptions = onProgress
      ? {
          signal,
          onUploadProgress: (event: { loaded: number; total?: number }) =>
            onProgress(event.loaded, event.total ?? file.size),
        }
      : { signal }
    response = await api.post(UPLOAD_PATH, form, multipartOptions)
  }
  if (ticket) {
    const destination = new URL(ticket.upload_url)
    if (
      destination.protocol !== 'https:' ||
      destination.username ||
      destination.password ||
      !destination.hostname.endsWith('.r2.cloudflarestorage.com')
    ) {
      throw new Error(
        'The upload session is invalid. Please refresh and try again.'
      )
    }
    const headers = new Headers()
    for (const [name, value] of Object.entries(ticket.upload_headers ?? {})) {
      if (['host', 'content-length'].includes(name.toLowerCase())) continue
      if (['authorization', 'cookie'].includes(name.toLowerCase())) {
        throw new Error(
          'The upload session is invalid. Please refresh and try again.'
        )
      }
      headers.set(name, value)
    }
    // XHR is the only browser API that reports PUT progress. Keep the simpler
    // fetch path for small files, where a percentage would flash and vanish.
    if (
      onProgress &&
      file.size >= 1024 * 1024 &&
      typeof XMLHttpRequest !== 'undefined'
    ) {
      await putWithProgress(
        ticket.upload_url,
        file,
        headers,
        signal,
        onProgress
      )
    } else {
      const uploaded = await fetch(ticket.upload_url, {
        method: 'PUT',
        body: file,
        headers,
        credentials: 'omit',
        redirect: 'error',
        signal,
      })
      if (!uploaded.ok) {
        throw new Error(
          i18next.t('File upload failed ({{status}}). Please try again.', {
            status: uploaded.status,
          })
        )
      }
    }
    response = await api.post(
      '/api/v1/files/upload/confirm',
      {
        file_id: ticket.file_id,
        file_size: file.size,
      },
      { signal }
    )
  }
  if (!response) {
    throw new Error('The upload returned no response. Please try again.')
  }
  const body = response.data as {
    success?: boolean
    data?: { file_url?: string; expires_at?: string }
    msg?: string
  }
  if (body.success === false) {
    throw createServerError(body, body.msg)
  }
  const url = body.data?.file_url
  if (!url) {
    throw new Error('The upload returned no file URL. Please try again.')
  }
  const expiresAt = Date.parse(body.data?.expires_at ?? '')
  return {
    url,
    expiresAt: Number.isFinite(expiresAt)
      ? expiresAt
      : Date.now() + DEFAULT_UPLOAD_RETENTION_MS,
  }
}

/** The characters published into the inspiration library by an administrator. */
export async function fetchPublishedPresets(): Promise<PublishedPresetRow[]> {
  const res = await api.get(PRESETS_PATH)
  const body = res.data as {
    success: boolean
    data?: { items?: PublishedPresetRow[] }
  }
  requireServerSuccess(body as unknown as Record<string, unknown>)
  return body.data?.items ?? []
}

export interface PublishPresetInput {
  /** The finished history record being published. */
  taskId: string
  name: string
  /** The character render, uploaded as bytes so the gateway can persist it. */
  preview: Blob
  /** The multi-view board, when the character has one. */
  sheet?: Blob
}

/**
 * Publishes one of the caller's finished characters. The tier, option
 * selection, brief and seed are read from the task server-side, so the request
 * carries only the artwork and the display name.
 */
export async function publishInfluencerPreset(
  input: PublishPresetInput
): Promise<PublishedPresetRow> {
  const form = new FormData()
  form.append('task_id', input.taskId)
  form.append('name', input.name)
  form.append('preview', input.preview)
  if (input.sheet) form.append('sheet', input.sheet)
  const res = await api.post(PRESETS_PATH, form, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  const body = res.data as {
    success?: boolean
    data?: PublishedPresetRow
    message?: string
    msg?: string
  }
  if (!body?.success || !body.data) {
    throw createServerError(body, 'Request failed')
  }
  return body.data
}

export async function deletePublishedPreset(id: string): Promise<void> {
  const res = await api.delete(`${PRESETS_PATH}/${id}`, {
    skipErrorHandler: true,
  } as Record<string, unknown>)
  requireServerSuccess(res.data as Record<string, unknown>)
}
