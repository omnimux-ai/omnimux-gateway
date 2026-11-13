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

import type { WorkbenchMode } from '../types'

const KEY = 'influencer:workbench-mode'

/**
 * The character builder stays the landing surface: it is what the workbench was
 * built around and what a returning visitor comes back for. An unknown or
 * unreadable value falls back to it.
 */
export const DEFAULT_WORKBENCH_MODE: WorkbenchMode = 'image'

export function isWorkbenchMode(value: unknown): value is WorkbenchMode {
  return value === 'image' || value === 'video'
}

export function readWorkbenchMode(): WorkbenchMode {
  try {
    const stored = localStorage.getItem(KEY)
    return isWorkbenchMode(stored) ? stored : DEFAULT_WORKBENCH_MODE
  } catch {
    return DEFAULT_WORKBENCH_MODE
  }
}

export function writeWorkbenchMode(mode: WorkbenchMode): void {
  try {
    localStorage.setItem(KEY, mode)
  } catch {
    /* storage may be unavailable; the choice still applies for this session */
  }
}
