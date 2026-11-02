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
import {
  Download,
  Images,
  RotateCcw,
  Trash2,
  Upload,
  Video,
} from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

import { describeFailReason } from '../lib/fail-reason'
import {
  parseTaskMeta,
  taskBrowserMediaUrl,
  taskImageUrl,
  taskKind,
  taskVideoUrl,
} from '../lib/history'
import { canDeriveMultiView } from '../lib/multiview'
import {
  mediaFormat,
  type PreviewEntry,
  type PreviewMediaKind,
} from '../lib/preview-reference'
import { parseStoredMotionPayload } from '../lib/submit-payload'
import { isTerminalStatus } from '../lib/task-status'
import type { InfluencerTaxonomy, TaskRecord } from '../types'
import { FailReasonDetail } from './fail-reason-detail'
import { FullscreenPreviewDialog } from './fullscreen-preview-dialog'
import { PreviewDetails } from './preview-details'
import { TaskMediaPreview } from './task-media-preview'
import { TaskPreviewInfo } from './task-preview-info'

/** Actions and their availability for a generated record, computed by the page. */
export interface TaskPreviewActions {
  onReloadPreview?: () => void
  onRegenerate: (task: TaskRecord) => void
  onDelete: (task: TaskRecord) => void
  /** Present while removal would hide a derived generation that still runs. */
  deleteDisabledReason?: string
  onGenerateMultiView: (task: TaskRecord) => void
  multiViewDisabledReason?: string
  onPublish?: (task: TaskRecord) => void
  onTurnToMotion?: (task: TaskRecord) => void
  turnToMotionDisabledReason?: string
}

/** Existing form-filling actions for an upload or an untraceable reference. */
export interface ReferencePreviewActions {
  onUseImage: (url: string, fileName: string) => void
  useImageDisabledReason?: (url: string) => string | undefined
  onUseVideo: (entry: Extract<PreviewEntry, { kind: 'upload' }>) => void
  useVideoDisabledReason?: string
}

interface Props {
  /** Bottom layer first; the last entry is shown. */
  stack: PreviewEntry[]
  taxonomy: InfluencerTaxonomy | null
  onClose: () => void
  onBack: () => void
  onOpenReference?: (url: string, media: PreviewMediaKind) => void
  /** Actions for the generated record shown on top, or undefined when it is read-only. */
  taskActions: (task: TaskRecord) => TaskPreviewActions | undefined
  referenceActions?: ReferencePreviewActions
}

/** Stored metadata is display-only: never resolve it as an applicable preset. */
function historicalDetails(
  task: TaskRecord,
  taxonomy: InfluencerTaxonomy | null
): { brief?: string; tags: string[] } {
  const meta = parseTaskMeta(task)
  let input: unknown
  try {
    input = JSON.parse(task.properties?.input ?? 'null')
  } catch {
    input = null
  }
  const raw = input as { brief?: unknown; prompt?: unknown } | null
  const brief = typeof raw?.brief === 'string' ? raw.brief : raw?.prompt
  const tags: string[] = []
  if (
    meta &&
    taxonomy &&
    meta.selection &&
    typeof meta.selection === 'object'
  ) {
    for (const category of taxonomy.categories) {
      const ids = meta.selection[category.id]
      if (!Array.isArray(ids)) continue
      for (const id of ids) {
        const option = category.options.find((item) => item.id === id)
        if (option) tags.push(option.label_en)
      }
    }
  }
  return { brief: typeof brief === 'string' ? brief : undefined, tags }
}

function entryKey(entry: PreviewEntry): string {
  if (entry.kind === 'task') {
    return `task:${entry.task.task_id || entry.task.id}`
  }
  if (entry.kind === 'upload') return `upload:${entry.record.url}`
  return `unknown:${entry.url}`
}

/**
 * Footer actions share one two-column grid: the primary spans the row and
 * the remaining actions take half a row each, all at the same 40px height.
 */
const ACTION_CLASS =
  'h-10 min-w-0 gap-1.5 rounded-[8px] px-3 text-sm font-medium whitespace-nowrap'
