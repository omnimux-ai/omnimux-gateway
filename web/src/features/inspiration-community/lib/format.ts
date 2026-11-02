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
/**
 * Display formatters for the Inspiration Community feature.
 *
 * The TikTok ad library mirrors the reference layout, which prints compact
 * magnitudes (`86.90M`) rather than localized numbers, so these helpers
 * intentionally stay locale-independent. Relative timestamps do use
 * `Intl.RelativeTimeFormat` because they are prose.
 */

/**
 * Printed where the service carries no value for a metric. A dash is not a
 * value: it says "unknown", which is the honest answer, where a `0` would be a
 * claim the payload never made.
 */
export const UNKNOWN_METRIC = '—'

const COMPACT_UNITS = [
  { limit: 1_000_000_000, suffix: 'B' },
  { limit: 1_000_000, suffix: 'M' },
  { limit: 1_000, suffix: 'K' },
] as const

/** `86900000` -> `86.90M`, `4250` -> `4.3K`, `912` -> `912`. */
export function formatCompactNumber(value: number): string {
  if (!Number.isFinite(value)) return '0'
  const sign = value < 0 ? '-' : ''
  const abs = Math.abs(value)

  for (const unit of COMPACT_UNITS) {
    if (abs >= unit.limit) {
      const scaled = abs / unit.limit
      const digits = unit.suffix === 'K' ? 1 : 2
      return `${sign}${scaled.toFixed(digits)}${unit.suffix}`
    }
  }

  return `${sign}${Math.round(abs)}`
}

/**
 * Compact play count, or the placeholder when the payload reported none.
 * A reported `0` stays `0`, because that one is a real count.
 */
export function formatViews(value: number | null): string {
  if (value === null || !Number.isFinite(value)) return UNKNOWN_METRIC
  return formatCompactNumber(value)
}

/**
 * `58` -> `0:58`, `3661` -> `1:01:01`.
 *
 * A null or non-positive length is unknown, not zero: photo posts report `0`,
 * and the card must not print a length it does not have.
 */
export function formatDuration(seconds: number | null): string {
  if (seconds === null || !Number.isFinite(seconds) || seconds <= 0) {
    return UNKNOWN_METRIC
  }
  const total = Math.round(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const secs = total % 60
  const pad = (n: number) => String(n).padStart(2, '0')

  if (hours > 0) return `${hours}:${pad(minutes)}:${pad(secs)}`
  return `${minutes}:${pad(secs)}`
}

/**
 * Thousands separators for result counters, e.g. `10,924`. `locale` is a
 * BCP-47 tag; pass `toIntlLocale(i18n.language)` so grouping follows the
 * interface language instead of the runtime default.
 */
export function formatCount(value: number, locale?: string): string {
  if (!Number.isFinite(value)) return '0'
  return Math.round(value).toLocaleString(locale ?? 'en-US')
}

/**
 * Relative publish time as prose. `locale` is a BCP-47 tag (`en`, `zh-CN`,
 * `ja`, ...) produced by `@/i18n/languages`' `toIntlLocale`.
 */
export function formatRelativeTime(
  iso: string,
  locale = 'en',
  now: Date = new Date()
): string {
  const timestamp = Date.parse(iso)
  if (Number.isNaN(timestamp)) return ''

  const diffSeconds = Math.round((timestamp - now.getTime()) / 1000)
  const abs = Math.abs(diffSeconds)
  const formatter = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })

  if (abs < 60) return formatter.format(Math.trunc(diffSeconds), 'second')
  if (abs < 3600) return formatter.format(Math.trunc(diffSeconds / 60), 'minute')
  if (abs < 86_400) {
    return formatter.format(Math.trunc(diffSeconds / 3600), 'hour')
  }
  if (abs < 2_592_000) {
    return formatter.format(Math.trunc(diffSeconds / 86_400), 'day')
  }
  if (abs < 31_536_000) {
    return formatter.format(Math.trunc(diffSeconds / 2_592_000), 'month')
  }
  return formatter.format(Math.trunc(diffSeconds / 31_536_000), 'year')
}

/** Clamp long text (prompt bodies) to a single gracefully truncated block. */
export function truncateText(text: string, max: number): string {
  const value = text.trim()
  if (value.length <= max) return value
  return `${value.slice(0, max - 1).trimEnd()}…`
}

/**
 * Turn a kebab-case facet key from the crawled data into a readable label:
 * `product-marketing` -> `Product marketing`.
 *
 * Facet values are source content, not UI copy, so they are not translated.
 */
export function humanizeFacet(value: string): string {
  if (!value) return ''
  const words = value.replaceAll(/[_-]+/gu, ' ').trim()
  if (!words) return ''
  return words.charAt(0).toUpperCase() + words.slice(1)
}
