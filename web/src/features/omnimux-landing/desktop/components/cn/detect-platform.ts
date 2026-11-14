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
import type { DetectedPlatform, MacArch } from './download-context'

type NavigatorWithUaData = Navigator & {
  userAgentData?: {
    platform?: string
    getHighEntropyValues?: (
      hints: string[]
    ) => Promise<{ architecture?: string }>
  }
  maxTouchPoints?: number
}

/**
 * The operating system behind this visit, ignoring chip. Phones and tablets get
 * `null` because a desktop installer is the wrong answer there.
 */
export function detectOs(): 'mac' | 'win' | null {
  if (typeof navigator === 'undefined') return null

  const ua = navigator.userAgent || ''
  const nav = navigator as NavigatorWithUaData
  const platform = (
    nav.userAgentData?.platform ||
    navigator.platform ||
    ''
  ).toLowerCase()

  // iPadOS reports a desktop Mac UA, so touch support is the tell.
  const isIpadOs = (nav.maxTouchPoints ?? 0) > 1 && /mac/.test(platform)
  if (isIpadOs || /iphone|ipad|ipod|android|mobile/i.test(ua)) return null

  if (/win/.test(platform) || /windows/i.test(ua)) return 'win'
  if (/mac/.test(platform) || /mac os x/i.test(ua)) return 'mac'
  return null
}

/**
 * Resolves which Mac build this visit needs. `arm` wins because every Mac sold
 * since late 2020 is Apple silicon, and an Intel user who downloads the arm64
 * build simply cannot open it — so `null` (unsupported API, rejected promise,
 * or a surprising value) still falls back to the safe default.
 */
export async function detectMacArch(): Promise<MacArch | null> {
  if (detectOs() !== 'mac') return null

  const nav = navigator as NavigatorWithUaData
  try {
    const values = await nav.userAgentData?.getHighEntropyValues?.([
      'architecture',
    ])
    const arch = (values?.architecture || '').toLowerCase()
    if (arch === 'x86' || arch === 'x86_64') return 'x64'
    if (arch === 'arm' || arch === 'arm64') return 'arm64'
  } catch {
    // High-entropy hints can reject; the fallback below decides.
  }

  // No usable high-entropy hint. `MacIntel` (and the frozen "Intel Mac OS X"
  // UA token) is reported by Apple-silicon browsers too, so it cannot mean an
  // Intel chip; only an explicit, non-frozen Intel marker is trusted. Anything
  // else defaults to arm64, the common case since 2020.
  const rawPlatform = (navigator.platform || '').toLowerCase()
  const rawUa = (navigator.userAgent || '').toLowerCase()
  if (/arm|aarch64/.test(rawPlatform) || /arm|aarch64/.test(rawUa)) {
    return 'arm64'
  }
  if (/intel/.test(rawPlatform) && rawPlatform !== 'macintel') return 'x64'
  if (/intel/.test(rawUa) && !/mac os x/.test(rawUa)) return 'x64'
  return 'arm64'
}

/**
 * Resolves the exact installer this visit needs, including the Mac chip:
 * `mac-arm64`, `mac-x64`, `win`, or `null` when we cannot safely guess.
 */
export async function detectPlatform(): Promise<DetectedPlatform> {
  const os = detectOs()
  if (os === 'win') return 'win'
  if (os === 'mac') {
    const arch = await detectMacArch()
    return arch === 'x64' ? 'mac-x64' : 'mac-arm64'
  }
  return null
}
