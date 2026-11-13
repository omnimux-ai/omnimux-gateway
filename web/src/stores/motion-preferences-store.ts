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
import { create } from 'zustand'

export const DEFAULT_MOTION_RESOLUTION = '480p'
const KEY = 'influencer:motion-resolution'

function readResolution(): string {
  try {
    const stored = localStorage.getItem(KEY)
    if (stored === '480p' || stored === '720p') return stored
  } catch {
    // Unavailable storage must not prevent generation.
  }
  return DEFAULT_MOTION_RESOLUTION
}

type MotionPreferences = {
  resolution: string
  setResolution: (resolution: string) => void
}

export const useMotionPreferencesStore = create<MotionPreferences>((set) => ({
  resolution: readResolution(),
  setResolution: (resolution) => {
    if (resolution !== '480p' && resolution !== '720p') return
    set({ resolution })
    try {
      localStorage.setItem(KEY, resolution)
    } catch {
      // The selected quality still applies for this page session.
    }
  },
}))
