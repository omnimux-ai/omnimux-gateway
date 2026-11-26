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
import type { CSSProperties } from 'react'

export type UserAvatarStyle = Pick<CSSProperties, 'backgroundColor' | 'color'>

function hashString(value: string): number {
  let hash = 0
  for (let i = 0; i < value.length; i++) {
    hash = (hash * 31 + value.charCodeAt(i)) >>> 0
  }
  return hash
}

/**
 * Curated dark-studio avatar swatches.
 * Excludes violet hues (reserved for AI energy flow in DESIGN.md §2.2) and
 * neon greens, while maintaining >= 5.0:1 contrast with white text.
 */
export const USER_AVATAR_PALETTE = [
  '#2563EB',
  '#0F766E',
  '#15803D',
  '#B45309',
  '#C2410C',
  '#BE123C',
  '#404040',
  '#0369A1',
] as const

export function getUserAvatarStyle(name: string): UserAvatarStyle {
  const hash = hashString(name)
  const backgroundColor = USER_AVATAR_PALETTE[hash % USER_AVATAR_PALETTE.length]

  return {
    backgroundColor,
    color: '#FFFFFF',
  }
}

export function getUserAvatarFallback(name: string): string {
  const firstChar = [...name.trim()][0]
  return firstChar ? firstChar.toUpperCase() : '?'
}
