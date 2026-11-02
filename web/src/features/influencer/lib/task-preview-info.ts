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
import type { TaskRecord } from '../types'

export interface TaskPreviewInfoData {
  resolution?: string
  size?: string
  durationSec?: number
  images: string[]
  video?: string
}

function referenceUrl(value: unknown): string | undefined {
  if (typeof value !== 'string') return undefined
  try {
    const url = new URL(value)
    if (url.protocol === 'https:' || url.protocol === 'http:') return value
  } catch {
    return undefined
  }
  return undefined
}

/** Only display known stored inputs; output dimensions and supplier details are not inferred. */
export function taskPreviewInfo(task: TaskRecord): TaskPreviewInfoData {
  const empty: TaskPreviewInfoData = { images: [] }
  let input: unknown
  try {
    input = JSON.parse(task.properties?.input ?? 'null')
  } catch {
    return empty
  }
  if (!input || typeof input !== 'object' || Array.isArray(input)) return empty
  const raw = input as Record<string, unknown>
  const images: string[] = []
  const candidates = [raw.image_url, raw.subject_image_url]
  if (Array.isArray(raw.subject_image_urls)) {
    candidates.push(...raw.subject_image_urls)
  }
  for (const candidate of candidates) {
    const url = referenceUrl(candidate)
    if (url && !images.includes(url)) images.push(url)
  }
  const storedSize = raw.size ?? raw.aspect_ratio
  return {
    resolution:
      typeof raw.resolution === 'string' && raw.resolution.trim()
        ? raw.resolution
        : undefined,
    size:
      typeof storedSize === 'string' && storedSize.trim()
        ? storedSize
        : undefined,
    durationSec:
      typeof raw.duration_sec === 'number' &&
      Number.isFinite(raw.duration_sec) &&
      raw.duration_sec > 0
        ? raw.duration_sec
        : undefined,
    images,
    video: referenceUrl(raw.video_url),
  }
}
