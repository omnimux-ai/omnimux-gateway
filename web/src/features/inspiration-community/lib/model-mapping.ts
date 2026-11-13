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
 * Resolves inspiration community model keys / slugs to OmniMux playground model IDs.
 *
 * Supported inspiration model tabs:
 * - Seedance 2.5    -> seedance-2-5
 * - Seedance 2.0    -> seedance-2-0
 * - Wan 3.0         -> wan-3.0
 * - GPT Image 2     -> gpt-image-2.5
 * - GPT Image 2.5   -> gpt-image-2.5
 * - Nano Banana Pro -> nano-banana-pro
 * - Grok Imagine    -> grok-imagine-video-1-5 (or grok-imagine-image-2-0 if mediaType === 'image')
 */
export function resolveInspirationPlaygroundModelId(
  modelKey: string,
  mediaType?: string
): string {
  const normalized = modelKey.trim().toLowerCase()

  // Canonical GPT Image 2.5 route (guaranteed to route to official gpt-image-2.5,
  // explicitly excluding flare / sunburst variants).
  if (normalized.startsWith('gpt-image') || normalized === 'image2') {
    return 'gpt-image-2.5'
  }

  switch (normalized) {
    case 'seedance-2.5':
    case 'seedance-2-5':
      return 'seedance-2-5'

    case 'seedance-2.0':
    case 'seedance-2-0':
      return 'seedance-2-0'

    case 'wan-3.0':
    case 'wan-3-0':
      return 'wan-3.0'

    case 'nano-banana-pro':
      return 'nano-banana-pro'

    case 'grok-imagine':
    case 'grok-imagine-video':
    case 'grok-imagine-video-1-5':
      return mediaType === 'image'
        ? 'grok-imagine-image-2-0'
        : 'grok-imagine-video-1-5'

    case 'grok-imagine-image':
    case 'grok-imagine-image-2-0':
      return 'grok-imagine-image-2-0'

    default:
      return normalized
  }
}
