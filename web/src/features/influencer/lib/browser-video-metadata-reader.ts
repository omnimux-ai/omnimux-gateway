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

export interface BrowserVideoMetadata {
  duration: number
  video_duration: number
  width: number
  height: number
  frame_rate: number
  video_codec: string
  audio_codec: string
  container: string
  size_bytes: number
}

export type BrowserVideoMetadataErrorCode =
  | 'unreadable'
  | 'unsupported'
  | 'timeout'
  | 'budget-exceeded'

export class BrowserVideoMetadataError extends Error {
  constructor(
    public readonly code: BrowserVideoMetadataErrorCode,
    message: string
  ) {
    super(message)
    this.name = 'BrowserVideoMetadataError'
  }
}

export const VIDEO_READ_CHUNK_SIZE = 256 * 1024
export const VIDEO_READ_BUDGET = 32 * 1024 * 1024
export const VIDEO_UNSUPPORTED_MESSAGES = {
  pixelAspect:
    'Videos with non-square pixels cannot be followed exactly. Choose a video with square pixels.',
  rotation:
    'Only videos rotated by 0°, 90°, 180° or 270° can be followed exactly.',
  videoTracks:
    'Videos with multiple video tracks cannot be followed exactly. Choose a video with a single video track.',
} as const
const MAX_FILE_BYTES = 200 * 1024 * 1024
const MAX_READ_COUNT = 256

type Track = Record<string, unknown>

function unreadable(message: string): never {
  throw new BrowserVideoMetadataError('unreadable', message)
}

function positive(value: unknown): number {
  const parsed = typeof value === 'number' ? value : Number(value)
  if (!Number.isFinite(parsed) || parsed <= 0) {
    return unreadable('Required video metadata is missing or invalid')
  }
  return parsed
}

function videoCodec(track: Track): string {
  const format = String(track.Format ?? '').toLowerCase()
  if (format === 'avc') return 'h264'
  if (format === 'hevc') return 'hevc'
  return format
}

function audioCodec(track: Track | undefined): string {
  if (!track) return ''
  const format = String(track.Format ?? '').toLowerCase()
  if (!format) return unreadable('Audio codec metadata is missing')
  if (
    format === 'mpeg audio' &&
    String(track.Format_Profile).toLowerCase() === 'layer 3'
  ) {
    return 'mp3'
  }
  return format
}

/** Map container metadata only; input/output capability rules belong to motion.ts. */
export function normalizeVideoMetadata(
  value: unknown,
  size: number
): BrowserVideoMetadata {
  const tracks = (value as { media?: { track?: Track[] } } | null)?.media?.track
  if (
    !Array.isArray(tracks) ||
    tracks.some((track) => !track || typeof track !== 'object')
  ) {
    return unreadable('Video track metadata is missing')
  }
  const general = tracks.filter((track) => track['@type'] === 'General')
  const video = tracks.filter((track) => track['@type'] === 'Video')
  const audio = tracks.filter((track) => track['@type'] === 'Audio')
  if (video.length > 1) {
    throw new BrowserVideoMetadataError(
      'unsupported',
      VIDEO_UNSUPPORTED_MESSAGES.videoTracks
    )
  }
  if (
    general.length !== 1 ||
    video.length !== 1 ||
    audio.length > 1 ||
    tracks.some(
      (track) =>
        !['General', 'Video', 'Audio', 'Text', 'Menu', 'Other'].includes(
          String(track['@type'])
        )
    )
  ) {
    throw new BrowserVideoMetadataError(
      'unsupported',
      'Unsupported video or audio track count'
    )
  }
  if (
    Number(general[0].VideoCount ?? video.length) !== video.length ||
    Number(general[0].AudioCount ?? audio.length) !== audio.length
  ) {
    return unreadable('Video track metadata is incomplete')
  }
  if (
    tracks.some(
      (track) =>
        track.Encryption &&
        !['no', 'none'].includes(String(track.Encryption).toLowerCase())
    )
  ) {
    throw new BrowserVideoMetadataError(
      'unsupported',
      'Encrypted video is not supported'
    )
  }
  const stream = video[0]
  let width = positive(stream.Width)
  let height = positive(stream.Height)
  if (!Number.isInteger(width) || !Number.isInteger(height)) {
    return unreadable('Video dimensions must be whole pixels')
  }
  // Non-square pixels cannot be represented by the existing width/height contract
  // without resampling or changing the source dimensions. Fail explicitly.
  if (
    stream.PixelAspectRatio !== undefined &&
    Math.abs(positive(stream.PixelAspectRatio) - 1) > 0.001
  ) {
    throw new BrowserVideoMetadataError(
      'unsupported',
      VIDEO_UNSUPPORTED_MESSAGES.pixelAspect
    )
  }
  const rotation = stream.Rotation === undefined ? 0 : Number(stream.Rotation)
  if (
    !Number.isFinite(rotation) ||
    Math.abs(rotation / 90 - Math.round(rotation / 90)) > 0.0001
  ) {
    throw new BrowserVideoMetadataError(
      'unsupported',
      VIDEO_UNSUPPORTED_MESSAGES.rotation
    )
  }
  if (Math.abs(Math.round(rotation / 90)) % 2 === 1) {
    ;[width, height] = [height, width]
  }
  const format = String(general[0].Format ?? '').toLowerCase()
  const brand = String(general[0].CodecID ?? '')
    .trim()
    .toLowerCase()
  let container = format
  if (format === 'mpeg-4') container = brand === 'qt' ? 'mov' : 'mp4'
  if (format === 'quicktime') container = 'mov'
  const codec = videoCodec(stream)
  if (!container || !codec) {
    return unreadable('Video container or codec metadata is missing')
  }
  return {
    duration: positive(general[0].Duration),
    video_duration: positive(stream.Duration),
    width,
    height,
    frame_rate: positive(stream.FrameRate),
    video_codec: codec,
    audio_codec: audioCodec(audio[0]),
    container,
    size_bytes: positive(size),
  }
}

