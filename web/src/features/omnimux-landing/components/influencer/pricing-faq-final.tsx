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
import { Link } from '@tanstack/react-router'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'
import { fetchPricing } from '@/features/influencer/api'
import { imageModels, quoteCredits } from '@/features/influencer/lib/catalog'
import { TOPUP_LADDER } from '@/features/pricing/constants'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import { WORK_LOOKS } from '../../lib/influencer-looks'
import { Reveal } from '../reveal'

/** Live lowest per-image price across image models, or null when the price list is unavailable. */
function useBestPrice(): {
  credits: number
  usd: number
  modelCount: number
} | null {
  const pricingQ = useQuery({
    queryKey: ['influencer', 'pricing'],
    queryFn: fetchPricing,
  })
  return useMemo(() => {
    const rows = imageModels(pricingQ.data)
    const groupRatio = pricingQ.data?.group_ratio
    let best: number | null = null
    for (const row of rows) {
      const q = quoteCredits(row, undefined, groupRatio)
      if (q && (best === null || q.usd < best)) best = q.usd
    }
    if (best === null) return null
    return { credits: best * 10, usd: best, modelCount: rows.length }
  }, [pricingQ.data])
}

/**
 * Pay-as-you-go pricing: the same preset top-up ladder sold in the wallet.
 * Reading the shared ladder keeps this surface from advertising a discount the
 * backend does not honour (a regression test pins it to the canonical ops file).
 */
export function InfluencerPricing() {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const price = useBestPrice()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user

  const usd = (value: number) =>
    Intl.NumberFormat(locale, {
      style: 'currency',
      currency: 'USD',
      maximumFractionDigits: value % 1 === 0 ? 0 : 2,
    }).format(value)

  const walletDestination = isAuthenticated
    ? '/wallet#wallet-add-funds'
    : `/sign-in?redirect=${encodeURIComponent('/wallet')}`

  return (
    <section className='inf-pricing' id='pricing'>
      <div className='inf-section inf-pricing'>
        <Reveal>
          <p className='inf-eyebrow'>
            {t('influencer_landing.pricing.eyebrow')}
          </p>
          <h2 className='inf-h2'>{t('influencer_landing.pricing.title')}</h2>
        </Reveal>

        <div className='inf-ladder'>
          {TOPUP_LADDER.map((rung, index) => {
            const offPercent = Math.round((1 - rung.discount) * 100)
            return (
              <Reveal
                key={rung.amountUsd}
                className={
                  offPercent > 0 ? 'inf-rung is-discounted' : 'inf-rung'
                }
                delay={80 + index * 50}
              >
                <span className='inf-rung-credits'>
                  {formatNumber(rung.credits, locale)}
                </span>
                <span className='inf-rung-unit'>
                  {t('influencer_landing.pricing.credits')}
                </span>
                {offPercent > 0 ? (
                  <span className='inf-rung-save'>
                    {t('Save {{percent}}%', { percent: offPercent })}
                  </span>
                ) : null}
                <span className='inf-rung-usd'>
                  {`$${formatNumber(rung.amountUsd, locale)}`}
                </span>
              </Reveal>
            )
          })}
        </div>

        <Reveal delay={300}>
          <p className='inf-price-note'>
            {price
              ? t('influencer_landing.pricing.usd', { usd: usd(price.usd) })
              : t('influencer_landing.pricing.fallback')}
          </p>
        </Reveal>

        <Reveal className='inf-ladder-cta' delay={360}>
          <Link to={walletDestination} className='omx-btn-primary'>
            {t('Top up credits')}
          </Link>
        </Reveal>

        <Reveal delay={420}>
          <ul className='inf-price-feats inf-price-feats-row'>
            <li>
              <span className='inf-check' aria-hidden='true'>
                ✓
              </span>
              {t('influencer_landing.pricing.feature.character')}
            </li>
            <li>
              <span className='inf-check' aria-hidden='true'>
                ✓
              </span>
              {t('influencer_landing.pricing.feature.preview')}
            </li>
            <li>
              <span className='inf-check' aria-hidden='true'>
                ✓
              </span>
              {t('influencer_landing.pricing.feature.video')}
            </li>
            <li>
              <span className='inf-check' aria-hidden='true'>
                ✓
              </span>
              {t('influencer_landing.pricing.feature.refund')}
            </li>
          </ul>
        </Reveal>
      </div>
    </section>
  )
}

const FAQ_ITEMS = [
  { id: 'q1' },
  { id: 'q2' },
  { id: 'q3' },
  {
    id: 'q4',
    guide: {
      href: '/guides/create-ai-character',
      labelKey: 'Read the character guide',
    },
  },
  {
    id: 'q5',
    guide: {
      href: '/guides/recreate-video-with-your-character',
      labelKey: 'Read the reference video guide',
    },
  },
] as const

/** Five Q&As, every answer describing a shipped behaviour. */
export function InfluencerFaq() {
  const { t } = useTranslation()
  return (
    <section className='inf-faq'>
      <div className='inf-section'>
        <Reveal>
          <p className='inf-eyebrow'>{t('influencer_landing.faq.eyebrow')}</p>
          <h2 className='inf-h2'>{t('influencer_landing.faq.title')}</h2>
        </Reveal>
        <Reveal className='inf-faq-list' delay={80}>
          <Accordion>
            {FAQ_ITEMS.map((item) => (
              <AccordionItem key={item.id} value={item.id} className='inf-faq-item'>
                <AccordionTrigger className='inf-faq-q'>
                  {t(`influencer_landing.faq.${item.id}`)}
                </AccordionTrigger>
                <AccordionContent className='inf-faq-a'>
                  <p>{t(`influencer_landing.faq.${item.id.replace('q', 'a')}`)}</p>
                  {'guide' in item ? (
                    <a href={item.guide.href} className='inf-guide-link'>
                      {t(item.guide.labelKey)} <span aria-hidden='true'>↗</span>
                    </a>
                  ) : null}
                </AccordionContent>
              </AccordionItem>
            ))}
          </Accordion>
        </Reveal>
      </div>
    </section>
  )
}

/** Dark final CTA with three fanned, floating character cards. */
export function InfluencerFinalCta() {
  const { t } = useTranslation()
  const looks = WORK_LOOKS.slice(0, 3)
  return (
    <section className='inf-final'>
      <div className='inf-section'>
        <Reveal className='inf-final-card'>
          <div>
            <h2 className='inf-final-title'>
              {t('influencer_landing.final.titleLead')}
              <span className='inf-h2-accent'>
                {t('influencer_landing.final.titleAccent')}
              </span>
            </h2>
            <p className='inf-final-sub'>{t('influencer_landing.final.sub')}</p>
            <div className='inf-final-ctas'>
              <Link to='/influencer' className='omx-btn-primary'>
                {t('influencer_landing.final.cta')}
              </Link>
            </div>
          </div>
          <div className='inf-final-looks' aria-hidden='true'>
            {looks.map((look) => (
              <img
                key={look.key}
                className='inf-final-look'
                src={look.image}
                alt=''
                loading='lazy'
              />
            ))}
          </div>
        </Reveal>
      </div>
    </section>
  )
}
