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
import { useEffect, useRef, useState } from 'react'

import {
  getUploadThumbnails,
  subscribeUploadThumbnails,
} from '../lib/media-thumbnails'

/**
 * Local derivatives for the upload library, keyed by the remote URL. The map
 * stays empty when IndexedDB is unavailable, so every card falls back to its
 * original URL instead of blocking the panel.
 */
export function useUploadThumbnails(
  ownerId: number | undefined,
  urls: readonly string[]
): Map<string, string> {
  const [thumbnails, setThumbnails] = useState<Map<string, string>>(
    () => new Map()
  )
  const [revision, setRevision] = useState(0)
  const urlsRef = useRef(urls)
  urlsRef.current = urls
  const signature = urls.join('\n')

  useEffect(
    () => subscribeUploadThumbnails(() => setRevision((value) => value + 1)),
    []
  )

  useEffect(() => {
    if (typeof ownerId !== 'number' || urlsRef.current.length === 0) {
      setThumbnails((current) => (current.size ? new Map() : current))
      return
    }
    let cancelled = false
    const created: string[] = []
    const load = async () => {
      try {
        const blobs = await getUploadThumbnails(ownerId, urlsRef.current)
        const next = new Map<string, string>()
        blobs.forEach((blob, url) => {
          const objectUrl = URL.createObjectURL(blob)
          created.push(objectUrl)
          next.set(url, objectUrl)
        })
        if (cancelled) {
          created.forEach((url) => URL.revokeObjectURL(url))
          return
        }
        setThumbnails(next)
      } catch {
        /* Original URLs remain the fallback. */
      }
    }
    void load()
    return () => {
      cancelled = true
      created.forEach((url) => URL.revokeObjectURL(url))
    }
  }, [ownerId, signature, revision])

  return thumbnails
}
