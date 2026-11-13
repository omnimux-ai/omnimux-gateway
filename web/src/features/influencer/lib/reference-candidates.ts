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

import type { MotionVideoMetadata, TaskRecord } from '../types'
import { taskImageUrl, taskKind, taskVideoUrl } from './history'
import { motionAspectRatio } from './motion'

export interface ReferenceVideoCandidate {
  taskId: string
  artifactKey: string
  previewUrl: string
  referenceUrl: string | null
  title: string
  posterUrl?: string
  /** Persisted server facts, so selecting this clip needs no media read. */
  metadata?: MotionVideoMetadata
}

/** Validates later through `reusableMotionMetadata`; an incomplete record is
 * ignored and the existing browser read remains the fallback. */
function persistedVideoMetadata(
  artifact: ReferenceArtifact | undefined
): MotionVideoMetadata | undefined {
  const raw = artifact?.video_metadata
  if (!raw) return undefined
  if (
    !Number.isFinite(raw.duration) ||
    raw.duration <= 0 ||
    !Number.isFinite(raw.video_duration) ||
    raw.video_duration <= 0 ||
    !Number.isFinite(raw.frame_rate) ||
    raw.frame_rate <= 0 ||
    !Number.isInteger(raw.width) ||
    !Number.isInteger(raw.height) ||
    raw.width <= 0 ||
    raw.height <= 0
  ) {
    return undefined
  }
  return {
    version: 1,
    browserMetadata: raw,
    rawDurationSec: raw.duration,
    durationSec: Math.round(raw.video_duration),
    width: raw.width,
    height: raw.height,
    aspectRatio: motionAspectRatio(raw.width, raw.height),
  }
}

function successful(task: TaskRecord): boolean {
  return task.status === 'SUCCESS' || task.status === 'succeeded'
}

function sameOrigin(url: string): string {
  try {
    const parsed = new URL(url)
    return parsed.pathname + parsed.search
  } catch {
    return url
  }
}

export function taskTitle(task: TaskRecord): string {
  try {
    const input: unknown = JSON.parse(task.properties?.input ?? '')
    if (!input || typeof input !== 'object' || Array.isArray(input)) return ''
    const meta = input as Record<string, unknown>
    for (const value of [meta.prompt, meta.brief]) {
      if (typeof value === 'string' && value.trim()) return value.trim()
    }
  } catch {
    return ''
  }
  return ''
}

type ReferenceArtifact = NonNullable<TaskRecord['artifacts']>[number] & {
  type?: string
  mime_type?: string
}

function artifactUrl(artifact: ReferenceArtifact): string | undefined {
  return artifact.content_url || artifact.url || undefined
}