const PRIMARY_CLASS = cn(
  ACTION_CLASS,
  'col-span-2 bg-white text-zinc-950 hover:bg-white/90'
)
const SECONDARY_CLASS = cn(
  ACTION_CLASS,
  'border border-white/10 bg-white/5 text-white hover:bg-white/10 hover:text-white'
)

function ActionGrid(props: { children: ReactNode }) {
  return <div className='grid grid-cols-2 gap-2'>{props.children}</div>
}

/** One history preview window; nested references swap its content in place. */
export function HistoryPreviewDialog(props: Props) {
  const { t } = useTranslation()
  const top = props.stack.at(-1)
  const [mediaUnavailableKey, setMediaUnavailableKey] = useState<string | null>(
    null
  )
  const [deleteTask, setDeleteTask] = useState<TaskRecord | null>(null)
  if (!top) return null
  const key = entryKey(top)
  const mediaUnavailable = mediaUnavailableKey === key
  const onUnavailableChange = (unavailable: boolean) =>
    setMediaUnavailableKey((current) => {
      if (unavailable) return key
      return current === key ? null : current
    })

  let view: RailView
  if (top.kind === 'task') {
    view = taskView({
      t,
      task: top.task,
      taxonomy: props.taxonomy,
      actions: top.readOnly ? undefined : props.taskActions(top.task),
      mediaUnavailable,
      onUnavailableChange,
      onOpenReference: props.onOpenReference,
      onDelete: setDeleteTask,
    })
  } else {
    view = referenceView({
      entry: top,
      actions: props.referenceActions,
      mediaUnavailable,
      onUnavailableChange,
    })
  }

  const actions = deleteTask ? props.taskActions(deleteTask) : undefined
  return (
    <>
      <FullscreenPreviewDialog
        open
        mediaFirst
        onOpenChange={(open) => {
          if (!open) props.onClose()
        }}
        onBack={props.stack.length > 1 ? props.onBack : undefined}
        title={t('Preview')}
        backgroundUrl={view.backgroundUrl}
        media={
          <div key={key} className='size-full'>
            {view.media}
          </div>
        }
        actions={view.actions}
      >
        <div key={key} className='flex flex-col gap-2'>
          {view.rail}
        </div>
      </FullscreenPreviewDialog>
      <ConfirmDialog
        open={deleteTask !== null}
        onOpenChange={(open) => {
          if (!open) setDeleteTask(null)
        }}
        title={t('Delete')}
        desc={t(
          'Remove this record from the history gallery? Source files will not be deleted.'
        )}
        confirmText={t('Delete')}
        destructive
        disabled={Boolean(actions?.deleteDisabledReason)}
        handleConfirm={() => {
          const task = deleteTask
          setDeleteTask(null)
          if (!task || !actions || actions.deleteDisabledReason) return
          actions.onDelete(task)
        }}
      />
    </>
  )
}

interface RailView {
  media: ReactNode
  rail: ReactNode
  actions?: ReactNode
  backgroundUrl?: string
}

