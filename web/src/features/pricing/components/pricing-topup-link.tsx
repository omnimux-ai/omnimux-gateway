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

import { buttonVariants } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import { usdToCredits } from '../lib/credits'
import {
  PRICING_ANALYTICS_EVENTS,
  trackPricingEvent,
} from '../lib/pricing-analytics'

export function PricingTopupLink(props: {
  position: 'hero' | 'ladder' | 'sticky_mobile'
  amountUsd?: number
  prominent?: boolean
  className?: string
}) {
  const { t, i18n } = useTranslation()
  const isAuthenticated = useAuthStore((state) => Boolean(state.auth.user))
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const preset = props.amountUsd
  const prominent = props.prominent ?? props.position !== 'ladder'

  return (
    <Link
      to={isAuthenticated ? '/wallet' : '/sign-in'}
      hash={isAuthenticated ? 'wallet-add-funds' : undefined}
      search={isAuthenticated ? undefined : { redirect: '/wallet' }}
      className={buttonVariants({
        variant: prominent ? 'default' : 'outline',
        className: cn(
          'h-10 rounded-full px-4',
          prominent &&
            'bg-foreground text-background [a]:hover:bg-foreground/90',
          props.className
        ),
      })}
      aria-label={
        preset === undefined
          ? undefined
          : t('Select {{amount}} credits', {
              amount: formatNumber(usdToCredits(preset), locale),
            })
      }
      onClick={() =>
        trackPricingEvent(PRICING_ANALYTICS_EVENTS.TOPUP_CLICK, {
          cta_position: props.position,
          amount_usd: preset,
          is_authenticated: isAuthenticated,
        })
      }
    >
      {preset === undefined ? t('Top up credits') : t('Select')}
    </Link>
  )
}
