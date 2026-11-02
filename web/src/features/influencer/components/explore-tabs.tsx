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

import { useMemo } from 'react'
import { Clapperboard, TrendingUp, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { InfluencerExploreTab } from '../types'
import { SegmentedToggle, type SegmentedOption } from './segmented-toggle'

interface Props {
  value: InfluencerExploreTab
  onChange: (value: InfluencerExploreTab) => void
}

/**
 * The library's three shelves: character presets first (what a new visitor
 * needs), then the video shelves mirrored from the reference product —
 * official motion presets and community trends.
 */
export function ExploreTabs(props: Props) {
  const { t } = useTranslation()

  const options = useMemo<SegmentedOption<InfluencerExploreTab>[]>(
    () => [
      {
        value: 'influencers',
        label: t('AI Influencers'),
        icon: <Users className='h-3.5 w-3.5' aria-hidden='true' />,
      },
      {
        value: 'presets',
        label: t('Motion presets'),
        icon: <Clapperboard className='h-3.5 w-3.5' aria-hidden='true' />,
      },
      {
        value: 'trends',
        label: t('Community trends'),
        icon: <TrendingUp className='h-3.5 w-3.5' aria-hidden='true' />,
      },
    ],
    [t]
  )

  return (
    <SegmentedToggle
      value={props.value}
      onChange={props.onChange}
      options={options}
      ariaLabel={t('Inspiration shelf')}
      variant='pill'
      className='inf-explore-tabs'
    />
  )
}
