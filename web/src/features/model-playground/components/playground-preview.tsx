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
import { ArrowExpand01Icon, Download01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'

import { Spinner } from '@/components/ui/spinner'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { cn } from '@/lib/utils'

import type { GenerationTask, PlaygroundParams } from '../types'

export type PlaygroundPreviewProps = {
  task: GenerationTask | null
  params: PlaygroundParams
  view: 'preview' | 'json'
  onViewChange: (view: 'preview' | 'json') => void
}

function requestJson(params: PlaygroundParams, modelId: string): string {
  return JSON.stringify({ model: modelId, ...params }, null, 2)
}

/** Statuses past the point where a result is still coming. */
function isSettled(task: GenerationTask): boolean {
  return (
    task.status === 'succeeded' ||
    task.status === 'failed' ||
    task.status === 'timeout'
  )
}

/**
 * Media stage: the darkroom the result lands in.
 *
 * Video and image nodes keep a dark backdrop in both themes (DESIGN.md §3.1),
 * so the media is never judged against a coloured canvas.
 */
function MediaStage({ media }: { media: GenerationTask['media'] }) {
  const { t } = useTranslation()
  const first = media[0]

  return (
    <div
      data-parity='playground.preview.stage'
      className='bg-media-dark relative overflow-hidden rounded-(--radius-card)'
    >
      {first.kind === 'video' ? (
        <video
          key={first.url}
          src={first.url}
          controls
          autoPlay
          playsInline
          muted
          loop
          className='aspect-video w-full object-contain'
        />
      ) : (
        <img
          src={first.url}
          alt={t('Generated result')}
          className='aspect-video w-full object-contain'
        />
      )}

      <a
        data-parity='playground.preview.expand'
        href={first.url}
        target='_blank'
        rel='noreferrer'
        aria-label={t('View full size')}
        className='absolute top-3 left-3 grid size-9 place-items-center rounded-full bg-black/55 text-white backdrop-blur transition-colors hover:bg-black/75'
      >
        <HugeiconsIcon icon={ArrowExpand01Icon} className='size-4' />
      </a>

      <a
        data-parity='playground.preview.download'
        href={first.url}
        download
        className='bg-background text-foreground absolute right-3 bottom-3 inline-flex items-center gap-2 rounded-full px-4 py-2 text-[14px] font-semibold shadow-lg transition-opacity hover:opacity-90'
      >
        <HugeiconsIcon icon={Download01Icon} className='size-4' />
        {t('Download')}
      </a>
    </div>
  )
}

/**
 * Waiting state: a ring, the percentage as the hero, and the plain word
 * underneath. There is deliberately no linear bar here — the reference shows
 * only the ring and the number.
 */
function ProcessingStage({ progress }: { progress: number }) {
  const { t } = useTranslation()

  return (
    <div
      data-parity='playground.preview.progress'
      className='bg-media-dark flex aspect-video w-full flex-col items-center justify-center gap-3 rounded-(--radius-card)'
      role='status'
      aria-live='polite'
    >
      <Spinner className='size-14 text-white/80' strokeWidth={2} />
      <span className='text-[28px] leading-none font-semibold text-white tabular-nums'>
        {progress}%
      </span>
      <span className='text-[14px] text-white/70'>{t('processing')}</span>
    </div>
  )
}

export function PlaygroundPreview(props: PlaygroundPreviewProps) {
  const { t } = useTranslation()
  const task = props.task

  return (
    <section className='bg-muted/30 flex min-h-[320px] flex-col rounded-(--radius-media) border'>
      <header className='flex items-center justify-between gap-3 border-b px-4 py-3'>
        <ToggleGroup
          data-parity='playground.preview.switch'
          value={[props.view]}
          onValueChange={(values) => {
            const next = values.at(0)
            if (next === 'preview' || next === 'json') props.onViewChange(next)
          }}
          variant='outline'
          size='sm'
          aria-label={t('Preview mode')}
        >
          <ToggleGroupItem value='preview'>{t('Preview')}</ToggleGroupItem>
          <ToggleGroupItem value='json'>{t('JSON')}</ToggleGroupItem>
        </ToggleGroup>
        <span
          data-parity='playground.preview.ready'
          className='text-muted-foreground text-[12px]'
        >
          {t('Preview JSON ready')}
        </span>
      </header>

      <div className='flex-1 p-4'>
        {props.view === 'json' && (
          <pre className='overflow-auto text-[12px] leading-[1.6] whitespace-pre-wrap'>
            {requestJson(props.params, task?.modelId ?? '')}
          </pre>
        )}

        {props.view !== 'json' && task && (
          <div className={cn('space-y-3')}>
            {(() => {
              if (task.media.length > 0)
                {return <MediaStage media={task.media} />}
              if (!isSettled(task))
                {return <ProcessingStage progress={task.progress} />}
              if (!task.failReason) {
                return (
                  <div
                    data-parity='playground.preview.no-media'
                    className='bg-media-dark text-muted-foreground flex aspect-video w-full flex-col items-center justify-center gap-2 rounded-(--radius-card) text-[13px]'
                  >
                    <p>{t('No media produced for this task')}</p>
                  </div>
                )
              }
              return null
            })()}

            {task.failReason && (
              <p className='text-destructive text-[13px]'>{task.failReason}</p>
            )}
          </div>
        )}

        {props.view !== 'json' && !task && (
          <p
            data-parity='playground.preview.empty'
            className='text-muted-foreground flex h-full items-center justify-center text-[13px]'
          >
            {t('No Task Running')}
          </p>
        )}
      </div>
    </section>
  )
}