function taskView(input: {
  t: (key: string, options?: Record<string, unknown>) => string
  task: TaskRecord
  taxonomy: InfluencerTaxonomy | null
  actions: TaskPreviewActions | undefined
  mediaUnavailable: boolean
  onUnavailableChange: (unavailable: boolean) => void
  onOpenReference?: (url: string, media: PreviewMediaKind) => void
  onDelete: (task: TaskRecord) => void
}): RailView {
  const { t, task, actions } = input
  const video = taskKind(task) === 'motion'
  const success = ['success', 'succeeded'].includes(
    (task.status ?? '').toLowerCase()
  )
  let url: string | null = null
  if (success) url = video ? taskVideoUrl(task) : taskImageUrl(task)
  const poster =
    success && task.poster_url
      ? taskBrowserMediaUrl(task.poster_url)
      : undefined
  const details = historicalDetails(task, input.taxonomy)
  const failure = describeFailReason(t, task.fail_reason)
  const failurePending =
    failure.kind === 'empty' && !isTerminalStatus(task.status)
  const unreadableMotion =
    video && !parseStoredMotionPayload(task.properties?.input)
  const cannotReuse =
    unreadableMotion ||
    (!video && taskKind(task) === 'sheet' && !parseTaskMeta(task))
  const media = success ? (
    <TaskMediaPreview
      identity={task.task_id || String(task.id)}
      src={url}
      poster={poster}
      video={video}
      detail
      onUnavailableChange={input.onUnavailableChange}
      onReloadRecord={actions?.onReloadPreview}
    />
  ) : (
    <div
      role='status'
      className='grid h-full w-full place-items-center p-3 text-center'
    >
      <div className='flex w-full flex-col items-center gap-2'>
        <p className='text-sm text-[var(--inf-danger)]'>
          {failurePending ? t('Generating…') : failure.message}
        </p>
        <FailReasonDetail view={failure} />
      </div>
    </div>
  )
  const download =
    url && !input.mediaUnavailable ? (
      <Button
        variant='ghost'
        className={SECONDARY_CLASS}
        nativeButton={false}
        render={
          <a
            href={taskBrowserMediaUrl(url, true)}
            download
            target='_blank'
            rel='noreferrer'
            role='link'
          />
        }
      >
        <Download aria-hidden='true' />
        {t('Download')}
      </Button>
    ) : null
  let footer: ReactNode
  if (actions) {
    footer = (
      <div className='flex flex-col gap-2'>
        <ActionGrid>
          <Button
            className={PRIMARY_CLASS}
            disabled={cannotReuse}
            onClick={() => actions.onRegenerate(task)}
          >
            <RotateCcw aria-hidden='true' />
            {t('Regenerate')}
          </Button>
          {!video && url && actions.onTurnToMotion ? (
            <Button
              variant='ghost'
              className={SECONDARY_CLASS}
              disabled={
                input.mediaUnavailable ||
                Boolean(actions.turnToMotionDisabledReason)
              }
              title={actions.turnToMotionDisabledReason}
              onClick={() => actions.onTurnToMotion?.(task)}
            >
              <Video aria-hidden='true' />
              {t('Turn to motion')}
            </Button>
          ) : null}
          {canDeriveMultiView(task) ? (
            <Button
              variant='ghost'
              className={SECONDARY_CLASS}
              disabled={Boolean(actions.multiViewDisabledReason)}
              aria-description={actions.multiViewDisabledReason || undefined}
              onClick={() => actions.onGenerateMultiView(task)}
            >
              <Images aria-hidden='true' />
              {t('Generate multi-view')}
            </Button>
          ) : null}
          {download}
          <Button
            variant='ghost'
            className={cn(
              SECONDARY_CLASS,
              'hover:text-[var(--inf-danger)]',
              !download && 'col-span-2'
            )}
            disabled={Boolean(actions.deleteDisabledReason)}
            aria-description={actions.deleteDisabledReason || undefined}
            onClick={() => input.onDelete(task)}
          >
            <Trash2 aria-hidden='true' />
            {t('Delete')}
          </Button>
          {actions.onPublish ? (
            <Button
              variant='ghost'
              className={cn(SECONDARY_CLASS, 'col-span-2')}
              onClick={() => actions.onPublish?.(task)}
            >
              <Upload aria-hidden='true' />
              {t('Publish')}
            </Button>
          ) : null}
        </ActionGrid>
      </div>
    )
  } else if (download) {
    footer = (
      <ActionGrid>
        <div className='col-span-2 grid'>{download}</div>
      </ActionGrid>
    )
  }
  return {
    media,
    backgroundUrl: video ? poster : (url ?? undefined),
    actions: footer,
    rail: (
      <>
        <TaskPreviewInfo task={task} onOpenReference={input.onOpenReference} />
        <PreviewDetails
          brief={details.brief}
          tags={details.tags.map((tag) => t(tag))}
        />
      </>
    ),
  }
}

