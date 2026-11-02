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
import { vendorSubTaskFields } from './operation-matrix'
import type {
  MediaRole,
  PlaygroundOperation,
  PlaygroundParamSchema,
  PlaygroundParams,
} from '../types'
import { submitParams } from './param-schema'

/**
 * Where each media slot lands on the wire.
 *
 * The gateway accepts `image_with_roles` for frame pairs and the three `*_urls`
 * arrays for reference material; it has no field for the form's own control
 * keys, so this mapping is what turns the form into a valid request body.
 */
const WIRE_BY_SLOT: Record<string, { field: string; role?: MediaRole }> = {
  first_frame: { field: 'image_with_roles', role: 'first_frame' },
  last_frame: { field: 'image_with_roles', role: 'last_frame' },
  reference_images: { field: 'image_urls' },
  reference_videos: { field: 'video_urls' },
  reference_audios: { field: 'audio_urls' },
  source_videos: { field: 'video_urls' },
}

/** Frame roles are emitted in this order so first/last never swap. */
const ROLE_ORDER: MediaRole[] = ['first_frame', 'last_frame']

function mediaKeys(schema: PlaygroundParamSchema): string[] {
  return schema.controls
    .filter((control) => control.kind === 'media')
    .map((control) => control.key)
}

function stringArray(value: unknown): string[] {
  if (!Array.isArray(value)) return []
  return value.filter((item): item is string => typeof item === 'string')
}

/**
 * Shapes form parameters into the request body the gateway validates.
 *
 * `submitParams` already handled the scalar coercions; this layer exists for the
 * two things it cannot know: which media family the chosen mode uses, and the
 * vendor sub-task field the mode implies.
 */
export function toSubmitBody(
  modelId: string,
  operation: PlaygroundOperation | undefined,
  schema: PlaygroundParamSchema,
  params: PlaygroundParams
): PlaygroundParams {
  const body: PlaygroundParams = submitParams(schema, params)

  const frames = new Map<MediaRole, string[]>()
  const wireArrays = new Map<string, string[]>()

  for (const key of mediaKeys(schema)) {
    // A slot with no wire mapping is left exactly as it was: the flat form's
    // single media control predates this layer, and there is no verified field
    // name to move it to outside the mode-aware path.
    const target = WIRE_BY_SLOT[key]
    if (!target) continue

    const urls = stringArray(body[key])
    delete body[key]
    if (urls.length === 0) continue

    if (target.role) {
      frames.set(target.role, urls)
      continue
    }
    wireArrays.set(target.field, [...(wireArrays.get(target.field) ?? []), ...urls])
  }

  const roleEntries = ROLE_ORDER.filter((role) => frames.has(role)).map((role) => ({
    url: frames.get(role)?.[0] ?? '',
    role,
  }))
  const imageWithRoles = roleEntries.filter((entry) => entry.url !== '')
  if (imageWithRoles.length > 0) {
    body.image_with_roles = imageWithRoles
  }

  for (const [field, urls] of wireArrays) {
    body[field] = urls
  }

  // The image edit surface takes references under `images`, so the flat slot is
  // renamed here rather than in WIRE_BY_SLOT: the video contract's own field
  // names do not apply to an image model.
  if (schema.modality === 'image') {
    const references = stringArray(body.reference_media)
    delete body.reference_media
    if (references.length > 0) body.images = references
  }

  // fork: wan30-resolution-tier-group — the public `wan-3.0` id is never
  // rewritten to a derivative id (those are delisted) and the request carries
  // no quality tier: the tier is the group named by `X-Omnimux-Group`, chosen
  // from the request's resolution. A `quality_tier` left over from a pasted
  // document is dropped so it cannot reach the gateway as a dead field.
  if (modelId === 'wan-3.0') {
    delete body.quality_tier
  }

  if (operation) {
    body.operation = operation
    for (const [field, value] of Object.entries(
      vendorSubTaskFields(modelId, operation)
    )) {
      body[field] = value
    }
  }

  return body
}
