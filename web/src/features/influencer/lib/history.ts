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
import { fetchTaskRecord } from '../api'
import type {
  InfluencerTaskKind,
  InfluencerTaskMeta,
  MotionCharacterSource,
  TaskRecord,
} from '../types'
import { selectionSchema } from './builder-draft'
import { isAnimalTier, isCharacterTier } from './taxonomy'

/** Uniform gallery frame measured across desktop and mobile reference widths. */
export const HISTORY_GRID_RATIO = '1.75578'

export { isTerminalStatus } from './task-status'

/**
 * Lenient read of the payload the controller stashed into properties.input.
 * Unlike parseTaskMeta it does not require sheet params, so it also reads the
 * rows written by derived actions.
 */
function readTaskMeta(record: TaskRecord): Record<string, unknown> | null {
  const raw = record.properties?.input
  if (!raw) return null
  try {
    const parsed: unknown = JSON.parse(raw)
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>
    }
    return null
  } catch {
    return null
  }
}

function hasSheetParams(meta: Record<string, unknown>): boolean {
  return (
    'selection' in meta ||
    typeof meta.brief === 'string' ||
    typeof meta.image_url === 'string'
  )
}

/** Missing legacy tiers are human only when readable sheet parameters prove it. */
function validatedTaskTier(meta: Record<string, unknown>): string | undefined {
  const selection = selectionSchema.safeParse(
    'selection' in meta ? meta.selection : {}
  )
  if (!selection.success) return undefined
  if ('brief' in meta && typeof meta.brief !== 'string') return undefined
  if (
    'image_url' in meta &&
    meta.image_url !== null &&
    typeof meta.image_url !== 'string'
  ) {
    return undefined
  }
  const tier = 'tier' in meta ? meta.tier : 'normal'
  if (!isCharacterTier(tier)) return undefined
  if (!('tier' in meta) && !hasSheetParams(meta)) return undefined
  if (
    !isAnimalTier(tier) &&
    ['animal_breed', 'coat_color', 'coat_pattern'].some(
      (category) => (selection.data[category]?.length ?? 0) > 0
    )
  ) {
    return undefined
  }
  return tier
}

/** Extracts the sheet params the controller stashed into properties.input. */
export function parseTaskMeta(record: TaskRecord): InfluencerTaskMeta | null {
  const parsed = readTaskMeta(record)
  if (!parsed) return null
  const tier = validatedTaskTier(parsed)
  if (!tier) return null
  if (!isAnimalTier(tier) && !hasSheetParams(parsed)) return null
  const selection = selectionSchema.safeParse(
    'selection' in parsed ? parsed.selection : {}
  )
  if (!selection.success) return null
  return {
    tier,
    selection: selection.data,
    brief: typeof parsed.brief === 'string' ? parsed.brief : undefined,
    image_mode:
      parsed.image_mode === 'text' ||
      parsed.image_mode === 'reference' ||
      parsed.image_mode === 'edit' ||
      parsed.image_mode === 'masked_edit'
        ? parsed.image_mode
        : undefined,
    mask_url: typeof parsed.mask_url === 'string' ? parsed.mask_url : undefined,
    edit_instruction:
      typeof parsed.edit_instruction === 'string'
        ? parsed.edit_instruction
        : undefined,
    image_url:
      typeof parsed.image_url === 'string' ? parsed.image_url : undefined,
    seed: typeof parsed.seed === 'number' ? parsed.seed : undefined,
    size: typeof parsed.size === 'string' ? parsed.size : undefined,
    kind: taskKind(record),
    parent_task_id:
      typeof parsed.parent_task_id === 'string'
        ? parsed.parent_task_id
        : undefined,
  }
}

/** A row without a kind is a character sheet: derived rows are the exception. */
export function taskKind(record: TaskRecord): InfluencerTaskKind {
  if (record.action === 'influencer_motion') return 'motion'
  const kind = readTaskMeta(record)?.kind
  if (kind === 'multiview' || kind === 'motion') return kind
  return 'sheet'
}

