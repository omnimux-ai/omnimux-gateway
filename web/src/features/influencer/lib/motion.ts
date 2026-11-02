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

import type {
  MotionAspectRatio,
  MotionMode,
  MotionPreset,
  MotionVideoMetadata,
} from '../types'
import type { BrowserVideoMetadata } from './browser-video-metadata'

/**
 * How a mode treats the driving video. `driver` reads the movement and
 * discards the pixels; `scene` keeps the pixels and repaints the person in
 * them; `reference-only` reads the clip once for its narrative and pacing and
 * never sends it to the generation model.
 */
export type MotionVideoRole = 'driver' | 'scene' | 'reference-only'

export interface MotionModeMeta {
  id: MotionMode
  videoRole: MotionVideoRole
  /** Whether the mode needs a reference image before submitting. */
  requiresCharacter: boolean
  /** Whether the mode needs a clip in the video slot before submitting. */
  requiresVideo: boolean
}

const MODE_META: Record<MotionMode, MotionModeMeta> = {
  motion: {
    id: 'motion',
    videoRole: 'driver',
    requiresCharacter: true,
    requiresVideo: true,
  },
  replace: {
    id: 'replace',
    videoRole: 'scene',
    requiresCharacter: true,
    requiresVideo: true,
  },
  recreate: {
    id: 'recreate',
    // The clip is decomposition material only: it shapes the server-compiled
    // prompt and is never part of the generation request.
    videoRole: 'reference-only',
    requiresCharacter: false,
    requiresVideo: true,
  },
}

/** Declaration order doubles as the order the sub-mode switch renders. */
export const MOTION_MODES: MotionMode[] = ['motion', 'replace', 'recreate']

export const DEFAULT_MOTION_MODE: MotionMode = 'motion'

export function isMotionMode(value: unknown): value is MotionMode {
  return value === 'motion' || value === 'replace' || value === 'recreate'
}

export function motionModeMeta(mode: MotionMode): MotionModeMeta {
  return MODE_META[mode]
}

/**
 * The instruction each mode sends the model. `motion` and `replace` name the
 * two attachments the way the upstream pipeline expects: `@Image 1` is the
 * character and `@Video` is the driving clip. `recreate` has no instruction
 * here at all: its prompt is compiled by the server from the decomposition,
 * and it must never mention a video because none is sent.
 */
const MODE_INSTRUCTION: Record<MotionMode, string> = {
  motion:
    'Full body motion transfer. Precisely replicate the exact body movement, gesture dynamics, and pacing from the reference video @Video. The character must strictly maintain the identical facial features, hairstyle, outfit, skin tone, and body proportions shown in @Image 1.',
  replace:
    'Seamlessly replace the primary person in the video with the character from @Image 1. Keep the exact background scenery, original camera trajectory, ambient lighting, shadows, and interacted objects completely intact.',
  recreate: '',
}

/**
 * The single point where the modes diverge on the wire, so the second phase
 * has one place to wire up rather than a prompt built inline in a component.
 * `recreate` returns the caller's text unchanged: the server already compiled
 * the full prompt and no local mode framing may add a video reference.
 */
export function composeMotionPrompt(
  mode: MotionMode,
  userPrompt: string
): string {
  const instruction = MODE_INSTRUCTION[mode]
  const extra = userPrompt.trim()
  if (mode === 'recreate') return extra
  return extra ? `${instruction} ${extra}` : instruction
}

/** Shared video-upload ceiling; image uploads retain their separate limit. */
export const MOTION_VIDEO_MAX_BYTES = 200 * 1024 * 1024

/** Reference inputs and edited originals have different minimum durations. */
export const MOTION_VIDEO_MIN_SECONDS = 2
export const MOTION_EDIT_MIN_SECONDS = 4
export const MOTION_VIDEO_MAX_SECONDS = 30

/**
 * The recreation line outputs at most 15 seconds, so a longer reference clip
 * cannot be followed and is refused before any upload or analysis.
 */
export const RECREATE_VIDEO_MIN_SECONDS = 4
export const RECREATE_VIDEO_MAX_SECONDS = MOTION_VIDEO_MAX_SECONDS

