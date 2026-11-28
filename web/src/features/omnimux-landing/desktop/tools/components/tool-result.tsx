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
import { Link, useNavigate } from '@tanstack/react-router'
import { AlertCircle, Lock, Search } from 'lucide-react'
import { useEffect, useRef, useState, type ReactNode } from 'react'
import { toast } from 'sonner'

import { writeWorkbenchMode } from '@/features/influencer/lib/workbench-mode'
import { trackToolCtaClick, type ToolCtaClick } from '@/lib/growth-tracking'

import { Icon } from '../../components/cn/icons'
import { useCopy, useLanding } from '../../i18n/context'
import { TOOL_PATHS, type ToolLocale } from '../content'
import type { ToolResult, ToolTranscriptTrack } from '../lib/api'
import {
  downloadTranscriptFile,
  formatCueShortTime,
  formatTrackLabel,
  formatTranscriptCopyText,
  formatTranscriptExport,
  pickDefaultTrackIndex,
  type TranscriptExportFormat,
} from '../lib/transcript-format'
import { TOOL_BREAKDOWN_ENABLED } from '../lib/workbench-entry'
import { useViralRecreateEntry } from '../lib/workbench-handoff'

const EMPTY_TRACKS: ToolTranscriptTrack[] = []

/** `TikTok · 2025-02-13`: the post date in UTC, independent of UI language. */
function postedLine(result: ToolResult, extra?: string): string {
  const parts = ['TikTok']
  if (result.created_at > 0) {
    parts.push(new Date(result.created_at * 1000).toISOString().slice(0, 10))
  }
  if (extra) parts.push(extra)
  return parts.join(' · ')
}

function ToolAuthor(props: { result: ToolResult; extra?: string }) {
  return (
    <div className='tool-who'>
      <i aria-hidden='true' />
      <div>
        <b>@{props.result.author.unique_id}</b>
        <span>{postedLine(props.result, props.extra)}</span>
      </div>
    </div>
  )
}

/** Tool pages render only English or Simplified Chinese. */
function useToolCtaLocale(): ToolCtaClick['locale'] {
  return useLanding().lang === 'zh-CN' ? 'zh-CN' : 'en'
}

function formatDuration(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000))
  const minutes = String(Math.floor(total / 60)).padStart(2, '0')
  const seconds = String(total % 60).padStart(2, '0')
  return `${minutes}:${seconds}`
}

function formatFrame(width: number, height: number): string {
  if (!width || !height) return ''
  if (Math.abs(width / height - 9 / 16) < 0.02) return '9:16'
  return `${width}×${height}`
}

export function ToolVideoResult(props: { result: ToolResult }) {
  const copy = useCopy().toolPage
  const locale = useToolCtaLocale()
  const startRecreate = useViralRecreateEntry(props.result)
  const video = props.result.video
  if (!video) return null
  const frame = formatFrame(video.width, video.height)
  return (
    <div className='tool-result' aria-live='polite'>
      <div className='tool-vid'>
        <video
          controls
          playsInline
          preload='metadata'
          poster={props.result.cover_url || undefined}
          src={video.preview_url}
        />
        <div>
          <ToolAuthor result={props.result} />
          {props.result.description ? (
            <p className='tool-desc'>{props.result.description}</p>
          ) : null}
          <div className='tool-chips'>
            {video.duration_ms > 0 ? (
              <span>{formatDuration(video.duration_ms)}</span>
            ) : null}
            {frame ? <span>{frame}</span> : null}
            <span>MP4</span>
          </div>
          <div className='tool-acts'>
            <a className='btn btn-light' href={video.download_url} download>
              <Icon name='dl' />
              {copy.downloadMp4}
            </a>
            <Link
              className='btn btn-primary'
              to='/influencer'
              onClick={(event) => {
                const outcome = startRecreate(event)
                if (!outcome.ok) {
                  toast.error(
                    copy.durationRange.replace('{duration}', outcome.duration)
                  )
                  return
                }
                trackToolCtaClick({
                  tool: 'video',
                  locale,
                  cta_id: 'workbench_recreate',
                  cta_variant: 'conservative',
                  placement: 'result_primary',
                })
              }}
            >
              <Icon name='spark' />
              {copy.recreateMine}
            </Link>
            {TOOL_BREAKDOWN_ENABLED ? (
              <Link className='btn btn-ghost' to='/influencer'>
                {copy.breakdown}
              </Link>
            ) : null}
          </div>
        </div>
      </div>
    </div>
  )
}

