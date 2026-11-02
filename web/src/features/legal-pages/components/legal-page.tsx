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

import { RichContent } from '@/components/rich-content'
import { LandingFooter } from '@/features/omnimux-landing/components/landing-footer'
import { LandingHeader } from '@/features/omnimux-landing/components/landing-header'
import { useInterfaceLanguage } from '@/hooks/use-interface-language'

// The legal pages wear the landing chrome, so they need its design tokens.
// Rsbuild emits the stylesheet once no matter how many modules import it.
import '@/features/omnimux-landing/landing.css'

import { resolveLegalDocument } from '../lib/content'
import type { LegalDocumentContent } from '../types'

interface LegalPageProps {
  content: LegalDocumentContent
}

export function LegalPage(props: LegalPageProps) {
  const { t } = useTranslation()
  // Same language source as the footer's switcher, so the body and the active
  // pill can never disagree.
  const { currentLanguage } = useInterfaceLanguage()
  const document = resolveLegalDocument(props.content, currentLanguage)

  return (
    <div className='omnimux-landing'>
      <LandingHeader />
      <main className='omx-shell py-16 sm:py-20'>
        <article className='mx-auto max-w-3xl'>
          <header className='border-b border-white/8 pb-8'>
            <h1 className='text-[32px] font-semibold tracking-tight text-white'>
              {document.title}
            </h1>
            <p className='mt-3 text-[14px] text-white/60'>
              {t('Last updated')} · {document.updated}
            </p>
          </header>

          <RichContent
            mode='markdown'
            content={document.body}
            className='prose-invert mt-8'
          />
        </article>
      </main>
      <LandingFooter />
    </div>
  )
}
