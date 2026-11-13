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
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import {
  HERO_EQUATION,
  HERO_LOOKS,
  STRIP_IMAGES,
} from '../../lib/influencer-looks'
import { prefersReducedMotion } from '../../hooks/motion-prefs'
import { Reveal } from '../reveal'

/** Two counter-scrolling rows of real studio output. */
export function InfluencerMarquee() {
  const { t } = useTranslation()
  const rows = [STRIP_IMAGES.slice(0, 10), STRIP_IMAGES.slice(10)]
  return (
    <section className='inf-marquee' aria-label={t('influencer_landing.marquee.label')}>
      <p className='inf-marquee-label'>{t('influencer_landing.marquee.label')}</p>
      {rows.map((row, i) => {
        const stripName = i === 0 ? "marquee-top" : "marquee-bottom";
        return (
          <div key={stripName} className={`inf-strip ${i === 0 ? "inf-strip--ltr" : "inf-strip--rtl"}`}>
            {row.map((src) => (
              <img
                key={`${stripName}-orig-${src}`}
                src={src}
                alt={t("influencer_landing.marquee.label")}
                loading="lazy"
                decoding="async"
              />
            ))}
            {row.map((src) => (
              <img
                key={`${stripName}-dup-${src}`}
                src={src}
                alt={t("influencer_landing.marquee.label")}
                aria-hidden="true"
                loading="lazy"
                decoding="async"
              />
            ))}
          </div>
        );
      })}
    </section>
  )
}

type MockKind = 'traits' | 'tier' | 'generate' | 'multiview'

interface StepDef {
  num: string
  titleKey: string
  descKey: string
  mock: MockKind
  guide?: { href: string; labelKey: string }
}

const STEPS: StepDef[] = [
  {
    num: '01',
    titleKey: 'influencer_landing.step1.title',
    descKey: 'influencer_landing.step1.desc',
    mock: 'traits',
    guide: {
      href: '/guides/create-ai-character',
      labelKey: 'Read the character guide',
    },
  },
  { num: '02', titleKey: 'influencer_landing.step2.title', descKey: 'influencer_landing.step2.desc', mock: 'tier' },
  {
    num: '03',
    titleKey: 'influencer_landing.step3.title',
    descKey: 'influencer_landing.step3.desc',
    mock: 'generate',
    guide: {
      href: '/guides/recreate-video-with-your-character',
      labelKey: 'Read the reference video guide',
    },
  },
  { num: '04', titleKey: 'influencer_landing.step4.title', descKey: 'influencer_landing.step4.desc', mock: 'multiview' },
]

