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
import { History, Sparkles } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { InfluencerSource } from '../types'
import { SegmentedToggle, type SegmentedOption } from './segmented-toggle'

interface Props {
  value: InfluencerSource
  onChange: (value: InfluencerSource) => void
  className?: string
}

/**
 * Switches the right pane between the inspiration library and this user's own
 * history. Explore comes first because it is the default a first-time visitor
 * lands on.
 */
export function SourceTabs(props: Props) {
  const { t } = useTranslation()

  const options = useMemo<SegmentedOption<InfluencerSource>[]>(
    () => [
      {
        value: 'explore',
        label: t('Explore'),
        icon: <Sparkles className='h-3.5 w-3.5' aria-hidden='true' />,
      },
      {
        value: 'history',
        label: t('History'),
        icon: <History className='h-3.5 w-3.5' aria-hidden='true' />,
      },
    ],
    [t]
  )

  return (
    <SegmentedToggle
      value={props.value}
      onChange={props.onChange}
      options={options}
      ariaLabel={t('View')}
      variant='primary'
      className={props.className}
    />
  )
}
