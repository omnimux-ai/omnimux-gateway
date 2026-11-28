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
 * Sand AI official monochrome brand mark.
 * Source: https://sand.ai/migrated/assets/svg/favicon.svg (scaled to 24x24 viewBox).
 */
import type { SVGProps } from 'react'

type IconSandAIProps = SVGProps<SVGSVGElement> & {
  size?: number
}

export function IconSandAI({ size = 20, ...props }: IconSandAIProps) {
  return (
    <svg
      xmlns='http://www.w3.org/2000/svg'
      viewBox='0 0 24 24'
      width={size}
      height={size}
      fill='currentColor'
      aria-hidden={props['aria-label'] ? undefined : true}
      {...props}
    >
      <path
        fillRule='evenodd'
        clipRule='evenodd'
        d='M19.208 19.288C19.208 20.237 19.208 20.712 19.023 21.075C18.861 21.393 18.601 21.653 18.282 21.815C17.919 22 17.444 22 16.495 22H7.505C7.446 22 7.389 22 7.333 22C6.497 21.999 6.058 21.988 5.718 21.815C5.399 21.653 5.139 21.393 4.977 21.075C4.792 20.712 4.792 20.237 4.792 19.288V17.977C4.792 16.528 4.792 15.804 5.124 15.197C5.385 14.718 6.018 14.245 6.872 13.683C7.222 13.453 7.397 13.337 7.598 13.336C7.8 13.335 7.979 13.45 8.338 13.68L9.23 14.254C9.63 14.511 9.83 14.639 9.83 14.824C9.83 15.009 9.63 15.138 9.23 15.395L6.717 17.009C6.363 17.237 6.149 17.629 6.149 18.05C6.149 18.734 6.703 19.288 7.387 19.288H16.613C17.297 19.288 17.851 18.734 17.851 18.05C17.851 17.629 17.637 17.237 17.283 17.009L7.284 10.586C6.065 9.803 5.455 9.411 5.124 8.803C4.792 8.196 4.792 7.472 4.792 6.023V4.712C4.792 3.763 4.792 3.288 4.977 2.925C5.139 2.607 5.399 2.347 5.718 2.185C6.081 2 6.556 2 7.505 2L16.495 2C17.444 2 17.919 2 18.282 2.185C18.601 2.347 18.861 2.607 19.023 2.925C19.208 3.288 19.208 3.763 19.208 4.712V6.023C19.208 7.472 19.208 8.196 18.877 8.803C18.615 9.282 17.982 9.755 17.128 10.317C16.778 10.548 16.603 10.663 16.402 10.664C16.2 10.665 16.021 10.55 15.662 10.32L14.77 9.746C14.37 9.489 14.17 9.361 14.17 9.176C14.17 8.991 14.37 8.862 14.77 8.606L17.282 6.991C17.637 6.764 17.851 6.371 17.851 5.95C17.851 5.266 17.297 4.712 16.613 4.712L7.387 4.712C6.703 4.712 6.149 5.266 6.149 5.95C6.149 6.371 6.363 6.764 6.717 6.991L16.716 13.414C17.935 14.198 18.545 14.589 18.877 15.197C19.208 15.804 19.208 16.528 19.208 17.977V19.288Z'
      />
    </svg>
  )
}
