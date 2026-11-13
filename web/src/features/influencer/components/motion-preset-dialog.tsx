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

import { Download, Info, Wand2 } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import { motionExploreModeLabel } from '../lib/motion-explore'
import type { MotionExplorePreset } from '../types'
import { FullscreenPreviewDialog } from './fullscreen-preview-dialog'
import { PreviewDetails } from './preview-details'
import { TaskMediaPreview } from './task-media-preview'

interface Props {
  preset: MotionExplorePreset | null
  onClose: () => void
  onApply: (preset: MotionExplorePreset) => void
}

export function MotionPresetDialog(props: Props) {
  if (!props.preset) return null
  return (
    <MotionPreview key={props.preset.id} {...props} preset={props.preset} />
  )
}

function MotionPreview(props: Props & { preset: MotionExplorePreset }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const preset = props.preset
  const [downloading, setDownloading] = useState(false)
  const downloadRequest = useRef<AbortController | null>(null)
  useEffect(() => () => downloadRequest.current?.abort(), [])

  const download = async (): Promise<void> => {
    if (downloadRequest.current) return
    const controller = new AbortController()
    downloadRequest.current = controller
    setDownloading(true)
    try {
      const response = await fetch(preset.videoUrl, {
        credentials: 'omit',
        cache: 'no-store',
        signal: AbortSignal.any([
          controller.signal,
          AbortSignal.timeout(30_000),
        ]),
      })
      if (!response.ok) throw new Error('Media download unavailable')
      const blob = await response.blob()
      if (!blob.size || !blob.type.startsWith('video/')) {
        throw new Error('Media download unavailable')
      }
      if (controller.signal.aborted) return
      const url = URL.createObjectURL(blob)
      const link = document.createElement('a')
      link.href = url
      link.download = `${preset.id}.mp4`
      document.body.append(link)
      try {
        link.click()
      } finally {
        link.remove()
        URL.revokeObjectURL(url)
      }
    } catch {
      if (!controller.signal.aborted) {
        toast.error(t('Could not download this video. Please try again.'))
      }
    } finally {
      if (!controller.signal.aborted) setDownloading(false)
      if (downloadRequest.current === controller) downloadRequest.current = null
    }
  }

  return (
    <FullscreenPreviewDialog
      open
      mediaFirst
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
      title={t(preset.title)}
      backgroundUrl={preset.thumbPath}
      media={
        <TaskMediaPreview
          identity={preset.id}
          src={preset.videoUrl}
          poster={preset.thumbPath}
          video
          detail
          muted
          loop
          label={t(preset.title)}
        />
      }
      actions={
        <div className='flex flex-wrap gap-2'>
          <Button
            type='button'
            className='inf-cta h-10 flex-1 basis-auto'
            onClick={() => props.onApply(preset)}
          >
            <Wand2 className='size-4' aria-hidden='true' />
            <span>{t('Use this motion')}</span>
          </Button>
          <Button
            type='button'
            variant='secondary'
            className='h-10 flex-1 basis-auto'
            disabled={downloading}
            aria-busy={downloading}
            onClick={() => void download()}
          >
            <Download className='size-4' aria-hidden='true' />
            <span>{t('Download')}</span>
          </Button>
        </div>
      }
    >
      <PreviewDetails brief={preset.description} briefLabel={t('Prompt')} />
      <section className='flex shrink-0 flex-col gap-3 rounded-2xl border border-white/5 p-3'>
        <header className='flex items-center gap-2 text-xs font-medium text-white/60 uppercase'>
          <Info className='size-4' aria-hidden='true' />
          {t('Details')}
        </header>
        <dl className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-2 text-sm'>
          <dt className='text-white/60'>{t('Mode')}</dt>
          <dd className='min-w-0 text-right text-white/80'>
            {t(motionExploreModeLabel(preset.mode))}
          </dd>
          <dt className='text-white/60'>{t('Video duration')}</dt>
          <dd className='text-right text-white/80'>
            {formatNumber(preset.durationSec, locale)} {t('seconds')}
          </dd>
        </dl>
      </section>
    </FullscreenPreviewDialog>
  )
}
