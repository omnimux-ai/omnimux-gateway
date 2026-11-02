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

import type { BrowserVideoMetadata } from './lib/browser-video-metadata'

export interface InfluencerOption {
  id: string
  label_en: string
  visibleIn: string[]
  slot?: string | null
  swatch?: string | null
  imageUrl?: string | null
}

export interface InfluencerCategory {
  id: string
  label_en: string
  kind: 'media' | 'color' | 'text'
  max: number
  options: InfluencerOption[]
}

export interface InfluencerTaxonomy {
  version: number
  source: string
  tier_group: {
    options: InfluencerOption[]
    comingSoonImageUrls: string[]
  }
  category_priority: string[]
  categories: InfluencerCategory[]
  prompt_map: {
    tiers: Record<string, string>
    categories: Record<string, { id: string; fragment: string }[]>
    animal_categories?: Record<string, { id: string; fragment: string }[]>
  }
}

export type Selection = Record<string, string[]>

/**
 * The categories the visitor owns by hand. A lock survives rolls and tier
 * switches, and it is only ever written by a manual pick — never derived from
 * a whole look, or the dice would have nothing left to roll.
 */
export type LookLocks = Record<string, string[]>

export type ImageMode = 'text' | 'reference' | 'edit' | 'masked_edit'

export interface SheetParams {
  model: string
  group?: string
  tier: string
  selection: Selection
  brief: string
  seed?: number
  size?: string
  image_url?: string
  image_mode?: ImageMode
  mask_url?: string
  edit_instruction?: string
}

export interface PricingRow {
  model_name: string
  description?: string
  icon?: string
  tags?: string[]
  vendor_id: number
  quota_type: number // 0 per-token, 1 per-call
  model_ratio?: number
  model_price?: number
  billing_mode?: string
  billing_expr?: string
  enable_groups?: string[]
  supported_endpoint_types?: string[]
}

export interface PricingResponse {
  success: boolean
  data: PricingRow[]
  vendors: { id: number; name: string; description?: string; icon?: string }[]
  group_ratio?: Record<string, number>
  usable_group?: Record<string, unknown>
}

export interface TaskRecord {
  id: number
  task_id: string
  action?: string
  status: string
  /** Server-owned original-result reconciliation; never authorizes another submission. */
  result_unverified?: boolean
  fail_reason?: string
  progress?: string
  quota?: number
  created_at?: string
  submit_time?: number
  finish_time?: number
  properties?: {
    input?: string
    origin_model_name?: string
    upstream_model_name?: string
  }
  artifacts?: {
    key?: string
    type?: string
    mime_type?: string
    thumbnail_url?: string
    thumbnail_width?: number
    thumbnail_height?: number
    content_url?: string
    url?: string
    /** Server-observed media facts of a generated video artifact; absent for
     * older tasks and non-video artifacts. */
    video_metadata?: BrowserVideoMetadata
  }[]
  /** Stored first-frame cover of a video task, present once the gateway holds
   * its own copy; the grid paints it before playback starts. */
  thumbnail_url?: string
  thumbnail_width?: number
  thumbnail_height?: number
  poster_url?: string
  result_url?: string
  legacy_content_url?: string
}

export interface InfluencerTaskMeta {
  tier: string
  selection: Selection
  brief?: string
  seed?: number
  size?: string
  image_url?: string
  image_mode?: ImageMode
  mask_url?: string
  edit_instruction?: string
  /** `multiview` marks a turnaround sheet derived from another task. A row
   * without a kind is a plain character sheet. */
  kind?: InfluencerTaskKind
  /** Set on a multiview row: the character sheet it was derived from. */
  parent_task_id?: string
}

export type InfluencerTaskKind = 'sheet' | 'multiview' | 'motion'

/** The right pane shows either the inspiration library or this user's history. */
export type InfluencerSource = 'explore' | 'history'

/**
 * The inspiration library's three shelves, mirroring the reference product:
 * character presets, motion presets and community-trend clips. `influencers`
 * stays first — a first-time visitor lands on the characters.
 */
export type InfluencerExploreTab = 'influencers' | 'presets' | 'trends'

/**
 * One mirrored motion preset or community-trend clip from
 * `data/motion-explore.json`. The `videoUrl` is a public HTTPS clip the
 * gateway pulls as the driving reference, so "Use" needs no download round
 * trip through the browser.
 */
