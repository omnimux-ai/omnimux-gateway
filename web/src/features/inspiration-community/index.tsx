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
 * Inspiration Community — three galleries (PRD
 * `docs/product/inspiration-community-prd-zh.md`).
 *
 * `/inspiration`                      L1: prompt previews, grouped by model
 * `/inspiration/models/$modelSlug`    L2: one model's full prompt library
 * `/inspiration/tiktok`               TikTok Ad Library, seven-dimension filtering
 *
 * The prompt pages read the statically served catalog; the ad library reads the
 * inspiration service's public feed (`api/tiktok.ts`). Neither renders a bundled
 * fixture.
 */
export {
  PromptGallery,
  PromptGallerySkeleton,
} from './components/prompt-gallery'
export {
  ModelPromptsPage,
  ModelPromptsSkeleton,
} from './components/model-prompts-page'
export type { ModelPromptsPageProps } from './components/model-prompts-page'
export {
  TikTokAdGallery,
  TikTokAdGallerySkeleton,
} from './components/tiktok-ad-gallery'
export { InspirationShell } from './components/inspiration-shell'
export type { InspirationSectionKey } from './components/inspiration-shell'
export type {
  PromptItem,
  PromptMediaType,
  PromptSection,
  PromptSort,
  TikTokAdCreator,
  TikTokAdItem,
  TikTokContentType,
  TikTokDurationBucket,
  TikTokFilters,
  TikTokMediaType,
  TikTokPublishedBucket,
  TikTokSort,
  TikTokTrafficType,
  TikTokViewsBucket,
} from './types'
export type { PromptFilterState } from './lib/filter'
