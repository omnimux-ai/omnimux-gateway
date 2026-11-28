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
import { Link } from '@tanstack/react-router'
import { AlertCircle, Check, Copy, Languages, Lock, Search } from 'lucide-react'
import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import { toast } from 'sonner'

import { trackToolCtaClick } from '@/lib/growth-tracking'

import { Icon } from '../../components/cn/icons'
import { useCopy } from '../../i18n/context'
import { TOOL_LOCALES, TOOL_PATHS, type ToolLocale } from '../content'
import {
  breakdownTikTokTranscript,
  ToolRequestError,
  translateTikTokTranscript,
  type ToolBreakdownResult,
  type ToolResult,
  type ToolTranscriptCue,
  type ToolTranscriptTrack,
} from '../lib/api'
import { formatBreakdownMarkdown } from '../lib/transcript-demo'
import {
  downloadTranscriptFile,
  formatCueShortTime,
  formatTrackLabel,
  formatTranscriptCopyText,
  formatTranscriptExport,
  pickDefaultTrackIndex,
  type TranscriptExportFormat,
} from '../lib/transcript-format'
import { useViralRecreateEntry } from '../lib/workbench-handoff'

const EMPTY_TRACKS: ToolTranscriptTrack[] = []
const EMPTY_CUES: readonly ToolTranscriptCue[] = []

type WorkbenchTab = 'script' | 'breakdown'
type TranslationViewMode = 'original' | 'bilingual' | 'translated'

const TARGET_LANG_NAMES: Record<ToolLocale, string> = {
  'zh-CN': '简体中文',
  'zh-TW': '繁體中文',
  en: 'English',
  ja: '日本語',
  es: 'Español',
  id: 'Bahasa Indonesia',
}

function formatDuration(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000))
  const minutes = String(Math.floor(total / 60)).padStart(2, '0')
  const seconds = String(total % 60).padStart(2, '0')
  return `${minutes}:${seconds}`
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

function pickDefaultTargetLocale(
  track: ToolTranscriptTrack | undefined,
  pageLocale: ToolLocale
): ToolLocale {
  const code = (track?.language_code || track?.lang || '').toLowerCase()
  if (pageLocale === 'en' && code.startsWith('en')) return 'zh-CN'
  if (
    (pageLocale === 'zh-CN' || pageLocale === 'zh-TW') &&
    code.startsWith('zh')
  ) {
    return 'en'
  }
  if (code.startsWith(pageLocale.toLowerCase().slice(0, 2))) {
    return pageLocale === 'en' ? 'zh-CN' : 'en'
  }
  return pageLocale
}

export type TranscriptWorkbenchProps = {
  result: ToolResult
  locale: ToolLocale
  isDemo?: boolean
  demoTranslationTarget?: ToolLocale
  demoTranslation?: readonly ToolTranscriptCue[]
  demoBreakdown?: ToolBreakdownResult
  loggedIn?: boolean
  onRequireLogin?: () => void
  onResetDemo?: () => void
}

