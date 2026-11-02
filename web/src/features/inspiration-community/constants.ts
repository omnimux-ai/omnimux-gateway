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
 * Option tables for the Inspiration Community filters.
 *
 * Values are i18n keys (identical to the English source copy) per
 * `web/AGENTS.md` 3.1; render them through `t(option.labelKey)`. Facet labels
 * that come from crawled data (prompt categories, tags) are source content and
 * are deliberately absent here.
 */
import type {
  TikTokContentType,
  TikTokDurationBucket,
  TikTokPublishedBucket,
  TikTokSort,
  TikTokTrafficType,
  TikTokViewsBucket,
} from './types'

/**
 * Traffic-type labels. These keys are feature-scoped on purpose: the bare
 * `Paid` key already means "settled / 已支付" in the billing surfaces, which is
 * the wrong sense for a paid-traffic bucket. The service's wire values are
 * `ad` and `organic`.
 */
export const TRAFFIC_LABEL_KEYS: Record<TikTokTrafficType, string> = {
  ad: 'Paid traffic',
  organic: 'Organic',
}

/** Content type of a creative, as offered by the `Content type` filter. */
export const CONTENT_TYPE_LABEL_KEYS: Record<TikTokContentType, string> = {
  video: 'Video',
  image: 'Photo post',
}

/**
 * Clip-length buckets. Their inclusive second edges live in
 * `lib/tiktok-query.ts`, which turns them into `duration_min`/`duration_max`.
 */
export const DURATION_LABEL_KEYS: Record<TikTokDurationBucket, string> = {
  lt15: '<15s',
  s15to30: '15–30s',
  s30to60: '30–60s',
  gt60: '>60s',
}

export const VIEWS_LABEL_KEYS: Record<TikTokViewsBucket, string> = {
  lt10k: '<10K',
  k10to100k: '10K–100K',
  k100to1m: '100K–1M',
  m1to10m: '1M–10M',
  gt10m: '>10M',
}

/**
 * Publish windows. The corpus grows in collected batches rather than by the
 * hour, so the buckets are cut at half a year and a year instead of the
 * reference site's 7/30/90 days — every one of which would answer with zero
 * rows today.
 */
export const PUBLISHED_LABEL_KEYS: Record<TikTokPublishedBucket, string> = {
  m6: 'Last 6 months',
  y1: 'Last year',
  older: 'Over a year ago',
}

/**
 * Sort dimensions. `views` and `new` are the service's own orderings; revenue
 * and ROAS were demo columns with no field behind them, so they are gone rather
 * than wired to something else.
 */
export const SORT_LABEL_KEYS: Record<TikTokSort, string> = {
  views: 'Views',
  new: 'Latest',
}

/**
 * Country names of the ad library's region filter, as i18n keys (the English
 * country name is the key, matching `src/i18n/static-keys.ts`). The ISO code is
 * always printed next to the name, so an unmapped code still reads.
 *
 * The list is the set of codes the service actually returns, in descending row
 * count (US 895, GB 126, CA 101, DE 57, NG 37, PH 34, FR 32, AU 28, NL 22,
 * IT 21, SE 20, ES 16, BR 13 as measured 2026-09). A country with no rows is
 * not offered: an option that can only answer "no results" is not a filter.
 * The 1,323 rows whose `country` is empty belong to no region — they are
 * reachable through "All", which is the absence of this filter.
 */
export const REGION_LABEL_KEYS: Record<string, string> = {
  US: 'United States',
  GB: 'United Kingdom',
  CA: 'Canada',
  DE: 'Germany',
  NG: 'Nigeria',
  PH: 'Philippines',
  FR: 'France',
  AU: 'Australia',
  NL: 'Netherlands',
  IT: 'Italy',
  SE: 'Sweden',
  ES: 'Spain',
  BR: 'Brazil',
}

/** Region codes in the order the dropdown lists them. */
export const REGION_CODES: string[] = Object.keys(REGION_LABEL_KEYS)

export function regionLabelKey(code: string): string {
  return REGION_LABEL_KEYS[code] ?? code
}

/**
 * DOM id of a model section on `/inspiration`. The anchor bar and the gallery
 * must agree on it, so it lives here rather than in either component.
 */
export function modelSectionAnchorId(modelKey: string): string {
  return `inspiration-model-${modelKey}`
}
