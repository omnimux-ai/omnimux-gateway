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
import { Link } from '@tanstack/react-router'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { HERO_EQUATION } from '../../lib/influencer-looks'
import { prefersReducedMotion } from '../../hooks/motion-prefs'

type MediaRefs = React.RefObject<Array<HTMLVideoElement | null>>

/**
 * Keeps the hero clips on one timeline: they start together once the whole pair
 * can play, then a small playback-rate nudge holds the phase, so the reference
 * move and the generated move stay in step without ever seeking.
 */
function useVideoSync(videos: MediaRefs) {
  useEffect(() => {
    if (prefersReducedMotion()) return
    const live = () =>
      (videos.current ?? []).filter((v): v is HTMLVideoElement => v !== null)
    let raf = 0
    let started = false

    const tick = () => {
      raf = requestAnimationFrame(tick)
      const items = live()
      if (items.length < 2) return
      const [lead, ...followers] = items
      if (lead.paused) return
      const shortest = Math.min(
        ...items.map((v) =>
          Number.isFinite(v.duration) && v.duration > 0 ? v.duration : Infinity
        )
      )
      if (!Number.isFinite(shortest) || shortest <= 0) return
      for (const v of followers) {
        const raw = v.currentTime - lead.currentTime
        const drift =
          ((((raw + shortest / 2) % shortest) + shortest) % shortest) - shortest / 2
        v.playbackRate = Math.min(1.06, Math.max(0.94, 1 - drift * 0.8))
      }
    }

    const startTogether = () => {
      if (started) return
      const items = live()
      if (items.length < 2 || !items.every((v) => v.readyState >= 2)) return
      started = true
      for (const v of items) void v.play().catch(() => undefined)
      tick()
    }

    const items = live()
    for (const v of items) v.addEventListener('canplay', startTogether)
    startTogether()
    return () => {
      if (raf) cancelAnimationFrame(raf)
      for (const v of items) v.removeEventListener('canplay', startTogether)
    }
  }, [videos])
}

/**
 * One media tile of the hero equation. Visitors who asked their system for
 * less motion get the poster frame instead of an autoplaying loop.
 */
function HeroMedia(props: {
  src: string
  poster: string
  label: string
  register: (el: HTMLVideoElement | null) => void
}) {
  if (prefersReducedMotion()) {
    return <img src={props.poster} alt={props.label} />
  }
  return (
    <video
      ref={props.register}
      src={props.src}
      poster={props.poster}
      aria-label={props.label}
      loop
      muted
      playsInline
      preload='auto'
    />
  )
}

export function InfluencerHero() {
  const { t } = useTranslation()
  const mediaRefs = useRef<Array<HTMLVideoElement | null>>([])
  useVideoSync(mediaRefs)

  const character = t('influencer_landing.hero.card.character')
  const anyVideo = t('influencer_landing.hero.card.anyVideo')
  const yourVideo = t('influencer_landing.hero.card.yourVideo')

  return (
    <section className='inf-hero'>
      <div className='inf-dot-bg' aria-hidden='true' />
      <div className='inf-section inf-hero-inner'>
        <span className='inf-hero-pill'>{t('influencer_landing.hero.eyebrow')}</span>
        <h1 className='inf-h1'>
          {t('influencer_landing.hero.titleLead')}
          <span className='inf-h1-accent'>{t('influencer_landing.hero.titleAccent')}</span>
          {t('influencer_landing.hero.titleTail')}
        </h1>
        <p className='inf-hero-sub'>{t('influencer_landing.hero.subline')}</p>
        <div className='inf-hero-ctas'>
          <Link to='/influencer' className='omx-btn-primary'>
            {t('influencer_landing.hero.cta')}
          </Link>
          <a href='#works' className='omx-btn-secondary'>
            {t('influencer_landing.hero.secondary')}
          </a>
        </div>
        <ul className='inf-hero-perks'>
          <li><span className='inf-check' aria-hidden='true'>✓</span>{t('influencer_landing.hero.perk.payPerImage')}</li>
          <li><span className='inf-check' aria-hidden='true'>✓</span>{t('influencer_landing.hero.perk.refund')}</li>
          <li><span className='inf-check' aria-hidden='true'>✓</span>{t('influencer_landing.hero.perk.guest')}</li>
        </ul>

        <div className='inf-stage'>
          <div className='inf-eq-row'>
            <figure className='inf-media-card'>
              <img src={HERO_EQUATION.character} alt={character} />
              <figcaption className='inf-media-label'>{character}</figcaption>
            </figure>

            <span className='inf-eq-sign' aria-hidden='true'>+</span>

            <figure className='inf-media-card'>
              <HeroMedia
                src={HERO_EQUATION.anyVideo.src}
                poster={HERO_EQUATION.anyVideo.poster}
                label={anyVideo}
                register={(el) => {
                  mediaRefs.current[0] = el
                }}
              />
              <figcaption className='inf-media-label'>{anyVideo}</figcaption>
            </figure>

            <span className='inf-eq-sign' aria-hidden='true'>=</span>

            <figure className='inf-media-card inf-media-card--result'>
              <HeroMedia
                src={HERO_EQUATION.yourVideo.src}
                poster={HERO_EQUATION.yourVideo.poster}
                label={yourVideo}
                register={(el) => {
                  mediaRefs.current[1] = el
                }}
              />
              <figcaption className='inf-media-label inf-media-label--brand'>
                {yourVideo}
              </figcaption>
            </figure>
          </div>
        </div>
        <p className='inf-hero-proof'>{t('influencer_landing.hero.stageCaption')}</p>
      </div>
    </section>
  )
}