/**
 * The minimum a mode accepts. `motion` only needs a readable pose; `replace`
 * and `recreate` need a usable scene window / decomposition beats, so they
 * share the four-second floor.
 */
export function motionMinVideoSeconds(mode: MotionMode): number {
  return mode === 'motion' ? MOTION_VIDEO_MIN_SECONDS : MOTION_EDIT_MIN_SECONDS
}

/** The maximum a mode accepts; recreation now shares the 30-second motion cap. */
export function motionMaxVideoSeconds(mode: MotionMode): number {
  return mode === 'recreate'
    ? RECREATE_VIDEO_MAX_SECONDS
    : MOTION_VIDEO_MAX_SECONDS
}

export const MOTION_VIDEO_MIME_TYPES = ['video/mp4', 'video/quicktime'] as const

const MOTION_VIDEO_EXTENSIONS = ['.mp4', '.mov']

/** The minimum a file picker hands us; the DOM `File` satisfies it. */
export interface MotionVideoCandidate {
  name: string
  type: string
  size: number
}

export type VideoRejectReason =
  | 'format'
  | 'size'
  | 'duration'
  | 'duration-short'
  | 'duration-long'
  | 'corrupt'
  | 'dimensions'
  | 'pixels'
  | 'aspect'
  | 'frame-rate'
  | 'codec'
  | 'audio'
  | 'output-duration'
  | 'output-aspect'

export interface VideoValidationSuccess extends MotionVideoMetadata {
  ok: true
}

export interface VideoValidationFailure {
  ok: false
  reason: VideoRejectReason
}

export type VideoValidationResult =
  | VideoValidationSuccess
  | VideoValidationFailure

/**
 * Checks format and size without reading media elements.
 */
export function checkVideoFileBasic(
  file: MotionVideoCandidate
): { ok: true } | { ok: false; reason: 'format' | 'size' } {
  if (file.size <= 0 || file.size > MOTION_VIDEO_MAX_BYTES) {
    return { ok: false, reason: 'size' }
  }
  const type = file.type.toLowerCase()
  if (MOTION_VIDEO_MIME_TYPES.some((allowed) => allowed === type)) {
    return { ok: true }
  }
  const name = file.name.toLowerCase()
  if (MOTION_VIDEO_EXTENSIONS.some((ext) => name.endsWith(ext))) {
    return { ok: true }
  }
  return { ok: false, reason: 'format' }
}

/**
 * Client-side guard only: the upload endpoint routes on MIME and enforces the
 * real ceiling. Some browsers report an empty type for `.mov`, so the extension
 * is a fallback rather than a second requirement.
 */
export function isAcceptableVideoFile(file: MotionVideoCandidate): boolean {
  return checkVideoFileBasic(file).ok
}

/** Standard display ratios recognized on a reference clip's own dimensions. */
export const MOTION_ASPECT_RATIOS = ['16:9', '9:16', '3:4'] as const

/**
 * Output framing the bound supplier lines are evidenced to produce per
 * resolution — the 720p J1 line only supports 16:9/9:16, so 3:4 is excluded
 * even though the generic vendor contract lists it. Mirrors the gateway's
 * `influencerMotionOutputRatios`; the form fetches the live value from
 * `GET /pg/influencer/motion/options` and this map is the typed fallback.
 */
export const MOTION_OUTPUT_ASPECTS: Record<string, MotionAspectRatio[]> = {
  '480p': ['16:9', '9:16', '3:4'],
  '720p': ['16:9', '9:16'],
}

export function motionOutputAspects(resolution: string): MotionAspectRatio[] {
  return MOTION_OUTPUT_ASPECTS[resolution] ?? MOTION_OUTPUT_ASPECTS['480p']
}

/**
 * Reports the clip's own display ratio when it matches a standard value.
 * This describes the input video only — a null result is no longer a reason
 * to reject the clip. Edited originals preserve their actual dimensions.
 */
export function motionAspectRatio(
  width: number,
  height: number
): MotionAspectRatio | null {
  if (
    !Number.isInteger(width) ||
    !Number.isInteger(height) ||
    width <= 0 ||
    height <= 0
  ) {
    return null
  }
  return (
    MOTION_ASPECT_RATIOS.find((ratio) => {
      const [w, h] = ratio.split(':').map(Number)
      // At most half a pixel of rounding on each dimension; no nearest-ratio crop.
      return Math.abs(width * h - height * w) <= (w + h) / 2
    }) ?? null
  )
}