export function ToolPhotoResult(props: {
  result: ToolResult
  loggedIn: boolean
}) {
  const copy = useCopy().toolPage
  const navigate = useNavigate()
  const locale = useToolCtaLocale()
  const images = props.result.images ?? []
  const count = copy.photoCount.replace('{count}', String(images.length))
  const zipUrl = props.result.zip_url
  return (
    <div className='tool-result' aria-live='polite'>
      <div className='tool-photo-head'>
        <ToolAuthor result={props.result} extra={count} />
        <span className='tool-spacer' />
        {props.loggedIn && zipUrl ? (
          <a className='btn btn-sm btn-ghost' href={zipUrl} download>
            {copy.downloadZip}
          </a>
        ) : null}
        {props.loggedIn ? null : (
          <>
            <span className='tool-lock' id='tool-zip-lock'>
              <Lock className='ico' aria-hidden='true' />
              {copy.zipLocked}
            </span>
            <button
              type='button'
              className='btn btn-sm btn-ghost'
              aria-describedby='tool-zip-lock'
              onClick={() =>
                void navigate({
                  to: '/sign-in',
                  search: {
                    redirect: `${window.location.pathname}${window.location.search}`,
                  },
                })
              }
            >
              {copy.downloadZip}
            </button>
          </>
        )}
      </div>
      <div className='tool-imgs'>
        {images.map((image, position) => {
          const n = String(position + 1)
          return (
            <div className='tool-img' key={image.index}>
              <img
                src={image.preview_url}
                alt={copy.imageAlt.replace('{n}', n)}
                loading='lazy'
                decoding='async'
              />
              <span className='tool-n'>{n.padStart(2, '0')}</span>
              <a
                href={image.download_url}
                download
                aria-label={copy.downloadImage.replace('{n}', n)}
              >
                <Icon name='dl' />
              </a>
            </div>
          )
        })}
      </div>
      <div className='tool-divider' />
      <div className='tool-btns'>
        <Link
          className='btn btn-primary'
          to='/influencer'
          onClick={() => {
            trackToolCtaClick({
              tool: 'photo',
              locale,
              cta_id: 'workbench_recreate',
              cta_variant: 'conservative',
              placement: 'result_primary',
            })
            writeWorkbenchMode('image')
          }}
        >
          {copy.recreatePhotos}
        </Link>
        {TOOL_BREAKDOWN_ENABLED ? (
          <Link className='btn btn-ghost' to='/influencer'>
            {copy.breakdown}
          </Link>
        ) : null}
      </div>
    </div>
  )
}

function renderCueTextWithMatch(text: string, query: string): ReactNode {
  if (!query) return text
  const lower = text.toLowerCase()
  const needle = query.toLowerCase()
  const idx = lower.indexOf(needle)
  if (idx < 0) return text
  return (
    <>
      {text.slice(0, idx)}
      <mark>{text.slice(idx, idx + needle.length)}</mark>
      {text.slice(idx + needle.length)}
    </>
  )
}

function countTranscriptWords(texts: readonly string[]): number {
  let count = 0
  for (const text of texts) {
    const trimmed = text.trim()
    if (!trimmed) continue
    const tokens = trimmed.split(/\s+/).filter(Boolean)
    if (tokens.length === 1 && /[\u4e00-\u9fff]/.test(trimmed)) {
      count += trimmed.length
    } else {
      count += tokens.length
    }
  }
  return count
}

