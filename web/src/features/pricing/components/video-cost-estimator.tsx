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
import { useEffect, useId, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Slider } from '@/components/ui/slider'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import { creditsToUsd } from '../lib/credits'
import {
  PRICING_ANALYTICS_EVENTS,
  trackPricingEvent,
} from '../lib/pricing-analytics'
import {
  DEFAULT_ESTIMATE_SECONDS,
  formatVideoCreditsPerSecond,
  type VideoEstimateRate,
} from '../lib/video-cost-estimate'

export const ESTIMATOR_SECTION_ID = 'estimator'

const MIN_DURATION = 5
const MAX_DURATION = 30
const ESTIMATOR_DEBOUNCE_MS = 500

export function VideoCostEstimator(props: {
  rates: readonly VideoEstimateRate[]
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const titleId = useId()
  const durationId = useId()
  const modelId = useId()
  const [seconds, setSeconds] = useState(DEFAULT_ESTIMATE_SECONDS)
  const [selectedModelId, setSelectedModelId] = useState(
    () => props.rates[0]?.id
  )
  const durationTimer = useRef<number>(undefined)
  const rate =
    props.rates.find((option) => option.id === selectedModelId) ??
    props.rates[0]

  useEffect(() => () => window.clearTimeout(durationTimer.current), [])

  if (!rate) return null

  const credits = rate.creditsPerSecond * seconds
  const displayedCredits = formatNumber(Math.round(credits * 10) / 10, locale)
  // The budgeting quote explicitly presents USD even when the wallet's
  // configured display currency differs.
  const displayedUsd = new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: 'USD',
    currencyDisplay: 'narrowSymbol',
  }).format(creditsToUsd(credits))

  return (
    <section
      id={ESTIMATOR_SECTION_ID}
      aria-labelledby={titleId}
      className='scroll-mt-24 space-y-4'
    >
      <div className='space-y-2'>
        <Badge variant='outline'>{t('Cost estimator')}</Badge>
        <h2
          id={titleId}
          className='text-2xl font-semibold tracking-tight sm:text-3xl'
        >
          {t('Roughly how much is a 15-second product video?')}
        </h2>
      </div>
      <Card className='grid gap-0 py-0 md:grid-cols-[1fr_0.8fr]'>
        <CardContent className='space-y-6 p-4 sm:p-6'>
          <div className='space-y-3'>
            <div className='flex items-baseline justify-between gap-4'>
              <span id={durationId} className='font-medium'>
                {t('Video duration')}
              </span>
              <span className='font-mono tabular-nums'>
                {t('{{value}} seconds', {
                  value: formatNumber(seconds, locale),
                })}
              </span>
            </div>
            <Slider
              aria-labelledby={durationId}
              locale={locale}
              format={{ style: 'unit', unit: 'second' }}
              min={MIN_DURATION}
              max={MAX_DURATION}
              step={1}
              value={[seconds]}
              onValueChange={(values) => {
                const next = typeof values === 'number' ? values : values[0]
                if (next === undefined || next === seconds) return
                setSeconds(next)
                window.clearTimeout(durationTimer.current)
                durationTimer.current = window.setTimeout(() => {
                  trackPricingEvent(
                    PRICING_ANALYTICS_EVENTS.ESTIMATOR_INTERACT,
                    {
                      model_id: rate.id,
                      seconds: next,
                    }
                  )
                }, ESTIMATOR_DEBOUNCE_MS)
              }}
            />
            <div
              className='text-muted-foreground flex justify-between text-xs'
              aria-hidden='true'
            >
              <span>
                {t('{{value}} seconds', {
                  value: formatNumber(MIN_DURATION, locale),
                })}
              </span>
              <span>
                {t('{{value}} seconds', {
                  value: formatNumber(MAX_DURATION, locale),
                })}
              </span>
            </div>
          </div>
          <div className='space-y-2'>
            <Label htmlFor={modelId}>{t('Estimator model')}</Label>
            <NativeSelect
              className='w-full'
              id={modelId}
              value={rate.id}
              onChange={(event) => {
                const nextId = event.currentTarget.value
                if (!props.rates.some((option) => option.id === nextId)) return
                window.clearTimeout(durationTimer.current)
                setSelectedModelId(nextId)
                trackPricingEvent(PRICING_ANALYTICS_EVENTS.ESTIMATOR_INTERACT, {
                  model_id: nextId,
                  seconds,
                })
              }}
            >
              {props.rates.map((option) => (
                <NativeSelectOption key={option.id} value={option.id}>
                  {t('{{model}} · {{rate}} credits/s', {
                    model: option.label,
                    rate: formatVideoCreditsPerSecond(
                      option.creditsPerSecond,
                      locale
                    ),
                  })}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>
        </CardContent>
        <CardContent className='border-border bg-muted/40 flex flex-col justify-center gap-3 border-t p-4 sm:p-6 md:border-t-0 md:border-l'>
          <h3 className='font-medium'>{t('Estimated cost')}</h3>
          <output className='space-y-1 tabular-nums' aria-live='polite'>
            <span className='block text-3xl font-semibold tracking-tight'>
              {t('{{amount}} credits', { amount: displayedCredits })}
            </span>
            <span className='text-muted-foreground block font-mono'>
              ≈ {displayedUsd}
            </span>
          </output>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Based on the published model rate for budgeting. Actual charges follow your model, resolution, task records, and wallet settings.'
            )}
          </p>
        </CardContent>
      </Card>
    </section>
  )
}
