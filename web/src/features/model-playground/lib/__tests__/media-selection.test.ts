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
import { describe, expect, test } from 'vitest'

import { acceptTypes, screenMediaSelection } from '../media-validation'

const IMAGES = ['image/png', 'image/jpeg', 'image/webp']

function picked(name: string, type: string): File {
  return new File(['x'], name, { type })
}

describe('acceptTypes', () => {
  test('splits an accept attribute into the MIME types it lists', () => {
    expect(acceptTypes('image/png,image/jpeg')).toEqual([
      'image/png',
      'image/jpeg',
    ])
  })

  test('treats a missing accept attribute as no declared format', () => {
    expect(acceptTypes(undefined)).toEqual([])
    expect(acceptTypes('')).toEqual([])
  })
})

describe('screenMediaSelection', () => {
  test('keeps the picked order when the selection is inside every limit', () => {
    const files = [
      picked('a.png', 'image/png'),
      picked('b.jpg', 'image/jpeg'),
      picked('c.webp', 'image/webp'),
    ]

    const result = screenMediaSelection({ accept: IMAGES, maxCount: 3 }, 0, files)

    expect(result.issue).toBeNull()
    expect(result.accepted.map((entry) => entry.name)).toEqual([
      'a.png',
      'b.jpg',
      'c.webp',
    ])
  })

  test('reports the ceiling and drops the overflow when the selection is too large', () => {
    const files = [
      picked('a.png', 'image/png'),
      picked('b.png', 'image/png'),
      picked('c.png', 'image/png'),
    ]

    const result = screenMediaSelection({ accept: IMAGES, maxCount: 1 }, 0, files)

    expect(result.accepted.map((entry) => entry.name)).toEqual(['a.png'])
    expect(result.issue).toEqual({ code: 'too_many', limit: 1 })
  })

  test('counts the files already held against the ceiling', () => {
    const result = screenMediaSelection({ accept: IMAGES, maxCount: 2 }, 2, [
      picked('a.png', 'image/png'),
    ])

    expect(result.accepted).toEqual([])
    expect(result.issue).toEqual({ code: 'too_many', limit: 2 })
  })

  test('reports the format and drops the file when the model does not take it', () => {
    const result = screenMediaSelection({ accept: IMAGES, maxCount: 4 }, 0, [
      picked('a.png', 'image/png'),
      picked('b.gif', 'image/gif'),
    ])

    expect(result.accepted.map((entry) => entry.name)).toEqual(['a.png'])
    expect(result.issue).toEqual({
      code: 'unsupported_type',
      mimeType: 'image/gif',
    })
  })

  test('accepts every file when the model declares no ceiling', () => {
    const files = [picked('a.png', 'image/png'), picked('b.png', 'image/png')]

    const result = screenMediaSelection({ accept: IMAGES }, 5, files)

    expect(result.accepted).toHaveLength(2)
    expect(result.issue).toBeNull()
  })

  test('takes any format when the slot declares none', () => {
    const result = screenMediaSelection({ accept: [], maxCount: 2 }, 0, [
      picked('a.bin', 'application/octet-stream'),
    ])

    expect(result.accepted).toHaveLength(1)
    expect(result.issue).toBeNull()
  })

  test('reports the ceiling and drops the file when it exceeds the documented size', () => {
    const limit = 1024
    const big = new File([new Uint8Array(limit + 1)], 'big.png', {
      type: 'image/png',
    })

    const result = screenMediaSelection(
      { accept: IMAGES, maxCount: 4, maxBytes: limit },
      0,
      [big]
    )

    expect(result.accepted).toEqual([])
    expect(result.issue).toEqual({ code: 'too_large', limitBytes: limit })
  })

  test('accepts a file exactly at the documented size', () => {
    const limit = 1024
    const exact = new File([new Uint8Array(limit)], 'exact.png', {
      type: 'image/png',
    })

    const result = screenMediaSelection(
      { accept: IMAGES, maxCount: 4, maxBytes: limit },
      0,
      [exact]
    )

    expect(result.accepted).toHaveLength(1)
    expect(result.issue).toBeNull()
  })

  test('ignores size when the model documents no ceiling', () => {
    const big = new File([new Uint8Array(2048)], 'big.png', {
      type: 'image/png',
    })

    const result = screenMediaSelection({ accept: IMAGES, maxCount: 4 }, 0, [
      big,
    ])

    expect(result.accepted).toHaveLength(1)
    expect(result.issue).toBeNull()
  })
})
