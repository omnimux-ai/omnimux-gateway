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

import {
  trackDownloadCtaClick,
  type DownloadCtaIntent,
  type DownloadCtaPage,
} from '@/lib/download-cta'

import { useCopy, useLanding } from '../../i18n/context'
import {
  fetchDesktopRelease,
  releaseUrls,
  type DesktopRelease,
  type ReleaseUrls,
} from './desktop-release'
import { detectPlatform } from './detect-platform'
import {
  DownloadContext,
  type DetectedPlatform,
  type DownloadTarget,
  type MacArch,
} from './download-context'
import { Icon } from './icons'

const DEFAULT_DIALOG_INTENT: DownloadCtaIntent = {
  cta_id: 'download_dialog',
  cta_position: 'footer',
  cta_variant: 'dialog',
}

const DEFAULT_DIRECT_INTENT: DownloadCtaIntent = {
  cta_id: 'download_direct',
  cta_position: 'footer',
  cta_variant: 'direct',
}

function urlFor(urls: ReleaseUrls, target: DownloadTarget): string {
  if (target === 'win') return urls.win
  return target === 'mac-x64' ? urls.x64 : urls.arm64
}

function archOf(platform: DetectedPlatform): MacArch | null {
  if (platform === 'mac-arm64') return 'arm64'
  if (platform === 'mac-x64') return 'x64'
  return null
}

export function DownloadProvider(props: {
  children: ReactNode
  page: DownloadCtaPage
}) {
  const { lang } = useLanding()
  const dialogRef = useRef<HTMLDialogElement>(null)
  const lastIntentRef = useRef<DownloadCtaIntent | null>(null)
  const [recommended, setRecommended] = useState<DownloadTarget>('mac-arm64')
  const [platform, setPlatform] = useState<DetectedPlatform>(null)
  const [release, setRelease] = useState<DesktopRelease>({ status: 'loading' })

  // Detection touches `navigator`, so it runs after mount to keep the server
  // and first client render identical.
  useEffect(() => {
    let stale = false
    void (async () => {
      const next = await detectPlatform()
      if (!stale) setPlatform(next)
    })()
    return () => {
      stale = true
    }
  }, [])

  // Read the stable-channel manifest after mount for the same reason. A 404
  // resolves to `unavailable` (disable the UI); any other failure to `fallback`
  // (point at the `releases/latest/` symlinks).
  useEffect(() => {
    let stale = false
    void (async () => {
      const next = await fetchDesktopRelease()
      if (!stale) setRelease(next)
    })()
    return () => {
      stale = true
    }
  }, [])

  const open = useCallback(
    (target?: DownloadTarget, intent?: DownloadCtaIntent) => {
      lastIntentRef.current = intent ?? DEFAULT_DIALOG_INTENT
      setRecommended(target ?? platform ?? 'mac-arm64')
      const dialog = dialogRef.current
      if (dialog && !dialog.open) dialog.showModal()
    },
    [platform]
  )

  const downloadDirect = useCallback(
    (intent?: DownloadCtaIntent) => {
      if (!platform) return false
      const urls = releaseUrls(release)
      if (!urls) return false
      const href = urlFor(urls, platform)
      trackDownloadCtaClick(
        {
          page: props.page,
          locale: lang,
          ...(intent ?? DEFAULT_DIRECT_INTENT),
        },
        href
      )
      const anchor = document.createElement('a')
      anchor.href = href
      anchor.download = ''
      anchor.rel = 'noopener'
      document.body.appendChild(anchor)
      anchor.click()
      anchor.remove()
      return true
    },
    [platform, release, props.page, lang]
  )

  const macArch = archOf(platform)

  const value = useMemo(
    () => ({ open, platform, macArch, release, downloadDirect }),
    [open, platform, macArch, release, downloadDirect]
  )

  return (
    <DownloadContext.Provider value={value}>
      {props.children}
      <DownloadDialog
        dialogRef={dialogRef}
        recommended={recommended}
        release={release}
        page={props.page}
        locale={lang}
        intentRef={lastIntentRef}
      />
    </DownloadContext.Provider>
  )
}

function DownloadDialog(props: {
  dialogRef: RefObject<HTMLDialogElement | null>
  recommended: DownloadTarget
  release: DesktopRelease
  page: DownloadCtaPage
  locale: string
  intentRef: RefObject<DownloadCtaIntent | null>
}) {
  const { dialogRef, recommended, release } = props
  const copy = useCopy().download
  const urls = releaseUrls(release)
  const version = release.status === 'ready' ? release.version : null

  const options: {
    target: DownloadTarget
    icon: 'apple' | 'win'
    title: string
    sub: string
  }[] = [
    {
      target: 'mac-arm64',
      icon: 'apple',
      title: copy.macArmTitle,
      sub: copy.macArmSub,
    },
    {
      target: 'mac-x64',
      icon: 'apple',
      title: copy.macIntelTitle,
      sub: copy.macIntelSub,
    },
    {
      target: 'win',
      icon: 'win',
      title: copy.winTitle,
      sub: copy.winSub,
    },
  ]

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
          <p>
            {version ? copy.version.replace('{version}', version) : copy.desc}
          </p>
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
        {options.map((option) => {
          const href = urls ? urlFor(urls, option.target) : null
          if (!href) {
            return (
              <span
                key={option.target}
                className='dl-opt dis'
                aria-disabled='true'
                title={copy.unavailable}
              >
                <Icon name={option.icon} filled />
                <div>
                  <b>{option.title}</b>
                  <span>{option.sub}</span>
                </div>
              </span>
            )
          }
          return (
            <a
              key={option.target}
              className={`dl-opt${recommended === option.target ? ' rec' : ''}`}
              href={href}
              onClick={() => {
                const intent = props.intentRef.current ?? DEFAULT_DIALOG_INTENT
                trackDownloadCtaClick(
                  { page: props.page, locale: props.locale, ...intent },
                  href
                )
              }}
            >
              <Icon name={option.icon} filled />
              <div>
                <b>{option.title}</b>
                <span>{option.sub}</span>
              </div>
              <em>{copy.recommended}</em>
            </a>
          )
        })}
      </div>
      <p className='dl-note'>
        {copy.notePre} <b>{copy.noteMore}</b> → <b>{copy.noteRun}</b>
        {copy.notePost}
      </p>
    </dialog>
  )
}
