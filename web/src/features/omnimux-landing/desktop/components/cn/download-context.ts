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
import { createContext, useContext } from 'react'

import type { DesktopRelease } from './desktop-release'

/**
 * The three installers a visit can resolve to. Apple silicon and Intel Macs
 * get distinct builds, so the target itself carries the chip.
 */
export type DownloadTarget = 'mac-arm64' | 'mac-x64' | 'win'

export type MacArch = 'arm64' | 'x64'

/**
 * `null` means the visitor is on a platform we cannot pick an installer for
 * (mobile, unknown, or a UA we do not trust). Callers fall back to the dialog
 * instead of guessing.
 */
export type DetectedPlatform = DownloadTarget | null

export const DownloadContext = createContext<{
  /** Opens the dialog with the given platform pre-selected. */
  open: (target?: DownloadTarget) => void
  /**
   * The platform this visit most likely needs, resolved once on mount.
   * Single download buttons use it to fetch the right installer directly.
   */
  platform: DetectedPlatform
  /**
   * The Mac chip this visit most likely needs, derived from `platform`.
   * `null` on Windows, unknown platforms, and before detection settles;
   * callers fall back to the Apple-silicon build, which is the common case.
   */
  macArch: MacArch | null
  /**
   * Stable-channel release state. `loading` / `fallback` resolve to the
   * `releases/latest/` symlinks; `ready` pins the exact version; `unavailable`
   * means the download controls must be disabled.
   */
  release: DesktopRelease
  /** Starts the installer download for `platform`; false when unknown. */
  downloadDirect: () => boolean
}>({
  open: () => {},
  platform: null,
  macArch: null,
  release: { status: 'loading' },
  downloadDirect: () => false,
})

export function useDownload() {
  return useContext(DownloadContext)
}
