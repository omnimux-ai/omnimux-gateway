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
import { InfluencerCreator } from './components/influencer/creator'
import { InfluencerHero } from './components/influencer/hero'
import {
  InfluencerFaq,
  InfluencerFinalCta,
  InfluencerPricing,
} from './components/influencer/pricing-faq-final'
import { InfluencerMarquee, InfluencerSteps } from './components/influencer/sections'
import { InfluencerMultiview, InfluencerWorks } from './components/influencer/works'
import { LandingFooter } from './components/landing-footer'
import { LandingHeader } from './components/landing-header'

import './landing.css'
import './influencer-landing.css'

export function OmniMuxLanding() {
  return (
    <div className='omnimux-landing'>
      <LandingHeader />
      <main>
        <InfluencerHero />
        <InfluencerMarquee />
        <InfluencerSteps />
        <InfluencerWorks />
        <InfluencerMultiview />
        <InfluencerCreator />
        <InfluencerPricing />
        <InfluencerFaq />
        <InfluencerFinalCta />
      </main>
      <LandingFooter />
    </div>
  )
}
