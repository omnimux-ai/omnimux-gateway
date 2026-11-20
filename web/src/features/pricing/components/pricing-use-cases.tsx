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
import { ChevronDown } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { toIntlLocale } from '@/i18n/languages'

import { creditsToUsd } from '../lib/credits'
import {
  PRICING_ANALYTICS_EVENTS,
  trackPricingEvent,
} from '../lib/pricing-analytics'
import {
  formatVideoCreditsPerSecond,
  type VideoEstimateRate,
} from '../lib/video-cost-estimate'

export function PricingUseCases(props: {
  rates: readonly VideoEstimateRate[]
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  // This table compares USD budget estimates independently of the wallet currency.
  const usdFormat = new Intl.NumberFormat(locale, {
    currency: 'USD',
    currencyDisplay: 'narrowSymbol',
    style: 'currency',
    minimumFractionDigits: 2,
    maximumFractionDigits: 3,
  })
  const videoLabel = t('Shoppable videos')
  const useCases = [
    {
      title: videoLabel,
      detail: t('Billed per second. Resolution varies by model.'),
    },
    {
      title: t('Product images'),
      detail: t('Billed per image. The image studio shows a reference quote.'),
    },
    {
      title: t('Audio and voiceover'),
      detail: t('Billed per second or per task. See the model for its rate.'),
    },
  ]

  return (
    <>
      <h2 className='text-foreground text-2xl font-semibold tracking-tight sm:text-3xl'>
        {t('Choose by use case')}
      </h2>
      <div className='grid gap-4 sm:grid-cols-3'>
        {useCases.map(({ title, detail }) => (
          <Card key={title}>
            <CardContent className='space-y-2'>
              <h3 className='text-base font-semibold'>{title}</h3>
              <p className='text-muted-foreground text-sm'>{detail}</p>
            </CardContent>
          </Card>
        ))}
      </div>
      <Collapsible
        onOpenChange={(open) => {
          if (open) trackPricingEvent(PRICING_ANALYTICS_EVENTS.CTA_VIEW_MODELS)
        }}
        className='bg-card border-border rounded-2xl border'
      >
        <CollapsibleTrigger className='group flex min-h-12 w-full items-center justify-between gap-3 px-4 py-3 text-start text-sm font-medium'>
          {t('View recommended video model rates')}
          <ChevronDown
            className='size-4 shrink-0 transition-transform group-aria-expanded:rotate-180'
            aria-hidden='true'
          />
        </CollapsibleTrigger>
        <CollapsibleContent className='border-border border-t px-4 pb-2'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead scope='col'>{t('Model')}</TableHead>
                <TableHead scope='col'>{t('Use case')}</TableHead>
                <TableHead scope='col'>{t('Unit rate')}</TableHead>
                <TableHead scope='col'>{t('Approx. USD')}</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {props.rates.map((rate, index) => (
                <TableRow key={rate.id}>
                  <TableCell>
                    {rate.label}
                    {index === 0 && (
                      <Badge variant='secondary' className='ms-2'>
                        {t('Recommended')}
                      </Badge>
                    )}
                  </TableCell>
                  <TableCell>{videoLabel}</TableCell>
                  <TableCell>
                    {t('{{amount}} credits/s', {
                      amount: formatVideoCreditsPerSecond(
                        rate.creditsPerSecond,
                        locale
                      ),
                    })}
                  </TableCell>
                  <TableCell>
                    ≈{' '}
                    {t('{{amount}}/s', {
                      amount: usdFormat.format(
                        creditsToUsd(rate.creditsPerSecond)
                      ),
                    })}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </CollapsibleContent>
      </Collapsible>
    </>
  )
}
