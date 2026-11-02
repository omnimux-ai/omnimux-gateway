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
import { validateMotionOutput } from './motion'

const storedMotionSchema = z.object({
  mode: z.enum(['motion', 'replace']),
  prompt: z.string(),
  subject_image_url: z.string(),
  subject_image_urls: z.array(z.string()).optional(),
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
  /**
   * Recreate mode only. The prompt is the editable server-compiled text and
   * `durationSec` is the output length it was compiled for. A recreate request
   * has no video: the clip was decomposition material only.
   */
  recreate?: {
    prompt: string
    durationSec: number
  }
}

/** The fixed framing a recreation always asks for. */
const RECREATE_ASPECT_RATIO: MotionAspectRatio = '9:16'

/** Recreate images keep their upload order and cap at the planner's limit. */
const RECREATE_MAX_IMAGES = 30

function buildRecreatePayload(input: BuildInput): MotionSubmitBody | null {
  const chars = (input.characters ?? []).slice(0, RECREATE_MAX_IMAGES)
  if (chars.length === 0 || !input.recreate) return null
  if (!input.video || input.video.kind === 'preset') return null
  if (!input.video.metadata) return null
  const prompt = input.recreate.prompt.trim()
  if (!prompt) return null
  if ([...prompt].length > 4000) {
    throw new Error('Prompt must be 4000 characters or fewer.')
  }
  const durationSec = input.recreate.durationSec
  // The reference clip decides the length: 4–30 whole seconds on the shared line.
  if (
    !Number.isInteger(durationSec) ||
    durationSec < 4 ||
    durationSec > 30
  ) {
    return null
  }
  const urls = chars.map((c) => c.url)
  return {
    mode: 'recreate',
    prompt,
    subject_image_url: urls[0],
    subject_image_urls: urls,
    subject_image_tags: [],
    duration_sec: durationSec,
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
