import { Link } from '@tanstack/react-router'
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
import { ArrowDown } from 'lucide-react'
import { useReducedMotion } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { buttonVariants } from '@/components/ui/button'

import { USD_PER_CREDIT } from '../constants'
import { PricingTopupLink } from './pricing-topup-link'
import { ESTIMATOR_SECTION_ID } from './video-cost-estimator'

export const PRICING_MODELS_SECTION_ID = 'pricing-models'

export function BillingHero() {
  const { t } = useTranslation()
  const reduceMotion = useReducedMotion()

  return (
    <section
      className='py-8 text-center sm:py-12'
      aria-labelledby='billing-hero-heading'
    >
      <h1
        id='billing-hero-heading'
        className='text-foreground mx-auto max-w-4xl text-4xl leading-tight font-bold tracking-tight text-balance sm:text-6xl'
      >
        {t('Pay for what you use — no subscription required')}
      </h1>
      <p className='text-muted-foreground mx-auto mt-4 max-w-2xl text-base'>
        {t(
          'Estimate a video’s cost first, then choose a credit top-up. Balances, charges, and credit returns are visible in your wallet.'
        )}
      </p>
      <div className='mx-auto mt-6 flex flex-col items-center justify-center gap-3 sm:flex-row'>
        <PricingTopupLink position='hero' />
        <Link
          to='/pricing'
          search
          hash={ESTIMATOR_SECTION_ID}
          hashScrollIntoView={{
            block: 'start',
            behavior: reduceMotion ? 'auto' : 'smooth',
          }}
          resetScroll={false}
          className={buttonVariants({
            size: 'lg',
            variant: 'outline',
            className: 'h-10 rounded-full px-4',
          })}
        >
          {t('Estimate a video cost')}
          <ArrowDown aria-hidden='true' />
        </Link>
      </div>
      <ul className='text-muted-foreground mx-auto mt-5 flex max-w-4xl flex-wrap justify-center gap-2 text-xs'>
        {[
          t('1 credit = ${{usd}} USD', { usd: USD_PER_CREDIT.toFixed(2) }),
          t('Card, Alipay/WeChat, or wire transfer'),
          t('Generation credit refunds follow task records'),
        ].map((item) => (
          <li
            key={item}
            className='border-border rounded-full border px-3 py-1'
          >
            {item}
          </li>
        ))}
      </ul>
    </section>
  )
}
