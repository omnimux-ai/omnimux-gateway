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
import { useEffect, useMemo, useRef, useState, type RefObject } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { useAuthStore } from '@/stores/auth-store'

import { useAutoLoadMore } from '../hooks/use-auto-load-more'
import { useUploadThumbnails } from '../hooks/use-upload-thumbnails'
import { toastInfluencerError } from '../lib/error-copy'
import {
  taskBrief,
  taskCardImageUrl,
  taskCharacterSource,
  resolveTaskCharacterSource,
  taskImageUrl,
  taskAbsoluteImageUrl,
} from '../lib/history'
import { referenceImageTasks } from '../lib/reference-candidates'
import type {
  MotionCharacterSource,
  PendingCharacterUpload,
  TaskRecord,
} from '../types'
import { PendingUploadCard } from './pending-upload-card'
import { PickerMediaCard } from './picker-media-card'
import {
  ReferencePickerTabs,
  ReferenceUploadCard,
  type ReferenceFilter,
} from './reference-picker-controls'
import { ReferencePickerShell } from './reference-picker-shell'

import './character-assets.css'

const EMPTY_UPLOADS: MotionCharacterSource[] = []
const EMPTY_PENDING: PendingCharacterUpload[] = []

interface Props {
  open: boolean
  anchorRef?: RefObject<HTMLElement | null>
  onClose: () => void
  characters: MotionCharacterSource[]
  /** Selection ceiling for this surface; the workbench itself allows 30. */
  max?: number
  uploadedCharacters?: MotionCharacterSource[]
  /** Files currently in the multi-select upload batch. */
  uploads?: PendingCharacterUpload[]
  historyItems: TaskRecord[]
  busy?: boolean
  hasMore?: boolean
  isFetchingMore?: boolean
  loadMoreFailed?: boolean
  onLoadMore?: () => void
  /** Receives every file from one selection, in picker order. */
  onUploadFiles: (files: File[]) => void
  onRetryUpload?: (id: string) => void
  onDismissUpload?: (id: string) => void
  onToggleUrl: (source: MotionCharacterSource) => void
  /** Opens the selected image in the shared preview window. */
  onPreviewImage?: (url: string) => void
}