export interface MotionExplorePreset {
  id: string
  title: string
  description?: string
  /** Optional generation supplement; an empty string keeps description display-only. */
  prompt?: string
  /** `motion` = clone the movement; `replace` = swap the person in the scene. */
  mode: MotionMode
  videoUrl: string
  referenceVideoUrl?: string
  thumbPath: string
  durationSec: number
  jobSetType: string
}

/**
 * One artwork of an inspiration preset. `path` is either a site-root path (a
 * mirrored static snapshot shipped with the build) or a same-origin gateway URL
 * (a row published into the library by an administrator, served back from our
 * own storage).
 */
export interface ExplorePresetAsset {
  path: string
  width: number
  height: number
}

/**
 * One inspiration preset: a ready-made character look plus the sheet
 * parameters that rebuild it. Mirrored from the upstream preset catalogue, so
 * every `selection` entry is known to resolve against our own taxonomy.
 */
export interface ExplorePreset {
  id: string
  name: string
  /** Card artwork, portrait-ish and variable in ratio. */
  thumbnail?: ExplorePresetAsset
  preview: ExplorePresetAsset
  /** The full character sheet, shown enlarged in the preview dialog. */
  sheet: ExplorePresetAsset
  tier: string
  selection: Selection
  brief?: string
  seed?: number
  /** Set on rows an administrator published into the library from their own
   * history; the static mirror omits it. */
  published?: boolean
  /** Publication time in epoch seconds, on published rows only. */
  createdAt?: number
}

/**
 * One row of the published inspiration library (`GET /api/influencer/presets`).
 * The gateway owns the assets and the parameters are copied from the source
 * task, so a published entry never depends on the upstream render surviving.
 */
export interface PublishedPresetRow {
  /** `pub-<row id>`. */
  id: string
  name: string
  tier: string
  selection: Selection
  brief?: string
  seed?: number
  thumbnail?: ExplorePresetAsset
  preview: ExplorePresetAsset
  sheet: ExplorePresetAsset
  /** Publication time in epoch seconds. */
  created_at: number
  published: true
}

/** A readable row of the preset's parameters, for the preview dialog sidebar. */
export interface PresetParamRow {
  categoryId: string
  label: string
  options: string[]
}

/** The workbench's two surfaces: the character builder and the video form. */
export type WorkbenchMode = 'image' | 'video'

/**
 * How the driving video is used. `motion` clones the body movement and keeps
 * the character's identity; `replace` keeps the video's pixels and swaps the
 * person standing in them. `recreate` only decomposes the clip's narrative and
 * pacing, then generates from the caller's own reference images and the
 * server-compiled prompt — the clip never reaches the generation model.
 */
export type MotionMode = 'motion' | 'replace' | 'recreate'

/** One shot of the decomposition. Every field is optional on purpose: an
 * older or partial analysis must still render without throwing. */
export interface RecreateShot {
  shot_id?: string
  start_s?: number
  end_s?: number
  shot_size?: string
  narrative_job?: string
  action?: {
    start_state?: string
    change?: string
    end_state?: string
  }
  product_visibility?: boolean | string | null
}

export interface RecreateHook {
  mechanism?: string[]
  why_it_stops_scroll?: string
  confidence?: string
}

export interface RecreatePace {
  cut_density?: string
  shots_per_10s?: number
  acceleration_points_s?: number[]
}

export interface RecreateProofStep {
  type?: string
  strength?: string
  note?: string
}

export interface RecreateViralLogic {
  structure_formula?: {
    opening?: string
    middle?: string
    late_stage?: string
    ending?: string
  }
  proof_chain?: RecreateProofStep[]
}

export interface RecreateCta {
  type?: string
  carrier?: string
  position_s?: number
}

export interface RecreateBeat {
  beat_id?: string
  source_shots?: string[]
  function?: string
  keep?: string[]
}

export interface RecreateTransferable {
  beats?: RecreateBeat[]
  shot_grammar?: string[]
  rhythm?: string
}

export interface RecreateReplacement {
  target?: string
  found_in?: string[]
  description?: string
  replacement_hint?: string
}

/** The clip breakdown the server returns next to the compiled prompt. */
export interface RecreateAnalysis {
  video_overview?: {
    one_line?: string
    structure_summary?: string
  }
  shots?: RecreateShot[]
  hook?: RecreateHook
  pace?: RecreatePace
  viral_logic?: RecreateViralLogic
  cta?: RecreateCta
  transferable?: RecreateTransferable
  replace?: RecreateReplacement[]
  uncertain?: { field_path?: string; why?: string }[]
}

