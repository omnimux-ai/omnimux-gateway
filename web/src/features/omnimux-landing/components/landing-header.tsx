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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { BrandWordmark } from '@/components/brand-wordmark'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

// Block 0 — navigation. Light theme glass chrome.
export function LandingHeader() {
  const { t } = useTranslation()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const [mobileOpen, setMobileOpen] = useState(false)

  // Two destinations only. Everything else was folded away so the header stops
  // competing with the primary action; 灵感 opens the studio itself.
  const navLinks = [
    { label: t('omnimux_landing.nav.pricing', '定价'), href: '#pricing' },
    {
      label: t('omnimux_landing.nav.inspiration', '灵感'),
      href: '#works',
    },
  ]

  return (
    <header className='sticky top-0 z-50 border-b border-white/8 bg-[#09090B]/85 backdrop-blur-md'>
      <nav className='omx-shell flex h-[60px] items-center justify-between sm:h-[72px]'>
        <a href='#' className='flex items-center' aria-label='OmniMux'>
          <BrandWordmark onDark className='h-6 sm:h-7' />
        </a>

        {/* Desktop right cluster: nav links + Auth/CTA, all right-aligned.
            No language control here: the public page follows the visitor's
            system language, and only the console lets a signed-in user change it. */}
        <div className='hidden items-center gap-2 lg:flex'>
          {navLinks.map((link) =>
            link.href.startsWith('/') ? (
              <Link
                key={link.href}
                to={link.href}
                className='rounded px-3 py-1.5 text-[14px] font-medium text-white/70 transition-colors duration-150 hover:text-white'
              >
                {link.label}
              </Link>
            ) : (
              <a
                key={link.href}
                href={link.href}
                className='rounded px-3 py-1.5 text-[14px] font-medium text-white/70 transition-colors duration-150 hover:text-white'
              >
                {link.label}
              </a>
            )
          )}
          {isAuthenticated ? (
            <Link to='/influencer' className='omx-btn-primary ml-1 h-9'>
              {t('Start')}
            </Link>
          ) : (
            <>
              <Link
                to='/sign-in'
                className='rounded px-3 py-1.5 text-[14px] font-medium text-white/70 transition-colors duration-150 hover:text-white'
              >
                {t('omnimux_landing.nav.login', '登录')}
              </Link>
              <Link to='/sign-up' className='omx-btn-primary h-9'>
                {t('Start')}
              </Link>
            </>
          )}
        </div>

        {/* Mobile / tablet: hamburger */}
        <div className='flex items-center gap-2 lg:hidden'>
          <button
            type='button'
            className='flex size-9 items-center justify-center rounded border border-white/10 text-white/70'
            aria-label={t('Toggle navigation menu', '切换菜单')}
            aria-expanded={mobileOpen}
            onClick={() => setMobileOpen((open) => !open)}
          >
            <div className='relative size-4'>
              <span
                className={cn(
                  'absolute inset-x-0 block h-[1.5px] origin-center bg-current transition-transform duration-200',
                  mobileOpen ? 'top-[7px] rotate-45' : 'top-[3px]'
                )}
              />
              <span
                className={cn(
                  'absolute inset-x-0 block h-[1.5px] origin-center bg-current transition-transform duration-200',
                  mobileOpen ? 'top-[7px] -rotate-45' : 'top-[11px]'
                )}
              />
            </div>
          </button>
        </div>
      </nav>

      {/* Mobile menu panel */}
      {mobileOpen && (
        <div className='border-t border-white/8 bg-[#09090B] px-4 py-4 sm:px-[60px] lg:hidden'>
          <div className='flex flex-col gap-1'>
            {navLinks.map((link) =>
              link.href.startsWith('/') ? (
                <Link
                  key={link.href}
                  to={link.href}
                  className='rounded px-2 py-2.5 text-sm font-medium text-white/70'
                  onClick={() => setMobileOpen(false)}
                >
                  {link.label}
                </Link>
              ) : (
                <a
                  key={link.href}
                  href={link.href}
                  className='rounded px-2 py-2.5 text-sm font-medium text-white/70'
                  onClick={() => setMobileOpen(false)}
                >
                  {link.label}
                </a>
              )
            )}
            <div className='mt-3 flex flex-col gap-2'>
              {isAuthenticated ? (
                <Link
                  to='/influencer'
                  className='omx-btn-primary'
                  onClick={() => setMobileOpen(false)}
                >
                  {t('Start')}
                </Link>
              ) : (
                <>
                  <Link
                    to='/sign-in'
                    className='omx-btn-secondary'
                    onClick={() => setMobileOpen(false)}
                  >
                    {t('omnimux_landing.nav.login', '登录')}
                  </Link>
                  <Link
                    to='/sign-up'
                    className='omx-btn-primary'
                    onClick={() => setMobileOpen(false)}
                  >
                    {t('Start')}
                  </Link>
                </>
              )}
            </div>
          </div>
        </div>
      )}
    </header>
  )
}