function StepMock(props: { kind: MockKind }) {
  const { t } = useTranslation()
  const reduceMotion = prefersReducedMotion()

  if (props.kind === 'traits') {
    return (
      <div className='inf-mock-body'>
        <span className='inf-mock-label'>{t('influencer_landing.step1.title')}</span>
        <div className='inf-mock-tabs'>
          <span className='inf-mock-tab is-active'>{t('influencer_landing.mock.tabDesc')}</span>
          <span className='inf-mock-tab'>📸 {t('influencer_landing.mock.tabPhoto')}</span>
        </div>
        <div className='inf-mock-input'>
          <span>{t('influencer_landing.mock.promptSample')}</span>
          <span className='inf-mock-cursor' aria-hidden='true' />
        </div>
        <div className='inf-mock-cta-bar'>
          <span className='omx-btn-primary inf-mock-btn-action'>
            ✨ {t('influencer_landing.mock.createBtn')}
          </span>
        </div>
      </div>
    )
  }

  if (props.kind === 'tier') {
    return (
      <div className='inf-mock-body'>
        <span className='inf-mock-label'>{t('influencer_landing.step2.title')}</span>
        <div className='inf-mock-tweak-wrap'>
          <div className='inf-mock-avatar-card'>
            <img src={HERO_LOOKS[0].image} alt='' aria-hidden='true' />
            <span className='inf-mock-avatar-badge'>{t('influencer_landing.mock.avatarConsistent')}</span>
          </div>
          <div className='inf-mock-tweak-controls'>
            <div className='inf-mock-chips-flow'>
              <span className='inf-mock-chip'>{t('influencer_landing.mock.chip1')}</span>
              <span className='inf-mock-chip is-picked'>{t('influencer_landing.mock.chip2')}</span>
              <span className='inf-mock-chip'>{t('influencer_landing.mock.chip3')}</span>
            </div>

          </div>
        </div>
      </div>
    )
  }

  if (props.kind === 'generate') {
    return (
      <div className='inf-mock-body'>
        <span className='inf-mock-label'>{t('influencer_landing.step3.title')}</span>
        <div className='inf-mock-vids-grid'>
          <div className='inf-mock-vid-card is-active'>
            <span className='inf-mock-vid-check' aria-hidden='true'>✓</span>
            {reduceMotion ? (
              <img src={HERO_EQUATION.anyVideo.poster} alt='' />
            ) : (
              <video src={HERO_EQUATION.anyVideo.src} poster={HERO_EQUATION.anyVideo.poster} autoPlay muted loop playsInline />
            )}
            <span className='inf-mock-vid-tag'>{t('influencer_landing.mock.clipWalk')}</span>
          </div>
          <div className='inf-mock-vid-card is-dim'>
            <img src={STRIP_IMAGES[0]} alt='' />
            <span className='inf-mock-vid-tag'>{t('influencer_landing.mock.clipDance')}</span>
          </div>
          <div className='inf-mock-vid-card is-dim'>
            <img src={STRIP_IMAGES[2]} alt='' />
            <span className='inf-mock-vid-tag'>{t('influencer_landing.mock.clipSpeech')}</span>
          </div>
          <div className='inf-mock-vid-card inf-mock-vid-upload'>
            <span className='inf-mock-upload-sym' aria-hidden='true'>⬆️</span>
            <span className='inf-mock-vid-tag'>{t('influencer_landing.mock.uploadClip')}</span>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className='inf-mock-body'>
      <span className='inf-mock-label'>{t('influencer_landing.step4.title')}</span>
      <div className='inf-mock-result-box'>
        {reduceMotion ? (
          <img src={HERO_EQUATION.yourVideo.poster} alt='' className='inf-mock-final-video' />
        ) : (
          <video src={HERO_EQUATION.yourVideo.src} poster={HERO_EQUATION.yourVideo.poster} autoPlay muted loop playsInline className='inf-mock-final-video' />
        )}
        <div className='inf-mock-pill-status'>
          <span className='inf-mock-live-dot' aria-hidden='true' />
          <span>{t('influencer_landing.mock.renderReady')}</span>
        </div>
      </div>
      <div className='inf-mock-actions-row'>
        <span className='omx-btn-secondary inf-mock-action-btn'>
          ⬇️ {t('influencer_landing.mock.downloadBtn')}
        </span>

      </div>
    </div>
  )
}

/**
 * Alternating four-step timeline. Scroll position drives the brand-violet
 * beam fill; each step card fades in via the shared Reveal wrapper.
 */
export function InfluencerSteps() {
  const { t } = useTranslation()
  const trackRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const track = trackRef.current
    if (!track || prefersReducedMotion()) {
      trackRef.current?.style.setProperty('--inf-progress', '1')
      return
    }
    let raf = 0
    const update = () => {
      const rect = track.getBoundingClientRect()
      const vh = window.innerHeight
      const total = rect.height + vh * 0.6
      const done = Math.min(1, Math.max(0, (vh * 0.8 - rect.top) / total))
      track.style.setProperty('--inf-progress', done.toFixed(3))
      raf = 0
    }
    const onScroll = () => {
      if (!raf) raf = requestAnimationFrame(update)
    }
    update()
    window.addEventListener('scroll', onScroll, { passive: true })
    window.addEventListener('resize', onScroll)
    return () => {
      if (raf) cancelAnimationFrame(raf)
      window.removeEventListener('scroll', onScroll)
      window.removeEventListener('resize', onScroll)
    }
  }, [])

  return (
    <section className='inf-steps' id='how'>
      <div className='inf-section'>
        <div className='inf-steps-head'>
          <p className='inf-eyebrow'>{t('influencer_landing.steps.eyebrow')}</p>
          <h2 className='inf-h2'>{t('influencer_landing.steps.title')}</h2>
          <p className='inf-sub'>{t('influencer_landing.steps.sub')}</p>
        </div>
        <div className='inf-steps-track' ref={trackRef}>
          <div className='inf-steps-line' aria-hidden='true'>
            <div className='inf-steps-fill' />
          </div>
          {STEPS.map((step) => (
            <div key={step.num} className='inf-step'>
              <span className='inf-step-dot' aria-hidden='true'>{step.num}</span>
              <Reveal className='inf-step-copy'>
                <span className='inf-step-num'>
                  {t('influencer_landing.steps.step', { n: Number(step.num) })}
                </span>
                <h3 className='inf-step-title'>{t(step.titleKey)}</h3>
                <p className='inf-step-desc'>{t(step.descKey)}</p>
                {step.guide ? (
                  <a href={step.guide.href} className='inf-guide-link'>
                    {t(step.guide.labelKey)} <span aria-hidden='true'>↗</span>
                  </a>
                ) : null}
              </Reveal>
              <Reveal className='inf-mock' delay={120}>
                <div className='inf-mock-bar' aria-hidden='true'>
                  <i /><i /><i />
                </div>
                <StepMock kind={step.mock} />
              </Reveal>
            </div>
          ))}
        </div>
      </div>
    </section>
  )
}
