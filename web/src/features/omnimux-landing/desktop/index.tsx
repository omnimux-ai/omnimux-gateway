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
import { useRouter } from '@tanstack/react-router'
import { useEffect } from 'react'

import './assets/cn/fonts/smiley/result.css'
import './desktop-cn.css'
import { CnAgentDemo } from './components/cn/agent-demo'
import { CnCreate } from './components/cn/create'
import { DownloadProvider } from './components/cn/download'
import { CnFaq } from './components/cn/faq'
import { CnFinalCta } from './components/cn/final-cta'
import { CnFlow } from './components/cn/flow'
import { CnFooter } from './components/cn/footer'
import { CnHero } from './components/cn/hero'
import { IconSprite } from './components/cn/icons'
import { CnNav } from './components/cn/nav'
import { CnProof } from './components/cn/proof'
import { CnSurfaces } from './components/cn/surfaces'
import { CnTools } from './components/cn/tools'
import { CnWorkspace } from './components/cn/workspace'
import {
  LandingLangProvider,
  landingLangOption,
  useLanding,
} from './i18n'

export function DesktopLanding() {
  return (
    <LandingLangProvider>
      <LandingView />
    </LandingLangProvider>
  )
}

function LandingView() {
  const { lang, copy } = useLanding()
  const router = useRouter()

  // `history.replaceState` in the lang setter resolves as a navigation and
  // triggers the global page-metadata sync, which rewrites <title>. Re-assert
  // the localized title after that sync runs.
  useEffect(
    () =>
      router.subscribe('onResolved', () => {
        document.title = copy.meta.title
        document.documentElement.lang = landingLangOption(lang).htmlLang
      }),
    [router, lang, copy.meta.title]
  )

  useEffect(() => {
    const root = document.documentElement
    const previousBg = root.style.backgroundColor
    const previousTitle = document.title
    const previousLang = root.lang
    root.style.backgroundColor = '#0C0B11'
    root.lang = landingLangOption(lang).htmlLang
    document.title = copy.meta.title
    return () => {
      root.style.backgroundColor = previousBg
      root.lang = previousLang
      document.title = previousTitle
    }
  }, [lang, copy.meta.title])

  return (
    <div className='omc js'>
      <DownloadProvider>
        <IconSprite />
        <a className='skip' href='#cn-main'>
          {copy.skip}
        </a>
        <CnNav />
        <main id='cn-main'>
          <CnHero />
          <CnAgentDemo />
          <CnFlow />
          <CnCreate />
          <CnSurfaces />
          <CnTools />
          <CnWorkspace />
          <CnProof />
          <CnFaq />
          <CnFinalCta />
        </main>
        <CnFooter />
      </DownloadProvider>
    </div>
  )
}