function referenceView(input: {
  entry: Exclude<PreviewEntry, { kind: 'task' }>
  actions?: ReferencePreviewActions
  mediaUnavailable: boolean
  onUnavailableChange: (unavailable: boolean) => void
}): RailView {
  const entry = input.entry
  const url = entry.kind === 'upload' ? entry.record.url : entry.url
  const media: PreviewMediaKind =
    entry.kind === 'upload' ? entry.record.kind : entry.media
  return {
    backgroundUrl: media === 'image' ? url : undefined,
    media: (
      <TaskMediaPreview
        identity={url}
        src={url}
        video={media === 'video'}
        detail
        onUnavailableChange={input.onUnavailableChange}
      />
    ),
    rail: <ReferencePreviewInfo entry={entry} />,
    actions: input.actions ? (
      <ReferenceActions
        entry={entry}
        url={url}
        media={media}
        actions={input.actions}
        mediaUnavailable={input.mediaUnavailable}
      />
    ) : undefined,
  }
}

function ReferenceActions(props: {
  entry: Exclude<PreviewEntry, { kind: 'task' }>
  url: string
  media: PreviewMediaKind
  actions: ReferencePreviewActions
  mediaUnavailable: boolean
}) {
  const { t } = useTranslation()
  const entry = props.entry
  if (props.media === 'video') {
    // An untraceable clip has no checked metadata, so it cannot fill the slot.
    if (entry.kind !== 'upload') return null
    const reason = props.actions.useVideoDisabledReason
    return (
      <ActionGrid>
        <Button
          className={PRIMARY_CLASS}
          disabled={props.mediaUnavailable || Boolean(reason)}
          title={reason}
          onClick={() => props.actions.onUseVideo(entry)}
        >
          <Video aria-hidden='true' />
          {t('Use this motion')}
        </Button>
      </ActionGrid>
    )
  }
  const reason = props.actions.useImageDisabledReason?.(props.url)
  const fileName =
    entry.kind === 'upload' ? entry.record.fileName : t('Reference images')
  return (
    <ActionGrid>
      <Button
        className={PRIMARY_CLASS}
        disabled={props.mediaUnavailable || Boolean(reason)}
        title={reason}
        onClick={() => props.actions.onUseImage(props.url, fileName)}
      >
        <Video aria-hidden='true' />
        {t('Turn to motion')}
      </Button>
    </ActionGrid>
  )
}

/** Only facts the browser actually holds for an upload; nothing is inferred beyond the format. */
export function ReferencePreviewInfo(props: {
  entry: Exclude<PreviewEntry, { kind: 'task' }>
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const entry = props.entry
  const media = entry.kind === 'upload' ? entry.record.kind : entry.media
  const url = entry.kind === 'upload' ? entry.record.url : entry.url
  const rows: { label: string; value: string }[] = [
    {
      label: t('Source'),
      value: entry.kind === 'upload' ? t('Uploaded') : t('Unknown'),
    },
    {
      label: t('Type'),
      value: media === 'video' ? t('Video file') : t('Image file'),
    },
  ]
  const format =
    mediaFormat(entry.kind === 'upload' ? entry.record.fileName : url) ??
    (entry.kind === 'upload' ? mediaFormat(url) : undefined)
  if (format) rows.push({ label: t('Format'), value: format })
  if (entry.kind === 'upload') {
    const metadata = entry.record.metadata
    if (metadata) {
      rows.push({
        label: t('Duration (seconds)'),
        value: formatNumber(metadata.durationSec, locale),
      })
      rows.push({
        label: t('Dimensions'),
        value: `${metadata.width}×${metadata.height}`,
      })
      if (metadata.aspectRatio) {
        rows.push({ label: t('Aspect ratio'), value: metadata.aspectRatio })
      }
    }
    if (entry.record.fileName) {
      rows.push({ label: t('File name'), value: entry.record.fileName })
    }
    rows.push({
      label: t('Expires at'),
      value: formatTimestampToDate(entry.record.expiresAt, 'milliseconds'),
    })
  }
  return (
    <section
      aria-label={t('Info')}
      className='flex shrink-0 flex-col gap-4 px-3 py-3 text-sm text-[var(--inf-txt)]'
    >
      <h2 className='font-semibold'>{t('Info')}</h2>
      <dl className='flex flex-col gap-3'>
        {rows.map((row) => (
          <div key={row.label} className='grid grid-cols-2 gap-3'>
            <dt className='text-[var(--inf-txt-2)]'>{row.label}</dt>
            <dd className='min-w-0 text-right break-words'>{row.value}</dd>
          </div>
        ))}
      </dl>
    </section>
  )
}
