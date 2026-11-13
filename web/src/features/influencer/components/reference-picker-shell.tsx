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
import { Loader2, X } from 'lucide-react'
import {
  useEffect,
  useRef,
  type DragEventHandler,
  type ReactNode,
  type RefObject,
} from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { useIsMobile } from '@/hooks/use-mobile'
import { cn } from '@/lib/utils'

import { useReferencePickerPosition } from '../hooks/use-reference-picker-position'

import './character-assets.css'

interface ReferencePickerShellProps {
  open: boolean
  anchorRef?: RefObject<HTMLElement | null>
  title: string
  description?: string
  headerContent?: ReactNode
  busy?: boolean
  busyLabel?: string
  hideBusyStatus?: boolean
  onClose: () => void
  children: ReactNode
  onDrop?: DragEventHandler<HTMLElement>
}

/** Shared desktop panel and mobile dialog for immediate reference selection. */
export function ReferencePickerShell(props: ReferencePickerShellProps) {
  const { t } = useTranslation()
  const isMobile = useIsMobile()
  const panelRef = useRef<HTMLElement>(null)
  const { open, onClose } = props
  useReferencePickerPosition({
    enabled: open && !isMobile,
    panelRef,
    anchorRef: props.anchorRef,
    onClose,
  })

  useEffect(() => {
    if (!open || isMobile) return

    const handlePointerDown = (event: PointerEvent) => {
      const panel = panelRef.current
      const target = event.target
      if (!panel || !(target instanceof Node) || panel.contains(target)) return
      // Portaled media previews own outside interactions until they close.
      if (
        panel.ownerDocument.querySelector(
          '[data-slot="dialog-content"][data-open]'
        )
      ) {
        return
      }
      onClose()
    }

    document.addEventListener('pointerdown', handlePointerDown, true)
    return () =>
      document.removeEventListener('pointerdown', handlePointerDown, true)
  }, [open, onClose, isMobile])

  if (!props.open) return null

  const content = (
    <aside
      ref={panelRef}
      style={
        !isMobile && props.anchorRef ? { visibility: 'hidden' } : undefined
      }
      onDragOver={props.onDrop ? (event) => event.preventDefault() : undefined}
      onDrop={props.onDrop}
      aria-label={props.title}
      aria-busy={Boolean(props.busy)}
      className={cn(
        'inf-character-picker',
        isMobile && '!static !w-full !max-h-none !rounded-none !shadow-none'
      )}
    >
      {(!isMobile || props.headerContent) && (
        <header className='inf-character-picker-header'>
          {props.headerContent || <h3>{props.title}</h3>}
          <Button
            type='button'
            variant='ghost'
            size='icon'
            className='inf-character-icon'
            onClick={props.onClose}
            aria-label={t('Close panel')}
          >
            <X className='size-4' aria-hidden='true' />
          </Button>
        </header>
      )}
      {props.busy && !props.hideBusyStatus && (
        <div role='status' className='inf-reference-picker-status'>
          <Loader2 className='size-4 animate-spin' aria-hidden='true' />
          <span>{props.busyLabel || t('Preparing reference')}</span>
        </div>
      )}
      <div className='inf-character-picker-body'>{props.children}</div>
    </aside>
  )
  if (!isMobile) return content
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
      title={props.title}
      headerClassName={props.headerContent ? 'sr-only' : undefined}
      showCloseButton={!props.headerContent}
      description={props.headerContent ? undefined : props.description}
      contentClassName='influencer-page inf-character-picker-dialog max-sm:gap-2'
      bodyClassName='p-0'
    >
      {content}
    </Dialog>
  )
}
