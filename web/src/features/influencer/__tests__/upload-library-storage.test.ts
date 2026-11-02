import { beforeEach, describe, expect, it } from 'vitest'

import {
  UPLOAD_EXPIRY_MARGIN_MS,
  UPLOAD_LIBRARY_LIMIT,
  getUploads,
  readUploadLibrary,
  rememberUpload,
  resetUploadCache,
  type UploadRecord,
} from '../lib/upload-library'

const NOW = 1_800_000_000_000
const HOUR = 60 * 60 * 1000
const KEY = 'influencer:upload-library:v1:7'

function image(url: string, uploadedAt = NOW, expiresAt = NOW + 72 * HOUR) {
  return {
    kind: 'image',
    url,
    fileName: url.split('/').pop() ?? url,
    uploadedAt,
    expiresAt,
  } satisfies UploadRecord
}

beforeEach(() => {
  localStorage.clear()
  resetUploadCache()
})

describe('upload library storage', () => {
  it('lists the newest upload first and replaces a repeated URL', () => {
    rememberUpload(7, image('https://f.example/a.png', NOW - 2), NOW)
    rememberUpload(7, image('https://f.example/b.png', NOW - 1), NOW)
    rememberUpload(7, image('https://f.example/a.png', NOW), NOW)

    expect(readUploadLibrary(7, NOW).map((r) => r.url)).toEqual([
      'https://f.example/a.png',
      'https://f.example/b.png',
    ])
  })

  it('drops a record before the gateway deletes its file', () => {
    rememberUpload(7, image('https://f.example/old.png', NOW, NOW + HOUR), NOW)

    expect(readUploadLibrary(7, NOW + HOUR - UPLOAD_EXPIRY_MARGIN_MS)).toEqual(
      []
    )
  })

  it('removes an expired record from the visible list without a new upload', () => {
    rememberUpload(7, image('https://f.example/old.png', NOW, NOW + HOUR), NOW)
    expect(getUploads(7, NOW)).toHaveLength(1)

    expect(getUploads(7, NOW + HOUR)).toEqual([])
  })

  it('keeps at most the configured number of records per kind', () => {
    for (let i = 0; i <= UPLOAD_LIBRARY_LIMIT; i += 1) {
      rememberUpload(7, image(`https://f.example/${i}.png`, NOW + i), NOW)
    }

    const urls = readUploadLibrary(7, NOW).map((r) => r.url)
    expect(urls).toHaveLength(UPLOAD_LIBRARY_LIMIT)
    expect(urls).not.toContain('https://f.example/0.png')
  })

  it('keeps valid records when one stored record is malformed', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({
        v: 1,
        items: [{ kind: 'video', url: 'x' }, image('https://f.example/ok.png')],
      })
    )

    expect(readUploadLibrary(7, NOW).map((r) => r.url)).toEqual([
      'https://f.example/ok.png',
    ])
  })

  it('retains a legacy video for a fresh browser read instead of hiding it', () => {
    localStorage.setItem(
      KEY,
      JSON.stringify({
        v: 1,
        items: [{ ...image('https://f.example/clip.mp4'), kind: 'video' }],
      })
    )

    expect(readUploadLibrary(7, NOW)).toEqual([
      { ...image('https://f.example/clip.mp4'), kind: 'video' },
    ])
  })

  it('persists the complete current local result beside its immutable uploaded URL', () => {
    const raw = {
      duration: 8,
      video_duration: 8,
      width: 720,
      height: 1280,
      frame_rate: 30,
      video_codec: 'h264',
      audio_codec: 'aac',
      container: 'mp4',
      size_bytes: 1024,
    }
    const record = {
      ...image('https://f.example/clip.mp4'),
      kind: 'video' as const,
      metadata: {
        rawDurationSec: 8,
        durationSec: 8,
        width: 720,
        height: 1280,
        aspectRatio: '9:16' as const,
        version: 1 as const,
        browserMetadata: raw,
      },
    }
    rememberUpload(7, record, NOW)
    resetUploadCache()
    expect(readUploadLibrary(7, NOW)).toEqual([record])
  })

  it('clears unreadable storage and returns an empty list', () => {
    localStorage.setItem(KEY, '{broken')

    expect(readUploadLibrary(7, NOW)).toEqual([])
    expect(localStorage.getItem(KEY)).toBeNull()
  })

  it('never stores or lists uploads for a guest', () => {
    rememberUpload(undefined, image('https://f.example/a.png'), NOW)

    expect(getUploads(undefined, NOW)).toEqual([])
    expect(localStorage.length).toBe(0)
  })

  it('keeps an upload made in another tab of the same account', () => {
    rememberUpload(7, image('https://f.example/here.png', NOW - 1), NOW)
    localStorage.setItem(
      KEY,
      JSON.stringify({
        v: 1,
        items: [
          image('https://f.example/other-tab.png', NOW - 2),
          image('https://f.example/here.png', NOW - 1),
        ],
      })
    )

    rememberUpload(7, image('https://f.example/new.png', NOW), NOW)

    expect(readUploadLibrary(7, NOW).map((r) => r.url)).toEqual([
      'https://f.example/new.png',
      'https://f.example/here.png',
      'https://f.example/other-tab.png',
    ])
  })
})
