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
import { Link, useNavigate, useRouter } from '@tanstack/react-router'
import { useCallback, useEffect } from 'react'

import '../assets/cn/fonts/smiley/result.css'
import '../desktop-cn.css'
import './tools.css'
import { ASSETS } from '../components/cn/data'
import { DownloadProvider } from '../components/cn/download'
import { useDownload } from '../components/cn/download-context'
import { CnFooter } from '../components/cn/footer'
import { Icon, IconSprite } from '../components/cn/icons'
import { CnNav } from '../components/cn/nav'
import { LandingLangProvider, landingLangOption, useLanding } from '../i18n'
import type { LandingLang } from '../i18n/types'
import { ToolForm } from './components/tool-form'
import {
  TOOL_PAGES,
  TOOL_PATHS,
  type ToolKind,
  type ToolLocale,
} from './content'

export type TikTokToolPageProps = {
  kind: ToolKind
  locale: ToolLocale
  /** `?url=` prefill; never auto-submitted. */
  initialUrl?: string
}

/**
 * A TikTok downloader page. The URL decides the language: `/zh/tools/…`
 * renders zh-CN and `/tools/…` renders English, whatever was stored before.
 */
export function TikTokToolPage(props: TikTokToolPageProps) {
  const navigate = useNavigate()
  const pinned: LandingLang = props.locale
  const kind = props.kind

  const onLangChange = useCallback(
    (next: LandingLang) => {
      const target: ToolLocale =
        next === 'zh-CN' || next === 'zh-TW' ? 'zh-CN' : 'en'
      void navigate({ href: TOOL_PATHS[target][kind] })
    },
    [kind, navigate]
  )

  return (
    <LandingLangProvider lang={pinned} onLangChange={onLangChange}>
      <ToolPageView {...props} />
    </LandingLangProvider>
  )
}

function ToolPageView(props: TikTokToolPageProps) {
  const { lang, copy } = useLanding()
  const router = useRouter()
  const page = TOOL_PAGES[props.locale][props.kind]
  const tool = copy.toolPage

  // The global page-metadata sync rewrites <title> after each navigation;
  // re-assert this page's title once it has run (same as the homepage).
  useEffect(
    () =>
      router.subscribe('onResolved', () => {
        document.title = page.title
        document.documentElement.lang = landingLangOption(lang).htmlLang
      }),
    [router, lang, page.title]
  )

  useEffect(() => {
    const root = document.documentElement
    const previousBg = root.style.backgroundColor
    const previousTitle = document.title
    const previousLang = root.lang
    root.style.backgroundColor = '#0C0B11'
    root.lang = landingLangOption(lang).htmlLang
    document.title = page.title
    return () => {
      root.style.backgroundColor = previousBg
      root.lang = previousLang
      document.title = previousTitle
    }
  }, [lang, page.title])

  return (
    <div className='omc js'>
      <DownloadProvider>
        <IconSprite />
        <a className='skip' href='#cn-main'>
          {copy.skip}
        </a>
        <CnNav anchorBase='/' current='tools' />
        <main id='cn-main'>
          <section className='tool'>
            <div className='hero-glow' aria-hidden='true' />
            <div className='wrap'>
              <nav className='tool-crumb' aria-label={tool.crumbAria}>
                <Link to='/'>OmniMux</Link>
                <span aria-hidden='true'>/</span>
                <a href='/#tools'>{tool.crumbTools}</a>
                <span aria-hidden='true'>/</span>
                <span aria-current='page'>{page.h1}</span>
              </nav>
              <h1 className='h1'>{page.h1}</h1>
              <p className='tool-sub'>{page.subtitle}</p>
              <nav className='c-tabs tool-kind' aria-label={tool.kindAria}>
                {(['video', 'photo'] as const).map((item) => (
                  <Link
                    key={item}
                    to={TOOL_PATHS[props.locale][item]}
                    aria-current={item === props.kind ? 'page' : undefined}
                  >
                    <Icon name={item === 'video' ? 'video' : 'img'} />
                    {item === 'video' ? tool.kindVideo : tool.kindPhoto}
                  </Link>
                ))}
              </nav>
              <ToolForm
                key={props.kind}
                kind={props.kind}
                locale={props.locale}
                initialUrl={props.initialUrl}
              />
            </div>
          </section>

          <section className='tool-sec'>
            <div className='wrap'>
              <h2 className='h2'>{page.stepsTitle}</h2>
              <ol className='tool-steps'>
                {page.steps.map((step, index) => (
                  <li className='tool-step' key={step.title}>
                    <i>{String(index + 1).padStart(2, '0')}</i>
                    <h3>{step.title}</h3>
                    <p>{step.body}</p>
                  </li>
                ))}
              </ol>
            </div>
          </section>

          <section className='tool-sec tool-sec-tight'>
            <div className='wrap'>
              <ToolRecreateBand />
            </div>
          </section>

          <section className='tool-sec tool-sec-tight'>
            <div className='wrap'>
              <h2 className='h2'>{page.faqTitle}</h2>
              <div className='faq-list tool-faq'>
                {page.faq.map((item) => (
                  <details key={item.q}>
                    <summary>
                      {item.q}
                      <span className='pm'>
                        <Icon name='plus' />
                      </span>
                    </summary>
                    <p>{item.a}</p>
                  </details>
                ))}
              </div>
              <nav className='tool-rel' aria-label={tool.relatedAria}>
                {page.related.map((item) => (
                  <Link className='skill' key={item.href} to={item.href}>
                    {item.label}
                  </Link>
                ))}
              </nav>
            </div>
          </section>
        </main>
        <CnFooter anchorBase='/' />
      </DownloadProvider>
    </div>
  )
}

function ToolRecreateBand() {
  const copy = useLanding().copy
  const tool = copy.toolPage
  const download = useDownload()
  return (
    <div className='tool-cta'>
      <div>
        <h2 className='h2'>{tool.ctaTitle}</h2>
        <p>{tool.ctaBody}</p>
        <div className='tool-btns'>
          <Link className='btn btn-primary' to='/influencer'>
            {tool.ctaStart}
          </Link>
          <button
            type='button'
            className='btn btn-ghost'
            onClick={() => {
              if (!download.downloadDirect()) download.open()
            }}
          >
            <Icon name='dl' />
            {copy.nav.download}
          </button>
        </div>
      </div>
      <div className='tool-pair'>
        <figure>
          <img
            src={ASSETS.serumPoster}
            alt=''
            loading='lazy'
            decoding='async'
          />
          <figcaption>{tool.ctaRef}</figcaption>
        </figure>
        <figure>
          <img src={ASSETS.polka} alt='' loading='lazy' decoding='async' />
          <figcaption>{tool.ctaMine}</figcaption>
        </figure>
      </div>
    </div>
  )
}