/** Local uploads and generated images share one grid, with up to 30 selections. */
export function CharacterPickerPanel(props: Props) {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const gridRef = useRef<HTMLDivElement>(null)
  const [filter, setFilter] = useState<ReferenceFilter>('all')
  const [resolving, setResolving] = useState(false)
  const requestRef = useRef(0)
  const ownerId = useAuthStore((state) => state.auth.user?.id)
  const uploads = useMemo(
    () =>
      filter === 'generated'
        ? EMPTY_UPLOADS
        : (props.uploadedCharacters ?? EMPTY_UPLOADS),
    [filter, props.uploadedCharacters]
  )
  const uploadUrls = useMemo(
    () => uploads.map((source) => source.url),
    [uploads]
  )
  const uploadThumbs = useUploadThumbnails(ownerId, uploadUrls)

  useEffect(() => {
    requestRef.current += 1
    setResolving(false)
  }, [props.open])

  const candidateTasks = useMemo(
    () => referenceImageTasks(props.historyItems),
    [props.historyItems]
  )
  const selectedUrlSet = useMemo(
    () => new Set(props.characters.map((character) => character.url)),
    [props.characters]
  )
  const sources = useMemo(() => {
    const history = filter === 'uploaded' ? [] : candidateTasks
    return [
      ...uploads.map((source) => ({
        source,
        task: undefined,
        displayUrl: uploadThumbs.get(source.url) ?? source.url,
        fallbackUrl: source.url,
      })),
      ...history.map((task) => {
        const original = taskImageUrl(task) || ''
        const url = taskAbsoluteImageUrl(task)
        const title = taskBrief(task) || t('Character')
        return {
          task,
          displayUrl: taskCardImageUrl(task) || original,
          fallbackUrl: original,
          source: taskCharacterSource(
            task,
            url ?? original,
            title,
            props.historyItems
          ),
        }
      }),
    ].filter(
      (item, index, items) =>
        item.displayUrl &&
        items.findIndex((other) => other.source.url === item.source.url) ===
          index
    )
  }, [candidateTasks, filter, props.historyItems, t, uploadThumbs, uploads])

  const max = props.max ?? 30
  const isFull = props.characters.length >= max
  const canAutoLoad =
    filter !== 'uploaded' && Boolean(props.hasMore) && !props.isFetchingMore
  const sentinelRef = useAutoLoadMore({
    enabled: props.open && canAutoLoad,
    rootRef: gridRef,
    onLoadMore: () => props.onLoadMore?.(),
  })

  if (!props.open) return null

  const pendingUploads = props.uploads ?? EMPTY_PENDING
  const activeUploads = pendingUploads.filter(
    (upload) => upload.status !== 'error'
  )
  // Pending transfers count toward the ceiling, so the entry disables only
  // when the list is genuinely full — never merely because a batch is running.
  const atCapacity = props.characters.length + activeUploads.length >= max

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const files = [...(event.target.files ?? [])]
    event.target.value = ''
    if (files.length === 0 || props.busy || atCapacity) return
    props.onUploadFiles(files)
  }

  const selectSource = (
    source: MotionCharacterSource,
    task: TaskRecord | undefined,
    url: string,
    selected: boolean
  ) => {
    if (!url || !url.startsWith('https://')) return
    if (!task || selected || source.characterType) {
      props.onToggleUrl(source)
      return
    }
    const request = ++requestRef.current
    setResolving(true)
    void resolveTaskCharacterSource(
      task,
      url,
      source.fileName,
      props.historyItems
    )
      .then((resolved) => {
        if (request === requestRef.current) {
          props.onToggleUrl(resolved)
        }
      })
      .catch((error: unknown) => {
        toastInfluencerError(t, error, 'Something went wrong!')
      })
      .finally(() => {
        if (request === requestRef.current) setResolving(false)
      })
  }

  return (
    <ReferencePickerShell
      anchorRef={props.anchorRef}
      open={props.open}
      onClose={props.onClose}
      busy={props.busy}
      hideBusyStatus
      headerContent={
        <ReferencePickerTabs value={filter} onChange={setFilter} />
      }
      title={t('Select reference images')}
      description={t(
        max === 30
          ? t(
              'Choose generated characters or upload character, product or clothing images (up to 30)'
            )
          : t(
              'Choose generated characters or upload character, product or clothing images (up to {{count}})',
              { count: max }
            )
      )}
    >
      <input
        ref={fileInputRef}
        type='file'
        accept='image/png,image/jpeg,image/webp'
        multiple
        className='hidden'
        aria-hidden='true'
        tabIndex={-1}
        disabled={props.busy || atCapacity}
        onChange={handleFileChange}
      />
      <div ref={gridRef} className='inf-character-grid'>
        <ReferenceUploadCard
          label={t('Upload local photo')}
          disabled={props.busy || atCapacity}
          onClick={() => {
            setFilter('all')
            fileInputRef.current?.click()
          }}
        />
        {pendingUploads.map((upload) => (
          <PendingUploadCard
            key={upload.id}
            upload={upload}
            busy={props.busy}
            onRetry={() => props.onRetryUpload?.(upload.id)}
            onDismiss={() => props.onDismissUpload?.(upload.id)}
          />
        ))}
        {sources.map(({ source, displayUrl, fallbackUrl, task }, index) => {
          const url = source.url
          const selectable = url.startsWith('https://')
          const selected = selectedUrlSet.has(url)
          return (
            <PickerMediaCard
              key={url}
              title={source.fileName || t('Character')}
              displaySrc={displayUrl}
              fallbackSrc={fallbackUrl}
              kind='image'
              selected={selected}
              priority={index < 8}
              disabled={
                props.busy || resolving || !selectable || (!selected && isFull)
              }
              unavailableTitle={t('No preview available')}
              onSelect={() => selectSource(source, task, url, selected)}
              onPreview={() => props.onPreviewImage?.(url)}
            />
          )
        })}
        {filter !== 'uploaded' && (props.hasMore || props.loadMoreFailed) ? (
          <div ref={sentinelRef} className='inf-picker-sentinel' aria-hidden />
        ) : null}
      </div>
      {filter !== 'uploaded' && (props.hasMore || props.loadMoreFailed) ? (
        <Button
          type='button'
          variant='ghost'
          className='inf-reference-picker-more'
          disabled={props.isFetchingMore}
          onClick={props.onLoadMore}
        >
          {props.isFetchingMore ? (
            <Loader2 className='size-4 animate-spin' aria-hidden='true' />
          ) : null}
          {props.loadMoreFailed ? t('Retry loading more') : t('Load more')}
        </Button>
      ) : null}
    </ReferencePickerShell>
  )
}
