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

import { isVideoCoverUrl } from '../media'

describe('isVideoCoverUrl', () => {
  it('recognizes the video file extensions the crawl publishes', () => {
    expect(isVideoCoverUrl('https://cdn.example.com/clip.mp4')).toBe(true)
    expect(isVideoCoverUrl('https://cdn.example.com/clip.webm')).toBe(true)
    expect(isVideoCoverUrl('https://cdn.example.com/clip.MOV')).toBe(true)
    expect(isVideoCoverUrl('https://cdn.example.com/clip.m4v')).toBe(true)
  })

  it('ignores a query string or fragment on the cover URL', () => {
    expect(
      isVideoCoverUrl('https://cdn.example.com/clip.mp4?token=abc#t=1')
    ).toBe(true)
  })

  it('treats still images and unknown URLs as images', () => {
    expect(isVideoCoverUrl('https://cdn.example.com/poster.jpg')).toBe(false)
    expect(isVideoCoverUrl('https://cdn.example.com/poster.webp')).toBe(false)
    expect(isVideoCoverUrl('https://cdn.example.com/mp4/cover.png')).toBe(false)
    expect(isVideoCoverUrl('')).toBe(false)
  })
})
