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
import { useEffect, useRef, useState } from 'react'

import { useCopy } from '../../i18n'
import {
  DOWNLOAD_MAC_ARM64,
  DOWNLOAD_MAC_X64,
  DOWNLOAD_WIN,
  REEL_MEDIA,
  SOURCE_REPO,
  type Mode,
} from './data'
import { useDownload } from './download-context'
import { usePrefersReducedMotion } from './hooks'
import { Icon } from './icons'
import { LazyVideo } from './lazy-video'

const REEL_LAYOUT = [
  { y: '44px', r: '-6deg' },
  { y: '18px', r: '-4deg' },
  { y: '5px', r: '-2deg' },
  { y: '0px', r: '0deg' },
  { y: '5px', r: '2deg' },
  { y: '18px', r: '4deg' },
  { y: '44px', r: '6deg' },
] as const

const TAB_ICONS = { agent: 'spark', video: 'video', image: 'img' } as const

export function CnHero() {
  const { open, platform, macArch } = useDownload()
  const reduced = usePrefersReducedMotion()
  const copy = useCopy().hero
  const [mode, setMode] = useState<Mode>('agent')
  const [value, setValue] = useState('')
  const [focused, setFocused] = useState(false)
  const [typed, setTyped] = useState(copy.modes.agent.prompts[0])
  const [promptIndex, setPromptIndex] = useState(0)
  const [dlOpen, setDlOpen] = useState(false)
  const [dlPlacement, setDlPlacement] = useState<'down' | 'up'>('down')
  const inputRef = useRef<HTMLTextAreaElement>(null)
  const dlRef = useRef<HTMLDivElement>(null)
  const menuRef = useRef<HTMLDivElement>(null)

  // The menu is a plain popover: outside clicks and Escape dismiss it, and
  // its vertical direction flips upward when the viewport bottom is too close.
  useEffect(() => {
    if (!dlOpen) return
    const updatePlacement = () => {
      const wrap = dlRef.current
      if (!wrap) return
      const rect = wrap.getBoundingClientRect()
      const menuHeight = menuRef.current?.offsetHeight || 132
      const gap = 16
      const spaceBelow = window.innerHeight - rect.bottom
      const spaceAbove = rect.top
      setDlPlacement(
        spaceBelow < menuHeight + gap && spaceAbove > spaceBelow ? 'up' : 'down'
      )
    }
    updatePlacement()
    const onPointerDown = (event: PointerEvent) => {
      if (!dlRef.current?.contains(event.target as Node)) setDlOpen(false)
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setDlOpen(false)
    }
    document.addEventListener('pointerdown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    window.addEventListener('scroll', updatePlacement, { passive: true })
    window.addEventListener('resize', updatePlacement)
    return () => {
      document.removeEventListener('pointerdown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('scroll', updatePlacement)
      window.removeEventListener('resize', updatePlacement)
    }
  }, [dlOpen])

  const prompts = copy.modes[mode].prompts
  const full = prompts[promptIndex % prompts.length]

  useEffect(() => {
    if (reduced || value || focused) {
      setTyped(full)
      return
    }
    let count = 0
    let timer = window.setTimeout(function tick() {
      count += 1
      setTyped(full.slice(0, count))
      if (count < full.length) {
        timer = window.setTimeout(tick, 55)
      } else {
        timer = window.setTimeout(
          () => setPromptIndex((index) => index + 1),
          2600
        )
      }
    }, 120)
    return () => window.clearTimeout(timer)
  }, [full, focused, reduced, value])

  const submit = () => open()

  // Unknown chip defaults to the Apple-silicon build: every Mac sold since
  // late 2020 is arm64, and an Intel user gets a clear "cannot open" rather
  // than a silently wrong installer.
  let dlMainHref = macArch === 'x64' ? DOWNLOAD_MAC_X64 : DOWNLOAD_MAC_ARM64
  if (platform === 'win') dlMainHref = DOWNLOAD_WIN

  return (
    <section className='hero' id='top'>
      <div className='hero-glow' aria-hidden='true' />
      <div className='wrap'>
        <h1 className='h1'>
          <span>{copy.titleA}</span>
          <span>{copy.titleB}</span>
        </h1>
        <p className='hero-sub'>
          {copy.subPre}
          <span className='nw'>{copy.subHighlight}</span>
          {copy.subPost}
        </p>

        <div className='hero-stack'>
          <div className='c-tabs' role='tablist' aria-label={copy.tabsAria}>
            {(['agent', 'video', 'image'] as const).map((item) => (
              <button
                key={item}
                type='button'
                role='tab'
                aria-selected={mode === item}
                onClick={() => {
                  setMode(item)
                  setPromptIndex(0)
                }}
              >
                <Icon name={TAB_ICONS[item]} />
                {copy.tabs[item]}
              </button>
            ))}
          </div>

          <form
            className='composer'
            autoComplete='off'
            onSubmit={(event) => {
              event.preventDefault()
              submit()
            }}
          >
            <label className='sr-only' htmlFor='cn-composer-input'>
              {copy.inputLabel}
            </label>
            <textarea
              id='cn-composer-input'
              ref={inputRef}
              rows={2}
              value={value}
              placeholder={typed}
              onFocus={() => setFocused(true)}
              onBlur={() => setFocused(false)}
              onChange={(event) => setValue(event.target.value)}
              onKeyDown={(event) => {
                if (
                  event.key === 'Enter' &&
                  !event.shiftKey &&
                  !event.nativeEvent.isComposing
                ) {
                  event.preventDefault()
                  submit()
                }
              }}
            />
            <div className='c-bar'>
              <button
                type='button'
                className='c-tool c-tool-icon'
                aria-label={copy.attachAria}
              >
                <Icon name='plus' />
              </button>
              <button type='button' className='c-tool'>
                <Icon name='book' />
                {copy.skillsLabel}
              </button>
              <span className='c-spacer' />
              {copy.modes[mode].model ? (
                <span className='c-model'>{copy.modes[mode].model}</span>
              ) : null}
              <button type='submit' className='c-send' aria-label={copy.sendAria}>
                <Icon name='up' />
              </button>
            </div>
          </form>
        </div>

        <div className='skills' role='group' aria-label={copy.skillsAria}>
          {copy.skills.map((skill) => (
            <button
              key={skill.label}
              type='button'
              className='skill'
              onClick={() => {
                setValue(skill.fill)
                inputRef.current?.focus()
              }}
            >
              <Icon name={skill.icon} />
              {skill.label}
            </button>
          ))}
        </div>

        <div className='hero-dl'>
          <div
            className={`split-dl${dlOpen ? ' open' : ''}${dlPlacement === 'up' ? ' up' : ''}`}
            ref={dlRef}
          >
            <a
              className='split-main'
              href={dlMainHref}
            >
              <svg
                aria-hidden='true'
                fill='none'
                height='16'
                viewBox='0 0 16 16'
                width='16'
              >
                <path
                  d='M15.3695 11.411L15.1234 12.8866C14.8869 14.3042 13.6603 15.3436 12.223 15.3436H3.77673C2.33958 15.3434 1.1128 14.3042 0.876343 12.8866L0.630249 11.411L2.05408 11.1747L2.29919 12.6493C2.41973 13.3713 3.04475 13.9001 3.77673 13.9003H12.223C12.9551 13.9002 13.58 13.3713 13.7006 12.6493L13.9457 11.1747L15.3695 11.411ZM8.72205 8.994C8.77717 8.93934 8.83792 8.88106 8.90271 8.81627L12.4828 5.23424L13.5043 6.25572L9.92224 9.8358C9.6395 10.1185 9.38763 10.3732 9.15857 10.5575C8.91892 10.7503 8.63953 10.9224 8.2865 10.9784C8.09711 11.0083 7.90363 11.0083 7.71423 10.9784C7.36106 10.9224 7.0809 10.7503 6.84119 10.5575C6.61215 10.3732 6.36022 10.1185 6.07751 9.8358L2.49646 6.25572L3.51697 5.23424L7.09705 8.81627C7.16219 8.88142 7.22331 8.94006 7.27869 8.99498V1.3065H8.72205V8.994Z'
                  fill='currentColor'
                />
              </svg>
              {platform === 'win' ? copy.dlWin : copy.dlMac}
            </a>
            <button
              aria-expanded={dlOpen}
              aria-haspopup='menu'
              aria-label={copy.dlMenuAria}
              className='split-caret'
              type='button'
              onClick={() => setDlOpen((v) => !v)}
            >
              <svg
                aria-hidden='true'
                fill='none'
                height='14'
                viewBox='0 0 14 14'
                width='14'
              >
                <path
                  d='M11.8486 5.5L11.4238 5.92383L8.69727 8.65137C8.44157 8.90706 8.21562 9.13382 8.01172 9.29785C7.79912 9.46883 7.55595 9.61756 7.25 9.66602C7.08435 9.69222 6.91565 9.69222 6.75 9.66602C6.44405 9.61756 6.20088 9.46883 5.98828 9.29785C5.78438 9.13382 5.55843 8.90706 5.30273 8.65137L2.57617 5.92383L2.15137 5.5L3 4.65137L3.42383 5.07617L6.15137 7.80273C6.42595 8.07732 6.59876 8.24849 6.74023 8.3623C6.87291 8.46904 6.92272 8.47813 6.9375 8.48047C6.97895 8.48703 7.02105 8.48703 7.0625 8.48047C7.07728 8.47813 7.12709 8.46904 7.25977 8.3623C7.40124 8.24849 7.57405 8.07732 7.84863 7.80273L10.5762 5.07617L11 4.65137L11.8486 5.5Z'
                  fill='currentColor'
                />
              </svg>
            </button>
            <div className='split-menu' ref={menuRef} role='menu'>
              <a
                className='split-item'
                href={DOWNLOAD_MAC_ARM64}
                role='menuitem'
                onClick={() => setDlOpen(false)}
              >
                <span className='t'>{copy.dlMacArm}</span>
              </a>
              <a
                className='split-item'
                href={DOWNLOAD_MAC_X64}
                role='menuitem'
                onClick={() => setDlOpen(false)}
              >
                <span className='t'>{copy.dlMacIntel}</span>
              </a>
              <a
                className='split-item'
                href={DOWNLOAD_WIN}
                role='menuitem'
                onClick={() => setDlOpen(false)}
              >
                <span className='t'>{copy.dlWinX64}</span>
              </a>
            </div>
          </div>

          <a
            className='gh-star'
            href={SOURCE_REPO}
            rel='noopener noreferrer'
            target='_blank'
          >
            <svg
              aria-hidden='true'
              fill='currentColor'
              height='17'
              viewBox='0 0 16 16'
              width='17'
            >
              <path d='M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27s1.36.09 2 .27c1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z' />
            </svg>
            {copy.ghStar}
            <svg
              aria-hidden='true'
              className='ext'
              fill='none'
              height='13'
              viewBox='0 0 12 12'
              width='13'
            >
              <path
                d='M4.5 2.5H2.5A1 1 0 0 0 1.5 3.5v6a1 1 0 0 0 1 1h6a1 1 0 0 0 1-1v-2M7 1.5h3.5V5M10.2 1.8 5.5 6.5'
                stroke='currentColor'
                strokeLinecap='round'
                strokeLinejoin='round'
                strokeWidth='1.2'
              />
            </svg>
          </a>
        </div>
      </div>

      <div className='reel' aria-label={copy.reelAria}>
        {copy.reel.map((item, index) => {
          const layout = REEL_LAYOUT[index]
          const media = REEL_MEDIA[index]
          return (
            <figure
              key={item.title}
              className='reel-card'
              style={
                {
                  '--i': index,
                  '--y': layout.y,
                  '--r': layout.r,
                } as React.CSSProperties
              }
            >
              <LazyVideo src={media.src} poster={media.poster} />
              <figcaption>
                <b>{item.title}</b>
                <span>{item.tag}</span>
              </figcaption>
            </figure>
          )
        })}
      </div>
    </section>
  )
}
