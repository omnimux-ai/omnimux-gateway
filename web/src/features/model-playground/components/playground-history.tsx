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
import { Cancel01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import dayjs from '@/lib/dayjs'
import { cn } from '@/lib/utils'

import type { GenerationTask, PlaygroundParams } from '../types'

export type PlaygroundHistoryProps = {
  tasks: GenerationTask[]
  loading: boolean
  selectedTaskId?: string | null
  onSelectTask?: (task: GenerationTask) => void
}

function statusTone(
  status: GenerationTask['status']
): 'default' | 'secondary' | 'destructive' {
  if (status === 'succeeded') return 'default'
  if (status === 'failed' || status === 'timeout') return 'destructive'
  return 'secondary'
}

function extractKeyParams(params: PlaygroundParams): string[] {
  const chips: string[] = []
  if (
    typeof params.aspect_ratio === 'string' &&
    params.aspect_ratio.trim() !== ''
  ) {
    chips.push(params.aspect_ratio)
  } else if (typeof params.size === 'string' && params.size.trim() !== '') {
    chips.push(params.size)
  }
  if (
    params.duration !== undefined &&
    params.duration !== null &&
    String(params.duration) !== ''
  ) {
    chips.push(`${params.duration}s`)
  }
  if (
    typeof params.resolution === 'string' &&
    params.resolution.trim() !== ''
  ) {
    chips.push(params.resolution)
  }
  if (typeof params.quality === 'string' && params.quality.trim() !== '') {
    chips.push(params.quality)
  }
  return chips.slice(0, 3)
}

function CostCell({ task }: { task: GenerationTask }) {
  const { t } = useTranslation()
  if (task.status === 'running' || task.status === 'queued') {
    return (
      <div className='text-right tabular-nums'>
        <span className='text-[12px] font-semibold text-amber-500'>
          {t('Pending settlement')}
        </span>
      </div>
    )
  }
  if (task.settledUsd !== null && typeof task.settledUsd === 'number') {
    const usd = task.settledUsd
    const credits = usd * 10
    return (
      <div className='flex flex-col items-end tabular-nums'>
        <span className='text-foreground text-[13px] font-medium'>
          ${usd.toFixed(4)}
        </span>
        <span className='text-muted-foreground text-[11px]'>
          {credits.toFixed(2)} {t('credits')}
        </span>
      </div>
    )
  }
  if (task.estimatedUsd !== null && typeof task.estimatedUsd === 'number') {
    const usd = task.estimatedUsd
    const credits = usd * 10
    return (
      <div className='flex flex-col items-end tabular-nums'>
        <span className='text-muted-foreground text-[13px]'>
          ~${usd.toFixed(4)}
        </span>
        <span className='text-muted-foreground text-[11px]'>
          ~{credits.toFixed(2)} {t('credits')}
        </span>
      </div>
    )
  }
  return (
    <div className='text-muted-foreground text-right text-[12px] tabular-nums'>
      —
    </div>
  )
}

function TaskMediaThumbnail({ media }: { media: GenerationTask['media'] }) {
  const [loadError, setLoadError] = useState(false)
  const first = media[0]
  if (!first || loadError) {
    return (
      <span className='bg-muted text-muted-foreground border-border/40 flex size-10 shrink-0 items-center justify-center rounded-lg border text-[11px] font-medium select-none'>
        AI
      </span>
    )
  }
  if (first.kind === 'image') {
    return (
      <span className='bg-muted/40 border-border/40 relative block size-10 shrink-0 overflow-hidden rounded-lg border'>
        <img
          src={first.url}
          alt=''
          width={40}
          height={40}
          loading='lazy'
          decoding='async'
          className='size-full object-cover transition-opacity duration-200'
          onError={() => setLoadError(true)}
        />
      </span>
    )
  }
  const videoSrc = first.url.includes('#') ? first.url : `${first.url}#t=0.001`
  return (
    <span className='bg-media-dark border-border/40 relative block size-10 shrink-0 overflow-hidden rounded-lg border'>
      <video
        src={videoSrc}
        preload='metadata'
        muted
        playsInline
        className='pointer-events-none size-full object-cover'
        onError={() => setLoadError(true)}
      />
      <span className='absolute right-1 bottom-1 flex size-3.5 items-center justify-center rounded bg-black/60 text-[8px] text-white backdrop-blur'>
        ▶
      </span>
    </span>
  )
}

export function PlaygroundHistory(props: PlaygroundHistoryProps) {
  const { t } = useTranslation()
  const [dismissedIds, setDismissedIds] = useState<Set<string>>(() => new Set())

  const displayedTasks = props.tasks.filter(
    (task) => !dismissedIds.has(task.taskId)
  )

  const handleDismiss = (taskId: string) => {
    setDismissedIds((prev) => {
      const next = new Set(prev)
      next.add(taskId)
      return next
    })
  }

  return (
    <section className='space-y-3'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <h3
          data-parity='playground.history.title'
          className='text-lg font-semibold'
        >
          {t('History')}
        </h3>
        <p
          data-parity='playground.history.retention-ours'
          className='text-muted-foreground text-[12px]'
        >
          {t(
            'Generated results are kept long term; download a copy you want to keep.'
          )}
        </p>
      </div>

      {displayedTasks.length === 0 ? (
        <p
          data-parity='playground.history.empty'
          className='text-muted-foreground rounded-(--radius-card) border border-dashed px-4 py-6 text-center text-[13px]'
        >
          {t('Generation history will appear here')}
        </p>
      ) : (
        <div className='bg-card/30 flex flex-col overflow-hidden rounded-(--radius-media) border'>
          <div
            data-parity='playground.history.scroll-area'
            className='max-h-[680px] overflow-auto'
          >
            <table className='w-full border-collapse text-left text-[13px]'>
              <thead className='bg-muted/95 text-muted-foreground sticky top-0 z-10 border-b text-[12px] font-medium shadow-xs backdrop-blur-xs'>
                <tr>
                  <th className='px-4 py-2.5'>{t('Content')}</th>
                  <th className='px-4 py-2.5'>{t('Params')}</th>
                  <th className='px-4 py-2.5'>{t('Status')}</th>
                  <th className='px-4 py-2.5'>{t('Completed')}</th>
                  <th className='px-4 py-2.5 text-right'>{t('Cost')}</th>
                  <th className='w-10 px-2 py-2.5' />
                </tr>
              </thead>
              <tbody className='divide-y'>
                {displayedTasks.map((task) => {
                  const promptText =
                    typeof task.params.prompt === 'string' &&
                    task.params.prompt.trim() !== ''
                      ? task.params.prompt
                      : task.modelId
                  const time = task.finishedAt
                    ? task.finishedAt
                    : task.createdAt
                  const keyParams = extractKeyParams(task.params)

                  const isSelected = props.selectedTaskId === task.taskId

                  return (
                    <tr
                      key={task.taskId}
                      data-task-id={task.taskId}
                      data-selected={isSelected ? 'true' : 'false'}
                      onClick={() => props.onSelectTask?.(task)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' || event.key === ' ') {
                          event.preventDefault()
                          props.onSelectTask?.(task)
                        }
                      }}
                      tabIndex={props.onSelectTask ? 0 : undefined}
                      role={props.onSelectTask ? 'button' : undefined}
                      aria-selected={isSelected}
                      className={cn(
                        'transition-colors',
                        props.onSelectTask && 'cursor-pointer',
                        isSelected
                          ? 'bg-muted/70 font-medium'
                          : 'hover:bg-muted/20'
                      )}
                    >
                      <td className='px-4 py-3'>
                        <div className='flex items-center gap-3'>
                          <TaskMediaThumbnail media={task.media} />
                          <div className='max-w-[200px] min-w-0 sm:max-w-[260px]'>
                            <p
                              className='text-foreground truncate font-medium'
                              title={promptText}
                            >
                              {promptText}
                            </p>
                            <p className='text-muted-foreground truncate font-mono text-[11px]'>
                              {task.taskId}
                            </p>
                          </div>
                        </div>
                      </td>
                      <td className='px-4 py-3'>
                        <div className='flex flex-wrap items-center gap-1.5'>
                          {keyParams.length > 0 ? (
                            keyParams.map((chip) => (
                              <span
                                key={`${task.taskId}-${chip}`}
                                className='bg-muted/60 text-muted-foreground rounded px-1.5 py-0.5 text-[11px] font-medium tabular-nums'
                              >
                                {chip}
                              </span>
                            ))
                          ) : (
                            <span className='text-muted-foreground text-[12px]'>
                              —
                            </span>
                          )}
                        </div>
                      </td>
                      <td className='px-4 py-3'>
                        <Badge
                          variant={statusTone(task.status)}
                          className='text-[11px] font-medium capitalize'
                        >
                          {task.status}
                        </Badge>
                      </td>
                      <td className='text-muted-foreground px-4 py-3 text-[12px] whitespace-nowrap tabular-nums'>
                        {time
                          ? dayjs.unix(time).format('YYYY-MM-DD HH:mm:ss')
                          : '—'}
                      </td>
                      <td className='px-4 py-3'>
                        <CostCell task={task} />
                      </td>
                      <td className='w-10 px-2 py-3 text-center'>
                        <button
                          type='button'
                          onClick={(event) => {
                            event.stopPropagation()
                            handleDismiss(task.taskId)
                          }}
                          aria-label={t('Dismiss')}
                          className='text-muted-foreground hover:text-foreground hover:bg-muted inline-flex size-6 items-center justify-center rounded transition-colors'
                        >
                          <HugeiconsIcon
                            icon={Cancel01Icon}
                            className='size-3.5'
                          />
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
          <footer className='text-muted-foreground bg-muted/20 flex shrink-0 items-center justify-between border-t px-4 py-2.5 text-[12px]'>
            <span>
              {t('{{count}} local records', { count: displayedTasks.length })}
            </span>
            <a
              href='/panel/tasks'
              className='text-foreground font-medium transition-colors hover:underline'
            >
              {t('View all records')}
            </a>
          </footer>
        </div>
      )}
    </section>
  )
}
