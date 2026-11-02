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
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'

import { buildModelPagePricing } from '../lib/model-page-pricing'
import type { PricingModel } from '../types'

/**
 * Cost-planning blocks above the model's rate table.
 *
 * Mirrors the benchmark page's "first test cost / budget reference / cost
 * examples" trio. Values come from `buildModelPagePricing` (catalog-derived);
 * blocks with no data are omitted rather than filled with placeholder numbers.
 */
export function ModelPagePricingPlan(props: {
  model: PricingModel
  priceRate: number
  usdExchangeRate: number
}) {
  const { t } = useTranslation()
  const view = useMemo(
    () => buildModelPagePricing(props.model, props.priceRate, props.usdExchangeRate),
    [props.model, props.priceRate, props.usdExchangeRate]
  )

  const formatUsd = (usd: number) => {
    if (usd === 0) return '$0'
    if (usd < 0.01) return `$${usd.toFixed(4)}`
    return `$${usd.toFixed(2)}`
  }
  const formatCredits = (credits: number) => credits.toFixed(2)
  const modelName = props.model.model_name

  return (
    <div className='space-y-5'>
      <div>
        <h3
          data-parity='pricing.intro.title'
          className='text-base font-semibold'
        >
          {t('Before you top up, understand the test cost of {{model}}.', {
            model: modelName,
          })}
        </h3>
        <p
          data-parity='pricing.intro.desc'
          className='text-muted-foreground mt-1 text-sm'
        >
          {t(
            'Start from the cost of a single sample and pick a test budget. Expand the rate table below for the full ladder.'
          )}
        </p>
      </div>

      {view.firstTest && (
        <div className='bg-card/60 space-y-3 rounded-xl border p-4'>
          <h4
            data-parity='pricing.first-test.title'
            className='text-sm font-medium'
          >
            {t('First test cost')}
          </h4>
          <div
            data-parity='pricing.first-test.card'
            className='flex flex-wrap items-baseline gap-x-4 gap-y-1 text-sm'
          >
            <span className='text-muted-foreground'>
              {t('Cheapest request on this model')}
            </span>
            <span className='text-foreground font-medium'>
              {formatUsd(view.firstTest.usd)}
            </span>
            <span className='text-muted-foreground text-xs'>
              {formatCredits(view.firstTest.credits)} {t('credits')}
            </span>
          </div>
        </div>
      )}

      {view.budgetRows.length > 0 && (
        <div className='space-y-2'>
          <h4
            data-parity='pricing.budget.title'
            className='text-sm font-medium'
          >
            {t('Test budget reference')}
          </h4>
          <div data-parity='pricing.budget.rows' className='flex flex-wrap gap-2'>
            {view.budgetRows.map((row) => (
              <Badge
                key={row.budgetUsd}
                variant='secondary'
                className='rounded-full px-3 py-1 text-xs font-normal'
              >
                {t('${{amount}} → about {{runs}} runs', {
                  amount: row.budgetUsd,
                  runs: row.runs,
                })}
              </Badge>
            ))}
          </div>
        </div>
      )}

      {view.examples.length > 1 && (
        <div className='space-y-2'>
          <h4
            data-parity='pricing.examples.title'
            className='text-sm font-medium'
          >
            {t('Common cost examples')}
          </h4>
          <div
            data-parity='pricing.examples.cards'
            className='grid gap-2 sm:grid-cols-3'
          >
            {view.examples.map((example) => (
              <div
                key={example.label}
                className='bg-card/60 rounded-lg border p-3 text-xs'
              >
                <div className='text-foreground/80 font-medium'>
                  {t(example.label)}
                </div>
                <div className='text-foreground mt-1 text-sm'>
                  {formatUsd(example.usd)}
                </div>
                <div className='text-muted-foreground'>
                  {formatCredits(example.credits)} {t('credits')}
                </div>
              </div>
            ))}
          </div>
          <p className='text-muted-foreground text-xs'>
            {t(
              'Estimated per request from the rates listed for this model. Final cost varies by mode, reference material and add-ons.'
            )}
          </p>
        </div>
      )}
    </div>
  )
}