/** The sheet a derived board belongs to, or null for a sheet itself. */
export function parentTaskId(record: TaskRecord): string | null {
  if (taskKind(record) !== 'multiview') return null
  const parent = readTaskMeta(record)?.parent_task_id
  return typeof parent === 'string' && parent ? parent : null
}

export function characterTypeForTier(
  tier: unknown
): MotionCharacterSource['characterType'] {
  if (tier === 'cats') return 'cat'
  if (tier === 'dogs') return 'dog'
  if (tier === 'normal' || tier === 'freak' || tier === 'total') return 'human'
  return undefined
}

/** Bind the selected artifact task, reading an old multiview's parent if needed. */
export function taskCharacterSource(
  record: TaskRecord,
  url: string,
  fileName: string,
  parents: TaskRecord[] = []
): MotionCharacterSource {
  const meta = readTaskMeta(record)
  let tier: string | undefined
  if (meta && 'tier' in meta) {
    tier = validatedTaskTier(meta)
  } else if (
    meta &&
    taskKind(record) === 'multiview' &&
    (!('selection' in meta) ||
      selectionSchema.safeParse(meta.selection).success)
  ) {
    const parent = parents.find((item) => item.task_id === parentTaskId(record))
    if (parent && taskKind(parent) === 'sheet') {
      tier = parseTaskMeta(parent)?.tier
    }
  } else if (taskKind(record) === 'sheet') {
    tier = parseTaskMeta(record)?.tier
  }
  const characterType = characterTypeForTier(tier)
  const source: MotionCharacterSource = { url, fileName }
  const previewUrl = taskImageUrl(record)
  if (previewUrl) source.previewUrl = previewUrl
  if (!characterType) return source
  return { ...source, characterType, sourceTaskId: record.task_id }
}

export async function resolveTaskCharacterSource(
  record: TaskRecord,
  url: string,
  fileName: string,
  parents: TaskRecord[] = []
): Promise<MotionCharacterSource> {
  const source = taskCharacterSource(record, url, fileName, parents)
  const parentId = parentTaskId(record)
  const meta = readTaskMeta(record)
  if (
    source.characterType ||
    !parentId ||
    !meta ||
    'tier' in meta ||
    ('selection' in meta && !selectionSchema.safeParse(meta.selection).success)
  ) {
    return source
  }
  try {
    const parent = await fetchTaskRecord(parentId)
    if (!parent || parent.task_id !== parentId) return source
    return taskCharacterSource(record, url, fileName, [parent])
  } catch {
    return source
  }
}

export function taskBrief(record: TaskRecord): string | undefined {
  const brief = readTaskMeta(record)?.brief
  return typeof brief === 'string' ? brief : undefined
}

/** The stored artifact URL, kept absolute: external providers must fetch it. */
function rawImageUrl(record: TaskRecord): string | null {
  const art = record.artifacts?.find(
    (a) => (a.content_url ?? a.url ?? '').length > 0
  )
  return art?.content_url ?? art?.url ?? null
}

/**
 * The absolute artifact URL, for callers that hand the image to someone else
 * (a download, or an upstream provider that must fetch it over the network).
 * A relative URL is refused rather than resolved: it would only work in this
 * browser session.
 */
export function taskAbsoluteImageUrl(record: TaskRecord): string | null {
  const url = rawImageUrl(record)
  if (!url) return null
  return /^https?:\/\//i.test(url) ? url : null
}

/**
 * The generation size the controller stashed into properties.input, as a CSS
 * `aspect-ratio` value. Reserving it keeps an image load from shifting the
 * gallery below it; a record without a readable size keeps natural sizing.
 * The workbench submits `9:16`/`16:9`, while the controller's own default and
 * older records carry pixel dimensions such as `1024x1536`.
 */
export function taskAspectRatio(record: TaskRecord): string | undefined {
  const raw = record.properties?.input
  if (!raw) return undefined
  try {
    const parsed = JSON.parse(raw) as { size?: unknown }
    if (typeof parsed?.size !== 'string') return undefined
    const match = /^(\d{1,4})\s*[:x*]\s*(\d{1,4})$/i.exec(parsed.size.trim())
    if (!match) return undefined
    const width = Number(match[1])
    const height = Number(match[2])
    if (!width || !height) return undefined
    return `${width} / ${height}`
  } catch {
    return undefined
  }
}

