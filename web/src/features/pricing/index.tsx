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
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import { LandingFooter } from '@/features/omnimux-landing/components/landing-footer'

import { PricingTopupLink } from './components/pricing-topup-link'
import {
  PublicPricingView,
  type PricingCatalogSearch,
} from './public-pricing-view'

export function Pricing(props: { search?: PricingCatalogSearch } = {}) {
  const { t } = useTranslation()

  useEffect(() => {
    document.documentElement.dataset.chatwootMobileLift = 'true'
    return () => {
      delete document.documentElement.dataset.chatwootMobileLift
    }
  }, [])

  return (
    <PublicLayout
      showMainContainer={false}
      headerProps={{ showSystemUpdate: false }}
    >
      <PageTransition>
        <PublicPricingView search={props.search} />
      </PageTransition>
      <div className='pb-28 md:pb-0'>
        <LandingFooter containerClassName='mx-auto max-w-7xl px-4 sm:px-6 lg:px-8' />
      </div>
      <div className='bg-background/95 border-border fixed inset-x-0 bottom-0 z-40 flex items-center justify-between gap-3 border-t px-4 pt-2 pb-[max(0.5rem,env(safe-area-inset-bottom))] backdrop-blur md:hidden'>
        <p className='text-foreground min-w-0 text-sm font-medium'>
          {t('Pay per use, presets from $10')}
        </p>
        <PricingTopupLink position='sticky_mobile' />
      </div>
    </PublicLayout>
  )
}
