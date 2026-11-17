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
/**
 * Lightweight entry for LobeHub icons.
 *
 * The icon registry and every icon variant live in `lobe-icon`, which is
 * loaded on demand. Keeping this module free of `@lobehub/icons` imports stops
 * the ~5.7 MB registry chunk from entering the initial route graph.
 */
import { Suspense, type ReactNode } from 'react'

import LobeIconLazyEntry from './lobe-icon-lazy-entry'

function renderLobeIconFallback(
  name: string,
  size: number | string
): ReactNode {
  return (
    <div
      className='bg-muted text-muted-foreground flex items-center justify-center rounded-full text-xs font-medium'
      style={{ width: size, height: size }}
    >
      {name.charAt(0).toUpperCase() || '?'}
    </div>
  )
}

/**
 * Get LobeHub icon component by name, loading the icon registry on first use.
 * @param iconName - Icon name/description (e.g., "OpenAI", "OpenAI.Color", "Claude.Avatar")
 * @param size - Icon size (default: 20)
 */
export function getLobeIcon(
  iconName: string | undefined | null,
  size: number = 20
): ReactNode {
  const name = typeof iconName === 'string' ? iconName.trim() : ''
  return (
    <Suspense fallback={renderLobeIconFallback(name, size)}>
      <LobeIconLazyEntry name={name} size={size} />
    </Suspense>
  )
}