/** The gateway serves artifact content on any origin it is hit on; keep the
 * URL relative so a dev proxy or another deployment host still resolves. */
export function taskBrowserMediaUrl(url: string, download = false): string {
  try {
    const base =
      typeof window === 'undefined'
        ? 'https://task-content.invalid'
        : window.location.origin
    const parsed = new URL(url, base)
    const trustedGateway =
      parsed.origin === new URL(base).origin ||
      ['https://omnimux.ai', 'https://api.omnimux.ai'].includes(parsed.origin)
    if (!trustedGateway) return url
    if (
      !/^\/v1\/tasks\/[^/]+\/artifacts\/[^/]+\/content$/.test(parsed.pathname)
    ) {
      return url
    }
    // Derivatives are small, app-served and immutable. The direct R2 redirect
    // answers with no-store and a short-lived ticket, so it would disable the
    // one-year cache these cards rely on; originals and videos keep it.
    const thumbnail = /\/artifacts\/thumb-v1-[^/]+\/content$/.test(
      parsed.pathname
    )
    if (thumbnail && !download) return parsed.pathname + parsed.search
    parsed.searchParams.set('delivery', 'direct')
    if (download) parsed.searchParams.set('download', '1')
    return parsed.pathname + parsed.search
  } catch {
    return url
  }
}

function sameOrigin(url: string): string {
  return taskBrowserMediaUrl(url)
}

export function taskImageUrl(record: TaskRecord): string | null {
  const art = record.artifacts?.find(
    (a) => (a.content_url ?? a.url ?? '').length > 0
  )
  const url = art?.content_url ?? art?.url
  if (url) return sameOrigin(url)
  if (record.result_url) return sameOrigin(record.result_url)
  if (record.legacy_content_url) return sameOrigin(record.legacy_content_url)
  return null
}

/** Card-only derivative; original selectors remain authoritative for reuse. */
export function taskCardImageUrl(record: TaskRecord): string | null {
  const image = record.artifacts?.find(
    (artifact) => (artifact.content_url ?? artifact.url ?? '').length > 0
  )
  const thumbnail = image?.thumbnail_url ?? record.thumbnail_url
  return thumbnail ? sameOrigin(thumbnail) : taskImageUrl(record)
}

/**
 * The playable address of a finished video. The gateway serves video content
 * through its own proxy, so the URL is kept relative like image artifacts.
 */
export function taskVideoUrl(record: TaskRecord): string | null {
  const art = record.artifacts?.find(
    (a) => a.key === 'video' && (a.content_url ?? a.url ?? '').length > 0
  )
  const url = art?.content_url ?? art?.url
  if (url) return sameOrigin(url)
  if (record.legacy_content_url) return sameOrigin(record.legacy_content_url)
  if (record.result_url) return sameOrigin(record.result_url)
  return null
}

/**
 * Resolves the submission timestamp of a task in epoch milliseconds.
 * Prefers `submit_time` (gateway unix timestamp in seconds or ms), falling back
 * to `created_at` (ISO date or string timestamp), or 0 if unresolvable.
 */
export function getTaskTimestamp(record: TaskRecord): number {
  if (typeof record.submit_time === 'number' && record.submit_time > 0) {
    return record.submit_time > 1e11
      ? record.submit_time
      : record.submit_time * 1000
  }
  if (record.created_at) {
    const parsed = Date.parse(record.created_at)
    if (!Number.isNaN(parsed)) return parsed
  }
  return 0
}

/**
 * Sorts tasks in reverse chronological order (newest first).
 * Breaks timestamp ties using descending primary task id.
 */
export function sortTasksByTime(tasks: TaskRecord[]): TaskRecord[] {
  return [...tasks].sort((a, b) => {
    const timeA = getTaskTimestamp(a)
    const timeB = getTaskTimestamp(b)
    if (timeA !== timeB) return timeB - timeA
    return (b.id ?? 0) - (a.id ?? 0)
  })
}
