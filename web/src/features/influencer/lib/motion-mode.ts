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

import type { MotionMode } from '../types'
import { DEFAULT_MOTION_MODE, isMotionMode } from './motion'

const KEY = 'influencer:motion-mode'

/**
 * Motion transfer is the mode a visitor is most likely to have seen elsewhere,
 * so it leads. An unknown or unreadable value falls back to it.
 */
export function readMotionMode(): MotionMode {
  try {
    const stored = localStorage.getItem(KEY)
    return isMotionMode(stored) ? stored : DEFAULT_MOTION_MODE
  } catch {
    return DEFAULT_MOTION_MODE
  }
}

export function writeMotionMode(mode: MotionMode): void {
  try {
    localStorage.setItem(KEY, mode)
  } catch {
    /* storage may be unavailable; the choice still applies for this session */
  }
}
