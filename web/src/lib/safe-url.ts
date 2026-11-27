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
const SAFE_LINK_PROTOCOLS = new Set(['http:', 'https:', 'mailto:'])
const SAFE_EXTERNAL_HTTP_PROTOCOLS = new Set(['http:', 'https:'])

export function hasControlCharacters(value: string): boolean {
  for (let i = 0; i < value.length; i += 1) {
    const code = value.charCodeAt(i)
    if (code <= 0x1f || code === 0x7f) {
      return true
    }
  }
  return false
}

/**
 * Validate a user-clickable href (Markdown links, AI citations).
 * Allows http:, https:, mailto:, in-page fragments (#...), and single-slash
 * relative paths without backslashes or ASCII control characters.
 */
export function sanitizeSafeHref(value: unknown): string | null {
  if (typeof value !== 'string') {
    return null
  }

  const trimmed = value.trim()
  if (!trimmed || hasControlCharacters(trimmed)) {
    return null
  }

  if (trimmed.startsWith('#')) {
    return trimmed
  }

  if (trimmed.startsWith('/')) {
    if (trimmed.startsWith('//') || trimmed.includes('\\')) {
      return null
    }
    return trimmed
  }

  try {
    const parsed = new URL(trimmed)
    return SAFE_LINK_PROTOCOLS.has(parsed.protocol) ? trimmed : null
  } catch {
    return null
  }
}

/**
 * Validate an absolute external http(s) URL (payment checkout redirects,
 * external form actions, popup windows).
 */
export function sanitizeExternalHttpUrl(value: unknown): string | null {
  if (typeof value !== 'string') {
    return null
  }

  const trimmed = value.trim()
  if (!trimmed || hasControlCharacters(trimmed)) {
    return null
  }

  try {
    const parsed = new URL(trimmed)
    return SAFE_EXTERNAL_HTTP_PROTOCOLS.has(parsed.protocol) ? trimmed : null
  } catch {
    return null
  }
}

export function isSafeExternalHttpUrl(value: unknown): value is string {
  return sanitizeExternalHttpUrl(value) !== null
}

/**
 * Open a validated external http(s) URL in a new browsing context with
 * opener isolation. Returns false when the URL is rejected.
 */
export function openSafeExternalUrl(value: unknown): boolean {
  const safeUrl = sanitizeExternalHttpUrl(value)
  if (!safeUrl) {
    return false
  }

  window.open(safeUrl, '_blank', 'noopener,noreferrer')
  return true
}
