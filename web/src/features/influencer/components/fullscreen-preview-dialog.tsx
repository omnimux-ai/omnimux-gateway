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

import { ChevronLeft, X } from 'lucide-react'
import { useRef, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogTitle,
} from '@/components/ui/dialog'
import { cn } from '@/lib/utils'

export interface FullscreenPreviewDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: ReactNode
  backgroundUrl?: string
  media: ReactNode
  children?: ReactNode
  actions?: ReactNode
  mediaFirst?: boolean
  /** Present while a nested reference is shown; returns one level. */
  onBack?: () => void
}

/** Shared full-screen media layout; the centred business dialog cannot provide a side rail. */
export function FullscreenPreviewDialog(props: FullscreenPreviewDialogProps) {
  const { t } = useTranslation()
  const popupRef = useRef<HTMLDivElement>(null)
  return (
    <Dialog open={props.open} onOpenChange={(open) => props.onOpenChange(open)}>
      <DialogContent
        showCloseButton={false}
        ref={popupRef}
        initialFocus={props.mediaFirst ? popupRef : undefined}
        className='influencer-page inset-0 top-0 left-0 h-dvh max-h-dvh w-screen max-w-none translate-x-0 translate-y-0 gap-0 overflow-hidden rounded-none bg-black/80 p-0 ring-0 outline-none sm:max-w-none'
      >
        {props.backgroundUrl ? (
          <img
            src={props.backgroundUrl}
            alt=''
            aria-hidden='true'
            className={cn(
              'pointer-events-none absolute inset-0 size-full scale-110 object-cover blur-2xl',
              props.mediaFirst && 'opacity-50'
            )}
          />
        ) : null}
        <div className='relative grid size-full grid-rows-[max-content_max-content] overflow-y-auto md:grid-cols-[1fr_368px] md:grid-rows-1 md:overflow-hidden'>
          <aside
            className={cn(
              'order-1 min-h-0 min-w-0 p-2 md:order-2',
              props.mediaFirst && 'max-md:order-2'
            )}
          >
            <div
              className={cn(
                'flex size-full min-h-0 flex-col gap-2 overflow-hidden rounded-3xl border border-white/5 bg-zinc-900/75 p-2 backdrop-blur-2xl',
                props.mediaFirst && 'max-md:backdrop-blur-none'
              )}
            >
              <header className='flex shrink-0 items-center gap-2 px-1 pt-1'>
                <DialogTitle className='min-w-0 flex-1 truncate text-lg font-semibold tracking-[-0.01em] text-white'>
                  {props.title}
                </DialogTitle>
                <DialogClose
                  render={
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon-sm'
                      aria-label={t('Close')}
                      className={cn(
                        'shrink-0 rounded-full bg-white/5 text-white hover:bg-white/10 hover:text-white',
                        props.mediaFirst &&
                          'max-md:fixed max-md:top-4 max-md:right-4 max-md:z-10 max-md:size-10 max-md:bg-zinc-900/80'
                      )}
                    />
                  }
                >
                  <X className='size-4' aria-hidden='true' />
                </DialogClose>
              </header>
              <div className='flex min-h-0 flex-col gap-2 md:flex-1 md:overflow-y-auto md:overscroll-contain'>
                {props.children}
              </div>
              {props.actions ? (
                <footer className='mt-auto flex shrink-0 flex-col gap-2 pt-2'>
                  {props.actions}
                </footer>
              ) : null}
            </div>
          </aside>
          <div
            className={cn(
              'relative order-2 min-h-0 min-w-0 p-3 max-md:h-[52dvh] md:order-1',
              props.mediaFirst && 'max-md:order-1 max-md:h-[65dvh]'
            )}
          >
            {props.media}
            {props.onBack ? (
              <Button
                type='button'
                variant='ghost'
                onClick={props.onBack}
                className={cn(
                  'absolute top-4 left-4 z-10 h-10 gap-1 rounded-full border border-white/10 bg-zinc-900/80 pr-4 pl-3 text-white backdrop-blur-md hover:bg-zinc-800 hover:text-white',
                  // The media area scrolls with the page on compact screens.
                  props.mediaFirst && 'max-md:fixed max-md:top-4 max-md:left-4'
                )}
              >
                <ChevronLeft className='size-4' aria-hidden='true' />
                {t('Back')}
              </Button>
            ) : null}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  )
}