function mediaPath(url: string): string {
  return sameOrigin(url).split(/[?#]/, 1)[0]
}

function videoPath(url: string): boolean {
  return (
    /\.(mp4|mov|webm|m4v|avi|mkv)$/i.test(mediaPath(url)) ||
    /\/artifacts\/video(?:\/content)?\/?$/i.test(mediaPath(url))
  )
}

function imageArtifact(artifact: ReferenceArtifact): boolean {
  return (
    /^image(?:$|[_-])/i.test(artifact.key ?? '') ||
    artifact.type?.toLowerCase() === 'image' ||
    /^image\//i.test(artifact.mime_type ?? '') ||
    /\.(png|jpe?g|gif|webp|avif|svg|bmp|heic)$/i.test(
      mediaPath(artifactUrl(artifact) ?? '')
    )
  )
}

function videoArtifact(artifact: ReferenceArtifact): boolean {
  const url = artifactUrl(artifact)
  if (!url || imageArtifact(artifact)) return false
  return (
    artifact.key === 'video' ||
    artifact.type?.toLowerCase() === 'video' ||
    /^video\//i.test(artifact.mime_type ?? '') ||
    videoPath(url)
  )
}

function publicReferenceUrl(url: string): string | null {
  if (!/^https:\/\//i.test(url) || /[\s\\#]/.test(url)) return null
  try {
    const parsed = new URL(url)
    if (parsed.protocol !== 'https:' || parsed.username || parsed.password) {
      return null
    }
    const authority = url.slice(url.indexOf('//') + 2).split(/[/?#]/, 1)[0]
    if (authority.includes('@')) return null
    const host = parsed.hostname.toLowerCase().replace(/\.$/, '')
    // Require a DNS hostname, never an IP literal or a single-label LAN name.
    if (!host.includes('.') || /^[\d.]+$/.test(host) || host.includes(':')) {
      return null
    }
    if (
      !host
        .split('.')
        .every((label) => /^[a-z\d](?:[a-z\d-]*[a-z\d])?$/i.test(label))
    ) {
      return null
    }
    if (
      /(?:^|\.)(localhost|local|localdomain|internal|intranet|lan|home|corp|onion)$/.test(
        host
      ) ||
      /(?:^|\.)home\.arpa$/.test(host)
    ) {
      return null
    }
    return url
  } catch {
    return null
  }
}

function playableProjection(url: string): string | null {
  if (/^\/(?!\/)/.test(url) && !url.includes('\\')) return url.split('#', 1)[0]
  try {
    const parsed = new URL(url)
    if (parsed.protocol !== 'https:' && parsed.protocol !== 'http:') return null
    const projection = parsed.pathname + parsed.search
    return /^\/(?!\/)/.test(projection) ? projection : null
  } catch {
    return null
  }
}

export function referenceVideoCandidate(
  task: TaskRecord
): ReferenceVideoCandidate | null {
  if (
    !task.task_id?.trim() ||
    !successful(task) ||
    taskKind(task) !== 'motion'
  ) {
    return null
  }
  const artifact = task.artifacts?.find(videoArtifact)
  let url = artifact && artifactUrl(artifact)
  if (!url && !task.artifacts?.length) {
    url = [task.legacy_content_url, task.result_url].find(
      (value) => value && videoPath(value)
    )
  }
  if (!url || !playableProjection(url)) return null
  const artifactKey = artifact?.key || 'video'
  // Restrict the history helper to the chosen artifact: its legacy fallback is
  // intentionally broader than this reference picker permits.
  const previewUrl =
    artifact?.key === 'video'
      ? taskVideoUrl({
          ...task,
          artifacts: [{ key: 'video', content_url: url }],
        })
      : playableProjection(url)
  if (!previewUrl) return null
  const posterUrl = task.poster_url && playableProjection(task.poster_url)
  const metadata = persistedVideoMetadata(artifact)
  return {
    taskId: task.task_id,
    artifactKey,
    previewUrl,
    referenceUrl: publicReferenceUrl(url),
    title: taskTitle(task),
    ...(posterUrl ? { posterUrl } : {}),
    ...(metadata ? { metadata } : {}),
  }
}

export function referenceVideoCandidates(
  items: readonly TaskRecord[],
  hiddenIds?: ReadonlySet<string>
): ReferenceVideoCandidate[] {
  const candidates: ReferenceVideoCandidate[] = []
  for (const task of items) {
    if (hiddenIds?.has(task.task_id)) continue
    const candidate = referenceVideoCandidate(task)
    if (candidate) candidates.push(candidate)
  }
  return candidates
}

export function referenceImageTasks(
  items: readonly TaskRecord[]
): TaskRecord[] {
  return items.filter((task) => {
    if (
      !task.task_id?.trim() ||
      !successful(task) ||
      taskKind(task) === 'motion'
    ) {
      return false
    }
    if (
      task.artifacts?.some(
        (artifact: ReferenceArtifact) =>
          artifact.key === 'video' ||
          artifact.type?.toLowerCase() === 'video' ||
          /^video\//i.test(artifact.mime_type ?? '') ||
          videoArtifact(artifact)
      )
    ) {
      return false
    }
    const imageUrl = taskImageUrl(task)
    return Boolean(imageUrl && !videoPath(imageUrl))
  })
}
