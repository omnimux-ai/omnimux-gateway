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
import { useEffect, useId, useRef, useState } from 'react'

import { trackDownloadCtaClick } from '@/lib/download-cta'

import drinkPoster from '../../assets/cn/posters/drink_ad_right.jpg'
import selfiePoster from '../../assets/cn/posters/selfie.jpg'
import serumPoster from '../../assets/cn/posters/serum_main.jpg'
import { useLanding } from '../../i18n/context'
import type { LandingCopy, LandingLang } from '../../i18n/types'
import {
  HOME_DESKTOP_DOWNLOAD_PAGE,
  trackHomeDesktopCopy,
  trackHomeHeroCtaClick,
} from '../../lib/home-analytics'
import { SOURCE_REPO } from './data'
import { releaseUrls, type ReleaseUrls } from './desktop-release'
import { isMobileDevice } from './detect-platform'
import { useDownload, type DownloadTarget } from './download-context'
import { Icon } from './icons'

const SPACED_H1_LANGS: ReadonlySet<LandingLang> = new Set(['en', 'es', 'id'])

const INSTALLERS = {
  'mac-arm64': {
    urlKey: 'arm64',
    labelKey: 'dlMacArm',
    ctaId: 'hero_download_mac_arm64',
  },
  'mac-x64': {
    urlKey: 'x64',
    labelKey: 'dlMacIntel',
    ctaId: 'hero_download_mac_x64',
  },
  win: { urlKey: 'win', labelKey: 'dlWinX64', ctaId: 'hero_download_win' },
} as const satisfies Record<
  DownloadTarget,
  {
    urlKey: keyof ReleaseUrls
    labelKey: keyof LandingCopy['hero']
    ctaId: string
  }
>

const INSTALLER_TARGETS: readonly DownloadTarget[] = [
  'mac-arm64',
  'mac-x64',
  'win',
]

const PREVIEW_SIDE_ICONS = [
  'plus',
  'apps',
  'box',
  'folder',
  'bulb',
  'send',
  'flow',
  'cut',
] as const

const PREVIEW_POSTERS = [selfiePoster, serumPoster, drinkPoster] as const

type HeroCopy = LandingCopy['hero']

export function CnHero() {
  const { platform, release } = useDownload()
  const { copy: landingCopy, lang } = useLanding()
  const copy = landingCopy.hero
  const [mobileEnvironment, setMobileEnvironment] = useState<boolean | null>(
    null
  )
  const [copied, setCopied] = useState(false)
  const mobileHintId = useId()

  useEffect(() => {
    const checkEnvironment = () => {
      setMobileEnvironment(isMobileDevice() || window.innerWidth <= 820)
    }
    checkEnvironment()
    window.addEventListener('resize', checkEnvironment)
    return () => window.removeEventListener('resize', checkEnvironment)
  }, [])

  const mobile = platform === null && mobileEnvironment === true
  const target = platform ?? 'mac-arm64'

  return (
    <section className='hero' id='top'>
      <div className='hero-glow' aria-hidden='true' />
      <div className='wrap'>
        <p className='hero-eyebrow'>
          <span aria-hidden='true' />
          {copy.eyebrow}
        </p>
        <h1 className='h1'>
          <span>{copy.titleA}</span>
          {SPACED_H1_LANGS.has(lang) ? ' ' : null}
          <span>{copy.titleB}</span>
        </h1>
        <p className='hero-sub'>
          {copy.subPre}
          <span>{copy.subHighlight}</span>
          {copy.subPost}
        </p>

        <div className='hero-dl'>
          {mobile ? (
            <button
              className='btn btn-primary'
              type='button'
              aria-describedby={mobileHintId}
              onClick={async () => {
                trackHomeHeroCtaClick('copy_link', 'mobile_fallback')
                trackHomeDesktopCopy(lang)
                try {
                  await navigator.clipboard.writeText(
                    HOME_DESKTOP_DOWNLOAD_PAGE
                  )
                  setCopied(true)
                } catch {
                  setCopied(false)
                  window.prompt(copy.mobileHint, HOME_DESKTOP_DOWNLOAD_PAGE)
                }
              }}
            >
              <Icon name={copied ? 'check' : 'copy'} />
              <span aria-live='polite'>
                {copied ? copy.mobileCopiedLabel : copy.mobileCopyLabel}
              </span>
            </button>
          ) : (
            <HeroInstaller
              copy={copy}
              lang={lang}
              target={target}
              urls={mobileEnvironment === null ? null : releaseUrls(release)}
              loading={mobileEnvironment === null}
            />
          )}

          <a
            className='btn btn-ghost'
            href='#workflow'
            onClick={() =>
              trackHomeHeroCtaClick(
                'scroll_workflow',
                mobile ? 'mobile_fallback' : target
              )
            }
          >
            {copy.secondaryCta}
          </a>
        </div>
        {mobile && (
          <p className='hero-download-note' id={mobileHintId}>
            {copy.mobileHint}
          </p>
        )}
        <p className='hero-download-note'>{copy.microTrust}</p>

        <ul className='hero-trust' aria-label={copy.trustAria}>
          <li>
            <a href={SOURCE_REPO} rel='noopener noreferrer' target='_blank'>
              {copy.trustStrip[0]}
            </a>
          </li>
          {landingCopy.proof.facts.map(({ value, label }) => (
            <li key={label}>
              {value} {label}
            </li>
          ))}
          <li>{copy.trustStrip[1]}</li>
        </ul>

        <HeroPreview copy={copy} />
      </div>
    </section>
  )
}