interface VideoMetadata {
  duration: number
  width: number
  height: number
}

/** Reads file or remote metadata and releases the media resource on every exit. */
export function inspectVideoSource(
  source: File | string,
  signal?: AbortSignal
): Promise<VideoMetadata> {
  return new Promise((resolve, reject) => {
    if (typeof document === 'undefined' || signal?.aborted) {
      reject(new Error('Video metadata unavailable'))
      return
    }
    let objectUrl: string | undefined
    let video: HTMLVideoElement
    try {
      if (typeof source !== 'string') objectUrl = URL.createObjectURL(source)
      video = document.createElement('video')
    } catch {
      if (objectUrl) URL.revokeObjectURL(objectUrl)
      reject(new Error('Video metadata unavailable'))
      return
    }
    video.preload = 'metadata'
    let settled = false
    const cleanup = () => {
      clearTimeout(timer)
      video.removeEventListener('loadedmetadata', loaded)
      video.removeEventListener('error', error)
      signal?.removeEventListener('abort', abort)
      video.removeAttribute('src')
      video.load()
      if (objectUrl) URL.revokeObjectURL(objectUrl)
    }
    const fail = (message: string) => {
      if (settled) return
      settled = true
      cleanup()
      reject(new Error(message))
    }
    const abort = () => fail('Video metadata cancelled')
    // Public presets may need a cold CDN/range read; keep local file checks fast.
    const timeoutMs = typeof source === 'string' ? 30_000 : 5000
    const timer = setTimeout(() => fail('Video metadata timeout'), timeoutMs)
    signal?.addEventListener('abort', abort, { once: true })
    const loaded = () => {
      if (settled) return
      const meta = {
        duration: video.duration,
        width: video.videoWidth,
        height: video.videoHeight,
      }
      if (
        !Number.isFinite(meta.duration) ||
        meta.duration <= 0 ||
        !meta.width ||
        !meta.height
      ) {
        fail('Invalid video metadata')
        return
      }
      settled = true
      cleanup()
      resolve(meta)
    }
    const error = () => fail('Failed to load video')
    video.addEventListener('loadedmetadata', loaded)
    video.addEventListener('error', error)
    try {
      video.src = objectUrl ?? (source as string)
    } catch {
      fail('Failed to load video')
    }
  })
}

export function inspectVideoFile(file: File): Promise<VideoMetadata> {
  return inspectVideoSource(file)
}

/**
 * A recreation clip reports whether it is too short or too long, because the
 * fix differs; the other modes keep the single duration reason.
 */
function motionDurationFailure(
  duration: number,
  mode: MotionMode
): VideoRejectReason | null {
  const minimum = motionMinVideoSeconds(mode)
  const maximum = motionMaxVideoSeconds(mode)
  if (mode === 'recreate') {
    if (duration < minimum) return 'duration-short'
    if (duration > maximum) return 'duration-long'
    return null
  }
  if (duration < minimum || duration > maximum) return 'duration'
  return null
}

export function validateMotionMetadata(
  meta: VideoMetadata,
  mode: MotionMode = 'motion'
): VideoValidationResult {
  if (!Number.isFinite(meta.duration) || meta.duration <= 0) {
    return { ok: false, reason: 'corrupt' }
  }
  const durationFailure = motionDurationFailure(meta.duration, mode)
  if (durationFailure) return { ok: false, reason: durationFailure }
  // Input acceptance stays separate from the output service's finite framing
  // choices. The browser checks those choices before accepting a selection.
  return {
    ok: true,
    rawDurationSec: meta.duration,
    durationSec: Math.round(meta.duration),
    width: meta.width,
    height: meta.height,
    aspectRatio: motionAspectRatio(meta.width, meta.height),
  }
}

