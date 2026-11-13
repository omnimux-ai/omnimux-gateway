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

import snapshot from '../data/motion-explore.json'
import type {
  InfluencerExploreTab,
  MotionExplorePreset,
  MotionMode,
  MotionPreset,
} from '../types'

interface MotionExploreSnapshot {
  version: number
  source: string
  fetched_at: string
  hero: string[]
  presets: MotionExplorePreset[]
  community: MotionExplorePreset[]
}

const SNAPSHOT = snapshot as MotionExploreSnapshot

/** Official motion presets — the "Motion Presets" tab of the library. */
export function motionExplorePresets(): MotionExplorePreset[] {
  return SNAPSHOT.presets
}

/** Community-trend clips — the "Community Trends" tab of the library. */
export function communityTrendPresets(): MotionExplorePreset[] {
  return SNAPSHOT.community
}

/** The five clips fanned out in the hero, taken from the preset list. */
export function heroExplorePresets(): MotionExplorePreset[] {
  const byId = new Map(SNAPSHOT.presets.map((p) => [p.id, p]))
  return SNAPSHOT.hero
    .map((id) => byId.get(id))
    .filter((p): p is MotionExplorePreset => Boolean(p))
}

/** A mirrored preset rendered as one of the sidebar's built-in templates. */
export function exploreToMotionPreset(item: MotionExplorePreset): MotionPreset {
  return {
    id: item.id,
    nameKey: item.title,
    mode: item.mode,
    description: item.description,
    prompt: item.prompt ?? item.description ?? item.title,
    thumbUrl: item.thumbPath,
    videoUrl: item.videoUrl,
    referenceVideoUrl: item.referenceVideoUrl,
    durationSec: item.durationSec,
  }
}

/** The sidebar template pack, mirrored from the official preset shelf. */
export function motionTemplatePresets(): MotionPreset[] {
  return SNAPSHOT.presets.map(exploreToMotionPreset)
}

export function motionExploreModeLabel(mode: MotionMode): string {
  return mode === 'replace' ? 'Object swap' : 'Motion transfer'
}

/** Which library shelf a tab shows; `influencers` is the character grid. */
export function exploreTabIsVideo(tab: InfluencerExploreTab): boolean {
  return tab !== 'influencers'
}

export const MOTION_EXPLORE_SNAPSHOT = {
  version: SNAPSHOT.version,
  source: SNAPSHOT.source,
  fetchedAt: SNAPSHOT.fetched_at,
  presets: SNAPSHOT.presets.length,
  community: SNAPSHOT.community.length,
} as const