function HeroInstaller({
  copy,
  lang,
  target,
  urls,
  loading,
}: {
  copy: HeroCopy
  lang: LandingLang
  target: DownloadTarget
  urls: ReleaseUrls | null
  loading: boolean
}) {
  const [open, setOpen] = useState(false)
  const [placement, setPlacement] = useState<'down' | 'up'>('down')
  const menuId = useId()
  const splitRef = useRef<HTMLDivElement>(null)
  const caretRef = useRef<HTMLButtonElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)
  const initialFocus = useRef<'first' | 'last'>('first')
  const menuOpen = open && urls !== null
  const href = urls?.[INSTALLERS[target].urlKey]

  useEffect(() => {
    if (!menuOpen) return
    const updatePlacement = () => {
      const wrap = splitRef.current
      if (!wrap) return
      const { top, bottom } = wrap.getBoundingClientRect()
      const menuHeight = menuRef.current?.offsetHeight || 132
      const spaceBelow = window.innerHeight - bottom
      setPlacement(
        spaceBelow < menuHeight + 16 && top > spaceBelow ? 'up' : 'down'
      )
    }
    const onPointerDown = (event: PointerEvent) => {
      if (!splitRef.current?.contains(event.target as Node)) setOpen(false)
    }
    updatePlacement()
    const items = [...(menuRef.current?.querySelectorAll('a') ?? [])]
    items.at(initialFocus.current === 'last' ? -1 : 0)?.focus()

    document.addEventListener('pointerdown', onPointerDown)
    window.addEventListener('scroll', updatePlacement, { passive: true })
    window.addEventListener('resize', updatePlacement)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      window.removeEventListener('scroll', updatePlacement)
      window.removeEventListener('resize', updatePlacement)
    }
  }, [menuOpen])

  return (
    <div
      className={`split-dl${menuOpen ? ' open' : ''}${placement === 'up' ? ' up' : ''}${!urls ? ' is-disabled' : ''}`}
      ref={splitRef}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false)
      }}
      onKeyDown={(event) => {
        if (event.key === 'Escape' && menuOpen) {
          event.preventDefault()
          setOpen(false)
          caretRef.current?.focus()
        }
      }}
    >
      <a
        className='split-main'
        href={href}
        aria-disabled={!href || undefined}
        title={!href && !loading ? copy.dlUnavailable : undefined}
        onClick={() => {
          if (!href) return
          trackHomeHeroCtaClick('download', target)
          trackDownloadCtaClick(
            {
              page: 'home',
              locale: lang,
              cta_id: 'hero_download_primary',
              cta_position: 'hero',
              cta_variant: 'primary',
            },
            href
          )
        }}
      >
        <Icon name='dl' />
        {target === 'win' ? copy.dlWin : copy.dlMac}
      </a>
      <button
        ref={caretRef}
        aria-expanded={menuOpen}
        aria-haspopup='menu'
        aria-controls={menuOpen ? menuId : undefined}
        aria-label={copy.dlMenuAria}
        className='split-caret'
        type='button'
        disabled={!urls}
        onClick={() => {
          initialFocus.current = 'first'
          setOpen(!menuOpen)
        }}
        onKeyDown={(event) => {
          if (event.key !== 'ArrowDown' && event.key !== 'ArrowUp') return
          event.preventDefault()
          initialFocus.current = event.key === 'ArrowUp' ? 'last' : 'first'
          setOpen(true)
        }}
      >
        <svg aria-hidden='true' className='ico' viewBox='0 0 24 24'>
          <path d='m6 9 6 6 6-6' />
        </svg>
      </button>
      {menuOpen && urls && (
        <div
          className='split-menu'
          id={menuId}
          ref={menuRef}
          role='menu'
          aria-label={copy.dlMenuAria}
          onKeyDown={(event) => {
            const items = [...event.currentTarget.querySelectorAll('a')]
            const index = items.indexOf(
              document.activeElement as HTMLAnchorElement
            )
            const last = items.length - 1
            const targets: Record<string, number> = {
              ArrowDown: index < last ? index + 1 : 0,
              ArrowUp: index > 0 ? index - 1 : last,
              Home: 0,
              End: last,
            }
            const next = targets[event.key]
            if (next === undefined) return
            event.preventDefault()
            items[next]?.focus()
          }}
        >
          {INSTALLER_TARGETS.map((option) => {
            const installer = INSTALLERS[option]
            const link = urls[installer.urlKey]
            return (
              <a
                key={option}
                className='split-item'
                href={link}
                role='menuitem'
                tabIndex={-1}
                onClick={() => {
                  trackHomeHeroCtaClick('download', option)
                  trackDownloadCtaClick(
                    {
                      page: 'home',
                      locale: lang,
                      cta_id: installer.ctaId,
                      cta_position: 'hero',
                      cta_variant: 'menu',
                    },
                    link
                  )
                  setOpen(false)
                  caretRef.current?.focus()
                }}
              >
                {copy[installer.labelKey]}
              </a>
            )
          })}
        </div>
      )}
    </div>
  )
}

