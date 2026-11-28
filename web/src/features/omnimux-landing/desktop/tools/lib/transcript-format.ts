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
import type { ToolLocale } from '../content'
import type { ToolTranscriptCue, ToolTranscriptTrack } from './api'

export type TranscriptExportFormat = 'txt' | 'srt' | 'vtt'

const THREE_LETTER_TO_BCP47: Record<string, string> = {
  eng: 'en',
  zho: 'zh',
  cmn: 'zh',
  spa: 'es',
  jpn: 'ja',
  kor: 'ko',
  fra: 'fr',
  deu: 'de',
  por: 'pt',
  ind: 'id',
  vie: 'vi',
  tha: 'th',
  rus: 'ru',
  ara: 'ar',
  hin: 'hi',
  ita: 'it',
  msa: 'ms',
  fil: 'fil',
  tur: 'tr',
  pol: 'pl',
  nld: 'nl',
}

export function formatCueShortTime(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1000))
  const hours = Math.floor(totalSeconds / 3600)
  const minutes = String(Math.floor((totalSeconds % 3600) / 60)).padStart(
    2,
    '0'
  )
  const seconds = String(totalSeconds % 60).padStart(2, '0')
  return hours > 0 ? `${hours}:${minutes}:${seconds}` : `${minutes}:${seconds}`
}

function formatSubtitleTimestamp(ms: number, separator: ',' | '.'): string {
  const clamped = Math.max(0, Math.round(ms))
  const millis = String(clamped % 1000).padStart(3, '0')
  const totalSeconds = Math.floor(clamped / 1000)
  const hours = String(Math.floor(totalSeconds / 3600)).padStart(2, '0')
  const minutes = String(Math.floor((totalSeconds % 3600) / 60)).padStart(
    2,
    '0'
  )
  const seconds = String(totalSeconds % 60).padStart(2, '0')
  return `${hours}:${minutes}:${seconds}${separator}${millis}`
}

export function formatTranscriptCopyText(
  cues: readonly ToolTranscriptCue[],
  includeTimestamps: boolean
): string {
  if (!includeTimestamps) {
    return cues.map((cue) => cue.text).join(' ')
  }
  return cues
    .map((cue) => `[${formatCueShortTime(cue.start_ms)}] ${cue.text}`)
    .join('\n')
}

export function formatTranscriptExport(
  cues: readonly ToolTranscriptCue[],
  format: TranscriptExportFormat,
  includeTimestamps: boolean
): string {
  if (format === 'txt') {
    return formatTranscriptCopyText(cues, includeTimestamps)
  }
  if (format === 'srt') {
    return cues
      .map(
        (cue, index) =>
          `${index + 1}\n${formatSubtitleTimestamp(cue.start_ms, ',')} --> ${formatSubtitleTimestamp(cue.end_ms, ',')}\n${cue.text}\n`
      )
      .join('\n')
  }
  return `WEBVTT\n\n${cues
    .map(
      (cue) =>
        `${formatSubtitleTimestamp(cue.start_ms, '.')} --> ${formatSubtitleTimestamp(cue.end_ms, '.')}\n${cue.text}\n`
    )
    .join('\n')}`
}

function normalizeLanguageTag(track: ToolTranscriptTrack): string {
  const candidate = (track.language_code || track.lang || '').trim()
  if (!candidate) return 'und'
  const [head, ...rest] = candidate.split('-')
  const lower = head.toLowerCase()
  const mapped = THREE_LETTER_TO_BCP47[lower] ?? lower
  if (mapped === 'zh' && rest.length > 0) {
    const sub = rest[0].toUpperCase()
    if (sub === 'CN' || sub === 'HANS') return 'zh-Hans'
    if (sub === 'TW' || sub === 'HK' || sub === 'HANT') return 'zh-Hant'
  }
  return mapped
}

const TOOL_DISPLAY_LOCALE: Record<ToolLocale, string> = {
  en: 'en',
  'zh-CN': 'zh-Hans',
  'zh-TW': 'zh-Hant',
  ja: 'ja',
  es: 'es',
  id: 'id',
}

export function formatTrackLabel(
  track: ToolTranscriptTrack,
  locale: ToolLocale,
  autoLabel: string
): string {
  const tag = normalizeLanguageTag(track)
  let name = track.lang || track.language_code || tag
  if (tag !== 'und' && typeof Intl !== 'undefined' && Intl.DisplayNames) {
    try {
      const displayLocale = TOOL_DISPLAY_LOCALE[locale] ?? 'en'
      const resolved = new Intl.DisplayNames([displayLocale], {
        type: 'language',
      }).of(tag)
      if (resolved) name = resolved
    } catch {
      /* Keep upstream tag string if Intl rejects a regional code. */
    }
  }
  return track.is_auto_generated ? `${name} (${autoLabel})` : name
}

const PREFERRED_TRACK_PREFIXES: Partial<Record<ToolLocale, readonly string[]>> =
  {
    'zh-TW': ['zh-hant', 'zh'],
    'zh-CN': ['zh-hans', 'zh'],
  }

export function pickDefaultTrackIndex(
  tracks: readonly ToolTranscriptTrack[],
  locale: ToolLocale
): number {
  if (tracks.length === 0) return 0
  const prefixes = PREFERRED_TRACK_PREFIXES[locale] ?? [locale]
  for (const preferredPrefix of prefixes) {
    const creatorPreferred = tracks.findIndex(
      (track) =>
        !track.is_auto_generated &&
        normalizeLanguageTag(track).toLowerCase().startsWith(preferredPrefix)
    )
    if (creatorPreferred >= 0) return creatorPreferred
    const anyPreferred = tracks.findIndex((track) =>
      normalizeLanguageTag(track).toLowerCase().startsWith(preferredPrefix)
    )
    if (anyPreferred >= 0) return anyPreferred
  }
  const creatorFirst = tracks.findIndex((track) => !track.is_auto_generated)
  return creatorFirst >= 0 ? creatorFirst : 0
}

export function downloadTranscriptFile(
  filename: string,
  content: string
): void {
  const blob = new Blob([content], { type: 'text/plain;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  document.body.appendChild(link)
  link.click()
  link.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1000)
}
