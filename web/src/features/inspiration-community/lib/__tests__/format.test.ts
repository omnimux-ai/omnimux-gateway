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
import { describe, expect, it } from 'vitest'

import {
  UNKNOWN_METRIC,
  formatCompactNumber,
  formatCount,
  formatDuration,
  formatViews,
  humanizeFacet,
  truncateText,
} from '../format'

describe('formatCompactNumber', () => {
  it('formats millions with two decimals', () => {
    expect(formatCompactNumber(86_900_000)).toBe('86.90M')
  })

  it('formats thousands with one decimal', () => {
    expect(formatCompactNumber(4250)).toBe('4.3K')
  })

  it('keeps sub-thousand values as rounded integers', () => {
    expect(formatCompactNumber(912.4)).toBe('912')
  })
})

describe('formatViews', () => {
  it('formats a reported play count compactly', () => {
    expect(formatViews(95_600_000)).toBe('95.60M')
  })

  it('prints a placeholder when the payload reported no count', () => {
    expect(formatViews(null)).toBe(UNKNOWN_METRIC)
  })

  it('keeps a reported zero as zero', () => {
    expect(formatViews(0)).toBe('0')
  })
})

describe('formatDuration', () => {
  it('formats sub-minute clips as m:ss', () => {
    expect(formatDuration(58)).toBe('0:58')
  })

  it('formats clips over an hour as h:mm:ss', () => {
    expect(formatDuration(3661)).toBe('1:01:01')
  })

  it('prints a placeholder when the length is unknown', () => {
    // The service reports 0 for photo posts, and that 0 is not a length.
    expect(formatDuration(0)).toBe(UNKNOWN_METRIC)
    expect(formatDuration(null)).toBe(UNKNOWN_METRIC)
  })
})

describe('formatCount', () => {
  it('adds thousands separators', () => {
    expect(formatCount(10_924, 'en-US')).toBe('10,924')
  })
})

describe('truncateText', () => {
  it('returns short text unchanged', () => {
    expect(truncateText('short prompt', 400)).toBe('short prompt')
  })

  it('appends an ellipsis when the limit is exceeded', () => {
    const result = truncateText('a'.repeat(500), 400)
    expect(result).toHaveLength(400)
    expect(result.endsWith('…')).toBe(true)
  })
})

describe('humanizeFacet', () => {
  it('turns a kebab-case facet into a sentence-case label', () => {
    expect(humanizeFacet('product-marketing')).toBe('Product marketing')
  })
})
