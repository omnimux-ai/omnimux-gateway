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
import { useTranslation } from 'react-i18next'

import { BrandWordmark } from '@/components/brand-wordmark'
import { useInterfaceLanguage } from '@/hooks/use-interface-language'
import { INTERFACE_LANGUAGE_OPTIONS } from '@/i18n/languages'
import { cn } from '@/lib/utils'

import { CONTACT_EMAIL, DOCS_URL } from '../lib/landing-urls'

interface FooterLink {
  label: string
  href: string
  external?: boolean
  fullPage?: boolean
}

/**
 * Seven interface languages do not fit the two-pill group this layout was
 * modelled on, so each language shows its own short form. The full name stays
 * available as the control's accessible name.
 */
const LANGUAGE_SHORT_LABELS: Record<string, string> = {
  zhCN: '简',
  en: 'EN',
  fr: 'FR',
  ru: 'RU',
  ja: 'JA',
  vi: 'VI',
  zhTW: '繁',
}

const LINK_CLASS =
  'text-[14px] font-medium text-white/70 transition-colors duration-150 hover:text-white'

export function LandingFooter() {
  const { t } = useTranslation()
  const { currentLanguage, changeLanguage } = useInterfaceLanguage()

  const links: FooterLink[] = [
    { label: t('Guides'), href: '/guides', fullPage: true },
    { label: t('About'), href: '/about' },
    { label: t('Documentation'), href: DOCS_URL, external: true },
    { label: t('omnimux_landing.footer.trends', '趋势'), href: '/influencer' },
    {
      label: t('omnimux_landing.footer.dashboard', '控制台'),
      href: '/dashboard',
    },
    {
      label: t('omnimux_landing.footer.support', '支持'),
      href: `mailto:${CONTACT_EMAIL}`,
      external: true,
    },
    { label: t('omnimux_landing.footer.terms', '服务条款'), href: '/terms' },
    {
      label: t('omnimux_landing.footer.privacy', '隐私政策'),
      href: '/privacy',
    },
    {
      label: t('omnimux_landing.footer.refunds', '退款政策'),
      href: '/refunds',
    },
    {
      label: t('omnimux_landing.footer.acceptableUse', '可接受使用'),
      href: '/acceptable-use',
    },
  ]

  return (
    <footer className='border-t border-white/8 bg-[#0C0C0E]'>
      <div className='omx-shell py-12 sm:py-14'>
        <div className='flex flex-col gap-8 lg:flex-row lg:items-center lg:justify-between lg:gap-10'>
          {/* Brand */}
          <Link
            to='/'
            className='flex shrink-0 items-center'
            aria-label='OmniMux'
          >
            <BrandWordmark onDark className='h-7' />
          </Link>

          {/* Destinations + language */}
          <div className='flex flex-col gap-6 lg:flex-row lg:items-center lg:gap-8'>
            <nav aria-label={t('omnimux_landing.footer.navLabel', '页脚导航')}>
              <ul className='flex flex-wrap items-center gap-x-6 gap-y-3'>
                {links.map((link) => (
                  <li key={link.href}>
                    {link.external || link.fullPage ? (
                      <a
                        href={link.href}
                        className={cn(
                          LINK_CLASS,
                          link.fullPage && 'inf-footer-guide-link'
                        )}
                      >
                        {link.label}
                      </a>
                    ) : (
                      <Link to={link.href} className={LINK_CLASS}>
                        {link.label}
                      </Link>
                    )}
                  </li>
                ))}
              </ul>
            </nav>

            <div
              role='group'
              aria-label={t('Change language')}
              className='flex shrink-0 items-center gap-1 self-start rounded-full border border-white/10 p-1 lg:self-auto'
            >
              {INTERFACE_LANGUAGE_OPTIONS.map((option) => {
                const active = option.code === currentLanguage
                return (
                  <button
                    key={option.code}
                    type='button'
                    title={option.label}
                    aria-label={option.label}
                    aria-pressed={active}
                    onClick={() => {
                      void changeLanguage(option.code)
                    }}
                    className={cn(
                      'rounded-full px-2.5 py-1 text-[12px] font-semibold transition-colors duration-150',
                      active
                        ? 'bg-white text-[#0C0C0E]'
                        : 'text-white/60 hover:text-white'
                    )}
                  >
                    {LANGUAGE_SHORT_LABELS[option.code] ??
                      option.code.toUpperCase()}
                  </button>
                )
              })}
            </div>
          </div>
        </div>

        <p className='mt-6 text-[12px] text-white/60'>
          {t(
            'App links follow your interface language. Guide articles are in English.'
          )}
        </p>

        {/* Tagline + legal line */}
        <div className='mt-8 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between'>
          <p className='text-[14px] text-white/60'>
            {t(
              'omnimux_landing.footer.tagline',
              'Create your own AI influencer'
            )}
          </p>
          <p className='text-[14px] text-white/60'>
            © {new Date().getFullYear()} OmniMux ·{' '}
            {t('footer.defaultCopyright', 'All rights reserved.')}
            {' · '}
            <a
              href='https://github.com/QuantumNous/new-api'
              target='_blank'
              rel='noopener noreferrer'
              className='transition-colors duration-150 hover:text-white'
            >
              {t('omnimux_landing.footer.attributionPrefix', '基于')} New API
            </a>
          </p>
        </div>
      </div>
    </footer>
  )
}
