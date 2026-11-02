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

import mediaInfoFactory, { type MediaInfo } from 'mediainfo.js'

import {
  BrowserVideoMetadataError,
  createVideoDataSource,
  normalizeVideoMetadata,
  VIDEO_READ_CHUNK_SIZE,
  type BrowserVideoMetadata,
  type BrowserVideoMetadataErrorCode,
} from './browser-video-metadata-reader'

interface WorkerRequest {
  id: number
  source: File | string
}
interface WorkerResult {
  id: number
  metadata?: BrowserVideoMetadata
  code?: BrowserVideoMetadataErrorCode
  message?: string
}

const workerScope = globalThis as unknown as {
  addEventListener(
    type: 'message',
    listener: (event: MessageEvent<WorkerRequest>) => void
  ): void
  postMessage(result: WorkerResult): void
}
let mediaInfo: MediaInfo | undefined

workerScope.addEventListener(
  'message',
  async (event: MessageEvent<WorkerRequest>) => {
    try {
      const source = await createVideoDataSource(event.data.source)
      // The WASM dependency is loaded only when a video is actually selected.
      // The parent serializes work, terminates on abort/timeout, and releases an
      // idle worker so its WASM heap never remains allocated indefinitely.
      mediaInfo ??= await mediaInfoFactory({
        format: 'object',
        coverData: false,
        full: false,
        chunkSize: VIDEO_READ_CHUNK_SIZE,
        locateFile: () =>
          new URL('mediainfo.js/MediaInfoModule.wasm', import.meta.url).href,
      })
      const result = await mediaInfo.analyzeData(source.size, source.readChunk)
      workerScope.postMessage({
        id: event.data.id,
        metadata: normalizeVideoMetadata(result, source.size),
      })
    } catch (error) {
      // Never pass parser internals, URLs or filesystem-like strings to the UI.
      workerScope.postMessage({
        id: event.data.id,
        code:
          error instanceof BrowserVideoMetadataError
            ? error.code
            : 'unreadable',
        message:
          error instanceof BrowserVideoMetadataError
            ? error.message
            : undefined,
      })
    }
  }
)
