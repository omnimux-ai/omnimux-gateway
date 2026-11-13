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

import type { InfluencerSource } from '../types'

const KEY = 'influencer:source'

/**
 * Explore is the default: a first-time visitor has no history worth showing,
 * and the preset library is what makes the workbench usable without typing.
 * An unknown or unreadable value falls back to it.
 */
export function readSource(): InfluencerSource {
  try {
    return localStorage.getItem(KEY) === 'history' ? 'history' : 'explore'
  } catch {
    return 'explore'
  }
}

export function writeSource(source: InfluencerSource): void {
  try {
    localStorage.setItem(KEY, source)
  } catch {
    /* storage may be unavailable; the choice still applies for this session */
  }
}
