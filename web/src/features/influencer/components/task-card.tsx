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
import { Download, Video } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { describeFailReason } from '../lib/fail-reason'
import {
  HISTORY_GRID_RATIO,
  taskImageUrl,
  taskCardImageUrl,
  taskBrowserMediaUrl,
  taskKind,
  taskVideoUrl,
} from '../lib/history'
import { multiViewProgress, multiViewState } from '../lib/multiview'
import { hasUnverifiedResult } from '../lib/task-status'
import type { TaskRecord } from '../types'
import { FailReasonDetail } from './fail-reason-detail'
import { MultiViewBadge } from './multi-view-badge'
import { TaskMediaPreview } from './task-media-preview'

interface Props {
  task: TaskRecord
  /** The board derived from this card, when one has been requested. */
  child: TaskRecord | null
  onView: (t: TaskRecord) => void
  onReloadPreview?: () => void
  onTurnToMotion?: (task: TaskRecord) => void
  turnToMotionDisabledReason?: (task: TaskRecord) => string | undefined
  onOpenMultiView?: (parent: TaskRecord, child: TaskRecord) => void
}

function statusOf(t: TaskRecord): 'q' | 's' | 'p' | 'f' | 'c' {
  const s = (t.status ?? '').toUpperCase()
  if (s === 'SUCCESS' || (t.status ?? '').toLowerCase() === 'succeeded') {
    return 'c'
  }
  if (s === 'FAILURE' || (t.status ?? '').toLowerCase() === 'failed') return 'f'
  if (
    s === 'WAITING' ||
    s === 'QUEUED' ||
    s === 'SUBMITTED' ||
    s === 'NOT_START'
  ) {
    return 'q'
  }
  if (s === 'CLAIMED' || s === 'DISPATCHING') return 's'
  return 'p'
}