function HeroPreview({ copy }: { copy: HeroCopy }) {
  return (
    <figure className='win hero-preview' aria-label={copy.mockAria}>
      <div className='win-bar'>
        <div className='dots' aria-hidden='true'>
          <i />
          <i />
          <i />
        </div>
        <div className='win-title'>
          <span>{copy.mockTitle}</span>
        </div>
        <span className='hero-preview-badge'>{copy.mockBadge}</span>
      </div>

      <div className='hero-preview-body'>
        <div className='hero-preview-side'>
          {copy.mockSide.map((label, index) => (
            <div
              key={label}
              className={index === 0 ? 'hero-preview-selected' : undefined}
            >
              {index === 6 && <p>{copy.mockSidebarGroup}</p>}
              <span>
                <Icon name={PREVIEW_SIDE_ICONS[index]} />
                {label}
              </span>
            </div>
          ))}
        </div>

        <div className='hero-preview-conversation'>
          <div className='hero-preview-message'>
            <span aria-hidden='true'>{copy.mockUserAvatar}</span>
            <p>{copy.mockPrompt}</p>
          </div>
          <div className='hero-preview-reference'>
            <Icon name='link' />
            <span>{copy.mockLinkCard}</span>
          </div>
          <ul className='hero-preview-steps'>
            {copy.mockSteps.map((step) => (
              <li key={step}>
                <Icon name='check' />
                {step}
              </li>
            ))}
          </ul>
          <ul className='hero-preview-samples'>
            {PREVIEW_POSTERS.map((poster, index) => (
              <li key={poster}>
                <div className='hero-preview-photo'>
                  <img src={poster} alt='' decoding='async' />
                  <span>15s · 720p</span>
                </div>
                <div className='hero-preview-sample-label'>
                  <b>{copy.mockTags[index]}</b>
                  <span>{copy.mockTileBadge}</span>
                </div>
              </li>
            ))}
          </ul>
        </div>
      </div>

      <figcaption>{copy.mockNote}</figcaption>
    </figure>
  )
}
