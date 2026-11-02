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

interface Props {
  count?: number
}

/**
 * Aspect ratios drawn from the mirrored preset catalogue so the skeleton
 * shares the footprint of the masonry grid it stands in for.
 */
const PRESET_RATIOS = [
  '3 / 4',
  '9 / 16',
  '4 / 5',
  '1 / 1',
  '3 / 4',
  '9 / 16',
  '4 / 5',
  '3 / 4',
]

/**
 * Masonry-shaped skeleton placeholder for the Explore library.
 * Replaces the flash of empty state while taxonomy and published presets load.
 */
export function ExploreSkeleton(props: Props) {
  const { t } = useTranslation()
  const count = props.count ?? 8

  return (
    <div
      role='status'
      aria-label={t('Loading inspiration library…')}
      className='columns-2 gap-4 py-4 sm:columns-3 xl:columns-4'
    >
      {Array.from({ length: count }, (_, index) => {
        const ratio = PRESET_RATIOS[index % PRESET_RATIOS.length]
        return (
          <div
            key={index}
            className='relative mb-4 break-inside-avoid overflow-hidden rounded-[var(--radius-card)] bg-[var(--inf-surface-2)] ring-1 ring-[var(--inf-line)] ring-inset'
            style={{ aspectRatio: ratio }}
          >
            <div className='inf-shimmer' />
          </div>
        )
      })}
    </div>
  )
}
