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

import {
  IMAGE_SIZE,
  SHEET_SIZE,
  type MotionSubmitBody,
  type MultiViewSubmitBody,
  type SheetSubmitBody,
} from '../api'
import type {
  MotionAspectRatio,
  PricingResponse,
  Selection,
  TaskRecord,
} from '../types'
import {
  quoteHistoricalImageRoute,
  SUNBURST_MODEL,
  type ImageRoute,
} from './catalog'
import {
  isSuccessStatus,
  parentTaskId,
  parseTaskMeta,
  taskKind,
} from './history'
import type { PaywallOrigin } from './insufficient-credits'
import { canDeriveMultiView, MULTIVIEW_SIZE } from './multiview'
import {
  matchingImageArtifactUrl,
  publicReferenceUrl,
} from './reference-candidates'
import { parseReplayableMotionPayload } from './submit-payload'
import { conflictMessage, isAnimalTier, totalSelected } from './taxonomy'
import { quoteVideoCredits } from './video-quote'

export interface SubmitSheet {
  tier: string
  selection: Selection
  seed?: string
  image_url?: string
}

export type GenerationRequest =
  | { origin: 'image'; body: SheetSubmitBody }
  | { origin: 'video'; body: MotionSubmitBody }
  | { origin: 'multiview'; body: MultiViewSubmitBody }

export interface PaywallTarget {
  origin: PaywallOrigin
  required: number | null
  generation?: GenerationRequest
}

export interface HistoryPaywallRequest {
  task: TaskRecord
  parent?: TaskRecord
  accepted?: PaywallTarget
}

export function sheetSubmitBody(
  sheet: SubmitSheet,
  route: ImageRoute
): SheetSubmitBody {
  const imageUrl = sheet.image_url?.trim() || undefined
  const sunburst = route.model === SUNBURST_MODEL
  const mode = imageUrl ? 'reference' : 'text'
  return {
    tier: sheet.tier,
    selection: sheet.selection,
    brief: '',
    size: sunburst ? IMAGE_SIZE : SHEET_SIZE,
    image_url: imageUrl,
    image_mode: sunburst ? mode : undefined,
    model: route.model,
    group: route.group,
  }
}

export function quoteImageGenerationRoute(
  generation: Exclude<GenerationRequest, { origin: 'video' }>,
  pricing: PricingResponse | undefined,
  groups: Record<string, { ratio: number }> | undefined
): ReturnType<typeof quoteHistoricalImageRoute> {
  const { model, group } = generation.body
  return quoteHistoricalImageRoute(
    { model, group: group ?? '' },
    pricing,
    groups,
    generation.origin === 'multiview' || Boolean(generation.body.image_url)
  )
}

export function historicalPaywallTarget(
  request: HistoryPaywallRequest,
  history: readonly TaskRecord[],
  pricing: PricingResponse | undefined,
  groups: Record<string, { ratio: number }> | undefined,
  outputAspects?: Record<string, MotionAspectRatio[]>
): PaywallTarget {
  const { task, accepted } = request
  const kind = taskKind(task)
  if (kind === 'motion') {
    if (accepted?.generation?.origin === 'video') return accepted
    const body = parseReplayableMotionPayload(
      task.properties?.input,
      outputAspects
    )
    if (!body) return { origin: 'video', required: null }
    return {
      origin: 'video',
      required:
        quoteVideoCredits(pricing, groups, body.resolution, body.duration_sec)
          ?.credits ?? null,
      generation: { origin: 'video', body },
    }
  }

  const origin = kind === 'multiview' ? 'multiview' : 'image'
  const topUpOnly: PaywallTarget = { origin, required: null }
  const meta = parseTaskMeta(task)
  const savedModel = task.properties?.origin_model_name?.trim()
  const savedGroup = task.group?.trim()
  const submitted = accepted?.generation
  if (submitted?.origin === origin) {
    const { model, group } = submitted.body
    if (
      (savedModel && savedModel !== model) ||
      (savedGroup && savedGroup !== group)
    ) {
      return topUpOnly
    }
    const quote = quoteImageGenerationRoute(submitted, pricing, groups)
    return quote
      ? { origin, required: quote.credits, generation: submitted }
      : topUpOnly
  }

  if (!savedModel || !savedGroup || !meta) return topUpOnly
  const route = { model: savedModel, group: savedGroup }
  const imageUrl = meta.image_url?.trim()
  const quote = quoteHistoricalImageRoute(
    route,
    pricing,
    groups,
    origin === 'multiview' || Boolean(imageUrl)
  )
  if (!quote) return topUpOnly

  if (origin === 'multiview') {
    const parentId = parentTaskId(task)
    if (!parentId || !imageUrl || meta.size !== MULTIVIEW_SIZE) return topUpOnly
    const parent =
      history.find((item) => item.task_id === parentId) ?? request.parent
    if (
      !parent ||
      parent.task_id !== parentId ||
      !isSuccessStatus(parent) ||
      !canDeriveMultiView(parent)
    ) {
      return topUpOnly
    }
    const parentMeta = parseTaskMeta(parent)
    if (
      !parentMeta ||
      (!isAnimalTier(parentMeta.tier) &&
        !totalSelected(parentMeta.selection) &&
        !parentMeta.brief?.trim() &&
        !parentMeta.image_url?.trim())
    ) {
      return topUpOnly
    }
    const image = matchingImageArtifactUrl(parent, imageUrl)
    if (!image) return topUpOnly
    return {
      origin,
      required: quote.credits,
      generation: {
        origin,
        body: {
          ...route,
          parentTaskId: parentId,
          imageUrl: image,
          size: meta.size,
        },
      },
    }
  }

  if (
    meta.brief?.trim() ||
    meta.seed ||
    meta.mask_url ||
    meta.edit_instruction ||
    (imageUrl && !publicReferenceUrl(imageUrl)) ||
    (meta.image_mode &&
      meta.image_mode !== (imageUrl ? 'reference' : 'text')) ||
    (meta.size !== SHEET_SIZE &&
      !(savedModel === SUNBURST_MODEL && meta.size === IMAGE_SIZE)) ||
    conflictMessage(meta.tier, meta.selection) ||
    (!isAnimalTier(meta.tier) && !totalSelected(meta.selection) && !imageUrl)
  ) {
    return topUpOnly
  }
  return {
    origin,
    required: quote.credits,
    generation: {
      origin,
      body: sheetSubmitBody(
        { tier: meta.tier, selection: meta.selection, image_url: imageUrl },
        route
      ),
    },
  }
}
