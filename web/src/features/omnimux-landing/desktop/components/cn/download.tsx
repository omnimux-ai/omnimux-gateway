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
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from 'react'

import { useCopy } from '../../i18n'
import { detectMacArch, detectPlatform } from './detect-platform'
import { DOWNLOAD_MAC, DOWNLOAD_WIN } from './data'
import {
  DownloadContext,
  type DetectedPlatform,
  type DownloadTarget,
  type MacArch,
} from './download-context'
import { Icon } from './icons'

export function DownloadProvider(props: { children: ReactNode }) {
  const dialogRef = useRef<HTMLDialogElement>(null)
  const [recommended, setRecommended] = useState<DownloadTarget>('mac')
  const [platform, setPlatform] = useState<DetectedPlatform>(null)
  const [macArch, setMacArch] = useState<MacArch | null>(null)

  // Detection touches `navigator`, so it runs after mount to keep the server
  // and first client render identical.
  useEffect(() => setPlatform(detectPlatform()), [])
  useEffect(() => {
    let stale = false
    void detectMacArch().then((arch) => {
      if (!stale) setMacArch(arch)
    })
    return () => {
      stale = true
    }
  }, [])

  const open = useCallback((target?: DownloadTarget) => {
    setRecommended(target ?? detectPlatform() ?? 'mac')
    const dialog = dialogRef.current
    if (dialog && !dialog.open) dialog.showModal()
  }, [])

  const downloadDirect = useCallback(() => {
    if (!platform) return false
    const href = platform === 'win' ? DOWNLOAD_WIN : DOWNLOAD_MAC
    const anchor = document.createElement('a')
    anchor.href = href
    anchor.download = ''
    anchor.rel = 'noopener'
    document.body.appendChild(anchor)
    anchor.click()
    anchor.remove()
    return true
  }, [platform])

  const value = useMemo(
    () => ({ open, platform, macArch, downloadDirect }),
    [open, platform, macArch, downloadDirect]
  )

  return (
    <DownloadContext.Provider value={value}>
      {props.children}
      <DownloadDialog dialogRef={dialogRef} recommended={recommended} />
    </DownloadContext.Provider>
  )
}

function DownloadDialog(props: {
  dialogRef: RefObject<HTMLDialogElement | null>
  recommended: DownloadTarget
}) {
  const { dialogRef, recommended } = props
  const copy = useCopy().download
  return (
    <dialog
      className='dl'
      id='dl'
      ref={dialogRef}
      aria-labelledby='dlTitle'
      onClick={(event) => {
        if (event.target === dialogRef.current) dialogRef.current?.close()
      }}
    >
      <div className='dl-top'>
        <img src='/logo.svg' alt='' />
        <div>
          <h2 id='dlTitle'>{copy.title}</h2>
          <p>{copy.desc}</p>
        </div>
        <button
          className='dl-x'
          type='button'
          aria-label={copy.close}
          onClick={() => dialogRef.current?.close()}
        >
          <Icon name='x' />
        </button>
      </div>
      <div className='dl-opts'>
        <a
          className={`dl-opt${recommended === 'mac' ? ' rec' : ''}`}
          href={DOWNLOAD_MAC}
        >
          <Icon name='apple' filled />
          <div>
            <b>{copy.macTitle}</b>
            <span>{copy.macSub}</span>
          </div>
          <em>{copy.recommended}</em>
        </a>
        <a
          className={`dl-opt${recommended === 'win' ? ' rec' : ''}`}
          href={DOWNLOAD_WIN}
        >
          <Icon name='win' filled />
          <div>
            <b>{copy.winTitle}</b>
            <span>{copy.winSub}</span>
          </div>
          <em>{copy.recommended}</em>
        </a>
      </div>
      <p className='dl-note'>
        {copy.notePre} <b>{copy.noteMore}</b> → <b>{copy.noteRun}</b>
        {copy.notePost}
      </p>
    </dialog>
  )
}