export interface VideoDataSource {
  size: number
  readChunk(size: number, offset: number): Promise<Uint8Array>
}

function validateSize(size: number): void {
  if (!Number.isSafeInteger(size) || size <= 0 || size > MAX_FILE_BYTES) {
    throw new BrowserVideoMetadataError(
      'budget-exceeded',
      'Video file size exceeds the browser inspection limit'
    )
  }
}

async function rangeResponse(
  url: string,
  start: number,
  end: number
): Promise<Response> {
  const response = await fetch(url, {
    method: 'GET',
    mode: 'cors',
    credentials: 'omit',
    redirect: 'error',
    headers: { Range: `bytes=${start}-${end}` },
  })
  if (response.status !== 206) {
    await response.body?.cancel()
    throw new BrowserVideoMetadataError(
      'unsupported',
      'Remote video does not support browser range inspection'
    )
  }
  return response
}

async function readResponse(
  response: Response,
  expected: number
): Promise<Uint8Array> {
  const declared = response.headers.get('Content-Length')
  if ((declared !== null && Number(declared) !== expected) || !response.body) {
    await response.body?.cancel()
    return unreadable('Remote video range length is invalid')
  }
  const reader = response.body.getReader()
  const bytes = new Uint8Array(expected)
  let length = 0
  try {
    for (;;) {
      const next = await reader.read()
      if (next.done) break
      if (next.value.byteLength > expected - length) {
        await reader.cancel()
        return unreadable('Remote video range exceeded the requested size')
      }
      bytes.set(next.value, length)
      length += next.value.byteLength
    }
  } finally {
    reader.releaseLock()
  }
  if (length !== expected) {
    return unreadable('Remote video range was incomplete')
  }
  return bytes
}

function validateReadRequest(
  requested: number,
  offset: number,
  size: number
): void {
  if (
    !Number.isSafeInteger(requested) ||
    requested <= 0 ||
    requested > VIDEO_READ_CHUNK_SIZE ||
    !Number.isSafeInteger(offset) ||
    offset < 0 ||
    offset >= size
  ) {
    throw new BrowserVideoMetadataError(
      'budget-exceeded',
      'Video parser requested an invalid or excessive range'
    )
  }
}

/**
 * Generated artifacts and uploaded files are fronted by the gateway, which
 * redirects or proxies every byte range. One round trip per parser chunk turned
 * a normal 4–30 second clip into thirty serial requests and an inspection
 * timeout, so an object that fits the whole-file budget is read once and parsed
 * from memory. Larger objects keep the bounded random-access path below.
 */
