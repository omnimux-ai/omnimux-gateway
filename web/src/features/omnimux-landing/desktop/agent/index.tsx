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
import { useCallback, useEffect, useState } from 'react'

import '../assets/cn/fonts/smiley/result.css'
import '../desktop-cn.css'
import './agent.css'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import { DownloadProvider } from '../components/cn/download'
import { CnFooter } from '../components/cn/footer'
import { Icon, IconSprite } from '../components/cn/icons'
import { CnNav } from '../components/cn/nav'
import { LandingLangProvider, landingLangOption, useLanding } from '../i18n'
import type { LandingLang } from '../i18n/types'
import { AgentCards, AgentKindTabs } from './components/agent-cards'
import {
  AGENT_NAMES,
  AGENT_PATHS,
  type AgentKind,
  type AgentLocale,
} from './content'
import { AGENT_PAGES } from './locales'

export type AgentPageProps = {
  kind: AgentKind
  locale: AgentLocale
}

/**
 * The CLI / Skills entry page. The URL pins the language: `/zh/…` renders
 * zh-CN, `/ja/…` Japanese, and `/…` English, whatever the visitor stored
 * before. Switching language navigates to that language's URL.
 */
export function AgentPage(props: AgentPageProps) {
  const navigate = useNavigate()
  const pinned: LandingLang = props.locale
  const kind = props.kind

  const onLangChange = useCallback(
    (next: LandingLang) => {
      void navigate({ href: AGENT_PATHS[next][kind] })
    },
    [kind, navigate]
  )

  return (
    <LandingLangProvider lang={pinned} onLangChange={onLangChange}>
      <AgentPageView {...props} />
    </LandingLangProvider>
  )
}

function AgentPageView(props: AgentPageProps) {
  const { lang, copy } = useLanding()
  const router = useRouter()
  const page = AGENT_PAGES[props.locale][props.kind]

  // The global page-metadata sync rewrites <title> after each navigation;
  // re-assert this page's title once it has run (same as the tool pages).
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
          <section className='agent-hero'>
            <div className='hero-glow' aria-hidden='true' />
            <div className='wrap'>
              <span className='agent-badge'>{page.badge}</span>
              <h1 className='h1'>{page.h1}</h1>
              <p className='agent-sub'>{page.subtitle}</p>
              <AgentKindTabs
                current={props.kind}
                paths={AGENT_PATHS[props.locale]}
                label={page.kindAria}
                labels={{ cli: page.kindCli, skills: page.kindSkills }}
              />
              <AgentInstall page={page} />
            </div>
          </section>

          <section className='sec'>
            <div className='wrap'>
              <div className='sec-head'>
                <h2 className='h2'>{page.compatTitle}</h2>
                <p className='lead'>{page.compatLead}</p>
              </div>
              <div className='agent-flow'>
                <div className='agent-flow-col'>
                  {AGENT_NAMES.map((name) => (
                    <div className='agent-chip' key={name}>
                      <i aria-hidden='true'>{name.charAt(0)}</i>
                      <span>{name}</span>
                    </div>
                  ))}
                </div>
                <div className='agent-hub'>
                  <span className='agent-hub-logo'>
                    <img src='/logo.svg' alt='' />
                  </span>
                  <b>{page.hubTitle}</b>
                  <small>{page.hubSub}</small>
                </div>
                <div className='agent-flow-col end'>
                  {page.flowCaps.map((cap) => (
                    <div className='agent-cap' key={cap.title}>
                      <Icon name='spark' />
                      <span>
                        <b>{cap.title}</b>
                        <span>{cap.body}</span>
                      </span>
                    </div>
                  ))}
                </div>
              </div>
              <div className='agent-flow-labels'>
                <span>{page.flowLabels.agents}</span>
                <span>{page.flowLabels.hub}</span>
                <span>{page.flowLabels.caps}</span>
              </div>
            </div>
          </section>

          <section className='sec sec-tight'>
            <div className='wrap'>
              <div className='sec-head'>
                <h2 className='h2'>{page.coreTitle}</h2>
                <p className='lead'>{page.coreLead}</p>
              </div>
              <AgentCards cards={page.cards} />
            </div>
          </section>

          <section className='sec sec-tight'>
            <div className='wrap'>
              <div className='sec-head'>
                <h2 className='h2'>{page.tryTitle}</h2>
              </div>
              <AgentTry page={page} />
            </div>
          </section>

          <section className='sec sec-tight'>
            <div className='wrap'>
              <div className='sec-head'>
                <h2 className='h2'>{page.faqTitle}</h2>
              </div>
              <div className='faq-list agent-faq'>
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
            </div>
          </section>

          <section className='sec sec-tight'>
            <div className='wrap'>
              <div className='agent-cta'>
                <div>
                  <h3>{page.ctaTitle}</h3>
                  <p>{page.ctaBody}</p>
                </div>
                <Link className='btn btn-primary' to='/keys'>
                  {page.ctaButton}
                  <Icon name='right' />
                </Link>
              </div>
            </div>
          </section>
        </main>
        <CnFooter anchorBase='/' />
      </DownloadProvider>
    </div>
  )
}

