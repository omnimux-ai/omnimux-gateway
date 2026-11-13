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

import { createHash } from 'node:crypto'
import { readFileSync } from 'node:fs'
import { basename, resolve } from 'node:path'

import { describe, expect, it } from 'vitest'

import en from '@/i18n/locales/en.json'
import fr from '@/i18n/locales/fr.json'
import ja from '@/i18n/locales/ja.json'
import ru from '@/i18n/locales/ru.json'
import vi from '@/i18n/locales/vi.json'
import zhTW from '@/i18n/locales/zh-TW.json'
import zh from '@/i18n/locales/zh.json'
import { STATIC_I18N_KEYS } from '@/i18n/static-keys'

import provenance from '../data/motion-style-provenance.json'
import { MOTION_VISUAL_STYLES } from '../lib/motion-visual-styles'

describe('motion visual style catalogue', () => {
  it('offers all twenty visual treatments with unique stable ids', () => {
    expect(MOTION_VISUAL_STYLES).toHaveLength(20)
    expect(new Set(MOTION_VISUAL_STYLES.map((style) => style.id)).size).toBe(20)
    expect(MOTION_VISUAL_STYLES[0]?.id).toBe('motion-cel-animation')
    expect(
      new Set(MOTION_VISUAL_STYLES.map((style) => style.nameKey)).size
    ).toBe(20)
    for (const style of MOTION_VISUAL_STYLES) {
      expect(style.id).toMatch(/^motion-[a-z0-9-]+$/)
      expect(style.mode).toBe('motion')
      expect(style.nameKey).toMatch(/^[A-Za-z0-9 ,.-]+$/)
      expect(style.summaryKey).toMatch(/^[A-Za-z0-9 ,.-]+$/)
    }
  })

  it('each adaptation preserves user identity, movement and timing across the whole clip', () => {
    for (const style of MOTION_VISUAL_STYLES) {
      expect(style.prompt).toContain('身份、物种和辨识点由用户角色参考确定')
      expect(style.prompt).toContain('动作、镜头与节奏由用户参考视频确定')
      expect(style.prompt).toContain(
        '不引入风格样片中的人物、衣物、道具、场景、字幕、水印或商标'
      )
      expect(style.prompt).toContain(
        '整个视频保持统一的画风、材质与配色，帧间稳定'
      )
    }
  })

  it('provides registered names and descriptions in every supported locale', () => {
    for (const style of MOTION_VISUAL_STYLES) {
      for (const key of [style.nameKey, style.summaryKey]) {
        expect(STATIC_I18N_KEYS).toContain(key)
        for (const locale of [en, zh, zhTW, fr, ja, ru, vi]) {
          const translations: Record<string, unknown> = locale.translation
          expect(translations[key]).toBeTruthy()
          if (locale !== en) expect(translations[key]).not.toBe(key)
        }
      }
    }
  })

  it('keeps every preview separate from generation media fields', () => {
    for (const style of MOTION_VISUAL_STYLES) {
      expect(Object.keys(style).sort()).toEqual([
        'id',
        'mode',
        'nameKey',
        'previewImageUrl',
        'previewVideoUrl',
        'prompt',
        'summaryKey',
      ])
      expect(style.previewImageUrl).toMatch(
        /^\/influencer-styles\/[a-f0-9]{64}\.webp$/
      )
      expect(style.previewVideoUrl).toMatch(
        /^\/influencer-styles\/[a-f0-9]{64}\.mp4\?v=identity-20261016$/
      )
    }
  })

  it('requests a fresh cache version for all twenty preview videos', () => {
    for (const style of MOTION_VISUAL_STYLES) {
      if (!style.previewVideoUrl) throw new Error('Missing preview video')
      const url = new URL(style.previewVideoUrl, 'https://omnimux.ai')
      expect(url.search).toBe('?v=identity-20261016')
      expect(url.hash).toBe('')
    }
  })

  it('ships all forty local previews with filenames matching their actual content', () => {
    const paths = MOTION_VISUAL_STYLES.flatMap((style) => [
      style.previewImageUrl,
      style.previewVideoUrl,
    ])
    expect(new Set(paths).size).toBe(40)
    for (const assetPath of paths) {
      if (!assetPath) throw new Error('Every style must provide both previews')
      const url = new URL(assetPath, 'https://omnimux.ai')
      const filename = basename(url.pathname)
      const asset = readFileSync(
        resolve(process.cwd(), 'public', url.pathname.slice(1))
      )
      expect(createHash('sha256').update(asset).digest('hex')).toBe(
        filename.split('.')[0]
      )
      expect(asset.byteLength).toBeLessThan(1_250_000)
    }
  })

  it('records source ids, preview checksums and non-official untested prompt provenance', () => {
    expect(provenance.catalogueVersion).toBe(
      'omnimux-motion-visual-styles-20261016-v1'
    )
    expect(provenance.sourceCatalogue.sha256).toMatch(/^[a-f0-9]{64}$/)
    expect(provenance.promptSource.sha256).toMatch(/^[a-f0-9]{64}$/)
    expect(provenance.promptSource.origin).toBe(
      'omnimux-adaptation-not-official'
    )
    expect(provenance.promptSource.officialFullPromptAvailable).toBe(false)
    expect(provenance.promptSource.generationValidated).toBe(false)
    expect(provenance.styles).toHaveLength(20)
    expect(
      new Set(provenance.styles.map((record) => record.sourceId)).size
    ).toBe(20)
    for (const style of MOTION_VISUAL_STYLES) {
      const record = provenance.styles.find((item) => item.id === style.id)
      if (!record) throw new Error(`Missing provenance for ${style.id}`)
      expect(record.sourceName).toBe(style.nameKey)
      expect(record.sourceId).toMatch(/^[a-f0-9-]{36}$/)
      expect(record.mediaRole).toBe('edit-preview-only')
      expect(record.promptSha256).toBe(
        createHash('sha256').update(style.prompt).digest('hex')
      )
      expect(record.image.path).toBe(style.previewImageUrl)
      if (!style.previewVideoUrl) throw new Error('Missing preview video')
      expect(record.video.path).toBe(
        new URL(style.previewVideoUrl, 'https://omnimux.ai').pathname
      )
      for (const media of [record.image, record.video]) {
        expect(media.path).toContain(media.sha256)
        expect(media.archivePath).toContain(media.sha256)
        const payload = readFileSync(
          resolve(process.cwd(), 'public', media.path.slice(1))
        )
        expect(payload.byteLength).toBe(media.sizeBytes)
      }
    }
  })
})