export function ToolTranscriptResult(props: {
  result: ToolResult
  locale: ToolLocale
}) {
  const copy = useCopy().toolPage
  const startRecreate = useViralRecreateEntry(props.result)
  const tracks = props.result.transcript?.tracks ?? EMPTY_TRACKS
  const hasCaptions =
    Boolean(props.result.transcript?.has_captions) && tracks.length > 0
  const [trackIndex, setTrackIndex] = useState(() =>
    pickDefaultTrackIndex(tracks, props.locale)
  )
  const [viewMode, setViewMode] = useState<'timestamps' | 'plain'>('timestamps')
  const [query, setQuery] = useState('')
  const [copiedMain, setCopiedMain] = useState(false)
  const [copiedCue, setCopiedCue] = useState<number | null>(null)
  const [coverFailed, setCoverFailed] = useState(false)
  const copiedMainTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const copiedCueTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    return () => {
      if (copiedMainTimer.current !== null) {
        clearTimeout(copiedMainTimer.current)
      }
      if (copiedCueTimer.current !== null) {
        clearTimeout(copiedCueTimer.current)
      }
    }
  }, [])

  useEffect(() => {
    setTrackIndex(pickDefaultTrackIndex(tracks, props.locale))
    setQuery('')
    setCopiedMain(false)
    setCopiedCue(null)
    setCoverFailed(false)
  }, [props.result.id, props.result.cover_url, props.locale, tracks])

  const video = props.result.video
  const frame = video ? formatFrame(video.width, video.height) : ''
  const dateLabel =
    props.result.created_at > 0
      ? new Date(props.result.created_at * 1000).toISOString().slice(0, 10)
      : ''

  const copyClipboard = (text: string, onDone: () => void) => {
    if (typeof navigator === 'undefined' || !navigator.clipboard?.writeText) {
      return
    }
    void navigator.clipboard.writeText(text).then(onDone, () => {})
  }

  const showCopiedMain = () => {
    setCopiedMain(true)
    if (copiedMainTimer.current !== null) {
      clearTimeout(copiedMainTimer.current)
    }
    copiedMainTimer.current = setTimeout(() => setCopiedMain(false), 1800)
  }

  const showCopiedCue = (index: number) => {
    setCopiedCue(index)
    if (copiedCueTimer.current !== null) {
      clearTimeout(copiedCueTimer.current)
    }
    copiedCueTimer.current = setTimeout(() => setCopiedCue(null), 1200)
  }

  const activeTrack = tracks[trackIndex] ?? tracks[0]
  const trimmedQuery = query.trim()
  let filteredCues = activeTrack?.cues ?? []
  if (trimmedQuery) {
    const needle = trimmedQuery.toLowerCase()
    filteredCues = filteredCues.filter((cue) =>
      cue.text.toLowerCase().includes(needle)
    )
  }
  const linesCountLabel = copy.transcriptLinesCount.replace(
    '{count}',
    String(filteredCues.length)
  )
  const wordsCountLabel = copy.transcriptWordsCount.replace(
    '{count}',
    String(countTranscriptWords(filteredCues.map((cue) => cue.text)))
  )

  const handle = props.result.author.unique_id || 'tiktok'
  let transcriptBody: ReactNode = null
  if (filteredCues.length === 0) {
    transcriptBody = <div className='tx-plain'>{copy.transcriptEmptyMatch}</div>
  } else if (viewMode === 'timestamps') {
    transcriptBody = filteredCues.map((cue, index) => (
      <button
        key={`${cue.start_ms}-${cue.end_ms}-${cue.text}`}
        type='button'
        className='tx-cue'
        onClick={() =>
          copyClipboard(
            `[${formatCueShortTime(cue.start_ms)}] ${cue.text}`,
            () => showCopiedCue(index)
          )
        }
      >
        <span className='tx-time'>
          {copiedCue === index
            ? copy.transcriptCueCopied
            : formatCueShortTime(cue.start_ms)}
        </span>
        <span className='tx-line'>
          {renderCueTextWithMatch(cue.text, trimmedQuery)}
        </span>
      </button>
    ))
  } else {
    transcriptBody = (
      <div className='tx-plain'>
        {filteredCues.map((cue) => cue.text).join(' ')}
      </div>
    )
  }

  return (
    <div className='tool-result' aria-live='polite'>
      <div className='tx-head'>
        {props.result.cover_url && !coverFailed ? (
          <img
            className='tx-cover'
            src={props.result.cover_url}
            alt={`@${handle}`}
            loading='lazy'
            decoding='async'
            onError={() => setCoverFailed(true)}
          />
        ) : (
          <div className='tx-cover tx-cover-empty' aria-hidden='true'>
            <Icon name='subs' />
          </div>
        )}
        <div className='tx-summary'>
          <div className='tx-who'>
            <b>@{handle}</b>
            {dateLabel ? <span className='tx-date'>{dateLabel}</span> : null}
          </div>
          <div className='tool-chips tx-chips'>
            {video && video.duration_ms > 0 ? (
              <span>{formatDuration(video.duration_ms)}</span>
            ) : null}
            {frame ? <span>{frame}</span> : null}
            <span className={hasCaptions ? 'tx-badge-ok' : 'tx-badge-empty'}>
              {hasCaptions
                ? copy.transcriptSourceBadge
                : copy.transcriptNoCapBadge}
            </span>
          </div>
          {props.result.description ? (
            <p className='tool-desc'>{props.result.description}</p>
          ) : null}
        </div>
        <Link
          className='btn btn-sm btn-ghost tx-head-dl'
          to={TOOL_PATHS[props.locale].video}
        >
          {copy.transcriptVideoDownloader}
        </Link>
      </div>

      {hasCaptions && activeTrack ? (
        <>
          <div className='tx-toolbar'>
            <div
              className='tx-tracks'
              role='group'
              aria-label={copy.transcriptTracksAria}
            >
              {tracks.map((track, index) => (
                <button
                  key={track.id}
                  type='button'
                  className='tx-pill'
                  aria-pressed={index === trackIndex}
                  onClick={() => {
                    setTrackIndex(index)
                    setCopiedCue(null)
                    setCopiedMain(false)
                  }}
                >
                  {formatTrackLabel(
                    track,
                    props.locale,
                    copy.transcriptAutoLabel
                  )}
                </button>
              ))}
            </div>
            <div
              className='tx-view-toggle'
              role='group'
              aria-label={copy.transcriptViewAria}
            >
              <button
                type='button'
                aria-pressed={viewMode === 'timestamps'}
                onClick={() => setViewMode('timestamps')}
              >
                {copy.transcriptModeTime}
              </button>
              <button
                type='button'
                aria-pressed={viewMode === 'plain'}
                onClick={() => setViewMode('plain')}
              >
                {copy.transcriptModePlain}
              </button>
            </div>
          </div>

          <div className='tx-search-bar'>
            <label className='tx-search'>
              <Search className='ico' aria-hidden='true' />
              <span className='sr-only'>
                {copy.transcriptSearchPlaceholder}
              </span>
              <input
                type='search'
                value={query}
                placeholder={copy.transcriptSearchPlaceholder}
                onChange={(event) => setQuery(event.target.value)}
              />
            </label>
            <span className='tx-stats'>
              <span>{linesCountLabel}</span>
              <span>{wordsCountLabel}</span>
            </span>
            <button
              type='button'
              className='btn btn-sm btn-light'
              disabled={filteredCues.length === 0}
              onClick={() =>
                copyClipboard(
                  formatTranscriptCopyText(
                    filteredCues,
                    viewMode === 'timestamps'
                  ),
                  showCopiedMain
                )
              }
            >
              <Icon name={copiedMain ? 'check' : 'copy'} />
              {copiedMain ? copy.transcriptCopied : copy.transcriptCopy}
            </button>
          </div>

          <div className='tx-body'>{transcriptBody}</div>

          <div className='tx-foot'>
            <div className='tx-exports'>
              {(
                [
                  ['txt', copy.transcriptExportTxt],
                  ['srt', copy.transcriptExportSrt],
                  ['vtt', copy.transcriptExportVtt],
                ] as [TranscriptExportFormat, string][]
              ).map(([format, label]) => (
                <button
                  key={format}
                  type='button'
                  className='btn btn-sm btn-ghost'
                  disabled={filteredCues.length === 0}
                  onClick={() =>
                    downloadTranscriptFile(
                      `${handle}-${props.result.id}-${activeTrack.language_code}.${format}`,
                      formatTranscriptExport(
                        filteredCues,
                        format,
                        viewMode === 'timestamps'
                      )
                    )
                  }
                >
                  {label}
                </button>
              ))}
            </div>
            <Link
              className='btn btn-sm btn-ghost'
              to='/influencer'
              onClick={(event) => {
                const outcome = startRecreate(event)
                if (!outcome.ok) {
                  toast.error(
                    copy.durationRange.replace('{duration}', outcome.duration)
                  )
                  return
                }
                trackToolCtaClick({
                  tool: 'transcript',
                  locale: props.locale,
                  cta_id: 'workbench_recreate',
                  cta_variant: 'conservative',
                  placement: 'result_primary',
                })
              }}
            >
              <Icon name='spark' />
              {copy.transcriptRecreate}
            </Link>
          </div>
        </>
      ) : (
        <div className='tx-fallback'>
          <div className='tx-fallback-head'>
            <AlertCircle className='ico' aria-hidden='true' />
            <span>{copy.transcriptNoCapTitle}</span>
          </div>
          <p>{copy.transcriptNoCapBody}</p>
          {props.result.description ? (
            <div className='tx-post-caption'>
              <span>{props.result.description}</span>
              <button
                type='button'
                className='btn btn-sm btn-ghost'
                onClick={() =>
                  copyClipboard(props.result.description, showCopiedMain)
                }
              >
                {copiedMain ? copy.transcriptCopied : copy.transcriptCopyDesc}
              </button>
            </div>
          ) : null}
          <div className='tx-fallback-acts'>
            {video ? (
              <a className='btn btn-light' href={video.download_url} download>
                <Icon name='dl' />
                {copy.downloadMp4}
              </a>
            ) : null}
            <Link className='btn btn-ghost' to='/ai-influencer-studio'>
              {copy.transcriptOpenStudio}
            </Link>
          </div>
        </div>
      )}
    </div>
  )
}