/** What the image-reading turn locked from one uploaded reference image. */
export interface RecreateImageAnalysis {
  image_index?: number
  kind?: string
  summary?: string
}

export interface RecreateMeta {
  shot_count?: number
  hook?: string
  risk_flags?: string[]
  /** The whole-second generation duration the server derived from the clip. */
  duration_sec?: number
  /** The reference clip's raw duration in seconds (fractional). */
  reference_duration_sec?: number
  /** Server-computed generation credits per quality tier. */
  pricing?: { '480p'?: number; '720p'?: number }
  resolution?: string
  aspect_ratio?: string
  image_count?: number
}

/** `POST /pg/influencer/recreate/analyze` success payload. */
export interface RecreateAnalyzeResponse {
  prompt: string
  analysis?: RecreateAnalysis | null
  image_analysis?: RecreateImageAnalysis[] | null
  meta?: RecreateMeta | null
}

/** The accepted decomposition held by the form. The output duration is not a
 * user choice: it comes from the reference clip, so the result keeps the
 * server-derived whole seconds (or the local rounded fallback) for submission. */
export interface RecreateResult {
  prompt: string
  analysis: RecreateAnalysis | null
  imageAnalysis: RecreateImageAnalysis[]
  meta: RecreateMeta | null
  durationSec: number
  referenceDurationSec?: number
}

/**
 * A built-in video template. `thumbUrl` stays absent while the pack is
 * placeholder-only, and the strip then draws its own tile instead of a broken
 * image.
 */
export interface MotionPreset {
  id: string
  /** An i18n key resolved through `t()` at render time, so it lives in
   * `src/i18n/static-keys.ts` — the literal scanner cannot see it here. */
  nameKey: string
  mode: MotionMode
  /** Display description, separate from instructions sent to generation. */
  description?: string
  /** Generation supplement joined with an enabled personal prompt. */
  prompt: string
  thumbUrl?: string
  /** The template's public clip URL; the gateway fetches it as the driving
   * reference. Absent while a placeholder pack has no real clip yet. */
  videoUrl?: string
  /** Full-quality generation input; videoUrl remains the lightweight preview. */
  referenceVideoUrl?: string
  durationSec: number
}

export type MotionVisualStyle = {
  id: string
  nameKey: string
  summaryKey: string
  prompt: string
  mode: 'motion'
  /** Display-only style cover; never attached as a character or style input. */
  previewImageUrl?: string
  /** Display-only style preview; never replaces the user's reference video. */
  previewVideoUrl?: string
}

export type MotionStyleChoice = MotionPreset | MotionVisualStyle

/**
 * One file travelling from the reference picker's multi-select input to the
 * form. It keeps its own local preview and progress so a batch stays readable
 * while three transfers run at once.
 */
export interface PendingCharacterUpload {
  id: string
  fileName: string
  /** Local object URL; revoked once the entry leaves the queue. */
  previewUrl: string
  /** 0–100 transfer progress for this file. */
  progress: number
  status: 'waiting' | 'uploading' | 'error'
}

/** The character image that goes into the motion form. */
export interface MotionCharacterSource {
  /** Public URL used for selection identity and submission. */
  url: string
  /** Historical artifact display address; uploads keep their original URL. */
  previewUrl?: string
  fileName: string
  characterType?: 'human' | 'cat' | 'dog'
  sourceTaskId?: string
}

/**
 * What fills the video slot. A built-in template carries its motion on the
 * server and is referenced by id; an upload carries the clip's own public URL.
 * Both mean "the slot is filled" to the submit gate, and the second phase sends
 * whichever one is present.
 */
export type MotionAspectRatio = '16:9' | '9:16' | '3:4'

export interface MotionVideoMetadata {
  /** Versioned complete browser result, bound to the source URL in its record. */
  version?: 1
  browserMetadata?: BrowserVideoMetadata
  rawDurationSec: number
  durationSec: number
  width: number
  height: number
  /** The clip's own display ratio when it matches a standard value — input
   * metadata. Null for a legal input the current output service cannot preserve. */
  aspectRatio: MotionAspectRatio | null
}

export type MotionVideoSource =
  | { kind: 'preset'; presetId: string; nameKey: string }
  | {
      kind: 'upload'
      url: string
      fileName: string
      metadata?: MotionVideoMetadata
    }
  | {
      kind: 'remote'
      url: string
      name: string
      previewUrl?: string
      taskId?: string
      artifactKey?: string
      metadata?: MotionVideoMetadata
    }
