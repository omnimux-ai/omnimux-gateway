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
/**
 * Media predicates of the Inspiration Community cards.
 *
 * The crawled feeds are not consistent about which media field holds a video:
 * some catalog rows published an `.mp4` as their poster, so a card that renders
 * `posterUrl` in an `<img>` without checking shows a broken frame instead of a
 * still. The cards carry no `<video>` — they are a browsing surface, and a grid
 * of tiles must never start playing on its own — so such a row is dropped from
 * the tile and the box keeps its flat backdrop; playback lives in the detail
 * layers.
 */

const VIDEO_FILE_PATTERN = /\.(?:mp4|webm|mov|m4v)$/iu

/**
 * Whether a cover URL points at a video file rather than a still image.
 * A query string or fragment is ignored, so a signed `.mp4?token=…` URL still
 * counts as video.
 */
export function isVideoCoverUrl(url: string): boolean {
  const path = url.split(/[?#]/u)[0]?.trim() ?? ''
  return VIDEO_FILE_PATTERN.test(path)
}

/**
 * Backdrops for a creative the service has no cover for, in the same tone as a
 * dark media still so the white title overlay and the play affordance stay
 * readable on either theme.
 */
const COVER_GRADIENTS = [
  'linear-gradient(135deg, #1f2937 0%, #0f172a 100%)',
  'linear-gradient(135deg, #26303e 0%, #151b26 100%)',
  'linear-gradient(135deg, #2a2440 0%, #16121f 100%)',
  'linear-gradient(135deg, #123038 0%, #0d1f24 100%)',
  'linear-gradient(135deg, #33272a 0%, #1c1416 100%)',
] as const

/**
 * Deterministic offline-safe backdrop for a creative id, so a card without a
 * cover (or whose cover fails to load) still renders a stable dark tile instead
 * of a broken image, and the same row keeps the same backdrop across renders.
 */
export function coverGradientFor(id: string): string {
  let hash = 0
  for (let index = 0; index < id.length; index++) {
    hash = (hash * 31 + id.charCodeAt(index)) % 100_000_007
  }
  return COVER_GRADIENTS[Math.abs(hash) % COVER_GRADIENTS.length]
}
