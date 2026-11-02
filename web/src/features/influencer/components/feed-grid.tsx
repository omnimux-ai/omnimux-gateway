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

import { useEffect, useMemo, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import {
  isTerminalStatus,
  sortTasksByTime,
  taskImageUrl,
  taskKind,
  taskVideoUrl,
} from '../lib/history'
import type { TaskRecord } from '../types'
import { HistorySkeleton } from './history-skeleton'
import { TaskCard } from './task-card'

interface Props {
  tasks: TaskRecord[]
  pending: TaskRecord | TaskRecord[] | null
  /** First load with nothing cached to paint yet. */
  isInitialLoading: boolean
  hasMore: boolean
  isFetchingMore: boolean
  loadMoreFailed: boolean
  onLoadMore: () => void
  onView: (t: TaskRecord) => void
  onReloadPreview?: () => void
  onTurnToMotion?: (task: TaskRecord) => void
  turnToMotionDisabledReason?: (task: TaskRecord) => string | undefined
  /** The board derived from each card, keyed by parent task id. */
  childByParent?: Map<string, TaskRecord>
  onOpenMultiView?: (parent: TaskRecord, child: TaskRecord) => void
}

/** How far below the fold the next page starts loading. */
const LOAD_MORE_MARGIN = '240px 0px'

const GRID_CLASSES = 'inf-history-grid grid w-full'

export function FeedGrid(props: Props) {
  const { t } = useTranslation()
  const sentinelRef = useRef<HTMLDivElement | null>(null)

  // 严格基于任务唯一编号去重
  const items = useMemo(() => {
    const list: TaskRecord[] = []
    const seen = new Set<string | number>()
    const pending = Array.isArray(props.pending)
      ? props.pending
      : [props.pending]
    for (const task of pending) {
      if (!task) continue
      const id = task.task_id || task.id
      if (id && seen.has(id)) continue
      if (id) seen.add(id)
      const stored = props.tasks.find(
        (item) => (item.task_id || item.id) === id
      )
      // The server history owns placeholders once the record becomes readable.
      const preferStored =
        stored && (isTerminalStatus(stored.status) || task.status === 'QUEUED')
      if (preferStored) {
        const storedMedia =
          taskKind(stored) === 'motion'
            ? taskVideoUrl(stored)
            : taskImageUrl(stored)
        const pendingMedia =
          taskKind(task) === 'motion' ? taskVideoUrl(task) : taskImageUrl(task)
        const keepConfirmedPreview =
          stored.status === 'SUCCESS' &&
          task.status === 'SUCCESS' &&
          !storedMedia &&
          Boolean(pendingMedia)
        list.push(
          keepConfirmedPreview
            ? {
                ...stored,
                artifacts: task.artifacts,
                legacy_content_url: task.legacy_content_url,
                result_url: task.result_url,
                poster_url: stored.poster_url ?? task.poster_url,
                thumbnail_url: stored.thumbnail_url ?? task.thumbnail_url,
                thumbnail_width: stored.thumbnail_width ?? task.thumbnail_width,
                thumbnail_height:
                  stored.thumbnail_height ?? task.thumbnail_height,
              }
            : stored
        )
      } else {
        list.push(task)
      }
    }
    for (const task of props.tasks) {
      const id = task.task_id || task.id
      if (id && seen.has(id)) continue
      if (id) seen.add(id)
      list.push(task)
    }
    return sortTasksByTime(list)
  }, [props.pending, props.tasks])

  const canAutoLoad =
    typeof IntersectionObserver !== 'undefined' && typeof window !== 'undefined'
  const loadMore = props.onLoadMore
  const hasMore = props.hasMore

  useEffect(() => {
    if (!hasMore || !canAutoLoad) return
    const node = sentinelRef.current
    if (!node) return
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) loadMore()
      },
      { rootMargin: LOAD_MORE_MARGIN }
    )
    observer.observe(node)
    return () => observer.disconnect()
  }, [canAutoLoad, hasMore, loadMore])

  if (!items.length && props.isInitialLoading) {
    return <HistorySkeleton />
  }

  if (!items.length) {
    return (
      <div className='flex flex-1 flex-col items-center justify-center gap-2 py-24 text-center'>
        <div className='text-sm font-medium text-[var(--inf-txt-2)]'>
          {t('Nothing here yet')}
        </div>
        <div className='text-xs text-[var(--inf-txt-3)]'>
          {t(
            'Pick options on the left and hit Generate, or hit Randomize for a character'
          )}
        </div>
      </div>
    )
  }

  return (
    <div className={GRID_CLASSES}>
      {items.map((task) => (
        <TaskCard
          key={task.task_id || task.id}
          task={task}
          child={
            props.childByParent?.get(String(task.task_id || task.id)) ?? null
          }
          onView={props.onView}
          onReloadPreview={props.onReloadPreview}
          onTurnToMotion={props.onTurnToMotion}
          turnToMotionDisabledReason={props.turnToMotionDisabledReason}
          onOpenMultiView={props.onOpenMultiView}
        />
      ))}

      {/* 分页行：网格模式下横跨所有列 */}
      <div className='col-span-full flex w-full flex-col gap-4'>
        {props.hasMore && (
          <div ref={sentinelRef} className='h-px w-full' aria-hidden='true' />
        )}

        {props.isFetchingMore && (
          <div
            role='status'
            aria-label={t('Loading more…')}
            className='relative h-20 w-full overflow-hidden rounded-none border border-[var(--inf-line)] bg-[var(--inf-surface-2)]'
          >
            <div className='inf-shimmer' />
          </div>
        )}

        {props.loadMoreFailed && (
          <button
            type='button'
            className='inf-abtn self-center'
            onClick={props.onLoadMore}
          >
            {t('Failed to load history')}
          </button>
        )}

        {/* Environments without IntersectionObserver still need a way forward. */}
        {props.hasMore && !canAutoLoad && !props.isFetchingMore && (
          <button
            type='button'
            className='inf-abtn self-center'
            onClick={props.onLoadMore}
          >
            {t('Load more')}
          </button>
        )}

        {!props.hasMore && (
          <div className='py-2 text-center text-[11px] text-[var(--inf-txt-3)]'>
            {t('No more records')}
          </div>
        )}
      </div>
    </div>
  )
}
