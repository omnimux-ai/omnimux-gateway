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
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { USD_PER_CREDIT } from '../constants'

export type BillingMethodStepsProps = {
  /** Live catalog size for step 02 footer; omit for generic copy. */
  modelCount?: number
  className?: string
}

type StepDef = {
  number: string
  titleKey: string
  descriptionKey: string
  footer: string
}

export function BillingMethodSteps(props: BillingMethodStepsProps) {
  const { t } = useTranslation()

  const modelFooter =
    typeof props.modelCount === 'number' && props.modelCount > 0
      ? t('One API → {{count}} models', { count: props.modelCount })
      : t('One API → production models')

  const steps: StepDef[] = [
    {
      number: '01',
      titleKey: 'Top up credits',
      descriptionKey: 'Pay with card, Alipay/WeChat, or wire transfer.',
      footer: t('1 credit = ${{usd}} USD', {
        usd: USD_PER_CREDIT.toFixed(2),
      }),
    },
    {
      number: '02',
      titleKey: 'Call any model',
      descriptionKey:
        'Video and audio billed per second, images per generation, LLMs per token. Unit prices sit on each model card — no hidden multipliers.',
      footer: modelFooter,
    },
    {
      number: '03',
      titleKey: 'Every cent is traceable',
      descriptionKey:
        'Usage logs update in milliseconds with tokens, latency, and cost per call. Export by API key, model, and time range.',
      footer: t('Failed tasks return generation credits'),
    },
  ]

  return (
    <section
      className={cn('mb-12 sm:mb-16', props.className)}
      aria-labelledby='billing-method-heading'
    >
      <div className='border-border/60 mb-6 flex flex-col gap-2 border-b pb-5 sm:flex-row sm:items-baseline sm:justify-between'>
        <h2
          id='billing-method-heading'
          className='text-foreground text-2xl font-bold tracking-tight sm:text-3xl'
        >
          {t('How billing works')}
        </h2>
        <p className='text-muted-foreground text-xs sm:text-sm'>
          {t(
            'One balance, every model. Metered per call, visible live in the console.'
          )}
        </p>
      </div>

      <div className='bg-card border-border grid grid-cols-1 divide-y rounded-sm border md:grid-cols-3 md:divide-x md:divide-y-0'>
        {steps.map((step) => (
          <div
            key={step.number}
            className='hover:bg-muted/30 flex flex-col justify-between p-6 transition-colors sm:p-7'
          >
            <div>
              <div className='mb-4 flex items-center justify-between'>
                <span className='text-foreground font-mono text-3xl font-bold tracking-tight sm:text-4xl'>
                  {step.number}
                </span>
                <div
                  className='border-border mx-4 h-px flex-1 border-t border-dashed'
                  aria-hidden='true'
                />
                <span
                  className='bg-success inline-block size-2.5 shrink-0 rounded-full shadow-[0_0_8px_color-mix(in_oklab,var(--success)_60%,transparent)]'
                  aria-hidden='true'
                />
              </div>

              <h3 className='text-foreground mb-2.5 text-lg font-bold tracking-tight'>
                {t(step.titleKey)}
              </h3>

              <p className='text-muted-foreground text-xs leading-relaxed sm:text-sm'>
                {t(step.descriptionKey)}
              </p>
            </div>

            <div className='mt-6 pt-2'>
              <div className='bg-background border-border text-foreground flex items-center justify-between rounded-sm border p-3 font-mono text-xs'>
                <span>{step.footer}</span>
              </div>
            </div>
          </div>
        ))}
      </div>
    </section>
  )
}