/** Agent picker + step rail + the selected step's command. */
function AgentInstall(props: { page: (typeof AGENT_PAGES)['en']['cli'] }) {
  const page = props.page
  const [agent, setAgent] = useState<(typeof AGENT_NAMES)[number]>(
    AGENT_NAMES[0]
  )
  const [step, setStep] = useState(0)
  const { copyToClipboard, copiedText } = useCopyToClipboard({ notify: false })
  const current = page.steps[step]

  return (
    <div className='agent-install'>
      <div className='agent-tabs' role='tablist' aria-label={page.kindAria}>
        {AGENT_NAMES.map((name) => (
          <button
            key={name}
            type='button'
            role='tab'
            aria-selected={name === agent}
            onClick={() => setAgent(name)}
          >
            <span className='dot' aria-hidden='true' />
            {name}
          </button>
        ))}
      </div>
      <div className='agent-steps'>
        <div className='agent-rail' role='tablist' aria-label={page.stepLabel}>
          {page.steps.map((item, index) => (
            <button
              key={item.title}
              type='button'
              role='tab'
              aria-current={index === step ? 'step' : undefined}
              aria-selected={index === step}
              onClick={() => setStep(index)}
            >
              <i aria-hidden='true'>{index + 1}</i>
              {item.title}
            </button>
          ))}
        </div>
        <div className='agent-stepbody'>
          <h3>
            {page.stepLabel} · {current.title}
          </h3>
          {current.command && (
            <div className='agent-code'>
              <code>{current.command}</code>
              <button
                className='agent-copy'
                type='button'
                aria-label={
                  copiedText === current.command ? page.copied : page.copy
                }
                onClick={() => void copyToClipboard(current.command ?? '')}
              >
                <Icon
                  name={copiedText === current.command ? 'check' : 'copy'}
                />
              </button>
            </div>
          )}
          {current.action && (
            <Link className='btn btn-sm btn-primary' to={current.action.href}>
              {current.action.label}
            </Link>
          )}
          <p>{current.body}</p>
        </div>
      </div>
    </div>
  )
}

/** Prompt list beside a static chat transcript. */
function AgentTry(props: { page: (typeof AGENT_PAGES)['en']['cli'] }) {
  const page = props.page
  const { copyToClipboard, copiedText } = useCopyToClipboard({ notify: false })
  const chat = page.tryChat

  return (
    <div className='agent-try'>
      <div>
        <h2>{page.tryHeading}</h2>
        <p>{page.tryBody}</p>
        {page.tryPrompts.map((prompt) => (
          <div className='agent-tryrow' key={prompt.label}>
            <b>{prompt.label}</b>
            <div className='agent-code'>
              <code>{prompt.text}</code>
              <button
                className='agent-copy'
                type='button'
                aria-label={
                  copiedText === prompt.text ? page.copied : page.copy
                }
                onClick={() => void copyToClipboard(prompt.text)}
              >
                <Icon name={copiedText === prompt.text ? 'check' : 'copy'} />
              </button>
            </div>
          </div>
        ))}
      </div>
      <div className='agent-chat'>
        <div className='agent-chat-bar' aria-hidden='true'>
          <i />
          <i />
          <i />
        </div>
        <div className='agent-chat-body'>
          <div className='agent-msg user'>{chat.prompt}</div>
          {chat.done.length > 0 ? (
            <div className='agent-msg bot'>
              {chat.done.map((row) => (
                <div className='agent-kv' key={row.label}>
                  <i>{row.label}</i>
                  <b>{row.value}</b>
                </div>
              ))}
            </div>
          ) : null}
          <div className='agent-thinking'>
            <span className='ok' aria-hidden='true'>
              ✓
            </span>
            {chat.follow}
          </div>
        </div>
      </div>
    </div>
  )
}
