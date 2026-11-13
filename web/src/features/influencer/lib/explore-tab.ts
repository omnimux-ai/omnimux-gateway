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

import type { InfluencerExploreTab } from '../types'

const KEY = 'influencer:explore-tab'

const TABS = new Set<InfluencerExploreTab>(['influencers', 'presets', 'trends'])

/**
 * The character shelf is the default: it is what the first visit needs most,
 * and the video shelves make sense once a character exists. An unknown or
 * unreadable value falls back to it.
 */
export function readExploreTab(): InfluencerExploreTab {
  try {
    const raw = localStorage.getItem(KEY) as InfluencerExploreTab | null
    return raw && TABS.has(raw) ? raw : 'influencers'
  } catch {
    return 'influencers'
  }
}

export function writeExploreTab(tab: InfluencerExploreTab): void {
  try {
    localStorage.setItem(KEY, tab)
  } catch {
    /* storage may be unavailable; the choice still applies for this session */
  }
}
