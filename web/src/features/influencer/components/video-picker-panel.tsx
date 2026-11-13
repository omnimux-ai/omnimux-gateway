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
import { Loader2 } from 'lucide-react'
import { useMemo, useRef, useState, type RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth-store'

import { useAutoLoadMore } from '../hooks/use-auto-load-more'
import { useUploadThumbnails } from '../hooks/use-upload-thumbnails'
import { toastInfluencerError } from '../lib/error-copy'
import { taskVideoUrl } from '../lib/history'
import { referenceVideoCandidate, taskTitle } from '../lib/reference-candidates'
import type { MotionVideoSource, TaskRecord } from '../types'
import { PickerMediaCard } from './picker-media-card'
import {
  ReferencePickerTabs,
  ReferenceUploadCard,
  type ReferenceFilter,
} from './reference-picker-controls'
import { ReferencePickerShell } from './reference-picker-shell'

const EMPTY_UPLOADS: Extract<MotionVideoSource, { kind: 'upload' }>[] = []

interface VideoPickerPanelProps {
  open: boolean
  anchorRef?: RefObject<HTMLElement | null>
  onClose: () => void
  historyItems: TaskRecord[]
  uploadedVideos?: Extract<MotionVideoSource, { kind: 'upload' }>[]
  onSelectUpload?: (
    source: Extract<MotionVideoSource, { kind: 'upload' }>
  ) => void
  selectedUploadUrl?: string
  selectedTaskId?: string
  selectedVideoUrl?: string
  selectedArtifactKey?: string
  preparationPhase?: 'reading' | 'checking' | 'uploading' | null
  uploadProgress?: number
  busy?: boolean
  preparingTaskId?: string
  isInitialLoading?: boolean
  error?: unknown
  hasMore?: boolean
  isFetchingMore?: boolean
  loadMoreFailed?: boolean
  onLoadMore: () => void
  onRetry?: () => void
  onUploadFile: (file: File) => Promise<void>
  onSelectTask: (task: TaskRecord) => void
  /** Opens the shared playback window; the picker itself never plays media. */
  onPreviewVideo: (url: string) => void
}

/** Receives eligible history from the parent; selection is committed by its callback. */
export function VideoPickerPanel(props: VideoPickerPanelProps) {
  return props.open ? <VideoPickerContent {...props} /> : null
}

function formatDuration(seconds: number | undefined): string | undefined {
  if (!Number.isFinite(seconds) || !seconds || seconds <= 0) return undefined
  const total = Math.round(seconds)
  const minutes = Math.floor(total / 60)
  const rest = total % 60
  return `${minutes}:${String(rest).padStart(2, '0')}`
}

function VideoPickerContent(props: VideoPickerPanelProps) {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const gridRef = useRef<HTMLDivElement>(null)
  const [filter, setFilter] = useState<ReferenceFilter>('all')
  const ownerId = useAuthStore((state) => state.auth.user?.id)
  const uploads = useMemo(
    () =>
      filter === 'generated'
        ? EMPTY_UPLOADS
        : (props.uploadedVideos ?? EMPTY_UPLOADS),
    [filter, props.uploadedVideos]
  )
  const uploadUrls = useMemo(
    () => uploads.map((source) => source.url),
    [uploads]
  )
  const uploadPosters = useUploadThumbnails(ownerId, uploadUrls)
  const history = filter === 'uploaded' ? [] : props.historyItems
  const uploadStatus =
    props.preparationPhase && !props.preparingTaskId
      ? t(
          {
            reading: 'Reading reference video…',
            checking: 'Checking reference video…',
            uploading: 'Uploading reference video…',
          }[props.preparationPhase]
        )
      : undefined
  const canAutoLoad =
    filter !== 'uploaded' && Boolean(props.hasMore) && !props.isFetchingMore
  const sentinelRef = useAutoLoadMore({
    enabled: props.open && canAutoLoad,
    rootRef: gridRef,
    onLoadMore: props.onLoadMore,
  })

  const upload = (files: FileList | File[]) => {
    if (props.busy || files.length === 0) return
    if (files.length !== 1) {
      toast.error(t('Choose one video at a time'))
      return
    }
    void props
      .onUploadFile(files[0])
      .catch((failure) =>
        toastInfluencerError(t, failure, 'Could not prepare video')
      )
  }
  if (!props.open) return null
  return (
    <ReferencePickerShell
      anchorRef={props.anchorRef}
      open
      onClose={props.onClose}
      busy={props.busy}
      hideBusyStatus
      headerContent={
        <ReferencePickerTabs value={filter} onChange={setFilter} />
      }
      title={t('Select reference videos')}
      onDrop={(event) => {
        event.preventDefault()
        upload(event.dataTransfer.files)
      }}
    >
      <input
        ref={fileInputRef}
        type='file'
        accept='video/mp4,video/quicktime,.mp4,.mov'
        className='hidden'
        aria-hidden='true'
        tabIndex={-1}
        disabled={props.busy}
        onChange={(event) => {
          if (event.target.files) upload(event.target.files)
          event.target.value = ''
        }}
      />
      <div ref={gridRef} className='inf-character-grid'>
        <ReferenceUploadCard
          label={t('Upload local video')}
          status={uploadStatus}
          progress={
            props.preparationPhase === 'uploading'
              ? props.uploadProgress
              : undefined
          }
          disabled={props.busy}
          onClick={() => {
            setFilter('all')
            fileInputRef.current?.click()
          }}
        />
        {uploads.map((source, index) => (
          <PickerMediaCard
            key={source.url}
            title={source.fileName}
            displaySrc={uploadPosters.get(source.url) ?? null}
            fallbackVideoSrc={source.url}
            kind='video'
            selected={props.selectedUploadUrl === source.url}
            priority={index < 8}
            disabled={props.busy}
            durationLabel={formatDuration(source.metadata?.durationSec)}
            onSelect={() => props.onSelectUpload?.(source)}
            onPreview={() => props.onPreviewVideo(source.url)}
          />
        ))}
        {history.map((task, index) => {
          const candidate = referenceVideoCandidate(task)
          const url = candidate?.previewUrl ?? taskVideoUrl(task)
          if (!url) return null
          const id = task.task_id || String(task.id)
          const title = taskTitle(task) || t('Reference video')
          const selected =
            props.selectedTaskId === id &&
            (props.selectedArtifactKey
              ? props.selectedArtifactKey === candidate?.artifactKey
              : !props.selectedVideoUrl || props.selectedVideoUrl === url)
          return (
            <PickerMediaCard
              key={`${id}:${url}`}
              title={title}
              displaySrc={candidate?.posterUrl ?? null}
              fallbackVideoSrc={url}
              kind='video'
              selected={selected}
              priority={index < 8}
              disabled={props.busy}
              durationLabel={formatDuration(candidate?.metadata?.durationSec)}
              overlay={
                props.preparingTaskId === id ? (
                  <span
                    aria-label={`${t('Preparing reference')}: ${title}`}
                    className='inf-reference-video-preparing'
                  >
                    <Loader2
                      className='size-6 animate-spin'
                      aria-hidden='true'
                    />
                  </span>
                ) : undefined
              }
              onSelect={() => props.onSelectTask(task)}
              onPreview={() => props.onPreviewVideo(url)}
            />
          )
        })}
        {filter !== 'uploaded' && (props.hasMore || props.loadMoreFailed) ? (
          <div ref={sentinelRef} className='inf-picker-sentinel' aria-hidden />
        ) : null}
      </div>
      {filter !== 'uploaded' && props.isInitialLoading && (
        <div role='status' className='inf-reference-picker-state'>
          {t('Loading videos')}
        </div>
      )}
      {filter !== 'uploaded' &&
        !props.isInitialLoading &&
        Boolean(props.error) && (
          <div role='alert' className='inf-reference-picker-state'>
            <span>{t('Could not load videos')}</span>
            {props.onRetry && (
              <Button type='button' variant='ghost' onClick={props.onRetry}>
                {t('Retry')}
              </Button>
            )}
          </div>
        )}
      {filter !== 'uploaded' && props.loadMoreFailed && (
        <div role='alert' className='inf-reference-picker-state'>
          {t('Could not load more videos')}
        </div>
      )}
      {filter !== 'uploaded' && (props.hasMore || props.loadMoreFailed) && (
        <Button
          type='button'
          variant='ghost'
          className='inf-reference-picker-more'
          disabled={props.isFetchingMore || props.isInitialLoading}
          onClick={props.onLoadMore}
        >
          {props.isFetchingMore && (
            <Loader2 className='size-4 animate-spin' aria-hidden='true' />
          )}
          {props.loadMoreFailed ? t('Retry loading more') : t('Load more')}
        </Button>
      )}
    </ReferencePickerShell>
  )
}
