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
import { DOWNLOAD_MAC_ARM64, DOWNLOAD_MAC_X64, DOWNLOAD_WIN } from './data'

/**
 * Where the desktop builds and their stable-version manifest live. Kept as one
 * origin so a mirror or a test double can swap it in a single place.
 */
export const DESKTOP_RELEASE_ORIGIN = 'https://dl.omnimux.ai'

/**
 * The stable channel manifest. Its `version` field is the exact release the
 * download buttons must point at; the manifest is the source of truth so a
 * stale or broken CDN file surfaces as a 404 instead of a silent old download.
 */
export const DESKTOP_RELEASE_META_URL = `${DESKTOP_RELEASE_ORIGIN}/api/desktop/version-stable.json`

export type ReleaseUrls = {
  arm64: string
  x64: string
  win: string
}

/**
 * Resolved state of the desktop release.
 *
 * - `loading`  — the manifest fetch has not settled yet; callers may use
 *   `LATEST_URLS` so the first paint (SSR / static export) is usable.
 * - `ready`    — a stable `version` was read; URLs are version-pinned.
 * - `fallback` — the manifest could not be read (network / parse / 5xx); URLs
 *   fall back to the `releases/latest/` symlinks, and a warning was logged.
 * - `unavailable` — the manifest answered 404 or carried no `version`; there is
 *   no safe installer to offer, so the download UI must be hidden/disabled.
 */
export type DesktopRelease =
  | { status: 'loading' }
  | { status: 'ready'; version: string; urls: ReleaseUrls }
  | { status: 'fallback'; urls: ReleaseUrls }
  | { status: 'unavailable' }

/**
 * The legacy `releases/latest/` symlinks. Older shipped apps still check these
 * for updates, so their meaning must not change; we only reuse them as the
 * network-error fallback here.
 */
export const LATEST_URLS: ReleaseUrls = {
  arm64: DOWNLOAD_MAC_ARM64,
  x64: DOWNLOAD_MAC_X64,
  win: DOWNLOAD_WIN,
}

function versionedUrls(version: string): ReleaseUrls {
  const base = `${DESKTOP_RELEASE_ORIGIN}/releases/v${version}`
  return {
    arm64: `${base}/OmniMux-${version}-arm64.dmg`,
    x64: `${base}/OmniMux-${version}-x64.dmg`,
    win: `${base}/OmniMux-${version}-x64-Setup.exe`,
  }
}

/**
 * Reads the stable-channel manifest and resolves the download URLs.
 *
 * A 404 (or a payload without a string `version`) is a hard "no stable build":
 * we return `unavailable` so the UI hides the download instead of offering a
 * dead or stale link. Any other failure — offline, DNS, a 5xx, malformed JSON —
 * is treated as transient and falls back to the `releases/latest/` symlinks.
 */
export async function fetchDesktopRelease(
  fetchImpl?: typeof fetch
): Promise<DesktopRelease> {
  const doFetch = fetchImpl ?? (typeof fetch === 'function' ? fetch : null)
  if (!doFetch) {
    // No fetch in this runtime (SSR / static export / older test env): treat it
    // like an outage and keep the latest symlinks usable.
    return { status: 'fallback', urls: LATEST_URLS }
  }
  try {
    const response = await doFetch(DESKTOP_RELEASE_META_URL, {
      headers: { Accept: 'application/json' },
    })
    if (response.status === 404) return { status: 'unavailable' }
    if (!response.ok) {
      throw new Error(`version-stable.json responded ${response.status}`)
    }
    const meta: unknown = await response.json()
    const version =
      typeof meta === 'object' && meta !== null && 'version' in meta
        ? String((meta as { version?: unknown }).version ?? '').trim()
        : ''
    if (!version) return { status: 'unavailable' }
    return { status: 'ready', version, urls: versionedUrls(version) }
  } catch (error) {
    console.warn(
      '[omnimux] desktop version-stable.json unavailable, using releases/latest:',
      error
    )
    return { status: 'fallback', urls: LATEST_URLS }
  }
}

/**
 * The URLs a caller should actually use. `loading` and `fallback` both point at
 * the latest symlinks so the first paint and transient outages stay usable;
 * `unavailable` returns `null` so callers disable the control.
 */
export function releaseUrls(release: DesktopRelease): ReleaseUrls | null {
  if (release.status === 'unavailable') return null
  if (release.status === 'ready') return release.urls
  return LATEST_URLS
}
