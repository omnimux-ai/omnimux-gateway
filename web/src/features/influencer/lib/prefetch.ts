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

const prefetchedUrls = new Set<string>()

/**
 * Prefetches an image URL into the browser HTTP cache upon hover or keyboard focus,
 * so opening the full-screen modal displays the sheet almost instantly.
 * Deduplicated per session via an in-memory Set.
 */
export function prefetchImage(url: string | undefined | null): boolean {
  if (!url || typeof window === 'undefined') return false
  const trimmed = url.trim()
  if (!trimmed || prefetchedUrls.has(trimmed)) return false
  prefetchedUrls.add(trimmed)

  try {
    const img = new Image()
    img.decoding = 'async'
    img.src = trimmed
    return true
  } catch {
    return false
  }
}

/** Reset prefetched cache set (useful in tests). */
export function clearPrefetchCache(): void {
  prefetchedUrls.clear()
}

/** Check if a URL has already been prefetched (useful in tests). */
export function isPrefetched(url: string): boolean {
  return prefetchedUrls.has(url.trim())
}
