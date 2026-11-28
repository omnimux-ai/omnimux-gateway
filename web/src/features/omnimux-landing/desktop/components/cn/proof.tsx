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
import { useState } from 'react'

import { TOPUP_LADDER } from '@/features/pricing/constants'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import { useLanding } from '../../i18n/context'
import { Icon } from './icons'

export function CnProof() {
  const { lang, copy: landingCopy } = useLanding()
  const copy = landingCopy.proof
  const user = useAuthStore((state) => state.auth.user)
  const [selectedAmount, setSelectedAmount] = useState<number>(100)

  const locale = toIntlLocale(lang)
  const selectedRung =
    TOPUP_LADDER.find((rung) => rung.amountUsd === selectedAmount) ??
    TOPUP_LADDER[0]
  const payableUsd = Math.round(selectedRung.amountUsd * selectedRung.discount)
  const savedUsd = selectedRung.amountUsd - payableUsd
  const offPercent = Math.round((1 - selectedRung.discount) * 100)
  const benefitLabel =
    offPercent > 0
      ? copy.saveSummary
          .replace('{saved}', formatNumber(savedUsd, locale))
          .replace('{percent}', formatNumber(offPercent, locale))
      : copy.standardRate

  const walletHref = user
    ? '/wallet#wallet-add-funds'
    : `/sign-in?redirect=${encodeURIComponent('/wallet')}`

  return (
    <section className='sec proof' id='proof'>
      <div className='wrap'>
        <h2 className='h2'>{copy.h2}</h2>
        <p className='lead' style={{ marginTop: 14 }}>
          {copy.lead}
        </p>

        <div className='facts'>
          {copy.facts.map((fact) => (
            <div key={fact.label}>
              <b>{fact.value}</b>
              <span>{fact.label}</span>
            </div>
          ))}
        </div>

        <div className='payg-panel' id='payg-pricing'>
          <div className='payg-head'>
            <h3>{copy.paygTitle}</h3>
            <p>{copy.paygLead}</p>
          </div>

          <div className='payg-pillars'>
            {copy.pillars.map((pillar) => (
              <div key={pillar.value} className='payg-pillar'>
                <strong>{pillar.value}</strong>
                <span>{pillar.label}</span>
              </div>
            ))}
          </div>

          <div
            className='payg-ladder'
            role='radiogroup'
            aria-label={copy.ladderAria}
          >
            {TOPUP_LADDER.map((rung) => {
              const active = rung.amountUsd === selectedRung.amountUsd
              const rungPay = Math.round(rung.amountUsd * rung.discount)
              const rungOff = Math.round((1 - rung.discount) * 100)
              return (
                <button
                  key={rung.amountUsd}
                  type='button'
                  role='radio'
                  aria-checked={active}
                  className={active ? 'payg-rung is-selected' : 'payg-rung'}
                  onClick={() => setSelectedAmount(rung.amountUsd)}
                >
                  <span className='payg-credits'>
                    {`${formatNumber(rung.credits, locale)} ${copy.creditsUnit}`}
                  </span>
                  <span className='payg-price'>
                    {`$${formatNumber(rungPay, locale)}`}
                    {rungOff > 0 ? (
                      <>
                        {' '}
                        <s className='payg-orig'>
                          {`$${formatNumber(rung.amountUsd, locale)}`}
                        </s>
                      </>
                    ) : null}
                  </span>
                  <span
                    className={
                      rungOff > 0 ? 'payg-tag payg-tag-discount' : 'payg-tag'
                    }
                  >
                    {rungOff > 0
                      ? copy.discountBadge.replace(
                          '{percent}',
                          formatNumber(rungOff, locale)
                        )
                      : copy.standardRate}
                  </span>
                </button>
              )
            })}
          </div>

          <div className='payg-checkout'>
            <div className='payg-summary'>
              <div>
                <span className='payg-sum-label'>{copy.summaryCredits}</span>
                <strong className='payg-sum-val'>
                  {`${formatNumber(selectedRung.credits, locale)} ${copy.creditsUnit}`}
                </strong>
              </div>
              <div>
                <span className='payg-sum-label'>{copy.summaryPay}</span>
                <strong className='payg-sum-val'>
                  {`$${formatNumber(payableUsd, locale)}`}
                </strong>
              </div>
              <div>
                <span className='payg-sum-label'>{copy.summaryBenefit}</span>
                <strong
                  className={
                    offPercent > 0
                      ? 'payg-sum-val payg-sum-val-ok'
                      : 'payg-sum-val'
                  }
                >
                  {benefitLabel}
                </strong>
              </div>
            </div>

            <div className='payg-actions'>
              <a className='btn btn-primary' href={walletHref}>
                {copy.ctaTopup}
              </a>
              <a className='btn btn-ghost' href='/pricing'>
                {copy.ctaPricing}
              </a>
            </div>
          </div>
        </div>

        <div className='platforms' aria-label={copy.platformsAria}>
          {copy.platforms.map((platform) => (
            <span key={platform}>{platform}</span>
          ))}
        </div>

        <div className='badges'>
          <span>
            <Icon name='shield' />
            {copy.badges[0]}
          </span>
          <span>
            <Icon name='undo' />
            {copy.badges[1]}
          </span>
        </div>
      </div>
    </section>
  )
}
