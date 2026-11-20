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
import { Link } from '@tanstack/react-router'
import { useEffect, useRef, useState } from 'react'

import { useIsSidebarModuleVisible } from '@/hooks/use-sidebar-config'
import type { DownloadCtaIntent } from '@/lib/download-cta'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { useLanding } from '../../i18n/context'
import { CnAccountMenu } from './account-menu'
import { useDownload } from './download-context'
import { Icon } from './icons'
import { CnLangMenu } from './lang-menu'
import { CnSignOutDialog } from './sign-out'

const NAV_DOWNLOAD_INTENT: DownloadCtaIntent = {
  cta_id: 'nav_download',
  cta_position: 'header',
  cta_variant: 'nav',
}

const NAV_LINKS = [
  { key: 'capabilities', hash: '#capabilities' },
  { key: 'workflow', hash: '#workflow' },
  { key: 'pricing', to: '/pricing' },
  { key: 'inspiration', to: '/inspiration' },
] as const

/** The anchor base sends links to homepage sections from tool and Agent pages. */
export function CnNav({ anchorBase = '' }: { anchorBase?: string }) {
  const [scrolled, setScrolled] = useState(false)
  const [sheetOpen, setSheetOpen] = useState(false)
  const [signOutOpen, setSignOutOpen] = useState(false)
  const menuTrigger = useRef<HTMLButtonElement>(null)
  const { open, downloadDirect, release } = useDownload()
  const user = useAuthStore((state) => state.auth.user)
  const isWalletVisible = useIsSidebarModuleVisible('/wallet')
  const isSecurityVisible = useIsSidebarModuleVisible('/security')
  const isSuperAdmin = user?.role === ROLE.SUPER_ADMIN
  const { lang, setLang, copy, langOptions } = useLanding()
  const nav = copy.nav

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 8)
    onScroll()
    window.addEventListener('scroll', onScroll, { passive: true })
    return () => window.removeEventListener('scroll', onScroll)
  }, [])

  useEffect(() => {
    if (!sheetOpen) return
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape' || event.defaultPrevented) return
      setSheetOpen(false)
      menuTrigger.current?.focus()
    }
    document.addEventListener('keydown', closeOnEscape)
    return () => document.removeEventListener('keydown', closeOnEscape)
  }, [sheetOpen])

  function renderNavLinks(onClick?: () => void) {
    return NAV_LINKS.map((link) =>
      'to' in link ? (
        <Link key={link.key} to={link.to} onClick={onClick}>
          {nav[link.key]}
        </Link>
      ) : (
        <a key={link.key} href={`${anchorBase}${link.hash}`} onClick={onClick}>
          {nav[link.key]}
        </a>
      )
    )
  }

  const closeSheet = () => setSheetOpen(false)

  return (
    <>
      <header className={scrolled ? 'nav scrolled' : 'nav'}>
        <div className='nav-in'>
          <a
            className='brand'
            href={anchorBase || '#top'}
            aria-label={nav.brandAria}
          >
            <img src='/logo.svg' alt='' />
            <b>OmniMux</b>
            <small>{nav.brand}</small>
          </a>
          <nav className='nav-links' aria-label={nav.navAria}>
            {renderNavLinks()}
          </nav>
          <div className='nav-end'>
            <CnLangMenu />
            <button
              className='btn btn-sm btn-primary'
              type='button'
              disabled={release.status === 'unavailable'}
              title={
                release.status === 'unavailable'
                  ? copy.download.unavailable
                  : undefined
              }
              onClick={() => {
                if (!downloadDirect(NAV_DOWNLOAD_INTENT)) {
                  open(undefined, NAV_DOWNLOAD_INTENT)
                }
              }}
            >
              <Icon name='dl' />
              <span>{nav.download}</span>
            </button>
            {user ? (
              <CnAccountMenu onSignOut={() => setSignOutOpen(true)} />
            ) : (
              <Link className='btn btn-sm btn-ghost login' to='/sign-in'>
                {nav.login}
              </Link>
            )}
          </div>
          <button
            ref={menuTrigger}
            className='menu-btn'
            type='button'
            aria-label={sheetOpen ? nav.menuClose : nav.menuOpen}
            aria-expanded={sheetOpen}
            aria-controls='cn-sheet'
            onClick={() => setSheetOpen((value) => !value)}
          >
            <Icon name={sheetOpen ? 'x' : 'menu'} />
          </button>
        </div>
      </header>
      <nav
        id='cn-sheet'
        className={sheetOpen ? 'sheet open' : 'sheet'}
        aria-label={nav.navAria}
        hidden={!sheetOpen}
      >
        {renderNavLinks(closeSheet)}
        <a
          className='btn btn-primary sheet-download'
          href={`${anchorBase}#download`}
          onClick={closeSheet}
        >
          {nav.downloadDesktop}
        </a>
        {user ? (
          <>
            <Link to='/influencer' onClick={closeSheet}>
              {nav.account.dashboard}
            </Link>
            <Link to='/profile' onClick={closeSheet}>
              {nav.account.profile}
            </Link>
            {isSecurityVisible && (
              <Link to='/security' onClick={closeSheet}>
                {nav.account.security}
              </Link>
            )}
            {isWalletVisible && (
              <Link to='/wallet' onClick={closeSheet}>
                {nav.account.wallet}
              </Link>
            )}
            {isSuperAdmin && (
              <Link
                to='/system-settings/site/$section'
                params={{ section: 'system-info' }}
                onClick={closeSheet}
              >
                {nav.account.systemSettings}
              </Link>
            )}
            <button
              type='button'
              className='sheet-signout'
              onClick={() => {
                closeSheet()
                menuTrigger.current?.focus()
                setSignOutOpen(true)
              }}
            >
              {nav.account.signOut}
            </button>
          </>
        ) : (
          <Link to='/sign-in' onClick={closeSheet}>
            {nav.login}
          </Link>
        )}
        <div className='sheet-langs' role='group' aria-label={nav.langTitle}>
          {langOptions.map((option) => (
            <button
              key={option.code}
              type='button'
              className={option.code === lang ? 'on' : undefined}
              aria-pressed={option.code === lang}
              lang={option.htmlLang}
              onClick={() => setLang(option.code)}
            >
              {option.label}
            </button>
          ))}
        </div>
      </nav>
      <CnSignOutDialog open={signOutOpen} onOpenChange={setSignOutOpen} />
    </>
  )
}