async function readRemoteObject(url: string): Promise<Uint8Array | undefined> {
  const response = await fetch(url, {
    method: 'GET',
    mode: 'cors',
    credentials: 'omit',
    redirect: 'follow',
  })
  if (response.status !== 200) {
    await response.body?.cancel()
    return undefined
  }
  const declared = response.headers.get('Content-Length')
  const size = declared === null ? Number.NaN : Number(declared)
  if (!Number.isSafeInteger(size) || size <= 0) {
    await response.body?.cancel()
    return undefined
  }
  try {
    validateSize(size)
  } catch (error) {
    await response.body?.cancel()
    throw error
  }
  if (size > VIDEO_READ_BUDGET) {
    await response.body?.cancel()
    return undefined
  }
  return readResponse(response, size)
}

/** The whole object already fits the read budget, so slices stay in memory. */
function wholeDataSource(bytes: Uint8Array): VideoDataSource {
  const size = bytes.byteLength
  let readCount = 0
  return {
    size,
    async readChunk(requested: number, offset: number): Promise<Uint8Array> {
      validateReadRequest(requested, offset, size)
      if (++readCount > MAX_READ_COUNT) {
        throw new BrowserVideoMetadataError(
          'budget-exceeded',
          'Video inspection exceeded its bounded read count'
        )
      }
      const length = Math.min(requested, size - offset)
      return bytes.slice(offset, offset + length)
    },
  }
}

/** Random access for remote objects above the whole-file read budget. */
async function createRemoteRangeDataSource(
  url: string
): Promise<VideoDataSource> {
  let identityHeader = 'ETag'
  const response = await rangeResponse(url, 0, 0)
  const range = /^bytes 0-0\/(\d+)$/.exec(
    response.headers.get('Content-Range') ?? ''
  )
  if (!range) {
    await response.body?.cancel()
    return unreadable('Remote video range metadata is unavailable')
  }
  const size = Number(range[1])
  try {
    validateSize(size)
  } catch (error) {
    await response.body?.cancel()
    throw error
  }
  let identity: string | null = response.headers.get(identityHeader)
  if (!identity) {
    identityHeader = 'Last-Modified'
    identity = response.headers.get(identityHeader)
  }
  const firstByte = await readResponse(response, 1)
  let bytesRead = firstByte.byteLength
  let readCount = 0
  return {
    size,
    async readChunk(requested: number, offset: number): Promise<Uint8Array> {
      validateReadRequest(requested, offset, size)
      const length = Math.min(requested, size - offset)
      if (
        bytesRead + length > VIDEO_READ_BUDGET ||
        ++readCount > MAX_READ_COUNT
      ) {
        throw new BrowserVideoMetadataError(
          'budget-exceeded',
          'Video inspection exceeded its bounded read budget'
        )
      }
      bytesRead += length
      if (offset === 0 && length === 1) return firstByte
      const response = await rangeResponse(url, offset, offset + length - 1)
      if (
        response.headers.get('Content-Range') !==
          `bytes ${offset}-${offset + length - 1}/${size}` ||
        (identity !== null && response.headers.get(identityHeader) !== identity)
      ) {
        await response.body?.cancel()
        return unreadable('Remote video changed during inspection')
      }
      return readResponse(response, length)
    },
  }
}

/** Bounded access: only a clip inside the read budget is buffered whole. */
export async function createVideoDataSource(
  source: File | string
): Promise<VideoDataSource> {
  if (typeof source === 'string') {
    let parsed: URL
    try {
      parsed = new URL(source)
    } catch {
      return unreadable('Remote video URL is invalid')
    }
    if (
      !['https:', 'http:'].includes(parsed.protocol) ||
      parsed.username ||
      parsed.password
    ) {
      throw new BrowserVideoMetadataError(
        'unsupported',
        'Remote video URL scheme is unsupported'
      )
    }
    const whole = await readRemoteObject(parsed.href)
    if (whole) return wholeDataSource(whole)
    return createRemoteRangeDataSource(parsed.href)
  }
  const size = source.size
  validateSize(size)
  let bytesRead = 0
  let readCount = 0
  return {
    size,
    async readChunk(requested: number, offset: number): Promise<Uint8Array> {
      validateReadRequest(requested, offset, size)
      const length = Math.min(requested, size - offset)
      if (
        bytesRead + length > VIDEO_READ_BUDGET ||
        ++readCount > MAX_READ_COUNT
      ) {
        throw new BrowserVideoMetadataError(
          'budget-exceeded',
          'Video inspection exceeded its bounded read budget'
        )
      }
      bytesRead += length
      const bytes = new Uint8Array(
        await source.slice(offset, offset + length).arrayBuffer()
      )
      if (bytes.byteLength !== length) {
        return unreadable('Local video range was incomplete')
      }
      return bytes
    },
  }
}
