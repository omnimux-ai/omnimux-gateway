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
import { Link, useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { writeGuestDraft } from '@/features/influencer/lib/guest'

import { MULTIVIEW, WORK_LOOKS, draftFromLook, type InfluencerLook } from '../../lib/influencer-looks'
import { Reveal } from '../reveal'

/** Works wall: four real outputs that hand their exact traits to the workbench, plus an inspiration-library card. */
export function InfluencerWorks() {
  const { t } = useTranslation()
  const navigate = useNavigate()

  const handlePickLook = (look: InfluencerLook) => {
    writeGuestDraft(draftFromLook(look))
    navigate({ to: '/influencer' })
  }

  return (
    <section className='inf-works' id='works'>
      <div className='inf-section'>
        <div className='inf-works-head'>
          <div>
            <p className='inf-eyebrow'>{t('influencer_landing.works.eyebrow')}</p>
            <h2 className='inf-h2'>{t('influencer_landing.works.title')}</h2>
            <p className='inf-sub'>{t('influencer_landing.works.sub')}</p>
          </div>
        </div>
        <div className='inf-works-grid'>
          {WORK_LOOKS.map((look) => (
            <Reveal key={look.key}>
              <button
                type='button'
                className='inf-work'
                onClick={() => handlePickLook(look)}
                aria-label={t('influencer_landing.works.use')}
              >
                <img src={look.image} alt={t(`influencer_landing.work.${look.key}`)} loading='lazy' />
                <span className='inf-work-meta'>
                  <span className='inf-work-name'>{t(`influencer_landing.work.${look.key}`)}</span>
                  <span className='inf-work-use'>{t('influencer_landing.works.use')}</span>
                </span>
              </button>
            </Reveal>
          ))}
          <Reveal>
            <Link to='/influencer' className='inf-work inf-work--more'>
              <span className='inf-work-more-arrow' aria-hidden='true'>→</span>
              <span className='inf-work-more-label'>{t('influencer_landing.works.more')}</span>
              <span className='inf-work-more-sub'>{t('influencer_landing.works.moreSub')}</span>
            </Link>
          </Reveal>
        </div>
      </div>
    </section>
  )
}

/** Multi-view spotlight: sheet → turnaround, with a scanning highlight sweeping the angles. */
export function InfluencerMultiview() {
  const { t } = useTranslation()
  return (
    <section>
      <div className='inf-section inf-mv'>
        <Reveal className='inf-mv-copy'>
          <p className='inf-eyebrow'>{t('influencer_landing.multiview.eyebrow')}</p>
          <h2 className='inf-h2'>
            {t('influencer_landing.multiview.titleLead')}
            <span className='inf-h2-accent'>{t('influencer_landing.multiview.titleAccent')}</span>
          </h2>
          <p className='inf-sub'>{t('influencer_landing.multiview.sub')}</p>
          <ul className='inf-mv-points'>
            <li><span className='inf-check' aria-hidden='true'>✓</span>{t('influencer_landing.multiview.point1')}</li>
            <li><span className='inf-check' aria-hidden='true'>✓</span>{t('influencer_landing.multiview.point2')}</li>
            <li><span className='inf-check' aria-hidden='true'>✓</span>{t('influencer_landing.multiview.point3')}</li>
          </ul>
        </Reveal>
        <Reveal className='inf-mv-visual' delay={120}>
          <img className='inf-mv-before' src={MULTIVIEW.before} alt={t('influencer_landing.multiview.before')} loading='lazy' />
          <img className='inf-mv-after' src={MULTIVIEW.after} alt={t('influencer_landing.multiview.after')} loading='lazy' />
          <span className='inf-mv-scan' aria-hidden='true'><i /></span>
        </Reveal>
      </div>
    </section>
  )
}
