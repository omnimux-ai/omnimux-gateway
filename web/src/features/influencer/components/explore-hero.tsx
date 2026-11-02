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

import { useEffect, useRef, type CSSProperties } from 'react'
import { useTranslation } from 'react-i18next'

import { heroExplorePresets } from '../lib/motion-explore'

/**
 * The hero that leads the inspiration library: five motion clips fanned out
 * above the value line, matching the reference product's landing block. The
 * clips play muted on a loop — they are the product demo, not decoration.
 */
export function ExploreHero() {
  const { t } = useTranslation()
  const presets = heroExplorePresets()
  const stripRef = useRef<HTMLDivElement>(null)

  // Videos autoplay muted; browsers may still refuse the first play() call
  // before the clip has data, so nudge each one once mounted.
  useEffect(() => {
    const videos = stripRef.current?.querySelectorAll('video') ?? []
    for (const video of videos) {
      // jsdom has no real media pipeline and play() returns undefined there.
      video.play()?.catch(() => undefined)
    }
  }, [presets.length])

  if (!presets.length) return null

  return (
    <div className='inf-hero'>
      <div ref={stripRef} className='inf-hero-strip' aria-hidden='true'>
        {presets.map((preset, index) => (
          <video
            key={preset.id}
            className='inf-hero-card'
            style={{ '--hero-i': index } as CSSProperties}
            src={preset.videoUrl}
            poster={preset.thumbPath}
            muted
            loop
            playsInline
            autoPlay
            preload='metadata'
          />
        ))}
      </div>
      <h2 className='inf-hero-title'>{t('Your influencer, your viral hit')}</h2>
      <p className='inf-hero-sub'>
        {t(
          'Build your AI influencer with the face, body, and style you want. Choose a motion preset and watch them perform it in a video ready to share.'
        )}
      </p>
    </div>
  )
}
