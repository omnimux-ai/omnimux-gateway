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
 * Types for the Inspiration Community feature (PRD section 3 and 4).
 *
 * Field names mirror the collected Topview / EvoLink community feeds
 * (`docs/product/inspiration-community-prd-zh.md` section 8) so the S2 data
 * swap can drop the fixtures without reshaping the UI layer.
 */

/** Media kind of a prompt item. Mirrors `taskType` in the source feeds. */
export type PromptMediaType = 'image' | 'video'

/** Sort dimension for the prompt gallery. */
export type PromptSort = 'latest' | 'hottest'

/**
 * One reusable prompt + its rendered artifact.
 *
 * `id` is the cross-source primary key `source:model:slug` defined in PRD
 * section 8; `prompt` is truncated to 400 characters in the checked-in
 * fixture.
 */
export interface PromptItem {
  /** Cross-source primary key: `source:modelKey:slug`. */
  id: string
  slug: string
  /** Machine model key, e.g. `seedance-2.5`. */
  modelKey: string
  /** Display label of the model section, e.g. `Seedance 2.5`. */
  modelLabel: string
  mediaType: PromptMediaType
  /** Chinese display title (the crawl's `titleOriginal` / `titleZh`). */
  title: string
  /**
   * English display title. Every catalog row carries both languages, so the UI
   * can render the one the active locale asks for instead of showing Chinese to
   * an English reader. Optional so the bundled demo fixture stays valid.
   */
  titleEn?: string
  /**
   * Human-written one-paragraph summary of the artifact, taken from the source
   * crawl (`descriptionOriginal` / `descriptionEn` on Topview, `summary` on
   * EvoLink) — never derived from `prompt`. Optional, and empty when the row
   * carries none; the card then renders no description at all.
   */
  description?: string
  /** English counterpart of `description`; see `titleEn`. */
  descriptionEn?: string
  /** Prompt body, truncated to 400 characters in the demo fixture. */
  prompt: string
  /** Primary category facet, mirrors `categoryPrimary`. */
  category: string
  /** Use-case facets, mirrors `facetUseCases`. */
  useCases: string[]
  /** Style facets, mirrors `facetStyles`. */
  styles: string[]
  /** Source tags, mirrors `tagsSource` / `tags`. */
  tags: string[]
  /** Still frame used for image items and as the video fallback. */
  previewImageUrl: string
  /** Animated preview for video items; empty for image items. */
  previewVideoUrl: string
  /** Poster frame for video items. */
  posterUrl: string
  /** CSS aspect-ratio of the poster, e.g. `"9 / 16"`. Baked at build time from the image header. */
  aspectRatio: string
  /** Original author handle from the source platform. */
  authorName: string
  /** Source platform key, e.g. `twitter`. */
  sourcePlatform: string
  /** Permalink to the original post. */
  sourceUrl: string
  /** Publish timestamp `YYYY-MM-DDTHH:mm:ss` (local to the source crawl). */
  publishedAt: string
  /** Display engagement count (demo stand-in when the crawl exposes none). */
  viewCount: number
  /** Raw `viewCount` from the source crawl; `0` when the crawl omits it. */
  originalViewCount: number
  /**
   * Live-demo link shown in the detail layer footer. Only populated for
   * skillry-sourced rows; absent everywhere else.
   */
  demoUrl?: string
}

/** A model section rendered on `/inspiration`. */
export interface PromptSection {
  key: string
  label: string
  count: number
}

/**
 * Media kind of a creative, mirroring the service's `type`. `link` is a real
 * wire value (a post that only points somewhere else); a payload whose `type`
 * is none of the three is carried as `null` rather than guessed into one.
 */
export type TikTokMediaType = 'video' | 'image' | 'link'

/** Content-type filter of the ad library (`type=video|image`). */
export type TikTokContentType = Extract<TikTokMediaType, 'video' | 'image'>

/**
 * Traffic mix of a creative, mirroring the service's `trafficType`. These are
 * the wire values, not the old demo vocabulary, so the label table maps `ad`
 * onto the paid-traffic copy.
 */
export type TikTokTrafficType = 'ad' | 'organic'

/** Clip-length bucket ids used by the Clip length filter. */
export type TikTokDurationBucket = 'lt15' | 's15to30' | 's30to60' | 'gt60'

/** View bucket ids used by the Views filter. */
export type TikTokViewsBucket =
  | 'lt10k'
  | 'k10to100k'
  | 'k100to1m'
  | 'm1to10m'
  | 'gt10m'

/** Publish-window bucket ids used by the Published filter. */
export type TikTokPublishedBucket = 'm6' | 'y1' | 'older'

/** Sort dimension of the ad library; the service takes `views` and `new`. */
export type TikTokSort = 'views' | 'new'

/** Creator attribution of a creative, mirroring the service's `creator`. */
export interface TikTokAdCreator {
  name: string
  handle: string
}

/**
 * One TikTok creative as the page renders it.
 *
 * Every field comes from the service payload; there is no placeholder column
 * to fill. Optional-by-nature fields are `null` (or `''` for the text ones) and
 * the UI hides them instead of printing a stand-in:
 *
 * - `views` is `null` when the payload reports no count at all — never `0`,
 *   which is a real count.
 * - `durationSeconds` is `null` when the clip length is unknown: the service
 *   reports `0` for photo posts, and that 0 is not "zero seconds".
 * - `trafficType` is `null` for a value outside the documented pair.
 * - `postedAt`, `coverUrl`, `sourceUrl` and `creator` are `null` when the
 *   payload carries none.
 *
 * `coverGradient` is the one derived field: an offline-safe backdrop for a
 * creative with no cover, keyed off `id` so it is stable across renders.
 */
export interface TikTokAdItem {
  id: string
  type: TikTokMediaType | null
  title: string
  caption: string
  /** ISO 3166-1 alpha-2 region code; `''` when the service carries none. */
  country: string
  /** Source category label; `''` when the service carries none. */
  category: string
  durationSeconds: number | null
  views: number | null
  trafficType: TikTokTrafficType | null
  postedAt: string | null
  /** Same-origin cover path of `/public/media/inspiration-covers/…`, or null. */
  coverUrl: string | null
  /**
   * Playable media of the creative in the service's order; may be empty.
   *
   * Contract data only — every object under `/r2/…` answers `404` today, so
   * nothing on the page builds a player out of this list. Playback comes from
   * `embedUrl` alone.
   */
  mediaUrls: string[]
  /**
   * TikTok's own player URL for a creative with a real TikTok video (~24% of
   * the corpus), or null. Cards show a play affordance and the detail layer
   * mounts an iframe only when this has a value.
   */
  embedUrl: string | null
  /** Permalink to the original post; null when the service has no real one. */
  sourceUrl: string | null
  creator: TikTokAdCreator | null
  /** CSS `background` value behind the cover, used when there is no image. */
  coverGradient: string
}

/**
 * Filter state of the ad library.
 *
 * One value per dimension, because that is what the service accepts: a single
 * `country`, `type` and `traffic_type`, and one range each for the numeric
 * dimensions. `null` means "no constraint".
 */
export interface TikTokFilters {
  country: string | null
  contentType: TikTokContentType | null
  duration: TikTokDurationBucket | null
  views: TikTokViewsBucket | null
  trafficType: TikTokTrafficType | null
  published: TikTokPublishedBucket | null
}
