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
import { z } from 'zod'

import type { MotionSubmitBody } from '../api'
import type {
  MotionAspectRatio,
  MotionCharacterSource,
  MotionMode,
  MotionVideoSource,
} from '../types'
import {
  MOTION_OUTPUT_ASPECTS,
  RECREATE_VIDEO_MIN_SECONDS,
  motionDurationsAgree,
  validateBrowserMotionMetadata,
  validateMotionOutput,
  validateRecreateReference,
} from './motion'
import { publicReferenceUrl } from './reference-candidates'

const storedMotionSchema = z.object({
  mode: z.enum(['motion', 'replace', 'recreate']),
  prompt: z.string(),
  subject_image_url: z.string(),
  subject_image_urls: z.array(z.string()).optional(),
  subject_image_tags: z.array(z.string()).optional(),
  subject_character_type: z.enum(['human', 'cat', 'dog']).optional(),
  source_task_id: z.string().min(1).optional(),
  video_url: z.string(),
  duration_sec: z.number(),
  size: z.enum(['16:9', '9:16', '3:4', 'adaptive']),
  resolution: z.enum(['480p', '720p']),
  video_metadata: z
    .object({
      duration: z.number().finite().positive(),
      video_duration: z.number().finite().positive(),
      width: z.number().int().positive(),
      height: z.number().int().positive(),
      frame_rate: z.number().finite().positive(),
      video_codec: z.string(),
      audio_codec: z.string(),
      container: z.string(),
      size_bytes: z.number().int().positive(),
    })
    .optional(),
})

/** Missing legacy identity remains unknown; malformed identity cannot be replayed. */
export function parseStoredMotionPayload(
  raw: string | undefined
): MotionSubmitBody | null {
  try {
    const meta: unknown = JSON.parse(raw ?? 'null')
    const stored =
      meta && typeof meta === 'object' && !Array.isArray(meta)
        ? (meta as Record<string, unknown>)
        : null
    const parsed = storedMotionSchema.safeParse(
      stored ? { ...stored, size: stored.size ?? stored.aspect_ratio } : meta
    )
    if (!parsed.success) return null
    const { size, ...body } = parsed.data
    return { ...body, aspect_ratio: size }
  } catch {
    return null
  }
}

interface BuildInput {
  mode: MotionMode
  prompt: string
  character: MotionCharacterSource | null
  characters?: MotionCharacterSource[]
  video: MotionVideoSource | null
  resolution: string
  outputCapabilities?: Record<string, MotionAspectRatio[]>
}

/** The fixed framing a recreation always asks for. */
const RECREATE_ASPECT_RATIO: MotionAspectRatio = '9:16'

/** The Web motion and recreation handlers accept at most 30 subject images. */
const MOTION_MAX_IMAGES = 30

function isMotionReferenceUrl(value: string): boolean {
  const url = value.trim()
  return (
    url.startsWith('https://') &&
    new TextEncoder().encode(url).length <= 2048 &&
    Boolean(publicReferenceUrl(url))
  )
}

