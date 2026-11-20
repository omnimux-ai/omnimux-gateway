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

import { TOPUP_LADDER, USD_PER_CREDIT } from '@/features/pricing/constants'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import { landingLangOption, useLanding } from '../../i18n/context'
import { trackHomePricingTeaserClick } from '../../lib/home-analytics'
import { Icon } from './icons'

const PREVIEW_CREDITS = new Set([100, 3_000, 10_000])
const previewTiers = TOPUP_LADDER.filter((tier) =>
  PREVIEW_CREDITS.has(tier.credits)
)

export function CnPricingTeaser() {
  const { lang, copy } = useLanding()
  const content = copy.pricingTeaser
  const locale = toIntlLocale(lang) ?? landingLangOption(lang).htmlLang
  const wholeUsd = new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: 'USD',
    currencyDisplay: 'narrowSymbol',
    maximumFractionDigits: 0,
  })
  const unitUsd = new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: 'USD',
    currencyDisplay: 'narrowSymbol',
  })
  const entry = previewTiers[0]
  if (!entry) return null

  return (
    <section className='sec' id='pricing'>
      <span className='legacy-home-anchor' id='models' aria-hidden='true' />
      <div className='wrap home-pricing'>
        <div>
          <p className='home-section-label'>{content.kicker}</p>
          <h2 className='h2'>{content.h2}</h2>
          <p className='lead'>{content.lead}</p>
          <div className='home-pricing-actions'>
            <Link
              className='btn btn-primary'
              to='/pricing'
              onClick={() => trackHomePricingTeaserClick('full_pricing')}
            >
              {content.ctaPrimary}
              <Icon name='right' />
            </Link>
            <a
              className='btn btn-text'
              href='#download'
              onClick={() => trackHomePricingTeaserClick('download_first')}
            >
              {content.ctaSecondary}
            </a>
          </div>
        </div>
        <div className='card home-pricing-card'>
          <p className='home-section-label'>{content.cardSubtitle}</p>
          <h3>
            {content.cardPriceTitle.replace(
              '{credits}',
              formatNumber(entry.credits, locale)
            )}
          </h3>
          <p className='home-pricing-start'>
            {content.cardPriceSub.replace(
              '{amount}',
              wholeUsd.format(entry.amountUsd)
            )}
          </p>
          <ul>
            {previewTiers.map((tier) => {
              const saving = Math.round((1 - tier.discount) * 100)
              return (
                <li key={tier.credits}>
                  <span>
                    {content.tierCredits.replace(
                      '{credits}',
                      formatNumber(tier.credits, locale)
                    )}
                    {saving > 0 && (
                      <small>
                        {' '}
                        {content.tierSave.replace(
                          '{percent}',
                          formatNumber(saving, locale)
                        )}
                      </small>
                    )}
                  </span>
                  <strong>{wholeUsd.format(tier.amountUsd)}</strong>
                </li>
              )
            })}
          </ul>
          <Link
            className='btn btn-ghost'
            to='/pricing'
            onClick={() => trackHomePricingTeaserClick('full_pricing')}
          >
            {content.cardCta}
            <Icon name='right' />
          </Link>
          <p className='home-pricing-note'>
            {content.cardFootnote.replace(
              '{price}',
              unitUsd.format(USD_PER_CREDIT)
            )}
          </p>
        </div>
      </div>
    </section>
  )
}