export function TaskCard(props: Props) {
  const { t } = useTranslation()
  const [playing, setPlaying] = useState(false)

  const task = props.task
  const st = statusOf(task)
  const failReason = describeFailReason(t, task.fail_reason)
  const resultUnverified = hasUnverifiedResult(task)
  const inReview = resultUnverified || (st === 's' && Boolean(task.fail_reason))
  let activityLabel = t('Generating…')
  if (st === 'q') {
    activityLabel =
      (task.status ?? '').toUpperCase() === 'WAITING'
        ? t('Waiting for a free slot')
        : t('Queued')
  }
  if (st === 's') {
    activityLabel = inReview ? t('Needs review') : t('Submitting…')
  }
  if (inReview) activityLabel = t('Needs review')
  const kind = taskKind(task)
  const isVideo = kind === 'motion'
  const sourceUrl = isVideo ? taskVideoUrl(task) : taskImageUrl(task)
  const img = isVideo ? null : sourceUrl
  // The gateway's stored cover wins: it is already on our own storage, while
  // the artifact image is whatever the provider happened to return.
  const poster =
    isVideo && task.poster_url ? taskBrowserMediaUrl(task.poster_url) : null
  const progress = Number.parseInt(task.progress ?? '0', 10) || 0
  const frameRatio = HISTORY_GRID_RATIO
  // 无论图片还是视频成片，均支持点击卡片放大打开预览弹窗
  const clickable = st === 'c'
  const mediaUrl = sourceUrl
  const board = props.child
  const openBoard = props.onOpenMultiView

  const isTouch =
    typeof window !== 'undefined' && window.matchMedia('(hover: none)').matches

  return (
    <div
      role={clickable ? 'button' : undefined}
      tabIndex={clickable ? 0 : undefined}
      onClick={clickable ? () => props.onView(task) : undefined}
      onMouseEnter={() => {
        if (isVideo && !isTouch) setPlaying(true)
      }}
      onMouseLeave={() => {
        if (isVideo) setPlaying(false)
      }}
      onFocusCapture={() => {
        if (isVideo && !isTouch) setPlaying(true)
      }}
      onBlurCapture={() => {
        if (isVideo) setPlaying(false)
      }}
      onKeyDown={
        clickable
          ? (e) => {
              if (e.target !== e.currentTarget) return
              if (e.key === 'Enter' || e.key === ' ') {
                e.preventDefault()
                props.onView(task)
              }
            }
          : undefined
      }
      className={cn(
        'inf-card group relative overflow-hidden rounded-none bg-[var(--inf-surface-2)] transition-all duration-150',
        'flex w-full flex-col self-start',
        clickable && 'cursor-pointer'
      )}
    >
      {/* 所有历史媒体完整包含在固定网格画幅中。 */}
      {st === 'c' && (
        <div
          className='inf-history-media relative aspect-[9/16] w-full overflow-hidden'
          style={{ aspectRatio: frameRatio }}
        >
          <TaskMediaPreview
            identity={task.task_id || String(task.id)}
            src={isVideo ? sourceUrl : taskCardImageUrl(task)}
            fallbackSrc={isVideo ? undefined : sourceUrl}
            poster={poster}
            video={isVideo}
            playing={playing}
            onReloadRecord={props.onReloadPreview}
          />

          {/* 悬停浮出的下载按钮：深色半透明圆钮压在媒体右上角，浅色画面也可见 */}
          {mediaUrl ? (
            <a
              href={taskBrowserMediaUrl(mediaUrl, true)}
              download
              target='_blank'
              rel='noreferrer'
              aria-label={t('Download')}
              title={t('Download')}
              onClick={(e) => e.stopPropagation()}
              className='inf-task-download absolute top-3 right-3 grid h-8 w-8 place-items-center rounded-full bg-black/60 text-white opacity-0 backdrop-blur-sm transition-opacity duration-150 group-hover:opacity-100 hover:bg-black/80 focus-visible:opacity-100 [@media(hover:none)]:opacity-100'
            >
              <Download className='h-4 w-4' aria-hidden='true' />
            </a>
          ) : null}

          {!isVideo && img && props.onTurnToMotion ? (
            <Button
              className='inf-turn-to-motion absolute right-3 bottom-3 h-9 max-w-[calc(100%-24px)] gap-2 rounded-full px-4 text-sm'
              disabled={Boolean(props.turnToMotionDisabledReason?.(task))}
              title={props.turnToMotionDisabledReason?.(task)}
              onClick={(event) => {
                event.stopPropagation()
                props.onTurnToMotion?.(task)
              }}
            >
              <Video aria-hidden='true' />
              {t('Turn to motion')}
            </Button>
          ) : null}

          {/* 多视角缩略图：贴在主图内侧左下角，1:1 方格，随派生任务状态变化 */}
          {board && openBoard ? (
            <div className='absolute bottom-3 left-3'>
              <MultiViewBadge
                key={JSON.stringify([
                  board.task_id || board.id,
                  taskCardImageUrl(board),
                  taskImageUrl(board),
                ])}
                state={multiViewState(board)}
                progress={multiViewProgress(board)}
                thumbnail={taskCardImageUrl(board)}
                original={taskImageUrl(board)}
                onOpen={() => openBoard(task, board)}
              />
            </div>
          ) : null}
        </div>
      )}

      {/* 生成中或排队中状态：极简进度条展示 */}
      {(st === 'p' || st === 'q' || st === 's') && (
        <div
          className='relative flex aspect-[9/16] w-full flex-col items-center justify-center bg-[var(--inf-surface)] p-3'
          style={{ aspectRatio: frameRatio }}
        >
          {inReview ? null : <div className='inf-shimmer' />}
          <div className='inf-status q'>{activityLabel}</div>
          {inReview ? (
            <p className='mt-2 max-w-full text-center text-xs break-words text-[var(--inf-txt-2)]'>
              {resultUnverified
                ? t('Checking the original result. Do not submit again.')
                : t(
                    'Confirming the result. Unconfirmed tasks fail automatically within about 60 minutes and are never resent.'
                  )}
            </p>
          ) : (
            <div className='inf-progress'>
              <i style={{ width: `${Math.min(progress, 100)}%` }} />
            </div>
          )}
        </div>
      )}

      {/* 失败状态：默认仅留一枚断链图标的深色渐变占位，悬停或键盘聚焦时揭示具体原因 */}
      {st === 'f' && (
        <div
          className='inf-fail-card relative flex aspect-[9/16] min-h-0 w-full flex-col items-center justify-center overflow-hidden p-4 text-center'
          style={{ aspectRatio: frameRatio }}
        >
          <div className='inf-fail-idle'>
            <div className='inf-fail-emblem' aria-hidden='true'>
              <svg viewBox='0 0 24 24' fill='none'>
                <path
                  d='M19.9898 3.99115C18.4371 2.46996 16.3085 2.41193 15.2251 3.4733L11.5251 7.09827C11.2292 7.38815 10.7543 7.38328 10.4645 7.0874C10.1746 6.79152 10.1795 6.31667 10.4753 6.02679L14.1754 2.40182C16.0113 0.603153 19.0891 1.00883 21.0395 2.91967C22.9927 4.8332 23.4179 7.87522 21.5681 9.68753L18.6609 12.5357C18.365 12.8256 17.8902 12.8208 17.6003 12.5249C17.3104 12.229 17.3153 11.7542 17.6112 11.4643L20.5184 8.61605C21.5878 7.56832 21.5398 5.50967 19.9898 3.99115Z'
                  fill='currentColor'
                />
                <path
                  d='M6.62424 3.58399C6.39448 3.23934 5.92882 3.14621 5.58418 3.37597C5.23953 3.60574 5.1464 4.07139 5.37617 4.41604L7.37617 7.41604C7.60593 7.76068 8.07158 7.85381 8.41623 7.62405C8.76088 7.39429 8.85401 6.92863 8.62424 6.58399L6.62424 3.58399Z'
                  fill='currentColor'
                />
                <path
                  d='M2.23737 7.2885C1.84442 7.15751 1.41968 7.36988 1.28869 7.76284C1.15771 8.1558 1.37008 8.58054 1.76303 8.71152L7.76303 10.7115C8.15599 10.8425 8.58073 10.6301 8.71172 10.2372C8.8427 9.84422 8.63033 9.41949 8.23737 9.2885L2.23737 7.2885Z'
                  fill='currentColor'
                />
                <path
                  d='M6.72775 12.506C7.00721 12.2003 6.98589 11.7259 6.68015 11.4464C6.37441 11.167 5.90001 11.1883 5.62056 11.494L4.3597 12.8735C2.63286 14.7628 3.01004 17.9268 4.86404 19.9552C6.72921 21.9958 9.73449 22.4595 11.5191 20.507L15.5538 16.0926C15.8333 15.7869 15.8119 15.3125 15.5062 15.033C15.2004 14.7536 14.7261 14.7749 14.4466 15.0806L10.4119 19.495C9.41099 20.59 7.44859 20.5596 5.97124 18.9432C4.48273 17.3147 4.4083 15.0437 5.4669 13.8855L6.72775 12.506Z'
                  fill='currentColor'
                />
              </svg>
            </div>
          </div>

          <div className='inf-fail-detail'>
            <div className='inf-fail-detail-label'>
              {t('Why this take paused')}
            </div>
            <div className='inf-fail-reason'>{failReason.message}</div>
            <FailReasonDetail
              view={failReason}
              className='items-center'
              rawClassName='max-h-20'
            />
          </div>
        </div>
      )}
    </div>
  )
}
