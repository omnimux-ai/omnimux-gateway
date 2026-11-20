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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { buttonVariants } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import {
  fetchDesktopRelease,
  releaseUrls,
} from '@/features/omnimux-landing/desktop/components/cn/desktop-release'
import { normalizeInterfaceLanguage, toIntlLocale } from '@/i18n/languages'
import { trackDownloadCtaClick } from '@/lib/download-cta'
import { cn } from '@/lib/utils'

export function PricingDownloadCta() {
  const { t, i18n } = useTranslation()
  const { data: release } = useQuery({
    queryKey: ['desktop-release', 'stable'],
    queryFn: () => fetchDesktopRelease(),
  })
  const urls = releaseUrls(release ?? { status: 'loading' })

  if (!urls) return null

  const locale =
    toIntlLocale(
      normalizeInterfaceLanguage(i18n.resolvedLanguage || i18n.language)
    ) ?? 'en'
  const installers = [
    {
      id: 'pricing_download_mac',
      url: urls.arm64,
      label: t('Download for macOS'),
    },
    {
      id: 'pricing_download_windows',
      url: urls.win,
      label: t('Download for Windows'),
    },
  ]

  return (
    <section aria-labelledby='pricing-desktop-heading'>
      <Card className='rounded-2xl'>
        <CardContent className='flex flex-col gap-4 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between'>
          <div className='space-y-2'>
            <h2
              id='pricing-desktop-heading'
              className='text-foreground text-2xl font-bold tracking-tight sm:text-3xl'
            >
              {t('Estimate first, then make your first top-up')}
            </h2>
            <p className='text-muted-foreground text-sm text-pretty'>
              {t(
                'Download the desktop app, sign in, and start with the 100-credit tier.'
              )}
            </p>
          </div>
          <div className='flex flex-wrap gap-2'>
            {installers.map((installer) => (
              <a
                key={installer.id}
                className={cn(
                  buttonVariants({ variant: 'outline' }),
                  'h-10 rounded-full px-4'
                )}
                href={installer.url}
                onClick={() =>
                  trackDownloadCtaClick(
                    {
                      page: 'pricing',
                      cta_id: installer.id,
                      cta_position: 'footer',
                      cta_variant: 'direct',
                      locale,
                    },
                    installer.url
                  )
                }
              >
                {installer.label}
              </a>
            ))}
          </div>
        </CardContent>
      </Card>
    </section>
  )
}
