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
 * GxgenAI graphic mark (eyeball: black ring + white disk + black bar).
 * Source: Gxgen apps/web/public/favicon.svg — not logo.png / logo-sizes wordmarks.
 */
import type { SVGProps } from 'react'

type IconGxgenAIProps = SVGProps<SVGSVGElement> & {
  size?: number
}

export function IconGxgenAI({ size = 20, className, ...props }: IconGxgenAIProps) {
  return (
    <svg
      xmlns='http://www.w3.org/2000/svg'
      viewBox='0 0 100 100'
      width={size}
      height={size}
      fill='none'
      className={className}
      aria-label='GxgenAI'
      role='img'
      {...props}
    >
      <circle cx='50' cy='50' r='50' fill='black' />
      <circle cx='50' cy='50' r='46' fill='white' />
      <ellipse cx='50' cy='52' rx='35' ry='6' fill='black' />
    </svg>
  )
}
