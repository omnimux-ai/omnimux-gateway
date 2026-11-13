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
  generateImageThumbnail,
  getUploadThumbnails,
  putUploadThumbnail,
} from '../lib/media-thumbnails'

const thumbnail = {
  blob: new Blob(['thumb'], { type: 'image/webp' }),
  width: 96,
  height: 96,
}

describe('local media thumbnails', () => {
  it('resolves without thumbnails when IndexedDB is unavailable', async () => {
    await expect(
      getUploadThumbnails(7, ['https://files.example/a.png'])
    ).resolves.toEqual(new Map())
  })

  it('treats a missing IndexedDB as a non-blocking write failure', async () => {
    await expect(
      putUploadThumbnail(7, 'https://files.example/a.png', thumbnail)
    ).resolves.toBeUndefined()
  })

  it('does not invent a derivative when the browser cannot decode the image', async () => {
    await expect(
      generateImageThumbnail(
        new File(['not an image'], 'a.png', { type: 'image/png' })
      )
    ).resolves.toBeNull()
  })
})
