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

/**
 * Resolves the installer this visit needs. Phones and tablets get `null`
 * because a desktop installer is the wrong answer there, and an unknown
 * desktop still gets a guess only when the UA actually says mac or win.
 */
export function detectPlatform(): DetectedPlatform {
  if (typeof navigator === 'undefined') return null

  const ua = navigator.userAgent || ''
  const nav = navigator as Navigator & {
    userAgentData?: { platform?: string }
    maxTouchPoints?: number
  }
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
 * Resolves which Mac build this visit needs. `arm` wins because every Mac
 * sold since late 2020 is Apple silicon, and an Intel user who downloads the
 * arm64 build simply cannot open it — so `null` (unsupported API, rejected
 * promise, or a surprising value) still falls back to the safe default.
 */
export async function detectMacArch(): Promise<MacArch | null> {
  if (detectPlatform() !== 'mac') return null

  const nav = navigator as Navigator & {
    userAgentData?: {
      getHighEntropyValues?: (
        hints: string[]
      ) => Promise<{ architecture?: string }>
    }
  }
  try {
    const values = await nav.userAgentData?.getHighEntropyValues?.([
      'architecture',
    ])
    if (values?.architecture === 'x86') return 'x64'
  } catch {
    // High-entropy hints can reject; arm64 stays the default below.
  }
  return 'arm64'
}
