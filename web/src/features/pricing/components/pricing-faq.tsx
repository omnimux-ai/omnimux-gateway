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
import { useTranslation } from 'react-i18next'

import {
  Accordion,
  AccordionContent,
  AccordionItem,
  AccordionTrigger,
} from '@/components/ui/accordion'

import {
  PRICING_ANALYTICS_EVENTS,
  trackPricingEvent,
} from '../lib/pricing-analytics'

export function PricingFaq() {
  const { t } = useTranslation()
  const [openItems, setOpenItems] = useState<string[]>(['q1'])
  const items = [
    {
      id: 'q1',
      question: t('How are credits deducted?'),
      answer: t(
        'Depending on the model, charges are based on seconds, generations, tokens, or tasks. Check individual charges in your task records or logs.'
      ),
    },
    {
      id: 'q2',
      question: t('What happens if a generation fails?'),
      answer: t(
        'For system errors, credit returns follow task and wallet records.'
      ),
    },
    {
      id: 'q3',
      question: t('Can I see the exact price before generating a video?'),
      answer: t(
        'The image studio shows a reference quote. The video estimator helps with budgeting. Actual charges follow your task and wallet records.'
      ),
    },
    {
      id: 'q4',
      question: t('Can I keep using OmniMux without a subscription?'),
      answer: t(
        'Yes. OmniMux uses pay-per-generation credits and wallet top-up tiers.'
      ),
    },
    {
      id: 'q5',
      question: t('Which payment methods are supported?'),
      answer: t(
        'Card, Alipay/WeChat, and wire transfer are supported. Check your wallet for available methods.'
      ),
    },
  ]

  function handleOpenItemsChange(nextItems: string[]) {
    setOpenItems(nextItems)
    for (const { id } of items) {
      const expanded = nextItems.includes(id)
      if (expanded !== openItems.includes(id)) {
        trackPricingEvent(PRICING_ANALYTICS_EVENTS.FAQ_TOGGLE, {
          faq_id: id,
          expanded,
        })
      }
    }
  }

  return (
    <section
      id='faq'
      aria-labelledby='pricing-faq-heading'
      className='scroll-mt-24'
    >
      <h2
        id='pricing-faq-heading'
        className='text-foreground mb-4 text-2xl font-bold tracking-tight sm:text-3xl'
      >
        {t('Clear answers before you top up')}
      </h2>
      <Accordion
        value={openItems}
        multiple
        onValueChange={handleOpenItemsChange}
        className='gap-2'
      >
        {items.map((item) => (
          <AccordionItem
            key={item.id}
            value={item.id}
            className='bg-card rounded-2xl border px-4 sm:px-6'
          >
            <AccordionTrigger className='py-4 text-base'>
              {item.question}
            </AccordionTrigger>
            <AccordionContent className='text-muted-foreground pb-4 text-pretty'>
              {item.answer}
            </AccordionContent>
          </AccordionItem>
        ))}
      </Accordion>
    </section>
  )
}