/** A history row can refill the form even when it cannot be submitted as-is. */
export function parseReplayableMotionPayload(
  raw: string | undefined,
  outputAspects: Record<string, MotionAspectRatio[]> = MOTION_OUTPUT_ASPECTS
): MotionSubmitBody | null {
  const body = parseStoredMotionPayload(raw)
  const metadata = body?.video_metadata
  if (!body || !metadata || !isMotionReferenceUrl(body.video_url ?? '')) {
    return null
  }

  const minimum = body.mode === 'recreate' ? RECREATE_VIDEO_MIN_SECONDS : 5
  const checked = validateBrowserMotionMetadata(metadata, body.mode)
  if (
    !checked.ok ||
    metadata.duration < minimum ||
    metadata.video_duration < minimum ||
    !motionDurationsAgree(metadata) ||
    body.duration_sec !== Math.round(metadata.video_duration)
  ) {
    return null
  }

  if (body.mode === 'recreate') {
    if (body.aspect_ratio !== RECREATE_ASPECT_RATIO) return null
  } else if (
    !checked.aspectRatio ||
    body.aspect_ratio !== checked.aspectRatio ||
    !outputAspects[body.resolution]?.includes(checked.aspectRatio)
  ) {
    return null
  }

  const images = body.subject_image_urls?.length
    ? body.subject_image_urls.filter((url) => url.trim())
    : [body.subject_image_url]
  const tags = body.subject_image_tags ?? []
  const maxPromptLength = body.mode === 'recreate' ? 1200 : 4000
  if (
    !images.length ||
    images.length > MOTION_MAX_IMAGES ||
    !images.every(isMotionReferenceUrl) ||
    tags.length > images.length ||
    tags.some((tag) => [...tag.trim()].length > 20) ||
    [...body.prompt.trim()].length > maxPromptLength ||
    new TextEncoder().encode(body.source_task_id?.trim() ?? '').length > 255
  ) {
    return null
  }
  return body
}

function buildRecreatePayload(input: BuildInput): MotionSubmitBody | null {
  const chars = (input.characters ?? []).slice(0, MOTION_MAX_IMAGES)
  if (chars.length === 0) return null
  if (!input.video || input.video.kind === 'preset') return null
  const checked = input.video.metadata
    ? validateRecreateReference(input.video.metadata)
    : null
  if (!checked?.ok || !checked.browserMetadata) return null
  const prompt = input.prompt.trim()
  if ([...prompt].length > 4000) {
    throw new Error('Prompt must be 4000 characters or fewer.')
  }
  const urls = chars.map((c) => c.url)
  return {
    mode: 'recreate',
    prompt,
    subject_image_url: urls[0],
    subject_image_urls: urls,
    subject_image_tags: [],
    video_url: input.video.url,
    video_metadata: checked.browserMetadata,
    duration_sec: Math.round(checked.browserMetadata.video_duration),
    aspect_ratio: RECREATE_ASPECT_RATIO,
    resolution: input.resolution,
  }
}

/**
 * The gateway body for a character video. The gateway frames the prompt for
 * the mode, picks the price tier from the resolution and adds the clip's own
 * sound. The original browser metadata and its derived output settings travel together.
 *
 * Returns null while a slot is empty, and for a built-in template: templates
 * carry no public clip yet, and the upstream model only fetches public HTTPS
 * addresses.
 */
export function buildMotionSubmitPayload(
  input: BuildInput
): MotionSubmitBody | null {
  if (input.mode === 'recreate') return buildRecreatePayload(input)
  let chars: MotionCharacterSource[] = []
  if (input.characters && input.characters.length > 0) {
    chars = input.characters
  } else if (input.character) {
    chars = [input.character]
  }
  if (chars.length === 0 || !input.video) return null
  if (input.video.kind !== 'upload' && input.video.kind !== 'remote') {
    return null
  }
  const metadata = input.video.metadata
  if (!metadata) return null
  const check = validateMotionOutput(
    metadata,
    input.mode,
    input.resolution,
    input.outputCapabilities
  )
  if (!check.ok || !check.browserMetadata || !check.aspectRatio) return null
  const prompt = input.prompt.trim()
  if (input.mode === 'motion' && [...prompt].length > 4000) {
    throw new Error('Prompt must be 4000 characters or fewer.')
  }
  const urls = chars.map((c) => c.url)
  return {
    mode: input.mode,
    prompt,
    subject_image_url: urls[0],
    subject_image_urls: urls,
    subject_character_type: chars[0].characterType,
    source_task_id: chars[0].sourceTaskId,
    video_url: input.video.url,
    duration_sec: check.durationSec,
    aspect_ratio: check.aspectRatio,
    video_metadata: check.browserMetadata,
    resolution: input.resolution,
  }
}
