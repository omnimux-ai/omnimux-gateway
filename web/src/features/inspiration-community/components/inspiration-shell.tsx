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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import { LandingFooter } from '@/features/omnimux-landing/components/landing-footer'
import { cn } from '@/lib/utils'

export type InspirationSectionKey = 'prompts' | 'tiktok'

/** First-level segmented navigation shared by both Inspiration pages. */
const SECTIONS: Array<{
  key: InspirationSectionKey
  to: '/inspiration' | '/inspiration/tiktok'
  label: string
}> = [
  { key: 'prompts', to: '/inspiration', label: 'Prompt inspiration' },
  { key: 'tiktok', to: '/inspiration/tiktok', label: 'TikTok Ad Library' },
]

export interface InspirationStat {
  label: string
  value: string
}

export interface InspirationShellProps {
  active: InspirationSectionKey
  title?: string
  subtitle?: string
  stats?: InspirationStat[]
  /**
   * Optional trail rendered above the H1. L2 uses it for
   * `Home / Prompt inspiration / Seedance 2.5`.
   */
  breadcrumb?: ReactNode
  children: ReactNode
  /**
   * Whether to render the hero block (top pill switcher, title, subtitle, stats).
   * Defaults to true; set to false for the streamlined, content-first layout.
   */
  showHero?: boolean
  /**
   * Whether to render the L1 top pill switcher (Prompt inspiration / Marketing Studio).
   * Defaults to true; set to false when viewing a specific 专区 page.
   */
  showSectionNav?: boolean
}

/**
 * Shared chrome for the Inspiration Community pages: the site header (reused
 * from `PublicLayout`), a first-level segmented nav, the hero block with the
 * collected-scale stats, and a compact footer.
 *
 * PRD section 2 keeps both pages under one shell so the two structurally
 * different galleries share navigation without merging into one filter screen.
 */
export function InspirationShell(props: InspirationShellProps) {
  const { t } = useTranslation()

  let headerContent: ReactNode = null
  if (props.showHero !== false) {
    headerContent = (
      <header className='flex flex-col gap-5'>
        {props.breadcrumb}

        {props.showSectionNav !== false && (
          <nav aria-label={t('Inspiration Community')}>
            <div className='border-border bg-muted/40 inline-flex items-center gap-1 rounded-full border p-1'>
              {SECTIONS.map((section) => {
                const isActive = section.key === props.active
                return (
                  <Link
                    key={section.key}
                    to={section.to}
                    aria-current={isActive ? 'page' : undefined}
                    className={cn(
                      'focus-visible:ring-ring rounded-full px-3.5 py-1.5 text-sm font-medium transition-colors focus-visible:ring-2 focus-visible:outline-none',
                      isActive
                        ? 'bg-foreground text-background'
                        : 'text-muted-foreground hover:text-foreground'
                    )}
                  >
                    {t(section.label)}
                  </Link>
                )
              })}
            </div>
          </nav>
        )}

        <div className='flex flex-col gap-3'>
          <h1 className='text-3xl font-extrabold tracking-[-0.03em] text-balance sm:text-4xl md:text-5xl'>
            {props.title}
          </h1>
          {props.subtitle ? (
            <p className='text-muted-foreground max-w-2xl text-sm leading-relaxed text-pretty sm:text-base'>
              {props.subtitle}
            </p>
          ) : null}
        </div>

        {props.stats && props.stats.length > 0 && (
          <dl className='flex flex-wrap gap-x-8 gap-y-3'>
            {props.stats.map((stat) => (
              <div key={stat.label} className='flex flex-col gap-0.5'>
                <dt className='text-muted-foreground text-[11px] font-medium tracking-[0.08em] uppercase'>
                  {stat.label}
                </dt>
                <dd className='font-mono text-lg font-semibold tracking-tight'>
                  {stat.value}
                </dd>
              </div>
            ))}
          </dl>
        )}
      </header>
    )
  } else if (props.breadcrumb) {
    headerContent = <header className='mb-4'>{props.breadcrumb}</header>
  }

  return (
    <PublicLayout showMainContainer={false} headerProps={{ fullWidth: true }}>
      <div className='relative min-h-svh'>
        <div
          aria-hidden='true'
          className='pointer-events-none absolute inset-x-0 top-0 h-[520px] opacity-[0.18] dark:opacity-[0.10]'
          style={{
            background:
              'radial-gradient(ellipse 55% 60% at 20% 0%, oklch(0.72 0.18 285 / 90%) 0%, transparent 70%), radial-gradient(ellipse 45% 55% at 80% 10%, oklch(0.66 0.16 300 / 70%) 0%, transparent 72%)',
            maskImage:
              'linear-gradient(to bottom, black 30%, transparent 100%)',
            WebkitMaskImage:
              'linear-gradient(to bottom, black 30%, transparent 100%)',
          }}
        />
        <div
          aria-hidden='true'
          className='pointer-events-none absolute inset-x-0 top-0 h-[520px] bg-[radial-gradient(circle,var(--border)_1px,transparent_1px)] bg-[size:20px_20px] opacity-60'
          style={{
            maskImage: 'linear-gradient(to bottom, black 10%, transparent 85%)',
            WebkitMaskImage:
              'linear-gradient(to bottom, black 10%, transparent 85%)',
          }}
        />

        <PageTransition className='relative'>
          <div
            className={cn(
              'mx-auto w-full max-w-[1440px] px-4 sm:px-6 xl:px-8',
              props.showHero !== false
                ? 'pt-16 sm:pt-20'
                : 'pt-20 sm:pt-24'
            )}
          >
            {headerContent}
          </div>

          <main className='mx-auto w-full max-w-[1440px] px-4 pb-16 sm:px-6 xl:px-8'>
            {props.children}
          </main>

          <LandingFooter />
        </PageTransition>
      </div>
    </PublicLayout>
  )
}
