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
import { ArrowDown, MessageSquareCode } from 'lucide-react'
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

/** Anchor target for the primary CTA (aligned with PRD / architecture). */
export const PRICING_MODELS_SECTION_ID = 'pricing-models'

const ENTERPRISE_MAILTO = 'mailto:sales@omnimux.ai'

const FEATURE_PILL_KEYS = [
  'Check prices and wallet allowances',
  'Review generation history',
  'Failed tasks return generation credits',
] as const

export type BillingHeroProps = {
  /** Live model count for the subtitle; omit to use a generic phrase. */
  modelCount?: number
  /** Override scroll target id (default: pricing-models). */
  scrollTargetId?: string
  onScrollToModels?: () => void
  onConsultEnterprise?: () => void
  className?: string
}

export function BillingHero(props: BillingHeroProps) {
  const { t } = useTranslation()
  const scrollTargetId = props.scrollTargetId || PRICING_MODELS_SECTION_ID

  const handleScrollClick = useCallback(() => {
    if (props.onScrollToModels) {
      props.onScrollToModels()
      return
    }
    const target = document.querySelector(`#${CSS.escape(scrollTargetId)}`)
    if (target) {
      target.scrollIntoView({ behavior: 'smooth', block: 'start' })
    }
  }, [props, scrollTargetId])

  const handleEnterpriseClick = useCallback(() => {
    if (props.onConsultEnterprise) {
      props.onConsultEnterprise()
      return
    }
    toast.info(
      t('Opening enterprise pricing contact… sales@omnimux.ai')
    )
    window.location.assign(ENTERPRISE_MAILTO)
  }, [props, t])

  const subtitle =
    typeof props.modelCount === 'number' && props.modelCount > 0
      ? t(
          'Top up and pay per generation. This catalog lists {{count}} available models; prices and allowances follow your wallet settings.',
          { count: props.modelCount }
        )
      : t(
          'Top up and pay per generation. Check current prices and allowances in your wallet.'
        )

  return (
    <section
      className={cn(
        'pt-6 pb-10 text-center sm:pt-10 sm:pb-14',
        props.className
      )}
      aria-labelledby='billing-hero-heading'
    >
      <div className='mb-4 inline-flex items-center justify-center'>
        <span className='bg-card text-primary border-border rounded-sm border px-2.5 py-1 font-mono text-xs font-semibold tracking-widest uppercase sm:text-sm'>
          {t('Pricing')}
        </span>
      </div>

      <h1
        id='billing-hero-heading'
        className='text-foreground mb-4 text-4xl leading-[1.1] font-bold tracking-tight sm:mb-5 sm:text-5xl lg:text-6xl'
      >
        {t('Pay for what you use')}
      </h1>

      <p className='text-muted-foreground mx-auto mb-8 max-w-2xl px-2 text-sm leading-relaxed sm:max-w-3xl sm:text-base'>
        {subtitle}
      </p>

      <div className='mx-auto mb-8 flex max-w-4xl flex-wrap items-center justify-center gap-2 px-2 sm:mb-10 sm:gap-3'>
        {FEATURE_PILL_KEYS.map((key) => (
          <div
            key={key}
            className='bg-card text-foreground border-border hover:border-border/80 inline-flex items-center rounded-sm border px-3.5 py-1.5 text-xs transition-colors select-none sm:text-sm'
          >
            <span
              className='text-success mr-2 font-mono text-xs'
              aria-hidden='true'
            >
              ✓
            </span>
            <span>{t(key)}</span>
          </div>
        ))}
      </div>

      <div className='flex flex-col items-center justify-center gap-3 px-4 sm:flex-row sm:gap-4'>
        <Button
          type='button'
          size='lg'
          onClick={handleScrollClick}
          className='w-full gap-2 sm:w-auto'
        >
          <span>{t('View model prices')}</span>
          <ArrowDown className='size-4' aria-hidden='true' />
        </Button>

        <Button
          type='button'
          variant='outline'
          size='lg'
          onClick={handleEnterpriseClick}
          className='bg-card w-full gap-2 sm:w-auto'
        >
          <MessageSquareCode
            className='text-muted-foreground size-4'
            aria-hidden='true'
          />
          <span>{t('Enterprise pricing')}</span>
        </Button>
      </div>
    </section>
  )
}
