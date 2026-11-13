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
import { useCallback, useMemo, useSyncExternalStore } from 'react'

import { useAuthStore } from '@/stores/auth-store'

import { uploadReferenceAsset } from '../api'
import {
  captureVideoPoster,
  generateImageThumbnail,
  putUploadThumbnail,
} from '../lib/media-thumbnails'
import {
  getUploads,
  rememberUpload,
  subscribeUploads,
  type UploadKind,
  type UploadRecord,
} from '../lib/upload-library'
import type { MotionVideoMetadata } from '../types'

const NONE: UploadRecord[] = []

/**
 * The single upload path for every reference entry on the page. A successful
 * upload joins the signed-in account's library before its URL is returned, so
 * the record survives a refresh whether or not the caller still selects it.
 */
export function useUploadLibrary() {
  const ownerId = useAuthStore((state) => state.auth.user?.id)
  const records = useSyncExternalStore(
    subscribeUploads,
    () => getUploads(ownerId),
    () => NONE
  )
  const images = useMemo(
    () => records.filter((record) => record.kind === 'image'),
    [records]
  )
  const videos = useMemo(
    () => records.filter((record) => record.kind === 'video'),
    [records]
  )
  /** Throws when the upload fails; nothing is recorded in that case. */
  const upload = useCallback(
    async (
      file: File,
      kind: UploadKind,
      metadata?: MotionVideoMetadata,
      onProgress?: (loaded: number, total: number) => void,
      signal?: AbortSignal
    ): Promise<string> => {
      if (signal?.aborted) throw new DOMException('Cancelled', 'AbortError')
      // Derivative generation starts beside the upload, never before it.
      const derivative =
        kind === 'image'
          ? generateImageThumbnail(file).catch(() => null)
          : Promise.resolve(null)
      const stored = await uploadReferenceAsset(
        file,
        signal,
        undefined,
        onProgress
      )
      // A cancelled batch must not leave a record behind in the library.
      if (signal?.aborted) throw new DOMException('Cancelled', 'AbortError')
      rememberUpload(ownerId, {
        kind,
        url: stored.url,
        fileName: file.name,
        uploadedAt: Date.now(),
        expiresAt: stored.expiresAt,
        ...(metadata ? { metadata } : {}),
      })
      void derivative.then((local) => {
        if (local) {
          void putUploadThumbnail(ownerId, stored.url, local)
        }
      })
      return stored.url
    },
    [ownerId]
  )
  /** Complete local checks precede the first upload request. */
  const uploadVideo = useCallback(
    async (
      file: File,
      metadata: MotionVideoMetadata,
      signal?: AbortSignal,
      onProgress?: (loaded: number, total: number) => void
    ) => {
      if (signal?.aborted) throw new DOMException('Cancelled', 'AbortError')
      if (
        metadata.version !== 1 ||
        !metadata.browserMetadata ||
        metadata.browserMetadata.size_bytes !== file.size
      ) {
        throw new Error('Could not prepare video')
      }
      const poster = captureVideoPoster(file).catch(() => null)
      const stored = await uploadReferenceAsset(
        file,
        signal,
        {
          purpose: 'influencer_reference_video',
          videoMetadata: metadata.browserMetadata,
        },
        onProgress
      )
      if (signal?.aborted) throw new DOMException('Cancelled', 'AbortError')
      rememberUpload(ownerId, {
        kind: 'video',
        url: stored.url,
        fileName: file.name,
        uploadedAt: Date.now(),
        expiresAt: stored.expiresAt,
        metadata,
      })
      void poster.then((local) => {
        if (local) {
          void putUploadThumbnail(ownerId, stored.url, local)
        }
      })
      return { url: stored.url, metadata }
    },
    [ownerId]
  )
  const rememberVideoMetadata = useCallback(
    (url: string, metadata: MotionVideoMetadata) => {
      const record = getUploads(ownerId).find(
        (item) => item.kind === 'video' && item.url === url
      )
      if (record && record.metadata?.version !== 1) {
        rememberUpload(ownerId, { ...record, metadata })
      }
    },
    [ownerId]
  )
  return { images, videos, upload, uploadVideo, rememberVideoMetadata }
}