export function TranscriptWorkbench(props: TranscriptWorkbenchProps) {
  const copy = useCopy().toolPage
  const startRecreate = useViralRecreateEntry(props.result)
  const tracks = props.result.transcript?.tracks ?? EMPTY_TRACKS
  const hasCaptions =
    Boolean(props.result.transcript?.has_captions) && tracks.length > 0

  const [tab, setTab] = useState<WorkbenchTab>('script')
  const [trackIndex, setTrackIndex] = useState(() =>
    pickDefaultTrackIndex(tracks, props.locale)
  )
  const [viewMode, setViewMode] = useState<'timestamps' | 'plain'>('timestamps')
  const [transView, setTransView] = useState<TranslationViewMode>('original')
  const [targetLang, setTargetLang] = useState<ToolLocale>(() =>
    props.isDemo && props.demoTranslationTarget
      ? props.demoTranslationTarget
      : pickDefaultTargetLocale(
          tracks[pickDefaultTrackIndex(tracks, props.locale)],
          props.locale
        )
  )
  const [translations, setTranslations] = useState<
    Record<string, readonly ToolTranscriptCue[]>
  >({})
  const [translating, setTranslating] = useState(false)

  const [breakdowns, setBreakdowns] = useState<
    Record<string, ToolBreakdownResult>
  >({})
  const [breakdownLoadingKey, setBreakdownLoadingKey] = useState<string | null>(
    null
  )
  const [breakdownError, setBreakdownError] = useState('')

  const [query, setQuery] = useState('')
  const [copiedMain, setCopiedMain] = useState(false)
  const [copiedBd, setCopiedBd] = useState(false)
  const [copiedCue, setCopiedCue] = useState<number | null>(null)
  const [activeCueIdx, setActiveCueIdx] = useState<number | null>(null)
  const [videoFailed, setVideoFailed] = useState(false)
  const [coverFailed, setCoverFailed] = useState(false)

  const videoRef = useRef<HTMLVideoElement | null>(null)
  const sectionRefs = useRef<(HTMLElement | null)[]>([])
  const copiedMainTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const copiedBdTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const copiedCueTimer = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    return () => {
      if (copiedMainTimer.current !== null) {
        clearTimeout(copiedMainTimer.current)
      }
      if (copiedBdTimer.current !== null) {
        clearTimeout(copiedBdTimer.current)
      }
      if (copiedCueTimer.current !== null) {
        clearTimeout(copiedCueTimer.current)
      }
    }
  }, [])

  useEffect(() => {
    const idx = pickDefaultTrackIndex(tracks, props.locale)
    setTrackIndex(idx)
    setTab('script')
    setTransView('original')
    setTargetLang(
      props.isDemo && props.demoTranslationTarget
        ? props.demoTranslationTarget
        : pickDefaultTargetLocale(tracks[idx], props.locale)
    )
    setTranslations({})
    setBreakdowns({})
    setBreakdownError('')
    setBreakdownLoadingKey(null)
    setQuery('')
    setCopiedMain(false)
    setCopiedBd(false)
    setCopiedCue(null)
    setActiveCueIdx(null)
    setVideoFailed(false)
    setCoverFailed(false)
  }, [
    props.result.id,
    props.result.cover_url,
    props.locale,
    props.isDemo,
    props.demoTranslationTarget,
    tracks,
  ])

  const activeTrack = tracks[trackIndex] ?? tracks[0]
  const activeTransKey = activeTrack ? `${activeTrack.id}:${targetLang}` : ''
  const activeTranslatedCues = activeTransKey
    ? translations[activeTransKey]
    : undefined

  const breakdownKey = activeTrack ? `${activeTrack.id}:${props.locale}` : ''
  const breakdownLoading =
    breakdownKey !== '' && breakdownLoadingKey === breakdownKey
  let activeBreakdown: ToolBreakdownResult | undefined
  if (props.isDemo) {
    activeBreakdown = props.demoBreakdown
  } else if (breakdownKey) {
    activeBreakdown = breakdowns[breakdownKey]
  }

  const mapAIError = useCallback(
    (error: unknown): string => {
      if (!(error instanceof ToolRequestError)) return copy.errors.generic
      if (error.code === 'login_required' || error.status === 401) {
        return copy.errors.loginRequired
      }
      if (error.code === 'cache_expired' || error.status === 410) {
        return copy.errors.cacheExpired
      }
      if (error.code === 'ai_quota_exceeded') return copy.errors.aiQuotaExceeded
      if (error.code === 'rate_limited' || error.status === 429) {
        return copy.errors.rateLimited
      }
      if (error.code === 'busy') return copy.errors.busy
      if (error.code === 'upstream_failed') return copy.errors.upstreamFailed
      if (error.code === 'unavailable_service') {
        return copy.errors.serviceUnavailable
      }
      return copy.errors.generic
    },
    [copy.errors]
  )

  const copyClipboard = (text: string, onDone: () => void) => {
    if (typeof navigator === 'undefined' || !navigator.clipboard?.writeText) {
      return
    }
    void navigator.clipboard.writeText(text).then(onDone, () => {})
  }

  const showCopiedMain = () => {
    setCopiedMain(true)
    if (copiedMainTimer.current !== null) clearTimeout(copiedMainTimer.current)
    copiedMainTimer.current = setTimeout(() => setCopiedMain(false), 1800)
  }

  const showCopiedBd = () => {
    setCopiedBd(true)
    if (copiedBdTimer.current !== null) clearTimeout(copiedBdTimer.current)
    copiedBdTimer.current = setTimeout(() => setCopiedBd(false), 1800)
  }

  const showCopiedCue = (index: number) => {
    setCopiedCue(index)
    if (copiedCueTimer.current !== null) clearTimeout(copiedCueTimer.current)
    copiedCueTimer.current = setTimeout(() => setCopiedCue(null), 1200)
  }

  const sourceCues = activeTrack?.cues ?? EMPTY_CUES
  const syncActiveCue = useCallback(
    (ms: number) => {
      const idx = sourceCues.findIndex(
        (cue) =>
          ms >= cue.start_ms && ms < Math.max(cue.end_ms, cue.start_ms + 500)
      )
      setActiveCueIdx(idx >= 0 ? idx : null)
    },
    [sourceCues]
  )

  const seekToMs = useCallback((ms: number, cueIndex?: number) => {
    if (typeof cueIndex === 'number') setActiveCueIdx(cueIndex)
    const videoEl = videoRef.current
    if (!videoEl) return
    try {
      videoEl.currentTime = Math.max(0, ms / 1000)
    } catch {
      /* Ignore seek before metadata settles in test or browser runtime. */
    }
  }, [])

  const handleTranslate = async () => {
    if (!activeTrack) return
    const demoTranslation = props.demoTranslation
    if (props.isDemo && demoTranslation) {
      setTranslations((prev) => ({
        ...prev,
        [activeTransKey]: demoTranslation,
      }))
      setTransView((prev) => (prev === 'original' ? 'bilingual' : 'original'))
      return
    }
    if (!props.loggedIn) {
      toast.error(copy.errors.loginRequired, {
        action: props.onRequireLogin
          ? {
              label: copy.transcriptBreakdownLoginBtn,
              onClick: props.onRequireLogin,
            }
          : undefined,
      })
      return
    }
    if (activeTranslatedCues) {
      setTransView((prev) => (prev === 'original' ? 'bilingual' : prev))
      return
    }
    setTranslating(true)
    try {
      const res = await translateTikTokTranscript(
        props.result.id,
        activeTrack.id,
        targetLang
      )
      setTranslations((prev) => ({ ...prev, [activeTransKey]: res.cues }))
      setTransView('bilingual')
    } catch (error) {
      const msg = mapAIError(error)
      toast.error(
        msg,
        error instanceof ToolRequestError &&
          (error.code === 'login_required' || error.status === 401) &&
          props.onRequireLogin
          ? {
              action: {
                label: copy.transcriptBreakdownLoginBtn,
                onClick: props.onRequireLogin,
              },
            }
          : undefined
      )
    } finally {
      setTranslating(false)
    }
  }

  const runBreakdown = useCallback(
    async (force = false) => {
      if (props.isDemo || !activeTrack || !props.loggedIn || !breakdownKey) {
        return
      }
      if (!force && activeBreakdown) return
      const keyForRun = breakdownKey
      setBreakdownLoadingKey(keyForRun)
      setBreakdownError('')
      try {
        const res = await breakdownTikTokTranscript(
          props.result.id,
          activeTrack.id,
          props.locale
        )
        setBreakdowns((prev) => ({ ...prev, [keyForRun]: res }))
      } catch (error) {
        setBreakdownError(mapAIError(error))
      } finally {
        setBreakdownLoadingKey((curr) => (curr === keyForRun ? null : curr))
      }
    },
    [
      props.isDemo,
      props.loggedIn,
      props.result.id,
      props.locale,
      activeTrack,
      activeBreakdown,
      breakdownKey,
      mapAIError,
    ]
  )

  const openBreakdownTab = () => {
    setTab('breakdown')
    if (
      !props.isDemo &&
      props.loggedIn &&
      !activeBreakdown &&
      !breakdownLoading
    ) {
      void runBreakdown(false)
    }
  }

  const effectiveCues = sourceCues.map((cue, index) => {
    const tr = activeTranslatedCues?.[index]?.text
    if (transView === 'translated' && tr) {
      return { ...cue, text: tr, translated: undefined as string | undefined }
    }
    if (transView === 'bilingual' && tr) {
      return { ...cue, translated: tr }
    }
    return { ...cue, translated: undefined as string | undefined }
  })

  const trimmedQuery = query.trim()
  const filteredItems = effectiveCues
    .map((cue, index) => ({ cue, index }))
    .filter(({ cue }) => {
      if (!trimmedQuery) return true
      const needle = trimmedQuery.toLowerCase()
      return (
        cue.text.toLowerCase().includes(needle) ||
        Boolean(cue.translated?.toLowerCase().includes(needle))
      )
    })

  const filteredExportCues: ToolTranscriptCue[] = filteredItems.map(
    ({ cue }) =>
      transView === 'bilingual' && cue.translated
        ? { ...cue, text: `${cue.text}\n${cue.translated}` }
        : { start_ms: cue.start_ms, end_ms: cue.end_ms, text: cue.text }
  )

  const linesCountLabel = copy.transcriptLinesCount.replace(
    '{count}',
    String(filteredItems.length)
  )
  const wordsCountLabel = copy.transcriptWordsCount.replace(
    '{count}',
    String(countTranscriptWords(filteredItems.map(({ cue }) => cue.text)))
  )

  const handle = props.result.author.unique_id || 'tiktok'
  const video = props.result.video
  const dateLabel =
    props.result.created_at > 0
      ? new Date(props.result.created_at * 1000).toISOString().slice(0, 10)
      : ''

  const recreateHref = '/influencer'
  const handleRecreateClick = (event: React.MouseEvent<HTMLAnchorElement>) => {
    if (props.isDemo) {
      trackToolCtaClick({
        tool: 'transcript',
        locale: props.locale,
        cta_id: 'workbench_recreate',
        cta_variant: 'conservative',
        placement: 'result_primary',
      })
      return
    }
    const outcome = startRecreate(event)
    if (!outcome.ok) {
      toast.error(copy.durationRange.replace('{duration}', outcome.duration))
      return
    }
    trackToolCtaClick({
      tool: 'transcript',
      locale: props.locale,
      cta_id: 'workbench_recreate',
      cta_variant: 'conservative',
      placement: 'result_primary',
    })
  }

  let mediaNode: ReactNode
  if (video?.preview_url && !videoFailed) {
    mediaNode = (
      <video
        ref={videoRef}
        className='tx-video'
        src={video.preview_url}
        poster={
          props.result.cover_url && !coverFailed
            ? props.result.cover_url
            : undefined
        }
        controls
        playsInline
        preload='metadata'
        aria-label={`@${handle}`}
        onTimeUpdate={(event) =>
          syncActiveCue(event.currentTarget.currentTime * 1000)
        }
        onSeeked={(event) =>
          syncActiveCue(event.currentTarget.currentTime * 1000)
        }
        onError={() => setVideoFailed(true)}
      />
    )
  } else if (props.result.cover_url && !coverFailed) {
    mediaNode = (
      <img
        className='tx-cover-poster'
        src={props.result.cover_url}
        alt={`@${handle}`}
        loading='lazy'
        decoding='async'
        onError={() => setCoverFailed(true)}
      />
    )
  } else {
    mediaNode = (
      <div className='tx-cover-poster tx-cover-empty' aria-hidden='true'>
        <Icon name='subs' />
      </div>
    )
  }

  let transcriptBody: ReactNode = null
  if (filteredItems.length === 0) {
    transcriptBody = <div className='tx-plain'>{copy.transcriptEmptyMatch}</div>
  } else if (viewMode === 'timestamps') {
    transcriptBody = filteredItems.map(({ cue, index }) => (
      <div
        key={`${cue.start_ms}-${cue.end_ms}-${index}`}
        className='tx-cue-row'
        data-active={activeCueIdx === index}
      >
        <button
          type='button'
          className='tx-cue-btn'
          onClick={() => seekToMs(cue.start_ms, index)}
        >
          <span className='tx-time'>{formatCueShortTime(cue.start_ms)}</span>
          <span className='tx-line'>
            <span className='tx-line-main'>
              {renderCueTextWithMatch(cue.text, trimmedQuery)}
            </span>
            {cue.translated ? (
              <span className='tx-cue-trans'>
                {renderCueTextWithMatch(cue.translated, trimmedQuery)}
              </span>
            ) : null}
          </span>
        </button>
        <button
          type='button'
          className='tx-cue-copy'
          aria-label={
            copiedCue === index ? copy.transcriptCueCopied : copy.transcriptCopy
          }
          onClick={() =>
            copyClipboard(
              `[${formatCueShortTime(cue.start_ms)}] ${cue.text}${cue.translated ? `\n${cue.translated}` : ''}`,
              () => showCopiedCue(index)
            )
          }
        >
          {copiedCue === index ? (
            <Check className='ico' aria-hidden='true' />
          ) : (
            <Copy className='ico' aria-hidden='true' />
          )}
        </button>
      </div>
    ))
  } else {
    transcriptBody = (
      <div className='tx-plain'>
        {filteredItems.map(({ cue }) => cue.text).join(' ')}
        {transView === 'bilingual' && activeTranslatedCues ? (
          <p className='tx-plain-trans'>
            {filteredItems
              .map(({ cue }) => cue.translated || '')
              .filter(Boolean)
              .join(' ')}
          </p>
        ) : null}
      </div>
    )
  }

  const stageLabelFor = (stage: string, fallback: string) => {
    const key = stage as keyof typeof copy.transcriptStageLabels
    return copy.transcriptStageLabels[key] ?? fallback
  }

  let breakdownPanelBody: ReactNode = null
  if (activeBreakdown) {
    breakdownPanelBody = (
      <>
        <div className='tx-bd-flow'>
          {activeBreakdown.sections.map((section, idx) => (
            <span
              key={`${section.stage}-${section.start_ms}-${section.label}`}
              className='tx-bd-flow-step'
            >
              {idx > 0 ? (
                <span className='tx-bd-arrow' aria-hidden='true'>
                  →
                </span>
              ) : null}
              <button
                type='button'
                className='tx-stage-pill'
                onClick={() => {
                  seekToMs(section.start_ms)
                  sectionRefs.current[idx]?.scrollIntoView({
                    behavior: 'smooth',
                    block: 'nearest',
                  })
                }}
              >
                {stageLabelFor(section.stage, section.label)}
              </button>
            </span>
          ))}
        </div>

        <div className='tx-body tx-bd-body'>
          <p className='tx-bd-summary'>{activeBreakdown.summary}</p>
          <div className='tx-bd-list'>
            {activeBreakdown.sections.map((section, idx) => (
              <article
                key={`${section.stage}-${section.start_ms}-${section.label}`}
                ref={(el) => {
                  sectionRefs.current[idx] = el
                }}
                className='tx-bd-card'
              >
                <div className='tx-bd-card-head'>
                  <div className='tx-bd-card-title'>
                    <span className='tx-stage-badge'>
                      {stageLabelFor(section.stage, section.label)}
                    </span>
                    <h3>{section.label}</h3>
                  </div>
                  <button
                    type='button'
                    className='tx-bd-time'
                    onClick={() => seekToMs(section.start_ms)}
                  >
                    {formatCueShortTime(section.start_ms)} –{' '}
                    {formatCueShortTime(section.end_ms)}
                  </button>
                </div>
                <blockquote className='tx-bd-quote'>{section.quote}</blockquote>
                <p className='tx-bd-analysis'>{section.analysis}</p>
              </article>
            ))}
          </div>
        </div>

        <div className='tx-foot'>
          <div className='tx-exports'>
            <button
              type='button'
              className='btn btn-sm btn-light'
              onClick={() =>
                copyClipboard(
                  formatBreakdownMarkdown(activeBreakdown),
                  showCopiedBd
                )
              }
            >
              <Icon name={copiedBd ? 'check' : 'copy'} />
              {copiedBd ? copy.transcriptCopied : copy.transcriptBreakdownCopy}
            </button>
          </div>
          <Link
            className='btn btn-sm btn-primary'
            to={recreateHref}
            onClick={handleRecreateClick}
          >
            <Icon name='spark' />
            {copy.transcriptRecreate}
          </Link>
        </div>
      </>
    )
  } else if (!props.loggedIn) {
    breakdownPanelBody = (
      <div className='tx-bd-gate'>
        <Lock className='ico' aria-hidden='true' />
        <h3>{copy.transcriptBreakdownLoginTitle}</h3>
        <p>{copy.transcriptBreakdownLoginBody}</p>
        {props.onRequireLogin ? (
          <button
            type='button'
            className='btn btn-primary'
            onClick={props.onRequireLogin}
          >
            {copy.transcriptBreakdownLoginBtn}
          </button>
        ) : null}
      </div>
    )
  } else if (breakdownLoading) {
    breakdownPanelBody = (
      <div className='tx-bd-gate'>
        <span className='tool-spin' aria-hidden='true' />
        <p>{copy.transcriptAnalyzing}</p>
      </div>
    )
  } else {
    breakdownPanelBody = (
      <div className='tx-bd-gate'>
        <AlertCircle className='ico' aria-hidden='true' />
        <p>{breakdownError || copy.errors.generic}</p>
        <button
          type='button'
          className='btn btn-light'
          onClick={() => void runBreakdown(true)}
        >
          {copy.transcriptBreakdownRetry}
        </button>
      </div>
    )
  }

  return (
    <div className='tool-result tx-workbench-wrap' aria-live='polite'>
      <div className='tx-workbench'>
        <aside className='tx-media-pane'>
          <div className='tx-media-box'>
            {props.isDemo ? (
              <span className='tx-demo-badge'>{copy.transcriptDemoBadge}</span>
            ) : null}
            {mediaNode}
          </div>

          <div className='tx-meta-card'>
            <div className='tx-who'>
              <b>@{handle}</b>
              {dateLabel ? <span className='tx-date'>{dateLabel}</span> : null}
            </div>
            <div className='tool-chips tx-chips'>
              <span className={hasCaptions ? 'tx-badge-ok' : 'tx-badge-empty'}>
                {hasCaptions
                  ? copy.transcriptSourceBadge
                  : copy.transcriptNoCapBadge}
              </span>
            </div>
            {props.result.description ? (
              <p className='tx-desc-clamp'>{props.result.description}</p>
            ) : null}
            <dl className='tx-stat-list'>
              {video && video.duration_ms > 0 ? (
                <div>
                  <dt>{copy.transcriptStatDuration}</dt>
                  <dd>{formatDuration(video.duration_ms)}</dd>
                </div>
              ) : null}
              {hasCaptions ? (
                <div>
                  <dt>{copy.transcriptStatSegments}</dt>
                  <dd>{sourceCues.length}</dd>
                </div>
              ) : null}
              {hasCaptions ? (
                <div>
                  <dt>
                    {tab === 'breakdown' && activeBreakdown
                      ? copy.transcriptStatSections
                      : copy.transcriptStatTracks}
                  </dt>
                  <dd>
                    {tab === 'breakdown' && activeBreakdown
                      ? activeBreakdown.sections.length
                      : tracks.length}
                  </dd>
                </div>
              ) : null}
            </dl>
            <div className='tx-meta-links'>
              <Link
                className='btn btn-sm btn-ghost'
                to={TOOL_PATHS[props.locale].video}
              >
                {copy.transcriptVideoDownloader}
              </Link>
              {!props.isDemo && props.onResetDemo ? (
                <button
                  type='button'
                  className='btn btn-sm btn-ghost'
                  onClick={props.onResetDemo}
                >
                  {copy.transcriptReturnDemo}
                </button>
              ) : null}
            </div>
          </div>
        </aside>

        <div className='tx-right-pane'>
          {hasCaptions && activeTrack ? (
            <>
              <div className='tx-tabs' role='tablist'>
                <button
                  type='button'
                  role='tab'
                  id='tx-tab-script'
                  className='tx-tab-script'
                  aria-selected={tab === 'script'}
                  aria-controls='tx-panel-script'
                  onClick={() => setTab('script')}
                >
                  {copy.transcriptTabScript}
                </button>
                <button
                  type='button'
                  role='tab'
                  id='tx-tab-breakdown'
                  className='tx-tab-breakdown'
                  aria-selected={tab === 'breakdown'}
                  aria-controls='tx-panel-breakdown'
                  onClick={openBreakdownTab}
                >
                  {copy.transcriptTabBreakdown}
                </button>
              </div>

              {tab === 'script' ? (
                <div
                  id='tx-panel-script'
                  role='tabpanel'
                  aria-labelledby='tx-tab-script'
                  className='tx-panel'
                >
                  <p className='tx-panel-sub'>{copy.transcriptScriptSub}</p>

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
                            setActiveCueIdx(null)
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

                    <div className='tx-view-groups'>
                      {activeTranslatedCues ? (
                        <div className='tx-view-toggle' role='group'>
                          {(
                            [
                              ['original', copy.transcriptViewOrig],
                              ['bilingual', copy.transcriptViewBilingual],
                              ['translated', copy.transcriptViewTranslated],
                            ] as const
                          ).map(([mode, label]) => (
                            <button
                              key={mode}
                              type='button'
                              aria-pressed={transView === mode}
                              onClick={() => setTransView(mode)}
                            >
                              {label}
                            </button>
                          ))}
                        </div>
                      ) : null}

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
                  </div>

                  <div className='tx-body'>{transcriptBody}</div>

                  <div className='tx-foot'>
                    <div className='tx-exports'>
                      {!props.isDemo && video ? (
                        <a
                          className='btn btn-sm btn-ghost'
                          href={video.download_url}
                          download
                        >
                          <Icon name='dl' />
                          {copy.downloadMp4}
                        </a>
                      ) : null}

                      <div className='tx-translate-wrap'>
                        {!props.isDemo ? (
                          <select
                            className='tx-lang-select'
                            aria-label={copy.transcriptTranslateTargetAria}
                            value={targetLang}
                            onChange={(event) =>
                              setTargetLang(event.target.value as ToolLocale)
                            }
                          >
                            {TOOL_LOCALES.map((loc) => (
                              <option key={loc} value={loc}>
                                {TARGET_LANG_NAMES[loc]}
                              </option>
                            ))}
                          </select>
                        ) : null}
                        <button
                          type='button'
                          className='btn btn-sm btn-ghost'
                          disabled={translating}
                          onClick={() => void handleTranslate()}
                        >
                          {translating ? (
                            <span className='tool-spin' aria-hidden='true' />
                          ) : (
                            <Languages className='ico' aria-hidden='true' />
                          )}
                          {translating
                            ? copy.transcriptTranslating
                            : copy.transcriptTranslate}
                        </button>
                      </div>

                      <button
                        type='button'
                        className='btn btn-sm btn-light'
                        disabled={filteredExportCues.length === 0}
                        onClick={() =>
                          copyClipboard(
                            formatTranscriptCopyText(
                              filteredExportCues,
                              viewMode === 'timestamps'
                            ),
                            showCopiedMain
                          )
                        }
                      >
                        <Icon name={copiedMain ? 'check' : 'copy'} />
                        {copiedMain
                          ? copy.transcriptCopied
                          : copy.transcriptCopy}
                      </button>

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
                          disabled={filteredExportCues.length === 0}
                          onClick={() =>
                            downloadTranscriptFile(
                              `${handle}-${props.result.id}-${activeTrack.language_code}.${format}`,
                              formatTranscriptExport(
                                filteredExportCues,
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

                    <div className='tx-foot-cta'>
                      <button
                        type='button'
                        className='btn btn-sm btn-ghost'
                        onClick={openBreakdownTab}
                      >
                        <Icon name='flow' />
                        {copy.transcriptAnalyzeBtn}
                      </button>
                      <Link
                        className='btn btn-sm btn-primary'
                        to={recreateHref}
                        onClick={handleRecreateClick}
                      >
                        <Icon name='spark' />
                        {copy.transcriptRecreate}
                      </Link>
                    </div>
                  </div>
                </div>
              ) : (
                <div
                  id='tx-panel-breakdown'
                  role='tabpanel'
                  aria-labelledby='tx-tab-breakdown'
                  className='tx-panel'
                >
                  <p className='tx-panel-sub'>{copy.transcriptBreakdownSub}</p>
                  {breakdownPanelBody}
                </div>
              )}
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
                    {copiedMain
                      ? copy.transcriptCopied
                      : copy.transcriptCopyDesc}
                  </button>
                </div>
              ) : null}
              <div className='tx-fallback-acts'>
                {video ? (
                  <a
                    className='btn btn-light'
                    href={video.download_url}
                    download
                  >
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
      </div>
    </div>
  )
}
