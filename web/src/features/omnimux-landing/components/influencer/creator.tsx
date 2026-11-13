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
import { useNavigate } from '@tanstack/react-router'
import { ArrowRight, ImagePlus, Sparkles } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { writeGuestDraft } from '@/features/influencer/lib/guest'
import { cn } from '@/lib/utils'

import { Reveal } from '../reveal'

/** How a visitor brings their own character in: by describing it, or by photo. */
type CreatorSource = 'describe' | 'photo'

const SOURCES = [
  {
    id: 'describe',
    key: 'influencer_landing.creator.tabDescribe',
    icon: Sparkles,
  },
  {
    id: 'photo',
    key: 'influencer_landing.creator.tabPhoto',
    icon: ImagePlus,
  },
] as const

/**
 * The "your turn" composer: a sentence or a photo, handed to the workbench
 * through the guest draft. The look itself (style, gender, boldness) is picked
 * in the studio, so the landing asks for one thing and gets out of the way.
 */
export function InfluencerCreator() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [brief, setBrief] = useState('')
  const [source, setSource] = useState<CreatorSource>('describe')

  // A photo needs no sentence; a sentence is the only thing the describe tab
  // can hand over.
  const canSubmit = useMemo(
    () => source === 'photo' || brief.trim().length > 0,
    [source, brief]
  )

  const submit = () => {
    writeGuestDraft({
      // No picks are made on this page any more, so the studio keeps its own
      // defaults instead of inheriting a hidden choice from the landing.
      tier: '',
      selection: {},
      brief: brief.trim(),
      seed: '',
      imageUrl: '',
      model: '',
      group: '',
      vendorId: null,
      // The photo slot lives on the workbench's video face, so a visitor who
      // came in through "Import a photo" must land there.
      mode: source === 'photo' ? 'video' : 'image',
    })
    navigate({ to: '/influencer' })
  }

  const tryExample = (textKey: string) => {
    setSource('describe')
    setBrief(t(textKey))
  }

  return (
    <section className='inf-creator'>
      <div className='inf-section inf-creator'>
        <Reveal>
          <p className='inf-eyebrow'>{t('influencer_landing.creator.eyebrow')}</p>
          <h2 className='inf-h2'>{t('influencer_landing.creator.title')}</h2>
          <p className='inf-sub'>{t('influencer_landing.creator.sub')}</p>
        </Reveal>

        <Reveal className='inf-creator-card' delay={120}>
          <div
            className='inf-creator-tabs'
            role='group'
            aria-label={t('influencer_landing.creator.title')}
          >
            {SOURCES.map((item) => {
              const Icon = item.icon
              return (
                <button
                  key={item.id}
                  type='button'
                  className={cn('inf-creator-tab', source === item.id && 'is-on')}
                  aria-pressed={source === item.id}
                  onClick={() => setSource(item.id)}
                >
                  <Icon className='h-4 w-4' aria-hidden='true' />
                  {t(item.key)}
                </button>
              )
            })}
          </div>

          {source === 'describe' ? (
            <div className='inf-creator-field'>
              <label className='inf-field-label' htmlFor='inf-brief'>
                {t('influencer_landing.creator.describe')}
              </label>
              <textarea
                id='inf-brief'
                className='inf-textarea'
                value={brief}
                onChange={(e) => setBrief(e.target.value)}
                placeholder={t('influencer_landing.creator.example1')}
              />
              <div className='inf-try-chips'>
                {['example1', 'example2', 'example3'].map((suffix) => (
                  <button
                    key={suffix}
                    type='button'
                    className='inf-try-btn'
                    onClick={() => tryExample(`influencer_landing.creator.${suffix}`)}
                  >
                    <em>{t('influencer_landing.creator.try')}</em>
                    {t(`influencer_landing.creator.${suffix}`)}
                  </button>
                ))}
              </div>
            </div>
          ) : (
            <div className='inf-creator-photo'>
              <ImagePlus className='h-6 w-6' aria-hidden='true' />
              <p>{t('influencer_landing.creator.photoHint')}</p>
            </div>
          )}

          <div className='inf-creator-actions'>
            <button
              type='button'
              className='omx-btn-primary inf-creator-submit'
              disabled={!canSubmit}
              onClick={submit}
            >
              {t('influencer_landing.creator.submit')}
              <ArrowRight className='h-4 w-4' aria-hidden='true' />
            </button>
            <p className='inf-creator-note'>{t('influencer_landing.creator.note')}</p>
          </div>
        </Reveal>
      </div>
    </section>
  )
}