/** Check the full local result before any upload or selected-source replacement. */
export function validateBrowserMotionMetadata(
  raw: BrowserVideoMetadata,
  mode: MotionMode = 'motion'
): VideoValidationResult {
  const basic = validateMotionMetadata(raw, mode)
  if (!basic.ok) return basic
  if (!Number.isFinite(raw.video_duration) || raw.video_duration <= 0) {
    return { ok: false, reason: 'corrupt' }
  }
  const durationFailure = motionDurationFailure(raw.video_duration, mode)
  if (durationFailure) return { ok: false, reason: durationFailure }
  if (
    !Number.isInteger(raw.size_bytes) ||
    raw.size_bytes <= 0 ||
    raw.size_bytes > MOTION_VIDEO_MAX_BYTES
  ) {
    return { ok: false, reason: 'size' }
  }
  if (!['mp4', 'mov'].includes(raw.container)) {
    return { ok: false, reason: 'format' }
  }
  if (!['h264', 'hevc'].includes(raw.video_codec)) {
    return { ok: false, reason: 'codec' }
  }
  if (
    !Number.isInteger(raw.width) ||
    !Number.isInteger(raw.height) ||
    raw.width < 300 ||
    raw.height < 300 ||
    raw.width > 6000 ||
    raw.height > 6000
  ) {
    return { ok: false, reason: 'dimensions' }
  }
  const pixels = raw.width * raw.height
  if (pixels < 407696 || pixels > 8295044) {
    return { ok: false, reason: 'pixels' }
  }
  const ratio = raw.width / raw.height
  if (ratio < 0.4 || ratio > 2.5) return { ok: false, reason: 'aspect' }
  if (
    !Number.isFinite(raw.frame_rate) ||
    raw.frame_rate < 24 ||
    raw.frame_rate > 60
  ) {
    return { ok: false, reason: 'frame-rate' }
  }
  if (
    !['', 'aac', 'mp3'].includes(raw.audio_codec) &&
    !(
      raw.container === 'mov' &&
      (raw.audio_codec === 'pcm' || raw.audio_codec.startsWith('pcm_'))
    )
  ) {
    return { ok: false, reason: 'audio' }
  }
  return { ...basic, version: 1, browserMetadata: raw }
}

/**
 * A recreation clip is never the output source: it is read once for its
 * narrative, so only input readiness matters (format, 4–30s, readable
 * metadata). Its own framing is not the requested 9:16 output framing and must
 * not be judged against it.
 */
export function validateRecreateReference(
  metadata: MotionVideoMetadata
): VideoValidationResult {
  if (metadata.version !== 1 || !metadata.browserMetadata) {
    return { ok: false, reason: 'corrupt' }
  }
  return validateBrowserMotionMetadata(metadata.browserMetadata, 'recreate')
}

/**
 * The video-slot gate for the active mode. Motion transfer and object swap
 * also have to preserve the clip's framing at the chosen resolution; a
 * decomposition-only clip has no output-framing contract, so only its own
 * player-readiness is checked.
 */
export function validateMotionReference(
  metadata: MotionVideoMetadata,
  mode: MotionMode,
  resolution: string,
  outputAspects?: Record<string, MotionAspectRatio[]>
): VideoValidationResult {
  if (mode === 'recreate') return validateRecreateReference(metadata)
  return validateMotionOutput(metadata, mode, resolution, outputAspects)
}

/**
 * Both current generation routes preserve a whole-second output and a listed
 * frame, so the requested duration is the rounded picture duration. A fractional
 * reference is serviceable; only a container/picture declaration that disagrees
 * beyond two frames of timebase is not.
 */
export function validateMotionOutput(
  metadata: MotionVideoMetadata,
  mode: MotionMode,
  resolution: string,
  outputAspects?: Record<string, MotionAspectRatio[]>
): VideoValidationResult {
  if (metadata.version !== 1 || !metadata.browserMetadata) {
    return { ok: false, reason: 'corrupt' }
  }
  const checked = validateBrowserMotionMetadata(metadata.browserMetadata, mode)
  if (!checked.ok) return checked
  const raw = metadata.browserMetadata
  const seconds = Math.round(raw.video_duration)
  const frameTolerance = 1 / raw.frame_rate + 0.001
  // The old contract let container and picture each sit within one frame of the
  // same whole second, so an accepted pair could differ by up to two frames.
  // Keep that envelope: fractional durations are fine, inconsistent declarations
  // are not. (A supported audio tail is inside this same bound.)
  const containerTolerance = 2 * frameTolerance
  const containerFollows =
    Math.abs(raw.duration - raw.video_duration) <= containerTolerance
  if (
    raw.duration < 4 ||
    raw.video_duration < 4 ||
    seconds < 4 ||
    seconds > 30 ||
    !containerFollows
  ) {
    return { ok: false, reason: 'output-duration' }
  }
  const aspects = outputAspects
    ? (outputAspects[resolution] ?? [])
    : (MOTION_OUTPUT_ASPECTS[resolution] ?? [])
  if (!checked.aspectRatio || !aspects.includes(checked.aspectRatio)) {
    return { ok: false, reason: 'output-aspect' }
  }
  return { ...checked, durationSec: seconds }
}

