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

import { useTranslation } from 'react-i18next'

import { HISTORY_GRID_RATIO } from '../lib/history'

interface Props {
  count?: number
}

export function HistorySkeleton(props: Props) {
  const { t } = useTranslation()
  const count = props.count ?? 3

  return (
    <div
      role='status'
      aria-label={t('Loading history…')}
      className='inf-history-grid grid w-full'
    >
      {Array.from({ length: count }, (_, index) => (
        <div
          key={index}
          className='relative w-full overflow-hidden rounded-none bg-[var(--inf-surface-2)]'
          style={{ aspectRatio: HISTORY_GRID_RATIO }}
        >
          <div className='inf-shimmer' />
        </div>
      ))}
    </div>
  )
}