/** Only complete current-version data can bypass another local read. */
export function reusableMotionMetadata(
  metadata: MotionVideoMetadata | undefined,
  mode: MotionMode = 'motion'
): MotionVideoMetadata | undefined {
  if (metadata?.version !== 1 || !metadata.browserMetadata) return undefined
  const checked = validateBrowserMotionMetadata(metadata.browserMetadata, mode)
  return checked.ok ? checked : undefined
}

export function motionVideoFailureMessage(
  reason: VideoRejectReason,
  mode: MotionMode
): string {
  switch (reason) {
    case 'format':
      return 'Unsupported video format. Please choose MP4 or MOV.'
    case 'size':
      return 'Video file exceeds 200 MB limit.'
    case 'duration':
      if (mode === 'recreate') {
        return 'Video duration must be between 4 and 15 seconds.'
      }
      return mode === 'motion'
        ? 'Video duration must be between 2 and 30 seconds.'
        : 'Video duration must be between 4 and 30 seconds.'
    case 'duration-short':
      return 'The reference video is too short. Choose a clip of at least 4 seconds.'
    case 'duration-long':
      return 'The reference video is too long. Choose a clip of 15 seconds or fewer.'
    case 'dimensions':
      return 'Video width and height must each be between 300 and 6000 pixels.'
    case 'pixels':
      return 'Video pixel count must be between 407696 and 8295044.'
    case 'aspect':
      return 'Video aspect ratio must be between 0.4 and 2.5.'
    case 'frame-rate':
      return 'Video frame rate must be between 24 and 60 fps.'
    case 'codec':
      return 'Video encoding must be H.264 or H.265.'
    case 'audio':
      return 'Video audio encoding is not supported. Use AAC or MP3, or PCM in MOV.'
    case 'output-duration':
      return 'The current service cannot follow this reference duration. Choose a video from 4 to 30 seconds.'
    case 'output-aspect':
      return 'The current service cannot preserve this reference frame at the selected resolution. Choose another video or resolution.'
    default:
      return 'Could not read this video in your browser. Choose another MP4 or MOV file.'
  }
}

export async function validateMotionVideoSource(
  source: File | string,
  signal?: AbortSignal
): Promise<VideoValidationResult> {
  try {
    return validateMotionMetadata(await inspectVideoSource(source, signal))
  } catch {
    return { ok: false, reason: 'corrupt' }
  }
}

/** File bounds are checked before reading its raw duration and display dimensions. */
export async function validateMotionVideo(
  file: File
): Promise<VideoValidationResult> {
  const basic = checkVideoFileBasic(file)
  if (!basic.ok) return basic
  return validateMotionVideoSource(file)
}

/** Everything the submit gate needs; deliberately narrower than the form state. */
export interface MotionDraft {
  mode: MotionMode
  hasCharacter: boolean
  hasVideo: boolean
}

/**
 * Mode-level input gate. `recreate` marks the reference clip as required but
 * its reference images as optional: the panel adds the "at least one image and
 * an accepted decomposition" rules on top.
 */
export function canSubmitMotion(draft: MotionDraft): boolean {
  const meta = motionModeMeta(draft.mode)
  if (meta.requiresCharacter && !draft.hasCharacter) return false
  if (meta.requiresVideo && !draft.hasVideo) return false
  return true
}

/** The templates that belong to the mode currently selected. */
export function presetsForMode(
  presets: MotionPreset[],
  mode: MotionMode
): MotionPreset[] {
  return presets.filter((preset) => preset.mode === mode)
}
